package vm

import (
	"fmt"
)

func (c *CPU) executeInstruction() int {
	op := c.fetchByte()

	switch op {
	case 0x10:
		return c.executePage1()
	case 0x11:
		return c.executePage2()
	default:
		return c.executePage0(op)
	}
}

// executePage0 dispatches single-byte opcodes.
func (c *CPU) executePage0(op byte) int {
	switch op {
	// Inherent & System
	case 0x12: // NOP
		return 2
	case 0x13: // SYNC
		c.Waiting = true
		return 4
	case 0x19: // DAA
		c.daa()
		return 2
	case 0x1A: // ORCC #imm
		imm := c.fetchByte()
		c.CC |= imm
		return 3
	case 0x1C: // ANDCC #imm
		imm := c.fetchByte()
		c.CC &= imm
		return 3
	case 0x1D: // SEX
		if (c.B & 0x80) != 0 {
			c.A = 0xFF
		} else {
			c.A = 0x00
		}
		c.updateNZ16(c.GetD())
		return 2
	case 0x1E: // EXG r1, r2
		pb := c.fetchByte()
		c.executeEXG(pb)
		return 8
	case 0x1F: // TFR r1, r2
		pb := c.fetchByte()
		c.executeTFR(pb)
		return 6
	case 0x39: // RTS
		c.PC = c.PullWord()
		return 5
	case 0x3A: // ABX
		c.X += uint16(c.B)
		return 3
	case 0x3B: // RTI
		c.ExecuteRTI()
		if (c.CC & FlagE) != 0 {
			if (c.MD & 0x01) != 0 {
				return 17 // 6309 native frame
			}
			return 15 // 6809 full frame
		}
		return 6 // FIRQ frame
	case 0x3C: // CWAI #imm
		imm := c.fetchByte()
		c.CC &= imm
		c.PushInterruptFrame(true)
		c.Waiting = true
		return 20
	case 0x3D: // MUL
		res := uint16(c.A) * uint16(c.B)
		c.SetD(res)
		c.updateNZ16(res)
		c.setCarry((c.B & 0x80) != 0)
		return 11
	case 0x3F: // SWI (SWI1)
		c.PushInterruptFrame(true)
		c.CC |= FlagI | FlagF
		c.triggerVector(0xFFFA)
		return 19

	// LEA Instructions
	case 0x30: // LEAX indexed
		c.X = c.resolveIndexed()
		if c.X == 0 {
			c.CC |= FlagZ
		} else {
			c.CC &^= FlagZ
		}
		return 4
	case 0x31: // LEAY indexed
		c.Y = c.resolveIndexed()
		if c.Y == 0 {
			c.CC |= FlagZ
		} else {
			c.CC &^= FlagZ
		}
		return 4
	case 0x32: // LEAS indexed
		c.S = c.resolveIndexed()
		return 4
	case 0x33: // LEAU indexed
		c.U = c.resolveIndexed()
		return 4

	// Stack PSH/PUL
	case 0x34: // PSHS
		pb := c.fetchByte()
		c.ExecutePSHS(pb)
		return 5
	case 0x35: // PULS
		pb := c.fetchByte()
		c.ExecutePULS(pb)
		return 5
	case 0x36: // PSHU
		pb := c.fetchByte()
		c.ExecutePSHU(pb)
		return 5
	case 0x37: // PULU
		pb := c.fetchByte()
		c.ExecutePULU(pb)
		return 5

	// Short Branches ($20..$2F)
	case 0x20: // BRA
		return c.branch8(true)
	case 0x21: // BRN
		return c.branch8(false)
	case 0x22: // BHI (C=0 and Z=0)
		return c.branch8((c.CC & (FlagC | FlagZ)) == 0)
	case 0x23: // BLS (C=1 or Z=1)
		return c.branch8((c.CC & (FlagC | FlagZ)) != 0)
	case 0x24: // BHS / BCC (C=0)
		return c.branch8((c.CC & FlagC) == 0)
	case 0x25: // BLO / BCS (C=1)
		return c.branch8((c.CC & FlagC) != 0)
	case 0x26: // BNE (Z=0)
		return c.branch8((c.CC & FlagZ) == 0)
	case 0x27: // BEQ (Z=1)
		return c.branch8((c.CC & FlagZ) != 0)
	case 0x28: // BVC (V=0)
		return c.branch8((c.CC & FlagV) == 0)
	case 0x29: // BVS (V=1)
		return c.branch8((c.CC & FlagV) != 0)
	case 0x2A: // BPL (N=0)
		return c.branch8((c.CC & FlagN) == 0)
	case 0x2B: // BMI (N=1)
		return c.branch8((c.CC & FlagN) != 0)
	case 0x2C: // BGE (N == V)
		n := (c.CC & FlagN) != 0
		v := (c.CC & FlagV) != 0
		return c.branch8(n == v)
	case 0x2D: // BLT (N != V)
		n := (c.CC & FlagN) != 0
		v := (c.CC & FlagV) != 0
		return c.branch8(n != v)
	case 0x2E: // BGT (Z=0 and N==V)
		z := (c.CC & FlagZ) != 0
		n := (c.CC & FlagN) != 0
		v := (c.CC & FlagV) != 0
		return c.branch8(!z && (n == v))
	case 0x2F: // BLE (Z=1 or N!=V)
		z := (c.CC & FlagZ) != 0
		n := (c.CC & FlagN) != 0
		v := (c.CC & FlagV) != 0
		return c.branch8(z || (n != v))
	case 0x8D: // BSR rel8
		offset := int8(c.fetchByte())
		c.PushWord(c.PC)
		c.PC = uint16(int32(c.PC) + int32(offset))
		return 7
	case 0x16: // LBRA rel16
		offset := int16(c.fetchWord())
		c.PC = uint16(int32(c.PC) + int32(offset))
		return 5
	case 0x17: // LBSR rel16
		offset := int16(c.fetchWord())
		c.PushWord(c.PC)
		c.PC = uint16(int32(c.PC) + int32(offset))
		return 9

	// Accumulator single-operand ($40..$4F for A, $50..$5F for B)
	case 0x40: // NEGA
		c.A = c.neg8(c.A)
		return 2
	case 0x50: // NEGB
		c.B = c.neg8(c.B)
		return 2
	case 0x43: // COMA
		c.A = c.com8(c.A)
		return 2
	case 0x53: // COMB
		c.B = c.com8(c.B)
		return 2
	case 0x44: // LSRA
		c.A = c.lsr8(c.A)
		return 2
	case 0x54: // LSRB
		c.B = c.lsr8(c.B)
		return 2
	case 0x46: // RORA
		c.A = c.ror8(c.A)
		return 2
	case 0x56: // RORB
		c.B = c.ror8(c.B)
		return 2
	case 0x47: // ASRA
		c.A = c.asr8(c.A)
		return 2
	case 0x57: // ASRB
		c.B = c.asr8(c.B)
		return 2
	case 0x48: // ASLA / LSLA
		c.A = c.asl8(c.A)
		return 2
	case 0x58: // ASLB / LSLB
		c.B = c.asl8(c.B)
		return 2
	case 0x49: // ROLA
		c.A = c.rol8(c.A)
		return 2
	case 0x59: // ROLB
		c.B = c.rol8(c.B)
		return 2
	case 0x4A: // DECA
		c.A = c.dec8(c.A)
		return 2
	case 0x5A: // DECB
		c.B = c.dec8(c.B)
		return 2
	case 0x4C: // INCA
		c.A = c.inc8(c.A)
		return 2
	case 0x5C: // INCB
		c.B = c.inc8(c.B)
		return 2
	case 0x4D: // TSTA
		c.tst8(c.A)
		return 2
	case 0x5D: // TSTB
		c.tst8(c.B)
		return 2
	case 0x4F: // CLRA
		c.A = 0
		c.updateNZ8(0)
		c.CC &^= (FlagV | FlagC)
		c.CC |= FlagZ
		return 2
	case 0x5F: // CLRB
		c.B = 0
		c.updateNZ8(0)
		c.CC &^= (FlagV | FlagC)
		c.CC |= FlagZ
		return 2

	// Memory single-operand ($00..$0F direct, $60..$6F indexed, $70..$7F extended)
	case 0x00, 0x60, 0x70: // NEG
		ea := c.getEA(op & 0xF0)
		val := c.neg8(c.Bus.ReadByte(ea))
		c.Bus.WriteByte(ea, val)
		return 6
	case 0x03, 0x63, 0x73: // COM
		ea := c.getEA(op & 0xF0)
		val := c.com8(c.Bus.ReadByte(ea))
		c.Bus.WriteByte(ea, val)
		return 6
	case 0x04, 0x64, 0x74: // LSR
		ea := c.getEA(op & 0xF0)
		val := c.lsr8(c.Bus.ReadByte(ea))
		c.Bus.WriteByte(ea, val)
		return 6
	case 0x06, 0x66, 0x76: // ROR
		ea := c.getEA(op & 0xF0)
		val := c.ror8(c.Bus.ReadByte(ea))
		c.Bus.WriteByte(ea, val)
		return 6
	case 0x07, 0x67, 0x77: // ASR
		ea := c.getEA(op & 0xF0)
		val := c.asr8(c.Bus.ReadByte(ea))
		c.Bus.WriteByte(ea, val)
		return 6
	case 0x08, 0x68, 0x78: // ASL / LSL
		ea := c.getEA(op & 0xF0)
		val := c.asl8(c.Bus.ReadByte(ea))
		c.Bus.WriteByte(ea, val)
		return 6
	case 0x09, 0x69, 0x79: // ROL
		ea := c.getEA(op & 0xF0)
		val := c.rol8(c.Bus.ReadByte(ea))
		c.Bus.WriteByte(ea, val)
		return 6
	case 0x0A, 0x6A, 0x7A: // DEC
		ea := c.getEA(op & 0xF0)
		val := c.dec8(c.Bus.ReadByte(ea))
		c.Bus.WriteByte(ea, val)
		return 6
	case 0x0C, 0x6C, 0x7C: // INC
		ea := c.getEA(op & 0xF0)
		val := c.inc8(c.Bus.ReadByte(ea))
		c.Bus.WriteByte(ea, val)
		return 6
	case 0x0D, 0x6D, 0x7D: // TST
		ea := c.getEA(op & 0xF0)
		c.tst8(c.Bus.ReadByte(ea))
		return 6
	case 0x0E, 0x6E, 0x7E: // JMP
		c.PC = c.getEA(op & 0xF0)
		return 3
	case 0x0F, 0x6F, 0x7F: // CLR
		ea := c.getEA(op & 0xF0)
		c.Bus.WriteByte(ea, 0)
		c.updateNZ8(0)
		c.CC &^= (FlagV | FlagC)
		c.CC |= FlagZ
		return 6

	// Two-operand instructions for Accumulator A ($80..$BF)
	case 0x80, 0x90, 0xA0, 0xB0: // SUBA
		c.A = c.sub8(c.A, c.getOperand8(op))
		return 2
	case 0x81, 0x91, 0xA1, 0xB1: // CMPA
		c.sub8(c.A, c.getOperand8(op))
		return 2
	case 0x82, 0x92, 0xA2, 0xB2: // SBCA
		c.A = c.sbc8(c.A, c.getOperand8(op))
		return 2
	case 0x83, 0x93, 0xA3, 0xB3: // SUBD
		c.SetD(c.sub16(c.GetD(), c.getOperand16(op)))
		return 4
	case 0x84, 0x94, 0xA4, 0xB4: // ANDA
		c.A &= c.getOperand8(op)
		c.updateNZ8(c.A)
		c.CC &^= FlagV
		return 2
	case 0x85, 0x95, 0xA5, 0xB5: // BITA
		val := c.A & c.getOperand8(op)
		c.updateNZ8(val)
		c.CC &^= FlagV
		return 2
	case 0x86, 0x96, 0xA6, 0xB6: // LDA
		c.A = c.getOperand8(op)
		c.updateNZ8(c.A)
		c.CC &^= FlagV
		return 2
	case 0x97, 0xA7, 0xB7: // STA
		ea := c.getEA(op & 0xF0)
		c.Bus.WriteByte(ea, c.A)
		c.updateNZ8(c.A)
		c.CC &^= FlagV
		return 4
	case 0x88, 0x98, 0xA8, 0xB8: // EORA
		c.A ^= c.getOperand8(op)
		c.updateNZ8(c.A)
		c.CC &^= FlagV
		return 2
	case 0x89, 0x99, 0xA9, 0xB9: // ADCA
		c.A = c.adc8(c.A, c.getOperand8(op))
		return 2
	case 0x8A, 0x9A, 0xAA, 0xBA: // ORA
		c.A |= c.getOperand8(op)
		c.updateNZ8(c.A)
		c.CC &^= FlagV
		return 2
	case 0x8B, 0x9B, 0xAB, 0xBB: // ADDA
		c.A = c.add8(c.A, c.getOperand8(op))
		return 2
	case 0x8C, 0x9C, 0xAC, 0xBC: // CMPX
		c.sub16(c.X, c.getOperand16(op))
		return 4
	case 0x9D, 0xAD, 0xBD: // JSR
		ea := c.getEA(op & 0xF0)
		c.PushWord(c.PC)
		c.PC = ea
		return 7
	case 0x8E, 0x9E, 0xAE, 0xBE: // LDX
		c.X = c.getOperand16(op)
		c.updateNZ16(c.X)
		c.CC &^= FlagV
		return 3
	case 0x9F, 0xAF, 0xBF: // STX
		ea := c.getEA(op & 0xF0)
		c.Bus.WriteWord(ea, c.X)
		c.updateNZ16(c.X)
		c.CC &^= FlagV
		return 4

	// Two-operand instructions for Accumulator B & 16-bit registers ($C0..$FF)
	case 0xC0, 0xD0, 0xE0, 0xF0: // SUBB
		c.B = c.sub8(c.B, c.getOperand8(op))
		return 2
	case 0xC1, 0xD1, 0xE1, 0xF1: // CMPB
		c.sub8(c.B, c.getOperand8(op))
		return 2
	case 0xC2, 0xD2, 0xE2, 0xF2: // SBCB
		c.B = c.sbc8(c.B, c.getOperand8(op))
		return 2
	case 0xC3, 0xD3, 0xE3, 0xF3: // ADDD
		c.SetD(c.add16(c.GetD(), c.getOperand16(op)))
		return 4
	case 0xC4, 0xD4, 0xE4, 0xF4: // ANDB
		c.B &= c.getOperand8(op)
		c.updateNZ8(c.B)
		c.CC &^= FlagV
		return 2
	case 0xC5, 0xD5, 0xE5, 0xF5: // BITB
		val := c.B & c.getOperand8(op)
		c.updateNZ8(val)
		c.CC &^= FlagV
		return 2
	case 0xC6, 0xD6, 0xE6, 0xF6: // LDB
		c.B = c.getOperand8(op)
		c.updateNZ8(c.B)
		c.CC &^= FlagV
		return 2
	case 0xD7, 0xE7, 0xF7: // STB
		ea := c.getEA(op & 0xF0)
		c.Bus.WriteByte(ea, c.B)
		c.updateNZ8(c.B)
		c.CC &^= FlagV
		return 4
	case 0xC8, 0xD8, 0xE8, 0xF8: // EORB
		c.B ^= c.getOperand8(op)
		c.updateNZ8(c.B)
		c.CC &^= FlagV
		return 2
	case 0xC9, 0xD9, 0xE9, 0xF9: // ADCB
		c.B = c.adc8(c.B, c.getOperand8(op))
		return 2
	case 0xCA, 0xDA, 0xEA, 0xFA: // ORB
		c.B |= c.getOperand8(op)
		c.updateNZ8(c.B)
		c.CC &^= FlagV
		return 2
	case 0xCB, 0xDB, 0xEB, 0xFB: // ADDB
		c.B = c.add8(c.B, c.getOperand8(op))
		return 2
	case 0xCC, 0xDC, 0xEC, 0xFC: // LDD
		c.SetD(c.getOperand16(op))
		c.updateNZ16(c.GetD())
		c.CC &^= FlagV
		return 3
	case 0xDD, 0xED, 0xFD: // STD
		ea := c.getEA(op & 0xF0)
		c.Bus.WriteWord(ea, c.GetD())
		c.updateNZ16(c.GetD())
		c.CC &^= FlagV
		return 4
	case 0xCE, 0xDE, 0xEE, 0xFE: // LDU
		c.U = c.getOperand16(op)
		c.updateNZ16(c.U)
		c.CC &^= FlagV
		return 3
	case 0xDF, 0xEF, 0xFF: // STU
		ea := c.getEA(op & 0xF0)
		c.Bus.WriteWord(ea, c.U)
		c.updateNZ16(c.U)
		c.CC &^= FlagV
		return 4

	default:
		panic(fmt.Errorf("unimplemented Page 0 opcode 0x%02X at PC 0x%04X", op, c.PC-1))
	}
}

