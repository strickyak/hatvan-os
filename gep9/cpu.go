package gep9

import (
	"fmt"
	"strings"

	"github.com/strickyak/hatvan-os/gep9/data"
)

const (
	FlagC = 1 << 0
	FlagV = 1 << 1
	FlagZ = 1 << 2
	FlagN = 1 << 3
	FlagI = 1 << 4
	FlagH = 1 << 5
	FlagF = 1 << 6
	FlagE = 1 << 7
)

// PendingTrap tracks an active kernel trap invocation for a user task.
type PendingTrap struct {
	CallNum byte
	Call    *data.Call
	Task    byte
	PC      uint16
	BufAddr uint16
}

// CPU implements the Hitachi 6309 / Motorola 6809 processor core.
type CPU struct {
	// 6809 Registers
	A, B byte
	X, Y uint16
	U, S uint16
	PC   uint16
	DP   byte
	CC   byte

	// 6309 Extensions
	E, F byte   // Pair W
	V    uint16 // Register V
	MD   byte   // Mode Register (Bit 0 = 6309 Native Mode)

	Bus *Bus

	Halted           bool
	Waiting          bool
	Cycles           uint64
	ExitCode         int
	EnableHypercalls bool

	irqLine  bool
	firqLine bool

	PendingTraps [16]*PendingTrap
}

// NewCPU constructs an initialized CPU wired to the given bus.
func NewCPU(bus *Bus) *CPU {
	c := &CPU{
		Bus: bus,
	}
	bus.OnIRQChanged = func(asserted bool) {
		c.irqLine = asserted
	}
	bus.OnHalt = func(exitCode int) {
		c.Halted = true
		c.ExitCode = exitCode
	}
	return c
}

// Reset resets the CPU and loads PC from the Reset vector at $FFFE.
func (c *CPU) Reset() {
	c.Bus.CurrentTask = 0
	c.CC = FlagI | FlagF
	c.MD = 0 // Boot in 6809 Emulation mode
	c.Halted = false
	c.ExitCode = 0
	c.Waiting = false
	c.PC = c.Bus.ReadWord(0xFFFE)
}

// GetD returns the concatenated accumulator D = A:B.
func (c *CPU) GetD() uint16 {
	return (uint16(c.A) << 8) | uint16(c.B)
}

// SetD stores a 16-bit word into A and B.
func (c *CPU) SetD(val uint16) {
	c.A = byte(val >> 8)
	c.B = byte(val)
}

// GetW returns the 6309 concatenated accumulator W = E:F.
func (c *CPU) GetW() uint16 {
	return (uint16(c.E) << 8) | uint16(c.F)
}

// SetW stores a 16-bit word into 6309 E and F.
func (c *CPU) SetW(val uint16) {
	c.E = byte(val >> 8)
	c.F = byte(val)
}

func (c *CPU) fetchByte() byte {
	b := c.Bus.ReadByte(c.PC)
	c.PC++
	return b
}

func (c *CPU) fetchWord() uint16 {
	hi := uint16(c.fetchByte())
	lo := uint16(c.fetchByte())
	return (hi << 8) | lo
}

func (c *CPU) triggerVector(vecAddr uint16) {
	// When vector fetch occurs: task number switches to 0 during (BA,BS) == (0,1)
	c.Bus.CurrentTask = 0
	c.PC = c.Bus.ReadWord(vecAddr)
}

// Step executes a single CPU instruction or handles pending interrupts.
func (c *CPU) Step() int {
	if c.Halted {
		return 0
	}

	// Check FIRQ
	if c.firqLine && (c.CC&FlagF) == 0 {
		wasWaiting := c.Waiting
		c.Waiting = false
		if !wasWaiting {
			c.PushInterruptFrame(false)
		}
		c.CC |= FlagI | FlagF
		c.triggerVector(0xFFF6)
		c.Cycles += 10
		return 10
	}

	// Check IRQ
	if c.irqLine && (c.CC&FlagI) == 0 {
		wasWaiting := c.Waiting
		c.Waiting = false
		if !wasWaiting {
			c.PushInterruptFrame(true)
		}
		c.CC |= FlagI
		c.triggerVector(0xFFF8)
		c.Cycles += 19
		return 19
	}

	if c.Waiting {
		c.Cycles++
		return 1
	}

	cycles := c.executeInstruction()
	c.Cycles += uint64(cycles)
	return cycles
}

// CC Flag Helpers

func (c *CPU) updateNZ8(val byte) {
	c.CC &^= (FlagN | FlagZ)
	if val == 0 {
		c.CC |= FlagZ
	}
	if (val & 0x80) != 0 {
		c.CC |= FlagN
	}
}

func (c *CPU) updateNZ16(val uint16) {
	c.CC &^= (FlagN | FlagZ)
	if val == 0 {
		c.CC |= FlagZ
	}
	if (val & 0x8000) != 0 {
		c.CC |= FlagN
	}
}

func (c *CPU) setCarry(cond bool) {
	if cond {
		c.CC |= FlagC
	} else {
		c.CC &^= FlagC
	}
}

// ReadUserString reads a NUL-, CR-, or LF-terminated string from user task memory.
func (c *CPU) ReadUserString(task byte, addr uint16) string {
	if int(task) >= len(c.Bus.Memory) {
		return ""
	}
	mem := c.Bus.Memory[task]
	var b []byte
	for i := 0; i < 64; i++ {
		cur := addr + uint16(i)
		if cur >= 0xFF00 {
			break
		}
		ch := mem[cur]
		if ch == 0 || ch == '\r' || ch == '\n' {
			break
		}
		if (ch < 32 || ch > 126) && ch != 0 {
			break
		}
		b = append(b, ch)
	}
	return string(b)
}

// ReadUserPreview reads a preview of bytes from user task memory, escaping non-printables.
func (c *CPU) ReadUserPreview(task byte, addr uint16, maxLen int) string {
	if int(task) >= len(c.Bus.Memory) {
		return ""
	}
	if maxLen > 32 {
		maxLen = 32
	}
	mem := c.Bus.Memory[task]
	var res strings.Builder
	for i := 0; i < maxLen; i++ {
		cur := addr + uint16(i)
		if cur >= 0xFF00 {
			break
		}
		ch := mem[cur]
		if ch == '\n' {
			res.WriteString(`\n`)
		} else if ch == '\r' {
			res.WriteString(`\r`)
		} else if ch == '\t' {
			res.WriteString(`\t`)
		} else if ch >= 32 && ch <= 126 {
			res.WriteByte(ch)
		} else {
			res.WriteString(fmt.Sprintf("{%d}", ch))
		}
	}
	return res.String()
}

func (c *CPU) setOverflow(cond bool) {
	if cond {
		c.CC |= FlagV
	} else {
		c.CC &^= FlagV
	}
}
