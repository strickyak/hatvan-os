package gepk

import (
	"testing"
)

func newTestCPU() *CPU {
	bus := NewBus()
	bus.CurrentFC = FCSupervisorData
	cpu := NewCPU(bus)
	cpu.SR = FlagS
	return cpu
}

func TestAddressingDirect(t *testing.T) {
	cpu := newTestCPU()

	// Mode 0: Dn
	cpu.D[3] = 0x12345678
	ea := cpu.ResolveEA(0, 3, SizeByte)
	if v := cpu.ReadEA(ea); v != 0x78 {
		t.Fatalf("ReadEA(D3, Byte) = 0x%02X, want 0x78", v)
	}
	eaW := cpu.ResolveEA(0, 3, SizeWord)
	if v := cpu.ReadEA(eaW); v != 0x5678 {
		t.Fatalf("ReadEA(D3, Word) = 0x%04X, want 0x5678", v)
	}
	eaL := cpu.ResolveEA(0, 3, SizeLong)
	if v := cpu.ReadEA(eaL); v != 0x12345678 {
		t.Fatalf("ReadEA(D3, Long) = 0x%08X, want 0x12345678", v)
	}

	// Write Dn
	cpu.WriteEA(ea, 0x99)
	if cpu.D[3] != 0x12345699 {
		t.Fatalf("WriteEA(D3, Byte) = 0x%08X, want 0x12345699", cpu.D[3])
	}
	cpu.WriteEA(eaW, 0xABCD)
	if cpu.D[3] != 0x1234ABCD {
		t.Fatalf("WriteEA(D3, Word) = 0x%08X, want 0x1234ABCD", cpu.D[3])
	}

	// Mode 1: An
	cpu.A[2] = 0x00010000
	eaA := cpu.ResolveEA(1, 2, SizeWord)
	// Word write to An sign-extends to 32 bits
	cpu.WriteEA(eaA, 0x8000)
	if cpu.A[2] != 0xFFFF8000 {
		t.Fatalf("WriteEA(A2, Word=0x8000) = 0x%08X, want 0xFFFF8000 (sign extended)", cpu.A[2])
	}
	cpu.WriteEA(eaA, 0x7000)
	if cpu.A[2] != 0x00007000 {
		t.Fatalf("WriteEA(A2, Word=0x7000) = 0x%08X, want 0x00007000", cpu.A[2])
	}
}

func TestAddressingIndirect(t *testing.T) {
	cpu := newTestCPU()

	cpu.A[1] = 0x00020000
	cpu.Bus.WriteLong(0x00020000, 0xCAFEBABE)

	// Mode 2: (An)
	ea := cpu.ResolveEA(2, 1, SizeLong)
	if ea.Addr != 0x00020000 {
		t.Fatalf("ResolveEA((A1)) addr = 0x%08X, want 0x00020000", ea.Addr)
	}
	if v := cpu.ReadEA(ea); v != 0xCAFEBABE {
		t.Fatalf("ReadEA((A1)) = 0x%08X, want 0xCAFEBABE", v)
	}
}

func TestAddressingPostincPredec(t *testing.T) {
	cpu := newTestCPU()

	// Mode 3: (An)+
	cpu.A[2] = 0x00004000
	ea := cpu.ResolveEA(3, 2, SizeWord)
	if ea.Addr != 0x00004000 {
		t.Fatalf("(A2)+ addr = 0x%08X, want 0x00004000", ea.Addr)
	}
	cpu.CommitEA(ea)
	if cpu.A[2] != 0x00004002 {
		t.Fatalf("(A2)+ Word commit: A2 = 0x%08X, want 0x00004002", cpu.A[2])
	}

	// Mode 4: -(An)
	cpu.A[3] = 0x00005004
	eaDec := cpu.ResolveEA(4, 3, SizeLong)
	if eaDec.Addr != 0x00005000 {
		t.Fatalf("-(A3) Long addr = 0x%08X, want 0x00005000", eaDec.Addr)
	}
	if cpu.A[3] != 0x00005000 {
		t.Fatalf("-(A3) Long A3 = 0x%08X, want 0x00005000", cpu.A[3])
	}

	// Hardware Quirk: A7 byte operations move by 2!
	cpu.A[7] = 0x00006000
	eaSPInc := cpu.ResolveEA(3, 7, SizeByte)
	cpu.CommitEA(eaSPInc)
	if cpu.A[7] != 0x00006002 {
		t.Fatalf("(A7)+ Byte: A7 = 0x%08X, want 0x00006002 (quirk: +2 instead of +1)", cpu.A[7])
	}

	eaSPDec := cpu.ResolveEA(4, 7, SizeByte)
	if eaSPDec.Addr != 0x00006000 {
		t.Fatalf("-(A7) Byte: addr = 0x%08X, want 0x00006000 (quirk: -2 instead of -1)", eaSPDec.Addr)
	}
	if cpu.A[7] != 0x00006000 {
		t.Fatalf("-(A7) Byte: A7 = 0x%08X, want 0x00006000", cpu.A[7])
	}
}

