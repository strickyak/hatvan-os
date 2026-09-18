package vmk

import (
	"fmt"
)

// Status Register bit masks
const (
	FlagC     = 0x0001 // Carry
	FlagV     = 0x0002 // Overflow
	FlagZ     = 0x0004 // Zero
	FlagN     = 0x0008 // Negative
	FlagX     = 0x0010 // Extend
	MaskCCR   = 0x001F // All condition codes
	MaskIPL   = 0x0700 // Interrupt Priority Mask (I2, I1, I0)
	FlagS     = 0x2000 // Supervisor State
	FlagTrace = 0x8000 // Trace Mode
)

// Exception Vector numbers
const (
	VecResetSSP       = 0
	VecResetPC        = 1
	VecBusError       = 2
	VecAddressError   = 3
	VecIllegalInstr   = 4
	VecZeroDivide     = 5
	VecCHK            = 6
	VecTRAPV          = 7
	VecPrivilegeViol  = 8
	VecTrace          = 9
	VecLine1010       = 10
	VecLine1111       = 11
	VecSpuriousIRQ    = 24
	VecAutovectorBase = 24 // Level 1..7 are vectors 25..31
	VecTrapBase       = 32 // TRAP #0..#15 are vectors 32..47
)

// OpSize represents operand data width
type OpSize int

const (
	SizeByte OpSize = 1
	SizeWord OpSize = 2
	SizeLong OpSize = 4
)

// CPU represents the state of a Motorola 68000 processor.
type CPU struct {
	Bus *Bus

	// Registers
	D   [8]uint32 // Data registers D0-D7
	A   [8]uint32 // Address registers A0-A7 (A7 is active SP)
	USP uint32    // User Stack Pointer
	SSP uint32    // Supervisor Stack Pointer
	PC  uint32    // Program Counter (24-bit active)
	SR  uint16    // Status Register

	// Execution States
	Halted  bool
	Stopped bool   // Stopped waiting for interrupt (via STOP instruction)
	Cycles  uint64 // Total cycles executed

	// Interrupt line state
	PendingIPL uint8 // Latched interrupt priority level (0..7)
}

// NewCPU constructs an initialized CPU attached to a Bus.
func NewCPU(bus *Bus) *CPU {
	cpu := &CPU{
		Bus: bus,
		SR:  FlagS | MaskIPL, // Supervisor state, interrupts masked
	}
	bus.OnInterruptLevelChanged = func(level uint8) {
		cpu.PendingIPL = level
	}
	return cpu
}

// Reset performs the 68000 hardware reset sequence.
func (c *CPU) Reset() {
	c.Bus.CurrentFC = FCSupervisorData
	c.Halted = false
	c.Stopped = false
	c.SR = FlagS | MaskIPL

	// Fetch initial SSP from Vector 0 ($000000)
	c.SSP = c.Bus.ReadLong(0x000000)
	c.A[7] = c.SSP
	c.USP = 0

	// Fetch initial PC from Vector 1 ($000004)
	c.PC = c.Bus.ReadLong(0x000004)
	c.Cycles = 40
}

// SetSR updates the Status Register, handling supervisor/user stack swaps.
func (c *CPU) SetSR(newSR uint16) {
	oldS := (c.SR & FlagS) != 0
	newS := (newSR & FlagS) != 0

	if oldS != newS {
		if newS {
			// User -> Supervisor
			c.USP = c.A[7]
			c.A[7] = c.SSP
		} else {
			// Supervisor -> User
			c.SSP = c.A[7]
			c.A[7] = c.USP
		}
	}

	c.SR = newSR
	c.updateBusPrivilege()
}

// SetCCR updates only the user condition code byte (low 8 bits of SR).
func (c *CPU) SetCCR(ccr byte) {
	c.SR = (c.SR & 0xFF00) | uint16(ccr&MaskCCR)
}

// CCR returns the low 8 bits of SR (condition code register).
func (c *CPU) CCR() byte {
	return byte(c.SR & MaskCCR)
}

