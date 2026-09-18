package vmk

func (c *CPU) dispatch(op uint16) {
	group := (op >> 12) & 0x0F

	switch group {
	case 0x0:
		c.dispatchGroup0(op)
	case 0x1:
		c.opMove(op, SizeByte)
	case 0x2:
		c.opMove(op, SizeLong)
	case 0x3:
		c.opMove(op, SizeWord)
	case 0x4:
		c.dispatchGroup4(op)
	case 0x5:
		c.dispatchGroup5(op)
	case 0x6:
		c.opBcc(op)
	case 0x7:
		if (op & 0x0100) == 0 {
			c.opMoveQ(op)
		} else {
			c.TriggerException(VecIllegalInstr)
		}
	case 0x8:
		c.dispatchGroup8(op)
	case 0x9:
		c.dispatchGroup9(op)
	case 0xA:
		c.TriggerException(VecLine1010)
	case 0xB:
		c.dispatchGroupB(op)
	case 0xC:
		c.dispatchGroupC(op)
	case 0xD:
		c.dispatchGroupD(op)
	case 0xE:
		if (op & 0x00C0) == 0x00C0 {
			c.opShiftMem(op)
		} else {
			c.opShiftReg(op)
		}
	case 0xF:
		c.TriggerException(VecLine1111)
	}
}

func (c *CPU) dispatchGroup0(op uint16) {
	// Special immediate instructions
	switch op {
	case 0x003C: // ORI to CCR
		val := c.FetchInstructionWord()
		c.SetCCR(c.CCR() | byte(val))
		return
	case 0x007C: // ORI to SR
		if (c.SR & FlagS) == 0 {
			c.TriggerException(VecPrivilegeViol)
			return
		}
		val := c.FetchInstructionWord()
		c.SetSR(c.SR | val)
		return
	case 0x023C: // ANDI to CCR
		val := c.FetchInstructionWord()
		c.SetCCR(c.CCR() & byte(val))
		return
	case 0x027C: // ANDI to SR
		if (c.SR & FlagS) == 0 {
			c.TriggerException(VecPrivilegeViol)
			return
		}
		val := c.FetchInstructionWord()
		c.SetSR(c.SR & val)
		return
	case 0x0A3C: // EORI to CCR
		val := c.FetchInstructionWord()
		c.SetCCR(c.CCR() ^ byte(val))
		return
	case 0x0A7C: // EORI to SR
		if (c.SR & FlagS) == 0 {
			c.TriggerException(VecPrivilegeViol)
			return
		}
		val := c.FetchInstructionWord()
		c.SetSR(c.SR ^ val)
		return
	}

	// MOVEP: 0000 RRR 1x0 001 rrr
	if (op & 0x0138) == 0x0108 {
		c.opMoveP(op)
		return
	}

	// Static bit instructions: 0000 1000 xx MMM rrr
	if (op & 0xFF00) == 0x0800 {
		c.opBit(op, true)
		return
	}

	// Dynamic bit instructions: 0000 RRR 1xx MMM rrr
	if (op & 0x0100) != 0 && (op&0x0038) != 0x0008 {
		c.opBit(op, false)
		return
	}

	// Immediate ALU instructions (ORI, ANDI, SUBI, ADDI, EORI, CMPI)
	typeBits := (op >> 9) & 7
	sizeBits := (op >> 6) & 3
	size, ok := decodeStandardSize(sizeBits)
	if !ok {
		c.TriggerException(VecIllegalInstr)
		return
	}

	mode := uint8((op >> 3) & 7)
	reg := uint8(op & 7)

	var imm uint32
	switch size {
	case SizeByte:
		imm = uint32(c.FetchInstructionWord() & 0xFF)
	case SizeWord:
		imm = uint32(c.FetchInstructionWord())
	case SizeLong:
		hi := c.FetchInstructionWord()
		lo := c.FetchInstructionWord()
		imm = (uint32(hi) << 16) | uint32(lo)
	}

	ea := c.ResolveEA(mode, reg, size)
	val := c.ReadEA(ea)

	var res uint32
	switch typeBits {
	case 0: // ORI
		res = c.Or(val, imm, size)
		c.WriteEA(ea, res)
	case 1: // ANDI
		res = c.And(val, imm, size)
		c.WriteEA(ea, res)
	case 2: // SUBI
		res = c.Sub(val, imm, size)
		c.WriteEA(ea, res)
	case 3: // ADDI
		res = c.Add(val, imm, size)
		c.WriteEA(ea, res)
	case 5: // EORI
		res = c.Eor(val, imm, size)
		c.WriteEA(ea, res)
	case 6: // CMPI
		c.Cmp(val, imm, size)
	default:
		c.TriggerException(VecIllegalInstr)
		return
	}
	c.CommitEA(ea)
}