// executePage1 dispatches opcodes with 0x10 prefix.
func (c *CPU) executePage1() int {
	op := c.fetchByte()
	switch op {
	case 0x3F: // SWI2
		c.PushInterruptFrame(true)
		c.triggerVector(0xFFF4)
		return 20

	case 0x83, 0x93, 0xA3, 0xB3: // CMPD
		c.sub16(c.GetD(), c.getOperand16(op))
		return 5
	case 0x8C, 0x9C, 0xAC, 0xBC: // CMPY
		c.sub16(c.Y, c.getOperand16(op))
		return 5
	case 0x8E, 0x9E, 0xAE, 0xBE: // LDY
		c.Y = c.getOperand16(op)
		c.updateNZ16(c.Y)
		c.CC &^= FlagV
		return 4
	case 0x9F, 0xAF, 0xBF: // STY
		ea := c.getEA(op & 0xF0)
		c.Bus.WriteWord(ea, c.Y)
		c.updateNZ16(c.Y)
		c.CC &^= FlagV
		return 5

	case 0xCE, 0xDE, 0xEE, 0xFE: // LDS
		c.S = c.getOperand16(op)
		c.updateNZ16(c.S)
		c.CC &^= FlagV
		return 4
	case 0xDF, 0xEF, 0xFF: // STS
		ea := c.getEA(op & 0xF0)
		c.Bus.WriteWord(ea, c.S)
		c.updateNZ16(c.S)
		c.CC &^= FlagV
		return 5

	// Long conditional branches ($10 $21..$2F)
	case 0x21: // LBRN
		return c.branch16(false)
	case 0x22: // LBHI
		return c.branch16((c.CC & (FlagC | FlagZ)) == 0)
	case 0x23: // LBLS
		return c.branch16((c.CC & (FlagC | FlagZ)) != 0)
	case 0x24: // LBCC / LBHS
		return c.branch16((c.CC & FlagC) == 0)
	case 0x25: // LBCS / LBLO
		return c.branch16((c.CC & FlagC) != 0)
	case 0x26: // LBNE
		return c.branch16((c.CC & FlagZ) == 0)
	case 0x27: // LBEQ
		return c.branch16((c.CC & FlagZ) != 0)
	case 0x28: // LBVC
		return c.branch16((c.CC & FlagV) == 0)
	case 0x29: // LBVS
		return c.branch16((c.CC & FlagV) != 0)
	case 0x2A: // LBPL
		return c.branch16((c.CC & FlagN) == 0)
	case 0x2B: // LBMI
		return c.branch16((c.CC & FlagN) != 0)
	case 0x2C: // LBGE
		n := (c.CC & FlagN) != 0
		v := (c.CC & FlagV) != 0
		return c.branch16(n == v)
	case 0x2D: // LBLT
		n := (c.CC & FlagN) != 0
		v := (c.CC & FlagV) != 0
		return c.branch16(n != v)
	case 0x2E: // LBGT
		z := (c.CC & FlagZ) != 0
		n := (c.CC & FlagN) != 0
		v := (c.CC & FlagV) != 0
		return c.branch16(!z && (n == v))
	case 0x2F: // LBLE
		z := (c.CC & FlagZ) != 0
		n := (c.CC & FlagN) != 0
		v := (c.CC & FlagV) != 0
		return c.branch16(z || (n != v))

	default:
		panic(fmt.Errorf("unimplemented Page 1 opcode 0x10 0x%02X at PC 0x%04X", op, c.PC-2))
	}
}

