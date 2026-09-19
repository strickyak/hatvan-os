package gepk

import (
	"fmt"
)

// Disassemble attempts to decode and format an instruction at addr.
// Returns the disassembled string and the total instruction size in bytes.
func (c *CPU) Disassemble(addr uint32) (string, uint32) {
	op := c.Bus.ReadWord(addr)
	var length uint32 = 2

	readExtWord := func() uint16 {
		w := c.Bus.ReadWord(addr + length)
		length += 2
		return w
	}

	readExtLong := func() uint32 {
		hi := uint32(readExtWord())
		lo := uint32(readExtWord())
		return (hi << 16) | lo
	}

	disasmEA := func(mode, reg uint8, size OpSize) string {
		switch mode {
		case 0:
			return fmt.Sprintf("D%d", reg)
		case 1:
			return fmt.Sprintf("A%d", reg)
		case 2:
			return fmt.Sprintf("(A%d)", reg)
		case 3:
			return fmt.Sprintf("(A%d)+", reg)
		case 4:
			return fmt.Sprintf("-(A%d)", reg)
		case 5:
			disp := int16(readExtWord())
			return fmt.Sprintf("(%d, A%d)", disp, reg)
		case 6:
			ext := readExtWord()
			d8 := int8(ext & 0xFF)
			idxReg := (ext >> 12) & 7
			isA := (ext & 0x8000) != 0
			isL := (ext & 0x0800) != 0
			rName := fmt.Sprintf("D%d", idxReg)
			if isA {
				rName = fmt.Sprintf("A%d", idxReg)
			}
			sz := ".W"
			if isL {
				sz = ".L"
			}
			return fmt.Sprintf("(%d, A%d, %s%s)", d8, reg, rName, sz)
		case 7:
			switch reg {
			case 0:
				w := readExtWord()
				return fmt.Sprintf("($%04X).W", w)
			case 1:
				l := readExtLong()
				return fmt.Sprintf("($%08X).L", l)
			case 2:
				disp := int16(readExtWord())
				target := uint32(int32(addr+2) + int32(disp))
				return fmt.Sprintf("(%d, PC) [=$%06X]", disp, target)
			case 3:
				ext := readExtWord()
				d8 := int8(ext & 0xFF)
				return fmt.Sprintf("(%d, PC, ...) [rel]", d8)
			case 4:
				switch size {
				case SizeByte:
					b := byte(readExtWord())
					return fmt.Sprintf("#$%02X", b)
				case SizeWord:
					w := readExtWord()
					return fmt.Sprintf("#$%04X", w)
				case SizeLong:
					l := readExtLong()
					return fmt.Sprintf("#$%08X", l)
				}
			}
		}
		return fmt.Sprintf("???(m=%d,r=%d)", mode, reg)
	}

	// 1. Branches (Group 6)
	if (op >> 12) == 6 {
		cond := (op >> 8) & 0x0F
		condNames := []string{"BRA", "BSR", "BHI", "BLS", "BCC", "BCS", "BNE", "BEQ",
			"BVC", "BVS", "BPL", "BMI", "BGE", "BLT", "BGT", "BLE"}
		disp8 := int8(op & 0xFF)
		var target uint32
		if disp8 == 0 {
			disp16 := int16(readExtWord())
			target = uint32(int32(addr+2) + int32(disp16))
			return fmt.Sprintf("%s.W $%06X", condNames[cond], target), length
		}
		target = uint32(int32(addr+2) + int32(disp8))
		return fmt.Sprintf("%s.S $%06X", condNames[cond], target), length
	}

	// 2. MOVEQ (Group 7)
	if (op >> 12) == 7 && (op&0x0100) == 0 {
		reg := (op >> 9) & 7
		data := int8(op & 0xFF)
		return fmt.Sprintf("MOVEQ #%d, D%d", data, reg), length
	}

	// 3. MOVE
	group := op >> 12
	if group == 1 || group == 2 || group == 3 {
		var szStr string
		var size OpSize
		switch group {
		case 1:
			szStr = ".B"
			size = SizeByte
		case 3:
			szStr = ".W"
			size = SizeWord
		case 2:
			szStr = ".L"
			size = SizeLong
		}

		dstReg := uint8((op >> 9) & 7)
		dstMode := uint8((op >> 6) & 7)
		srcMode := uint8((op >> 3) & 7)
		srcReg := uint8(op & 7)

		srcStr := disasmEA(srcMode, srcReg, size)
		dstStr := disasmEA(dstMode, dstReg, size)

		mnem := "MOVE"
		if dstMode == 1 {
			mnem = "MOVEA"
		}
		return fmt.Sprintf("%s%s %s, %s", mnem, szStr, srcStr, dstStr), length
	}

	// 4. Common single instructions
	switch op {
	case 0x4E70:
		return "RESET", length
	case 0x4E71:
		return "NOP", length
	case 0x4E72:
		w := readExtWord()
		return fmt.Sprintf("STOP #$%04X", w), length
	case 0x4E73:
		return "RTE", length
	case 0x4E75:
		return "RTS", length
	case 0x4E76:
		return "TRAPV", length
	case 0x4E77:
		return "RTR", length
	case 0x4AFC:
		return "ILLEGAL", length
	}

	if (op & 0xFFF0) == 0x4E40 {
		return fmt.Sprintf("TRAP #%d", op&0x0F), length
	}

	if (op & 0xFFC0) == 0x4E80 {
		mode := uint8((op >> 3) & 7)
		reg := uint8(op & 7)
		return fmt.Sprintf("JSR %s", disasmEA(mode, reg, SizeLong)), length
	}
	if (op & 0xFFC0) == 0x4EC0 {
		mode := uint8((op >> 3) & 7)
		reg := uint8(op & 7)
		return fmt.Sprintf("JMP %s", disasmEA(mode, reg, SizeLong)), length
	}

	// Default fallback
	return fmt.Sprintf("DC.W $%04X", op), length
}