func (c *CPU) dispatchGroup4(op uint16) {
	if op == 0x4AFC {
		c.TriggerException(VecIllegalInstr)
		return
	}
	if op == 0x4E70 {
		if (c.SR & FlagS) == 0 {
			c.TriggerException(VecPrivilegeViol)
		}
		return
	}
	if op == 0x4E71 { // NOP
		return
	}
	if op == 0x4E72 { // STOP
		c.opStop()
		return
	}
	if op == 0x4E73 { // RTE
		c.opRte()
		return
	}
	if op == 0x4E75 { // RTS
		c.opRts()
		return
	}
	if op == 0x4E76 { // TRAPV
		c.opTrapv()
		return
	}
	if op == 0x4E77 { // RTR
		c.opRtr()
		return
	}

	if (op & 0xFFF0) == 0x4E40 {
		c.opTrap(op)
		return
	}
	if (op & 0xFFF8) == 0x4E50 {
		c.opLink(op)
		return
	}
	if (op & 0xFFF8) == 0x4E58 {
		c.opUnlk(op)
		return
	}
	if (op & 0xFFF8) == 0x4E60 {
		if (c.SR & FlagS) == 0 {
			c.TriggerException(VecPrivilegeViol)
			return
		}
		c.USP = c.A[op&7]
		return
	}
	if (op & 0xFFF8) == 0x4E68 {
		if (c.SR & FlagS) == 0 {
			c.TriggerException(VecPrivilegeViol)
			return
		}
		c.A[op&7] = c.USP
		return
	}

	if (op & 0xFFC0) == 0x4E80 {
		c.opJsr(op)
		return
	}
	if (op & 0xFFC0) == 0x4EC0 {
		c.opJmp(op)
		return
	}

	// MOVEM
	if (op & 0xFB80) == 0x4880 {
		c.opMoveM(op)
		return
	}

	// LEA
	if (op & 0xF1C0) == 0x41C0 {
		c.opLea(op)
		return
	}

	// CHK
	if (op & 0xF1C0) == 0x4180 {
		c.opChk(op)
		return
	}

	// PEA / SWAP
	if (op & 0xFFC0) == 0x4840 {
		if ((op >> 3) & 7) == 0 {
			c.opSwap(op)
		} else {
			c.opPea(op)
		}
		return
	}

	// EXT
	if (op & 0xFFB8) == 0x4880 {
		c.opExt(op)
		return
	}

	// MOVE from SR
	if (op & 0xFFC0) == 0x40C0 {
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), SizeWord)
		c.WriteEA(ea, uint32(c.SR))
		c.CommitEA(ea)
		return
	}

	// MOVE to CCR
	if (op & 0xFFC0) == 0x44C0 {
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), SizeWord)
		val := c.ReadEA(ea)
		c.CommitEA(ea)
		c.SetCCR(byte(val))
		return
	}

	// MOVE to SR
	if (op & 0xFFC0) == 0x46C0 {
		if (c.SR & FlagS) == 0 {
			c.TriggerException(VecPrivilegeViol)
			return
		}
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), SizeWord)
		val := c.ReadEA(ea)
		c.CommitEA(ea)
		c.SetSR(uint16(val))
		return
	}

	// TAS
	if (op & 0xFFC0) == 0x4AC0 {
		c.opTas(op)
		return
	}

	// TST
	if (op & 0xFF00) == 0x4A00 {
		size, ok := decodeStandardSize((op >> 6) & 3)
		if !ok {
			c.TriggerException(VecIllegalInstr)
			return
		}
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
		val := c.ReadEA(ea)
		c.CommitEA(ea)
		c.Tst(val, size)
		return
	}

	// CLR
	if (op & 0xFF00) == 0x4200 {
		size, ok := decodeStandardSize((op >> 6) & 3)
		if !ok {
			c.TriggerException(VecIllegalInstr)
			return
		}
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
		res := c.Clr(size)
		c.WriteEA(ea, res)
		c.CommitEA(ea)
		return
	}

	// NEG
	if (op & 0xFF00) == 0x4400 {
		size, ok := decodeStandardSize((op >> 6) & 3)
		if !ok {
			c.TriggerException(VecIllegalInstr)
			return
		}
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
		val := c.ReadEA(ea)
		res := c.Neg(val, size)
		c.WriteEA(ea, res)
		c.CommitEA(ea)
		return
	}

	// NEGX
	if (op & 0xFF00) == 0x4000 {
		size, ok := decodeStandardSize((op >> 6) & 3)
		if !ok {
			c.TriggerException(VecIllegalInstr)
			return
		}
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
		val := c.ReadEA(ea)
		res := c.NegX(val, size)
		c.WriteEA(ea, res)
		c.CommitEA(ea)
		return
	}

	// NOT
	if (op & 0xFF00) == 0x4600 {
		size, ok := decodeStandardSize((op >> 6) & 3)
		if !ok {
			c.TriggerException(VecIllegalInstr)
			return
		}
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
		val := c.ReadEA(ea)
		res := c.Not(val, size)
		c.WriteEA(ea, res)
		c.CommitEA(ea)
		return
	}

	c.TriggerException(VecIllegalInstr)
}

