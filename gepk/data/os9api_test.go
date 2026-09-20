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

	cCreate := FindCall(0x85)
	if cCreate == nil || cCreate.Name != "I$Create" {
		t.Fatalf("expected I$Create for 0x85, got %v", cCreate)
	}

	cExit := FindCall(0x06)
	if cExit == nil || cExit.Name != "F$Exit" {
		t.Fatalf("expected F$Exit for 0x06, got %v", cExit)
	}
}

func TestFormatCallAndResult(t *testing.T) {
	c := FindCall(0x8C)
	formatted := FormatCall(c, 0x8C, 1, 5, 0x00010000, 0, nil, func(addr uint32, maxLen int) string {
		return "hello"
	})
	if !strings.Contains(formatted, "I$WritLn(path=1, buf=$00010000, num_bytes=5): \"hello\"") {
		t.Fatalf("unexpected FormatCall output: %s", formatted)
	}

	// Test success result: SR Carry bit = 0
	resSuccess := FormatResult(c, 0x8C, 0x0000, 0, 1, 5, 0, 0, 0x00010000, nil)
	if !strings.Contains(resSuccess, "I$WritLn: actual_num_bytes=5 (OK)") {
		t.Fatalf("unexpected FormatResult success output: %s", resSuccess)
	}

	// Test error result: SR Carry bit = 1
	resErr := FormatResult(c, 0x8C, 0x0001, 217, 0, 0, 0, 0, 0x00010000, nil)
	if !strings.Contains(resErr, "ERROR 217 (E$BPNum") {
		t.Fatalf("unexpected FormatResult error output: %s", resErr)
	}
}
