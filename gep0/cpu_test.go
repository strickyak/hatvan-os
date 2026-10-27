package gep0

import (
	"bytes"
	"testing"
)

func TestSmallModelStackFrameAndCall(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	cpu.Reset()

	// Place a function add(a, b word) word at 1000:0300:
	//   55             PUSH BP
	//   89 E5          MOV BP, SP
	//   8B 46 04       MOV AX, [BP+4]
	//   03 46 06       ADD AX, [BP+6]
	//   89 EC          MOV SP, BP
	//   5D             POP BP
	//   C3             RET
	addFn := []byte{
		0x55,
		0x89, 0xE5,
		0x8B, 0x46, 0x04,
		0x03, 0x46, 0x06,
		0x89, 0xEC,
		0x5D,
		0xC3,
	}
	for i, b := range addFn {
		bus.WriteByteSeg(0x1000, 0x0300+uint16(i), b)
	}

	// Main at 1000:0200:
	//   B8 22 00       MOV AX, 0x0022
	//   50             PUSH AX
	//   B8 14 00       MOV AX, 0x0014
	//   50             PUSH AX
	//   E8 F3 00       CALL 0x0300 (from 0x020B -> +0x00F5 = 0x0300)
	//   83 C4 04       ADD SP, 4
	//   A2 05 FF       MOV [0xFF05], AL  ; Exit with AL (0x36 = 54)
	mainCode := []byte{
		0xB8, 0x22, 0x00,
		0x50,
		0xB8, 0x14, 0x00,
		0x50,
		0xE8, 0xF5, 0x00,
		0x83, 0xC4, 0x04,
		0xA2, 0x05, 0xFF,
	}
	for i, b := range mainCode {
		bus.WriteByteSeg(0x1000, 0x0200+uint16(i), b)
	}

	for !bus.Halted && cpu.Cycles < 1000 {
		cpu.Step()
	}

	if !bus.Halted {
		t.Fatalf("expected VM to halt via PORT_EXIT")
	}
	if bus.ExitCode != 0x36 {
		t.Fatalf("expected ExitCode 0x36 (54), got 0x%02X (AX=0x%04X)", bus.ExitCode, cpu.AX)
	}
	if cpu.SP != 0xE000 {
		t.Fatalf("expected SP restored to 0xE000, got 0x%04X", cpu.SP)
	}
}

func TestMulDivAndStringOps(t *testing.T) {
	bus := NewBus()
	var out bytes.Buffer
	bus.ConsoleOut = &out
	cpu := NewCPU(bus)
	cpu.Reset()

	// Write "Hello, 8086!" into DS:0400
	msg := []byte("Hello, 8086!")
	for i, b := range msg {
		bus.WriteByteSeg(0x0000, 0x0400+uint16(i), b)
	}

	// Code at 1000:0200:
	//   FC             CLD
	//   BE 00 04       MOV SI, 0x0400
	//   BF 00 05       MOV DI, 0x0500
	//   B9 0C 00       MOV CX, 12
	//   F3 A4          REP MOVSB          ; copy DS:0400..040B -> ES:0500..050B
	//   B8 2C 01       MOV AX, 300
	//   BB 0A 00       MOV BX, 10
	//   F7 E3          MUL BX             ; DX:AX = 3000
	//   BB 1E 00       MOV BX, 30
	//   F7 F3          DIV BX             ; AX = 100, DX = 0
	//   A2 05 FF       MOV [0xFF05], AL   ; Exit(100)
	code := []byte{
		0xFC,
		0xBE, 0x00, 0x04,
		0xBF, 0x00, 0x05,
		0xB9, 0x0C, 0x00,
		0xF3, 0xA4,
		0xB8, 0x2C, 0x01,
		0xBB, 0x0A, 0x00,
		0xF7, 0xE3,
		0xBB, 0x1E, 0x00,
		0xF7, 0xF3,
		0xA2, 0x05, 0xFF,
	}
	for i, b := range code {
		bus.WriteByteSeg(0x1000, 0x0200+uint16(i), b)
	}

	for !bus.Halted && cpu.Cycles < 2000 {
		cpu.Step()
	}

	if bus.ExitCode != 100 {
		t.Fatalf("expected ExitCode 100, got %d", bus.ExitCode)
	}
	for i, want := range msg {
		got := bus.ReadByteSeg(0x0000, 0x0500+uint16(i))
		if got != want {
			t.Fatalf("byte %d at 0000:0500 mismatch: got %q, want %q", i, got, want)
		}
	}
}

