package gepz

const (
	FlagS = 0x80 // Sign
	FlagZ = 0x40 // Zero
	FlagY = 0x20 // Undocumented bit 5
	FlagH = 0x10 // Half Carry
	FlagX = 0x08 // Undocumented bit 3
	FlagV = 0x04 // Parity / Overflow (P/V)
	FlagN = 0x02 // Add / Subtract
	FlagC = 0x01 // Carry
)

var parityTable [256]byte

func init() {
	for i := 0; i < 256; i++ {
		bits := 0
		for j := 0; j < 8; j++ {
			if (i & (1 << j)) != 0 {
				bits++
			}
		}
		if (bits % 2) == 0 {
			parityTable[i] = FlagV // Even parity
		} else {
			parityTable[i] = 0
		}
	}
}

// Add8 performs 8-bit addition: a + b + carry.
func Add8(a, b byte, withCarry bool, oldF byte) (byte, byte) {
	cIn := 0
	if withCarry && (oldF&FlagC) != 0 {
		cIn = 1
	}
	res16 := int(a) + int(b) + cIn
	res := byte(res16)

	var f byte
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	if ((int(a)&0x0F) + (int(b)&0x0F) + cIn) > 0x0F {
		f |= FlagH
	}
	// Signed overflow: operands have same sign, result has opposite sign
	if ^(a ^ b)&(a ^ res)&0x80 != 0 {
		f |= FlagV
	}
	if res16 > 0xFF {
		f |= FlagC
	}
	return res, f
}

// Sub8 performs 8-bit subtraction: a - b - borrow.
func Sub8(a, b byte, withBorrow bool, oldF byte) (byte, byte) {
	bIn := 0
	if withBorrow && (oldF&FlagC) != 0 {
		bIn = 1
	}
	res16 := int(a) - int(b) - bIn
	res := byte(res16)

	f := byte(FlagN)
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	if ((int(a) & 0x0F) - (int(b) & 0x0F) - bIn) < 0 {
		f |= FlagH
	}
	// Signed overflow: operands have different signs, result has different sign from minuend
	if (a ^ b)&(a ^ res)&0x80 != 0 {
		f |= FlagV
	}
	if res16 < 0 {
		f |= FlagC
	}
	return res, f
}

// And8 performs bitwise AND: a & b.
func And8(a, b byte) (byte, byte) {
	res := a & b
	f := byte(FlagH) // Z80 AND always sets H
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	f |= parityTable[res]
	return res, f
}

// Or8 performs bitwise OR: a | b.
func Or8(a, b byte) (byte, byte) {
	res := a | b
	var f byte
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	f |= parityTable[res]
	return res, f
}

// Xor8 performs bitwise XOR: a ^ b.
func Xor8(a, b byte) (byte, byte) {
	res := a ^ b
	var f byte
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	f |= parityTable[res]
	return res, f
}

// Cp8 compares a and b: updates flags like Sub8 without saving result.
func Cp8(a, b byte) byte {
	_, f := Sub8(a, b, false, 0)
	// Undocumented Z80 trait: bits 3 and 5 come from operand b
	f &^= (FlagY | FlagX)
	f |= (b & (FlagY | FlagX))
	return f
}

// Inc8 increments byte: preserves C flag.
func Inc8(a byte, oldF byte) (byte, byte) {
	res := a + 1
	f := oldF & FlagC // Preserve Carry
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	if (a & 0x0F) == 0x0F {
		f |= FlagH
	}
	if a == 0x7F {
		f |= FlagV // Overflow from +127 to -128
	}
	return res, f
}

// Dec8 decrements byte: preserves C flag, sets N.
func Dec8(a byte, oldF byte) (byte, byte) {
	res := a - 1
	f := (oldF & FlagC) | FlagN // Preserve Carry, set N
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	if (a & 0x0F) == 0x00 {
		f |= FlagH
	}
	if a == 0x80 {
		f |= FlagV // Overflow from -128 to +127
	}
	return res, f
}

// Add16 adds two 16-bit integers: preserves S, Z, V; sets H, C; clears N.
func Add16(hl, val uint16, oldF byte) (uint16, byte) {
	res32 := uint32(hl) + uint32(val)
	res := uint16(res32)

	f := oldF & (FlagS | FlagZ | FlagV) // Preserve S, Z, V
	if ((hl & 0x0FFF) + (val & 0x0FFF)) > 0x0FFF {
		f |= FlagH
	}
	f |= (byte(res>>8) & (FlagY | FlagX))
	if res32 > 0xFFFF {
		f |= FlagC
	}
	return res, f
}

