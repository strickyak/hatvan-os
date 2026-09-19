package gep9

// PushByte pushes a single byte onto the system stack S.
func (c *CPU) PushByte(val byte) {
	c.S--
	c.Bus.WriteByte(c.S, val)
}

// PullByte pulls a single byte from the system stack S.
func (c *CPU) PullByte() byte {
	val := c.Bus.ReadByte(c.S)
	c.S++
	return val
}

// PushWord pushes a 16-bit word onto the system stack S (big-endian in memory).
func (c *CPU) PushWord(val uint16) {
	c.PushByte(byte(val))
	c.PushByte(byte(val >> 8))
}

// PullWord pulls a 16-bit word from the system stack S.
func (c *CPU) PullWord() uint16 {
	hi := uint16(c.PullByte())
	lo := uint16(c.PullByte())
	return (hi << 8) | lo
}

// PushByteU pushes a single byte onto the user stack U.
func (c *CPU) PushByteU(val byte) {
	c.U--
	c.Bus.WriteByte(c.U, val)
}

// PullByteU pulls a single byte from the user stack U.
func (c *CPU) PullByteU() byte {
	val := c.Bus.ReadByte(c.U)
	c.U++
	return val
}

// PushWordU pushes a 16-bit word onto the user stack U.
func (c *CPU) PushWordU(val uint16) {
	c.PushByteU(byte(val))
	c.PushByteU(byte(val >> 8))
}

// PullWordU pulls a 16-bit word from the user stack U.
func (c *CPU) PullWordU() uint16 {
	hi := uint16(c.PullByteU())
	lo := uint16(c.PullByteU())
	return (hi << 8) | lo
}

// ExecutePSHS pushes registers specified by postbyte onto S.
func (c *CPU) ExecutePSHS(pb byte) {
	if (pb & 0x80) != 0 {
		c.PushWord(c.PC)
	}
	if (pb & 0x40) != 0 {
		c.PushWord(c.U)
	}
	if (pb & 0x20) != 0 {
		c.PushWord(c.Y)
	}
	if (pb & 0x10) != 0 {
		c.PushWord(c.X)
	}
	if (pb & 0x08) != 0 {
		c.PushByte(c.DP)
	}
	if (pb & 0x04) != 0 {
		c.PushByte(c.B)
	}
	if (pb & 0x02) != 0 {
		c.PushByte(c.A)
	}
	if (pb & 0x01) != 0 {
		c.PushByte(c.CC)
	}
}

// ExecutePULS pulls registers specified by postbyte from S.
func (c *CPU) ExecutePULS(pb byte) {
	if (pb & 0x01) != 0 {
		c.CC = c.PullByte()
	}
	if (pb & 0x02) != 0 {
		c.A = c.PullByte()
	}
	if (pb & 0x04) != 0 {
		c.B = c.PullByte()
	}
	if (pb & 0x08) != 0 {
		c.DP = c.PullByte()
	}
	if (pb & 0x10) != 0 {
		c.X = c.PullWord()
	}
	if (pb & 0x20) != 0 {
		c.Y = c.PullWord()
	}
	if (pb & 0x40) != 0 {
		c.U = c.PullWord()
	}
	if (pb & 0x80) != 0 {
		c.PC = c.PullWord()
	}
}

// ExecutePSHU pushes registers specified by postbyte onto U.
func (c *CPU) ExecutePSHU(pb byte) {
	if (pb & 0x80) != 0 {
		c.PushWordU(c.PC)
	}
	if (pb & 0x40) != 0 {
		c.PushWordU(c.S)
	}
	if (pb & 0x20) != 0 {
		c.PushWordU(c.Y)
	}
	if (pb & 0x10) != 0 {
		c.PushWordU(c.X)
	}
	if (pb & 0x08) != 0 {
		c.PushByteU(c.DP)
	}
	if (pb & 0x04) != 0 {
		c.PushByteU(c.B)
	}
	if (pb & 0x02) != 0 {
		c.PushByteU(c.A)
	}
	if (pb & 0x01) != 0 {
		c.PushByteU(c.CC)
	}
}

// ExecutePULU pulls registers specified by postbyte from U.
func (c *CPU) ExecutePULU(pb byte) {
	if (pb & 0x01) != 0 {
		c.CC = c.PullByteU()
	}
	if (pb & 0x02) != 0 {
		c.A = c.PullByteU()
	}
	if (pb & 0x04) != 0 {
		c.B = c.PullByteU()
	}
	if (pb & 0x08) != 0 {
		c.DP = c.PullByteU()
	}
	if (pb & 0x10) != 0 {
		c.X = c.PullWordU()
	}
	if (pb & 0x20) != 0 {
		c.Y = c.PullWordU()
	}
	if (pb & 0x40) != 0 {
		c.S = c.PullWordU()
	}
	if (pb & 0x80) != 0 {
		c.PC = c.PullWordU()
	}
}

// PushInterruptFrame pushes CPU registers on S for an interrupt or SWI.
func (c *CPU) PushInterruptFrame(entire bool) {
	if entire {
		c.CC |= FlagE
		c.PushWord(c.PC)
		c.PushWord(c.U)
		c.PushWord(c.Y)
		c.PushWord(c.X)
		c.PushByte(c.DP)
		c.PushByte(c.B)
		c.PushByte(c.A)
		if (c.MD & 0x01) != 0 { // 6309 Native Mode
			c.PushByte(c.E)
			c.PushByte(c.F)
		}
		c.PushByte(c.CC)
	} else {
		c.CC &^= FlagE
		c.PushWord(c.PC)
		c.PushByte(c.CC)
	}
}

// ExecuteRTI returns from an interrupt/trap, honoring TaskFuse and 6809 vs 6309 stack frames.
func (c *CPU) ExecuteRTI() {
	// Inside RTI: switch active task to latched user task before pulling registers!
	if c.Bus.TaskFuseArmed {
		c.Bus.CurrentTask = c.Bus.TaskFuseTarget
		c.Bus.TaskFuseArmed = false
	}

	c.CC = c.PullByte()
	if (c.CC & FlagE) != 0 {
		if (c.MD & 0x01) != 0 { // 6309 Native Mode
			c.F = c.PullByte()
			c.E = c.PullByte()
		}
		c.A = c.PullByte()
		c.B = c.PullByte()
		c.DP = c.PullByte()
		c.X = c.PullWord()
		c.Y = c.PullWord()
		c.U = c.PullWord()
		c.PC = c.PullWord()
	} else {
		c.PC = c.PullWord()
	}
}
