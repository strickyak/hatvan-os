package vmk

// EAOperand holds resolved effective addressing information.
type EAOperand struct {
	Mode         uint8
	Reg          uint8
	Size         OpSize
	Addr         uint32 // computed address for memory modes
	IsReg        bool   // true for Dn
	IsAReg       bool   // true for An
	IsImm        bool   // true for immediate #<data>
	ImmVal       uint32 // value for immediate #<data>
	PostIncReg   int    // -1 if none, 0..7 if postincrement pending
	PostIncDelta uint32 // amount to increment A[reg] when committed
}

// ResolveEA resolves an Effective Address mode and register.
// If the addressing mode requires extension words (displacement, absolute address, immediate),
// they are fetched from PC.
func (c *CPU) ResolveEA(mode, reg uint8, size OpSize) *EAOperand {
	op := &EAOperand{
		Mode:       mode,
		Reg:        reg,
		Size:       size,
		PostIncReg: -1,
	}

	switch mode {
	case 0: // Dn
		op.IsReg = true

	case 1: // An
		op.IsAReg = true

	case 2: // (An)
		op.Addr = c.A[reg]

	case 3: // (An)+
		op.Addr = c.A[reg]
		delta := uint32(size)
		if reg == 7 && size == SizeByte {
			delta = 2
		}
		op.PostIncReg = int(reg)
		op.PostIncDelta = delta

	case 4: // -(An)
		delta := uint32(size)
		if reg == 7 && size == SizeByte {
			delta = 2
		}
		c.A[reg] -= delta
		op.Addr = c.A[reg]

	case 5: // (d16, An)
		disp := int32(int16(c.FetchInstructionWord()))
		op.Addr = uint32(int32(c.A[reg]) + disp)

	case 6: // (d8, An, Xn)
		ext := c.FetchInstructionWord()
		d8 := int32(int8(ext & 0xFF))
		idxReg := (ext >> 12) & 7
		isAn := (ext & 0x8000) != 0
		isLong := (ext & 0x0800) != 0

		var idxVal int32
		if isAn {
			idxVal = int32(c.A[idxReg])
		} else {
			idxVal = int32(c.D[idxReg])
		}
		if !isLong {
			idxVal = int32(int16(idxVal))
		}
		op.Addr = uint32(int32(c.A[reg]) + idxVal + d8)

	case 7: // Special modes
		switch reg {
		case 0: // (xxx).W Absolute Short
			w := c.FetchInstructionWord()
			op.Addr = uint32(int32(int16(w)))

		case 1: // (xxx).L Absolute Long
			hi := c.FetchInstructionWord()
			lo := c.FetchInstructionWord()
			op.Addr = (uint32(hi) << 16) | uint32(lo)

		case 2: // (d16, PC)
			basePC := c.PC
			disp := int32(int16(c.FetchInstructionWord()))
			op.Addr = uint32(int32(basePC) + disp)

		case 3: // (d8, PC, Xn)
			basePC := c.PC
			ext := c.FetchInstructionWord()
			d8 := int32(int8(ext & 0xFF))
			idxReg := (ext >> 12) & 7
			isAn := (ext & 0x8000) != 0
			isLong := (ext & 0x0800) != 0

			var idxVal int32
			if isAn {
				idxVal = int32(c.A[idxReg])
			} else {
				idxVal = int32(c.D[idxReg])
			}
			if !isLong {
				idxVal = int32(int16(idxVal))
			}
			op.Addr = uint32(int32(basePC) + idxVal + d8)

		case 4: // Immediate #<data>
			op.IsImm = true
			switch size {
			case SizeByte:
				w := c.FetchInstructionWord()
				op.ImmVal = uint32(w & 0xFF)
			case SizeWord:
				w := c.FetchInstructionWord()
				op.ImmVal = uint32(w)
			case SizeLong:
				hi := c.FetchInstructionWord()
				lo := c.FetchInstructionWord()
				op.ImmVal = (uint32(hi) << 16) | uint32(lo)
			}

		default:
			// Mode 7, Reg 5..7 are invalid
			c.TriggerException(VecIllegalInstr)
		}

	default:
		c.TriggerException(VecIllegalInstr)
	}

	return op
}

// ReadEA reads the value from an effective address operand.
func (c *CPU) ReadEA(op *EAOperand) uint32 {
	if op.IsReg {
		switch op.Size {
		case SizeByte:
			return c.D[op.Reg] & 0xFF
		case SizeWord:
			return c.D[op.Reg] & 0xFFFF
		case SizeLong:
			return c.D[op.Reg]
		}
	}

	if op.IsAReg {
		switch op.Size {
		case SizeWord:
			return c.A[op.Reg] & 0xFFFF
		case SizeLong:
			return c.A[op.Reg]
		}
	}

	if op.IsImm {
		return op.ImmVal
	}

	// Memory operand
	switch op.Size {
	case SizeByte:
		return uint32(c.Bus.ReadByte(op.Addr))
	case SizeWord:
		return uint32(c.Bus.ReadWord(op.Addr))
	case SizeLong:
		return c.Bus.ReadLong(op.Addr)
	}

	return 0
}

// WriteEA writes a value to an effective address operand.
func (c *CPU) WriteEA(op *EAOperand, val uint32) {
	if op.IsReg {
		switch op.Size {
		case SizeByte:
			c.D[op.Reg] = (c.D[op.Reg] & 0xFFFFFF00) | (val & 0xFF)
		case SizeWord:
			c.D[op.Reg] = (c.D[op.Reg] & 0xFFFF0000) | (val & 0xFFFF)
		case SizeLong:
			c.D[op.Reg] = val
		}
		return
	}

	if op.IsAReg {
		switch op.Size {
		case SizeWord:
			// Word writes to An are sign-extended to 32 bits
			c.A[op.Reg] = uint32(int32(int16(val)))
		case SizeLong:
			c.A[op.Reg] = val
		}
		return
	}

	if op.IsImm {
		// Immediate is not alterable
		c.TriggerException(VecIllegalInstr)
		return
	}

	// Memory operand
	switch op.Size {
	case SizeByte:
		c.Bus.WriteByte(op.Addr, byte(val))
	case SizeWord:
		c.Bus.WriteWord(op.Addr, uint16(val))
	case SizeLong:
		c.Bus.WriteLong(op.Addr, val)
	}
}

// CommitEA applies any pending postincrement side-effects to address registers.
func (c *CPU) CommitEA(op *EAOperand) {
	if op.PostIncReg >= 0 {
		c.A[op.PostIncReg] += op.PostIncDelta
		op.PostIncReg = -1
	}
}
