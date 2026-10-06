package gepz

import (
	"bytes"
	"errors"
	"testing"
)

func TestZ80BasicExecution(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// Simple program:
	// ld a, 40
	// ld b, 2
	// add a, b
	// halt
	prog := []byte{
		0x3E, 40,   // LD A, 40
		0x06, 2,    // LD B, 2
		0x80,       // ADD A, B
		0x76,       // HALT
	}
	copy(bus.Memory[0][0x1000:], prog)
	cpu.PC = 0x1000

	for !cpu.Halted {
		cpu.Step()
	}

	if cpu.A != 42 {
		t.Fatalf("expected A = 42, got %d", cpu.A)
	}
	if (cpu.F & FlagZ) != 0 {
		t.Fatalf("expected Zero flag clear, got %02X", cpu.F)
	}
}

func TestZ80StackAndCall(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// Main:
	//   ld sp, $2000
	//   call subr
	//   halt
	// subr:
	//   ld a, 99
	//   ret
	prog := []byte{
		0x31, 0x00, 0x20,       // LD SP, $2000
		0xCD, 0x07, 0x10,       // CALL $1007
		0x76,                   // HALT
		0x3E, 99,               // LD A, 99
		0xC9,                   // RET
	}
	copy(bus.Memory[0][0x1000:], prog)
	cpu.PC = 0x1000

	for !cpu.Halted {
		cpu.Step()
	}

	if cpu.A != 99 {
		t.Fatalf("expected A = 99, got %d", cpu.A)
	}
	if cpu.SP != 0x2000 {
		t.Fatalf("expected SP restored to $2000, got %04X", cpu.SP)
	}
}

func TestZ80LDIR(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// Copy 4 bytes from $1020 to $1030 using LDIR
	copy(bus.Memory[0][0x1020:], []byte{0xDE, 0xAD, 0xBE, 0xEF})

	// ld hl, $1020
	// ld de, $1030
	// ld bc, 4
	// ldir
	// halt
	prog := []byte{
		0x21, 0x20, 0x10, // LD HL, $1020
		0x11, 0x30, 0x10, // LD DE, $1030
		0x01, 0x04, 0x00, // LD BC, 4
		0xED, 0xB0,       // LDIR
		0x76,             // HALT
	}
	copy(bus.Memory[0][0x1000:], prog)
	cpu.PC = 0x1000

	for !cpu.Halted {
		cpu.Step()
	}

	dest := bus.Memory[0][0x1030 : 0x1030+4]
	expected := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if !bytes.Equal(dest, expected) {
		t.Fatalf("LDIR copy mismatch: got %X, expected %X", dest, expected)
	}
	if cpu.GetBC() != 0 {
		t.Fatalf("expected BC = 0 after LDIR, got %d", cpu.GetBC())
	}
}

func TestZ80ConsoleAndExit(t *testing.T) {
	bus := NewBus()
	var out bytes.Buffer
	bus.ConsoleOut = &out
	cpu := NewCPU(bus)

	// Write 'H', 'i', '\n' to $FF00, then write 42 to $FF05 (exit)
	prog := []byte{
		0x3E, 'H',        // LD A, 'H'
		0x32, 0x00, 0xFF, // LD ($FF00), A
		0x3E, 'i',        // LD A, 'i'
		0x32, 0x00, 0xFF, // LD ($FF00), A
		0x3E, '\n',       // LD A, '\n'
		0x32, 0x00, 0xFF, // LD ($FF00), A
		0x3E, 42,         // LD A, 42
		0x32, 0x05, 0xFF, // LD ($FF05), A (Exit)
	}
	copy(bus.Memory[0][0x1000:], prog)
	cpu.PC = 0x1000

	for !cpu.Halted {
		cpu.Step()
	}

	if out.String() != "Hi\n" {
		t.Fatalf("expected console output 'Hi\\n', got %q", out.String())
	}
	if bus.ExitCode != 42 {
		t.Fatalf("expected ExitCode = 42, got %d", bus.ExitCode)
	}
}

