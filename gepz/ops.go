package gepz

func (c *CPU) readReg8(r uint8) byte {
	switch r {
	case 0:
		return c.B
	case 1:
		return c.C
	case 2:
		return c.D
	case 3:
		return c.E
	case 4:
		return c.H
	case 5:
		return c.L
	case 6:
		return c.Bus.ReadByte(c.GetHL())
	case 7:
		return c.A
	}
	return 0
}

func (c *CPU) writeReg8(r uint8, val byte) {
	switch r {
	case 0:
		c.B = val
	case 1:
		c.C = val
	case 2:
		c.D = val
	case 3:
		c.E = val
	case 4:
		c.H = val
	case 5:
		c.L = val
	case 6:
		c.Bus.WriteByte(c.GetHL(), val)
	case 7:
		c.A = val
	}
}

func (c *CPU) checkCond(cond uint8) bool {
	switch cond {
	case 0: // NZ
		return (c.F & FlagZ) == 0
	case 1: // Z
		return (c.F & FlagZ) != 0
	case 2: // NC
		return (c.F & FlagC) == 0
	case 3: // C
		return (c.F & FlagC) != 0
	case 4: // PO (Parity Odd / Overflow Clear)
		return (c.F & FlagV) == 0
	case 5: // PE (Parity Even / Overflow Set)
		return (c.F & FlagV) != 0
	case 6: // P (Sign Positive)
		return (c.F & FlagS) == 0
	case 7: // M (Sign Negative)
		return (c.F & FlagS) != 0
	}
	return false
}

