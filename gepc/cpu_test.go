package gepc

import (
	"bytes"
	"strings"
	"testing"
)

func newTestCPU() (*Bus, *CPU) {
	bus := NewBus()
	cpu := NewCPU(bus)
	return bus, cpu
}

func TestALUAdd(t *testing.T) {
	bus, cpu := newTestCPU()

	// LDI $42; ADI $10; ADI $B0 (overflow 0x42+0x10+0xB0 = 0x102 -> D=0x02, DF=1)
	prog := []byte{
		0xF8, 0x42, // LDI $42
		0xFC, 0x10, // ADI $10
		0xFC, 0xB0, // ADI $B0
	}
	for i, b := range prog {
		bus.Memory[0][i] = b
	}

	cpu.Step() // LDI $42
	if cpu.D != 0x42 || cpu.DF != 0 {
		t.Fatalf("expected D=0x42 DF=0, got D=0x%02X DF=%d", cpu.D, cpu.DF)
	}

	cpu.Step() // ADI $10 -> 0x52, no carry
	if cpu.D != 0x52 || cpu.DF != 0 {
		t.Fatalf("expected D=0x52 DF=0, got D=0x%02X DF=%d", cpu.D, cpu.DF)
	}

	cpu.Step() // ADI $B0 -> 0x02, carry=1
	if cpu.D != 0x02 || cpu.DF != 1 {
		t.Fatalf("expected D=0x02 DF=1, got D=0x%02X DF=%d", cpu.D, cpu.DF)
	}
}

func TestALUAdc(t *testing.T) {
	bus, cpu := newTestCPU()

	// Set DF=1, LDI $05, ADCI $02 -> 0x05 + 0x02 + 1 = 0x08, DF=0
	prog := []byte{
		0xF8, 0x05, // LDI $05
		0x7C, 0x02, // ADCI $02
	}
	for i, b := range prog {
		bus.Memory[0][i] = b
	}
	cpu.DF = 1

	cpu.Step() // LDI $05 (does not affect DF)
	cpu.Step() // ADCI $02
	if cpu.D != 0x08 || cpu.DF != 0 {
		t.Fatalf("expected D=0x08 DF=0, got D=0x%02X DF=%d", cpu.D, cpu.DF)
	}
}

func TestALUSubtract(t *testing.T) {
	bus, cpu := newTestCPU()

	// 1802 Subtraction Borrow Convention:
	// DF = 1 means NO BORROW (result >= 0)
	// DF = 0 means BORROW (result < 0)

	// SMI: D - imm
	// Case 1: 0x50 - 0x20 = 0x30, no borrow -> DF=1
	// Case 2: 0x30 - 0x40 = 0xF0, borrow -> DF=0
	prog := []byte{
		0xF8, 0x50, // LDI $50
		0xFF, 0x20, // SMI $20 -> D=0x30, DF=1
		0xFF, 0x40, // SMI $40 -> D=0xF0, DF=0
	}
	for i, b := range prog {
		bus.Memory[0][i] = b
	}

	cpu.Step()
	cpu.Step()
	if cpu.D != 0x30 || cpu.DF != 1 {
		t.Fatalf("SMI no-borrow failed: D=0x%02X DF=%d (expected D=0x30 DF=1)", cpu.D, cpu.DF)
	}

	cpu.Step()
	if cpu.D != 0xF0 || cpu.DF != 0 {
		t.Fatalf("SMI borrow failed: D=0x%02X DF=%d (expected D=0xF0 DF=0)", cpu.D, cpu.DF)
	}

	// SDI: imm - D
	// D is currently 0xF0.
	// SDI $FF -> 0xFF - 0xF0 = 0x0F, no borrow -> DF=1
	bus.Memory[0][6] = 0xFD
	bus.Memory[0][7] = 0xFF
	cpu.Step()
	if cpu.D != 0x0F || cpu.DF != 1 {
		t.Fatalf("SDI no-borrow failed: D=0x%02X DF=%d (expected D=0x0F DF=1)", cpu.D, cpu.DF)
	}
}

