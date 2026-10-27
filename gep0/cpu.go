package gep0

import (
	"fmt"
	"os"
)

// 8086 FLAGS bit masks
const (
	FlagCF uint16 = 1 << 0  // Carry Flag
	FlagPF uint16 = 1 << 2  // Parity Flag
	FlagAF uint16 = 1 << 4  // Auxiliary Carry Flag
	FlagZF uint16 = 1 << 6  // Zero Flag
	FlagSF uint16 = 1 << 7  // Sign Flag
	FlagTF uint16 = 1 << 8  // Trap Flag
	FlagIF uint16 = 1 << 9  // Interrupt Enable Flag
	FlagDF uint16 = 1 << 10 // Direction Flag
	FlagOF uint16 = 1 << 11 // Overflow Flag

	FlagsAlwaysOn uint16 = 0xF002 // Bits 1, 12..15 are always 1 on 8086
)

var parityTable [256]bool

func init() {
	for i := 0; i < 256; i++ {
		bits := 0
		v := i
		for v != 0 {
			bits += v & 1
			v >>= 1
		}
		parityTable[i] = (bits % 2) == 0
	}
}

// CPU models the Intel 8086 microprocessor.
type CPU struct {
	// 16-bit General-Purpose Registers
	AX, CX, DX, BX uint16
	SP, BP, SI, DI uint16

	// 16-bit Segment Registers
	ES, CS, SS, DS uint16

	// Instruction Pointer & Flags
	IP    uint16
	Flags uint16

	Bus *Bus

	// Execution & Interrupt State
	Cycles        uint64
	Halted        bool
	Waiting       bool
	ExitCode      int
	IRQLine       bool
	NMIPending    bool
	InhibitIRQ    bool // Inhibits hardware IRQ for 1 instruction after MOV SS / POP SS
	SegOverride   int  // -1 = none, 0 = ES, 1 = CS, 2 = SS, 3 = DS
	RepPrefix     byte // 0 = none, 0xF2 = REPNE, 0xF3 = REP/REPE
	OnInstruction func(cs, ip uint16, op byte)
}

// NewCPU creates an initialized Intel 8086 CPU attached to bus.
func NewCPU(bus *Bus) *CPU {
	c := &CPU{
		Bus:         bus,
		Flags:       FlagsAlwaysOn,
		SegOverride: -1,
	}
	bus.CPU = c
	bus.OnIRQChanged = func(asserted bool) {
		c.IRQLine = asserted
	}
	bus.OnNMITrigger = func() {
		c.NMIPending = true
	}
	return c
}

// Reset initializes the 8086 registers for Hatvan Small Model execution (CS=$1000, DS=SS=ES=$0000).
func (c *CPU) Reset() {
	c.AX, c.CX, c.DX, c.BX = 0, 0, 0, 0
	c.SP = 0xE000
	c.BP, c.SI, c.DI = 0, 0, 0
	c.CS = 0x1000
	c.DS = 0x0000
	c.SS = 0x0000
	c.ES = 0x0000
	c.IP = 0x0200
	c.Flags = FlagsAlwaysOn
	c.Halted = false
	c.Waiting = false
	c.SegOverride = -1
	c.RepPrefix = 0
	c.InhibitIRQ = false
}

// GetReg8 reads an 8-bit register by 3-bit index (0=AL, 1=CL, 2=DL, 3=BL, 4=AH, 5=CH, 6=DH, 7=BH).
func (c *CPU) GetReg8(reg byte) byte {
	switch reg & 7 {
	case 0:
		return byte(c.AX)
	case 1:
		return byte(c.CX)
	case 2:
		return byte(c.DX)
	case 3:
		return byte(c.BX)
	case 4:
		return byte(c.AX >> 8)
	case 5:
		return byte(c.CX >> 8)
	case 6:
		return byte(c.DX >> 8)
	case 7:
		return byte(c.BX >> 8)
	}
	return 0
}

// SetReg8 writes an 8-bit register by 3-bit index.
func (c *CPU) SetReg8(reg byte, val byte) {
	switch reg & 7 {
	case 0:
		c.AX = (c.AX & 0xFF00) | uint16(val)
	case 1:
		c.CX = (c.CX & 0xFF00) | uint16(val)
	case 2:
		c.DX = (c.DX & 0xFF00) | uint16(val)
	case 3:
		c.BX = (c.BX & 0xFF00) | uint16(val)
	case 4:
		c.AX = (c.AX & 0x00FF) | (uint16(val) << 8)
	case 5:
		c.CX = (c.CX & 0x00FF) | (uint16(val) << 8)
	case 6:
		c.DX = (c.DX & 0x00FF) | (uint16(val) << 8)
	case 7:
		c.BX = (c.BX & 0x00FF) | (uint16(val) << 8)
	}
}