func TestMultiSegmentDECBAndTrapTaskFuse(t *testing.T) {
	bus := NewBus()
	var out bytes.Buffer
	bus.ConsoleOut = &out
	cpu := NewCPU(bus)
	cpu.Reset()

	// Construct a multi-segment Extended DECB binary for Task 0:
	// 1. Magic: FD 00 00 'x' '0'
	// 2. High16 = $0000 (Data segment): install IVT Vector 30h at $00C0 -> IP=$0300, CS=$1000
	// 3. High16 = $0001 (Code segment):
	//    - Entry at $0200: arm TaskFuse for Task 3 (SP=$0FE0, CS=$1000, IP=$0200) and IRET
	//    - Trap 30h handler at $0300: read syscall num from [$FF60], write it to [$FF05] (ExitCode)
	var decb bytes.Buffer
	decb.Write([]byte{0xFD, 0x00, 0x00, 'x', '0'})

	// Data Segment (High16 = 0): Vector 30h at 0x00C0 -> [0x00, 0x03, 0x00, 0x10]
	decb.Write([]byte{0xFE, 0x00, 0x00, 0x00, 0x00})
	decb.Write([]byte{0x00, 0x00, 0x04, 0x00, 0xC0, 0x00, 0x03, 0x00, 0x10})

	// Code Segment (High16 = 1):
	decb.Write([]byte{0xFE, 0x00, 0x00, 0x00, 0x01})
	// At 1000:0200:
	//   BC E0 0F       MOV SP, 0x0FE0
	//   B0 03          MOV AL, 3
	//   A2 20 FF       MOV [0xFF20], AL  ; Arm TaskFuse for Task 3
	//   CF             IRET
	code200 := []byte{
		0xBC, 0xE0, 0x0F,
		0xB0, 0x03,
		0xA2, 0x20, 0xFF,
		0xCF,
	}
	decb.Write([]byte{0x00, 0x00, byte(len(code200)), 0x02, 0x00})
	decb.Write(code200)

	// At 1000:0300 (Trap 30h handler in Task 0):
	//   A0 60 FF       MOV AL, [0xFF60]  ; TrapSyscallNum
	//   A2 05 FF       MOV [0xFF05], AL  ; Exit with syscall number
	code300 := []byte{
		0xA0, 0x60, 0xFF,
		0xA2, 0x05, 0xFF,
	}
	decb.Write([]byte{0x00, 0x00, byte(len(code300)), 0x03, 0x00})
	decb.Write(code300)

	// Postamble: Entry IP = 0x0200
	decb.Write([]byte{0xFF, 0x00, 0x00, 0x02, 0x00})

	lb, err := bus.LoadDECB(&decb)
	if err != nil {
		t.Fatalf("LoadDECB failed: %v", err)
	}
	if lb.EntryCS != 0x1000 || lb.EntryIP != 0x0200 {
		t.Fatalf("unexpected entry %04X:%04X, want 1000:0200", lb.EntryCS, lb.EntryIP)
	}
	cpu.CS = lb.EntryCS
	cpu.IP = lb.EntryIP

	// Prepare Task 3 stack at 0000:0FE0 with [IP=0x0200, CS=0x1000, FLAGS=0xF202]
	// and Task 3 code at 1000:0200: `INT 30h; DB 0x8A`
	bus.WriteTaskByte20(3, 0x00FE0, 0x00)
	bus.WriteTaskByte20(3, 0x00FE1, 0x02) // IP = 0x0200
	bus.WriteTaskByte20(3, 0x00FE2, 0x00)
	bus.WriteTaskByte20(3, 0x00FE3, 0x10) // CS = 0x1000
	bus.WriteTaskByte20(3, 0x00FE4, 0x02)
	bus.WriteTaskByte20(3, 0x00FE5, 0xF2) // FLAGS
	bus.WriteTaskByte20(3, 0x10200, 0xCD) // INT 30h
	bus.WriteTaskByte20(3, 0x10201, 0x30)
	bus.WriteTaskByte20(3, 0x10202, 0x8A) // DB 0x8A (I$Write)

	for !bus.Halted && cpu.Cycles < 1000 {
		cpu.Step()
	}

	if !bus.Halted {
		t.Fatalf("expected VM to halt")
	}
	if bus.ExitCode != 0x8A {
		t.Fatalf("expected ExitCode 0x8A from INT 30h inline byte, got 0x%02X", bus.ExitCode)
	}
	if bus.TrapSourceTask != 3 {
		t.Fatalf("expected TrapSourceTask 3, got %d", bus.TrapSourceTask)
	}
}

