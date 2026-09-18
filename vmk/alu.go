package vmk

func maskForSize(size OpSize) uint32 {
	switch size {
	case SizeByte:
		return 0x000000FF
	case SizeWord:
		return 0x0000FFFF
	default:
		return 0xFFFFFFFF
	}
}

func msbForSize(size OpSize) uint32 {
	switch size {
	case SizeByte:
		return 0x00000080
	case SizeWord:
		return 0x00008000
	default:
		return 0x80000000
	}
}

func toSigned(val uint32, size OpSize) int64 {
	switch size {
	case SizeByte:
		return int64(int8(val))
	case SizeWord:
		return int64(int16(val))
	default:
		return int64(int32(val))
	}
}

func minSignedForSize(size OpSize) int64 {
	switch size {
	case SizeByte:
		return -128
	case SizeWord:
		return -32768
	default:
		return -2147483648
	}
}

func maxSignedForSize(size OpSize) int64 {
	switch size {
	case SizeByte:
		return 127
	case SizeWord:
		return 32767
	default:
		return 2147483647
	}
}

func (c *CPU) setFlag(flag uint16, set bool) {
	if set {
		c.SR |= flag
	} else {
		c.SR &^= flag
	}
}

func (c *CPU) setNZFlags(val uint32, size OpSize) {
	mask := maskForSize(size)
	msb := msbForSize(size)
	val &= mask

	c.setFlag(FlagZ, val == 0)
	c.setFlag(FlagN, (val&msb) != 0)
}

// EvaluateCondition tests the 4-bit M68000 condition code against the SR.
func (c *CPU) EvaluateCondition(cond uint8) bool {
	carry := (c.SR & FlagC) != 0
	overflow := (c.SR & FlagV) != 0
	zero := (c.SR & FlagZ) != 0
	negative := (c.SR & FlagN) != 0

	switch cond & 0x0F {
	case 0: // T: True
		return true
	case 1: // F: False
		return false
	case 2: // HI: High (C=0 and Z=0)
		return !carry && !zero
	case 3: // LS: Low or Same (C=1 or Z=1)
		return carry || zero
	case 4: // CC/HS: Carry Clear (C=0)
		return !carry
	case 5: // CS/LO: Carry Set (C=1)
		return carry
	case 6: // NE: Not Equal (Z=0)
		return !zero
	case 7: // EQ: Equal (Z=1)
		return zero
	case 8: // VC: Overflow Clear (V=0)
		return !overflow
	case 9: // VS: Overflow Set (V=1)
		return overflow
	case 10: // PL: Plus (N=0)
		return !negative
	case 11: // MI: Minus (N=1)
		return negative
	case 12: // GE: Greater or Equal (N=V)
		return negative == overflow
	case 13: // LT: Less Than (N!=V)
		return negative != overflow
	case 14: // GT: Greater Than (N=V and Z=0)
		return (negative == overflow) && !zero
	case 15: // LE: Less or Equal (N!=V or Z=1)
		return (negative != overflow) || zero
	}
	return false
}

// Add performs dst + src and sets X, N, Z, V, C.
func (c *CPU) Add(dst, src uint32, size OpSize) uint32 {
	mask := maskForSize(size)
	msb := msbForSize(size)
	dst &= mask
	src &= mask
	res := (dst + src) & mask

	sm := (src & msb) != 0
	dm := (dst & msb) != 0
	rm := (res & msb) != 0

	v := (sm && dm && !rm) || (!sm && !dm && rm)
	cBit := (sm && dm) || (!rm && dm) || (sm && !rm)

	c.setNZFlags(res, size)
	c.setFlag(FlagV, v)
	c.setFlag(FlagC, cBit)
	c.setFlag(FlagX, cBit)
	return res
}

// Sub performs dst - src and sets X, N, Z, V, C.
func (c *CPU) Sub(dst, src uint32, size OpSize) uint32 {
	mask := maskForSize(size)
	msb := msbForSize(size)
	dst &= mask
	src &= mask
	res := (dst - src) & mask

	sm := (src & msb) != 0
	dm := (dst & msb) != 0
	rm := (res & msb) != 0

	v := (!sm && dm && !rm) || (sm && !dm && rm)
	cBit := (sm && !dm) || (rm && !dm) || (sm && rm)

	c.setNZFlags(res, size)
	c.setFlag(FlagV, v)
	c.setFlag(FlagC, cBit)
	c.setFlag(FlagX, cBit)
	return res
}

