package gep0

// activeSeg returns the active data segment, honoring any segment override prefix.
func (c *CPU) activeSeg(defaultSeg uint16) uint16 {
	if c.SegOverride >= 0 {
		return c.GetSeg(byte(c.SegOverride))
	}
	return defaultSeg
}

// decodeModRM fetches and decodes an 8086 ModR/M byte and any displacement bytes.
// For mod != 3, it returns the effective segment and 16-bit offset (seg:ea).
func (c *CPU) decodeModRM() (mod, reg, rm byte, seg, ea uint16) {
	modrm := c.fetchByte()
	mod = (modrm >> 6) & 3
	reg = (modrm >> 3) & 7
	rm = modrm & 7

	if mod == 3 {
		return mod, reg, rm, 0, 0
	}

	var base uint16
	defaultSeg := c.DS
	switch rm {
	case 0:
		base = c.BX + c.SI
	case 1:
		base = c.BX + c.DI
	case 2:
		base = c.BP + c.SI
		defaultSeg = c.SS
	case 3:
		base = c.BP + c.DI
		defaultSeg = c.SS
	case 4:
		base = c.SI
	case 5:
		base = c.DI
	case 6:
		if mod == 0 {
			disp16 := c.fetchWord()
			return mod, reg, rm, c.activeSeg(c.DS), disp16
		}
		base = c.BP
		defaultSeg = c.SS
	case 7:
		base = c.BX
	}

	var disp uint16
	switch mod {
	case 0:
		disp = 0
	case 1:
		disp = uint16(int16(int8(c.fetchByte())))
	case 2:
		disp = c.fetchWord()
	}

	return mod, reg, rm, c.activeSeg(defaultSeg), base + disp
}

func (c *CPU) readRM8(mod, rm byte, seg, ea uint16) byte {
	if mod == 3 {
		return c.GetReg8(rm)
	}
	return c.Bus.ReadByteSeg(seg, ea)
}

func (c *CPU) writeRM8(mod, rm byte, seg, ea uint16, val byte) {
	if mod == 3 {
		c.SetReg8(rm, val)
		return
	}
	c.Bus.WriteByteSeg(seg, ea, val)
}

func (c *CPU) readRM16(mod, rm byte, seg, ea uint16) uint16 {
	if mod == 3 {
		return c.GetReg16(rm)
	}
	return c.Bus.ReadWordSeg(seg, ea)
}

func (c *CPU) writeRM16(mod, rm byte, seg, ea uint16, val uint16) {
	if mod == 3 {
		c.SetReg16(rm, val)
		return
	}
	c.Bus.WriteWordSeg(seg, ea, val)
}

// alu8 performs an 8-bit ALU operation (0=ADD, 1=OR, 2=ADC, 3=SBB, 4=AND, 5=SUB, 6=XOR, 7=CMP).
func (c *CPU) alu8(aluOp byte, a, b byte) byte {
	switch aluOp & 7 {
	case 0: // ADD
		res16 := uint16(a) + uint16(b)
		res := byte(res16)
		c.SetFlag(FlagCF, res16 > 0xFF)
		c.SetFlag(FlagAF, ((a^b^res)&0x10) != 0)
		c.SetFlag(FlagOF, ((^(a ^ b) & (a ^ res)) & 0x80) != 0)
		c.updateFlagsSZP8(res)
		return res
	case 1: // OR
		res := a | b
		c.SetFlag(FlagCF, false)
		c.SetFlag(FlagOF, false)
		c.SetFlag(FlagAF, false)
		c.updateFlagsSZP8(res)
		return res
	case 2: // ADC
		var cf uint16
		if c.GetFlag(FlagCF) {
			cf = 1
		}
		res16 := uint16(a) + uint16(b) + cf
		res := byte(res16)
		c.SetFlag(FlagCF, res16 > 0xFF)
		c.SetFlag(FlagAF, ((a^b^res)&0x10) != 0)
		c.SetFlag(FlagOF, ((^(a ^ b) & (a ^ res)) & 0x80) != 0)
		c.updateFlagsSZP8(res)
		return res
	case 3: // SBB
		var cf uint16
		if c.GetFlag(FlagCF) {
			cf = 1
		}
		res16 := uint16(a) - uint16(b) - cf
		res := byte(res16)
		c.SetFlag(FlagCF, uint16(a) < uint16(b)+cf)
		c.SetFlag(FlagAF, ((a^b^res)&0x10) != 0)
		c.SetFlag(FlagOF, (((a ^ b) & (a ^ res)) & 0x80) != 0)
		c.updateFlagsSZP8(res)
		return res
	case 4: // AND
		res := a & b
		c.SetFlag(FlagCF, false)
		c.SetFlag(FlagOF, false)
		c.SetFlag(FlagAF, false)
		c.updateFlagsSZP8(res)
		return res
	case 5, 7: // SUB, CMP
		res16 := uint16(a) - uint16(b)
		res := byte(res16)
		c.SetFlag(FlagCF, a < b)
		c.SetFlag(FlagAF, ((a^b^res)&0x10) != 0)
		c.SetFlag(FlagOF, (((a ^ b) & (a ^ res)) & 0x80) != 0)
		c.updateFlagsSZP8(res)
		return res
	case 6: // XOR
		res := a ^ b
		c.SetFlag(FlagCF, false)
		c.SetFlag(FlagOF, false)
		c.SetFlag(FlagAF, false)
		c.updateFlagsSZP8(res)
		return res
	}
	return a
}