func (c *CPU) executeOp(op byte) int {
	// Base opcodes
	switch op {
	case 0x00: // NOP
		return 4
	case 0x76: // HALT
		c.Halted = true
		c.PC-- // Keep PC at HALT until interrupt
		return 4
	case 0xF3: // DI
		c.IFF1 = false
		c.IFF2 = false
		return 4
	case 0xFB: // EI
		c.eiDelay = true
		return 4
	case 0x08: // EX AF, AF'
		c.A, c.A_ = c.A_, c.A
		c.F, c.F_ = c.F_, c.F
		return 4
	case 0xD9: // EXX
		c.B, c.B_ = c.B_, c.B
		c.C, c.C_ = c.C_, c.C
		c.D, c.D_ = c.D_, c.D
		c.E, c.E_ = c.E_, c.E
		c.H, c.H_ = c.H_, c.H
		c.L, c.L_ = c.L_, c.L
		return 4
	case 0xEB: // EX DE, HL
		c.D, c.H = c.H, c.D
		c.E, c.L = c.L, c.E
		return 4
	case 0xE3: // EX (SP), HL
		lo := c.Bus.ReadByte(c.SP)
		hi := c.Bus.ReadByte(c.SP + 1)
		c.Bus.WriteByte(c.SP, c.L)
		c.Bus.WriteByte(c.SP+1, c.H)
		c.L = lo
		c.H = hi
		return 19
	case 0xF9: // LD SP, HL
		c.SP = c.GetHL()
		return 6
	case 0x27: // DAA
		c.A, c.F = Daa(c.A, c.F)
		return 4
	case 0x2F: // CPL
		c.A = ^c.A
		c.F |= (FlagH | FlagN)
		c.F &^= (FlagY | FlagX)
		c.F |= (c.A & (FlagY | FlagX))
		return 4
	case 0x37: // SCF
		c.F |= FlagC
		c.F &^= (FlagH | FlagN)
		c.F &^= (FlagY | FlagX)
		c.F |= (c.A & (FlagY | FlagX))
		return 4
	case 0x3F: // CCF
		var hBit byte
		if (c.F & FlagC) != 0 {
			hBit = FlagH
			c.F &^= FlagC
		} else {
			c.F |= FlagC
		}
		c.F &^= FlagN
		c.F &^= FlagH
		c.F |= hBit
		c.F &^= (FlagY | FlagX)
		c.F |= (c.A & (FlagY | FlagX))
		return 4

	// Relative Jumps
	case 0x18: // JR e
		disp := int8(c.fetchByte())
		c.PC = uint16(int32(c.PC) + int32(disp))
		return 12
	case 0x10: // DJNZ e
		disp := int8(c.fetchByte())
		c.B--
		if c.B != 0 {
			c.PC = uint16(int32(c.PC) + int32(disp))
			return 13
		}
		return 8
	case 0x20, 0x28, 0x30, 0x38: // JR cc, e
		cond := (op >> 3) & 3
		disp := int8(c.fetchByte())
		if c.checkCond(cond) {
			c.PC = uint16(int32(c.PC) + int32(disp))
			return 12
		}
		return 7

	// 16-bit Immediate Loads
	case 0x01: // LD BC, nn
		c.SetBC(c.fetchWord())
		return 10
	case 0x11: // LD DE, nn
		c.SetDE(c.fetchWord())
		return 10
	case 0x21: // LD HL, nn
		c.SetHL(c.fetchWord())
		return 10
	case 0x31: // LD SP, nn
		c.SP = c.fetchWord()
		return 10

	// 16-bit ADD HL, rr
	case 0x09: // ADD HL, BC
		c.SetHL(c.add16HL(c.GetBC()))
		return 11
	case 0x19: // ADD HL, DE
		c.SetHL(c.add16HL(c.GetDE()))
		return 11
	case 0x29: // ADD HL, HL
		c.SetHL(c.add16HL(c.GetHL()))
		return 11
	case 0x39: // ADD HL, SP
		c.SetHL(c.add16HL(c.SP))
		return 11

	// Indirect Loads
	case 0x02: // LD (BC), A
		c.Bus.WriteByte(c.GetBC(), c.A)
		return 7
	case 0x12: // LD (DE), A
		c.Bus.WriteByte(c.GetDE(), c.A)
		return 7
	case 0x22: // LD (nn), HL
		c.Bus.WriteWord(c.fetchWord(), c.GetHL())
		return 16
	case 0x32: // LD (nn), A
		c.Bus.WriteByte(c.fetchWord(), c.A)
		return 13
	case 0x0A: // LD A, (BC)
		c.A = c.Bus.ReadByte(c.GetBC())
		return 7
	case 0x1A: // LD A, (DE)
		c.A = c.Bus.ReadByte(c.GetDE())
		return 7
	case 0x2A: // LD HL, (nn)
		c.SetHL(c.Bus.ReadWord(c.fetchWord()))
		return 16
	case 0x3A: // LD A, (nn)
		c.A = c.Bus.ReadByte(c.fetchWord())
		return 13

	// 16-bit INC / DEC
	case 0x03:
		c.SetBC(c.GetBC() + 1)
		return 6
	case 0x13:
		c.SetDE(c.GetDE() + 1)
		return 6
	case 0x23:
		c.SetHL(c.GetHL() + 1)
		return 6
	case 0x33:
		c.SP++
		return 6
	case 0x0B:
		c.SetBC(c.GetBC() - 1)
		return 6
	case 0x1B:
		c.SetDE(c.GetDE() - 1)
		return 6
	case 0x2B:
		c.SetHL(c.GetHL() - 1)
		return 6
	case 0x3B:
		c.SP--
		return 6

	// 8-bit INC
	case 0x04, 0x0C, 0x14, 0x1C, 0x24, 0x2C, 0x3C:
		r := (op >> 3) & 7
		res, f := Inc8(c.readReg8(r), c.F)
		c.writeReg8(r, res)
		c.F = f
		return 4
	case 0x34: // INC (HL)
		res, f := Inc8(c.Bus.ReadByte(c.GetHL()), c.F)
		c.Bus.WriteByte(c.GetHL(), res)
		c.F = f
		return 11

	// 8-bit DEC
	case 0x05, 0x0D, 0x15, 0x1D, 0x25, 0x2D, 0x3D:
		r := (op >> 3) & 7
		res, f := Dec8(c.readReg8(r), c.F)
		c.writeReg8(r, res)
		c.F = f
		return 4
	case 0x35: // DEC (HL)
		res, f := Dec8(c.Bus.ReadByte(c.GetHL()), c.F)
		c.Bus.WriteByte(c.GetHL(), res)
		c.F = f
		return 11

	// 8-bit Immediate Loads
	case 0x06, 0x0E, 0x16, 0x1E, 0x26, 0x2E, 0x3E:
		r := (op >> 3) & 7
		c.writeReg8(r, c.fetchByte())
		return 7
	case 0x36: // LD (HL), n
		c.Bus.WriteByte(c.GetHL(), c.fetchByte())
		return 10

	// Accumulator Rotates
	case 0x07: // RLCA
		c.A, c.F = c.rlca()
		return 4
	case 0x0F: // RRCA
		c.A, c.F = c.rrca()
		return 4
	case 0x17: // RLA
		c.A, c.F = c.rla()
		return 4
	case 0x1F: // RRA
		c.A, c.F = c.rra()
		return 4

	// Absolute Jumps, Calls, Returns
	case 0xC3: // JP nn
		c.PC = c.fetchWord()
		return 10
	case 0xE9: // JP (HL)
		c.PC = c.GetHL()
		return 4
	case 0xC2, 0xCA, 0xD2, 0xDA, 0xE2, 0xEA, 0xF2, 0xFA: // JP cc, nn
		target := c.fetchWord()
		if c.checkCond((op >> 3) & 7) {
			c.PC = target
		}
		return 10
	case 0xCD: // CALL nn
		target := c.fetchWord()
		c.PushWord(c.PC)
		c.PC = target
		return 17
	case 0xC4, 0xCC, 0xD4, 0xDC, 0xE4, 0xEC, 0xF4, 0xFC: // CALL cc, nn
		target := c.fetchWord()
		if c.checkCond((op >> 3) & 7) {
			c.PushWord(c.PC)
			c.PC = target
			return 17
		}
		return 10
	case 0xC9: // RET
		c.PC = c.PullWord()
		return 10
	case 0xC0, 0xC8, 0xD0, 0xD8, 0xE0, 0xE8, 0xF0, 0xF8: // RET cc
		if c.checkCond((op >> 3) & 7) {
			c.PC = c.PullWord()
			return 11
		}
		return 5

	// PUSH and POP
	case 0xC5: // PUSH BC
		c.PushWord(c.GetBC())
		return 11
	case 0xD5: // PUSH DE
		c.PushWord(c.GetDE())
		return 11
	case 0xE5: // PUSH HL
		c.PushWord(c.GetHL())
		return 11
	case 0xF5: // PUSH AF
		c.PushWord(c.GetAF())
		return 11
	case 0xC1: // POP BC
		c.SetBC(c.PullWord())
		return 10
	case 0xD1: // POP DE
		c.SetDE(c.PullWord())
		return 10
	case 0xE1: // POP HL
		c.SetHL(c.PullWord())
		return 10
	case 0xF1: // POP AF
		c.SetAF(c.PullWord())
		return 10

	// RST
	case 0xC7, 0xCF, 0xD7, 0xDF, 0xE7, 0xEF, 0xF7, 0xFF:
		target := uint16(op & 0x38)
		c.PushWord(c.PC)
		c.PC = target
		return 11

	// I/O Ports
	case 0xD3: // OUT (n), A
		port := c.fetchByte()
		c.Bus.WritePort(uint16(port), c.A)
		return 11
	case 0xDB: // IN A, (n)
		port := c.fetchByte()
		c.A = c.Bus.ReadPort(uint16(port))
		return 11

	// 8-bit Immediate ALU
	case 0xC6: // ADD A, n
		c.A, c.F = Add8(c.A, c.fetchByte(), false, c.F)
		return 7
	case 0xCE: // ADC A, n
		c.A, c.F = Add8(c.A, c.fetchByte(), true, c.F)
		return 7
	case 0xD6: // SUB n
		c.A, c.F = Sub8(c.A, c.fetchByte(), false, c.F)
		return 7
	case 0xDE: // SBC A, n
		c.A, c.F = Sub8(c.A, c.fetchByte(), true, c.F)
		return 7
	case 0xE6: // AND n
		c.A, c.F = And8(c.A, c.fetchByte())
		return 7
	case 0xEE: // XOR n
		c.A, c.F = Xor8(c.A, c.fetchByte())
		return 7
	case 0xF6: // OR n
		c.A, c.F = Or8(c.A, c.fetchByte())
		return 7
	case 0xFE: // CP n
		c.F = Cp8(c.A, c.fetchByte())
		return 7

	// Prefixes
	case 0xCB:
		return c.executeCB()
	case 0xED:
		return c.executeED()
	case 0xDD:
		return c.executeDD_FD(&c.IX, 0xDD)
	case 0xFD:
		return c.executeDD_FD(&c.IY, 0xFD)
	}

	// 0x40..0x7F: LD r, r'
	if op >= 0x40 && op <= 0x7F {
		dst := (op >> 3) & 7
		src := op & 7
		c.writeReg8(dst, c.readReg8(src))
		if dst == 6 || src == 6 {
			return 7
		}
		return 4
	}

	// 0x80..0xBF: 8-bit ALU register operations
	if op >= 0x80 && op <= 0xBF {
		aluOp := (op >> 3) & 7
		val := c.readReg8(op & 7)
		cycles := 4
		if (op & 7) == 6 {
			cycles = 7
		}

		switch aluOp {
		case 0: // ADD
			c.A, c.F = Add8(c.A, val, false, c.F)
		case 1: // ADC
			c.A, c.F = Add8(c.A, val, true, c.F)
		case 2: // SUB
			c.A, c.F = Sub8(c.A, val, false, c.F)
		case 3: // SBC
			c.A, c.F = Sub8(c.A, val, true, c.F)
		case 4: // AND
			c.A, c.F = And8(c.A, val)
		case 5: // XOR
			c.A, c.F = Xor8(c.A, val)
		case 6: // OR
			c.A, c.F = Or8(c.A, val)
		case 7: // CP
			c.F = Cp8(c.A, val)
		}
		return cycles
	}

	c.Panic("unimplemented Z80 opcode 0x%02X at PC=0x%04X", op, c.PC-1)
	return 4
}

