package gep5

// Addressing mode helpers

func (c *CPU) addrZP() uint16 {
	return uint16(c.fetchByte())
}

func (c *CPU) addrZPX() uint16 {
	return uint16(c.fetchByte() + c.X)
}

func (c *CPU) addrZPY() uint16 {
	return uint16(c.fetchByte() + c.Y)
}

func (c *CPU) addrAbs() uint16 {
	return c.fetchWord()
}

func (c *CPU) addrAbsX() (uint16, int) {
	base := c.fetchWord()
	addr := base + uint16(c.X)
	if (base & 0xFF00) != (addr & 0xFF00) {
		return addr, 1
	}
	return addr, 0
}

func (c *CPU) addrAbsY() (uint16, int) {
	base := c.fetchWord()
	addr := base + uint16(c.Y)
	if (base & 0xFF00) != (addr & 0xFF00) {
		return addr, 1
	}
	return addr, 0
}

func (c *CPU) addrIndX() uint16 {
	zp := c.fetchByte() + c.X
	lo := uint16(c.Bus.ReadByte(uint16(zp)))
	hi := uint16(c.Bus.ReadByte(uint16(zp + 1)))
	return (hi << 8) | lo
}

func (c *CPU) addrIndY() (uint16, int) {
	zp := c.fetchByte()
	lo := uint16(c.Bus.ReadByte(uint16(zp)))
	hi := uint16(c.Bus.ReadByte(uint16(zp + 1)))
	base := (hi << 8) | lo
	addr := base + uint16(c.Y)
	if (base & 0xFF00) != (addr & 0xFF00) {
		return addr, 1
	}
	return addr, 0
}

// ALU & Shift helpers

func (c *CPU) adc(val byte) {
	carry := uint16(0)
	if (c.P & FlagC) != 0 {
		carry = 1
	}

	if (c.P & FlagD) != 0 {
		// BCD addition
		al := (uint16(c.A) & 0x0F) + (uint16(val) & 0x0F) + carry
		if al > 9 {
			al += 6
		}
		ah := (uint16(c.A) >> 4) + (uint16(val) >> 4)
		if al > 0x0F {
			ah++
		}
		binRes := uint16(c.A) + uint16(val) + carry
		c.setFlag(FlagZ, byte(binRes) == 0)
		resHi := ah << 4
		c.setFlag(FlagN, (resHi&0x80) != 0)
		c.setFlag(FlagV, ((^(uint16(c.A)^uint16(val)))&(uint16(c.A)^resHi)&0x80) != 0)
		if ah > 9 {
			ah += 6
		}
		c.setFlag(FlagC, ah > 0x0F)
		c.A = byte((ah << 4) | (al & 0x0F))
		return
	}

	sum := uint16(c.A) + uint16(val) + carry
	res := byte(sum)
	c.setFlag(FlagC, sum > 0xFF)
	c.setFlag(FlagV, ((^(c.A ^ val))&(c.A^res)&0x80) != 0)
	c.A = res
	c.updateNZ(c.A)
}

func (c *CPU) sbc(val byte) {
	if (c.P & FlagD) != 0 {
		// BCD subtraction
		borrow := uint16(0)
		if (c.P & FlagC) == 0 {
			borrow = 1
		}
		binDiff := uint16(c.A) - uint16(val) - borrow
		al := int16(c.A&0x0F) - int16(val&0x0F) - int16(borrow)
		if al < 0 {
			al = ((al - 6) & 0x0F) - 0x10
		}
		ah := int16(c.A>>4) - int16(val>>4)
		if al < 0 {
			ah--
		}
		if ah < 0 {
			ah -= 6
		}
		c.setFlag(FlagC, binDiff < 0x100)
		c.setFlag(FlagV, (((c.A ^ val) & (c.A ^ byte(binDiff)) & 0x80) != 0))
		c.updateNZ(byte(binDiff))
		c.A = byte(((ah & 0x0F) << 4) | (al & 0x0F))
		return
	}
	c.adc(^val)
}

