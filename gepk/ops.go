package gepk

import (
	"fmt"
	"os"

	"github.com/strickyak/hatvan-os/gepk/data"
)

func traceTrapEnabled() bool {
	return os.Getenv("HATVAN_TRACE_TRAP") != ""
}

func decodeMoveSize(ss uint16) (OpSize, bool) {
	switch ss {
	case 1: // 01
		return SizeByte, true
	case 3: // 11
		return SizeWord, true
	case 2: // 10
		return SizeLong, true
	default:
		return 0, false
	}
}

func decodeStandardSize(ss uint16) (OpSize, bool) {
	switch ss {
	case 0:
		return SizeByte, true
	case 1:
		return SizeWord, true
	case 2:
		return SizeLong, true
	default:
		return 0, false
	}
}

// opMove handles MOVE and MOVEA instructions.
func (c *CPU) opMove(op uint16, size OpSize) {
	dstReg := uint8((op >> 9) & 7)
	dstMode := uint8((op >> 6) & 7)
	srcMode := uint8((op >> 3) & 7)
	srcReg := uint8(op & 7)

	srcEA := c.ResolveEA(srcMode, srcReg, size)
	val := c.ReadEA(srcEA)
	c.CommitEA(srcEA)

	// If destination is An (mode 1), this is MOVEA
	if dstMode == 1 {
		if size == SizeWord {
			val = uint32(int32(int16(val)))
		}
		c.A[dstReg] = val
		return
	}

	dstEA := c.ResolveEA(dstMode, dstReg, size)
	c.WriteEA(dstEA, val)
	c.CommitEA(dstEA)

	// Set condition codes: N, Z set; V, C cleared; X unchanged
	c.setNZFlags(val, size)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
}

// opMoveQ handles MOVEQ #data, Dn
func (c *CPU) opMoveQ(op uint16) {
	reg := (op >> 9) & 7
	data := uint32(int32(int8(op & 0xFF)))
	c.D[reg] = data
	c.setNZFlags(data, SizeLong)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
}

// opMoveM handles MOVEM to/from memory.
func (c *CPU) opMoveM(op uint16) {
	dirMemToReg := (op & 0x0400) != 0 // 1 = mem to reg, 0 = reg to mem
	isLong := (op & 0x0040) != 0
	size := SizeWord
	delta := uint32(2)
	if isLong {
		size = SizeLong
		delta = 4
	}

	mode := uint8((op >> 3) & 7)
	reg := uint8(op & 7)
	mask := c.FetchInstructionWord()

	if dirMemToReg {
		// Memory to Registers
		if mode == 3 {
			// (An)+
			for i := 0; i < 16; i++ {
				if (mask & (1 << i)) != 0 {
					var val uint32
					if size == SizeWord {
						val = uint32(int32(int16(c.Bus.ReadWord(c.A[reg]))))
					} else {
						val = c.Bus.ReadLong(c.A[reg])
					}
					c.A[reg] += delta
					if i < 8 {
						c.D[i] = val
					} else {
						c.A[i-8] = val
					}
				}
			}
		} else {
			// Control modes
			ea := c.ResolveEA(mode, reg, size)
			addr := ea.Addr
			for i := 0; i < 16; i++ {
				if (mask & (1 << i)) != 0 {
					var val uint32
					if size == SizeWord {
						val = uint32(int32(int16(c.Bus.ReadWord(addr))))
					} else {
						val = c.Bus.ReadLong(addr)
					}
					addr += delta
					if i < 8 {
						c.D[i] = val
					} else {
						c.A[i-8] = val
					}
				}
			}
		}
	} else {
		// Registers to Memory
		if mode == 4 {
			// -(An) reverse bit order
			for i := 0; i < 16; i++ {
				if (mask & (1 << i)) != 0 {
					var val uint32
					if i < 8 {
						val = c.A[7-i]
					} else {
						val = c.D[15-i]
					}
					c.A[reg] -= delta
					if size == SizeWord {
						c.Bus.WriteWord(c.A[reg], uint16(val))
					} else {
						c.Bus.WriteLong(c.A[reg], val)
					}
				}
			}
		} else {
			// Control alterable modes
			ea := c.ResolveEA(mode, reg, size)
			addr := ea.Addr
			for i := 0; i < 16; i++ {
				if (mask & (1 << i)) != 0 {
					var val uint32
					if i < 8 {
						val = c.D[i]
					} else {
						val = c.A[i-8]
					}
					if size == SizeWord {
						c.Bus.WriteWord(addr, uint16(val))
					} else {
						c.Bus.WriteLong(addr, val)
					}
					addr += delta
				}
			}
		}
	}
}