// executePage2 dispatches opcodes with 0x11 prefix.
func (c *CPU) executePage2() int {
	op := c.fetchByte()
	switch op {
	case 0x3D: // LDMD #imm (Hitachi 6309 Mode Register)
		imm := c.fetchByte()
		c.MD = imm
		return 5

	case 0x3F: // SWI3
		c.PushInterruptFrame(true)
		c.triggerVector(0xFFF2)
		return 20

	case 0x83, 0x93, 0xA3, 0xB3: // CMPU
		c.sub16(c.U, c.getOperand16(op))
		return 5
	case 0x8C, 0x9C, 0xAC, 0xBC: // CMPS
		c.sub16(c.S, c.getOperand16(op))
		return 5

	default:
		panic(fmt.Errorf("unimplemented Page 2 opcode 0x11 0x%02X at PC 0x%04X", op, c.PC-2))
	}
}

// Addressing resolution helpers

func (c *CPU) getEA(mode byte) uint16 {
	switch mode {
	case 0x00, 0x90, 0xD0: // Direct
		offset := c.fetchByte()
		return (uint16(c.DP) << 8) | uint16(offset)
	case 0x60, 0xA0, 0xE0: // Indexed
		return c.resolveIndexed()
	case 0x70, 0xB0, 0xF0: // Extended
		return c.fetchWord()
	default:
		panic(fmt.Sprintf("invalid EA mode: 0x%02X", mode))
	}
}