// GetReg16 reads a 16-bit register by 3-bit index (0=AX, 1=CX, 2=DX, 3=BX, 4=SP, 5=BP, 6=SI, 7=DI).
func (c *CPU) GetReg16(reg byte) uint16 {
	switch reg & 7 {
	case 0:
		return c.AX
	case 1:
		return c.CX
	case 2:
		return c.DX
	case 3:
		return c.BX
	case 4:
		return c.SP
	case 5:
		return c.BP
	case 6:
		return c.SI
	case 7:
		return c.DI
	}
	return 0
}

// SetReg16 writes a 16-bit register by 3-bit index.
func (c *CPU) SetReg16(reg byte, val uint16) {
	switch reg & 7 {
	case 0:
		c.AX = val
	case 1:
		c.CX = val
	case 2:
		c.DX = val
	case 3:
		c.BX = val
	case 4:
		c.SP = val
	case 5:
		c.BP = val
	case 6:
		c.SI = val
	case 7:
		c.DI = val
	}
}

// GetSeg reads a 16-bit segment register (0=ES, 1=CS, 2=SS, 3=DS).
func (c *CPU) GetSeg(sreg byte) uint16 {
	switch sreg & 3 {
	case 0:
		return c.ES
	case 1:
		return c.CS
	case 2:
		return c.SS
	case 3:
		return c.DS
	}
	return 0
}

// SetSeg writes a 16-bit segment register (0=ES, 1=CS, 2=SS, 3=DS).
func (c *CPU) SetSeg(sreg byte, val uint16) {
	switch sreg & 3 {
	case 0:
		c.ES = val
	case 1:
		c.CS = val
	case 2:
		c.SS = val
		c.InhibitIRQ = true
	case 3:
		c.DS = val
	}
}

func (c *CPU) GetFlag(mask uint16) bool {
	return (c.Flags & mask) != 0
}

func (c *CPU) SetFlag(mask uint16, cond bool) {
	if cond {
		c.Flags |= mask
	} else {
		c.Flags &^= mask
	}
	c.Flags |= FlagsAlwaysOn
}

func (c *CPU) updateFlagsSZP8(res byte) {
	c.SetFlag(FlagZF, res == 0)
	c.SetFlag(FlagSF, (res&0x80) != 0)
	c.SetFlag(FlagPF, parityTable[res])
}

func (c *CPU) updateFlagsSZP16(res uint16) {
	c.SetFlag(FlagZF, res == 0)
	c.SetFlag(FlagSF, (res&0x8000) != 0)
	c.SetFlag(FlagPF, parityTable[byte(res)])
}

func (c *CPU) fetchByte() byte {
	b := c.Bus.ReadByteSeg(c.CS, c.IP)
	c.IP++
	return b
}

func (c *CPU) fetchWord() uint16 {
	lo := uint16(c.fetchByte())
	hi := uint16(c.fetchByte())
	return (hi << 8) | lo
}

func (c *CPU) push16(val uint16) {
	c.SP -= 2
	c.Bus.WriteWordSeg(c.SS, c.SP, val)
}

func (c *CPU) pop16() uint16 {
	val := c.Bus.ReadWordSeg(c.SS, c.SP)
	c.SP += 2
	return val
}

// Step executes a single 8086 instruction (including any prefixes) and services interrupts.
func (c *CPU) Step() int {
	if c.Halted {
		return 0
	}

	if c.NMIPending {
		c.NMIPending = false
		c.Waiting = false
		c.serviceHardwareInterrupt(2)
		c.Cycles += 50
		return 50
	}

	if !c.InhibitIRQ && c.IRQLine && c.GetFlag(FlagIF) {
		c.Waiting = false
		c.serviceHardwareInterrupt(8) // Hatvan OS hardware IRQ vector 08h ($00020)
		c.Cycles += 50
		return 50
	}

	c.InhibitIRQ = false

	if c.Waiting {
		c.Cycles += 4
		return 4
	}

	c.Bus.LastCS = c.CS
	c.Bus.LastIP = c.IP
	c.SegOverride = -1
	c.RepPrefix = 0

	// Consume instruction prefixes (Segment Overrides, REP/REPE/REPNE, LOCK)
	var op byte
	for {
		op = c.Bus.ReadByteSeg(c.CS, c.IP)
		if c.SegOverride == -1 && c.RepPrefix == 0 && c.OnInstruction != nil {
			c.OnInstruction(c.CS, c.IP, op)
		}
		c.IP++
		switch op {
		case 0x26: // ES:
			c.SegOverride = 0
			continue
		case 0x2E: // CS:
			c.SegOverride = 1
			continue
		case 0x36: // SS:
			c.SegOverride = 2
			continue
		case 0x3E: // DS:
			c.SegOverride = 3
			continue
		case 0xF0: // LOCK
			continue
		case 0xF2, 0xF3: // REPNE / REP
			c.RepPrefix = op
			continue
		}
		break
	}

	cycles := c.executeOp(op)
	c.Cycles += uint64(cycles)
	return cycles
}