// String returns a formatted dump of CPU registers and status.
func (c *CPU) String() string {
	return fmt.Sprintf(
		"PC=%08X SR=%04X [T=%d S=%d IPL=%d X=%d N=%d Z=%d V=%d C=%d]\n"+
			"D: %08X %08X %08X %08X %08X %08X %08X %08X\n"+
			"A: %08X %08X %08X %08X %08X %08X %08X %08X (USP=%08X SSP=%08X)",
		c.PC, c.SR,
		boolToInt((c.SR&FlagTrace) != 0),
		boolToInt((c.SR&FlagS) != 0),
		(c.SR&MaskIPL)>>8,
		boolToInt((c.SR&FlagX) != 0),
		boolToInt((c.SR&FlagN) != 0),
		boolToInt((c.SR&FlagZ) != 0),
		boolToInt((c.SR&FlagV) != 0),
		boolToInt((c.SR&FlagC) != 0),
		c.D[0], c.D[1], c.D[2], c.D[3], c.D[4], c.D[5], c.D[6], c.D[7],
		c.A[0], c.A[1], c.A[2], c.A[3], c.A[4], c.A[5], c.A[6], c.A[7],
		c.USP, c.SSP,
	)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (c *CPU) updateBusPrivilege() {
	if (c.SR & FlagS) != 0 {
		c.Bus.CurrentFC = FCSupervisorData
	} else {
		c.Bus.CurrentFC = FCUserData
	}
}

// PushWord pushes a 16-bit word onto the active stack (A7).
func (c *CPU) PushWord(val uint16) {
	c.A[7] -= 2
	c.Bus.WriteWord(c.A[7], val)
}

// PushLong pushes a 32-bit longword onto the active stack (A7).
func (c *CPU) PushLong(val uint32) {
	c.A[7] -= 4
	c.Bus.WriteLong(c.A[7], val)
}

// PopWord pulls a 16-bit word from the active stack (A7).
func (c *CPU) PopWord() uint16 {
	val := c.Bus.ReadWord(c.A[7])
	c.A[7] += 2
	return val
}

// PopLong pulls a 32-bit longword from the active stack (A7).
func (c *CPU) PopLong() uint32 {
	val := c.Bus.ReadLong(c.A[7])
	c.A[7] += 4
	return val
}

// FetchInstructionWord fetches a 16-bit instruction word from PC and advances PC by 2.
func (c *CPU) FetchInstructionWord() uint16 {
	savedFC := c.Bus.CurrentFC
	if (c.SR & FlagS) != 0 {
		c.Bus.CurrentFC = FCSupervisorProg
	} else {
		c.Bus.CurrentFC = FCUserProgram
	}

	word := c.Bus.ReadWord(c.PC)
	c.PC += 2
	c.Bus.CurrentFC = savedFC
	return word
}

// TriggerException processes a 68000 exception vector.
func (c *CPU) TriggerException(vectorNum uint8) {
	oldSR := c.SR

	// If in user mode, switch to supervisor stack
	if (c.SR & FlagS) == 0 {
		c.USP = c.A[7]
		c.A[7] = c.SSP
	}

	// Enter supervisor mode, clear trace flag
	c.SR |= FlagS
	c.SR &^= FlagTrace
	c.updateBusPrivilege()

	// Push PC and SR onto Supervisor Stack
	c.PushLong(c.PC)
	c.PushWord(oldSR)

	// Fetch target PC from vector table
	vecAddr := uint32(vectorNum) * 4
	savedFC := c.Bus.CurrentFC
	c.Bus.CurrentFC = FCSupervisorData
	c.PC = c.Bus.ReadLong(vecAddr)
	c.Bus.CurrentFC = savedFC

	c.Stopped = false
	c.Cycles += 34
}

// Step executes one instruction or services a pending interrupt.
func (c *CPU) Step() int {
	if c.Halted {
		return 0
	}

	// 1. Check Interrupts
	currentIPL := uint8((c.SR & MaskIPL) >> 8)
	if c.PendingIPL > 0 && (c.PendingIPL > currentIPL || c.PendingIPL == 7) {
		level := c.PendingIPL
		c.TriggerException(VecAutovectorBase + level)
		// Update interrupt mask in SR to current interrupt level (except level 7 NMI)
		if level < 7 {
			c.SR = (c.SR &^ MaskIPL) | (uint16(level) << 8)
		}
		return 44
	}

	if c.Stopped {
		c.Cycles++
		return 1
	}

	startCycles := c.Cycles

	// 2. Fetch and decode opcode
	opcode := c.FetchInstructionWord()
	c.ExecuteOpcode(opcode)

	return int(c.Cycles - startCycles)
}

func (c *CPU) ExecuteOpcode(op uint16) {
	// Dispatched by ops.go
	c.dispatch(op)
}
