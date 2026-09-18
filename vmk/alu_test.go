package vmk

import (
	"testing"
)

func TestALUAddSubFlags(t *testing.T) {
	cpu := newTestCPU()

	// ADD 0x7F + 0x01 (signed overflow: +127 + 1 = -128)
	cpu.SR = 0
	res := cpu.Add(0x7F, 0x01, SizeByte)
	if res != 0x80 {
		t.Fatalf("Add(0x7F, 0x01) = 0x%02X, want 0x80", res)
	}
	if (cpu.SR & FlagV) == 0 {
		t.Fatalf("Add 0x7F + 0x01 should set FlagV")
	}
	if (cpu.SR & FlagN) == 0 {
		t.Fatalf("Add 0x7F + 0x01 should set FlagN")
	}
	if (cpu.SR & FlagC) != 0 {
		t.Fatalf("Add 0x7F + 0x01 should NOT set FlagC")
	}

	// ADD 0xFF + 0x01 (carry out)
	cpu.SR = 0
	res = cpu.Add(0xFF, 0x01, SizeByte)
	if res != 0x00 {
		t.Fatalf("Add(0xFF, 0x01) = 0x%02X, want 0x00", res)
	}
	if (cpu.SR & FlagZ) == 0 {
		t.Fatalf("Add 0xFF + 0x01 should set FlagZ")
	}
	if (cpu.SR & FlagC) == 0 {
		t.Fatalf("Add 0xFF + 0x01 should set FlagC")
	}
	if (cpu.SR & FlagX) == 0 {
		t.Fatalf("Add 0xFF + 0x01 should set FlagX")
	}

	// SUB 0x00 - 0x01 (borrow)
	cpu.SR = 0
	res = cpu.Sub(0x00, 0x01, SizeByte)
	if res != 0xFF {
		t.Fatalf("Sub(0x00, 0x01) = 0x%02X, want 0xFF", res)
	}
	if (cpu.SR & FlagC) == 0 {
		t.Fatalf("Sub 0x00 - 0x01 should set FlagC")
	}
	if (cpu.SR & FlagX) == 0 {
		t.Fatalf("Sub 0x00 - 0x01 should set FlagX")
	}
	if (cpu.SR & FlagN) == 0 {
		t.Fatalf("Sub 0x00 - 0x01 should set FlagN")
	}
}

func TestALUAddXSubXZeroFlag(t *testing.T) {
	cpu := newTestCPU()

	// In M68000 ADDX and SUBX:
	// If result == 0, Z flag is UNCHANGED (not set)!
	// If result != 0, Z flag is CLEARED!

	// Case 1: Z=1 initially, result=0 -> Z remains 1
	cpu.SR = FlagZ
	res := cpu.AddX(0, 0, SizeByte)
	if res != 0 {
		t.Fatalf("AddX(0, 0) = %d, want 0", res)
	}
	if (cpu.SR & FlagZ) == 0 {
		t.Fatalf("AddX with zero result should leave FlagZ unchanged (still 1)")
	}

	// Case 2: Z=0 initially, result=0 -> Z remains 0!
	cpu.SR = 0
	res = cpu.AddX(0, 0, SizeByte)
	if res != 0 {
		t.Fatalf("AddX(0, 0) = %d, want 0", res)
	}
	if (cpu.SR & FlagZ) != 0 {
		t.Fatalf("AddX with zero result must NOT set FlagZ if it was 0")
	}

	// Case 3: Z=1 initially, result!=0 -> Z cleared to 0
	cpu.SR = FlagZ
	res = cpu.AddX(5, 2, SizeByte)
	if res != 7 {
		t.Fatalf("AddX(5, 2) = %d, want 7", res)
	}
	if (cpu.SR & FlagZ) != 0 {
		t.Fatalf("AddX with non-zero result must clear FlagZ")
	}
}

