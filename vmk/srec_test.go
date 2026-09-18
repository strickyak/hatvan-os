package vmk

import (
	"strings"
	"testing"
)

func TestLoadSRecords(t *testing.T) {
	bus := NewBus()

	// S-Record test stream:
	// S0: Header
	// S1: 4 bytes at 0x1000: 0x11, 0x22, 0x33, 0x44
	// S9: Termination at 0x1000
	srecData := `
S00F000068656C6C6F202020202000003C
S1071000112233443E
S9031000EC
`
	entry, hasEntry, err := bus.LoadSRecords(strings.NewReader(srecData))
	if err != nil {
		t.Fatalf("LoadSRecords failed: %v", err)
	}
	if !hasEntry {
		t.Fatalf("expected hasEntry = true")
	}
	if entry != 0x1000 {
		t.Fatalf("entry = 0x%04X, want 0x1000", entry)
	}

	if b := bus.Tasks[0].readByte(0x1000); b != 0x11 {
		t.Fatalf("Task 0 [0x1000] = 0x%02X, want 0x11", b)
	}
	if b := bus.Tasks[0].readByte(0x1001); b != 0x22 {
		t.Fatalf("Task 0 [0x1001] = 0x%02X, want 0x22", b)
	}
	if b := bus.Tasks[0].readByte(0x1002); b != 0x33 {
		t.Fatalf("Task 0 [0x1002] = 0x%02X, want 0x33", b)
	}
	if b := bus.Tasks[0].readByte(0x1003); b != 0x44 {
		t.Fatalf("Task 0 [0x1003] = 0x%02X, want 0x44", b)
	}
}
