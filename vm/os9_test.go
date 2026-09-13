package vm

import (
	"bytes"
	"testing"
)

func TestScanOS9Modules(t *testing.T) {
	mem := make([]byte, 65536)

	// Create a mock OS-9 module at 0xD000
	// Header:
	// 0x87, 0xCD : Sync
	// 0x00, 0x20 : Size 32 bytes
	// 0x00, 0x0D : Name offset 13 ($0D)
	// 0x11       : Type/Lang
	// 0x81       : Attr/Rev
	// Parity byte: complement of XOR of previous 8 bytes
	// 0x00, 0x10 : Exec offset
	// Name at offset 13: "mymod" with high bit on last char: 'm', 'y', 'm', 'o', 'd'|0x80
	base := 0xD000
	mem[base+0] = 0x87
	mem[base+1] = 0xCD
	mem[base+2] = 0x00
	mem[base+3] = 0x20
	mem[base+4] = 0x00
	mem[base+5] = 0x0D
	mem[base+6] = 0x11
	mem[base+7] = 0x81

	var parity byte
	for i := 0; i < 8; i++ {
		parity ^= mem[base+i]
	}
	mem[base+8] = ^parity

	mem[base+9] = 0x00
	mem[base+10] = 0x10

	// Name at base + 13
	nameBytes := []byte{'m', 'y', 'm', 'o', 'd' | 0x80}
	copy(mem[base+13:], nameBytes)

	mods := ScanOS9Modules(mem, 0xC000, 0xE000)
	if len(mods) != 1 {
		t.Fatalf("expected 1 module, got %d", len(mods))
	}

	m := mods[0]
	if m.Name != "mymod" {
		t.Errorf("expected module name %q, got %q", "mymod", m.Name)
	}
	if m.BaseAddr != 0xD000 {
		t.Errorf("expected base addr 0xD000, got 0x%04X", m.BaseAddr)
	}
	if m.Size != 32 {
		t.Errorf("expected size 32, got %d", m.Size)
	}
	if m.ExecOffset != 0x0010 {
		t.Errorf("expected exec offset 0x0010, got 0x%04X", m.ExecOffset)
	}
}

func TestListingOffsetCopy(t *testing.T) {
	listText := `0000 8642         (test.asm):00010 start  lda #$42
0002 7E1234       (test.asm):00011        jmp $1234
`
	l, err := LoadListing(bytes.NewBufferString(listText), "mymod.list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l.ModuleName != "mymod" {
		t.Errorf("expected module name 'mymod', got %q", l.ModuleName)
	}

	shifted := l.OffsetCopy(0xD000)
	line0, ok := shifted.LinesByAddr[0xD000]
	if !ok || line0.LineNum != 10 {
		t.Errorf("expected shifted line at 0xD000, got %+v (ok=%v)", line0, ok)
	}
	line2, ok := shifted.LinesByAddr[0xD002]
	if !ok || line2.LineNum != 11 {
		t.Errorf("expected shifted line at 0xD002, got %+v (ok=%v)", line2, ok)
	}
}
