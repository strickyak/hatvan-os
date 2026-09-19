package gepk

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// LoadSRecords parses Motorola S-Records from reader and writes them into Task 0 memory.
// Returns the entry address if specified by an S7/S8/S9 termination record.
func (b *Bus) LoadSRecords(r io.Reader) (entryAddr uint32, hasEntry bool, err error) {
	return b.LoadSRecordsIntoTask(0, r)
}

// LoadSRecordsIntoTask parses Motorola S-Records from reader and writes them into the specified task memory.
func (b *Bus) LoadSRecordsIntoTask(task uint8, r io.Reader) (entryAddr uint32, hasEntry bool, err error) {
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}
		if line[0] != 'S' && line[0] != 's' {
			continue // skip comments or blank lines
		}
		if len(line) < 4 {
			return 0, false, fmt.Errorf("line %d: S-record too short: %q", lineNum, line)
		}

		recType := line[1]
		hexData, err := hex.DecodeString(line[2:])
		if err != nil {
			return 0, false, fmt.Errorf("line %d: invalid hex in %q: %w", lineNum, line, err)
		}

		if len(hexData) < 2 {
			return 0, false, fmt.Errorf("line %d: record payload too short", lineNum)
		}

		count := int(hexData[0])
		if len(hexData) != count+1 {
			return 0, false, fmt.Errorf("line %d: record byte count mismatch: header %d, actual %d", lineNum, count, len(hexData)-1)
		}

		// Verify checksum (one's complement of sum of bytes)
		var csum byte = 0
		for _, b := range hexData {
			csum += b
		}
		if csum != 0xFF {
			return 0, false, fmt.Errorf("line %d: checksum failure: sum is 0x%02X, expected 0xFF", lineNum, csum)
		}

		payload := hexData[1 : len(hexData)-1]

		switch recType {
		case '0':
			// Header record, ignore

		case '1': // Data with 16-bit address
			if len(payload) < 2 {
				return 0, false, fmt.Errorf("line %d: S1 record payload too short for 16-bit address", lineNum)
			}
			addr := (uint32(payload[0]) << 8) | uint32(payload[1])
			data := payload[2:]
			b.mu.Lock()
			for i, v := range data {
				b.Tasks[task].writeByte(addr+uint32(i), v)
			}
			b.mu.Unlock()

		case '2': // Data with 24-bit address
			if len(payload) < 3 {
				return 0, false, fmt.Errorf("line %d: S2 record payload too short for 24-bit address", lineNum)
			}
			addr := (uint32(payload[0]) << 16) | (uint32(payload[1]) << 8) | uint32(payload[2])
			data := payload[3:]
			b.mu.Lock()
			for i, v := range data {
				b.Tasks[task].writeByte(addr+uint32(i), v)
			}
			b.mu.Unlock()

		case '3': // Data with 32-bit address
			if len(payload) < 4 {
				return 0, false, fmt.Errorf("line %d: S3 record payload too short for 32-bit address", lineNum)
			}
			addr := (uint32(payload[0]) << 24) | (uint32(payload[1]) << 16) | (uint32(payload[2]) << 8) | uint32(payload[3])
			data := payload[4:]
			b.mu.Lock()
			for i, v := range data {
				b.Tasks[task].writeByte(addr+uint32(i), v)
			}
			b.mu.Unlock()

		case '7': // Termination with 32-bit entry address
			if len(payload) >= 4 {
				entryAddr = (uint32(payload[0]) << 24) | (uint32(payload[1]) << 16) | (uint32(payload[2]) << 8) | uint32(payload[3])
				hasEntry = true
			}

		case '8': // Termination with 24-bit entry address
			if len(payload) >= 3 {
				entryAddr = (uint32(payload[0]) << 16) | (uint32(payload[1]) << 8) | uint32(payload[2])
				hasEntry = true
			}

		case '9': // Termination with 16-bit entry address
			if len(payload) >= 2 {
				entryAddr = (uint32(payload[0]) << 8) | uint32(payload[1])
				hasEntry = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, false, err
	}

	return entryAddr, hasEntry, nil
}