// opMoveP handles MOVEP to/from peripherals.
func (c *CPU) opMoveP(op uint16) {
	dReg := (op >> 9) & 7
	aReg := op & 7
	memToReg := (op & 0x0080) == 0
	isLong := (op & 0x0040) != 0

	disp := int32(int16(c.FetchInstructionWord()))
	addr := uint32(int32(c.A[aReg]) + disp)

	if memToReg {
		if isLong {
			b0 := uint32(c.Bus.ReadByte(addr))
			b1 := uint32(c.Bus.ReadByte(addr + 2))
			b2 := uint32(c.Bus.ReadByte(addr + 4))
			b3 := uint32(c.Bus.ReadByte(addr + 6))
			c.D[dReg] = (b0 << 24) | (b1 << 16) | (b2 << 8) | b3
		} else {
			b0 := uint32(c.Bus.ReadByte(addr))
			b1 := uint32(c.Bus.ReadByte(addr + 2))
			c.D[dReg] = (c.D[dReg] & 0xFFFF0000) | (b0 << 8) | b1
		}
	} else {
		if isLong {
			val := c.D[dReg]
			c.Bus.WriteByte(addr, byte(val>>24))
			c.Bus.WriteByte(addr+2, byte(val>>16))
			c.Bus.WriteByte(addr+4, byte(val>>8))
			c.Bus.WriteByte(addr+6, byte(val))
		} else {
			val := c.D[dReg]
			c.Bus.WriteByte(addr, byte(val>>8))
			c.Bus.WriteByte(addr+2, byte(val))
		}
	}
}

// opLea handles LEA <ea>, An
func (c *CPU) opLea(op uint16) {
	aReg := (op >> 9) & 7
	mode := uint8((op >> 3) & 7)
	reg := uint8(op & 7)

	ea := c.ResolveEA(mode, reg, SizeLong)
	c.A[aReg] = ea.Addr
}

// opPea handles PEA <ea>
func (c *CPU) opPea(op uint16) {
	mode := uint8((op >> 3) & 7)
	reg := uint8(op & 7)

	ea := c.ResolveEA(mode, reg, SizeLong)
	c.PushLong(ea.Addr)
}

// opLink handles LINK An, #disp16
func (c *CPU) opLink(op uint16) {
	aReg := op & 7
	disp := int32(int16(c.FetchInstructionWord()))

	c.PushLong(c.A[aReg])
	c.A[aReg] = c.A[7]
	c.A[7] = uint32(int32(c.A[7]) + disp)
}

// opUnlk handles UNLK An
func (c *CPU) opUnlk(op uint16) {
	aReg := op & 7
	c.A[7] = c.A[aReg]
	c.A[aReg] = c.PopLong()
}

// opExt handles EXT.W and EXT.L
func (c *CPU) opExt(op uint16) {
	dReg := op & 7
	isLong := (op & 0x0040) != 0

	if isLong {
		val := int32(int16(c.D[dReg]))
		c.D[dReg] = uint32(val)
		c.setNZFlags(c.D[dReg], SizeLong)
	} else {
		val := uint32(uint16(int16(int8(c.D[dReg]))))
		c.D[dReg] = (c.D[dReg] & 0xFFFF0000) | val
		c.setNZFlags(val, SizeWord)
	}
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
}

// opSwap handles SWAP Dn
func (c *CPU) opSwap(op uint16) {
	dReg := op & 7
	c.D[dReg] = (c.D[dReg] >> 16) | (c.D[dReg] << 16)
	c.setNZFlags(c.D[dReg], SizeLong)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
}

