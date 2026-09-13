package vm

// AddressingMode defines how the effective address or operand is resolved.
type AddressingMode int

const (
	AddrInherent AddressingMode = iota
	AddrImmediate8
	AddrImmediate16
	AddrDirect
	AddrExtended
	AddrRelative8
	AddrRelative16
	AddrIndexed
)

// resolveIndexed computes the effective address for 6809 / 6309 indexed addressing.
func (c *CPU) resolveIndexed() uint16 {
	pb := c.fetchByte()

	// Bit 7 == 0: 5-bit signed constant offset from X, Y, U, or S
	if (pb & 0x80) == 0 {
		regVal := c.getIndexRegister((pb >> 5) & 0x03)
		// Sign extend 5-bit to 16-bit
		offset := int16(pb & 0x1F)
		if (offset & 0x10) != 0 {
			offset |= ^int16(0x1F)
		}
		return uint16(int32(regVal) + int32(offset))
	}

	// Bit 7 == 1: Decoded post-byte modes
	regCode := (pb >> 5) & 0x03
	indirect := (pb & 0x10) != 0
	mode := pb & 0x0F

	var ea uint16

	switch mode {
	case 0x00: // ,R+ (cannot be indirect)
		regVal := c.getIndexRegister(regCode)
		ea = regVal
		c.setIndexRegister(regCode, regVal+1)

	case 0x01: // ,R++
		regVal := c.getIndexRegister(regCode)
		ea = regVal
		c.setIndexRegister(regCode, regVal+2)

	case 0x02: // ,-R (cannot be indirect)
		regVal := c.getIndexRegister(regCode) - 1
		c.setIndexRegister(regCode, regVal)
		ea = regVal

	case 0x03: // ,--R
		regVal := c.getIndexRegister(regCode) - 2
		c.setIndexRegister(regCode, regVal)
		ea = regVal

	case 0x04: // ,R (0 offset)
		ea = c.getIndexRegister(regCode)

	case 0x05: // B,R (B accumulator offset, signed)
		regVal := c.getIndexRegister(regCode)
		ea = uint16(int32(regVal) + int32(int8(c.B)))

	case 0x06: // A,R (A accumulator offset, signed)
		regVal := c.getIndexRegister(regCode)
		ea = uint16(int32(regVal) + int32(int8(c.A)))

	case 0x07: // E,R (6309 E accumulator offset, signed)
		regVal := c.getIndexRegister(regCode)
		ea = uint16(int32(regVal) + int32(int8(c.E)))

	case 0x08: // 8-bit constant offset
		offset := int8(c.fetchByte())
		regVal := c.getIndexRegister(regCode)
		ea = uint16(int32(regVal) + int32(offset))

	case 0x09: // 16-bit constant offset
		offset := int16(c.fetchWord())
		regVal := c.getIndexRegister(regCode)
		ea = uint16(int32(regVal) + int32(offset))

	case 0x0A: // F,R (6309 F accumulator offset, signed)
		regVal := c.getIndexRegister(regCode)
		ea = uint16(int32(regVal) + int32(int8(c.F)))

	case 0x0B: // D,R (D accumulator offset, signed)
		regVal := c.getIndexRegister(regCode)
		ea = uint16(int32(regVal) + int32(int16(c.GetD())))

	case 0x0C: // 8-bit PC relative offset
		offset := int8(c.fetchByte())
		ea = uint16(int32(c.PC) + int32(offset))

	case 0x0D: // 16-bit PC relative offset
		offset := int16(c.fetchWord())
		ea = uint16(int32(c.PC) + int32(offset))

	case 0x0E: // W,R (6309 W accumulator offset, signed)
		regVal := c.getIndexRegister(regCode)
		ea = uint16(int32(regVal) + int32(int16(c.GetW())))

	case 0x0F: // Extended indirect [address]
		ea = c.fetchWord()
		return c.Bus.ReadWord(ea)
	}

	if indirect {
		ea = c.Bus.ReadWord(ea)
	}

	return ea
}

func (c *CPU) getIndexRegister(code byte) uint16 {
	switch code & 0x03 {
	case 0:
		return c.X
	case 1:
		return c.Y
	case 2:
		return c.U
	case 3:
		return c.S
	default:
		return 0
	}
}

func (c *CPU) setIndexRegister(code byte, val uint16) {
	switch code & 0x03 {
	case 0:
		c.X = val
	case 1:
		c.Y = val
	case 2:
		c.U = val
	case 3:
		c.S = val
	}
}