// alu16 performs a 16-bit ALU operation (0=ADD, 1=OR, 2=ADC, 3=SBB, 4=AND, 5=SUB, 6=XOR, 7=CMP).
func (c *CPU) alu16(aluOp byte, a, b uint16) uint16 {
	switch aluOp & 7 {
	case 0: // ADD
		res32 := uint32(a) + uint32(b)
		res := uint16(res32)
		c.SetFlag(FlagCF, res32 > 0xFFFF)
		c.SetFlag(FlagAF, ((a^b^res)&0x10) != 0)
		c.SetFlag(FlagOF, ((^(a ^ b) & (a ^ res)) & 0x8000) != 0)
		c.updateFlagsSZP16(res)
		return res
	case 1: // OR
		res := a | b
		c.SetFlag(FlagCF, false)
		c.SetFlag(FlagOF, false)
		c.SetFlag(FlagAF, false)
		c.updateFlagsSZP16(res)
		return res
	case 2: // ADC
		var cf uint32
		if c.GetFlag(FlagCF) {
			cf = 1
		}
		res32 := uint32(a) + uint32(b) + cf
		res := uint16(res32)
		c.SetFlag(FlagCF, res32 > 0xFFFF)
		c.SetFlag(FlagAF, ((a^b^res)&0x10) != 0)
		c.SetFlag(FlagOF, ((^(a ^ b) & (a ^ res)) & 0x8000) != 0)
		c.updateFlagsSZP16(res)
		return res
	case 3: // SBB
		var cf uint32
		if c.GetFlag(FlagCF) {
			cf = 1
		}
		res32 := uint32(a) - uint32(b) - cf
		res := uint16(res32)
		c.SetFlag(FlagCF, uint32(a) < uint32(b)+cf)
		c.SetFlag(FlagAF, ((a^b^res)&0x10) != 0)
		c.SetFlag(FlagOF, (((a ^ b) & (a ^ res)) & 0x8000) != 0)
		c.updateFlagsSZP16(res)
		return res
	case 4: // AND
		res := a & b
		c.SetFlag(FlagCF, false)
		c.SetFlag(FlagOF, false)
		c.SetFlag(FlagAF, false)
		c.updateFlagsSZP16(res)
		return res
	case 5, 7: // SUB, CMP
		res32 := uint32(a) - uint32(b)
		res := uint16(res32)
		c.SetFlag(FlagCF, a < b)
		c.SetFlag(FlagAF, ((a^b^res)&0x10) != 0)
		c.SetFlag(FlagOF, (((a ^ b) & (a ^ res)) & 0x8000) != 0)
		c.updateFlagsSZP16(res)
		return res
	case 6: // XOR
		res := a ^ b
		c.SetFlag(FlagCF, false)
		c.SetFlag(FlagOF, false)
		c.SetFlag(FlagAF, false)
		c.updateFlagsSZP16(res)
		return res
	}
	return a
}

// shift8 performs an 8-bit shift/rotate operation (0=ROL, 1=ROR, 2=RCL, 3=RCR, 4/6=SHL/SAL, 5=SHR, 7=SAR).
func (c *CPU) shift8(op byte, val byte, count byte) byte {
	if count == 0 {
		return val
	}
	res := val
	switch op & 7 {
	case 0: // ROL
		for i := byte(0); i < count; i++ {
			msb := (res >> 7) & 1
			res = (res << 1) | msb
			c.SetFlag(FlagCF, msb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, ((res>>7)&1 != 0) != c.GetFlag(FlagCF))
		}
	case 1: // ROR
		for i := byte(0); i < count; i++ {
			lsb := res & 1
			res = (res >> 1) | (lsb << 7)
			c.SetFlag(FlagCF, lsb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, (((res>>7)^(res>>6))&1) != 0)
		}
	case 2: // RCL
		for i := byte(0); i < count; i++ {
			var oldCF byte
			if c.GetFlag(FlagCF) {
				oldCF = 1
			}
			msb := (res >> 7) & 1
			res = (res << 1) | oldCF
			c.SetFlag(FlagCF, msb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, ((res>>7)&1 != 0) != c.GetFlag(FlagCF))
		}
	case 3: // RCR
		for i := byte(0); i < count; i++ {
			var oldCF byte
			if c.GetFlag(FlagCF) {
				oldCF = 1
			}
			lsb := res & 1
			res = (res >> 1) | (oldCF << 7)
			c.SetFlag(FlagCF, lsb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, (((res>>7)^(res>>6))&1) != 0)
		}
	case 4, 6: // SHL / SAL
		for i := byte(0); i < count; i++ {
			msb := (res >> 7) & 1
			res <<= 1
			c.SetFlag(FlagCF, msb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, ((res>>7)&1 != 0) != c.GetFlag(FlagCF))
		}
		c.updateFlagsSZP8(res)
	case 5: // SHR
		origMSB := (res & 0x80) != 0
		for i := byte(0); i < count; i++ {
			lsb := res & 1
			res >>= 1
			c.SetFlag(FlagCF, lsb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, origMSB)
		}
		c.updateFlagsSZP8(res)
	case 7: // SAR
		for i := byte(0); i < count; i++ {
			lsb := res & 1
			res = (res >> 1) | (res & 0x80)
			c.SetFlag(FlagCF, lsb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, false)
		}
		c.updateFlagsSZP8(res)
	}
	return res
}

// shift16 performs a 16-bit shift/rotate operation (0=ROL, 1=ROR, 2=RCL, 3=RCR, 4/6=SHL/SAL, 5=SHR, 7=SAR).
func (c *CPU) shift16(op byte, val uint16, count byte) uint16 {
	if count == 0 {
		return val
	}
	res := val
	switch op & 7 {
	case 0: // ROL
		for i := byte(0); i < count; i++ {
			msb := (res >> 15) & 1
			res = (res << 1) | msb
			c.SetFlag(FlagCF, msb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, ((res>>15)&1 != 0) != c.GetFlag(FlagCF))
		}
	case 1: // ROR
		for i := byte(0); i < count; i++ {
			lsb := res & 1
			res = (res >> 1) | (lsb << 15)
			c.SetFlag(FlagCF, lsb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, (((res>>15)^(res>>14))&1) != 0)
		}
	case 2: // RCL
		for i := byte(0); i < count; i++ {
			var oldCF uint16
			if c.GetFlag(FlagCF) {
				oldCF = 1
			}
			msb := (res >> 15) & 1
			res = (res << 1) | oldCF
			c.SetFlag(FlagCF, msb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, ((res>>15)&1 != 0) != c.GetFlag(FlagCF))
		}
	case 3: // RCR
		for i := byte(0); i < count; i++ {
			var oldCF uint16
			if c.GetFlag(FlagCF) {
				oldCF = 1
			}
			lsb := res & 1
			res = (res >> 1) | (oldCF << 15)
			c.SetFlag(FlagCF, lsb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, (((res>>15)^(res>>14))&1) != 0)
		}
	case 4, 6: // SHL / SAL
		for i := byte(0); i < count; i++ {
			msb := (res >> 15) & 1
			res <<= 1
			c.SetFlag(FlagCF, msb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, ((res>>15)&1 != 0) != c.GetFlag(FlagCF))
		}
		c.updateFlagsSZP16(res)
	case 5: // SHR
		origMSB := (res & 0x8000) != 0
		for i := byte(0); i < count; i++ {
			lsb := res & 1
			res >>= 1
			c.SetFlag(FlagCF, lsb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, origMSB)
		}
		c.updateFlagsSZP16(res)
	case 7: // SAR
		for i := byte(0); i < count; i++ {
			lsb := res & 1
			res = (res >> 1) | (res & 0x8000)
			c.SetFlag(FlagCF, lsb != 0)
		}
		if count == 1 {
			c.SetFlag(FlagOF, false)
		}
		c.updateFlagsSZP16(res)
	}
	return res
}