func (c *CPU) cmp(reg, val byte) {
	diff := uint16(reg) - uint16(val)
	c.setFlag(FlagC, reg >= val)
	c.updateNZ(byte(diff))
}

func (c *CPU) aslByte(val byte) byte {
	c.setFlag(FlagC, (val&0x80) != 0)
	res := val << 1
	c.updateNZ(res)
	return res
}

func (c *CPU) lsrByte(val byte) byte {
	c.setFlag(FlagC, (val&0x01) != 0)
	res := val >> 1
	c.updateNZ(res)
	return res
}

func (c *CPU) rolByte(val byte) byte {
	carryIn := byte(0)
	if (c.P & FlagC) != 0 {
		carryIn = 1
	}
	c.setFlag(FlagC, (val&0x80) != 0)
	res := (val << 1) | carryIn
	c.updateNZ(res)
	return res
}

func (c *CPU) rorByte(val byte) byte {
	carryIn := byte(0)
	if (c.P & FlagC) != 0 {
		carryIn = 0x80
	}
	c.setFlag(FlagC, (val&0x01) != 0)
	res := (val >> 1) | carryIn
	c.updateNZ(res)
	return res
}

func (c *CPU) branch(cond bool) int {
	offset := int8(c.fetchByte())
	if !cond {
		return 2
	}
	oldPC := c.PC
	c.PC = uint16(int32(c.PC) + int32(offset))
	if (oldPC & 0xFF00) != (c.PC & 0xFF00) {
		return 4
	}
	return 3
}