func TestCrossPageDMAAndSharedCurtain(t *testing.T) {
	bus := NewBus()

	// 1. Verify Shared Curtain ($0E000..$0FEFF) is shared across Tasks 0, 1, 2
	// while $1E000..$1FFFF (Code Segment) is private to each task!
	bus.CurrentTask = 0
	bus.WriteByteSeg(0x0000, 0xE100, 0xAA)
	bus.WriteByteSeg(0x1000, 0xE100, 0x11)

	bus.CurrentTask = 1
	if got := bus.ReadByteSeg(0x0000, 0xE100); got != 0xAA {
		t.Fatalf("expected Task 1 DS:E100 to read shared 0xAA, got 0x%02X", got)
	}
	if got := bus.ReadByteSeg(0x1000, 0xE100); got != 0x00 {
		t.Fatalf("expected Task 1 CS:E100 ($1E100) to be private (0x00), got 0x%02X", got)
	}

	// 2. Test Cross-Page DMA from Task 0 Data Segment ($00600) to Task 4 Code Segment ($10200, page=1)
	bus.CurrentTask = 0
	payload := []byte{0xB0, 0x2A, 0xA2, 0x05, 0xFF}
	for i, b := range payload {
		bus.WriteByteSeg(0x0000, 0x0600+uint16(i), b)
	}

	// Program DMA: Src = Task 0, Page 0, Addr $0600; Dst = Task 4, Page 1, Addr $0200; Count = 5
	bus.WriteByteSeg(0x0000, 0xFF24, 0)
	bus.WriteByteSeg(0x0000, 0xFF25, 0)
	bus.WriteByteSeg(0x0000, 0xFF26, 0x06)
	bus.WriteByteSeg(0x0000, 0xFF27, 0x00)

	bus.WriteByteSeg(0x0000, 0xFF28, 4)
	bus.WriteByteSeg(0x0000, 0xFF29, 1) // Page 1 ($1xxxx Code Segment)
	bus.WriteByteSeg(0x0000, 0xFF2A, 0x02)
	bus.WriteByteSeg(0x0000, 0xFF2B, 0x00)

	bus.WriteByteSeg(0x0000, 0xFF2C, 0)
	bus.WriteByteSeg(0x0000, 0xFF2D, 0)
	bus.WriteByteSeg(0x0000, 0xFF2E, 0)
	bus.WriteByteSeg(0x0000, 0xFF2F, byte(len(payload)))
	bus.WriteByteSeg(0x0000, 0xFF21, 1) // Execute DMA

	for i, want := range payload {
		got := bus.ReadTaskByte20(4, 0x10200+uint32(i))
		if got != want {
			t.Fatalf("DMA byte %d at Task 4 $10200 mismatch: got 0x%02X, want 0x%02X", i, got, want)
		}
	}
}

func TestUserRedpageProtection(t *testing.T) {
	bus := NewBus()
	bus.CurrentTask = 3

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected user task write to $0FF00 to panic with ErrUserAccessTrap")
		}
	}()

	bus.WriteByteSeg(0x0000, 0xFF00, 'X')
}

