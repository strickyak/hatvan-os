package vm

import (
	"fmt"
	"strings"
)

// executeHyperOp handles GOMAR-compatible hypercalls ($12, $21, <hop>).
func (c *CPU) executeHyperOp(hop byte) int {
	switch hop {
	case 107: // Hyper Exit
		c.Halted = true
		c.ExitCode = int(c.GetD())
		if c.ExitCode == 0 && c.X != 0 {
			c.ExitCode = int(c.X)
		}
		return 2

	case 111: // Hyper Printf (PrintH2: var_ptr in X register)
		c.hyperPrintf(c.X)
		return 6

	case 132: // Hyper PutChar (character in register B)
		ch := c.B
		if c.Bus.ConsoleOut != nil {
			c.Bus.ConsoleOut.Write([]byte{ch})
		}
		return 4

	case 133: // Hyper GetChar (returns character in register B or 0, clears A)
		b := c.Bus.ReadByte(0xFF01)
		c.B = b
		c.A = 0
		return 4

	default:
		return 2
	}
}

// hyperPrintf handles GOMAR-style formatted printing from 6809 memory.
// varPtr points to the format string pointer on the stack, followed by argument words.
func (c *CPU) hyperPrintf(varPtr uint16) {
	fmtAddr := c.Bus.ReadWord(varPtr)
	varPtr += 2

	// Read null-terminated format string from memory
	var formatBytes []byte
	for count := 0; count < 65536; count++ {
		ch := c.Bus.ReadByte(fmtAddr)
		if ch == 0 {
			break
		}
		formatBytes = append(formatBytes, ch)
		fmtAddr++
	}
	format := string(formatBytes)

	var sb strings.Builder
	for i := 0; i < len(format); i++ {
		ch := format[i]
		if ch == '%' && i+1 < len(format) {
			i++
			start := i
			for i < len(format) && ((format[i] >= '0' && format[i] <= '9') || format[i] == '-' || format[i] == '+' || format[i] == '0' || format[i] == '#' || format[i] == ' ' || format[i] == '.') {
				i++
			}
			if i >= len(format) {
				sb.WriteByte('%')
				sb.WriteString(format[start:])
				break
			}
			spec := format[start : i+1]
			kind := format[i]

			switch kind {
			case '%':
				sb.WriteByte('%')
			case 'c':
				b := c.Bus.ReadByte(varPtr + 1)
				sb.WriteString(fmt.Sprintf("%"+spec, b))
				varPtr += 2
			case 'x':
				val := c.Bus.ReadWord(varPtr)
				if spec == "x" {
					sb.WriteString(fmt.Sprintf("$%04x", val))
				} else {
					sb.WriteString(fmt.Sprintf("%"+spec, val))
				}
				varPtr += 2
			case 'd':
				val := int16(c.Bus.ReadWord(varPtr))
				sb.WriteString(fmt.Sprintf("%"+spec, val))
				varPtr += 2
			case 'u':
				val := c.Bus.ReadWord(varPtr)
				formatVerb := spec[:len(spec)-1] + "d"
				sb.WriteString(fmt.Sprintf("%"+formatVerb, val))
				varPtr += 2
			case 's', 'q':
				strAddr := c.Bus.ReadWord(varPtr)
				varPtr += 2
				var strBytes []byte
				if strAddr != 0 {
					for count := 0; count < 65536; count++ {
						b := c.Bus.ReadByte(strAddr)
						if b == 0 {
							break
						}
						strBytes = append(strBytes, b)
						strAddr++
					}
				}
				sb.WriteString(fmt.Sprintf("%"+spec, string(strBytes)))
			default:
				sb.WriteByte('%')
				sb.WriteString(spec)
			}
		} else {
			sb.WriteByte(ch)
		}
	}

	if c.Bus.ConsoleOut != nil {
		c.Bus.ConsoleOut.Write([]byte(sb.String()))
	}
}
