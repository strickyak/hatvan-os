package gepz

import (
	"fmt"
)

// Disasm single instruction at pc.
func Disasm(pc uint16, readByte func(uint16) byte) (string, int) {
	op := readByte(pc)

	switch op {
	case 0x00:
		return "nop", 1
	case 0x76:
		return "halt", 1
	case 0xF3:
		return "di", 1
	case 0xFB:
		return "ei", 1
	case 0x08:
		return "ex af, af'", 1
	case 0xD9:
		return "exx", 1
	case 0xEB:
		return "ex de, hl", 1
	case 0xE3:
		return "ex (sp), hl", 1
	case 0xF9:
		return "ld sp, hl", 1
	case 0xC9:
		return "ret", 1
	case 0xE9:
		return "jp (hl)", 1
	case 0x27:
		return "daa", 1
	case 0x2F:
		return "cpl", 1
	case 0x37:
		return "scf", 1
	case 0x3F:
		return "ccf", 1
	case 0x07:
		return "rlca", 1
	case 0x0F:
		return "rrca", 1
	case 0x17:
		return "rla", 1
	case 0x1F:
		return "rra", 1

	case 0x18:
		disp := int8(readByte(pc + 1))
		return fmt.Sprintf("jr %04X", uint16(int32(pc+2)+int32(disp))), 2
	case 0x10:
		disp := int8(readByte(pc + 1))
		return fmt.Sprintf("djnz %04X", uint16(int32(pc+2)+int32(disp))), 2
	case 0x20:
		disp := int8(readByte(pc + 1))
		return fmt.Sprintf("jr nz, %04X", uint16(int32(pc+2)+int32(disp))), 2
	case 0x28:
		disp := int8(readByte(pc + 1))
		return fmt.Sprintf("jr z, %04X", uint16(int32(pc+2)+int32(disp))), 2
	case 0x30:
		disp := int8(readByte(pc + 1))
		return fmt.Sprintf("jr nc, %04X", uint16(int32(pc+2)+int32(disp))), 2
	case 0x38:
		disp := int8(readByte(pc + 1))
		return fmt.Sprintf("jr c, %04X", uint16(int32(pc+2)+int32(disp))), 2

	case 0x01:
		nn := uint16(readByte(pc+1)) | (uint16(readByte(pc+2)) << 8)
		return fmt.Sprintf("ld bc, %04X", nn), 3
	case 0x11:
		nn := uint16(readByte(pc+1)) | (uint16(readByte(pc+2)) << 8)
		return fmt.Sprintf("ld de, %04X", nn), 3
	case 0x21:
		nn := uint16(readByte(pc+1)) | (uint16(readByte(pc+2)) << 8)
		return fmt.Sprintf("ld hl, %04X", nn), 3
	case 0x31:
		nn := uint16(readByte(pc+1)) | (uint16(readByte(pc+2)) << 8)
		return fmt.Sprintf("ld sp, %04X", nn), 3

	case 0x02:
		return "ld (bc), a", 1
	case 0x12:
		return "ld (de), a", 1
	case 0x22:
		nn := uint16(readByte(pc+1)) | (uint16(readByte(pc+2)) << 8)
		return fmt.Sprintf("ld (%04X), hl", nn), 3
	case 0x32:
		nn := uint16(readByte(pc+1)) | (uint16(readByte(pc+2)) << 8)
		return fmt.Sprintf("ld (%04X), a", nn), 3
	case 0x0A:
		return "ld a, (bc)", 1
	case 0x1A:
		return "ld a, (de)", 1
	case 0x2A:
		nn := uint16(readByte(pc+1)) | (uint16(readByte(pc+2)) << 8)
		return fmt.Sprintf("ld hl, (%04X)", nn), 3
	case 0x3A:
		nn := uint16(readByte(pc+1)) | (uint16(readByte(pc+2)) << 8)
		return fmt.Sprintf("ld a, (%04X)", nn), 3

	case 0x09:
		return "add hl, bc", 1
	case 0x19:
		return "add hl, de", 1
	case 0x29:
		return "add hl, hl", 1
	case 0x39:
		return "add hl, sp", 1

	case 0x03:
		return "inc bc", 1
	case 0x13:
		return "inc de", 1
	case 0x23:
		return "inc hl", 1
	case 0x33:
		return "inc sp", 1
	case 0x0B:
		return "dec bc", 1
	case 0x1B:
		return "dec de", 1
	case 0x2B:
		return "dec hl", 1
	case 0x3B:
		return "dec sp", 1

	case 0x34:
		return "inc (hl)", 1
	case 0x35:
		return "dec (hl)", 1
	case 0x36:
		n := readByte(pc + 1)
		return fmt.Sprintf("ld (hl), %02X", n), 2

	case 0xC3:
		nn := uint16(readByte(pc+1)) | (uint16(readByte(pc+2)) << 8)
		return fmt.Sprintf("jp %04X", nn), 3
	case 0xCD:
		nn := uint16(readByte(pc+1)) | (uint16(readByte(pc+2)) << 8)
		return fmt.Sprintf("call %04X", nn), 3
	case 0xD3:
		n := readByte(pc + 1)
		return fmt.Sprintf("out (%02X), a", n), 2
	case 0xDB:
		n := readByte(pc + 1)
		return fmt.Sprintf("in a, (%02X)", n), 2

	case 0xC5:
		return "push bc", 1
	case 0xD5:
		return "push de", 1
	case 0xE5:
		return "push hl", 1
	case 0xF5:
		return "push af", 1
	case 0xC1:
		return "pop bc", 1
	case 0xD1:
		return "pop de", 1
	case 0xE1:
		return "pop hl", 1
	case 0xF1:
		return "pop af", 1

	case 0xC6:
		return fmt.Sprintf("add a, %02X", readByte(pc+1)), 2
	case 0xCE:
		return fmt.Sprintf("adc a, %02X", readByte(pc+1)), 2
	case 0xD6:
		return fmt.Sprintf("sub %02X", readByte(pc+1)), 2
	case 0xDE:
		return fmt.Sprintf("sbc a, %02X", readByte(pc+1)), 2
	case 0xE6:
		return fmt.Sprintf("and %02X", readByte(pc+1)), 2
	case 0xEE:
		return fmt.Sprintf("xor %02X", readByte(pc+1)), 2
	case 0xF6:
		return fmt.Sprintf("or %02X", readByte(pc+1)), 2
	case 0xFE:
		return fmt.Sprintf("cp %02X", readByte(pc+1)), 2

	case 0xED:
		edOp := readByte(pc + 1)
		switch edOp {
		case 0x45:
			return "retn", 2
		case 0x4D:
			return "reti", 2
		case 0xB0:
			return "ldir", 2
		case 0xA0:
			return "ldi", 2
		case 0xB8:
			return "lddr", 2
		case 0xA8:
			return "ldd", 2
		case 0x56:
			return "im 1", 2
		case 0x46:
			return "im 0", 2
		case 0x5E:
			return "im 2", 2
		case 0x44:
			return "neg", 2
		case 0x47:
			return "ld i, a", 2
		case 0x4F:
			return "ld r, a", 2
		case 0x57:
			return "ld a, i", 2
		case 0x5F:
			return "ld a, r", 2
		}
		return fmt.Sprintf("ed %02X", edOp), 2

	case 0xDD:
		ddOp := readByte(pc + 1)
		switch ddOp {
		case 0x21:
			nn := uint16(readByte(pc+2)) | (uint16(readByte(pc+3)) << 8)
			return fmt.Sprintf("ld ix, %04X", nn), 4
		case 0xE5:
			return "push ix", 2
		case 0xE1:
			return "pop ix", 2
		case 0xE9:
			return "jp (ix)", 2
		case 0x23:
			return "inc ix", 2
		case 0x2B:
			return "dec ix", 2
		}
		return fmt.Sprintf("dd %02X", ddOp), 2

	case 0xFD:
		fdOp := readByte(pc + 1)
		switch fdOp {
		case 0x21:
			nn := uint16(readByte(pc+2)) | (uint16(readByte(pc+3)) << 8)
			return fmt.Sprintf("ld iy, %04X", nn), 4
		case 0xE5:
			return "push iy", 2
		case 0xE1:
			return "pop iy", 2
		case 0xE9:
			return "jp (iy)", 2
		case 0x23:
			return "inc iy", 2
		case 0x2B:
			return "dec iy", 2
		}
		return fmt.Sprintf("fd %02X", fdOp), 2
	}

	regs := []string{"b", "c", "d", "e", "h", "l", "(hl)", "a"}

	// 8-bit INC
	if (op & 0xC7) == 0x04 {
		return fmt.Sprintf("inc %s", regs[(op>>3)&7]), 1
	}
	// 8-bit DEC
	if (op & 0xC7) == 0x05 {
		return fmt.Sprintf("dec %s", regs[(op>>3)&7]), 1
	}
	// 8-bit LD r, n
	if (op & 0xC7) == 0x06 {
		return fmt.Sprintf("ld %s, %02X", regs[(op>>3)&7], readByte(pc+1)), 2
	}

	// 8-bit LD r, r'
	if op >= 0x40 && op <= 0x7F && op != 0x76 {
		dst := (op >> 3) & 7
		src := op & 7
		return fmt.Sprintf("ld %s, %s", regs[dst], regs[src]), 1
	}

	// 8-bit ALU
	if op >= 0x80 && op <= 0xBF {
		aluOps := []string{"add a,", "adc a,", "sub", "sbc a,", "and", "xor", "or", "cp"}
		aluOp := (op >> 3) & 7
		r := op & 7
		return fmt.Sprintf("%s %s", aluOps[aluOp], regs[r]), 1
	}

	return fmt.Sprintf("db %02X", op), 1
}