// executeOp decodes and executes a single MOS 6502 opcode, returning cycle count.
func (c *CPU) executeOp(op byte) int {
	switch op {
	// --- Load / Store ---

	// LDA
	case 0xA9: // LDA #imm
		c.A = c.fetchByte()
		c.updateNZ(c.A)
		return 2
	case 0xA5: // LDA zp
		c.A = c.Bus.ReadByte(c.addrZP())
		c.updateNZ(c.A)
		return 3
	case 0xB5: // LDA zp,X
		c.A = c.Bus.ReadByte(c.addrZPX())
		c.updateNZ(c.A)
		return 4
	case 0xAD: // LDA abs
		c.A = c.Bus.ReadByte(c.addrAbs())
		c.updateNZ(c.A)
		return 4
	case 0xBD: // LDA abs,X
		addr, cross := c.addrAbsX()
		c.A = c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 4 + cross
	case 0xB9: // LDA abs,Y
		addr, cross := c.addrAbsY()
		c.A = c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 4 + cross
	case 0xA1: // LDA (zp,X)
		c.A = c.Bus.ReadByte(c.addrIndX())
		c.updateNZ(c.A)
		return 6
	case 0xB1: // LDA (zp),Y
		addr, cross := c.addrIndY()
		c.A = c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 5 + cross

	// LDX
	case 0xA2: // LDX #imm
		c.X = c.fetchByte()
		c.updateNZ(c.X)
		return 2
	case 0xA6: // LDX zp
		c.X = c.Bus.ReadByte(c.addrZP())
		c.updateNZ(c.X)
		return 3
	case 0xB6: // LDX zp,Y
		c.X = c.Bus.ReadByte(c.addrZPY())
		c.updateNZ(c.X)
		return 4
	case 0xAE: // LDX abs
		c.X = c.Bus.ReadByte(c.addrAbs())
		c.updateNZ(c.X)
		return 4
	case 0xBE: // LDX abs,Y
		addr, cross := c.addrAbsY()
		c.X = c.Bus.ReadByte(addr)
		c.updateNZ(c.X)
		return 4 + cross

	// LDY
	case 0xA0: // LDY #imm
		c.Y = c.fetchByte()
		c.updateNZ(c.Y)
		return 2
	case 0xA4: // LDY zp
		c.Y = c.Bus.ReadByte(c.addrZP())
		c.updateNZ(c.Y)
		return 3
	case 0xB4: // LDY zp,X
		c.Y = c.Bus.ReadByte(c.addrZPX())
		c.updateNZ(c.Y)
		return 4
	case 0xAC: // LDY abs
		c.Y = c.Bus.ReadByte(c.addrAbs())
		c.updateNZ(c.Y)
		return 4
	case 0xBC: // LDY abs,X
		addr, cross := c.addrAbsX()
		c.Y = c.Bus.ReadByte(addr)
		c.updateNZ(c.Y)
		return 4 + cross

	// STA
	case 0x85: // STA zp
		c.Bus.WriteByte(c.addrZP(), c.A)
		return 3
	case 0x95: // STA zp,X
		c.Bus.WriteByte(c.addrZPX(), c.A)
		return 4
	case 0x8D: // STA abs
		c.Bus.WriteByte(c.addrAbs(), c.A)
		return 4
	case 0x9D: // STA abs,X
		addr, _ := c.addrAbsX()
		c.Bus.WriteByte(addr, c.A)
		return 5
	case 0x99: // STA abs,Y
		addr, _ := c.addrAbsY()
		c.Bus.WriteByte(addr, c.A)
		return 5
	case 0x81: // STA (zp,X)
		c.Bus.WriteByte(c.addrIndX(), c.A)
		return 6
	case 0x91: // STA (zp),Y
		addr, _ := c.addrIndY()
		c.Bus.WriteByte(addr, c.A)
		return 6

	// STX
	case 0x86: // STX zp
		c.Bus.WriteByte(c.addrZP(), c.X)
		return 3
	case 0x96: // STX zp,Y
		c.Bus.WriteByte(c.addrZPY(), c.X)
		return 4
	case 0x8E: // STX abs
		c.Bus.WriteByte(c.addrAbs(), c.X)
		return 4

	// STY
	case 0x84: // STY zp
		c.Bus.WriteByte(c.addrZP(), c.Y)
		return 3
	case 0x94: // STY zp,X
		c.Bus.WriteByte(c.addrZPX(), c.Y)
		return 4
	case 0x8C: // STY abs
		c.Bus.WriteByte(c.addrAbs(), c.Y)
		return 4

	// --- Register Transfers ---

	case 0xAA: // TAX
		c.X = c.A
		c.updateNZ(c.X)
		return 2
	case 0xA8: // TAY
		c.Y = c.A
		c.updateNZ(c.Y)
		return 2
	case 0x8A: // TXA
		c.A = c.X
		c.updateNZ(c.A)
		return 2
	case 0x98: // TYA
		c.A = c.Y
		c.updateNZ(c.A)
		return 2
	case 0xBA: // TSX
		c.X = c.SP
		c.updateNZ(c.X)
		return 2
	case 0x9A: // TXS
		c.SP = c.X
		return 2

	// --- Stack Operations ---

	case 0x48: // PHA
		c.PushByte(c.A)
		return 3
	case 0x08: // PHP
		c.PushByte(c.P | FlagB | FlagU)
		return 3
	case 0x68: // PLA
		c.A = c.PullByte()
		c.updateNZ(c.A)
		return 4
	case 0x28: // PLP
		c.P = (c.PullByte() &^ FlagB) | FlagU
		return 4

	// --- Logical ---

	// AND
	case 0x29: // AND #imm
		c.A &= c.fetchByte()
		c.updateNZ(c.A)
		return 2
	case 0x25: // AND zp
		c.A &= c.Bus.ReadByte(c.addrZP())
		c.updateNZ(c.A)
		return 3
	case 0x35: // AND zp,X
		c.A &= c.Bus.ReadByte(c.addrZPX())
		c.updateNZ(c.A)
		return 4
	case 0x2D: // AND abs
		c.A &= c.Bus.ReadByte(c.addrAbs())
		c.updateNZ(c.A)
		return 4
	case 0x3D: // AND abs,X
		addr, cross := c.addrAbsX()
		c.A &= c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 4 + cross
	case 0x39: // AND abs,Y
		addr, cross := c.addrAbsY()
		c.A &= c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 4 + cross
	case 0x21: // AND (zp,X)
		c.A &= c.Bus.ReadByte(c.addrIndX())
		c.updateNZ(c.A)
		return 6
	case 0x31: // AND (zp),Y
		addr, cross := c.addrIndY()
		c.A &= c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 5 + cross

	// EOR
	case 0x49: // EOR #imm
		c.A ^= c.fetchByte()
		c.updateNZ(c.A)
		return 2
	case 0x45: // EOR zp
		c.A ^= c.Bus.ReadByte(c.addrZP())
		c.updateNZ(c.A)
		return 3
	case 0x55: // EOR zp,X
		c.A ^= c.Bus.ReadByte(c.addrZPX())
		c.updateNZ(c.A)
		return 4
	case 0x4D: // EOR abs
		c.A ^= c.Bus.ReadByte(c.addrAbs())
		c.updateNZ(c.A)
		return 4
	case 0x5D: // EOR abs,X
		addr, cross := c.addrAbsX()
		c.A ^= c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 4 + cross
	case 0x59: // EOR abs,Y
		addr, cross := c.addrAbsY()
		c.A ^= c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 4 + cross
	case 0x41: // EOR (zp,X)
		c.A ^= c.Bus.ReadByte(c.addrIndX())
		c.updateNZ(c.A)
		return 6
	case 0x51: // EOR (zp),Y
		addr, cross := c.addrIndY()
		c.A ^= c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 5 + cross

	// ORA
	case 0x09: // ORA #imm
		c.A |= c.fetchByte()
		c.updateNZ(c.A)
		return 2
	case 0x05: // ORA zp
		c.A |= c.Bus.ReadByte(c.addrZP())
		c.updateNZ(c.A)
		return 3
	case 0x15: // ORA zp,X
		c.A |= c.Bus.ReadByte(c.addrZPX())
		c.updateNZ(c.A)
		return 4
	case 0x0D: // ORA abs
		c.A |= c.Bus.ReadByte(c.addrAbs())
		c.updateNZ(c.A)
		return 4
	case 0x1D: // ORA abs,X
		addr, cross := c.addrAbsX()
		c.A |= c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 4 + cross
	case 0x19: // ORA abs,Y
		addr, cross := c.addrAbsY()
		c.A |= c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 4 + cross
	case 0x01: // ORA (zp,X)
		c.A |= c.Bus.ReadByte(c.addrIndX())
		c.updateNZ(c.A)
		return 6
	case 0x11: // ORA (zp),Y
		addr, cross := c.addrIndY()
		c.A |= c.Bus.ReadByte(addr)
		c.updateNZ(c.A)
		return 5 + cross

	// BIT
	case 0x24: // BIT zp
		val := c.Bus.ReadByte(c.addrZP())
		c.setFlag(FlagZ, (c.A&val) == 0)
		c.setFlag(FlagN, (val&0x80) != 0)
		c.setFlag(FlagV, (val&0x40) != 0)
		return 3
	case 0x2C: // BIT abs
		val := c.Bus.ReadByte(c.addrAbs())
		c.setFlag(FlagZ, (c.A&val) == 0)
		c.setFlag(FlagN, (val&0x80) != 0)
		c.setFlag(FlagV, (val&0x40) != 0)
		return 4

	// --- Arithmetic ---

	// ADC
	case 0x69: // ADC #imm
		c.adc(c.fetchByte())
		return 2
	case 0x65: // ADC zp
		c.adc(c.Bus.ReadByte(c.addrZP()))
		return 3
	case 0x75: // ADC zp,X
		c.adc(c.Bus.ReadByte(c.addrZPX()))
		return 4
	case 0x6D: // ADC abs
		c.adc(c.Bus.ReadByte(c.addrAbs()))
		return 4
	case 0x7D: // ADC abs,X
		addr, cross := c.addrAbsX()
		c.adc(c.Bus.ReadByte(addr))
		return 4 + cross
	case 0x79: // ADC abs,Y
		addr, cross := c.addrAbsY()
		c.adc(c.Bus.ReadByte(addr))
		return 4 + cross
	case 0x61: // ADC (zp,X)
		c.adc(c.Bus.ReadByte(c.addrIndX()))
		return 6
	case 0x71: // ADC (zp),Y
		addr, cross := c.addrIndY()
		c.adc(c.Bus.ReadByte(addr))
		return 5 + cross

	// SBC
	case 0xE9: // SBC #imm
		c.sbc(c.fetchByte())
		return 2
	case 0xE5: // SBC zp
		c.sbc(c.Bus.ReadByte(c.addrZP()))
		return 3
	case 0xF5: // SBC zp,X
		c.sbc(c.Bus.ReadByte(c.addrZPX()))
		return 4
	case 0xED: // SBC abs
		c.sbc(c.Bus.ReadByte(c.addrAbs()))
		return 4
	case 0xFD: // SBC abs,X
		addr, cross := c.addrAbsX()
		c.sbc(c.Bus.ReadByte(addr))
		return 4 + cross
	case 0xF9: // SBC abs,Y
		addr, cross := c.addrAbsY()
		c.sbc(c.Bus.ReadByte(addr))
		return 4 + cross
	case 0xE1: // SBC (zp,X)
		c.sbc(c.Bus.ReadByte(c.addrIndX()))
		return 6
	case 0xF1: // SBC (zp),Y
		addr, cross := c.addrIndY()
		c.sbc(c.Bus.ReadByte(addr))
		return 5 + cross

	// CMP
	case 0xC9: // CMP #imm
		c.cmp(c.A, c.fetchByte())
		return 2
	case 0xC5: // CMP zp
		c.cmp(c.A, c.Bus.ReadByte(c.addrZP()))
		return 3
	case 0xD5: // CMP zp,X
		c.cmp(c.A, c.Bus.ReadByte(c.addrZPX()))
		return 4
	case 0xCD: // CMP abs
		c.cmp(c.A, c.Bus.ReadByte(c.addrAbs()))
		return 4
	case 0xDD: // CMP abs,X
		addr, cross := c.addrAbsX()
		c.cmp(c.A, c.Bus.ReadByte(addr))
		return 4 + cross
	case 0xD9: // CMP abs,Y
		addr, cross := c.addrAbsY()
		c.cmp(c.A, c.Bus.ReadByte(addr))
		return 4 + cross
	case 0xC1: // CMP (zp,X)
		c.cmp(c.A, c.Bus.ReadByte(c.addrIndX()))
		return 6
	case 0xD1: // CMP (zp),Y
		addr, cross := c.addrIndY()
		c.cmp(c.A, c.Bus.ReadByte(addr))
		return 5 + cross

	// CPX
	case 0xE0: // CPX #imm
		c.cmp(c.X, c.fetchByte())
		return 2
	case 0xE4: // CPX zp
		c.cmp(c.X, c.Bus.ReadByte(c.addrZP()))
		return 3
	case 0xEC: // CPX abs
		c.cmp(c.X, c.Bus.ReadByte(c.addrAbs()))
		return 4

	// CPY
	case 0xC0: // CPY #imm
		c.cmp(c.Y, c.fetchByte())
		return 2
	case 0xC4: // CPY zp
		c.cmp(c.Y, c.Bus.ReadByte(c.addrZP()))
		return 3
	case 0xCC: // CPY abs
		c.cmp(c.Y, c.Bus.ReadByte(c.addrAbs()))
		return 4

	// --- Increments / Decrements ---

	case 0xE6: // INC zp
		addr := c.addrZP()
		val := c.Bus.ReadByte(addr) + 1
		c.Bus.WriteByte(addr, val)
		c.updateNZ(val)
		return 5
	case 0xF6: // INC zp,X
		addr := c.addrZPX()
		val := c.Bus.ReadByte(addr) + 1
		c.Bus.WriteByte(addr, val)
		c.updateNZ(val)
		return 6
	case 0xEE: // INC abs
		addr := c.addrAbs()
		val := c.Bus.ReadByte(addr) + 1
		c.Bus.WriteByte(addr, val)
		c.updateNZ(val)
		return 6
	case 0xFE: // INC abs,X
		addr, _ := c.addrAbsX()
		val := c.Bus.ReadByte(addr) + 1
		c.Bus.WriteByte(addr, val)
		c.updateNZ(val)
		return 7

	case 0xE8: // INX
		c.X++
		c.updateNZ(c.X)
		return 2
	case 0xC8: // INY
		c.Y++
		c.updateNZ(c.Y)
		return 2

	case 0xC6: // DEC zp
		addr := c.addrZP()
		val := c.Bus.ReadByte(addr) - 1
		c.Bus.WriteByte(addr, val)
		c.updateNZ(val)
		return 5
	case 0xD6: // DEC zp,X
		addr := c.addrZPX()
		val := c.Bus.ReadByte(addr) - 1
		c.Bus.WriteByte(addr, val)
		c.updateNZ(val)
		return 6
	case 0xCE: // DEC abs
		addr := c.addrAbs()
		val := c.Bus.ReadByte(addr) - 1
		c.Bus.WriteByte(addr, val)
		c.updateNZ(val)
		return 6
	case 0xDE: // DEC abs,X
		addr, _ := c.addrAbsX()
		val := c.Bus.ReadByte(addr) - 1
		c.Bus.WriteByte(addr, val)
		c.updateNZ(val)
		return 7

	case 0xCA: // DEX
		c.X--
		c.updateNZ(c.X)
		return 2
	case 0x88: // DEY
		c.Y--
		c.updateNZ(c.Y)
		return 2

	// --- Shifts / Rotates ---

	// ASL
	case 0x0A: // ASL A
		c.A = c.aslByte(c.A)
		return 2
	case 0x06: // ASL zp
		addr := c.addrZP()
		c.Bus.WriteByte(addr, c.aslByte(c.Bus.ReadByte(addr)))
		return 5
	case 0x16: // ASL zp,X
		addr := c.addrZPX()
		c.Bus.WriteByte(addr, c.aslByte(c.Bus.ReadByte(addr)))
		return 6
	case 0x0E: // ASL abs
		addr := c.addrAbs()
		c.Bus.WriteByte(addr, c.aslByte(c.Bus.ReadByte(addr)))
		return 6
	case 0x1E: // ASL abs,X
		addr, _ := c.addrAbsX()
		c.Bus.WriteByte(addr, c.aslByte(c.Bus.ReadByte(addr)))
		return 7

	// LSR
	case 0x4A: // LSR A
		c.A = c.lsrByte(c.A)
		return 2
	case 0x46: // LSR zp
		addr := c.addrZP()
		c.Bus.WriteByte(addr, c.lsrByte(c.Bus.ReadByte(addr)))
		return 5
	case 0x56: // LSR zp,X
		addr := c.addrZPX()
		c.Bus.WriteByte(addr, c.lsrByte(c.Bus.ReadByte(addr)))
		return 6
	case 0x4E: // LSR abs
		addr := c.addrAbs()
		c.Bus.WriteByte(addr, c.lsrByte(c.Bus.ReadByte(addr)))
		return 6
	case 0x5E: // LSR abs,X
		addr, _ := c.addrAbsX()
		c.Bus.WriteByte(addr, c.lsrByte(c.Bus.ReadByte(addr)))
		return 7

	// ROL
	case 0x2A: // ROL A
		c.A = c.rolByte(c.A)
		return 2
	case 0x26: // ROL zp
		addr := c.addrZP()
		c.Bus.WriteByte(addr, c.rolByte(c.Bus.ReadByte(addr)))
		return 5
	case 0x36: // ROL zp,X
		addr := c.addrZPX()
		c.Bus.WriteByte(addr, c.rolByte(c.Bus.ReadByte(addr)))
		return 6
	case 0x2E: // ROL abs
		addr := c.addrAbs()
		c.Bus.WriteByte(addr, c.rolByte(c.Bus.ReadByte(addr)))
		return 6
	case 0x3E: // ROL abs,X
		addr, _ := c.addrAbsX()
		c.Bus.WriteByte(addr, c.rolByte(c.Bus.ReadByte(addr)))
		return 7

	// ROR
	case 0x6A: // ROR A
		c.A = c.rorByte(c.A)
		return 2
	case 0x66: // ROR zp
		addr := c.addrZP()
		c.Bus.WriteByte(addr, c.rorByte(c.Bus.ReadByte(addr)))
		return 5
	case 0x76: // ROR zp,X
		addr := c.addrZPX()
		c.Bus.WriteByte(addr, c.rorByte(c.Bus.ReadByte(addr)))
		return 6
	case 0x6E: // ROR abs
		addr := c.addrAbs()
		c.Bus.WriteByte(addr, c.rorByte(c.Bus.ReadByte(addr)))
		return 6
	case 0x7E: // ROR abs,X
		addr, _ := c.addrAbsX()
		c.Bus.WriteByte(addr, c.rorByte(c.Bus.ReadByte(addr)))
		return 7

	// --- Jumps & Calls ---

	case 0x4C: // JMP abs
		c.PC = c.addrAbs()
		return 3
	case 0x6C: // JMP (ind)
		ptr := c.addrAbs()
		c.PC = c.Bus.ReadWord(ptr)
		return 5
	case 0x20: // JSR abs
		target := c.addrAbs()
		c.PushWord(c.PC - 1)
		c.PC = target
		return 6
	case 0x60: // RTS
		c.PC = c.PullWord() + 1
		return 6
	case 0x40: // RTI
		// Hatvan TaskFuse: switch active task before pulling P and PC!
		if c.Bus.TaskFuseArmed {
			c.Bus.CurrentTask = c.Bus.TaskFuseTarget
			c.Bus.TaskFuseArmed = false
		}
		c.P = (c.PullByte() &^ FlagB) | FlagU
		c.PC = c.PullWord()
		return 6

	// --- Branches ---

	case 0x10: // BPL
		return c.branch((c.P & FlagN) == 0)
	case 0x30: // BMI
		return c.branch((c.P & FlagN) != 0)
	case 0x50: // BVC
		return c.branch((c.P & FlagV) == 0)
	case 0x70: // BVS
		return c.branch((c.P & FlagV) != 0)
	case 0x90: // BCC
		return c.branch((c.P & FlagC) == 0)
	case 0xB0: // BCS
		return c.branch((c.P & FlagC) != 0)
	case 0xD0: // BNE
		return c.branch((c.P & FlagZ) == 0)
	case 0xF0: // BEQ
		return c.branch((c.P & FlagZ) != 0)

	// --- Status Flag Changes ---

	case 0x18: // CLC
		c.P &^= FlagC
		return 2
	case 0x38: // SEC
		c.P |= FlagC
		return 2
	case 0x58: // CLI
		c.P &^= FlagI
		return 2
	case 0x78: // SEI
		c.P |= FlagI
		return 2
	case 0xB8: // CLV
		c.P &^= FlagV
		return 2
	case 0xD8: // CLD
		c.P &^= FlagD
		return 2
	case 0xF8: // SED
		c.P |= FlagD
		return 2

	// --- System Functions ---

	case 0x00: // BRK
		sig := c.fetchByte() // Consume signature / padding byte (PC+2 pushed)
		c.Bus.TrapSyscallNum = sig
		c.Bus.TrapSourceTask = c.Bus.CurrentTask
		c.Bus.InterruptedTask = c.Bus.CurrentTask
		c.PushWord(c.PC)
		c.PushByte(c.P | FlagB | FlagU)
		c.P |= FlagI
		c.Bus.CurrentTask = 0
		c.PC = c.Bus.ReadWord(VecIRQ)
		return 7

	case 0xEA: // NOP
		return 2

	case 0xCB: // WAI (65C02 Wait for Interrupt)
		c.Waiting = true
		return 3

	default:
		c.Panic("unimplemented 6502 opcode 0x%02X at PC=0x%04X", op, c.PC-1)
		return 2
	}
}