func (c *CPU) add16HL(val uint16) uint16 {
	res, f := Add16(c.GetHL(), val, c.F)
	c.F = f
	return res
}

func (c *CPU) rlca() (byte, byte) {
	cf := (c.A >> 7) & 1
	res := (c.A << 1) | cf
	f := (c.F & (FlagS | FlagZ | FlagV)) | cf
	f |= (res & (FlagY | FlagX))
	return res, f
}

func (c *CPU) rrca() (byte, byte) {
	cf := c.A & 1
	res := (c.A >> 1) | (cf << 7)
	f := (c.F & (FlagS | FlagZ | FlagV)) | cf
	f |= (res & (FlagY | FlagX))
	return res, f
}

func (c *CPU) rla() (byte, byte) {
	cf := (c.A >> 7) & 1
	oldC := c.F & FlagC
	res := (c.A << 1) | oldC
	f := (c.F & (FlagS | FlagZ | FlagV)) | cf
	f |= (res & (FlagY | FlagX))
	return res, f
}

func (c *CPU) rra() (byte, byte) {
	cf := c.A & 1
	oldC := (c.F & FlagC) << 7
	res := (c.A >> 1) | oldC
	f := (c.F & (FlagS | FlagZ | FlagV)) | cf
	f |= (res & (FlagY | FlagX))
	return res, f
}