func TestAddressingDisplacementAndIndex(t *testing.T) {
	cpu := newTestCPU()

	// Mode 5: (d16, An)
	cpu.A[4] = 0x00008000
	cpu.PC = 0x00001000
	// Write 16-bit signed displacement -0x10 (-16)
	cpu.Bus.WriteWord(0x00001000, 0xFFF0)
	eaDisp := cpu.ResolveEA(5, 4, SizeWord)
	if eaDisp.Addr != 0x00007FF0 {
		t.Fatalf("(d16, A4) addr = 0x%08X, want 0x00007FF0", eaDisp.Addr)
	}

	// Mode 6: (d8, An, Xn)
	cpu.A[5] = 0x0000A000
	cpu.D[1] = 0x00000020 // index +32
	cpu.PC = 0x00001002
	// Extension word: D1.W index, disp = -4 (0xFC)
	// ext = 0001 (D1) 0000 (word) 1111 1100 (-4) -> 0x10FC
	cpu.Bus.WriteWord(0x00001002, 0x10FC)
	eaIdx := cpu.ResolveEA(6, 5, SizeWord)
	if eaIdx.Addr != 0x0000A01C { // 0xA000 + 32 - 4 = 0xA01C
		t.Fatalf("(d8, A5, D1.W) addr = 0x%08X, want 0x0000A01C", eaIdx.Addr)
	}
}

func TestAddressingSpecialModes(t *testing.T) {
	cpu := newTestCPU()

	// Mode 7, Reg 0: (xxx).W Absolute Short
	cpu.PC = 0x00001000
	cpu.Bus.WriteWord(0x00001000, 0x8000)
	eaShort := cpu.ResolveEA(7, 0, SizeWord)
	if eaShort.Addr != 0xFFFF8000 {
		t.Fatalf("(xxx).W sign-extended = 0x%08X, want 0xFFFF8000", eaShort.Addr)
	}

	// Mode 7, Reg 1: (xxx).L Absolute Long
	cpu.PC = 0x00001002
	cpu.Bus.WriteWord(0x00001002, 0x0012)
	cpu.Bus.WriteWord(0x00001004, 0x3456)
	eaLong := cpu.ResolveEA(7, 1, SizeLong)
	if eaLong.Addr != 0x00123456 {
		t.Fatalf("(xxx).L addr = 0x%08X, want 0x00123456", eaLong.Addr)
	}

	// Mode 7, Reg 2: (d16, PC)
	cpu.PC = 0x00002000
	cpu.Bus.WriteWord(0x00002000, 0x0040) // +64 bytes
	eaPCD := cpu.ResolveEA(7, 2, SizeWord)
	if eaPCD.Addr != 0x00002040 {
		t.Fatalf("(d16, PC) addr = 0x%08X, want 0x00002040", eaPCD.Addr)
	}

	// Mode 7, Reg 4: Immediate #<data>
	cpu.PC = 0x00003000
	cpu.Bus.WriteWord(0x00003000, 0x00AB)
	eaImmB := cpu.ResolveEA(7, 4, SizeByte)
	if eaImmB.ImmVal != 0xAB {
		t.Fatalf("#<data> Byte = 0x%02X, want 0xAB", eaImmB.ImmVal)
	}

	cpu.PC = 0x00003002
	cpu.Bus.WriteWord(0x00003002, 0xBEEF)
	eaImmW := cpu.ResolveEA(7, 4, SizeWord)
	if eaImmW.ImmVal != 0xBEEF {
		t.Fatalf("#<data> Word = 0x%04X, want 0xBEEF", eaImmW.ImmVal)
	}

	cpu.PC = 0x00003004
	cpu.Bus.WriteWord(0x00003004, 0x1234)
	cpu.Bus.WriteWord(0x00003006, 0x5678)
	eaImmL := cpu.ResolveEA(7, 4, SizeLong)
	if eaImmL.ImmVal != 0x12345678 {
		t.Fatalf("#<data> Long = 0x%08X, want 0x12345678", eaImmL.ImmVal)
	}
}
