package gep5

import (
	"fmt"
	"os"
)

// MOS 6502 Status Register (P) Flags
const (
	FlagC byte = 0x01 // Bit 0: Carry
	FlagZ byte = 0x02 // Bit 1: Zero
	FlagI byte = 0x04 // Bit 2: Interrupt Disable
	FlagD byte = 0x08 // Bit 3: Decimal Mode
	FlagB byte = 0x10 // Bit 4: Break Command
	FlagU byte = 0x20 // Bit 5: Unused (always 1)
	FlagV byte = 0x40 // Bit 6: Overflow
	FlagN byte = 0x80 // Bit 7: Negative
)

// Hardware Vector Addresses
const (
	VecNMI   uint16 = 0xFFFA
	VecReset uint16 = 0xFFFC
	VecIRQ   uint16 = 0xFFFE
)

// CPU implements the MOS 6502 8-bit microprocessor state machine.
type CPU struct {
	Bus *Bus

	// 8-bit Registers
	A  byte // Accumulator
	X  byte // Index Register X
	Y  byte // Index Register Y
	SP byte // Stack Pointer ($0100 + SP)
	P  byte // Processor Status Flags

	// 16-bit Program Counter
	PC uint16

	// Execution state
	Waiting  bool
	Halted   bool
	ExitCode int
	Cycles   uint64

	// Interrupt lines
	irqLine bool
	nmiLine bool

	// Trace hook
	OnInstruction func(pc uint16, op byte)
}

// NewCPU constructs an initialized MOS 6502 CPU attached to bus.
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

// Reset resets the CPU to MOS 6502 power-on state and loads PC from $FFFC..$FFFD in Task 0.
func (c *CPU) Reset() {
	c.Bus.CurrentTask = 0
	c.A = 0
	c.X = 0
	c.Y = 0
	c.SP = 0xFD
	c.P = FlagI | FlagU
	c.Waiting = false
	c.Halted = false
	c.ExitCode = 0
	c.Cycles = 0
	c.irqLine = false
	c.nmiLine = false
	c.PC = c.Bus.ReadWord(VecReset)
}

// 8-bit Stack Operations (Page 1: $0100..$01FF)
func (c *CPU) PushByte(val byte) {
	c.Bus.WriteByte(0x0100|uint16(c.SP), val)
	c.SP--
}

func (c *CPU) PullByte() byte {
	c.SP++
	return c.Bus.ReadByte(0x0100 | uint16(c.SP))
}

// 16-bit Stack Operations (6502 pushes high byte first, then low byte)
func (c *CPU) PushWord(val uint16) {
	c.PushByte(byte(val >> 8))
	c.PushByte(byte(val & 0xFF))
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

func (c *CPU) updateNZ(val byte) {
	c.P &^= (FlagN | FlagZ)
	if val == 0 {
		c.P |= FlagZ
	}
	if (val & 0x80) != 0 {
		c.P |= FlagN
	}
}

func (c *CPU) setFlag(flag byte, cond bool) {
	if cond {
		c.P |= flag
	} else {
		c.P &^= flag
	}
}

// TriggerNMI triggers a non-maskable interrupt on the next Step.
func (c *CPU) TriggerNMI() {
	c.nmiLine = true
}

func (c *CPU) serviceNMI() int {
	c.Waiting = false
	c.PushWord(c.PC)
	c.PushByte((c.P &^ FlagB) | FlagU)
	c.P |= FlagI
	c.Bus.InterruptedTask = c.Bus.CurrentTask
	c.Bus.TrapSourceTask = 0xFF
	c.Bus.CurrentTask = 0
	c.PC = c.Bus.ReadWord(VecNMI)
	c.nmiLine = false
	c.Cycles += 7
	return 7
}

func (c *CPU) serviceIRQ() int {
	c.Waiting = false
	c.PushWord(c.PC)
	c.PushByte((c.P &^ FlagB) | FlagU)
	c.P |= FlagI
	c.Bus.InterruptedTask = c.Bus.CurrentTask
	c.Bus.TrapSourceTask = 0xFF
	c.Bus.CurrentTask = 0
	c.PC = c.Bus.ReadWord(VecIRQ)
	c.Cycles += 7
	return 7
}

// Step executes one instruction or services pending interrupts.
func (c *CPU) Step() int {
	if c.Halted || c.Bus.Halted {
		c.Halted = true
		return 0
	}

	// Service NMI (highest priority)
	if c.nmiLine {
		return c.serviceNMI()
	}

	// Service maskable IRQ
	if c.irqLine && (c.P&FlagI) == 0 {
		return c.serviceIRQ()
	}

	if c.Waiting {
		if c.irqLine {
			c.Waiting = false
		} else {
			c.Cycles += 2
			return 2
		}
	}

	pc := c.PC
	c.Bus.LastPC = pc
	op := c.Bus.ReadByte(pc)

	if c.OnInstruction != nil {
		c.OnInstruction(pc, op)
	}

	c.PC++

	cycles := c.executeOp(op)
	c.Cycles += uint64(cycles)
	return cycles
}

// DumpState returns a concise single-line representation of CPU register state.
func (c *CPU) DumpState() string {
	return fmt.Sprintf("PC=%04X A=%02X X=%02X Y=%02X SP=%02X P=%02X Task=%d Flags=[%c%c%c%c%c%c]",
		c.PC, c.A, c.X, c.Y, c.SP, c.P|FlagU, c.Bus.CurrentTask,
		boolChar((c.P&FlagN) != 0, 'N', '.'),
		boolChar((c.P&FlagV) != 0, 'V', '.'),
		boolChar((c.P&FlagD) != 0, 'D', '.'),
		boolChar((c.P&FlagI) != 0, 'I', '.'),
		boolChar((c.P&FlagZ) != 0, 'Z', '.'),
		boolChar((c.P&FlagC) != 0, 'C', '.'),
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
	fmt.Fprintf(os.Stderr, "FATAL 6502 PANIC: %s\n%s\n", msg, c.DumpState())
	panic(msg)
}