// Cmp performs dst - src, setting N, Z, V, C without modifying X.
func (c *CPU) Cmp(dst, src uint32, size OpSize) {
	mask := maskForSize(size)
	msb := msbForSize(size)
	dst &= mask
	src &= mask
	res := (dst - src) & mask

	sm := (src & msb) != 0
	dm := (dst & msb) != 0
	rm := (res & msb) != 0

	v := (!sm && dm && !rm) || (sm && !dm && rm)
	cBit := (sm && !dm) || (rm && !dm) || (sm && rm)

	c.setNZFlags(res, size)
	c.setFlag(FlagV, v)
	c.setFlag(FlagC, cBit)
}

// AddX performs dst + src + X and sets X, N, Z, V, C.
func (c *CPU) AddX(dst, src uint32, size OpSize) uint32 {
	mask := maskForSize(size)
	x := uint64(0)
	if (c.SR & FlagX) != 0 {
		x = 1
	}

	fullSum := uint64(dst&mask) + uint64(src&mask) + x
	res := uint32(fullSum) & mask

	sumSigned := toSigned(dst, size) + toSigned(src, size) + int64(x)
	v := sumSigned < minSignedForSize(size) || sumSigned > maxSignedForSize(size)
	cBit := fullSum > uint64(mask)

	if res != 0 {
		c.setFlag(FlagZ, false)
	}
	c.setFlag(FlagN, (res&msbForSize(size)) != 0)
	c.setFlag(FlagV, v)
	c.setFlag(FlagC, cBit)
	c.setFlag(FlagX, cBit)
	return res
}

// SubX performs dst - src - X and sets X, N, Z, V, C.
func (c *CPU) SubX(dst, src uint32, size OpSize) uint32 {
	mask := maskForSize(size)
	x := uint64(0)
	if (c.SR & FlagX) != 0 {
		x = 1
	}

	fullDiff := int64(dst&mask) - int64(src&mask) - int64(x)
	res := uint32(fullDiff) & mask

	diffSigned := toSigned(dst, size) - toSigned(src, size) - int64(x)
	v := diffSigned < minSignedForSize(size) || diffSigned > maxSignedForSize(size)
	cBit := fullDiff < 0

	if res != 0 {
		c.setFlag(FlagZ, false)
	}
	c.setFlag(FlagN, (res&msbForSize(size)) != 0)
	c.setFlag(FlagV, v)
	c.setFlag(FlagC, cBit)
	c.setFlag(FlagX, cBit)
	return res
}

// Neg performs 0 - dst.
func (c *CPU) Neg(dst uint32, size OpSize) uint32 {
	return c.Sub(0, dst, size)
}

// NegX performs 0 - dst - X.
func (c *CPU) NegX(dst uint32, size OpSize) uint32 {
	return c.SubX(0, dst, size)
}

// And performs dst & src and sets N, Z, clearing V, C.
func (c *CPU) And(dst, src uint32, size OpSize) uint32 {
	res := (dst & src) & maskForSize(size)
	c.setNZFlags(res, size)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
	return res
}

// Or performs dst | src and sets N, Z, clearing V, C.
func (c *CPU) Or(dst, src uint32, size OpSize) uint32 {
	res := (dst | src) & maskForSize(size)
	c.setNZFlags(res, size)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
	return res
}

// Eor performs dst ^ src and sets N, Z, clearing V, C.
func (c *CPU) Eor(dst, src uint32, size OpSize) uint32 {
	res := (dst ^ src) & maskForSize(size)
	c.setNZFlags(res, size)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
	return res
}

// Not performs ^val and sets N, Z, clearing V, C.
func (c *CPU) Not(val uint32, size OpSize) uint32 {
	res := (^val) & maskForSize(size)
	c.setNZFlags(res, size)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
	return res
}

// Tst tests a value, setting N, Z and clearing V, C.
func (c *CPU) Tst(val uint32, size OpSize) {
	c.setNZFlags(val, size)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
}

// Clr clears condition codes and returns 0.
func (c *CPU) Clr(size OpSize) uint32 {
	c.setFlag(FlagN, false)
	c.setFlag(FlagZ, true)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
	return 0
}

// Shifts and Rotates

