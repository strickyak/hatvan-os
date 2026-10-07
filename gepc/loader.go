package gepc

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// LoadedBinary holds segments and execution entry point.
type LoadedBinary struct {
	EntryPC   uint16
	HasEntry  bool
	BaseAddr  uint16
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

		if tag == 0x00 { // Data chunk
			data := make([]byte, length)
			if _, err := io.ReadFull(r, data); err != nil {
				return nil, fmt.Errorf("error reading data chunk at 0x%04X: %w", addr, err)
			}
			for i, byteVal := range data {
				b.Memory[task][addr+uint16(i)] = byteVal
			}
			lb.TotalSize += int(length)
			if lb.BaseAddr == 0 {
				lb.BaseAddr = addr
			}
		} else if tag == 0xFF { // Trailer / Entry point
			lb.EntryPC = addr
			lb.HasEntry = true
			break
		} else if tag == 0xFD { // Architecture magic marker ('x', <arch>)
			if header[3] == 'x' && header[4] != 'c' {
				return nil, fmt.Errorf("incompatible architecture magic in DECB: expected 'xc', got 'x%c'", header[4])
			}
			if length > 0 {
				discard := make([]byte, length)
				io.ReadFull(r, discard)
			}
		} else { // Skip unknown or debug chunks
			if length > 0 {
				discard := make([]byte, length)
				io.ReadFull(r, discard)
			}
		}
	}
	return lb, nil
}

// LoadRawBinary loads a raw binary file into Task 0 starting at baseAddr.
func (b *Bus) LoadRawBinary(r io.Reader, baseAddr uint16) (*LoadedBinary, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	for i, byteVal := range data {
		b.Memory[0][baseAddr+uint16(i)] = byteVal
	}
	return &LoadedBinary{
		EntryPC:   baseAddr,
		HasEntry:  true,
		BaseAddr:  baseAddr,
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

	if header[0] == 0x00 || header[0] == 0xFD { // DECB
		return b.LoadDECBIntoTask(task, f)
	}
	return b.LoadRawBinary(f, 0x0000)
}
