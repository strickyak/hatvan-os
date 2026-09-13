package vm

import (
	"encoding/binary"
	"strings"
)

// OS9LoadedModule represents an OS-9 module located in memory.
type OS9LoadedModule struct {
	Name       string
	BaseAddr   uint16
	Size       uint16
	TypeLang   byte
	AttrRev    byte
	ExecOffset uint16
	CRC        uint32
}

// ScanOS9Modules scans a memory range for valid OS-9 module headers.
func ScanOS9Modules(mem []byte, startAddr, endAddr uint16) []*OS9LoadedModule {
	var modules []*OS9LoadedModule
	addr := startAddr

	for int(addr) < int(endAddr)-9 && int(addr)+9 <= len(mem) {
		if mem[addr] == 0x87 && mem[addr+1] == 0xCD {
			size := binary.BigEndian.Uint16(mem[addr+2 : addr+4])
			if size < 9 || int(addr)+int(size) > len(mem) {
				addr++
				continue
			}

			// Validate header parity check
			var parity byte
			for i := 0; i < 8; i++ {
				parity ^= mem[int(addr)+i]
			}
			if ^parity != mem[addr+8] {
				addr++
				continue
			}

			nameOff := binary.BigEndian.Uint16(mem[addr+4 : addr+6])
			if int(nameOff) >= int(size) {
				addr++
				continue
			}

			// Extract module name (last char has bit 7 set)
			var sb strings.Builder
			nPtr := int(addr) + int(nameOff)
			for nPtr < int(addr)+int(size) && nPtr < len(mem) {
				c := mem[nPtr]
				sb.WriteByte(c & 0x7F)
				nPtr++
				if (c & 0x80) != 0 {
					break
				}
			}
			modName := strings.ToLower(sb.String())

			execOff := binary.BigEndian.Uint16(mem[addr+9 : addr+11])

			mod := &OS9LoadedModule{
				Name:       modName,
				BaseAddr:   addr,
				Size:       size,
				TypeLang:   mem[addr+6],
				AttrRev:    mem[addr+7],
				ExecOffset: execOff,
			}
			modules = append(modules, mod)

			// Advance by module size
			addr += size
			continue
		}
		addr++
	}

	return modules
}
