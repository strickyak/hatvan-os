package gepc

func (c *CPU) fetchByte() byte {
	val := c.Bus.ReadByte(c.R[c.P])
	c.R[c.P]++
	return val
}

func (c *CPU) executeOp(op byte) {
	hiNibble := op >> 4
	loNibble := op & 0x0F

	switch hiNibble {
	case 0x0:
		if loNibble == 0 { // 0x00: IDL
			c.Idle = true
			c.Cycles += 16
		} else { // 0x01..0x0F: LDN Rn
			c.D = c.Bus.ReadByte(c.R[loNibble])
			c.Cycles += 16
		}

	case 0x1: // 0x10..0x1F: INC Rn
		c.R[loNibble]++
		c.Cycles += 16

	case 0x2: // 0x20..0x2F: DEC Rn
		c.R[loNibble]--
		c.Cycles += 16

	case 0x3: // 0x30..0x3F: Short branches and short skip
		if op == 0x38 { // 0x38: SKP / NBR (unconditional skip 1 byte)
			c.R[c.P]++
			c.Cycles += 16
			return
		}
		targetLo := c.fetchByte()
		take := false
		switch op {
		case 0x30: // BR
			take = true
		case 0x31: // BQ
			take = (c.Q == 1)
		case 0x32: // BZ
			take = (c.D == 0)
		case 0x33: // BDF / BPZ / BGE
			take = (c.DF == 1)
		case 0x34: // B1
			take = c.Bus.EF1()
		case 0x35: // B2
			take = c.Bus.EF2()
		case 0x36: // B3
			take = c.Bus.EF3()
		case 0x37: // B4
			take = c.Bus.EF4()
		case 0x39: // BNQ
			take = (c.Q == 0)
		case 0x3A: // BNZ
			take = (c.D != 0)
		case 0x3B: // BNF / BM / BL
			take = (c.DF == 0)
		case 0x3C: // BN1
			take = !c.Bus.EF1()
		case 0x3D: // BN2
			take = !c.Bus.EF2()
		case 0x3E: // BN3
			take = !c.Bus.EF3()
		case 0x3F: // BN4
			take = !c.Bus.EF4()
		}
		if take {
			c.R[c.P] = (c.R[c.P] & 0xFF00) | uint16(targetLo)
		}
		c.Cycles += 16

	case 0x4: // 0x40..0x4F: LDA Rn
		c.D = c.Bus.ReadByte(c.R[loNibble])
		c.R[loNibble]++
		c.Cycles += 16

	case 0x5: // 0x50..0x5F: STR Rn
		c.Bus.WriteByte(c.R[loNibble], c.D)
		c.Cycles += 16

	case 0x6: // 0x60..0x6F: I/O and IRX
		if op == 0x60 { // IRX
			c.R[c.X]++
			c.Cycles += 16
		} else if op >= 0x61 && op <= 0x67 { // OUT 1..7
			port := op & 0x07
			val := c.Bus.ReadByte(c.R[c.X])
			c.R[c.X]++
			c.Bus.WritePort(port, val)
			c.Cycles += 16
		} else if op == 0x68 { // Unused opcode
			c.Cycles += 16
		} else { // 0x69..0x6F: INP 1..7
			port := op & 0x07
			val := c.Bus.ReadPort(port)
			c.Bus.WriteByte(c.R[c.X], val)
			c.D = val
			c.Cycles += 16
		}

	case 0x7: // 0x70..0x7F: Control, Stack, and ALU operations
		switch op {
		case 0x70: // RET
			if c.Bus.TaskFuseArmed {
				c.Bus.CurrentTask = c.Bus.TaskFuseTarget
				c.Bus.TaskFuseArmed = false
			}
			b := c.Bus.ReadByte(c.R[c.X])
			c.P = b & 0x0F
			c.X = b >> 4
			c.R[c.X]++
			c.IE = 1
			c.Cycles += 16

		case 0x71: // DIS
			if c.Bus.TaskFuseArmed {
				c.Bus.CurrentTask = c.Bus.TaskFuseTarget
				c.Bus.TaskFuseArmed = false
			}
			b := c.Bus.ReadByte(c.R[c.X])
			c.P = b & 0x0F
			c.X = b >> 4
			c.R[c.X]++
			c.IE = 0
			c.Cycles += 16

		case 0x72: // LDXA
			c.D = c.Bus.ReadByte(c.R[c.X])
			c.R[c.X]++
			c.Cycles += 16

		case 0x73: // STXD
			c.Bus.WriteByte(c.R[c.X], c.D)
			c.R[c.X]--
			c.Cycles += 16

		case 0x74: // ADC
			m := c.Bus.ReadByte(c.R[c.X])
			sum := int(c.D) + int(m) + int(c.DF)
			c.D = byte(sum)
			if sum >= 256 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16

		case 0x75: // SDB (M - D - (1 - DF))
			m := c.Bus.ReadByte(c.R[c.X])
			diff := int(m) - int(c.D) - int(1-c.DF)
			c.D = byte(diff)
			if diff >= 0 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16

		case 0x76: // SHRC / RSHR (shift right through carry)
			oldDF := c.DF
			c.DF = c.D & 1
			c.D = (c.D >> 1) | (oldDF << 7)
			c.Cycles += 16

		case 0x77: // SMB (D - M - (1 - DF))
			m := c.Bus.ReadByte(c.R[c.X])
			diff := int(c.D) - int(m) - int(1-c.DF)
			c.D = byte(diff)
			if diff >= 0 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16

		case 0x78: // SAV
			c.Bus.WriteByte(c.R[c.X], c.T)
			c.Cycles += 16

		case 0x79: // MARK
			c.T = (c.X << 4) | (c.P & 0x0F)
			c.Bus.WriteByte(c.R[2], c.T)
			c.X = c.P
			c.R[2]--
			c.Cycles += 16

		case 0x7A: // REQ
			c.Q = 0
			c.Cycles += 16

		case 0x7B: // SEQ
			c.Q = 1
			c.Cycles += 16

		case 0x7C: // ADCI
			imm := c.fetchByte()
			sum := int(c.D) + int(imm) + int(c.DF)
			c.D = byte(sum)
			if sum >= 256 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16

		case 0x7D: // SDBI (imm - D - (1 - DF))
			imm := c.fetchByte()
			diff := int(imm) - int(c.D) - int(1-c.DF)
			c.D = byte(diff)
			if diff >= 0 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16

		case 0x7E: // SHLC / RSHL (shift left through carry)
			oldDF := c.DF
			c.DF = (c.D >> 7) & 1
			c.D = (c.D << 1) | oldDF
			c.Cycles += 16

		case 0x7F: // SMBI (D - imm - (1 - DF))
			imm := c.fetchByte()
			diff := int(c.D) - int(imm) - int(1-c.DF)
			c.D = byte(diff)
			if diff >= 0 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16
		}

	case 0x8: // 0x80..0x8F: GLO Rn
		c.D = byte(c.R[loNibble] & 0xFF)
		c.Cycles += 16

	case 0x9: // 0x90..0x9F: GHI Rn
		c.D = byte(c.R[loNibble] >> 8)
		c.Cycles += 16

	case 0xA: // 0xA0..0xAF: PLO Rn
		c.R[loNibble] = (c.R[loNibble] & 0xFF00) | uint16(c.D)
		c.Cycles += 16

	case 0xB: // 0xB0..0xBF: PHI Rn
		c.R[loNibble] = (c.R[loNibble] & 0x00FF) | (uint16(c.D) << 8)
		c.Cycles += 16

	case 0xC: // 0xC0..0xCF: Long branches, Long skips, NOP
		switch op {
		case 0xC0, 0xC1, 0xC2, 0xC3, 0xC9, 0xCA, 0xCB: // Long branches (3 cycles, 3 bytes)
			hi := c.fetchByte()
			lo := c.fetchByte()
			target := (uint16(hi) << 8) | uint16(lo)
			take := false
			switch op {
			case 0xC0: // LBR
				take = true
			case 0xC1: // LBQ
				take = (c.Q == 1)
			case 0xC2: // LBZ
				take = (c.D == 0)
			case 0xC3: // LBDF
				take = (c.DF == 1)
			case 0xC9: // LBNQ
				take = (c.Q == 0)
			case 0xCA: // LBNZ
				take = (c.D != 0)
			case 0xCB: // LBNF
				take = (c.DF == 0)
			}
			if take {
				c.R[c.P] = target
			}
			c.Cycles += 24

		case 0xC4: // NOP (3 cycles, 1 byte)
			c.Cycles += 24

		case 0xC5: // LSNQ (skip 2 bytes if Q == 0)
			if c.Q == 0 {
				c.R[c.P] += 2
			}
			c.Cycles += 24

		case 0xC6: // LSNZ (skip 2 bytes if D != 0)
			if c.D != 0 {
				c.R[c.P] += 2
			}
			c.Cycles += 24

		case 0xC7: // LSNF (skip 2 bytes if DF == 0)
			if c.DF == 0 {
				c.R[c.P] += 2
			}
			c.Cycles += 24

		case 0xC8: // LSKP / NLBR (unconditionally skip 2 bytes)
			c.R[c.P] += 2
			c.Cycles += 24

		case 0xCC: // LSIE (skip 2 bytes if IE == 1)
			if c.IE == 1 {
				c.R[c.P] += 2
			}
			c.Cycles += 24

		case 0xCD: // LSQ (skip 2 bytes if Q == 1)
			if c.Q == 1 {
				c.R[c.P] += 2
			}
			c.Cycles += 24

		case 0xCE: // LSZ (skip 2 bytes if D == 0)
			if c.D == 0 {
				c.R[c.P] += 2
			}
			c.Cycles += 24

		case 0xCF: // LSDF (skip 2 bytes if DF == 1)
			if c.DF == 1 {
				c.R[c.P] += 2
			}
			c.Cycles += 24
		}

	case 0xD: // 0xD0..0xDF: SEP Rn
		c.P = loNibble
		if c.Bus.TaskFuseArmed {
			c.Bus.CurrentTask = c.Bus.TaskFuseTarget
			c.Bus.TaskFuseArmed = false
		}
		c.Cycles += 16

	case 0xE: // 0xE0..0xEF: SEX Rn
		c.X = loNibble
		c.Cycles += 16

	case 0xF: // 0xF0..0xFF: ALU operations
		switch op {
		case 0xF0: // LDX
			c.D = c.Bus.ReadByte(c.R[c.X])
			c.Cycles += 16

		case 0xF1: // OR
			c.D |= c.Bus.ReadByte(c.R[c.X])
			c.Cycles += 16

		case 0xF2: // AND
			c.D &= c.Bus.ReadByte(c.R[c.X])
			c.Cycles += 16

		case 0xF3: // XOR
			c.D ^= c.Bus.ReadByte(c.R[c.X])
			c.Cycles += 16

		case 0xF4: // ADD
			m := c.Bus.ReadByte(c.R[c.X])
			sum := int(c.D) + int(m)
			c.D = byte(sum)
			if sum >= 256 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16

		case 0xF5: // SD (M - D)
			m := c.Bus.ReadByte(c.R[c.X])
			diff := int(m) - int(c.D)
			c.D = byte(diff)
			if diff >= 0 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16

		case 0xF6: // SHR
			c.DF = c.D & 1
			c.D >>= 1
			c.Cycles += 16

		case 0xF7: // SM (D - M)
			m := c.Bus.ReadByte(c.R[c.X])
			diff := int(c.D) - int(m)
			c.D = byte(diff)
			if diff >= 0 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16

		case 0xF8: // LDI
			c.D = c.fetchByte()
			c.Cycles += 16

		case 0xF9: // ORI
			c.D |= c.fetchByte()
			c.Cycles += 16

		case 0xFA: // ANI
			c.D &= c.fetchByte()
			c.Cycles += 16

		case 0xFB: // XRI
			c.D ^= c.fetchByte()
			c.Cycles += 16

		case 0xFC: // ADI
			imm := c.fetchByte()
			sum := int(c.D) + int(imm)
			c.D = byte(sum)
			if sum >= 256 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16

		case 0xFD: // SDI (imm - D)
			imm := c.fetchByte()
			diff := int(imm) - int(c.D)
			c.D = byte(diff)
			if diff >= 0 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16

		case 0xFE: // SHL
			c.DF = (c.D >> 7) & 1
			c.D <<= 1
			c.Cycles += 16

		case 0xFF: // SMI (D - imm)
			imm := c.fetchByte()
			diff := int(c.D) - int(imm)
			c.D = byte(diff)
			if diff >= 0 {
				c.DF = 1
			} else {
				c.DF = 0
			}
			c.Cycles += 16
		}
	}
}