func TestEvaluateCondition(t *testing.T) {
	cpu := newTestCPU()

	// T (0) and F (1)
	if !cpu.EvaluateCondition(0) {
		t.Fatalf("Cond T should be true")
	}
	if cpu.EvaluateCondition(1) {
		t.Fatalf("Cond F should be false")
	}

	// EQ (7) and NE (6)
	cpu.SR = FlagZ
	if !cpu.EvaluateCondition(7) {
		t.Fatalf("Cond EQ should be true when Z=1")
	}
	if cpu.EvaluateCondition(6) {
		t.Fatalf("Cond NE should be false when Z=1")
	}

	cpu.SR = 0
	if cpu.EvaluateCondition(7) {
		t.Fatalf("Cond EQ should be false when Z=0")
	}
	if !cpu.EvaluateCondition(6) {
		t.Fatalf("Cond NE should be true when Z=0")
	}

	// Signed GE (12), LT (13), GT (14), LE (15)
	// Positive, non-zero: N=0, V=0, Z=0
	cpu.SR = 0
	if !cpu.EvaluateCondition(12) { // GE: N==V
		t.Fatalf("GE should be true for N=0, V=0")
	}
	if cpu.EvaluateCondition(13) { // LT: N!=V
		t.Fatalf("LT should be false for N=0, V=0")
	}
	if !cpu.EvaluateCondition(14) { // GT: N==V and Z=0
		t.Fatalf("GT should be true for N=0, V=0, Z=0")
	}

	// Negative: N=1, V=0, Z=0
	cpu.SR = FlagN
	if cpu.EvaluateCondition(12) {
		t.Fatalf("GE should be false for N=1, V=0")
	}
	if !cpu.EvaluateCondition(13) {
		t.Fatalf("LT should be true for N=1, V=0")
	}
}

func TestALUShiftsRotates(t *testing.T) {
	cpu := newTestCPU()

	// LSL by 2
	cpu.SR = 0
	res := cpu.Lsl(0b00110001, 2, SizeByte)
	if res != 0b11000100 {
		t.Fatalf("Lsl(0x31, 2) = 0x%02X, want 0xC4", res)
	}
	// Last bit shifted out was 0
	if (cpu.SR & FlagC) != 0 {
		t.Fatalf("Lsl carry should be 0")
	}

	// LSR by 1
	cpu.SR = 0
	res = cpu.Lsr(0b00000011, 1, SizeByte)
	if res != 0b00000001 {
		t.Fatalf("Lsr(0x03, 1) = 0x%02X, want 0x01", res)
	}
	if (cpu.SR & FlagC) == 0 {
		t.Fatalf("Lsr carry should be 1")
	}

	// ASR sign replication
	cpu.SR = 0
	res = cpu.Asr(0x80, 2, SizeByte)
	if res != 0xE0 {
		t.Fatalf("Asr(0x80, 2) = 0x%02X, want 0xE0", res)
	}

	// ROL
	cpu.SR = 0
	res = cpu.Rol(0x81, 1, SizeByte)
	if res != 0x03 {
		t.Fatalf("Rol(0x81, 1) = 0x%02X, want 0x03", res)
	}
}

func TestALUMulDiv(t *testing.T) {
	cpu := newTestCPU()

	// MULU 100 * 200 = 20000
	res := cpu.Mulu(100, 200)
	if res != 20000 {
		t.Fatalf("Mulu(100, 200) = %d, want 20000", res)
	}

	// MULS -5 * 10 = -50
	res = cpu.Muls(-5, 10)
	if int32(res) != -50 {
		t.Fatalf("Muls(-5, 10) = %d, want -50", int32(res))
	}

	// DIVU 20005 / 100 = quot 200, rem 5
	divRes, ok := cpu.Divu(20005, 100)
	if !ok {
		t.Fatalf("Divu should have succeeded")
	}
	quot := divRes & 0xFFFF
	rem := divRes >> 16
	if quot != 200 || rem != 5 {
		t.Fatalf("Divu quot=%d rem=%d, want quot=200 rem=5", quot, rem)
	}
}

func TestALUBCD(t *testing.T) {
	cpu := newTestCPU()

	// ABCD: 0x48 + 0x35 = 0x83
	cpu.SR = 0
	res := cpu.Abcd(0x48, 0x35)
	if res != 0x83 {
		t.Fatalf("Abcd(0x48, 0x35) = 0x%02X, want 0x83", res)
	}

	// ABCD with carry: 0x79 + 0x25 = 0x04, Carry=1
	cpu.SR = 0
	res = cpu.Abcd(0x79, 0x25)
	if res != 0x04 {
		t.Fatalf("Abcd(0x79, 0x25) = 0x%02X, want 0x04", res)
	}
	if (cpu.SR & FlagC) == 0 {
		t.Fatalf("Abcd should set FlagC")
	}

	// SBCD: 0x52 - 0x18 = 0x34
	cpu.SR = 0
	res = cpu.Sbcd(0x52, 0x18)
	if res != 0x34 {
		t.Fatalf("Sbcd(0x52, 0x18) = 0x%02X, want 0x34", res)
	}
}
