package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHatvanVMKHello(t *testing.T) {
	// Build temporary test srec file
	srecLines := []string{
		"S00F000068656C6C6F202020202000003C",
		// Vectors: SSP=0x00080000, PC=0x00001000 at 0x00000000
		"S30D000000000008000000001000DA",
		// Code at 0x00001000:
		// 1000: 41F9 0000 1020        LEA $1020, A0
		// 1006: 1018                  MOVE.B (A0)+, D0
		// 1008: 6708                  BEQ done ($1012)
		// 100A: 13C0 00FF 0000        MOVE.B D0, ($00FF0000).L
		// 1010: 60F4                  BRA loop ($1006)
		// 1012: done:
		// 1012: 33FC 002A 00FF 000A   MOVE.W #42, ($00FF000A).L
		// 101A: 4E71                  NOP
		"S3210000100041F9000010201018670813C000FF000060F433FC002A00FF000A4E7186",
		// String at 0x00001020: "Hello, Hatvan VM/K!\n\0"
		"S31A0000102048656C6C6F2C2048617476616E20564D2F4B210A00AB",
		// S7 entry at 0x00001000
		"S70500001000EA",
	}

	tmpDir := t.TempDir()
	srecPath := filepath.Join(tmpDir, "hello.s37")
	if err := os.WriteFile(srecPath, []byte(strings.Join(srecLines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("Failed to write test srec: %v", err)
	}

	binPath := filepath.Join(tmpDir, "gepk")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build gepk: %v\nOutput: %s", err, string(out))
	}
	cmd := exec.Command(binPath, "--trace", "--max-cycles", "1000", srecPath)
	cmd.Dir = "../.."
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	// Exit code should be 42
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 42 {
			t.Fatalf("Expected exit code 42, got %d. Stderr:\n%s", exitErr.ExitCode(), stderr.String())
		}
	} else if err != nil {
		t.Fatalf("Command failed unexpectedly: %v. Stderr:\n%s", err, stderr.String())
	}

	output := stdout.String()
	expected := "Hello, Hatvan VM/K!\n"
	if output != expected {
		t.Fatalf("Stdout mismatch:\nGot:      %q\nExpected: %q\nStderr:\n%s", output, expected, stderr.String())
	}
}