func (c *CPU) dispatchGroup5(op uint16) {
	isSub := (op & 0x0100) != 0
	sizeBits := (op >> 6) & 3

	if sizeBits == 3 {
		// Scc or DBcc
		mode := uint8((op >> 3) & 7)
		if mode == 1 {
			c.opDbcc(op)
		} else {
			c.opScc(op)
		}
		return
	}

	size, ok := decodeStandardSize(sizeBits)
	if !ok {
		c.TriggerException(VecIllegalInstr)
		return
	}

	data := uint32((op >> 9) & 7)
	if data == 0 {
		data = 8
	}

	mode := uint8((op >> 3) & 7)
	reg := uint8(op & 7)

	if mode == 1 {
		// ADDQ / SUBQ to An affects full 32-bit register without touching flags
		if isSub {
			c.A[reg] -= data
		} else {
			c.A[reg] += data
		}
		return
	}

	ea := c.ResolveEA(mode, reg, size)
	val := c.ReadEA(ea)
	var res uint32
	if isSub {
		res = c.Sub(val, data, size)
	} else {
		res = c.Add(val, data, size)
	}
	c.WriteEA(ea, res)
	c.CommitEA(ea)
}

func (c *CPU) dispatchGroup8(op uint16) {
	if (op & 0x01C0) == 0x00C0 {
		// DIVU
		dReg := (op >> 9) & 7
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), SizeWord)
		divisor := uint16(c.ReadEA(ea))
		c.CommitEA(ea)
		if res, ok := c.Divu(c.D[dReg], divisor); ok {
			c.D[dReg] = res
		}
		return
	}
	if (op & 0x01C0) == 0x01C0 {
		// DIVS
		dReg := (op >> 9) & 7
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), SizeWord)
		divisor := int16(c.ReadEA(ea))
		c.CommitEA(ea)
		if res, ok := c.Divs(c.D[dReg], divisor); ok {
			c.D[dReg] = res
		}
		return
	}
	if (op & 0x01F0) == 0x0100 {
		// SBCD
		rx := uint8((op >> 9) & 7)
		ry := uint8(op & 7)
		isMem := (op & 0x0008) != 0
		if isMem {
			eaY := c.ResolveEA(4, ry, SizeByte)
			src := byte(c.ReadEA(eaY))
			c.CommitEA(eaY)
			eaX := c.ResolveEA(4, rx, SizeByte)
			dst := byte(c.ReadEA(eaX))
			res := c.Sbcd(dst, src)
			c.WriteEA(eaX, uint32(res))
			c.CommitEA(eaX)
		} else {
			src := byte(c.D[ry])
			dst := byte(c.D[rx])
			res := c.Sbcd(dst, src)
			c.D[rx] = (c.D[rx] & 0xFFFFFF00) | uint32(res)
		}
		return
	}

	// OR
	dir := (op & 0x0100) != 0
	size, ok := decodeStandardSize((op >> 6) & 3)
	if !ok {
		c.TriggerException(VecIllegalInstr)
		return
	}
	dReg := (op >> 9) & 7
	ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
	if !dir {
		val := c.ReadEA(ea)
		c.CommitEA(ea)
		res := c.Or(c.D[dReg], val, size)
		mask := maskForSize(size)
		c.D[dReg] = (c.D[dReg] &^ mask) | (res & mask)
	} else {
		val := c.ReadEA(ea)
		res := c.Or(val, c.D[dReg], size)
		c.WriteEA(ea, res)
		c.CommitEA(ea)
	}
}