func TestALUShifts(t *testing.T) {
	bus, cpu := newTestCPU()

	// SHR: LSB -> DF, 0 -> MSB
	// SHRC: LSB -> DF, old DF -> MSB
	// SHL: MSB -> DF, 0 -> LSB
	// SHLC: MSB -> DF, old DF -> LSB
	prog := []byte{
		0xF8, 0x85, // LDI $85 (1000 0101)
		0xF6,       // SHR -> D=0x42 (0100 0010), DF=1
		0x76,       // SHRC -> D=0xA1 (1010 0001), DF=0
		0xFE,       // SHL -> D=0x42 (0100 0010), DF=1
		0x7E,       // SHLC -> D=0x85 (1000 0101), DF=0
	}
	for i, b := range prog {
		bus.Memory[0][i] = b
	}

	cpu.Step() // LDI
	cpu.Step() // SHR
	if cpu.D != 0x42 || cpu.DF != 1 {
		t.Fatalf("SHR failed: D=0x%02X DF=%d (want D=0x42 DF=1)", cpu.D, cpu.DF)
	}

	cpu.Step() // SHRC
	if cpu.D != 0xA1 || cpu.DF != 0 {
		t.Fatalf("SHRC failed: D=0x%02X DF=%d (want D=0xA1 DF=0)", cpu.D, cpu.DF)
	}

	cpu.Step() // SHL
	if cpu.D != 0x42 || cpu.DF != 1 {
		t.Fatalf("SHL failed: D=0x%02X DF=%d (want D=0x42 DF=1)", cpu.D, cpu.DF)
	}

	cpu.Step() // SHLC
	if cpu.D != 0x85 || cpu.DF != 0 {
		t.Fatalf("SHLC failed: D=0x%02X DF=%d (want D=0x85 DF=0)", cpu.D, cpu.DF)
	}
}

func TestRegisterTransfersAndMemory(t *testing.T) {
	bus, cpu := newTestCPU()

	// Load R7 with $1234 using LDI, PHI, PLO
	// Store D to M(R7) using STR R7
	// Read back via LDN R7 and LDA R7
	prog := []byte{
		0xF8, 0x12, // LDI $12
		0xB7,       // PHI R7
		0xF8, 0x34, // LDI $34
		0xA7,       // PLO R7
		0xF8, 0x99, // LDI $99
		0x57,       // STR R7 -> M($1234) = $99
		0xF8, 0x00, // LDI $00
		0x07,       // LDN R7 -> D = $99, R7 remains $1234
		0x47,       // LDA R7 -> D = $99, R7 becomes $1235
		0x17,       // INC R7 -> R7 becomes $1236
		0x27,       // DEC R7 -> R7 becomes $1235
	}
	for i, b := range prog {
		bus.Memory[0][i] = b
	}

	for i := 0; i < 4; i++ {
		cpu.Step()
	}
	if cpu.R[7] != 0x1234 {
		t.Fatalf("expected R7=0x1234, got 0x%04X", cpu.R[7])
	}

	cpu.Step() // LDI $99
	cpu.Step() // STR R7
	if bus.Memory[0][0x1234] != 0x99 {
		t.Fatalf("STR R7 failed: memory at 0x1234 is 0x%02X (want 0x99)", bus.Memory[0][0x1234])
	}
	if cpu.R[7] != 0x1234 {
		t.Fatalf("STR R7 should not modify R7, got 0x%04X", cpu.R[7])
	}

	cpu.Step() // LDI $00
	cpu.Step() // LDN R7
	if cpu.D != 0x99 || cpu.R[7] != 0x1234 {
		t.Fatalf("LDN R7 failed: D=0x%02X R7=0x%04X", cpu.D, cpu.R[7])
	}

	cpu.Step() // LDA R7
	if cpu.D != 0x99 || cpu.R[7] != 0x1235 {
		t.Fatalf("LDA R7 failed: D=0x%02X R7=0x%04X", cpu.D, cpu.R[7])
	}

	cpu.Step() // INC R7
	if cpu.R[7] != 0x1236 {
		t.Fatalf("INC R7 failed: R7=0x%04X", cpu.R[7])
	}

	cpu.Step() // DEC R7
	if cpu.R[7] != 0x1235 {
		t.Fatalf("DEC R7 failed: R7=0x%04X", cpu.R[7])
	}
}

func TestShortAndLongBranches(t *testing.T) {
	bus, cpu := newTestCPU()

	// Short branch: BR $0020
	bus.Memory[0][0x0000] = 0x30 // BR
	bus.Memory[0][0x0001] = 0x20
	// Long branch at 0x0020: LBR $0567
	bus.Memory[0][0x0020] = 0xC0 // LBR
	bus.Memory[0][0x0021] = 0x05
	bus.Memory[0][0x0022] = 0x67

	cpu.Step() // BR $0020
	if cpu.PC() != 0x0020 {
		t.Fatalf("short branch failed: PC=0x%04X (want 0x0020)", cpu.PC())
	}

	cpu.Step() // LBR $0567
	if cpu.PC() != 0x0567 {
		t.Fatalf("long branch failed: PC=0x%04X (want 0x0567)", cpu.PC())
	}
}

