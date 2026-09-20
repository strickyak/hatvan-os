package data

import (
	"strings"
	"testing"
)

func TestFindCall(t *testing.T) {
	c := FindCall(0x8C)
	if c == nil || c.Name != "I$WritLn" {
		t.Fatalf("expected I$WritLn for 0x8C, got %v", c)
	}

	cDup := FindCall(0x82)
	if cDup == nil || cDup.Name != "I$Dup" {
		t.Fatalf("expected I$Dup for 0x82, got %v", cDup)
	}

	cExit := FindCall(0x06)
	if cExit == nil || cExit.Name != "F$Exit" {
		t.Fatalf("expected F$Exit for 0x06, got %v", cExit)
	}
}

func TestFormatCallAndResult(t *testing.T) {
	c := FindCall(0x8C)
	formatted := FormatCall(c, 0x8C, 1, 0, 0, 0x0100, 5, 0, nil, func(addr uint16, maxLen int) string {
		return "hello"
	})
	if !strings.Contains(formatted, "I$WritLn(path=1, buf=$0100, num_bytes=5): \"hello\"") {
		t.Fatalf("unexpected FormatCall output: %s", formatted)
	}

	// Test success result
	resSuccess := FormatResult(c, 0x8C, 0x80, 1, 0, 0, 0, 5, 0, 0x0100, nil)
	if !strings.Contains(resSuccess, "I$WritLn: actual_num_bytes=5 (OK)") {
		t.Fatalf("unexpected FormatResult success output: %s", resSuccess)
	}

	// Test error result (Carry set: 0x81)
	resErr := FormatResult(c, 0x8C, 0x81, 0, 217, 0, 0, 0, 0, 0x0100, nil)
	if !strings.Contains(resErr, "ERROR 217 (E$BPNum") {
		t.Fatalf("unexpected FormatResult error output: %s", resErr)
	}
}