// Adc16 adds two 16-bit integers with carry: updates all flags.
func Adc16(hl, val uint16, oldF byte) (uint16, byte) {
	cIn := uint32(0)
	if (oldF & FlagC) != 0 {
		cIn = 1
	}
	res32 := uint32(hl) + uint32(val) + cIn
	res := uint16(res32)

	var f byte
	if (res & 0x8000) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (byte(res>>8) & (FlagY | FlagX))
	if ((hl & 0x0FFF) + (val & 0x0FFF) + uint16(cIn)) > 0x0FFF {
		f |= FlagH
	}
	if ^(hl ^ val)&(hl ^ res)&0x8000 != 0 {
		f |= FlagV
	}
	if res32 > 0xFFFF {
		f |= FlagC
	}
	return res, f
}

// Sbc16 subtracts two 16-bit integers with borrow: updates all flags.
func Sbc16(hl, val uint16, oldF byte) (uint16, byte) {
	bIn := int(0)
	if (oldF & FlagC) != 0 {
		bIn = 1
	}
	res32 := int(hl) - int(val) - bIn
	res := uint16(res32)

	f := byte(FlagN)
	if (res & 0x8000) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (byte(res>>8) & (FlagY | FlagX))
	if ((int(hl) & 0x0FFF) - (int(val) & 0x0FFF) - bIn) < 0 {
		f |= FlagH
	}
	if (hl ^ val)&(hl ^ res)&0x8000 != 0 {
		f |= FlagV
	}
	if res32 < 0 {
		f |= FlagC
	}
	return res, f
}

// Rotate and Shift Instructions (CB group)
func Rlc(val byte) (byte, byte) {
	c := (val >> 7) & 1
	res := (val << 1) | c
	var f byte
	if c != 0 {
		f |= FlagC
	}
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	f |= parityTable[res]
	return res, f
}

func Rrc(val byte) (byte, byte) {
	c := val & 1
	res := (val >> 1) | (c << 7)
	var f byte
	if c != 0 {
		f |= FlagC
	}
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	f |= parityTable[res]
	return res, f
}

func Rl(val byte, oldF byte) (byte, byte) {
	cIn := oldF & FlagC
	cOut := (val >> 7) & 1
	res := (val << 1) | cIn
	var f byte
	if cOut != 0 {
		f |= FlagC
	}
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	f |= parityTable[res]
	return res, f
}

func Rr(val byte, oldF byte) (byte, byte) {
	cIn := (oldF & FlagC) << 7
	cOut := val & 1
	res := (val >> 1) | cIn
	var f byte
	if cOut != 0 {
		f |= FlagC
	}
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	f |= parityTable[res]
	return res, f
}

func Sla(val byte) (byte, byte) {
	c := (val >> 7) & 1
	res := val << 1
	var f byte
	if c != 0 {
		f |= FlagC
	}
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	f |= parityTable[res]
	return res, f
}

func Sra(val byte) (byte, byte) {
	c := val & 1
	res := (val >> 1) | (val & 0x80) // Arithmetic shift: preserve bit 7
	var f byte
	if c != 0 {
		f |= FlagC
	}
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	f |= parityTable[res]
	return res, f
}

func Srl(val byte) (byte, byte) {
	c := val & 1
	res := val >> 1 // Logical shift: 0 into bit 7
	var f byte
	if c != 0 {
		f |= FlagC
	}
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	f |= parityTable[res]
	return res, f
}

// Daa decimal adjust accumulator for BCD arithmetic.
func Daa(a byte, oldF byte) (byte, byte) {
	cf := (oldF & FlagC) != 0
	hf := (oldF & FlagH) != 0
	nf := (oldF & FlagN) != 0

	var corr byte
	if hf || (a&0x0F) > 9 {
		corr |= 0x06
	}
	if cf || a > 0x99 {
		corr |= 0x60
		cf = true
	}

	res := a
	if nf {
		res -= corr
	} else {
		res += corr
	}

	var f byte
	if (res & 0x80) != 0 {
		f |= FlagS
	}
	if res == 0 {
		f |= FlagZ
	}
	f |= (res & (FlagY | FlagX))
	if (a^res)&0x10 != 0 {
		f |= FlagH
	}
	f |= parityTable[res]
	if nf {
		f |= FlagN
	}
	if cf {
		f |= FlagC
	}
	return res, f
}
