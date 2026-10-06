package gepz

import (
	"fmt"
	"os"
)

// CPU implements the Zilog Z80 microprocessor state machine.
type CPU struct {
	Bus *Bus

	// 8-bit Main Registers
	A, F byte
	B, C byte
	D, E byte
	H, L byte

	// 8-bit Alternate Registers
	A_, F_ byte
	B_, C_ byte
	D_, E_ byte
	H_, L_ byte

	// 16-bit Index Registers
	IX uint16
	IY uint16

	// 16-bit Stack Pointer and Program Counter
	SP uint16
	PC uint16

	// Special Registers
	I byte
	R byte

	// Interrupt Flip-Flops & Mode
	IFF1 bool // Active interrupt enable flip-flop
	IFF2 bool // Temporary storage for IFF1 during NMI
	IM   byte // Interrupt Mode: 0, 1, or 2

	// Execution state
	Halted   bool
	ExitCode int
	Cycles   uint64

	// Interrupt Lines
	irqLine bool
	nmiLine bool
	eiDelay bool // EI delays interrupt enabling by one instruction

	// Trace hook
	OnInstruction func(pc uint16, op byte)
}

func NewCPU(bus *Bus) *CPU {
	c := &CPU{
		Bus: bus,
	}
	bus.CPU = c
	bus.OnHalt = func(exitCode int) {
		c.Halted = true
		c.ExitCode = exitCode
	}
	bus.OnIRQChanged = func(asserted bool) {
		c.irqLine = asserted
	}
	bus.OnNMITrigger = func() {
		c.nmiLine = true
	}
	c.Reset()
	return c
}

func (c *CPU) Reset() {
	c.Bus.CurrentTask = 0
	c.A = 0xFF
	c.F = 0xFF
	c.B, c.C = 0, 0
	c.D, c.E = 0, 0
	c.H, c.L = 0, 0
	c.A_, c.F_ = 0xFF, 0xFF
	c.B_, c.C_ = 0, 0
	c.D_, c.E_ = 0, 0
	c.H_, c.L_ = 0, 0
	c.IX = 0xFFFF
	c.IY = 0xFFFF
	c.SP = 0xFFFF
	c.PC = 0x0000
	c.I = 0
	c.R = 0
	c.IFF1 = false
	c.IFF2 = false
	c.IM = 0
	c.Halted = false
	c.ExitCode = 0
	c.Cycles = 0
	c.irqLine = false
	c.nmiLine = false
	c.eiDelay = false
}

// 16-bit Register Pair Getters & Setters
func (c *CPU) GetBC() uint16 {
	return (uint16(c.B) << 8) | uint16(c.C)
}

func (c *CPU) SetBC(val uint16) {
	c.B = byte(val >> 8)
	c.C = byte(val)
}

func (c *CPU) GetDE() uint16 {
	return (uint16(c.D) << 8) | uint16(c.E)
}

func (c *CPU) SetDE(val uint16) {
	c.D = byte(val >> 8)
	c.E = byte(val)
}

func (c *CPU) GetHL() uint16 {
	return (uint16(c.H) << 8) | uint16(c.L)
}

func (c *CPU) SetHL(val uint16) {
	c.H = byte(val >> 8)
	c.L = byte(val)
}

func (c *CPU) GetAF() uint16 {
	return (uint16(c.A) << 8) | uint16(c.F)
}

func (c *CPU) SetAF(val uint16) {
	c.A = byte(val >> 8)
	c.F = byte(val)
}

// 8-bit Stack Operations
func (c *CPU) PushByte(val byte) {
	c.SP--
	c.Bus.WriteByte(c.SP, val)
}

func (c *CPU) PullByte() byte {
	val := c.Bus.ReadByte(c.SP)
	c.SP++
	return val
}

// 16-bit Stack Operations (Little-endian in memory)
func (c *CPU) PushWord(val uint16) {
	c.PushByte(byte(val >> 8))   // High byte pushed first
	c.PushByte(byte(val & 0xFF)) // Low byte pushed second (at lower SP)
}

func (c *CPU) PullWord() uint16 {
	lo := uint16(c.PullByte())
	hi := uint16(c.PullByte())
	return (hi << 8) | lo
}

func (c *CPU) fetchByte() byte {
	val := c.Bus.ReadByte(c.PC)
	c.PC++
	return val
}

func (c *CPU) fetchWord() uint16 {
	lo := uint16(c.fetchByte())
	hi := uint16(c.fetchByte())
	return (hi << 8) | lo
}

// Interrupt Handling
func (c *CPU) triggerNMI() int {
	c.Halted = false
	c.IFF2 = c.IFF1
	c.IFF1 = false

	// Push return PC onto current active stack (in active task)
	c.PushWord(c.PC)

	// MMU switches to Task 0 upon vector fetch at $0066
	c.Bus.CurrentTask = 0
	c.PC = 0x0066
	c.nmiLine = false
	c.Cycles += 11
	return 11
}

func (c *CPU) triggerINT() int {
	c.Halted = false
	c.IFF1 = false
	c.IFF2 = false

	// Mode 1: Automatically vectors to $0038
	c.PushWord(c.PC)
	c.Bus.CurrentTask = 0
	c.PC = 0x0038
	c.Cycles += 13
	return 13
}

// Step executes one instruction or services pending interrupts.
func (c *CPU) Step() int {
	if c.Halted {
		// When halted, processor still checks interrupts
		if c.nmiLine || c.Bus.TrapPending {
			c.Bus.TrapPending = false
			return c.triggerNMI()
		}
		if c.irqLine && c.IFF1 && !c.eiDelay {
			return c.triggerINT()
		}
		c.Cycles += 4
		return 4
	}

	// Service NMI (highest priority)
	if c.nmiLine || c.Bus.TrapPending {
		c.Bus.TrapPending = false
		return c.triggerNMI()
	}

	// Service maskable INT
	if c.irqLine && c.IFF1 && !c.eiDelay {
		return c.triggerINT()
	}

	if c.eiDelay {
		c.eiDelay = false
		c.IFF1 = true
		c.IFF2 = true
	}

	pc := c.PC
	c.Bus.LastPC = pc
	op := c.Bus.ReadByte(pc)

	// Advance refresh counter
	c.R = (c.R & 0x80) | ((c.R + 1) & 0x7F)

	if c.OnInstruction != nil {
		c.OnInstruction(pc, op)
	}

	c.PC++

	cycles := c.executeOp(op)
	c.Cycles += uint64(cycles)
	return cycles
}

func (c *CPU) DumpState() string {
	return fmt.Sprintf("PC=%04X SP=%04X AF=%04X BC=%04X DE=%04X HL=%04X IX=%04X IY=%04X Task=%d Flags=[%c%c%c%c%c%c]",
		c.PC, c.SP, c.GetAF(), c.GetBC(), c.GetDE(), c.GetHL(), c.IX, c.IY, c.Bus.CurrentTask,
		boolChar((c.F&FlagS) != 0, 'S', '.'),
		boolChar((c.F&FlagZ) != 0, 'Z', '.'),
		boolChar((c.F&FlagH) != 0, 'H', '.'),
		boolChar((c.F&FlagV) != 0, 'V', '.'),
		boolChar((c.F&FlagN) != 0, 'N', '.'),
		boolChar((c.F&FlagC) != 0, 'C', '.'),
	)
}

func boolChar(cond bool, t, f rune) rune {
	if cond {
		return t
	}
	return f
}

func (c *CPU) Panic(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "FATAL Z80 PANIC: %s\n%s\n", msg, c.DumpState())
	panic(msg)
}