func (c *CPU) executeCB() int {
	cbOp := c.fetchByte()
	r := cbOp & 7
	val := c.readReg8(r)
	cycles := 8
	if r == 6 {
		cycles = 15
	}

	// 0x00..0x3F: Rotates & Shifts
	if cbOp < 0x40 {
		var res byte
		switch (cbOp >> 3) & 7 {
		case 0:
			res, c.F = Rlc(val)
		case 1:
			res, c.F = Rrc(val)
		case 2:
			res, c.F = Rl(val, c.F)
		case 3:
			res, c.F = Rr(val, c.F)
		case 4:
			res, c.F = Sla(val)
		case 5:
			res, c.F = Sra(val)
		case 6: // SLL (undocumented)
			cOut := (val >> 7) & 1
			res = (val << 1) | 1
			c.F = 0
			if cOut != 0 {
				c.F |= FlagC
			}
			if (res & 0x80) != 0 {
				c.F |= FlagS
			}
			if res == 0 {
				c.F |= FlagZ
			}
			c.F |= (res & (FlagY | FlagX)) | parityTable[res]
		case 7:
			res, c.F = Srl(val)
		}
		c.writeReg8(r, res)
		return cycles
	}

	bit := (cbOp >> 3) & 7

	// 0x40..0x7F: BIT b, r
	if cbOp < 0x80 {
		c.F &^= (FlagS | FlagZ | FlagH | FlagV | FlagN)
		c.F |= FlagH // BIT always sets H
		if (val & (1 << bit)) == 0 {
			c.F |= (FlagZ | FlagV)
		}
		if bit == 7 && (val&0x80) != 0 {
			c.F |= FlagS
		}
		c.F |= (val & (FlagY | FlagX))
		if r == 6 {
			return 12
		}
		return 8
	}

	// 0x80..0xBF: RES b, r
	if cbOp < 0xC0 {
		val &^= (1 << bit)
		c.writeReg8(r, val)
		return cycles
	}

	// 0xC0..0xFF: SET b, r
	val |= (1 << bit)
	c.writeReg8(r, val)
	return cycles
}