// opBcc handles Bcc, BRA, and BSR.
func (c *CPU) opBcc(op uint16) {
	cond := uint8((op >> 8) & 0x0F)
	disp8 := int8(op & 0xFF)

	basePC := c.PC
	var disp int32
	if disp8 == 0 {
		disp = int32(int16(c.FetchInstructionWord()))
	} else {
		disp = int32(disp8)
	}

	if cond == 1 {
		// BSR: Push return address (current PC past displacement word)
		c.PushLong(c.PC)
		c.PC = uint32(int32(basePC) + disp)
		return
	}

	if cond == 0 || c.EvaluateCondition(cond) {
		// BRA or condition met
		c.PC = uint32(int32(basePC) + disp)
	}
}

// opDbcc handles DBcc Dn, disp16
func (c *CPU) opDbcc(op uint16) {
	cond := uint8((op >> 8) & 0x0F)
	dReg := op & 7

	basePC := c.PC
	disp := int32(int16(c.FetchInstructionWord()))

	if c.EvaluateCondition(cond) {
		// Condition true: loop terminates
		return
	}

	// Condition false: decrement low 16 bits of Dn
	cur := int16(c.D[dReg]) - 1
	c.D[dReg] = (c.D[dReg] & 0xFFFF0000) | uint32(uint16(cur))

	if cur != -1 {
		// Loop not exhausted: branch
		c.PC = uint32(int32(basePC) + disp)
	}
}

// opScc handles Scc <ea>
func (c *CPU) opScc(op uint16) {
	cond := uint8((op >> 8) & 0x0F)
	mode := uint8((op >> 3) & 7)
	reg := uint8(op & 7)

	val := byte(0x00)
	if c.EvaluateCondition(cond) {
		val = 0xFF
	}

	ea := c.ResolveEA(mode, reg, SizeByte)
	c.WriteEA(ea, uint32(val))
	c.CommitEA(ea)
}

// opJmp handles JMP <ea>
func (c *CPU) opJmp(op uint16) {
	mode := uint8((op >> 3) & 7)
	reg := uint8(op & 7)

	ea := c.ResolveEA(mode, reg, SizeLong)
	c.PC = ea.Addr
}

// opJsr handles JSR <ea>
func (c *CPU) opJsr(op uint16) {
	mode := uint8((op >> 3) & 7)
	reg := uint8(op & 7)

	ea := c.ResolveEA(mode, reg, SizeLong)
	c.PushLong(c.PC)
	c.PC = ea.Addr
}

// opRts handles RTS
func (c *CPU) opRts() {
	c.PC = c.PopLong()
}

// opRtr handles RTR
func (c *CPU) opRtr() {
	c.SetCCR(byte(c.PopWord()))
	c.PC = c.PopLong()
}

// opRte handles RTE (privileged)
func (c *CPU) opRte() {
	if (c.SR & FlagS) == 0 {
		c.TriggerException(VecPrivilegeViol)
		return
	}
	newSR := c.PopWord()
	newPC := c.PopLong()
	c.SetSR(newSR)
	c.PC = newPC
	if traceTrapEnabled() && (newSR&FlagS) == 0 && c.Bus.TaskReg >= 2 {
		if pt := c.PendingTraps[c.Bus.TaskReg]; pt != nil {
			c.PendingTraps[c.Bus.TaskReg] = nil
			resStr := data.FormatResult(pt.Call, pt.CallNum, c.SR, c.D[0], c.D[1], c.D[2], c.A[0], c.A[1], pt.BufAddr, func(addr uint32, maxLen int) string {
				return c.ReadUserPreview(c.Bus.TaskReg, addr, maxLen)
			})
			fmt.Fprintf(os.Stderr, "    <== [PID %d] %s\n", c.Bus.TaskReg, resStr)
		}
	}
}

