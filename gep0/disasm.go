package gep0

import "fmt"

var (
	reg8Names  = [8]string{"AL", "CL", "DL", "BL", "AH", "CH", "DH", "BH"}
	reg16Names = [8]string{"AX", "CX", "DX", "BX", "SP", "BP", "SI", "DI"}
	sregNames  = [4]string{"ES", "CS", "SS", "DS"}
	aluNames   = [8]string{"ADD", "OR", "ADC", "SBB", "AND", "SUB", "XOR", "CMP"}
	shiftNames = [8]string{"ROL", "ROR", "RCL", "RCR", "SHL", "SHR", "SAL", "SAR"}
	jccNames   = [16]string{"JO", "JNO", "JB", "JAE", "JE", "JNE", "JBE", "JA", "JS", "JNS", "JP", "JNP", "JL", "JGE", "JLE", "JG"}
	eaBaseStr  = [8]string{"BX+SI", "BX+DI", "BP+SI", "BP+DI", "SI", "DI", "BP", "BX"}
)

// Disasm disassembles one 8086 instruction at offset ip using readByte(ip+k).
func Disasm(ip uint16, readByte func(uint16) byte) (string, int) {
	pos := ip
	next8 := func() byte {
		b := readByte(pos)
		pos++
		return b
	}
	next16 := func() uint16 {
		lo := uint16(next8())
		hi := uint16(next8())
		return (hi << 8) | lo
	}

	prefix := ""
	segPref := ""
	var op byte
	for {
		op = next8()
		switch op {
		case 0x26:
			segPref = "ES:"
			continue
		case 0x2E:
			segPref = "CS:"
			continue
		case 0x36:
			segPref = "SS:"
			continue
		case 0x3E:
			segPref = "DS:"
			continue
		case 0xF0:
			prefix += "LOCK "
			continue
		case 0xF2:
			prefix += "REPNE "
			continue
		case 0xF3:
			prefix += "REP "
			continue
		}
		break
	}

	formatRM := func(mod, rm byte, isWord bool) string {
		if mod == 3 {
			if isWord {
				return reg16Names[rm&7]
			}
			return reg8Names[rm&7]
		}
		if mod == 0 && rm == 6 {
			disp16 := next16()
			return fmt.Sprintf("%s[$%04X]", segPref, disp16)
		}
		base := eaBaseStr[rm&7]
		switch mod {
		case 0:
			return fmt.Sprintf("%s[%s]", segPref, base)
		case 1:
			disp8 := int8(next8())
			if disp8 < 0 {
				return fmt.Sprintf("%s[%s-%d]", segPref, base, -int(disp8))
			}
			return fmt.Sprintf("%s[%s+%d]", segPref, base, int(disp8))
		case 2:
			disp16 := next16()
			return fmt.Sprintf("%s[%s+$%04X]", segPref, base, disp16)
		}
		return ""
	}

	if op < 0x40 {
		aluOp := (op >> 3) & 7
		sub := op & 7
		switch sub {
		case 0, 1, 2, 3:
			isWord := (sub & 1) != 0
			modrm := next8()
			mod, reg, rm := (modrm>>6)&3, (modrm>>3)&7, modrm&7
			rmStr := formatRM(mod, rm, isWord)
			rStr := reg8Names[reg]
			if isWord {
				rStr = reg16Names[reg]
			}
			if (sub & 2) == 0 {
				return fmt.Sprintf("%s%s %s, %s", prefix, aluNames[aluOp], rmStr, rStr), int(pos - ip)
			}
			return fmt.Sprintf("%s%s %s, %s", prefix, aluNames[aluOp], rStr, rmStr), int(pos - ip)
		case 4:
			return fmt.Sprintf("%s%s AL, $%02X", prefix, aluNames[aluOp], next8()), int(pos - ip)
		case 5:
			return fmt.Sprintf("%s%s AX, $%04X", prefix, aluNames[aluOp], next16()), int(pos - ip)
		case 6:
			if aluOp < 4 {
				return fmt.Sprintf("%sPUSH %s", prefix, sregNames[aluOp]), int(pos - ip)
			}
		case 7:
			if aluOp < 4 {
				return fmt.Sprintf("%sPOP %s", prefix, sregNames[aluOp]), int(pos - ip)
			}
		}
	}

	if op >= 0x40 && op <= 0x47 {
		return fmt.Sprintf("%sINC %s", prefix, reg16Names[op&7]), int(pos - ip)
	}
	if op >= 0x48 && op <= 0x4F {
		return fmt.Sprintf("%sDEC %s", prefix, reg16Names[op&7]), int(pos - ip)
	}
	if op >= 0x50 && op <= 0x57 {
		return fmt.Sprintf("%sPUSH %s", prefix, reg16Names[op&7]), int(pos - ip)
	}
	if op >= 0x58 && op <= 0x5F {
		return fmt.Sprintf("%sPOP %s", prefix, reg16Names[op&7]), int(pos - ip)
	}
	if op >= 0x70 && op <= 0x7F {
		rel := int16(int8(next8()))
		target := uint16(int16(pos) + rel)
		return fmt.Sprintf("%s%s $%04X", prefix, jccNames[op&0x0F], target), int(pos - ip)
	}
	if op == 0x90 {
		return prefix + "NOP", int(pos - ip)
	}
	if op >= 0x91 && op <= 0x97 {
		return fmt.Sprintf("%sXCHG AX, %s", prefix, reg16Names[op&7]), int(pos - ip)
	}
	if op >= 0xB0 && op <= 0xB7 {
		return fmt.Sprintf("%sMOV %s, $%02X", prefix, reg8Names[op&7], next8()), int(pos - ip)
	}
	if op >= 0xB8 && op <= 0xBF {
		return fmt.Sprintf("%sMOV %s, $%04X", prefix, reg16Names[op&7], next16()), int(pos - ip)
	}

	switch op {
	case 0x80, 0x82:
		modrm := next8()
		mod, reg, rm := (modrm>>6)&3, (modrm>>3)&7, modrm&7
		rmStr := formatRM(mod, rm, false)
		imm := next8()
		return fmt.Sprintf("%s%s %s, $%02X", prefix, aluNames[reg], rmStr, imm), int(pos - ip)
	case 0x81:
		modrm := next8()
		mod, reg, rm := (modrm>>6)&3, (modrm>>3)&7, modrm&7
		rmStr := formatRM(mod, rm, true)
		imm := next16()
		return fmt.Sprintf("%s%s %s, $%04X", prefix, aluNames[reg], rmStr, imm), int(pos - ip)
	case 0x83:
		modrm := next8()
		mod, reg, rm := (modrm>>6)&3, (modrm>>3)&7, modrm&7
		rmStr := formatRM(mod, rm, true)
		imm := int8(next8())
		return fmt.Sprintf("%s%s %s, %d", prefix, aluNames[reg], rmStr, imm), int(pos - ip)
	case 0x88, 0x89, 0x8A, 0x8B:
		isWord := (op & 1) != 0
		modrm := next8()
		mod, reg, rm := (modrm>>6)&3, (modrm>>3)&7, modrm&7
		rmStr := formatRM(mod, rm, isWord)
		rStr := reg8Names[reg]
		if isWord {
			rStr = reg16Names[reg]
		}
		if (op & 2) == 0 {
			return fmt.Sprintf("%sMOV %s, %s", prefix, rmStr, rStr), int(pos - ip)
		}
		return fmt.Sprintf("%sMOV %s, %s", prefix, rStr, rmStr), int(pos - ip)
	case 0x8D:
		modrm := next8()
		mod, reg, rm := (modrm>>6)&3, (modrm>>3)&7, modrm&7
		rmStr := formatRM(mod, rm, true)
		return fmt.Sprintf("%sLEA %s, %s", prefix, reg16Names[reg], rmStr), int(pos - ip)
	case 0xC2:
		return fmt.Sprintf("%sRET $%04X", prefix, next16()), int(pos - ip)
	case 0xC3:
		return prefix + "RET", int(pos - ip)
	case 0xC6:
		modrm := next8()
		mod, _, rm := (modrm>>6)&3, (modrm>>3)&7, modrm&7
		rmStr := formatRM(mod, rm, false)
		imm := next8()
		return fmt.Sprintf("%sMOV BYTE %s, $%02X", prefix, rmStr, imm), int(pos - ip)
	case 0xC7:
		modrm := next8()
		mod, _, rm := (modrm>>6)&3, (modrm>>3)&7, modrm&7
		rmStr := formatRM(mod, rm, true)
		imm := next16()
		return fmt.Sprintf("%sMOV WORD %s, $%04X", prefix, rmStr, imm), int(pos - ip)
	case 0xCD:
		vec := next8()
		if vec == 0x30 {
			callNum := next8()
			return fmt.Sprintf("%sINT $30 ; call=$%02X", prefix, callNum), int(pos - ip)
		}
		return fmt.Sprintf("%sINT $%02X", prefix, vec), int(pos - ip)
	case 0xCF:
		return prefix + "IRET", int(pos - ip)
	case 0xE8:
		rel := int16(next16())
		target := uint16(int16(pos) + rel)
		return fmt.Sprintf("%sCALL $%04X", prefix, target), int(pos - ip)
	case 0xE9:
		rel := int16(next16())
		target := uint16(int16(pos) + rel)
		return fmt.Sprintf("%sJMP $%04X", prefix, target), int(pos - ip)
	case 0xEB:
		rel := int16(int8(next8()))
		target := uint16(int16(pos) + rel)
		return fmt.Sprintf("%sJMP SHORT $%04X", prefix, target), int(pos - ip)
	case 0xF4:
		return prefix + "HLT", int(pos - ip)
	}

	return fmt.Sprintf("%sDB $%02X", prefix, op), int(pos - ip)
}