func (c *CPU) executeED() int {
	edOp := c.fetchByte()

	switch edOp {
	// Block transfers
	case 0xA0: // LDI
		return c.executeLDI(false)
	case 0xB0: // LDIR
		return c.executeLDI(true)
	case 0xA8: // LDD
		return c.executeLDD(false)
	case 0xB8: // LDDR
		return c.executeLDD(true)
	case 0xA1: // CPI
		return c.executeCPI(false)
	case 0xB1: // CPIR
		return c.executeCPI(true)
	case 0xA9: // CPD
		return c.executeCPD(false)
	case 0xB9: // CPDR
		return c.executeCPD(true)

	// Interrupt / Returns
	case 0x4D: // RETI
		c.PC = c.PullWord()
		return 14
	case 0x45: // RETN
		// Hatvan TaskFuse: switch task before pulling PC!
		if c.Bus.TaskFuseArmed {
			c.Bus.CurrentTask = c.Bus.TaskFuseTarget
			c.Bus.TaskFuseArmed = false
		}
		c.PC = c.PullWord()
		c.IFF1 = c.IFF2
		return 14

	case 0x46: // IM 0
		c.IM = 0
		return 8
	case 0x56: // IM 1
		c.IM = 1
		return 8
	case 0x5E: // IM 2
		c.IM = 2
		return 8

	case 0x44: // NEG
		c.A, c.F = Sub8(0, c.A, false, 0)
		return 8

	case 0x47: // LD I, A
		c.I = c.A
		return 9
	case 0x4F: // LD R, A
		c.R = c.A
		return 9
	case 0x57: // LD A, I
		c.A = c.I
		c.F &^= (FlagS | FlagZ | FlagH | FlagV | FlagN)
		if (c.A & 0x80) != 0 {
			c.F |= FlagS
		}
		if c.A == 0 {
			c.F |= FlagZ
		}
		if c.IFF2 {
			c.F |= FlagV
		}
		c.F |= (c.A & (FlagY | FlagX))
		return 9
	case 0x5F: // LD A, R
		c.A = c.R
		c.F &^= (FlagS | FlagZ | FlagH | FlagV | FlagN)
		if (c.A & 0x80) != 0 {
			c.F |= FlagS
		}
		if c.A == 0 {
			c.F |= FlagZ
		}
		if c.IFF2 {
			c.F |= FlagV
		}
		c.F |= (c.A & (FlagY | FlagX))
		return 9

	// 16-bit ADC / SBC HL, rr
	case 0x4A: // ADC HL, BC
		c.SetHL(c.adc16HL(c.GetBC()))
		return 15
	case 0x5A: // ADC HL, DE
		c.SetHL(c.adc16HL(c.GetDE()))
		return 15
	case 0x6A: // ADC HL, HL
		c.SetHL(c.adc16HL(c.GetHL()))
		return 15
	case 0x7A: // ADC HL, SP
		c.SetHL(c.adc16HL(c.SP))
		return 15

	case 0x42: // SBC HL, BC
		c.SetHL(c.sbc16HL(c.GetBC()))
		return 15
	case 0x52: // SBC HL, DE
		c.SetHL(c.sbc16HL(c.GetDE()))
		return 15
	case 0x62: // SBC HL, HL
		c.SetHL(c.sbc16HL(c.GetHL()))
		return 15
	case 0x72: // SBC HL, SP
		c.SetHL(c.sbc16HL(c.SP))
		return 15

	// 16-bit Memory Loads
	case 0x43: // LD (nn), BC
		c.Bus.WriteWord(c.fetchWord(), c.GetBC())
		return 20
	case 0x53: // LD (nn), DE
		c.Bus.WriteWord(c.fetchWord(), c.GetDE())
		return 20
	case 0x63: // LD (nn), HL
		c.Bus.WriteWord(c.fetchWord(), c.GetHL())
		return 20
	case 0x73: // LD (nn), SP
		c.Bus.WriteWord(c.fetchWord(), c.SP)
		return 20

	case 0x4B: // LD BC, (nn)
		c.SetBC(c.Bus.ReadWord(c.fetchWord()))
		return 20
	case 0x5B: // LD DE, (nn)
		c.SetDE(c.Bus.ReadWord(c.fetchWord()))
		return 20
	case 0x6B: // LD HL, (nn)
		c.SetHL(c.Bus.ReadWord(c.fetchWord()))
		return 20
	case 0x7B: // LD SP, (nn)
		c.SP = c.Bus.ReadWord(c.fetchWord())
		return 20

	// I/O with C register
	case 0x78: // IN A, (C)
		c.A = c.Bus.ReadPort(uint16(c.C))
		return 12
	case 0x40, 0x48, 0x50, 0x58, 0x60, 0x68: // IN r, (C)
		r := (edOp >> 3) & 7
		val := c.Bus.ReadPort(uint16(c.C))
		c.writeReg8(r, val)
		return 12
	case 0x79: // OUT (C), A
		c.Bus.WritePort(uint16(c.C), c.A)
		return 12
	case 0x41, 0x49, 0x51, 0x59, 0x61, 0x69: // OUT (C), r
		r := (edOp >> 3) & 7
		c.Bus.WritePort(uint16(c.C), c.readReg8(r))
		return 12
	}

	c.Panic("unimplemented Z80 ED opcode 0x%02X at PC=0x%04X", edOp, c.PC-2)
	return 4
}