func (c *CPU) getOperand8(op byte) byte {
	mode := op & 0xF0
	if mode == 0x80 || mode == 0xC0 {
		return c.fetchByte()
	}
	ea := c.getEA(mode)
	return c.Bus.ReadByte(ea)
}

func (c *CPU) getOperand16(op byte) uint16 {
	mode := op & 0xF0
	if mode == 0x80 || mode == 0xC0 {
		return c.fetchWord()
	}
	ea := c.getEA(mode)
	return c.Bus.ReadWord(ea)
}

func (c *CPU) branch8(cond bool) int {
	offset := int8(c.fetchByte())
	if cond {
		c.PC = uint16(int32(c.PC) + int32(offset))
		return 3
	}
	return 3
}

func (c *CPU) branch16(cond bool) int {
	offset := int16(c.fetchWord())
	if cond {
		c.PC = uint16(int32(c.PC) + int32(offset))
		return 6
	}
	return 5
}

// ALU Math Operations

func (c *CPU) add8(a, b byte) byte {
	sum := int(a) + int(b)
	res := byte(sum)
	c.updateNZ8(res)
	c.setCarry(sum > 0xFF)
	c.setOverflow(((a ^ res) & (b ^ res) & 0x80) != 0)
	// Half carry
	if ((a&0x0F)+(b&0x0F)) > 0x0F {
		c.CC |= FlagH
	} else {
		c.CC &^= FlagH
	}
	return res
}

