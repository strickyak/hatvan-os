package gepc

import "fmt"

// CPU implements the RCA COSMAC CDP1802 8-bit microprocessor.
type CPU struct {
	Bus *Bus

	// 16 Scratchpad Registers (16-bit)
	R [16]uint16

	// Pointer registers (4-bit nibbles)
	P uint8 // Selects PC register R[P]
	X uint8 // Selects Data Pointer register R[X]

	// 8-bit Accumulator
	D byte

	// 1-bit Data Flag (Carry / Borrow / Shift out)
	DF byte

	// 8-bit Temporary register (latches (X<<4)|P on interrupt/MARK)
	T byte

	// 1-bit Interrupt Enable
	IE byte

	// 1-bit Output flip-flop (Q)
	Q byte

	// Execution state
	Idle   bool
	Halted bool
	Cycles uint64

	// Callbacks
	OnInstruction func(pc uint16, op byte)
}

// NewCPU constructs an initialized CDP1802 CPU attached to bus.
func NewCPU(bus *Bus) *CPU {
	c := &CPU{
		Bus: bus,
	}
	c.Reset()
	return c
}

// Reset resets CPU state to standard CDP1802 power-on defaults.
func (c *CPU) Reset() {
	c.P = 0
	c.X = 0
	c.D = 0
	c.DF = 0
	c.T = 0
	c.IE = 1
	c.Q = 0
	c.Idle = false
	c.Halted = false
	c.Cycles = 0
	for i := 0; i < 16; i++ {
		c.R[i] = 0
	}
}

// PC returns the current Program Counter address (R[P]).
func (c *CPU) PC() uint16 {
	return c.R[c.P]
}

// SetPC sets the current Program Counter address (R[P]).
func (c *CPU) SetPC(addr uint16) {
	c.R[c.P] = addr
}

// Step executes a single machine cycle or instruction.
func (c *CPU) Step() {
	if c.Halted || c.Bus.Halted {
		c.Halted = true
		return
	}

	// Check hardware maskable interrupt (/INT)
	if c.IE == 1 && (c.Bus.RegStat&c.Bus.RegCtrl&0x03) != 0 {
		c.Idle = false
		// S3 State: Interrupt Response
		c.T = (c.X << 4) | (c.P & 0x0F)
		c.P = 1
		c.X = 2
		c.IE = 0
		c.Bus.CurrentTask = 0 // MMU forces Task 0 on interrupt response!
		c.Cycles += 16
		return
	}

	if c.Idle {
		c.Cycles += 16
		return
	}

	pc := c.R[c.P]
	op := c.Bus.ReadByte(pc)
	c.R[c.P]++

	if c.OnInstruction != nil {
		c.OnInstruction(pc, op)
	}

	c.executeOp(op)
}

// DumpState returns a concise single-line representation of CPU register state.
func (c *CPU) DumpState() string {
	return fmt.Sprintf("P=%X X=%X D=%02X DF=%d Q=%d T=%02X IE=%d R0=%04X R1=%04X R2=%04X R3=%04X R7=%04X R8=%04X R9=%04X RE=%04X RF=%04X",
		c.P, c.X, c.D, c.DF, c.Q, c.T, c.IE,
		c.R[0], c.R[1], c.R[2], c.R[3], c.R[7], c.R[8], c.R[9], c.R[14], c.R[15])
}

// DumpAllRegs returns all 16 scratchpad registers.
func (c *CPU) DumpAllRegs() string {
	return fmt.Sprintf("R0=%04X R1=%04X R2=%04X R3=%04X R4=%04X R5=%04X R6=%04X R7=%04X R8=%04X R9=%04X RA=%04X RB=%04X RC=%04X RD=%04X RE=%04X RF=%04X",
		c.R[0], c.R[1], c.R[2], c.R[3], c.R[4], c.R[5], c.R[6], c.R[7],
		c.R[8], c.R[9], c.R[10], c.R[11], c.R[12], c.R[13], c.R[14], c.R[15])
}