func TestZ80Port60TrapAndNMI(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// Setup NMI vector at $0066 in Task 0:
	// Kernel NMI handler:
	//   ld a, 77
	//   halt
	kernelNMI := []byte{
		0x3E, 77, // LD A, 77
		0x76,     // HALT
	}
	copy(bus.Memory[0][0x0066:], kernelNMI)

	// User code in Task 1 at $0100:
	//   ld sp, $2000
	//   ld a, $84       ; Syscall I$Open
	//   out ($60), a    ; Trigger syscall trap!
	//   halt            ; Should not be reached before NMI
	userCode := []byte{
		0x31, 0x00, 0x20, // LD SP, $2000
		0x3E, 0x84,       // LD A, $84
		0xD3, 0x60,       // OUT ($60), A
		0x76,             // HALT
	}
	copy(bus.Memory[1][0x0100:], userCode)

	// Boot into Task 1
	bus.CurrentTask = 1
	cpu.PC = 0x0100

	// Step 1: LD SP, $2000
	cpu.Step()
	// Step 2: LD A, $84
	cpu.Step()
	// Step 3: OUT ($60), A (sets TrapPending, triggers NMI on next step)
	cpu.Step()

	if !bus.TrapPending {
		t.Fatalf("expected TrapPending = true after OUT (60), A")
	}
	if bus.TrapVal != 0x84 {
		t.Fatalf("expected TrapVal = 0x84, got %02X", bus.TrapVal)
	}

	// Step 4: Step() services NMI!
	cpu.Step()

	// Should now be in Task 0 at PC = 0x0066!
	if bus.CurrentTask != 0 {
		t.Fatalf("expected CurrentTask = 0 after NMI trap, got %d", bus.CurrentTask)
	}
	if cpu.PC != 0x0066 {
		t.Fatalf("expected PC = 0x0066 after NMI trap, got %04X", cpu.PC)
	}

	// The return address pushed onto Task 1 stack ($1FFE) should be $0107
	retLo := bus.Memory[1][0x1FFE]
	retHi := bus.Memory[1][0x1FFF]
	retPC := (uint16(retHi) << 8) | uint16(retLo)
	if retPC != 0x0107 {
		t.Fatalf("expected pushed return PC = 0x0107 on user stack, got %04X", retPC)
	}

	// Run kernel handler to halt
	for !cpu.Halted {
		cpu.Step()
	}
	if cpu.A != 77 {
		t.Fatalf("expected A = 77 from kernel handler, got %d", cpu.A)
	}
}

func TestZ80TaskFuseAndRETN(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// Setup Task 1 user memory:
	// Stack at $1FFE has return address $0200
	bus.Memory[1][0x1FFE] = 0x00
	bus.Memory[1][0x1FFF] = 0x02

	// Code at $0200 in Task 1:
	//   ld a, 88
	//   halt
	userCode := []byte{
		0x3E, 88, // LD A, 88
		0x76,     // HALT
	}
	copy(bus.Memory[1][0x0200:], userCode)

	// Kernel code in Task 0 at $1000:
	//   ld sp, $1FFE      ; Point SP at user return address
	//   ld a, 1
	//   ld ($FF20), a     ; Arm TaskFuse with Task 1
	//   retn              ; Return from NMI
	kernelCode := []byte{
		0x31, 0xFE, 0x1F, // LD SP, $1FFE
		0x3E, 1,          // LD A, 1
		0x32, 0x20, 0xFF, // LD ($FF20), A
		0xED, 0x45,       // RETN
	}
	copy(bus.Memory[0][0x1000:], kernelCode)

	bus.CurrentTask = 0
	cpu.PC = 0x1000

	// Step through kernel return sequence
	cpu.Step() // LD SP
	cpu.Step() // LD A
	cpu.Step() // LD ($FF20), A -> arms TaskFuse
	if !bus.TaskFuseArmed || bus.TaskFuseTarget != 1 {
		t.Fatalf("expected TaskFuse armed to target 1")
	}

	// Step RETN: switches to Task 1, pops $0200 from Task 1 stack!
	cpu.Step()

	if bus.CurrentTask != 1 {
		t.Fatalf("expected CurrentTask = 1 after RETN TaskFuse, got %d", bus.CurrentTask)
	}
	if cpu.PC != 0x0200 {
		t.Fatalf("expected PC = 0x0200 after RETN, got %04X", cpu.PC)
	}
	if cpu.SP != 0x2000 {
		t.Fatalf("expected SP popped to $2000, got %04X", cpu.SP)
	}

	// Run user code to halt
	for !cpu.Halted {
		cpu.Step()
	}
	if cpu.A != 88 {
		t.Fatalf("expected A = 88 in user task, got %d", cpu.A)
	}
}

func TestZ80MemoryProtection(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// User task 1 code attempting to read $FF00 directly
	userProg := []byte{
		0x3A, 0x00, 0xFF, // LD A, ($FF00) - Protected access!
	}
	copy(bus.Memory[1][0x0100:], userProg)

	bus.CurrentTask = 1
	cpu.PC = 0x0100

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic on user access to protected $FFxx page")
		}
		err, ok := r.(error)
		if !ok || !errors.Is(err, ErrUserAccessTrap) {
			t.Fatalf("expected ErrUserAccessTrap, got: %v", r)
		}
	}()

	cpu.Step()
}
