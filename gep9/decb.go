package gep9

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Standard and Extended DECB Chunk Types
const (
	ChunkData    = 0x00 // Standard binary data chunk
	ChunkSrcAbs  = 0x01 // Extended: Absolute source line
	ChunkModDef  = 0x02 // Extended: OS-9 module definition
	ChunkSrcRel  = 0x03 // Extended: OS-9 module-relative source line
	ChunkSymAbs  = 0x04 // Extended: Absolute symbol
	ChunkSymRel  = 0x05 // Extended: OS-9 module-relative symbol
	ChunkSrcFile = 0x06 // Extended: Source file name declaration
	ChunkExec    = 0xFF // Standard execution address trailer
)

// DataSegment represents a contiguous block of binary data loaded into memory.
type DataSegment struct {
	Addr uint16
	Data []byte
}

// SourceLine stores line number and text for a source line.
type SourceLine struct {
	LineNum uint16
	Text    string
}

// OS9Module stores metadata and relative debugging information for an OS-9 module.
type OS9Module struct {
	Name       string
	Size       uint16
	CRC        uint32
	ExecOffset uint16
	Lines      map[uint16]SourceLine
	Symbols    map[string]uint16
}

// DECB stores all binary segments and debug information loaded from an Extended DECB file.
type DECB struct {
	Segments         []DataSegment
	ExecAddr         uint16
	HasExec          bool
	AbsLines         map[uint16]SourceLine
	AbsSymbols       map[string]uint16
	AbsSymbolsByAddr map[uint16]string
	Modules          []*OS9Module
	Files            map[uint16]string
}

// NewDECB creates an initialized DECB structure.
func NewDECB() *DECB {
	return &DECB{
		AbsLines:         make(map[uint16]SourceLine),
		AbsSymbols:       make(map[string]uint16),
		AbsSymbolsByAddr: make(map[uint16]string),
		Files:            make(map[uint16]string),
	}
}

// LoadDECB reads an Extended DECB stream.
func LoadDECB(r io.Reader) (*DECB, error) {
	d := NewDECB()
	header := make([]byte, 5)
	var activeMod *OS9Module

	for {
		_, err := io.ReadFull(r, header)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, fmt.Errorf("reading decb chunk header: %w", err)
		}

		chunkType := header[0]
		length := binary.BigEndian.Uint16(header[1:3])
		addr := binary.BigEndian.Uint16(header[3:5])

		payload := make([]byte, length)
		if length > 0 {
			if _, err := io.ReadFull(r, payload); err != nil {
				return nil, fmt.Errorf("reading decb chunk payload (type 0x%02X, len %d): %w", chunkType, length, err)
			}
		}

		switch chunkType {
		case ChunkData:
			dataCopy := make([]byte, length)
			copy(dataCopy, payload)
			d.Segments = append(d.Segments, DataSegment{
				Addr: addr,
				Data: dataCopy,
			})

		case ChunkExec:
			d.ExecAddr = addr
			d.HasExec = true

		case ChunkSrcAbs:
			var lineNum uint16
			text := string(payload)
			if length >= 2 {
				lineNum = binary.BigEndian.Uint16(payload[0:2])
				text = string(payload[2:])
			}
			d.AbsLines[addr] = SourceLine{LineNum: lineNum, Text: text}

		case ChunkModDef:
			var crc uint32
			var execOffset uint16
			var name string
			if length >= 5 {
				crc = (uint32(payload[0]) << 16) | (uint32(payload[1]) << 8) | uint32(payload[2])
				execOffset = binary.BigEndian.Uint16(payload[3:5])
				name = string(payload[5:])
			} else {
				name = string(payload)
			}
			mod := &OS9Module{
				Name:       name,
				Size:       addr,
				CRC:        crc,
				ExecOffset: execOffset,
				Lines:      make(map[uint16]SourceLine),
				Symbols:    make(map[string]uint16),
			}
			d.Modules = append(d.Modules, mod)
			activeMod = mod

		case ChunkSrcRel:
			var lineNum uint16
			text := string(payload)
			if length >= 2 {
				lineNum = binary.BigEndian.Uint16(payload[0:2])
				text = string(payload[2:])
			}
			if activeMod != nil {
				activeMod.Lines[addr] = SourceLine{LineNum: lineNum, Text: text}
			}

		case ChunkSymAbs:
			symName := string(payload)
			d.AbsSymbols[symName] = addr
			d.AbsSymbolsByAddr[addr] = symName

		case ChunkSymRel:
			symName := string(payload)
			if activeMod != nil {
				activeMod.Symbols[symName] = addr
			}

		case ChunkSrcFile:
			d.Files[addr] = string(payload)

		default:
			// Safely skip any unknown chunk types
		}
	}

	return d, nil
}

// LoadDECBFile loads an extended DECB file from disk.
func LoadDECBFile(path string) (*DECB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadDECB(f)
}