func (c *CPU) checkCond(cc byte) bool {
	switch cc & 0x0F {
	case 0x0: // JO
		return c.GetFlag(FlagOF)
	case 0x1: // JNO
		return !c.GetFlag(FlagOF)
	case 0x2: // JB / JC / JNAE
		return c.GetFlag(FlagCF)
	case 0x3: // JAE / JNC / JNB
		return !c.GetFlag(FlagCF)
	case 0x4: // JE / JZ
		return c.GetFlag(FlagZF)
	case 0x5: // JNE / JNZ
		return !c.GetFlag(FlagZF)
	case 0x6: // JBE / JNA
		return c.GetFlag(FlagCF) || c.GetFlag(FlagZF)
	case 0x7: // JA / JNBE
		return !c.GetFlag(FlagCF) && !c.GetFlag(FlagZF)
	case 0x8: // JS
		return c.GetFlag(FlagSF)
	case 0x9: // JNS
		return !c.GetFlag(FlagSF)
	case 0xA: // JP / JPE
		return c.GetFlag(FlagPF)
	case 0xB: // JNP / JPO
		return !c.GetFlag(FlagPF)
	case 0xC: // JL / JNGE
		return c.GetFlag(FlagSF) != c.GetFlag(FlagOF)
	case 0xD: // JGE / JNL
		return c.GetFlag(FlagSF) == c.GetFlag(FlagOF)
	case 0xE: // JLE / JNG
		return c.GetFlag(FlagZF) || (c.GetFlag(FlagSF) != c.GetFlag(FlagOF))
	case 0xF: // JG / JNLE
		return !c.GetFlag(FlagZF) && (c.GetFlag(FlagSF) == c.GetFlag(FlagOF))
	}
	return false
}

func (c *CPU) stepDelta(size uint16) uint16 {
	if c.GetFlag(FlagDF) {
		return ^size + 1
	}
	return size
}

func (c *CPU) execStringOp(op byte) int {
	srcSeg := c.activeSeg(c.DS)
	isCompare := (op == 0xA6 || op == 0xA7 || op == 0xAE || op == 0xAF)

	stepOnce := func() {
		switch op {
		case 0xA4: // MOVSB
			val := c.Bus.ReadByteSeg(srcSeg, c.SI)
			c.Bus.WriteByteSeg(c.ES, c.DI, val)
			d := c.stepDelta(1)
			c.SI += d
			c.DI += d
		case 0xA5: // MOVSW
			val := c.Bus.ReadWordSeg(srcSeg, c.SI)
			c.Bus.WriteWordSeg(c.ES, c.DI, val)
			d := c.stepDelta(2)
			c.SI += d
			c.DI += d
		case 0xA6: // CMPSB
			a := c.Bus.ReadByteSeg(srcSeg, c.SI)
			b := c.Bus.ReadByteSeg(c.ES, c.DI)
			c.alu8(7, a, b)
			d := c.stepDelta(1)
			c.SI += d
			c.DI += d
		case 0xA7: // CMPSW
			a := c.Bus.ReadWordSeg(srcSeg, c.SI)
			b := c.Bus.ReadWordSeg(c.ES, c.DI)
			c.alu16(7, a, b)
			d := c.stepDelta(2)
			c.SI += d
			c.DI += d
		case 0xAA: // STOSB
			c.Bus.WriteByteSeg(c.ES, c.DI, byte(c.AX))
			c.DI += c.stepDelta(1)
		case 0xAB: // STOSW
			c.Bus.WriteWordSeg(c.ES, c.DI, c.AX)
			c.DI += c.stepDelta(2)
		case 0xAC: // LODSB
			c.SetReg8(0, c.Bus.ReadByteSeg(srcSeg, c.SI))
			c.SI += c.stepDelta(1)
		case 0xAD: // LODSW
			c.AX = c.Bus.ReadWordSeg(srcSeg, c.SI)
			c.SI += c.stepDelta(2)
		case 0xAE: // SCASB
			b := c.Bus.ReadByteSeg(c.ES, c.DI)
			c.alu8(7, byte(c.AX), b)
			c.DI += c.stepDelta(1)
		case 0xAF: // SCASW
			b := c.Bus.ReadWordSeg(c.ES, c.DI)
			c.alu16(7, c.AX, b)
			c.DI += c.stepDelta(2)
		}
	}

	if c.RepPrefix == 0 {
		stepOnce()
		return 14
	}

	cycles := 9
	for c.CX != 0 {
		stepOnce()
		c.CX--
		cycles += 12
		if isCompare {
			if c.RepPrefix == 0xF3 && !c.GetFlag(FlagZF) { // REPE / REPZ
				break
			}
			if c.RepPrefix == 0xF2 && c.GetFlag(FlagZF) { // REPNE / REPNZ
				break
			}
		}
	}
	return cycles
}