func (c *CPU) adc8(a, b byte) byte {
	carryIn := 0
	if (c.CC & FlagC) != 0 {
		carryIn = 1
	}
	sum := int(a) + int(b) + carryIn
	res := byte(sum)
	c.updateNZ8(res)
	c.setCarry(sum > 0xFF)
	c.setOverflow(((a ^ res) & (b ^ res) & 0x80) != 0)
	if ((a&0x0F)+(b&0x0F)+byte(carryIn)) > 0x0F {
		c.CC |= FlagH
	} else {
		c.CC &^= FlagH
	}
	return res
}

func (c *CPU) sub8(a, b byte) byte {
	diff := int(a) - int(b)
	res := byte(diff)
	c.updateNZ8(res)
	c.setCarry(diff < 0)
	c.setOverflow(((a ^ b) & (a ^ res) & 0x80) != 0)
	return res
}

func (c *CPU) sbc8(a, b byte) byte {
	borrowIn := 0
	if (c.CC & FlagC) != 0 {
		borrowIn = 1
	}
	diff := int(a) - int(b) - borrowIn
	res := byte(diff)
	c.updateNZ8(res)
	c.setCarry(diff < 0)
	c.setOverflow(((a ^ b) & (a ^ res) & 0x80) != 0)
	return res
}

