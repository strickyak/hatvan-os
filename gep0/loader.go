package gep0

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// LoadedBinary holds segment summary and execution entry point (CS:IP).
type LoadedBinary struct {
	EntryCS   uint16
	EntryIP   uint16
	HasEntry  bool
	BaseAddr  uint32
	TotalSize int
}

// LoadDECB loads an Extended or Standard DECB file into Task 0.
func (b *Bus) LoadDECB(r io.Reader) (*LoadedBinary, error) {
	return b.LoadDECBIntoTask(0, r)
}

// LoadDECBIntoTask loads an Extended or Standard DECB file into target task.
func (b *Bus) LoadDECBIntoTask(task uint8, r io.Reader) (*LoadedBinary, error) {
	lb := &LoadedBinary{}
	header := make([]byte, 5)
	high16 := uint32(0)
	codeHigh16 := uint32(0)
	mem := b.taskMem(task)

	for {
		_, err := io.ReadFull(r, header)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, err
		}

		tag := header[0]
		length := binary.BigEndian.Uint16(header[1:3])
		addr := binary.BigEndian.Uint16(header[3:5])

		switch tag {
		case 0x00: // DECB_DATA chunk
			data := make([]byte, length)
			if _, err := io.ReadFull(r, data); err != nil {
				return nil, fmt.Errorf("error reading data chunk at 0x%04X: %w", addr, err)
			}
			base20 := ((high16 << 16) | uint32(addr)) & 0xFFFFF
			for i, byteVal := range data {
				mem[(base20+uint32(i))&0xFFFFF] = byteVal
			}
			if high16 != 0 {
				codeHigh16 = high16
			}
			lb.TotalSize += int(length)
			if lb.BaseAddr == 0 && lb.TotalSize == int(length) {
				lb.BaseAddr = base20
			}

		case 0xFD: // Architecture magic marker ('x', '0')
			if header[3] == 'x' && header[4] != '0' {
				return nil, fmt.Errorf("incompatible architecture magic in DECB: expected 'x0', got 'x%c'", header[4])
			}
			if length > 0 {
				discard := make([]byte, length)
				io.ReadFull(r, discard)
			}

		case 0xFE: // SET_HIGH16_ADDR32
			high16 = uint32(addr)

		case 0xFF: // Execution postamble
			lb.EntryIP = addr
			lb.HasEntry = true
			if high16 != 0 {
				lb.EntryCS = uint16((high16 & 0x0F) << 12)
			} else if codeHigh16 != 0 {
				lb.EntryCS = uint16((codeHigh16 & 0x0F) << 12)
			} else {
				lb.EntryCS = 0x0000
			}
			return lb, nil

		default: // Skip unknown or debug chunks
			if length > 0 {
				discard := make([]byte, length)
				io.ReadFull(r, discard)
			}
		}
	}
	return lb, nil
}

// LoadRawBinary loads a raw binary file into Task 0 starting at CS=0x1000, IP=baseAddr
// (and also mirrors to 0x0000:baseAddr if baseAddr < 0xE000 for tiny-mode test snippets).
func (b *Bus) LoadRawBinary(r io.Reader, baseAddr uint16) (*LoadedBinary, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	mem := b.taskMem(0)
	for i, byteVal := range data {
		off := uint32(baseAddr) + uint32(i)
		if off < 0x10000 {
			mem[0x10000+off] = byteVal
		}
	}
	return &LoadedBinary{
		EntryCS:   0x1000,
		EntryIP:   baseAddr,
		HasEntry:  true,
		BaseAddr:  0x10000 + uint32(baseAddr),
		TotalSize: len(data),
	}, nil
}

// LoadFile automatically detects DECB or raw binary and loads it into Task 0.
func (b *Bus) LoadFile(path string) (*LoadedBinary, error) {
	return b.LoadFileIntoTask(0, path)
}

// LoadFileIntoTask automatically detects DECB or raw binary and loads it into target task.
func (b *Bus) LoadFileIntoTask(task uint8, path string) (*LoadedBinary, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header := make([]byte, 1)
	if _, err := f.Read(header); err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	if header[0] == 0x00 || header[0] == 0xFD || header[0] == 0xFE { // DECB
		return b.LoadDECBIntoTask(task, f)
	}
	return b.LoadRawBinary(f, 0x0000)
}