func (c *CPU) executeOp(op byte) int {
	// 0x00..0x3F: Standard ALU rows (ADD, OR, ADC, SBB, AND, SUB, XOR, CMP) & Segment PUSH/POP & BCD
	if op < 0x40 {
		aluOp := (op >> 3) & 7
		sub := op & 7
		switch sub {
		case 0: // OP r/m8, r8
			mod, reg, rm, seg, ea := c.decodeModRM()
			lhs := c.readRM8(mod, rm, seg, ea)
			rhs := c.GetReg8(reg)
			res := c.alu8(aluOp, lhs, rhs)
			if aluOp != 7 {
				c.writeRM8(mod, rm, seg, ea, res)
			}
			return 10
		case 1: // OP r/m16, r16
			mod, reg, rm, seg, ea := c.decodeModRM()
			lhs := c.readRM16(mod, rm, seg, ea)
			rhs := c.GetReg16(reg)
			res := c.alu16(aluOp, lhs, rhs)
			if aluOp != 7 {
				c.writeRM16(mod, rm, seg, ea, res)
			}
			return 10
		case 2: // OP r8, r/m8
			mod, reg, rm, seg, ea := c.decodeModRM()
			lhs := c.GetReg8(reg)
			rhs := c.readRM8(mod, rm, seg, ea)
			res := c.alu8(aluOp, lhs, rhs)
			if aluOp != 7 {
				c.SetReg8(reg, res)
			}
			return 10
		case 3: // OP r16, r/m16
			mod, reg, rm, seg, ea := c.decodeModRM()
			lhs := c.GetReg16(reg)
			rhs := c.readRM16(mod, rm, seg, ea)
			res := c.alu16(aluOp, lhs, rhs)
			if aluOp != 7 {
				c.SetReg16(reg, res)
			}
			return 10
		case 4: // OP AL, imm8
			imm := c.fetchByte()
			res := c.alu8(aluOp, byte(c.AX), imm)
			if aluOp != 7 {
				c.SetReg8(0, res)
			}
			return 4
		case 5: // OP AX, imm16
			imm := c.fetchWord()
			res := c.alu16(aluOp, c.AX, imm)
			if aluOp != 7 {
				c.AX = res
			}
			return 4
		case 6:
			switch aluOp {
			case 0: // 0x06 PUSH ES
				c.push16(c.ES)
				return 10
			case 1: // 0x0E PUSH CS
				c.push16(c.CS)
				return 10
			case 2: // 0x16 PUSH SS
				c.push16(c.SS)
				return 10
			case 3: // 0x1E PUSH DS
				c.push16(c.DS)
				return 10
			}
		case 7:
			switch aluOp {
			case 0: // 0x07 POP ES
				c.ES = c.pop16()
				return 8
			case 1: // 0x0F POP CS
				c.CS = c.pop16()
				return 8
			case 2: // 0x17 POP SS
				c.SS = c.pop16()
				c.InhibitIRQ = true
				return 8
			case 3: // 0x1F POP DS
				c.DS = c.pop16()
				return 8
			case 4: // 0x27 DAA
				c.execDAA()
				return 4
			case 5: // 0x2F DAS
				c.execDAS()
				return 4
			case 6: // 0x37 AAA
				c.execAAA()
				return 4
			case 7: // 0x3F AAS
				c.execAAS()
				return 4
			}
		}
	}

	// 0x40..0x47: INC r16
	if op >= 0x40 && op <= 0x47 {
		reg := op & 7
		val := c.GetReg16(reg)
		res := val + 1
		c.SetReg16(reg, res)
		c.SetFlag(FlagAF, ((val^1^res)&0x10) != 0)
		c.SetFlag(FlagOF, val == 0x7FFF)
		c.updateFlagsSZP16(res)
		return 2
	}

	// 0x48..0x4F: DEC r16
	if op >= 0x48 && op <= 0x4F {
		reg := op & 7
		val := c.GetReg16(reg)
		res := val - 1
		c.SetReg16(reg, res)
		c.SetFlag(FlagAF, ((val^1^res)&0x10) != 0)
		c.SetFlag(FlagOF, val == 0x8000)
		c.updateFlagsSZP16(res)
		return 2
	}

	// 0x50..0x57: PUSH r16
	if op >= 0x50 && op <= 0x57 {
		reg := op & 7
		if reg == 4 { // PUSH SP on 8086 pushes the decremented SP, or current SP on 286+; support standard value
			val := c.SP
			c.push16(val)
		} else {
			c.push16(c.GetReg16(reg))
		}
		return 11
	}

	// 0x58..0x5F: POP r16
	if op >= 0x58 && op <= 0x5F {
		reg := op & 7
		c.SetReg16(reg, c.pop16())
		return 8
	}

	// 0x70..0x7F: Short conditional jumps Jcc rel8
	if op >= 0x70 && op <= 0x7F {
		rel := int16(int8(c.fetchByte()))
		if c.checkCond(op & 0x0F) {
			c.IP = uint16(int16(c.IP) + rel)
			return 16
		}
		return 4
	}

	// 0x90..0x97: NOP / XCHG AX, r16
	if op >= 0x90 && op <= 0x97 {
		reg := op & 7
		if reg != 0 {
			tmp := c.AX
			c.AX = c.GetReg16(reg)
			c.SetReg16(reg, tmp)
		}
		return 3
	}

	// 0xB0..0xB7: MOV r8, imm8
	if op >= 0xB0 && op <= 0xB7 {
		c.SetReg8(op&7, c.fetchByte())
		return 4
	}

	// 0xB8..0xBF: MOV r16, imm16
	if op >= 0xB8 && op <= 0xBF {
		c.SetReg16(op&7, c.fetchWord())
		return 4
	}

	switch op {
	case 0x60: // PUSHA (80186+ convenience)
		tempSP := c.SP
		c.push16(c.AX)
		c.push16(c.CX)
		c.push16(c.DX)
		c.push16(c.BX)
		c.push16(tempSP)
		c.push16(c.BP)
		c.push16(c.SI)
		c.push16(c.DI)
		return 36

	case 0x61: // POPA (80186+ convenience)
		c.DI = c.pop16()
		c.SI = c.pop16()
		c.BP = c.pop16()
		_ = c.pop16() // Skip SP
		c.BX = c.pop16()
		c.DX = c.pop16()
		c.CX = c.pop16()
		c.AX = c.pop16()
		return 51

	case 0x68: // PUSH imm16
		c.push16(c.fetchWord())
		return 10

	case 0x69: // IMUL r16, r/m16, imm16
		mod, reg, rm, seg, ea := c.decodeModRM()
		lhs := int32(int16(c.readRM16(mod, rm, seg, ea)))
		rhs := int32(int16(c.fetchWord()))
		prod := lhs * rhs
		res := uint16(prod)
		c.SetReg16(reg, res)
		ovf := prod < -32768 || prod > 32767
		c.SetFlag(FlagCF, ovf)
		c.SetFlag(FlagOF, ovf)
		return 25

	case 0x6A: // PUSH sign-extended imm8
		imm := uint16(int16(int8(c.fetchByte())))
		c.push16(imm)
		return 10

	case 0x6B: // IMUL r16, r/m16, sign-extended imm8
		mod, reg, rm, seg, ea := c.decodeModRM()
		lhs := int32(int16(c.readRM16(mod, rm, seg, ea)))
		rhs := int32(int8(c.fetchByte()))
		prod := lhs * rhs
		res := uint16(prod)
		c.SetReg16(reg, res)
		ovf := prod < -32768 || prod > 32767
		c.SetFlag(FlagCF, ovf)
		c.SetFlag(FlagOF, ovf)
		return 25

	case 0x80, 0x82: // ALU r/m8, imm8
		mod, reg, rm, seg, ea := c.decodeModRM()
		lhs := c.readRM8(mod, rm, seg, ea)
		imm := c.fetchByte()
		res := c.alu8(reg, lhs, imm)
		if reg != 7 {
			c.writeRM8(mod, rm, seg, ea, res)
		}
		return 10

	case 0x81: // ALU r/m16, imm16
		mod, reg, rm, seg, ea := c.decodeModRM()
		lhs := c.readRM16(mod, rm, seg, ea)
		imm := c.fetchWord()
		res := c.alu16(reg, lhs, imm)
		if reg != 7 {
			c.writeRM16(mod, rm, seg, ea, res)
		}
		return 10

	case 0x83: // ALU r/m16, sign-extended imm8
		mod, reg, rm, seg, ea := c.decodeModRM()
		lhs := c.readRM16(mod, rm, seg, ea)
		imm := uint16(int16(int8(c.fetchByte())))
		res := c.alu16(reg, lhs, imm)
		if reg != 7 {
			c.writeRM16(mod, rm, seg, ea, res)
		}
		return 10

	case 0x84: // TEST r/m8, r8
		mod, reg, rm, seg, ea := c.decodeModRM()
		lhs := c.readRM8(mod, rm, seg, ea)
		rhs := c.GetReg8(reg)
		_ = c.alu8(4, lhs, rhs)
		return 6

	case 0x85: // TEST r/m16, r16
		mod, reg, rm, seg, ea := c.decodeModRM()
		lhs := c.readRM16(mod, rm, seg, ea)
		rhs := c.GetReg16(reg)
		_ = c.alu16(4, lhs, rhs)
		return 6

	case 0x86: // XCHG r/m8, r8
		mod, reg, rm, seg, ea := c.decodeModRM()
		lhs := c.readRM8(mod, rm, seg, ea)
		rhs := c.GetReg8(reg)
		c.writeRM8(mod, rm, seg, ea, rhs)
		c.SetReg8(reg, lhs)
		return 10

	case 0x87: // XCHG r/m16, r16
		mod, reg, rm, seg, ea := c.decodeModRM()
		lhs := c.readRM16(mod, rm, seg, ea)
		rhs := c.GetReg16(reg)
		c.writeRM16(mod, rm, seg, ea, rhs)
		c.SetReg16(reg, lhs)
		return 10

	case 0x88: // MOV r/m8, r8
		mod, reg, rm, seg, ea := c.decodeModRM()
		c.writeRM8(mod, rm, seg, ea, c.GetReg8(reg))
		return 8

	case 0x89: // MOV r/m16, r16
		mod, reg, rm, seg, ea := c.decodeModRM()
		c.writeRM16(mod, rm, seg, ea, c.GetReg16(reg))
		return 8

	case 0x8A: // MOV r8, r/m8
		mod, reg, rm, seg, ea := c.decodeModRM()
		c.SetReg8(reg, c.readRM8(mod, rm, seg, ea))
		return 8

	case 0x8B: // MOV r16, r/m16
		mod, reg, rm, seg, ea := c.decodeModRM()
		c.SetReg16(reg, c.readRM16(mod, rm, seg, ea))
		return 8

	case 0x8C: // MOV r/m16, sreg
		mod, reg, rm, seg, ea := c.decodeModRM()
		c.writeRM16(mod, rm, seg, ea, c.GetSeg(reg&3))
		return 8

	case 0x8D: // LEA r16, m
		_, reg, _, _, ea := c.decodeModRM()
		c.SetReg16(reg, ea)
		return 4

	case 0x8E: // MOV sreg, r/m16
		mod, reg, rm, seg, ea := c.decodeModRM()
		c.SetSeg(reg&3, c.readRM16(mod, rm, seg, ea))
		return 8

	case 0x8F: // POP r/m16
		mod, _, rm, seg, ea := c.decodeModRM()
		val := c.pop16()
		c.writeRM16(mod, rm, seg, ea, val)
		return 12

	case 0x98: // CBW
		c.AX = uint16(int16(int8(byte(c.AX))))
		return 2

	case 0x99: // CWD
		if (c.AX & 0x8000) != 0 {
			c.DX = 0xFFFF
		} else {
			c.DX = 0x0000
		}
		return 5

	case 0x9A: // CALL ptr16:16 (far call)
		newIP := c.fetchWord()
		newCS := c.fetchWord()
		c.push16(c.CS)
		c.push16(c.IP)
		c.CS = newCS
		c.IP = newIP
		return 28

	case 0x9B: // WAIT
		return 4

	case 0x9C: // PUSHF
		c.push16(c.Flags | FlagsAlwaysOn)
		return 10

	case 0x9D: // POPF
		c.Flags = (c.pop16() & 0x0FD5) | FlagsAlwaysOn
		return 8

	case 0x9E: // SAHF
		ah := uint16(c.AX >> 8)
		c.Flags = (c.Flags & 0xFF00) | (ah & 0xD5) | FlagsAlwaysOn
		return 4

	case 0x9F: // LAHF
		fl := byte(c.Flags | FlagsAlwaysOn)
		c.SetReg8(4, fl) // AH = low byte of FLAGS
		return 4

	case 0xA0: // MOV AL, [moffs16]
		off := c.fetchWord()
		c.SetReg8(0, c.Bus.ReadByteSeg(c.activeSeg(c.DS), off))
		return 10

	case 0xA1: // MOV AX, [moffs16]
		off := c.fetchWord()
		c.AX = c.Bus.ReadWordSeg(c.activeSeg(c.DS), off)
		return 10

	case 0xA2: // MOV [moffs16], AL
		off := c.fetchWord()
		c.Bus.WriteByteSeg(c.activeSeg(c.DS), off, byte(c.AX))
		return 10

	case 0xA3: // MOV [moffs16], AX
		off := c.fetchWord()
		c.Bus.WriteWordSeg(c.activeSeg(c.DS), off, c.AX)
		return 10

	case 0xA4, 0xA5, 0xA6, 0xA7, 0xAA, 0xAB, 0xAC, 0xAD, 0xAE, 0xAF: // String instructions
		return c.execStringOp(op)

	case 0xA8: // TEST AL, imm8
		imm := c.fetchByte()
		_ = c.alu8(4, byte(c.AX), imm)
		return 4

	case 0xA9: // TEST AX, imm16
		imm := c.fetchWord()
		_ = c.alu16(4, c.AX, imm)
		return 4

	case 0xC0: // Shift/Rotate r/m8, imm8 (80186+ convenience)
		mod, reg, rm, seg, ea := c.decodeModRM()
		val := c.readRM8(mod, rm, seg, ea)
		cnt := c.fetchByte() & 0x1F
		res := c.shift8(reg, val, cnt)
		c.writeRM8(mod, rm, seg, ea, res)
		return 12

	case 0xC1: // Shift/Rotate r/m16, imm8 (80186+ convenience)
		mod, reg, rm, seg, ea := c.decodeModRM()
		val := c.readRM16(mod, rm, seg, ea)
		cnt := c.fetchByte() & 0x1F
		res := c.shift16(reg, val, cnt)
		c.writeRM16(mod, rm, seg, ea, res)
		return 12

	case 0xC2: // RET imm16
		popBytes := c.fetchWord()
		c.IP = c.pop16()
		c.SP += popBytes
		return 12

	case 0xC3: // RET
		c.IP = c.pop16()
		return 8

	case 0xC4: // LES r16, m16:16
		_, reg, _, seg, ea := c.decodeModRM()
		off := c.Bus.ReadWordSeg(seg, ea)
		newES := c.Bus.ReadWordSeg(seg, ea+2)
		c.SetReg16(reg, off)
		c.ES = newES
		return 16

	case 0xC5: // LDS r16, m16:16
		_, reg, _, seg, ea := c.decodeModRM()
		off := c.Bus.ReadWordSeg(seg, ea)
		newDS := c.Bus.ReadWordSeg(seg, ea+2)
		c.SetReg16(reg, off)
		c.DS = newDS
		return 16

	case 0xC6: // MOV r/m8, imm8
		mod, _, rm, seg, ea := c.decodeModRM()
		imm := c.fetchByte()
		c.writeRM8(mod, rm, seg, ea, imm)
		return 10

	case 0xC7: // MOV r/m16, imm16
		mod, _, rm, seg, ea := c.decodeModRM()
		imm := c.fetchWord()
		c.writeRM16(mod, rm, seg, ea, imm)
		return 10

	case 0xC8: // ENTER imm16, imm8
		allocSize := c.fetchWord()
		level := c.fetchByte() & 0x1F
		c.push16(c.BP)
		frameTemp := c.SP
		if level > 0 {
			for i := byte(1); i < level; i++ {
				c.BP -= 2
				c.push16(c.Bus.ReadWordSeg(c.SS, c.BP))
			}
			c.push16(frameTemp)
		}
		c.BP = frameTemp
		c.SP -= allocSize
		return 15

	case 0xC9: // LEAVE
		c.SP = c.BP
		c.BP = c.pop16()
		return 8

	case 0xCA: // RETF imm16
		popBytes := c.fetchWord()
		c.IP = c.pop16()
		c.CS = c.pop16()
		c.SP += popBytes
		return 18

	case 0xCB: // RETF
		c.IP = c.pop16()
		c.CS = c.pop16()
		return 18

	case 0xCC: // INT 3
		c.triggerSoftwareInterrupt(3)
		return 50

	case 0xCD: // INT imm8
		vec := c.fetchByte()
		c.triggerSoftwareInterrupt(vec)
		return 50

	case 0xCE: // INTO
		if c.GetFlag(FlagOF) {
			c.triggerSoftwareInterrupt(4)
			return 50
		}
		return 4

	case 0xCF: // IRET
		if c.Bus.TaskFuseArmed {
			c.Bus.CurrentTask = c.Bus.TaskFuseTarget
			c.Bus.TaskFuseArmed = false
		}
		c.IP = c.pop16()
		c.CS = c.pop16()
		c.Flags = (c.pop16() & 0x0FD5) | FlagsAlwaysOn
		return 24

	case 0xD0: // Shift/Rotate r/m8, 1
		mod, reg, rm, seg, ea := c.decodeModRM()
		val := c.readRM8(mod, rm, seg, ea)
		res := c.shift8(reg, val, 1)
		c.writeRM8(mod, rm, seg, ea, res)
		return 8

	case 0xD1: // Shift/Rotate r/m16, 1
		mod, reg, rm, seg, ea := c.decodeModRM()
		val := c.readRM16(mod, rm, seg, ea)
		res := c.shift16(reg, val, 1)
		c.writeRM16(mod, rm, seg, ea, res)
		return 8

	case 0xD2: // Shift/Rotate r/m8, CL
		mod, reg, rm, seg, ea := c.decodeModRM()
		val := c.readRM8(mod, rm, seg, ea)
		res := c.shift8(reg, val, byte(c.CX))
		c.writeRM8(mod, rm, seg, ea, res)
		return 12

	case 0xD3: // Shift/Rotate r/m16, CL
		mod, reg, rm, seg, ea := c.decodeModRM()
		val := c.readRM16(mod, rm, seg, ea)
		res := c.shift16(reg, val, byte(c.CX))
		c.writeRM16(mod, rm, seg, ea, res)
		return 12

	case 0xD4: // AAM imm8
		base := c.fetchByte()
		if base == 0 {
			c.triggerSoftwareInterrupt(0)
			return 50
		}
		al := byte(c.AX)
		ah := al / base
		al = al % base
		c.AX = (uint16(ah) << 8) | uint16(al)
		c.updateFlagsSZP8(al)
		return 16

	case 0xD5: // AAD imm8
		base := c.fetchByte()
		al := byte(c.AX)
		ah := byte(c.AX >> 8)
		res := al + (ah * base)
		c.AX = uint16(res)
		c.updateFlagsSZP8(res)
		return 14

	case 0xD6: // SALC
		if c.GetFlag(FlagCF) {
			c.SetReg8(0, 0xFF)
		} else {
			c.SetReg8(0, 0x00)
		}
		return 4

	case 0xD7: // XLATB
		al := byte(c.AX)
		val := c.Bus.ReadByteSeg(c.activeSeg(c.DS), c.BX+uint16(al))
		c.SetReg8(0, val)
		return 11

	case 0xE0: // LOOPNE / LOOPNZ rel8
		rel := int16(int8(c.fetchByte()))
		c.CX--
		if c.CX != 0 && !c.GetFlag(FlagZF) {
			c.IP = uint16(int16(c.IP) + rel)
			return 19
		}
		return 5

	case 0xE1: // LOOPE / LOOPZ rel8
		rel := int16(int8(c.fetchByte()))
		c.CX--
		if c.CX != 0 && c.GetFlag(FlagZF) {
			c.IP = uint16(int16(c.IP) + rel)
			return 18
		}
		return 6

	case 0xE2: // LOOP rel8
		rel := int16(int8(c.fetchByte()))
		c.CX--
		if c.CX != 0 {
			c.IP = uint16(int16(c.IP) + rel)
			return 17
		}
		return 5

	case 0xE3: // JCXZ rel8
		rel := int16(int8(c.fetchByte()))
		if c.CX == 0 {
			c.IP = uint16(int16(c.IP) + rel)
			return 18
		}
		return 6

	case 0xE4: // IN AL, imm8
		port := uint16(c.fetchByte())
		c.SetReg8(0, c.Bus.ReadByte20(0x0FF00|(uint32(port)&0xFF)))
		return 10

	case 0xE5: // IN AX, imm8
		port := uint16(c.fetchByte())
		lo := uint16(c.Bus.ReadByte20(0x0FF00 | (uint32(port) & 0xFF)))
		hi := uint16(c.Bus.ReadByte20(0x0FF00 | (uint32(port+1) & 0xFF)))
		c.AX = (hi << 8) | lo
		return 10

	case 0xE6: // OUT imm8, AL
		port := uint16(c.fetchByte())
		c.Bus.WriteByte20(0x0FF00|(uint32(port)&0xFF), byte(c.AX))
		return 10

	case 0xE7: // OUT imm8, AX
		port := uint16(c.fetchByte())
		c.Bus.WriteByte20(0x0FF00|(uint32(port)&0xFF), byte(c.AX))
		c.Bus.WriteByte20(0x0FF00|(uint32(port+1)&0xFF), byte(c.AX>>8))
		return 10

	case 0xE8: // CALL rel16
		rel := int16(c.fetchWord())
		c.push16(c.IP)
		c.IP = uint16(int16(c.IP) + rel)
		return 19

	case 0xE9: // JMP rel16
		rel := int16(c.fetchWord())
		c.IP = uint16(int16(c.IP) + rel)
		return 15

	case 0xEA: // JMP ptr16:16 (far jump)
		newIP := c.fetchWord()
		newCS := c.fetchWord()
		c.IP = newIP
		c.CS = newCS
		return 15

	case 0xEB: // JMP rel8
		rel := int16(int8(c.fetchByte()))
		c.IP = uint16(int16(c.IP) + rel)
		return 15

	case 0xEC: // IN AL, DX
		c.SetReg8(0, c.Bus.ReadByte20(0x0FF00|(uint32(c.DX)&0xFF)))
		return 8

	case 0xED: // IN AX, DX
		lo := uint16(c.Bus.ReadByte20(0x0FF00 | (uint32(c.DX) & 0xFF)))
		hi := uint16(c.Bus.ReadByte20(0x0FF00 | (uint32(c.DX+1) & 0xFF)))
		c.AX = (hi << 8) | lo
		return 8

	case 0xEE: // OUT DX, AL
		c.Bus.WriteByte20(0x0FF00|(uint32(c.DX)&0xFF), byte(c.AX))
		return 8

	case 0xEF: // OUT DX, AX
		c.Bus.WriteByte20(0x0FF00|(uint32(c.DX)&0xFF), byte(c.AX))
		c.Bus.WriteByte20(0x0FF00|(uint32(c.DX+1)&0xFF), byte(c.AX>>8))
		return 8

	case 0xF4: // HLT
		if c.GetFlag(FlagIF) {
			c.Waiting = true
		} else {
			c.Halted = true
		}
		return 2

	case 0xF5: // CMC
		c.SetFlag(FlagCF, !c.GetFlag(FlagCF))
		return 2

	case 0xF6: // Group 3 (r/m8: TEST, NOT, NEG, MUL, IMUL, DIV, IDIV)
		mod, reg, rm, seg, ea := c.decodeModRM()
		val := c.readRM8(mod, rm, seg, ea)
		switch reg {
		case 0, 1: // TEST r/m8, imm8
			imm := c.fetchByte()
			_ = c.alu8(4, val, imm)
			return 6
		case 2: // NOT r/m8
			c.writeRM8(mod, rm, seg, ea, ^val)
			return 6
		case 3: // NEG r/m8
			res := byte(0 - val)
			c.writeRM8(mod, rm, seg, ea, res)
			c.SetFlag(FlagCF, val != 0)
			c.SetFlag(FlagAF, (val&0x0F) != 0)
			c.SetFlag(FlagOF, val == 0x80)
			c.updateFlagsSZP8(res)
			return 6
		case 4: // MUL r/m8 (unsigned: AX = AL * r/m8)
			prod := uint16(byte(c.AX)) * uint16(val)
			c.AX = prod
			ovf := (prod & 0xFF00) != 0
			c.SetFlag(FlagCF, ovf)
			c.SetFlag(FlagOF, ovf)
			return 70
		case 5: // IMUL r/m8 (signed: AX = int8(AL) * int8(r/m8))
			prod := int16(int8(byte(c.AX))) * int16(int8(val))
			c.AX = uint16(prod)
			ovf := prod < -128 || prod > 127
			c.SetFlag(FlagCF, ovf)
			c.SetFlag(FlagOF, ovf)
			return 80
		case 6: // DIV r/m8 (unsigned: AL = AX / val, AH = AX % val)
			if val == 0 {
				c.triggerSoftwareInterrupt(0)
				return 50
			}
			q := c.AX / uint16(val)
			r := c.AX % uint16(val)
			if q > 0xFF {
				c.triggerSoftwareInterrupt(0)
				return 50
			}
			c.AX = (r << 8) | (q & 0xFF)
			return 80
		case 7: // IDIV r/m8 (signed: AL = int16(AX) / int8(val), AH = int16(AX) % int8(val))
			if val == 0 {
				c.triggerSoftwareInterrupt(0)
				return 50
			}
			dividend := int16(c.AX)
			divisor := int16(int8(val))
			q := dividend / divisor
			r := dividend % divisor
			if q < -128 || q > 127 {
				c.triggerSoftwareInterrupt(0)
				return 50
			}
			c.AX = (uint16(byte(r)) << 8) | uint16(byte(q))
			return 100
		}

	case 0xF7: // Group 3 (r/m16: TEST, NOT, NEG, MUL, IMUL, DIV, IDIV)
		mod, reg, rm, seg, ea := c.decodeModRM()
		val := c.readRM16(mod, rm, seg, ea)
		switch reg {
		case 0, 1: // TEST r/m16, imm16
			imm := c.fetchWord()
			_ = c.alu16(4, val, imm)
			return 6
		case 2: // NOT r/m16
			c.writeRM16(mod, rm, seg, ea, ^val)
			return 6
		case 3: // NEG r/m16
			res := uint16(0 - val)
			c.writeRM16(mod, rm, seg, ea, res)
			c.SetFlag(FlagCF, val != 0)
			c.SetFlag(FlagAF, (val&0x0F) != 0)
			c.SetFlag(FlagOF, val == 0x8000)
			c.updateFlagsSZP16(res)
			return 6
		case 4: // MUL r/m16 (unsigned: DX:AX = AX * r/m16)
			prod := uint32(c.AX) * uint32(val)
			c.AX = uint16(prod)
			c.DX = uint16(prod >> 16)
			ovf := c.DX != 0
			c.SetFlag(FlagCF, ovf)
			c.SetFlag(FlagOF, ovf)
			return 118
		case 5: // IMUL r/m16 (signed: DX:AX = int16(AX) * int16(r/m16))
			prod := int32(int16(c.AX)) * int32(int16(val))
			c.AX = uint16(prod)
			c.DX = uint16(uint32(prod) >> 16)
			ovf := prod < -32768 || prod > 32767
			c.SetFlag(FlagCF, ovf)
			c.SetFlag(FlagOF, ovf)
			return 128
		case 6: // DIV r/m16 (unsigned: AX = DX:AX / val, DX = DX:AX % val)
			if val == 0 {
				c.triggerSoftwareInterrupt(0)
				return 50
			}
			dividend := (uint32(c.DX) << 16) | uint32(c.AX)
			q := dividend / uint32(val)
			r := dividend % uint32(val)
			if q > 0xFFFF {
				c.triggerSoftwareInterrupt(0)
				return 50
			}
			c.AX = uint16(q)
			c.DX = uint16(r)
			return 144
		case 7: // IDIV r/m16 (signed: AX = int32(DX:AX) / int16(val), DX = int32(DX:AX) % int16(val))
			if val == 0 {
				c.triggerSoftwareInterrupt(0)
				return 50
			}
			dividend := int32((uint32(c.DX) << 16) | uint32(c.AX))
			divisor := int32(int16(val))
			if dividend == -2147483648 && divisor == -1 {
				c.triggerSoftwareInterrupt(0)
				return 50
			}
			q := dividend / divisor
			r := dividend % divisor
			if q < -32768 || q > 32767 {
				c.triggerSoftwareInterrupt(0)
				return 50
			}
			c.AX = uint16(int16(q))
			c.DX = uint16(int16(r))
			return 165
		}

	case 0xF8: // CLC
		c.SetFlag(FlagCF, false)
		return 2
	case 0xF9: // STC
		c.SetFlag(FlagCF, true)
		return 2
	case 0xFA: // CLI
		c.SetFlag(FlagIF, false)
		return 2
	case 0xFB: // STI
		c.SetFlag(FlagIF, true)
		return 2
	case 0xFC: // CLD
		c.SetFlag(FlagDF, false)
		return 2
	case 0xFD: // STD
		c.SetFlag(FlagDF, true)
		return 2

	case 0xFE: // Group 4 (INC/DEC r/m8)
		mod, reg, rm, seg, ea := c.decodeModRM()
		val := c.readRM8(mod, rm, seg, ea)
		switch reg {
		case 0: // INC r/m8
			res := val + 1
			c.writeRM8(mod, rm, seg, ea, res)
			c.SetFlag(FlagAF, ((val^1^res)&0x10) != 0)
			c.SetFlag(FlagOF, val == 0x7F)
			c.updateFlagsSZP8(res)
			return 10
		case 1: // DEC r/m8
			res := val - 1
			c.writeRM8(mod, rm, seg, ea, res)
			c.SetFlag(FlagAF, ((val^1^res)&0x10) != 0)
			c.SetFlag(FlagOF, val == 0x80)
			c.updateFlagsSZP8(res)
			return 10
		default:
			c.Panic("unimplemented 8086 Group 4 opcode 0xFE /%d at %04X:%04X", reg, c.Bus.LastCS, c.Bus.LastIP)
		}

	case 0xFF: // Group 5 (INC/DEC/CALL/JMP/PUSH r/m16)
		mod, reg, rm, seg, ea := c.decodeModRM()
		switch reg {
		case 0: // INC r/m16
			val := c.readRM16(mod, rm, seg, ea)
			res := val + 1
			c.writeRM16(mod, rm, seg, ea, res)
			c.SetFlag(FlagAF, ((val^1^res)&0x10) != 0)
			c.SetFlag(FlagOF, val == 0x7FFF)
			c.updateFlagsSZP16(res)
			return 10
		case 1: // DEC r/m16
			val := c.readRM16(mod, rm, seg, ea)
			res := val - 1
			c.writeRM16(mod, rm, seg, ea, res)
			c.SetFlag(FlagAF, ((val^1^res)&0x10) != 0)
			c.SetFlag(FlagOF, val == 0x8000)
			c.updateFlagsSZP16(res)
			return 10
		case 2: // CALL r/m16 (near indirect)
			target := c.readRM16(mod, rm, seg, ea)
			c.push16(c.IP)
			c.IP = target
			return 16
		case 3: // CALL m16:16 (far indirect)
			newIP := c.Bus.ReadWordSeg(seg, ea)
			newCS := c.Bus.ReadWordSeg(seg, ea+2)
			c.push16(c.CS)
			c.push16(c.IP)
			c.CS = newCS
			c.IP = newIP
			return 24
		case 4: // JMP r/m16 (near indirect)
			c.IP = c.readRM16(mod, rm, seg, ea)
			return 11
		case 5: // JMP m16:16 (far indirect)
			newIP := c.Bus.ReadWordSeg(seg, ea)
			newCS := c.Bus.ReadWordSeg(seg, ea+2)
			c.IP = newIP
			c.CS = newCS
			return 18
		case 6: // PUSH r/m16
			val := c.readRM16(mod, rm, seg, ea)
			c.push16(val)
			return 12
		default:
			c.Panic("unimplemented 8086 Group 5 opcode 0xFF /%d at %04X:%04X", reg, c.Bus.LastCS, c.Bus.LastIP)
		}
	}

	c.Panic("unimplemented 8086 opcode 0x%02X at %04X:%04X", op, c.Bus.LastCS, c.Bus.LastIP)
	return 0
}