func (c *CPU) adc16HL(val uint16) uint16 {
	res, f := Adc16(c.GetHL(), val, c.F)
	c.F = f
	return res
}

func (c *CPU) sbc16HL(val uint16) uint16 {
	res, f := Sbc16(c.GetHL(), val, c.F)
	c.F = f
	return res
}

func (c *CPU) executeLDI(repeat bool) int {
	b := c.Bus.ReadByte(c.GetHL())
	c.Bus.WriteByte(c.GetDE(), b)
	c.SetHL(c.GetHL() + 1)
	c.SetDE(c.GetDE() + 1)
	bc := c.GetBC() - 1
	c.SetBC(bc)

	c.F &^= (FlagH | FlagN | FlagV)
	if bc != 0 {
		c.F |= FlagV
	}
	if repeat && bc != 0 {
		c.PC -= 2 // Repeat LDIR
		return 21
	}
	return 16
}

func (c *CPU) executeLDD(repeat bool) int {
	b := c.Bus.ReadByte(c.GetHL())
	c.Bus.WriteByte(c.GetDE(), b)
	c.SetHL(c.GetHL() - 1)
	c.SetDE(c.GetDE() - 1)
	bc := c.GetBC() - 1
	c.SetBC(bc)

	c.F &^= (FlagH | FlagN | FlagV)
	if bc != 0 {
		c.F |= FlagV
	}
	if repeat && bc != 0 {
		c.PC -= 2 // Repeat LDDR
		return 21
	}
	return 16
}

