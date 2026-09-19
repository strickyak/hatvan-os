package gepk

import (
	"testing"
)

func TestCPUReset(t *testing.T) {
	bus := NewBus()
	// Set Vector 0 (SSP) = 0x00080000
	bus.WriteLong(0x000000, 0x00080000)
	// Set Vector 1 (PC) = 0x00001000
	bus.WriteLong(0x000004, 0x00001000)

	cpu := NewCPU(bus)
	cpu.Reset()

	if cpu.SSP != 0x00080000 || cpu.A[7] != 0x00080000 {
		t.Fatalf("Reset SSP = 0x%08X, A[7] = 0x%08X, want 0x00080000", cpu.SSP, cpu.A[7])
	}
	if cpu.PC != 0x00001000 {
		t.Fatalf("Reset PC = 0x%08X, want 0x00001000", cpu.PC)
	}
	if (cpu.SR & FlagS) == 0 {
		t.Fatalf("Reset should enter Supervisor mode (FlagS set)")
	}
	if (cpu.SR & MaskIPL) != MaskIPL {
		t.Fatalf("Reset should mask interrupts (IPL=7)")
	}
}

func TestCPUPrivilegeSwap(t *testing.T) {
	cpu := newTestCPU()
	cpu.A[7] = 0x00080000 // SSP
	cpu.SSP = 0x00080000
	cpu.USP = 0x00040000

	// Switch to User Mode
	cpu.SetSR(0x0000) // S=0
	if (cpu.SR & FlagS) != 0 {
		t.Fatalf("SR should be in user mode")
	}
	if cpu.A[7] != 0x00040000 {
		t.Fatalf("In user mode, A[7] should be USP (0x00040000), got 0x%08X", cpu.A[7])
	}
	if cpu.SSP != 0x00080000 {
		t.Fatalf("SSP was corrupted: 0x%08X", cpu.SSP)
	}

	// Modify active SP in user mode
	cpu.A[7] = 0x0003FFFE

	// Switch back to Supervisor Mode
	cpu.SetSR(FlagS)
	if cpu.A[7] != 0x00080000 {
		t.Fatalf("In supervisor mode, A[7] should be SSP (0x00080000), got 0x%08X", cpu.A[7])
	}
	if cpu.USP != 0x0003FFFE {
		t.Fatalf("USP should be preserved as 0x0003FFFE, got 0x%08X", cpu.USP)
	}
}

func TestCPUTrapAndRTE(t *testing.T) {
	bus := NewBus()
	// Set Vector 0 (SSP) = 0x00080000
	bus.WriteLong(0x000000, 0x00080000)
	// Set Vector 1 (PC) = 0x00002000 (User code entry)
	bus.WriteLong(0x000004, 0x00002000)
	// Set Vector 32 (TRAP #0 handler) = 0x00001000 (Supervisor handler)
	bus.WriteLong(0x000080, 0x00001000)

	// Supervisor handler at 0x1000: RTE (0x4E73) in Task 0
	bus.WriteWord(0x1000, 0x4E73)

	cpu := NewCPU(bus)
	cpu.Reset()

	// Switch to User Mode with USP = 0x00050000
	cpu.USP = 0x00050000
	cpu.SetSR(0x0000) // User mode

	// User code at 0x2000: TRAP #0 (0x4E40) in Task 1
	bus.WriteWord(0x2000, 0x4E40)

	// Step 1: Execute TRAP #0
	cycles := cpu.Step()
	if cycles <= 0 {
		t.Fatalf("Expected CPU cycles > 0")
	}

	// CPU should now be in Supervisor mode at handler 0x1000
	if (cpu.SR & FlagS) == 0 {
		t.Fatalf("After TRAP #0, CPU should be in Supervisor mode")
	}
	if cpu.PC != 0x00001000 {
		t.Fatalf("After TRAP #0, PC should be 0x00001000, got 0x%08X", cpu.PC)
	}
	if cpu.A[7] != 0x00080000-6 {
		t.Fatalf("SSP should have pushed SR (2 bytes) and PC (4 bytes), got 0x%08X", cpu.A[7])
	}

	// Step 2: Execute RTE
	cpu.Step()

	// CPU should return to User mode at 0x2002
	if (cpu.SR & FlagS) != 0 {
		t.Fatalf("After RTE, CPU should be back in User mode")
	}
	if cpu.PC != 0x00002002 {
		t.Fatalf("After RTE, PC should be 0x00002002, got 0x%08X", cpu.PC)
	}
	if cpu.A[7] != 0x00050000 {
		t.Fatalf("After RTE, A[7] should be USP (0x00050000), got 0x%08X", cpu.A[7])
	}
}

func TestCPUEndToEndProgram(t *testing.T) {
	bus := NewBus()
	// Set Vector 0 (SSP) = 0x00080000
	bus.WriteLong(0x000000, 0x00080000)
	// Set Vector 1 (PC) = 0x00001000
	bus.WriteLong(0x000004, 0x00001000)

	// Program at 0x1000:
	// 0x1000: 7005           MOVEQ #5, D0
	// 0x1002: 7200           MOVEQ #0, D1
	// loop:
	// 0x1004: D280           ADD.L D0, D1
	// 0x1006: 5380           SUBQ.L #1, D0
	// 0x1008: 66FA           BNE loop (-6 bytes from 0x100A -> 0x1004)
	// 0x100A: 33C1 00FF 000A MOVE.W D1, ($00FF000A).L  (writes result to Exit.Code)
	// 0x1010: 4E71           NOP

	bus.WriteWord(0x1000, 0x7005)
	bus.WriteWord(0x1002, 0x7200)
	bus.WriteWord(0x1004, 0xD280)
	bus.WriteWord(0x1006, 0x5380)
	bus.WriteWord(0x1008, 0x66FA)
	bus.WriteWord(0x100A, 0x33C1)
	bus.WriteWord(0x100C, 0x00FF)
	bus.WriteWord(0x100E, 0x000A)
	bus.WriteWord(0x1010, 0x4E71)

	cpu := NewCPU(bus)
	cpu.Reset()

	for step := 0; step < 100; step++ {
		if bus.ExitCode != 0 {
			break
		}
		cpu.Step()
	}

	if bus.ExitCode != 15 {
		t.Fatalf("Program completed with ExitCode = %d, want 15 (sum 1..5)", bus.ExitCode)
	}
	if cpu.D[1] != 15 {
		t.Fatalf("D1 = %d, want 15", cpu.D[1])
	}
	if cpu.D[0] != 0 {
		t.Fatalf("D0 = %d, want 0", cpu.D[0])
	}
}
