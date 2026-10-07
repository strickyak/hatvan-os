package gepc

import "fmt"

// Disasm disassembles a single instruction at pc and returns symbolic text and instruction byte length.
func Disasm(pc uint16, readByte func(uint16) byte) (string, int) {
	op := readByte(pc)
	hi := op >> 4
	lo := op & 0x0F

	switch hi {
	case 0x0:
		if lo == 0 {
			return "IDL", 1
		}
		return fmt.Sprintf("LDN R%X", lo), 1

	case 0x1:
		return fmt.Sprintf("INC R%X", lo), 1

	case 0x2:
		return fmt.Sprintf("DEC R%X", lo), 1

	case 0x3:
		if op == 0x38 {
			return "SKP", 1
		}
		targetLo := readByte(pc + 1)
		// When short branch executes, PC has already fetched opcode and target byte
		target := ((pc + 2) & 0xFF00) | uint16(targetLo)
		names := map[byte]string{
			0x30: "BR",
			0x31: "BQ",
			0x32: "BZ",
			0x33: "BDF",
			0x34: "B1",
			0x35: "B2",
			0x36: "B3",
			0x37: "B4",
			0x39: "BNQ",
			0x3A: "BNZ",
			0x3B: "BNF",
			0x3C: "BN1",
			0x3D: "BN2",
			0x3E: "BN3",
			0x3F: "BN4",
		}
		return fmt.Sprintf("%-4s $%04X", names[op], target), 2

	case 0x4:
		return fmt.Sprintf("LDA R%X", lo), 1

	case 0x5:
		return fmt.Sprintf("STR R%X", lo), 1

	case 0x6:
		if op == 0x60 {
			return "IRX", 1
		} else if op >= 0x61 && op <= 0x67 {
			return fmt.Sprintf("OUT %d", lo), 1
		} else if op == 0x68 {
			return "NOP", 1
		}
		return fmt.Sprintf("INP %d", lo&7), 1

	case 0x7:
		switch op {
		case 0x70:
			return "RET", 1
		case 0x71:
			return "DIS", 1
		case 0x72:
			return "LDXA", 1
		case 0x73:
			return "STXD", 1
		case 0x74:
			return "ADC", 1
		case 0x75:
			return "SDB", 1
		case 0x76:
			return "SHRC", 1
		case 0x77:
			return "SMB", 1
		case 0x78:
			return "SAV", 1
		case 0x79:
			return "MARK", 1
		case 0x7A:
			return "REQ", 1
		case 0x7B:
			return "SEQ", 1
		case 0x7C:
			return fmt.Sprintf("ADCI $%02X", readByte(pc+1)), 2
		case 0x7D:
			return fmt.Sprintf("SDBI $%02X", readByte(pc+1)), 2
		case 0x7E:
			return "SHLC", 1
		case 0x7F:
			return fmt.Sprintf("SMBI $%02X", readByte(pc+1)), 2
		}

	case 0x8:
		return fmt.Sprintf("GLO R%X", lo), 1

	case 0x9:
		return fmt.Sprintf("GHI R%X", lo), 1

	case 0xA:
		return fmt.Sprintf("PLO R%X", lo), 1

	case 0xB:
		return fmt.Sprintf("PHI R%X", lo), 1

	case 0xC:
		switch op {
		case 0xC0, 0xC1, 0xC2, 0xC3, 0xC9, 0xCA, 0xCB:
			target := (uint16(readByte(pc+1)) << 8) | uint16(readByte(pc+2))
			names := map[byte]string{
				0xC0: "LBR",
				0xC1: "LBQ",
				0xC2: "LBZ",
				0xC3: "LBDF",
				0xC9: "LBNQ",
				0xCA: "LBNZ",
				0xCB: "LBNF",
			}
			return fmt.Sprintf("%-4s $%04X", names[op], target), 3
		case 0xC4:
			return "NOP", 1
		case 0xC5:
			return "LSNQ", 1
		case 0xC6:
			return "LSNZ", 1
		case 0xC7:
			return "LSNF", 1
		case 0xC8:
			return "LSKP", 1
		case 0xCC:
			return "LSIE", 1
		case 0xCD:
			return "LSQ", 1
		case 0xCE:
			return "LSZ", 1
		case 0xCF:
			return "LSDF", 1
		}

	case 0xD:
		return fmt.Sprintf("SEP R%X", lo), 1

	case 0xE:
		return fmt.Sprintf("SEX R%X", lo), 1

	case 0xF:
		switch op {
		case 0xF0:
			return "LDX", 1
		case 0xF1:
			return "OR", 1
		case 0xF2:
			return "AND", 1
		case 0xF3:
			return "XOR", 1
		case 0xF4:
			return "ADD", 1
		case 0xF5:
			return "SD", 1
		case 0xF6:
			return "SHR", 1
		case 0xF7:
			return "SM", 1
		case 0xF8:
			return fmt.Sprintf("LDI  $%02X", readByte(pc+1)), 2
		case 0xF9:
			return fmt.Sprintf("ORI  $%02X", readByte(pc+1)), 2
		case 0xFA:
			return fmt.Sprintf("ANI  $%02X", readByte(pc+1)), 2
		case 0xFB:
			return fmt.Sprintf("XRI  $%02X", readByte(pc+1)), 2
		case 0xFC:
			return fmt.Sprintf("ADI  $%02X", readByte(pc+1)), 2
		case 0xFD:
			return fmt.Sprintf("SDI  $%02X", readByte(pc+1)), 2
		case 0xFE:
			return "SHL", 1
		case 0xFF:
			return fmt.Sprintf("SMI  $%02X", readByte(pc+1)), 2
		}
	}

	return fmt.Sprintf(".DB  $%02X", op), 1
}