// serviceHardwareInterrupt handles hardware IRQ (vector 08h) or NMI (vector 02h).
func (c *CPU) serviceHardwareInterrupt(vec byte) {
	c.Bus.TrapSourceTask = 0xFF
	c.Bus.InterruptedTask = c.Bus.CurrentTask

	c.push16(c.Flags | FlagsAlwaysOn)
	c.push16(c.CS)
	c.push16(c.IP)

	c.SetFlag(FlagIF, false)
	c.SetFlag(FlagTF, false)

	c.Bus.CurrentTask = 0
	vecAddr := uint32(vec) * 4
	ip := uint16(c.Bus.ReadTaskByte20(0, vecAddr)) | (uint16(c.Bus.ReadTaskByte20(0, vecAddr+1)) << 8)
	cs := uint16(c.Bus.ReadTaskByte20(0, vecAddr+2)) | (uint16(c.Bus.ReadTaskByte20(0, vecAddr+3)) << 8)
	c.IP = ip
	c.CS = cs
}

// triggerSoftwareInterrupt handles INT n instructions (including INT 30h Hatvan syscall trap).
func (c *CPU) triggerSoftwareInterrupt(vec byte) {
	if vec == 0x30 {
		// Hatvan OS syscall & driver sleep trap:
		// Inline byte immediately following `INT 30h` is the syscall number.
		callNum := c.fetchByte()
		c.Bus.TrapSyscallNum = callNum
		c.Bus.TrapSourceTask = c.Bus.CurrentTask
	} else {
		c.Bus.TrapSyscallNum = vec
		c.Bus.TrapSourceTask = c.Bus.CurrentTask
	}

	c.push16(c.Flags | FlagsAlwaysOn)
	c.push16(c.CS)
	c.push16(c.IP)

	c.SetFlag(FlagIF, false)
	c.SetFlag(FlagTF, false)

	c.Bus.CurrentTask = 0
	vecAddr := uint32(vec) * 4
	ip := uint16(c.Bus.ReadTaskByte20(0, vecAddr)) | (uint16(c.Bus.ReadTaskByte20(0, vecAddr+1)) << 8)
	cs := uint16(c.Bus.ReadTaskByte20(0, vecAddr+2)) | (uint16(c.Bus.ReadTaskByte20(0, vecAddr+3)) << 8)
	c.IP = ip
	c.CS = cs
}

// FormatFlags returns a human-readable string of active 8086 flags.
func (c *CPU) FormatFlags() string {
	b := []byte(".........")
	if c.GetFlag(FlagOF) {
		b[0] = 'O'
	}
	if c.GetFlag(FlagDF) {
		b[1] = 'D'
	}
	if c.GetFlag(FlagIF) {
		b[2] = 'I'
	}
	if c.GetFlag(FlagTF) {
		b[3] = 'T'
	}
	if c.GetFlag(FlagSF) {
		b[4] = 'S'
	}
	if c.GetFlag(FlagZF) {
		b[5] = 'Z'
	}
	if c.GetFlag(FlagAF) {
		b[6] = 'A'
	}
	if c.GetFlag(FlagPF) {
		b[7] = 'P'
	}
	if c.GetFlag(FlagCF) {
		b[8] = 'C'
	}
	return string(b)
}

// DumpState returns a formatted one-line summary of the CPU state.
func (c *CPU) DumpState() string {
	return fmt.Sprintf("CS:IP=%04X:%04X AX=%04X BX=%04X CX=%04X DX=%04X SP=%04X BP=%04X SI=%04X DI=%04X DS=%04X SS=%04X ES=%04X Task=%d Flags=[%s]",
		c.CS, c.IP, c.AX, c.BX, c.CX, c.DX, c.SP, c.BP, c.SI, c.DI, c.DS, c.SS, c.ES, c.Bus.CurrentTask, c.FormatFlags())
}

// Panic prints CPU state and panics with a formatted error message.
func (c *CPU) Panic(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "\nFATAL 8086 PANIC: %s\n%s\n", msg, c.DumpState())
	panic(msg)
}
