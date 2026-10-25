package gep5

import "fmt"

type addrMode int

const (
	modeImp addrMode = iota
	modeAcc
	modeImm
	modeZP
	modeZPX
	modeZPY
	modeAbs
	modeAbsX
	modeAbsY
	modeInd
	modeIndX
	modeIndY
	modeRel
)

type opInfo struct {
	mnemonic string
	mode     addrMode
}

var disasmTable = map[byte]opInfo{
	0x00: {"BRK", modeImm},
	0x01: {"ORA", modeIndX},
	0x05: {"ORA", modeZP},
	0x06: {"ASL", modeZP},
	0x08: {"PHP", modeImp},
	0x09: {"ORA", modeImm},
	0x0A: {"ASL", modeAcc},
	0x0D: {"ORA", modeAbs},
	0x0E: {"ASL", modeAbs},
	0x10: {"BPL", modeRel},
	0x11: {"ORA", modeIndY},
	0x15: {"ORA", modeZPX},
	0x16: {"ASL", modeZPX},
	0x18: {"CLC", modeImp},
	0x19: {"ORA", modeAbsY},
	0x1D: {"ORA", modeAbsX},
	0x1E: {"ASL", modeAbsX},
	0x20: {"JSR", modeAbs},
	0x21: {"AND", modeIndX},
	0x24: {"BIT", modeZP},
	0x25: {"AND", modeZP},
	0x26: {"ROL", modeZP},
	0x28: {"PLP", modeImp},
	0x29: {"AND", modeImm},
	0x2A: {"ROL", modeAcc},
	0x2C: {"BIT", modeAbs},
	0x2D: {"AND", modeAbs},
	0x2E: {"ROL", modeAbs},
	0x30: {"BMI", modeRel},
	0x31: {"AND", modeIndY},
	0x35: {"AND", modeZPX},
	0x36: {"ROL", modeZPX},
	0x38: {"SEC", modeImp},
	0x39: {"AND", modeAbsY},
	0x3D: {"AND", modeAbsX},
	0x3E: {"ROL", modeAbsX},
	0x40: {"RTI", modeImp},
	0x41: {"EOR", modeIndX},
	0x45: {"EOR", modeZP},
	0x46: {"LSR", modeZP},
	0x48: {"PHA", modeImp},
	0x49: {"EOR", modeImm},
	0x4A: {"LSR", modeAcc},
	0x4C: {"JMP", modeAbs},
	0x4D: {"EOR", modeAbs},
	0x4E: {"LSR", modeAbs},
	0x50: {"BVC", modeRel},
	0x51: {"EOR", modeIndY},
	0x55: {"EOR", modeZPX},
	0x56: {"LSR", modeZPX},
	0x58: {"CLI", modeImp},
	0x59: {"EOR", modeAbsY},
	0x5D: {"EOR", modeAbsX},
	0x5E: {"LSR", modeAbsX},
	0x60: {"RTS", modeImp},
	0x61: {"ADC", modeIndX},
	0x65: {"ADC", modeZP},
	0x66: {"ROR", modeZP},
	0x68: {"PLA", modeImp},
	0x69: {"ADC", modeImm},
	0x6A: {"ROR", modeAcc},
	0x6C: {"JMP", modeInd},
	0x6D: {"ADC", modeAbs},
	0x6E: {"ROR", modeAbs},
	0x70: {"BVS", modeRel},
	0x71: {"ADC", modeIndY},
	0x75: {"ADC", modeZPX},
	0x76: {"ROR", modeZPX},
	0x78: {"SEI", modeImp},
	0x79: {"ADC", modeAbsY},
	0x7D: {"ADC", modeAbsX},
	0x7E: {"ROR", modeAbsX},
	0x81: {"STA", modeIndX},
	0x84: {"STY", modeZP},
	0x85: {"STA", modeZP},
	0x86: {"STX", modeZP},
	0x88: {"DEY", modeImp},
	0x8A: {"TXA", modeImp},
	0x8C: {"STY", modeAbs},
	0x8D: {"STA", modeAbs},
	0x8E: {"STX", modeAbs},
	0x90: {"BCC", modeRel},
	0x91: {"STA", modeIndY},
	0x94: {"STY", modeZPX},
	0x95: {"STA", modeZPX},
	0x96: {"STX", modeZPY},
	0x98: {"TYA", modeImp},
	0x99: {"STA", modeAbsY},
	0x9A: {"TXS", modeImp},
	0x9D: {"STA", modeAbsX},
	0xA0: {"LDY", modeImm},
	0xA1: {"LDA", modeIndX},
	0xA2: {"LDX", modeImm},
	0xA4: {"LDY", modeZP},
	0xA5: {"LDA", modeZP},
	0xA6: {"LDX", modeZP},
	0xA8: {"TAY", modeImp},
	0xA9: {"LDA", modeImm},
	0xAA: {"TAX", modeImp},
	0xAC: {"LDY", modeAbs},
	0xAD: {"LDA", modeAbs},
	0xAE: {"LDX", modeAbs},
	0xB0: {"BCS", modeRel},
	0xB1: {"LDA", modeIndY},
	0xB4: {"LDY", modeZPX},
	0xB5: {"LDA", modeZPX},
	0xB6: {"LDX", modeZPY},
	0xB8: {"CLV", modeImp},
	0xB9: {"LDA", modeAbsY},
	0xBA: {"TSX", modeImp},
	0xBC: {"LDY", modeAbsX},
	0xBD: {"LDA", modeAbsX},
	0xBE: {"LDX", modeAbsY},
	0xC0: {"CPY", modeImm},
	0xC1: {"CMP", modeIndX},
	0xC4: {"CPY", modeZP},
	0xC5: {"CMP", modeZP},
	0xC6: {"DEC", modeZP},
	0xC8: {"INY", modeImp},
	0xC9: {"CMP", modeImm},
	0xCA: {"DEX", modeImp},
	0xCB: {"WAI", modeImp},
	0xCC: {"CPY", modeAbs},
	0xCD: {"CMP", modeAbs},
	0xCE: {"DEC", modeAbs},
	0xD0: {"BNE", modeRel},
	0xD1: {"CMP", modeIndY},
	0xD5: {"CMP", modeZPX},
	0xD6: {"DEC", modeZPX},
	0xD8: {"CLD", modeImp},
	0xD9: {"CMP", modeAbsY},
	0xDD: {"CMP", modeAbsX},
	0xDE: {"DEC", modeAbsX},
	0xE0: {"CPX", modeImm},
	0xE1: {"SBC", modeIndX},
	0xE4: {"CPX", modeZP},
	0xE5: {"SBC", modeZP},
	0xE6: {"INC", modeZP},
	0xE8: {"INX", modeImp},
	0xE9: {"SBC", modeImm},
	0xEA: {"NOP", modeImp},
	0xEC: {"CPX", modeAbs},
	0xED: {"SBC", modeAbs},
	0xEE: {"INC", modeAbs},
	0xF0: {"BEQ", modeRel},
	0xF1: {"SBC", modeIndY},
	0xF5: {"SBC", modeZPX},
	0xF6: {"INC", modeZPX},
	0xF8: {"SED", modeImp},
	0xF9: {"SBC", modeAbsY},
	0xFD: {"SBC", modeAbsX},
	0xFE: {"INC", modeAbsX},
}