func (c *CPU) dispatchGroup9(op uint16) {
	if (op & 0x00C0) == 0x00C0 {
		// SUBA
		isLong := (op & 0x0100) != 0
		size := SizeWord
		if isLong {
			size = SizeLong
		}
		aReg := (op >> 9) & 7
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
		val := c.ReadEA(ea)
		c.CommitEA(ea)
		if size == SizeWord {
			val = uint32(int32(int16(val)))
		}
		c.A[aReg] -= val
		return
	}
	if (op & 0x0130) == 0x0100 {
		// SUBX
		size, ok := decodeStandardSize((op >> 6) & 3)
		if !ok {
			c.TriggerException(VecIllegalInstr)
			return
		}
		rx := uint8((op >> 9) & 7)
		ry := uint8(op & 7)
		isMem := (op & 0x0008) != 0
		if isMem {
			eaY := c.ResolveEA(4, ry, size)
			src := c.ReadEA(eaY)
			c.CommitEA(eaY)
			eaX := c.ResolveEA(4, rx, size)
			dst := c.ReadEA(eaX)
			res := c.SubX(dst, src, size)
			c.WriteEA(eaX, res)
			c.CommitEA(eaX)
		} else {
			res := c.SubX(c.D[rx], c.D[ry], size)
			mask := maskForSize(size)
			c.D[rx] = (c.D[rx] &^ mask) | (res & mask)
		}
		return
	}

	// SUB
	dir := (op & 0x0100) != 0
	size, ok := decodeStandardSize((op >> 6) & 3)
	if !ok {
		c.TriggerException(VecIllegalInstr)
		return
	}
	dReg := (op >> 9) & 7
	ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
	if !dir {
		val := c.ReadEA(ea)
		c.CommitEA(ea)
		res := c.Sub(c.D[dReg], val, size)
		mask := maskForSize(size)
		c.D[dReg] = (c.D[dReg] &^ mask) | (res & mask)
	} else {
		val := c.ReadEA(ea)
		res := c.Sub(val, c.D[dReg], size)
		c.WriteEA(ea, res)
		c.CommitEA(ea)
	}
}

func (c *CPU) dispatchGroupB(op uint16) {
	if (op & 0x00C0) == 0x00C0 {
		// CMPA
		isLong := (op & 0x0100) != 0
		size := SizeWord
		if isLong {
			size = SizeLong
		}
		aReg := (op >> 9) & 7
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
		val := c.ReadEA(ea)
		c.CommitEA(ea)
		if size == SizeWord {
			val = uint32(int32(int16(val)))
		}
		c.Cmp(c.A[aReg], val, SizeLong)
		return
	}
	if (op & 0x0138) == 0x0108 {
		// CMPM (Ay)+, (Ax)+
		size, ok := decodeStandardSize((op >> 6) & 3)
		if !ok {
			c.TriggerException(VecIllegalInstr)
			return
		}
		ax := uint8((op >> 9) & 7)
		ay := uint8(op & 7)
		eaY := c.ResolveEA(3, ay, size)
		src := c.ReadEA(eaY)
		c.CommitEA(eaY)
		eaX := c.ResolveEA(3, ax, size)
		dst := c.ReadEA(eaX)
		c.CommitEA(eaX)
		c.Cmp(dst, src, size)
		return
	}

	size, ok := decodeStandardSize((op >> 6) & 3)
	if !ok {
		c.TriggerException(VecIllegalInstr)
		return
	}
	dReg := (op >> 9) & 7
	ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)

	if (op & 0x0100) != 0 {
		// EOR
		val := c.ReadEA(ea)
		res := c.Eor(val, c.D[dReg], size)
		c.WriteEA(ea, res)
		c.CommitEA(ea)
	} else {
		// CMP
		val := c.ReadEA(ea)
		c.CommitEA(ea)
		c.Cmp(c.D[dReg], val, size)
	}
}