func (c *CPU) Lsl(val uint32, count int, size OpSize) uint32 {
	count &= 63
	mask := maskForSize(size)
	msb := msbForSize(size)
	val &= mask

	lastOut := false
	for i := 0; i < count; i++ {
		lastOut = (val & msb) != 0
		val = (val << 1) & mask
	}

	if count > 0 {
		c.setFlag(FlagC, lastOut)
		c.setFlag(FlagX, lastOut)
	} else {
		c.setFlag(FlagC, false)
	}
	c.setFlag(FlagV, false)
	c.setNZFlags(val, size)
	return val
}

func (c *CPU) Lsr(val uint32, count int, size OpSize) uint32 {
	count &= 63
	mask := maskForSize(size)
	val &= mask

	lastOut := false
	for i := 0; i < count; i++ {
		lastOut = (val & 1) != 0
		val >>= 1
	}

	if count > 0 {
		c.setFlag(FlagC, lastOut)
		c.setFlag(FlagX, lastOut)
	} else {
		c.setFlag(FlagC, false)
	}
	c.setFlag(FlagV, false)
	c.setNZFlags(val, size)
	return val
}

func (c *CPU) Asl(val uint32, count int, size OpSize) uint32 {
	count &= 63
	mask := maskForSize(size)
	msb := msbForSize(size)
	val &= mask

	v := false
	lastOut := false
	for i := 0; i < count; i++ {
		out := (val & msb) != 0
		val = (val << 1) & mask
		if ((val & msb) != 0) != out {
			v = true
		}
		lastOut = out
	}

	if count > 0 {
		c.setFlag(FlagC, lastOut)
		c.setFlag(FlagX, lastOut)
	} else {
		c.setFlag(FlagC, false)
	}
	c.setFlag(FlagV, v)
	c.setNZFlags(val, size)
	return val
}

func (c *CPU) Asr(val uint32, count int, size OpSize) uint32 {
	count &= 63
	mask := maskForSize(size)
	msb := msbForSize(size)
	val &= mask

	lastOut := false
	for i := 0; i < count; i++ {
		lastOut = (val & 1) != 0
		msbBit := val & msb
		val = (val >> 1) | msbBit
	}

	if count > 0 {
		c.setFlag(FlagC, lastOut)
		c.setFlag(FlagX, lastOut)
	} else {
		c.setFlag(FlagC, false)
	}
	c.setFlag(FlagV, false)
	c.setNZFlags(val, size)
	return val
}

func (c *CPU) Rol(val uint32, count int, size OpSize) uint32 {
	count &= 63
	mask := maskForSize(size)
	msb := msbForSize(size)
	val &= mask

	lastOut := false
	for i := 0; i < count; i++ {
		lastOut = (val & msb) != 0
		val = (val << 1) & mask
		if lastOut {
			val |= 1
		}
	}

	if count > 0 {
		c.setFlag(FlagC, lastOut)
	} else {
		c.setFlag(FlagC, false)
	}
	c.setFlag(FlagV, false)
	c.setNZFlags(val, size)
	return val
}

func (c *CPU) Ror(val uint32, count int, size OpSize) uint32 {
	count &= 63
	mask := maskForSize(size)
	msb := msbForSize(size)
	val &= mask

	lastOut := false
	for i := 0; i < count; i++ {
		lastOut = (val & 1) != 0
		val >>= 1
		if lastOut {
			val |= msb
		}
	}

	if count > 0 {
		c.setFlag(FlagC, lastOut)
	} else {
		c.setFlag(FlagC, false)
	}
	c.setFlag(FlagV, false)
	c.setNZFlags(val, size)
	return val
}

func (c *CPU) Roxl(val uint32, count int, size OpSize) uint32 {
	count &= 63
	mask := maskForSize(size)
	msb := msbForSize(size)
	val &= mask

	xBit := (c.SR & FlagX) != 0
	for i := 0; i < count; i++ {
		out := (val & msb) != 0
		val = (val << 1) & mask
		if xBit {
			val |= 1
		}
		xBit = out
	}

	if count > 0 {
		c.setFlag(FlagC, xBit)
		c.setFlag(FlagX, xBit)
	} else {
		c.setFlag(FlagC, (c.SR&FlagX) != 0)
	}
	c.setFlag(FlagV, false)
	c.setNZFlags(val, size)
	return val
}