// Disasm disassembles a single 6502 instruction at pc using readByte, returning the text and byte length.
func Disasm(pc uint16, readByte func(uint16) byte) (string, int) {
	op := readByte(pc)
	info, ok := disasmTable[op]
	if !ok {
		return fmt.Sprintf("DB $%02X", op), 1
	}

	b1 := func() byte { return readByte(pc + 1) }
	w1 := func() uint16 {
		lo := uint16(readByte(pc + 1))
		hi := uint16(readByte(pc + 2))
		return (hi << 8) | lo
	}

	switch info.mode {
	case modeImp:
		return info.mnemonic, 1
	case modeAcc:
		return info.mnemonic + " A", 1
	case modeImm:
		return fmt.Sprintf("%s #$%02X", info.mnemonic, b1()), 2
	case modeZP:
		return fmt.Sprintf("%s $%02X", info.mnemonic, b1()), 2
	case modeZPX:
		return fmt.Sprintf("%s $%02X,X", info.mnemonic, b1()), 2
	case modeZPY:
		return fmt.Sprintf("%s $%02X,Y", info.mnemonic, b1()), 2
	case modeAbs:
		return fmt.Sprintf("%s $%04X", info.mnemonic, w1()), 3
	case modeAbsX:
		return fmt.Sprintf("%s $%04X,X", info.mnemonic, w1()), 3
	case modeAbsY:
		return fmt.Sprintf("%s $%04X,Y", info.mnemonic, w1()), 3
	case modeInd:
		return fmt.Sprintf("%s ($%04X)", info.mnemonic, w1()), 3
	case modeIndX:
		return fmt.Sprintf("%s ($%02X,X)", info.mnemonic, b1()), 2
	case modeIndY:
		return fmt.Sprintf("%s ($%02X),Y", info.mnemonic, b1()), 2
	case modeRel:
		rel := int8(b1())
		target := uint16(int32(pc) + 2 + int32(rel))
		return fmt.Sprintf("%s $%04X", info.mnemonic, target), 2
	default:
		return info.mnemonic, 1
	}
}