func (c *CPU) add16(a, b uint16) uint16 {
	sum := int32(a) + int32(b)
	res := uint16(sum)
	c.updateNZ16(res)
	c.setCarry(sum > 0xFFFF)
	c.setOverflow(((a ^ res) & (b ^ res) & 0x8000) != 0)
	return res
}

func (c *CPU) sub16(a, b uint16) uint16 {
	diff := int32(a) - int32(b)
	res := uint16(diff)
	c.updateNZ16(res)
	c.setCarry(diff < 0)
	c.setOverflow(((a ^ b) & (a ^ res) & 0x8000) != 0)
	return res
}

func (c *CPU) neg8(val byte) byte {
	res := byte(-int8(val))
	c.updateNZ8(res)
	c.setCarry(val != 0)
	c.setOverflow(val == 0x80)
	return res
}

func (c *CPU) com8(val byte) byte {
	res := ^val
	c.updateNZ8(res)
	c.CC &^= FlagV
	c.CC |= FlagC
	return res
}

func (c *CPU) lsr8(val byte) byte {
	c.setCarry((val & 0x01) != 0)
	res := val >> 1
	c.updateNZ8(res)
	return res
}

func (c *CPU) ror8(val byte) byte {
	carryIn := byte(0)
	if (c.CC & FlagC) != 0 {
		carryIn = 0x80
	}
	c.setCarry((val & 0x01) != 0)
	res := (val >> 1) | carryIn
	c.updateNZ8(res)
	return res
}