func (c *CPU) executeCPI(repeat bool) int {
	memVal := c.Bus.ReadByte(c.GetHL())
	res, f := Sub8(c.A, memVal, false, 0)
	c.SetHL(c.GetHL() + 1)
	bc := c.GetBC() - 1
	c.SetBC(bc)

	c.F = (c.F & FlagC) | (f & (FlagS | FlagZ | FlagH | FlagN))
	if bc != 0 {
		c.F |= FlagV
	}
	if repeat && bc != 0 && res != 0 {
		c.PC -= 2
		return 21
	}
	return 16
}

func (c *CPU) executeCPD(repeat bool) int {
	memVal := c.Bus.ReadByte(c.GetHL())
	res, f := Sub8(c.A, memVal, false, 0)
	c.SetHL(c.GetHL() - 1)
	bc := c.GetBC() - 1
	c.SetBC(bc)

	c.F = (c.F & FlagC) | (f & (FlagS | FlagZ | FlagH | FlagN))
	if bc != 0 {
		c.F |= FlagV
	}
	if repeat && bc != 0 && res != 0 {
		c.PC -= 2
		return 21
	}
	return 16
}

func (c *CPU) executeDD_FD(reg16 *uint16, prefix byte) int {
	op := c.fetchByte()

	switch op {
	case 0x21: // LD IX/IY, nn
		*reg16 = c.fetchWord()
		return 14
	case 0x22: // LD (nn), IX/IY
		c.Bus.WriteWord(c.fetchWord(), *reg16)
		return 20
	case 0x2A: // LD IX/IY, (nn)
		*reg16 = c.Bus.ReadWord(c.fetchWord())
		return 20
	case 0xF9: // LD SP, IX/IY
		c.SP = *reg16
		return 10
	case 0x23: // INC IX/IY
		*reg16++
		return 10
	case 0x2B: // DEC IX/IY
		*reg16--
		return 10
	case 0xE5: // PUSH IX/IY
		c.PushWord(*reg16)
		return 15
	case 0xE1: // POP IX/IY
		*reg16 = c.PullWord()
		return 14
	case 0xE9: // JP (IX/IY)
		c.PC = *reg16
		return 8
	case 0xE3: // EX (SP), IX/IY
		lo := c.Bus.ReadByte(c.SP)
		hi := c.Bus.ReadByte(c.SP + 1)
		c.Bus.WriteByte(c.SP, byte(*reg16&0xFF))
		c.Bus.WriteByte(c.SP+1, byte(*reg16>>8))
		*reg16 = (uint16(hi) << 8) | uint16(lo)
		return 23

	// 16-bit ADD IX/IY, rr
	case 0x09: // ADD IX/IY, BC
		*reg16, c.F = Add16(*reg16, c.GetBC(), c.F)
		return 15
	case 0x19: // ADD IX/IY, DE
		*reg16, c.F = Add16(*reg16, c.GetDE(), c.F)
		return 15
	case 0x29: // ADD IX/IY, IX/IY
		*reg16, c.F = Add16(*reg16, *reg16, c.F)
		return 15
	case 0x39: // ADD IX/IY, SP
		*reg16, c.F = Add16(*reg16, c.SP, c.F)
		return 15

	// Indexed Displacement (IX+d) / (IY+d)
	case 0x34: // INC (IX+d)
		d := int8(c.fetchByte())
		addr := uint16(int32(*reg16) + int32(d))
		res, f := Inc8(c.Bus.ReadByte(addr), c.F)
		c.Bus.WriteByte(addr, res)
		c.F = f
		return 23
	case 0x35: // DEC (IX+d)
		d := int8(c.fetchByte())
		addr := uint16(int32(*reg16) + int32(d))
		res, f := Dec8(c.Bus.ReadByte(addr), c.F)
		c.Bus.WriteByte(addr, res)
		c.F = f
		return 23
	case 0x36: // LD (IX+d), n
		d := int8(c.fetchByte())
		n := c.fetchByte()
		addr := uint16(int32(*reg16) + int32(d))
		c.Bus.WriteByte(addr, n)
		return 19

	case 0xCB: // Indexed Bit instructions: DD CB d op
		d := int8(c.fetchByte())
		addr := uint16(int32(*reg16) + int32(d))
		cbOp := c.fetchByte()
		val := c.Bus.ReadByte(addr)
		bit := (cbOp >> 3) & 7

		if cbOp < 0x40 {
			var res byte
			switch bit {
			case 0:
				res, c.F = Rlc(val)
			case 1:
				res, c.F = Rrc(val)
			case 2:
				res, c.F = Rl(val, c.F)
			case 3:
				res, c.F = Rr(val, c.F)
			case 4:
				res, c.F = Sla(val)
			case 5:
				res, c.F = Sra(val)
			case 7:
				res, c.F = Srl(val)
			}
			c.Bus.WriteByte(addr, res)
			return 23
		}
		if cbOp < 0x80 { // BIT b, (IX+d)
			c.F &^= (FlagS | FlagZ | FlagH | FlagV | FlagN)
			c.F |= FlagH
			if (val & (1 << bit)) == 0 {
				c.F |= (FlagZ | FlagV)
			}
			if bit == 7 && (val&0x80) != 0 {
				c.F |= FlagS
			}
			return 20
		}
		if cbOp < 0xC0 { // RES b, (IX+d)
			val &^= (1 << bit)
			c.Bus.WriteByte(addr, val)
			return 23
		}
		// SET b, (IX+d)
		val |= (1 << bit)
		c.Bus.WriteByte(addr, val)
		return 23
	}

	// LD (IX+d), r
	if op >= 0x70 && op <= 0x77 && op != 0x76 {
		d := int8(c.fetchByte())
		addr := uint16(int32(*reg16) + int32(d))
		c.Bus.WriteByte(addr, c.readReg8(op&7))
		return 19
	}

	// LD r, (IX+d)
	if (op & 0xC7) == 0x46 {
		d := int8(c.fetchByte())
		addr := uint16(int32(*reg16) + int32(d))
		r := (op >> 3) & 7
		c.writeReg8(r, c.Bus.ReadByte(addr))
		return 19
	}

	// ALU A, (IX+d)
	if op >= 0x80 && op <= 0xBF && (op&7) == 6 {
		d := int8(c.fetchByte())
		addr := uint16(int32(*reg16) + int32(d))
		val := c.Bus.ReadByte(addr)
		aluOp := (op >> 3) & 7

		switch aluOp {
		case 0:
			c.A, c.F = Add8(c.A, val, false, c.F)
		case 1:
			c.A, c.F = Add8(c.A, val, true, c.F)
		case 2:
			c.A, c.F = Sub8(c.A, val, false, c.F)
		case 3:
			c.A, c.F = Sub8(c.A, val, true, c.F)
		case 4:
			c.A, c.F = And8(c.A, val)
		case 5:
			c.A, c.F = Xor8(c.A, val)
		case 6:
			c.A, c.F = Or8(c.A, val)
		case 7:
			c.F = Cp8(c.A, val)
		}
		return 19
	}

	c.Panic("unimplemented Z80 prefix 0x%02X opcode 0x%02X at PC=0x%04X", prefix, op, c.PC-2)
	return 4
}