func TestSkips(t *testing.T) {
	bus, cpu := newTestCPU()

	// SKP at 0x0000 -> skips 1 byte to 0x0002
	// LSKP at 0x0002 -> skips 2 bytes to 0x0005
	bus.Memory[0][0x0000] = 0x38 // SKP
	bus.Memory[0][0x0001] = 0xFF // skipped byte
	bus.Memory[0][0x0002] = 0xC8 // LSKP
	bus.Memory[0][0x0003] = 0xFF // skipped byte 1
	bus.Memory[0][0x0004] = 0xEE // skipped byte 2
	bus.Memory[0][0x0005] = 0xC4 // NOP

	cpu.Step() // SKP
	if cpu.PC() != 0x0002 {
		t.Fatalf("SKP failed: PC=0x%04X (want 0x0002)", cpu.PC())
	}

	cpu.Step() // LSKP
	if cpu.PC() != 0x0005 {
		t.Fatalf("LSKP failed: PC=0x%04X (want 0x0005)", cpu.PC())
	}
}

func TestHardwareFlags(t *testing.T) {
	bus, cpu := newTestCPU()

	// B1: branch if EF1 active (char available)
	// BN1: branch if EF1 inactive (no char available)
	bus.Memory[0][0x0000] = 0x3C // BN1 $0010
	bus.Memory[0][0x0001] = 0x10
	bus.Memory[0][0x0010] = 0x34 // B1 $0020
	bus.Memory[0][0x0011] = 0x20

	cpu.Step() // BN1 with empty input -> should take branch to 0x0010
	if cpu.PC() != 0x0010 {
		t.Fatalf("BN1 failed on empty input: PC=0x%04X (want 0x0010)", cpu.PC())
	}

	// Now enqueue a character
	bus.EnqueueKey('A')

	cpu.Step() // B1 with char available -> should take branch to 0x0020
	if cpu.PC() != 0x0020 {
		t.Fatalf("B1 failed on available char: PC=0x%04X (want 0x0020)", cpu.PC())
	}
}

func TestTrapAndUntrapFuses(t *testing.T) {
	bus, cpu := newTestCPU()

	// 1. Setup User Process in Task 5
	bus.CurrentTask = 5
	cpu.P = 3 // R3 is PC
	cpu.R[3] = 0x0200
	cpu.R[14] = 0x0004 // RE points to trampoline at $0004

	// In Task 5:
	// At $0200: SEP RE (switch P to RE=0x0004)
	// At $0201: inline syscall byte $84 (I$Open)
	// At $0202: user code resumes here!
	bus.Memory[5][0x0200] = 0xDE // SEP RE
	bus.Memory[5][0x0201] = 0x84 // CALL_NUM = $84
	bus.Memory[5][0x0202] = 0xC4 // NOP (resumed)

	// Trampoline at $0004 in Task 5:
	// SEX R3 (point RX to user PC R3)
	// OUT 6  (strobe Port 6 trap!)
	bus.Memory[5][0x0004] = 0xE3 // SEX R3
	bus.Memory[5][0x0005] = 0x66 // OUT 6

	// In Task 0 (Kernel mode):
	// Address $0006 contains kernel entry handler
	bus.Memory[0][0x0006] = 0xC0 // LBR $0800
	bus.Memory[0][0x0007] = 0x08
	bus.Memory[0][0x0008] = 0x00

	// Step 1: User executes SEP RE at $0200
	cpu.Step()
	if cpu.P != 14 || cpu.PC() != 0x0004 {
		t.Fatalf("SEP RE failed: P=%d PC=0x%04X", cpu.P, cpu.PC())
	}
	if bus.CurrentTask != 5 {
		t.Fatalf("task changed unexpectedly: %d", bus.CurrentTask)
	}

	// Step 2: Trampoline executes SEX R3 at $0004
	cpu.Step()
	if cpu.X != 3 {
		t.Fatalf("SEX R3 failed: X=%d", cpu.X)
	}

	// Step 3: Trampoline executes OUT 6 at $0005
	// This should:
	// - Read byte at R3 (0x0201) -> $84
	// - Increment R3 to 0x0202
	// - Latch $84 into $FF28 (TrapSyscallNum)
	// - Record TrapSourceTask = 5
	// - Flip CurrentTask to 0!
	cpu.Step()
	if bus.CurrentTask != 0 {
		t.Fatalf("OUT 6 failed to flip to Task 0, got task %d", bus.CurrentTask)
	}
	if bus.TrapSyscallNum != 0x84 {
		t.Fatalf("OUT 6 failed to latch syscall code, got 0x%02X (want 0x84)", bus.TrapSyscallNum)
	}
	if bus.TrapSourceTask != 5 {
		t.Fatalf("OUT 6 failed to record source task, got %d (want 5)", bus.TrapSourceTask)
	}
	if cpu.R[3] != 0x0202 {
		t.Fatalf("OUT 6 should have advanced R3 past inline byte to 0x0202, got 0x%04X", cpu.R[3])
	}
	if cpu.PC() != 0x0006 {
		t.Fatalf("Next instruction should be fetched from Task 0 at $0006, got 0x%04X", cpu.PC())
	}

	// Step 4: Kernel executes in Task 0
	cpu.Step() // LBR $0800
	if cpu.PC() != 0x0800 {
		t.Fatalf("Kernel jump failed: PC=0x%04X (want 0x0800)", cpu.PC())
	}

	// 2. Test Untrap Fuse (Kernel Task 0 -> User Task 5)
	// Kernel sets up return:
	// Address $0800:
	// SEX RF
	// OUT 7  (where M(RF) = 5, the target task!)
	// SEP R3 (flips MMU back to Task 5 and sets P=3!)
	cpu.R[15] = 0x0850
	bus.Memory[0][0x0850] = 5 // target task ID

	bus.Memory[0][0x0800] = 0xEF // SEX RF
	bus.Memory[0][0x0801] = 0x67 // OUT 7
	bus.Memory[0][0x0802] = 0xD3 // SEP R3

	cpu.Step() // SEX RF
	if cpu.X != 15 {
		t.Fatalf("SEX RF failed: X=%d", cpu.X)
	}

	cpu.Step() // OUT 7
	if !bus.TaskFuseArmed || bus.TaskFuseTarget != 5 {
		t.Fatalf("OUT 7 failed: armed=%v target=%d", bus.TaskFuseArmed, bus.TaskFuseTarget)
	}
	if bus.CurrentTask != 0 {
		t.Fatalf("OUT 7 should not flip task immediately: task=%d", bus.CurrentTask)
	}

	cpu.Step() // SEP R3 -> MMU flips task to 5!
	if bus.CurrentTask != 5 {
		t.Fatalf("SEP R3 failed to flip to target task 5, got task %d", bus.CurrentTask)
	}
	if bus.TaskFuseArmed {
		t.Fatalf("TaskFuse should be disarmed after SEP, got armed=%v", bus.TaskFuseArmed)
	}
	if cpu.P != 3 || cpu.PC() != 0x0202 {
		t.Fatalf("user execution failed to resume: P=%d PC=0x%04X (want P=3 PC=0x0202)", cpu.P, cpu.PC())
	}
}