// opTrap handles TRAP #vector
func (c *CPU) opTrap(op uint16) {
	vec := uint8(op & 0x0F)
	if vec == 0 && traceTrapEnabled() && (c.SR&FlagS) == 0 && c.Bus.TaskReg >= 2 {
		callNum := byte(c.D[0] & 0xFF)
		var bufStr string
		if callNum == 0x8C || callNum == 0x8A {
			n := int(c.D[2])
			if n > 32 {
				n = 32
			}
			for i := 0; i < n; i++ {
				b := c.Bus.ReadUserByte(c.Bus.TaskReg, c.A[0]+uint32(i))
				bufStr += fmt.Sprintf(" %02X", b)
			}
		}
		fmt.Fprintf(os.Stderr, "[PID %d PC=%06X TRAP #0: $%02X (D1=%08X A0=%08X D2=%08X A1=%08X):%s]\n",
			c.Bus.TaskReg, c.PC-2, callNum, c.D[1], c.A[0], c.D[2], c.A[1], bufStr)
		call := data.FindCall(callNum)
		pretty := data.FormatCall(call, callNum, c.D[1], c.D[2], c.A[0], c.A[1], func(addr uint32) string {
			return c.ReadUserString(c.Bus.TaskReg, addr)
		}, func(addr uint32, maxLen int) string {
			return c.ReadUserPreview(c.Bus.TaskReg, addr, maxLen)
		})
		fmt.Fprintf(os.Stderr, "    ==> %s\n", pretty)
		if callNum != 0x06 {
			c.PendingTraps[c.Bus.TaskReg] = &PendingTrap{
				CallNum: callNum,
				Call:    call,
				Task:    c.Bus.TaskReg,
				PC:      c.PC - 2,
				BufAddr: c.A[0],
			}
		} else {
			c.PendingTraps[c.Bus.TaskReg] = nil
		}
	}
	c.TriggerException(VecTrapBase + vec)
}

// opChk handles CHK <ea>, Dn
func (c *CPU) opChk(op uint16) {
	dReg := (op >> 9) & 7
	mode := uint8((op >> 3) & 7)
	reg := uint8(op & 7)

	ea := c.ResolveEA(mode, reg, SizeWord)
	bound := int16(c.ReadEA(ea))
	c.CommitEA(ea)

	val := int16(c.D[dReg])
	if val < 0 {
		c.setFlag(FlagN, true)
		c.TriggerException(VecCHK)
	} else if val > bound {
		c.setFlag(FlagN, false)
		c.TriggerException(VecCHK)
	}
}

// opTrapv handles TRAPV
func (c *CPU) opTrapv() {
	if (c.SR & FlagV) != 0 {
		c.TriggerException(VecTRAPV)
	}
}

// opTas handles TAS <ea>
func (c *CPU) opTas(op uint16) {
	mode := uint8((op >> 3) & 7)
	reg := uint8(op & 7)

	ea := c.ResolveEA(mode, reg, SizeByte)
	val := c.ReadEA(ea)
	c.setNZFlags(val, SizeByte)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)

	val |= 0x80
	c.WriteEA(ea, val)
	c.CommitEA(ea)
}

// opExg handles EXG Rx, Ry
func (c *CPU) opExg(op uint16) {
	rx := (op >> 9) & 7
	ry := op & 7
	mode := (op >> 3) & 0x1F

	switch mode {
	case 0x08: // EXG Dx, Dy
		c.D[rx], c.D[ry] = c.D[ry], c.D[rx]
	case 0x09: // EXG Ax, Ay
		c.A[rx], c.A[ry] = c.A[ry], c.A[rx]
	case 0x11: // EXG Dx, Ay
		c.D[rx], c.A[ry] = c.A[ry], c.D[rx]
	default:
		c.TriggerException(VecIllegalInstr)
	}
}