func (c *CPU) asr8(val byte) byte {
	c.setCarry((val & 0x01) != 0)
	res := byte(int8(val) >> 1)
	c.updateNZ8(res)
	return res
}

func (c *CPU) asl8(val byte) byte {
	c.setCarry((val & 0x80) != 0)
	res := val << 1
	c.updateNZ8(res)
	c.setOverflow(((val ^ res) & 0x80) != 0)
	return res
}

func (c *CPU) rol8(val byte) byte {
	carryIn := byte(0)
	if (c.CC & FlagC) != 0 {
		carryIn = 1
	}
	c.setCarry((val & 0x80) != 0)
	res := (val << 1) | carryIn
	c.updateNZ8(res)
	c.setOverflow(((val ^ res) & 0x80) != 0)
	return res
}

func (c *CPU) dec8(val byte) byte {
	res := val - 1
	c.updateNZ8(res)
	c.setOverflow(val == 0x80)
	return res
}

func (c *CPU) inc8(val byte) byte {
	res := val + 1
	c.updateNZ8(res)
	c.setOverflow(val == 0x7F)
	return res
}

func (c *CPU) tst8(val byte) {
	c.updateNZ8(val)
	c.CC &^= FlagV
}

func (c *CPU) daa() {
	cf := false
	var msn, lsn byte = (c.A & 0xF0) >> 4, c.A & 0x0F
	var adj byte = 0

	if (c.CC&FlagC) != 0 || msn > 9 || (msn > 8 && lsn > 9) {
		adj |= 0x60
		cf = true
	}
	if (c.CC&FlagH) != 0 || lsn > 9 {
		adj |= 0x06
	}
	c.A += adj
	c.updateNZ8(c.A)
	if cf {
		c.CC |= FlagC
	}
}

// TFR / EXG implementation

func (c *CPU) executeTFR(pb byte) {
	src := (pb >> 4) & 0x0F
	dst := pb & 0x0F

	if (src < 8 && dst < 8) || (src >= 8 && dst >= 8) {
		val := c.readReg(src)
		c.writeReg(dst, val)
	} else if src < 8 && dst >= 8 {
		// 16-bit to 8-bit (takes low 8 bits)
		val := c.readReg(src)
		c.writeReg(dst, val&0xFF)
	} else {
		// 8-bit to 16-bit (zero extended)
		val := c.readReg(src)
		c.writeReg(dst, val)
	}
}

func (c *CPU) executeEXG(pb byte) {
	r1 := (pb >> 4) & 0x0F
	r2 := pb & 0x0F

	v1 := c.readReg(r1)
	v2 := c.readReg(r2)
	c.writeReg(r1, v2)
	c.writeReg(r2, v1)
}

func (c *CPU) readReg(code byte) uint16 {
	switch code {
	case 0:
		return c.GetD()
	case 1:
		return c.X
	case 2:
		return c.Y
	case 3:
		return c.U
	case 4:
		return c.S
	case 5:
		return c.PC
	case 6:
		return c.GetW()
	case 7:
		return c.V
	case 8:
		return uint16(c.A)
	case 9:
		return uint16(c.B)
	case 10:
		return uint16(c.CC)
	case 11:
		return uint16(c.DP)
	case 14:
		return uint16(c.E)
	case 15:
		return uint16(c.F)
	default:
		return 0
	}
}

func (c *CPU) writeReg(code byte, val uint16) {
	switch code {
	case 0:
		c.SetD(val)
	case 1:
		c.X = val
	case 2:
		c.Y = val
	case 3:
		c.U = val
	case 4:
		c.S = val
	case 5:
		c.PC = val
	case 6:
		c.SetW(val)
	case 7:
		c.V = val
	case 8:
		c.A = byte(val)
	case 9:
		c.B = byte(val)
	case 10:
		c.CC = byte(val)
	case 11:
		c.DP = byte(val)
	case 14:
		c.E = byte(val)
	case 15:
		c.F = byte(val)
	}
}