func (c *CPU) dispatchGroupC(op uint16) {
	if (op & 0x01C0) == 0x00C0 {
		// MULU
		dReg := (op >> 9) & 7
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), SizeWord)
		src := uint16(c.ReadEA(ea))
		c.CommitEA(ea)
		c.D[dReg] = c.Mulu(uint16(c.D[dReg]), src)
		return
	}
	if (op & 0x01C0) == 0x01C0 {
		// MULS
		dReg := (op >> 9) & 7
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), SizeWord)
		src := int16(c.ReadEA(ea))
		c.CommitEA(ea)
		c.D[dReg] = c.Muls(int16(c.D[dReg]), src)
		return
	}
	if (op & 0x01F0) == 0x0100 {
		// ABCD
		rx := uint8((op >> 9) & 7)
		ry := uint8(op & 7)
		isMem := (op & 0x0008) != 0
		if isMem {
			eaY := c.ResolveEA(4, ry, SizeByte)
			src := byte(c.ReadEA(eaY))
			c.CommitEA(eaY)
			eaX := c.ResolveEA(4, rx, SizeByte)
			dst := byte(c.ReadEA(eaX))
			res := c.Abcd(dst, src)
			c.WriteEA(eaX, uint32(res))
			c.CommitEA(eaX)
		} else {
			src := byte(c.D[ry])
			dst := byte(c.D[rx])
			res := c.Abcd(dst, src)
			c.D[rx] = (c.D[rx] & 0xFFFFFF00) | uint32(res)
		}
		return
	}
	if (op & 0x0130) == 0x0100 && (op&0x0008) == 0 {
		// EXG
		c.opExg(op)
		return
	}

	// AND
	dir := (op & 0x0100) != 0
	size, ok := decodeStandardSize((op >> 6) & 3)
	if !ok {
		c.TriggerException(VecIllegalInstr)
		return
	}
	dReg := (op >> 9) & 7
	ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
	if !dir {
		val := c.ReadEA(ea)
		c.CommitEA(ea)
		res := c.And(c.D[dReg], val, size)
		mask := maskForSize(size)
		c.D[dReg] = (c.D[dReg] &^ mask) | (res & mask)
	} else {
		val := c.ReadEA(ea)
		res := c.And(val, c.D[dReg], size)
		c.WriteEA(ea, res)
		c.CommitEA(ea)
	}
}

func (c *CPU) dispatchGroupD(op uint16) {
	if (op & 0x00C0) == 0x00C0 {
		// ADDA
		isLong := (op & 0x0100) != 0
		size := SizeWord
		if isLong {
			size = SizeLong
		}
		aReg := (op >> 9) & 7
		ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
		val := c.ReadEA(ea)
		c.CommitEA(ea)
		if size == SizeWord {
			val = uint32(int32(int16(val)))
		}
		c.A[aReg] += val
		return
	}
	if (op & 0x0130) == 0x0100 {
		// ADDX
		size, ok := decodeStandardSize((op >> 6) & 3)
		if !ok {
			c.TriggerException(VecIllegalInstr)
			return
		}
		rx := uint8((op >> 9) & 7)
		ry := uint8(op & 7)
		isMem := (op & 0x0008) != 0
		if isMem {
			eaY := c.ResolveEA(4, ry, size)
			src := c.ReadEA(eaY)
			c.CommitEA(eaY)
			eaX := c.ResolveEA(4, rx, size)
			dst := c.ReadEA(eaX)
			res := c.AddX(dst, src, size)
			c.WriteEA(eaX, res)
			c.CommitEA(eaX)
		} else {
			res := c.AddX(c.D[rx], c.D[ry], size)
			mask := maskForSize(size)
			c.D[rx] = (c.D[rx] &^ mask) | (res & mask)
		}
		return
	}

	// ADD
	dir := (op & 0x0100) != 0
	size, ok := decodeStandardSize((op >> 6) & 3)
	if !ok {
		c.TriggerException(VecIllegalInstr)
		return
	}
	dReg := (op >> 9) & 7
	ea := c.ResolveEA(uint8((op>>3)&7), uint8(op&7), size)
	if !dir {
		val := c.ReadEA(ea)
		c.CommitEA(ea)
		res := c.Add(c.D[dReg], val, size)
		mask := maskForSize(size)
		c.D[dReg] = (c.D[dReg] &^ mask) | (res & mask)
	} else {
		val := c.ReadEA(ea)
		res := c.Add(val, c.D[dReg], size)
		c.WriteEA(ea, res)
		c.CommitEA(ea)
	}
}