// opShiftReg handles register shifts and rotates.
func (c *CPU) opShiftReg(op uint16) {
	cntReg := (op >> 9) & 7
	isLeft := (op & 0x0100) != 0
	sizeBits := (op >> 6) & 3
	size, ok := decodeStandardSize(sizeBits)
	if !ok {
		c.TriggerException(VecIllegalInstr)
		return
	}
	isRegCount := (op & 0x0020) != 0
	shiftType := (op >> 3) & 3
	dReg := op & 7

	var count int
	if isRegCount {
		count = int(c.D[cntReg] & 63)
	} else {
		count = int(cntReg)
		if count == 0 {
			count = 8
		}
	}

	val := c.D[dReg]
	var res uint32

	switch shiftType {
	case 0: // ASL / ASR
		if isLeft {
			res = c.Asl(val, count, size)
		} else {
			res = c.Asr(val, count, size)
		}
	case 1: // LSL / LSR
		if isLeft {
			res = c.Lsl(val, count, size)
		} else {
			res = c.Lsr(val, count, size)
		}
	case 2: // ROXL / ROXR
		if isLeft {
			res = c.Roxl(val, count, size)
		} else {
			res = c.Roxr(val, count, size)
		}
	case 3: // ROL / ROR
		if isLeft {
			res = c.Rol(val, count, size)
		} else {
			res = c.Ror(val, count, size)
		}
	}

	mask := maskForSize(size)
	c.D[dReg] = (c.D[dReg] &^ mask) | (res & mask)
}

// opShiftMem handles memory shifts and rotates (always 1-bit, word size).
func (c *CPU) opShiftMem(op uint16) {
	shiftType := (op >> 9) & 3
	isLeft := (op & 0x0100) != 0
	mode := uint8((op >> 3) & 7)
	reg := uint8(op & 7)

	ea := c.ResolveEA(mode, reg, SizeWord)
	val := c.ReadEA(ea)
	var res uint32

	switch shiftType {
	case 0: // ASL / ASR
		if isLeft {
			res = c.Asl(val, 1, SizeWord)
		} else {
			res = c.Asr(val, 1, SizeWord)
		}
	case 1: // LSL / LSR
		if isLeft {
			res = c.Lsl(val, 1, SizeWord)
		} else {
			res = c.Lsr(val, 1, SizeWord)
		}
	case 2: // ROXL / ROXR
		if isLeft {
			res = c.Roxl(val, 1, SizeWord)
		} else {
			res = c.Roxr(val, 1, SizeWord)
		}
	case 3: // ROL / ROR
		if isLeft {
			res = c.Rol(val, 1, SizeWord)
		} else {
			res = c.Ror(val, 1, SizeWord)
		}
	}

	c.WriteEA(ea, res)
	c.CommitEA(ea)
}

// opBit handles BTST, BCHG, BCLR, and BSET.
func (c *CPU) opBit(op uint16, isStatic bool) {
	var bitNum uint32
	var mode, reg uint8

	if isStatic {
		bitNum = uint32(c.FetchInstructionWord() & 0xFF)
		mode = uint8((op >> 3) & 7)
		reg = uint8(op & 7)
	} else {
		dReg := (op >> 9) & 7
		bitNum = c.D[dReg]
		mode = uint8((op >> 3) & 7)
		reg = uint8(op & 7)
	}

	size := SizeByte
	if mode == 0 {
		size = SizeLong
		bitNum &= 31
	} else {
		bitNum &= 7
	}

	opType := (op >> 6) & 3
	ea := c.ResolveEA(mode, reg, size)
	val := c.ReadEA(ea)

	// Test bit
	isBitZero := (val & (1 << bitNum)) == 0
	c.setFlag(FlagZ, isBitZero)

	switch opType {
	case 0: // BTST (no writeback)
		c.CommitEA(ea)
		return
	case 1: // BCHG
		val ^= (1 << bitNum)
	case 2: // BCLR
		val &^= (1 << bitNum)
	case 3: // BSET
		val |= (1 << bitNum)
	}

	c.WriteEA(ea, val)
	c.CommitEA(ea)
}

// opStop handles STOP #imm16 (privileged)
func (c *CPU) opStop() {
	if (c.SR & FlagS) == 0 {
		c.TriggerException(VecPrivilegeViol)
		return
	}
	newSR := c.FetchInstructionWord()
	c.SetSR(newSR)
	c.Stopped = true
}