func (c *CPU) Roxr(val uint32, count int, size OpSize) uint32 {
	count &= 63
	mask := maskForSize(size)
	msb := msbForSize(size)
	val &= mask

	xBit := (c.SR & FlagX) != 0
	for i := 0; i < count; i++ {
		out := (val & 1) != 0
		val >>= 1
		if xBit {
			val |= msb
		}
		xBit = out
	}

	if count > 0 {
		c.setFlag(FlagC, xBit)
		c.setFlag(FlagX, xBit)
	} else {
		c.setFlag(FlagC, (c.SR&FlagX) != 0)
	}
	c.setFlag(FlagV, false)
	c.setNZFlags(val, size)
	return val
}

// Mulu performs unsigned 16x16 -> 32 multiplication.
func (c *CPU) Mulu(dst, src uint16) uint32 {
	res := uint32(dst) * uint32(src)
	c.setNZFlags(res, SizeLong)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
	return res
}

// Muls performs signed 16x16 -> 32 multiplication.
func (c *CPU) Muls(dst, src int16) uint32 {
	res := uint32(int32(dst) * int32(src))
	c.setNZFlags(res, SizeLong)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
	return res
}

// Divu performs unsigned 32/16 division, returning (rem<<16 | quot).
func (c *CPU) Divu(dividend uint32, divisor uint16) (uint32, bool) {
	if divisor == 0 {
		c.TriggerException(VecZeroDivide)
		return 0, false
	}
	quot := dividend / uint32(divisor)
	rem := dividend % uint32(divisor)
	if quot > 0xFFFF {
		c.setFlag(FlagV, true)
		c.setFlag(FlagC, false)
		return 0, false
	}
	res := (rem << 16) | (quot & 0xFFFF)
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
	c.setFlag(FlagZ, (quot&0xFFFF) == 0)
	c.setFlag(FlagN, (quot&0x8000) != 0)
	return res, true
}

// Divs performs signed 32/16 division, returning (rem<<16 | quot).
func (c *CPU) Divs(dividend uint32, divisor int16) (uint32, bool) {
	if divisor == 0 {
		c.TriggerException(VecZeroDivide)
		return 0, false
	}
	quot := int32(dividend) / int32(divisor)
	rem := int32(dividend) % int32(divisor)
	if quot > 32767 || quot < -32768 {
		c.setFlag(FlagV, true)
		c.setFlag(FlagC, false)
		return 0, false
	}
	res := (uint32(uint16(rem)) << 16) | uint32(uint16(quot))
	c.setFlag(FlagV, false)
	c.setFlag(FlagC, false)
	c.setFlag(FlagZ, int16(quot) == 0)
	c.setFlag(FlagN, int16(quot) < 0)
	return res, true
}

// Abcd adds decimal with extend.
func (c *CPU) Abcd(dst, src byte) byte {
	x := byte(0)
	if (c.SR & FlagX) != 0 {
		x = 1
	}

	lo := (dst & 0x0F) + (src & 0x0F) + x
	carryLo := lo > 9
	if carryLo {
		lo += 6
	}

	hi := (dst >> 4) + (src >> 4) + (lo >> 4)
	carryHi := hi > 9
	if carryHi {
		hi += 6
	}

	res := ((hi & 0x0F) << 4) | (lo & 0x0F)
	cBit := carryHi || hi > 0x0F

	if res != 0 {
		c.setFlag(FlagZ, false)
	}
	c.setFlag(FlagN, (res&0x80) != 0)
	c.setFlag(FlagC, cBit)
	c.setFlag(FlagX, cBit)
	return res
}

// Sbcd subtracts decimal with extend.
func (c *CPU) Sbcd(dst, src byte) byte {
	x := byte(0)
	if (c.SR & FlagX) != 0 {
		x = 1
	}

	lo := int(dst&0x0F) - int(src&0x0F) - int(x)
	borrowLo := lo < 0
	if borrowLo {
		lo += 10
	}

	hi := int(dst>>4) - int(src>>4)
	if borrowLo {
		hi -= 1
	}
	borrowHi := hi < 0
	if borrowHi {
		hi += 10
	}

	res := byte(((hi & 0x0F) << 4) | (lo & 0x0F))
	cBit := borrowHi

	if res != 0 {
		c.setFlag(FlagZ, false)
	}
	c.setFlag(FlagN, (res&0x80) != 0)
	c.setFlag(FlagC, cBit)
	c.setFlag(FlagX, cBit)
	return res
}