func TestMemoryProtection(t *testing.T) {
	bus, _ := newTestCPU()

	bus.CurrentTask = 1 // User task without privilege

	// Reading $FF00 in Task 1 should trigger panic ErrUserAccessTrap
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic on user access to $FF00, got nil")
		}
		err, ok := r.(error)
		if !ok || !strings.Contains(err.Error(), "protected page") {
			t.Fatalf("expected ErrUserAccessTrap, got: %v", r)
		}
	}()

	_ = bus.ReadByte(0xFF00)
}

func TestLoaderMagic(t *testing.T) {
	bus, _ := newTestCPU()

	// 1. DECB with valid magic 'x', 'c'
	validDecb := []byte{
		0xFD, 0x00, 0x00, 'x', 'c', // Magic header
		0x00, 0x00, 0x02, 0x02, 0x00, 0x12, 0x34, // Data: 2 bytes at $0200: $12, $34
		0xFF, 0x00, 0x00, 0x02, 0x00, // Exec trailer: $0200
	}
	lb, err := bus.LoadDECB(bytes.NewReader(validDecb))
	if err != nil {
		t.Fatalf("valid DECB failed to load: %v", err)
	}
	if !lb.HasEntry || lb.EntryPC != 0x0200 {
		t.Fatalf("expected entry 0x0200, got 0x%04X", lb.EntryPC)
	}
	if bus.Memory[0][0x0200] != 0x12 || bus.Memory[0][0x0201] != 0x34 {
		t.Fatalf("data chunk loaded incorrectly: %02X %02X", bus.Memory[0][0x0200], bus.Memory[0][0x0201])
	}

	// 2. DECB with wrong magic 'x', '9' -> must return error
	wrongDecb := []byte{
		0xFD, 0x00, 0x00, 'x', '9',
		0xFF, 0x00, 0x00, 0x02, 0x00,
	}
	_, err = bus.LoadDECB(bytes.NewReader(wrongDecb))
	if err == nil {
		t.Fatalf("expected architecture mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "incompatible architecture magic") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