func (c *CPU) execDAA() {
	al := byte(c.AX)
	oldAL := al
	oldCF := c.GetFlag(FlagCF)
	c.SetFlag(FlagCF, false)
	if (al&0x0F) > 9 || c.GetFlag(FlagAF) {
		al += 6
		c.SetFlag(FlagCF, oldCF || (al < oldAL))
		c.SetFlag(FlagAF, true)
	} else {
		c.SetFlag(FlagAF, false)
	}
	if oldAL > 0x99 || oldCF {
		al += 0x60
		c.SetFlag(FlagCF, true)
	}
	c.SetReg8(0, al)
	c.updateFlagsSZP8(al)
}

func (c *CPU) execDAS() {
	al := byte(c.AX)
	oldAL := al
	oldCF := c.GetFlag(FlagCF)
	c.SetFlag(FlagCF, false)
	if (al&0x0F) > 9 || c.GetFlag(FlagAF) {
		c.SetFlag(FlagCF, oldCF || (al < 6))
		al -= 6
		c.SetFlag(FlagAF, true)
	} else {
		c.SetFlag(FlagAF, false)
	}
	if oldAL > 0x99 || oldCF {
		al -= 0x60
		c.SetFlag(FlagCF, true)
	}
	c.SetReg8(0, al)
	c.updateFlagsSZP8(al)
}

func (c *CPU) execAAA() {
	al := byte(c.AX)
	ah := byte(c.AX >> 8)
	if (al&0x0F) > 9 || c.GetFlag(FlagAF) {
		al = (al + 6) & 0x0F
		ah++
		c.SetFlag(FlagAF, true)
		c.SetFlag(FlagCF, true)
	} else {
		al &= 0x0F
		c.SetFlag(FlagAF, false)
		c.SetFlag(FlagCF, false)
	}
	c.AX = (uint16(ah) << 8) | uint16(al)
}

func (c *CPU) execAAS() {
	al := byte(c.AX)
	ah := byte(c.AX >> 8)
	if (al&0x0F) > 9 || c.GetFlag(FlagAF) {
		al = (al - 6) & 0x0F
		ah--
		c.SetFlag(FlagAF, true)
		c.SetFlag(FlagCF, true)
	} else {
		al &= 0x0F
		c.SetFlag(FlagAF, false)
		c.SetFlag(FlagCF, false)
	}
	c.AX = (uint16(ah) << 8) | uint16(al)
}
