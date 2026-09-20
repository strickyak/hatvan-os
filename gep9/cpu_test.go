package gep9

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestDECBStandardAndExtended(t *testing.T) {
	buf := new(bytes.Buffer)

	// 1. Data chunk ($00) at 0x1000, 4 bytes: 0x8E, 0x12, 0x34, 0x39 (LDX #$1234; RTS)
	buf.WriteByte(ChunkData)
	binary.Write(buf, binary.BigEndian, uint16(4))
	binary.Write(buf, binary.BigEndian, uint16(0x1000))
	buf.Write([]byte{0x8E, 0x12, 0x34, 0x39})

	// 2. Absolute source line ($01) at 0x1000
	srcLineText := "start ldx #$1234"
	buf.WriteByte(ChunkSrcAbs)
	binary.Write(buf, binary.BigEndian, uint16(2+len(srcLineText)))
	binary.Write(buf, binary.BigEndian, uint16(0x1000))
	binary.Write(buf, binary.BigEndian, uint16(10)) // line 10
	buf.WriteString(srcLineText)

	// 3. OS-9 Module ($02)
	modName := "testmod"
	buf.WriteByte(ChunkModDef)
	binary.Write(buf, binary.BigEndian, uint16(5+len(modName)))
	binary.Write(buf, binary.BigEndian, uint16(0x0200)) // size 512
	buf.Write([]byte{0x12, 0x34, 0x56})                 // CRC
	binary.Write(buf, binary.BigEndian, uint16(0x0020)) // exec offset
	buf.WriteString(modName)

	// 4. Relative source line ($03) at offset 0x0020
	relText := "main lda #0"
	buf.WriteByte(ChunkSrcRel)
	binary.Write(buf, binary.BigEndian, uint16(2+len(relText)))
	binary.Write(buf, binary.BigEndian, uint16(0x0020))
	binary.Write(buf, binary.BigEndian, uint16(42))
	buf.WriteString(relText)

	// 5. Exec chunk ($FF) at 0x1000
	buf.WriteByte(ChunkExec)
	binary.Write(buf, binary.BigEndian, uint16(0))
	binary.Write(buf, binary.BigEndian, uint16(0x1000))

	decb, err := LoadDECB(buf)
	if err != nil {
		t.Fatalf("LoadDECB failed: %v", err)
	}

	if !decb.HasExec || decb.ExecAddr != 0x1000 {
		t.Fatalf("expected exec addr 0x1000, got %04X", decb.ExecAddr)
	}
	if len(decb.Segments) != 1 || decb.Segments[0].Addr != 0x1000 {
		t.Fatalf("unexpected segments: %+v", decb.Segments)
	}
	if line, ok := decb.AbsLines[0x1000]; !ok || line.LineNum != 10 || line.Text != srcLineText {
		t.Fatalf("unexpected abs line: %+v", line)
	}
	if len(decb.Modules) != 1 || decb.Modules[0].Name != modName {
		t.Fatalf("unexpected module: %+v", decb.Modules)
	}
	if relLine, ok := decb.Modules[0].Lines[0x0020]; !ok || relLine.LineNum != 42 || relLine.Text != relText {
		t.Fatalf("unexpected rel line: %+v", relLine)
	}
}

func TestCPUExecutionBasic(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// Load code into Task 0:
	// 0x1000: LDA #40
	// 0x1002: ADDA #2
	// 0x1004: STA $20 (direct page $0020)
	// 0x1006: NOP
	code := []byte{
		0x86, 40, // LDA #40
		0x8B, 2, // ADDA #2
		0x97, 0x20, // STA $20
		0x12, // NOP
	}
	for i, b := range code {
		bus.WriteByte(uint16(0x1000+i), b)
	}

	cpu.PC = 0x1000
	cpu.DP = 0x00

	for i := 0; i < 4; i++ {
		cpu.Step()
	}

	if cpu.A != 42 {
		t.Fatalf("expected A=42, got %d", cpu.A)
	}
	if bus.ReadByte(0x0020) != 42 {
		t.Fatalf("expected memory at 0x0020 to be 42, got %d", bus.ReadByte(0x0020))
	}
}

func TestRTIStackFrames6809And6309(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// 1. Test 6809 mode full stack frame
	cpu.MD = 0 // 6809 Emulation mode
	cpu.S = 0x2000
	cpu.A = 0x11
	cpu.B = 0x22
	cpu.DP = 0x05
	cpu.X = 0x1234
	cpu.Y = 0x5678
	cpu.U = 0x9ABC
	cpu.PC = 0x3000
	cpu.CC = FlagN | FlagZ

	cpu.PushInterruptFrame(true)
	if cpu.S != 0x2000-12 {
		t.Fatalf("expected S to decrement by 12 bytes in 6809 mode, got 0x%04X", cpu.S)
	}

	// Corrupt registers to prove RTI restores them
	cpu.A, cpu.B, cpu.DP, cpu.X, cpu.Y, cpu.U, cpu.PC = 0, 0, 0, 0, 0, 0, 0
	cpu.ExecuteRTI()

	if cpu.A != 0x11 || cpu.B != 0x22 || cpu.DP != 0x05 || cpu.X != 0x1234 || cpu.Y != 0x5678 || cpu.U != 0x9ABC || cpu.PC != 0x3000 {
		t.Fatalf("6809 RTI restored incorrect registers: A=%X B=%X DP=%X X=%X Y=%X U=%X PC=%X",
			cpu.A, cpu.B, cpu.DP, cpu.X, cpu.Y, cpu.U, cpu.PC)
	}
	if cpu.S != 0x2000 {
		t.Fatalf("expected S to return to 0x2000, got 0x%04X", cpu.S)
	}

	// 2. Test 6309 Native Mode full stack frame (14 bytes with E and F)
	cpu.MD = 1 // 6309 Native Mode
	cpu.S = 0x2000
	cpu.A = 0x11
	cpu.B = 0x22
	cpu.E = 0x33
	cpu.F = 0x44
	cpu.DP = 0x05
	cpu.X = 0x1234
	cpu.Y = 0x5678
	cpu.U = 0x9ABC
	cpu.PC = 0x4000
	cpu.CC = FlagN | FlagZ

	cpu.PushInterruptFrame(true)
	if cpu.S != 0x2000-14 {
		t.Fatalf("expected S to decrement by 14 bytes in 6309 native mode, got 0x%04X", cpu.S)
	}

	// Corrupt registers
	cpu.A, cpu.B, cpu.E, cpu.F, cpu.DP, cpu.X, cpu.Y, cpu.U, cpu.PC = 0, 0, 0, 0, 0, 0, 0, 0, 0
	cpu.ExecuteRTI()

	if cpu.A != 0x11 || cpu.B != 0x22 || cpu.E != 0x33 || cpu.F != 0x44 || cpu.DP != 0x05 ||
		cpu.X != 0x1234 || cpu.Y != 0x5678 || cpu.U != 0x9ABC || cpu.PC != 0x4000 {
		t.Fatalf("6309 RTI restored incorrect registers: A=%X B=%X E=%X F=%X DP=%X X=%X Y=%X U=%X PC=%X",
			cpu.A, cpu.B, cpu.E, cpu.F, cpu.DP, cpu.X, cpu.Y, cpu.U, cpu.PC)
	}
	if cpu.S != 0x2000 {
		t.Fatalf("expected S to return to 0x2000, got 0x%04X", cpu.S)
	}
}

func TestTurbo9SimTimerIRQ(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// Set IRQ vector in Task 0 at $FFF8 to 0x5000
	bus.WriteWord(0xFFF8, 0x5000)

	// At 0x5000:
	// SvcIRQ:
	//   LDA $FF02 (read stat)
	//   STA $FF02 (write back to clear timer interrupt)
	//   RTI
	irqHandler := []byte{
		0xB6, 0xFF, 0x02, // LDA $FF02
		0xB7, 0xFF, 0x02, // STA $FF02
		0x3B, // RTI
	}
	for i, b := range irqHandler {
		bus.WriteByte(uint16(0x5000+i), b)
	}

	// Main code at 0x1000:
	//   LDA #$01
	//   STA $FF03 (enable timer IRQ)
	//   ANDCC #^FlagI (enable interrupts)
	// loop:
	//   BRA loop
	mainCode := []byte{
		0x86, 0x01, // LDA #$01
		0xB7, 0xFF, 0x03, // STA $FF03
		0x1C, ^byte(FlagI), // ANDCC #^FlagI
		0x20, 0xFE, // BRA loop (-2)
	}
	for i, b := range mainCode {
		bus.WriteByte(uint16(0x1000+i), b)
	}

	cpu.PC = 0x1000
	cpu.S = 0x0F00
	cpu.CC = FlagI // Interrupts masked initially

	// Step through initialization
	cpu.Step() // LDA
	cpu.Step() // STA $FF03
	cpu.Step() // ANDCC

	// Now trigger a timer tick
	bus.TimerTick()

	// Next step should take the IRQ vector
	cpu.Step()
	if cpu.PC != 0x5000 {
		t.Fatalf("expected PC to be 0x5000 (IRQ handler), got 0x%04X", cpu.PC)
	}

	// Execute handler
	cpu.Step() // LDA $FF02
	cpu.Step() // STA $FF02 (clears interrupt)
	cpu.Step() // RTI

	// Should have returned to 0x1007 (loop)
	if cpu.PC != 0x1007 {
		t.Fatalf("expected PC to return to loop at 0x1007, got 0x%04X", cpu.PC)
	}
	// Timer interrupt should be cleared
	if (bus.RegStat & 0x01) != 0 {
		t.Fatalf("expected Timer.Ready to be cleared, got 0x%02X", bus.RegStat)
	}
}

func TestHatvanDMACopyAndTaskFuse(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// Set data in Task 1 at 0x0200
	bus.Memory[1][0x0200] = 0xDE
	bus.Memory[1][0x0201] = 0xAD
	bus.Memory[1][0x0202] = 0xBE
	bus.Memory[1][0x0203] = 0xEF

	// Program in Task 0: copy 4 bytes from Task 1 (0x0200) to Task 0 (0x0300)
	bus.CurrentTask = 0
	bus.WriteByte(0xFF21, 1)      // src task
	bus.WriteWord(0xFF22, 0x0200) // src addr
	bus.WriteByte(0xFF24, 0)      // dst task
	bus.WriteWord(0xFF25, 0x0300) // dst addr
	bus.WriteByte(0xFF27, 4)      // copy 4 bytes

	if bus.DmaStatus != 1 {
		t.Fatalf("expected DMA status 1, got %d", bus.DmaStatus)
	}
	if bus.Memory[0][0x0300] != 0xDE || bus.Memory[0][0x0301] != 0xAD ||
		bus.Memory[0][0x0302] != 0xBE || bus.Memory[0][0x0303] != 0xEF {
		t.Fatalf("DMA copy failed, got: %X", bus.Memory[0][0x0300:0x0304])
	}

	// Test TaskFuse inside RTI
	// Push fake frame in Task 1 at S=0x1000
	bus.CurrentTask = 1
	cpu.S = 0x1000
	cpu.PC = 0x4567
	cpu.PushInterruptFrame(false) // short frame (PC, CC)

	// Switch back to Task 0
	bus.CurrentTask = 0
	// Arm TaskFuse to Task 1
	bus.WriteByte(0xFF20, 1)

	// Execute RTI
	cpu.ExecuteRTI()

	if bus.CurrentTask != 1 {
		t.Fatalf("expected CurrentTask to switch to 1 via TaskFuse, got %d", bus.CurrentTask)
	}
	if cpu.PC != 0x4567 {
		t.Fatalf("expected PC restored to 0x4567, got 0x%04X", cpu.PC)
	}
}

func TestUserPageProtection(t *testing.T) {
	bus := NewBus()
	bus.CurrentTask = 1 // User task

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected user access to $FF00 to panic with ErrUserAccessTrap")
		}
	}()

	bus.ReadByte(0xFF00)
}

func TestTaskFlagsIOBlessing(t *testing.T) {
	bus := NewBus()

	// 1. Task 1 unblessed: accessing $FF00 panics
	bus.CurrentTask = 1
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected unblessed Task 1 access to $FF00 to panic")
			}
		}()
		bus.ReadByte(0xFF00)
	}()

	// 2. Task 0 blesses Task 1 via $FF2E = 0x01
	bus.CurrentTask = 0
	bus.WriteByte(0xFF2E, 0x01)
	if got := bus.ReadByte(0xFF2E); got != 0x01 {
		t.Fatalf("ReadByte(0xFF2E) = 0x%02X, want 0x01", got)
	}

	// 3. Task 1 now has I/O privileges: reading and writing $FF00..$FFFF succeeds without panic
	bus.CurrentTask = 1
	bus.WriteByte(0xFF10, 2) // Set disk drive to 2
	if bus.DiskDrive != 2 {
		t.Fatalf("expected DiskDrive=2 from Task 1, got %d", bus.DiskDrive)
	}

	// 4. Task 2 remains unblessed: accessing $FF00 still panics
	bus.CurrentTask = 2
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected unblessed Task 2 access to $FF00 to panic")
			}
		}()
		bus.ReadByte(0xFF00)
	}()

	// 5. Purging Task 1 via $FF2F revokes blessing
	bus.CurrentTask = 0
	bus.WriteByte(0xFF2F, 1) // Purge Task 1
	if got := bus.ReadByte(0xFF2E); got != 0x00 {
		t.Fatalf("ReadByte(0xFF2E) after purge = 0x%02X, want 0x00", got)
	}

	// Task 1 access now panics again
	bus.CurrentTask = 1
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected Task 1 access after purge to panic")
			}
		}()
		bus.ReadByte(0xFF00)
	}()
}

func TestBusPollStdin(t *testing.T) {
	bus := NewBus()
	if !bus.ConsoleInEmpty() {
		t.Fatalf("expected initial ConsoleIn to be empty")
	}

	bus.EnqueueString("hi\n")
	if bus.ConsoleInEmpty() {
		t.Fatalf("expected ConsoleIn not empty after EnqueueString")
	}

	// Drain
	c1 := bus.ReadByte(0xFF01)
	c2 := bus.ReadByte(0xFF01)
	c3 := bus.ReadByte(0xFF01)
	if c1 != 'h' || c2 != 'i' || c3 != '\r' {
		t.Fatalf("expected 'h', 'i', '\\r', got %q, %q, %q", c1, c2, c3)
	}
	if !bus.ConsoleInEmpty() {
		t.Fatalf("expected ConsoleIn empty after drain")
	}

	// Attach StdinChan
	stdinCh := make(chan byte, 10)
	bus.StdinChan = stdinCh
	stdinCh <- 'o'
	stdinCh <- 'k'
	stdinCh <- '\n'

	bus.PollStdin()
	if bus.ConsoleInEmpty() {
		t.Fatalf("expected ConsoleIn populated from StdinChan")
	}

	r1 := bus.ReadByte(0xFF01)
	r2 := bus.ReadByte(0xFF01)
	r3 := bus.ReadByte(0xFF01)
	if r1 != 'o' || r2 != 'k' || r3 != '\r' {
		t.Fatalf("expected 'o', 'k', '\\r', got %q, %q, %q", r1, r2, r3)
	}
}

func TestExitPortFF05(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// LDA #42; STA $FF05 (extended: $B7 $FF $05)
	code := []byte{0x86, 42, 0xB7, 0xFF, 0x05, 0x12}
	for i, b := range code {
		bus.WriteByte(uint16(0x1000+i), b)
	}
	cpu.PC = 0x1000

	for !cpu.Halted {
		c := cpu.Step()
		if c == 0 {
			break
		}
	}

	if !cpu.Halted {
		t.Fatalf("expected CPU to be halted after writing $FF05")
	}
	if cpu.ExitCode != 42 {
		t.Fatalf("expected ExitCode 42, got %d", cpu.ExitCode)
	}
	if bus.ReadByte(0xFF05) != 42 {
		t.Fatalf("expected read $FF05 to return 42, got %d", bus.ReadByte(0xFF05))
	}
}

func TestHyperCalls(t *testing.T) {
	bus := NewBus()
	outBuf := new(bytes.Buffer)
	bus.ConsoleOut = outBuf
	cpu := NewCPU(bus)
	cpu.EnableHypercalls = true

	// 1. PutChar (hop 132): LDB #'A' ($C6 $41); FCB $12,$21,132
	code := []byte{0xC6, 'A', 0x12, 0x21, 132}
	for i, b := range code {
		bus.WriteByte(uint16(0x1000+i), b)
	}
	cpu.PC = 0x1000
	cpu.Step() // LDB
	cpu.Step() // Hyper PutChar
	if outBuf.String() != "A" {
		t.Fatalf("expected 'A', got %q", outBuf.String())
	}
	outBuf.Reset()

	// 2. Printf (hop 111):
	// Format string at $2000: "num=%d str=%s\n\0"
	fmtStr := "num=%d str=%s\n\x00"
	for i := 0; i < len(fmtStr); i++ {
		bus.WriteByte(uint16(0x2000+i), fmtStr[i])
	}
	// Target string at $2020: "minigolf\0"
	targetStr := "minigolf\x00"
	for i := 0; i < len(targetStr); i++ {
		bus.WriteByte(uint16(0x2020+i), targetStr[i])
	}
	// Stack frame at $1100:
	bus.WriteWord(0x1100, 0x2000) // format string ptr
	bus.WriteWord(0x1102, 999)    // %d
	bus.WriteWord(0x1104, 0x2020) // %s
	// Code: LDX #$1100 ($8E $11 $00); FCB $12,$21,111
	pCode := []byte{0x8E, 0x11, 0x00, 0x12, 0x21, 111}
	for i, b := range pCode {
		bus.WriteByte(uint16(0x1010+i), b)
	}
	cpu.PC = 0x1010
	cpu.Step() // LDX
	cpu.Step() // Hyper Printf
	if outBuf.String() != "num=999 str=minigolf\n" {
		t.Fatalf("expected 'num=999 str=minigolf\\n', got %q", outBuf.String())
	}

	// 3. Exit (hop 107): LDD #15 ($CC $00 $0F); FCB $12,$21,107
	eCode := []byte{0xCC, 0x00, 0x0F, 0x12, 0x21, 107}
	for i, b := range eCode {
		bus.WriteByte(uint16(0x1020+i), b)
	}
	cpu.PC = 0x1020
	cpu.Step() // LDD
	cpu.Step() // Hyper Exit
	if !cpu.Halted {
		t.Fatalf("expected CPU to be halted after Hyper Exit")
	}
	if cpu.ExitCode != 15 {
		t.Fatalf("expected ExitCode 15, got %d", cpu.ExitCode)
	}
}

func TestLEAS(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)
	// 10D3: 32 66 ED 60 (LEAS 6,S; STD 0,S)
	code := []byte{0x32, 0x66, 0xED, 0x60, 0xE6, 0x60}
	for i, b := range code {
		bus.WriteByte(uint16(0x10D3+i), b)
	}
	cpu.PC = 0x10D3
	cpu.S = 0x0EC6
	cpu.SetD(0x0100)

	cpu.Step()
	if cpu.PC != 0x10D5 || cpu.S != 0x0ECC {
		t.Fatalf("expected PC=10D5 S=0ECC, got PC=%04X S=%04X", cpu.PC, cpu.S)
	}
	cpu.Step()
	if cpu.PC != 0x10D7 || cpu.S != 0x0ECC {
		t.Fatalf("expected PC=10D7 S=0ECC, got PC=%04X S=%04X", cpu.PC, cpu.S)
	}
}

func TestCurlyEscapeAndNewline(t *testing.T) {
	bus := NewBus()
	outBuf := new(bytes.Buffer)
	bus.ConsoleOut = outBuf

	// Without CurlyEscape:
	// Newline translation (13 -> \n, 10 -> \n)
	bus.WriteByte(0xFF00, 13)
	bus.WriteByte(0xFF00, 10)
	bus.WriteByte(0xFF00, 'A')
	bus.WriteByte(0xFF00, 7)
	if got := outBuf.String(); got != "\n\nA\x07" {
		t.Fatalf("expected \\n\\nA\\x07, got %q", got)
	}

	// With CurlyEscape:
	outBuf.Reset()
	bus.CurlyEscape = true
	bus.WriteByte(0xFF00, 13)
	bus.WriteByte(0xFF00, 10)
	bus.WriteByte(0xFF00, ' ')
	bus.WriteByte(0xFF00, 'Z')
	bus.WriteByte(0xFF00, '~')
	bus.WriteByte(0xFF00, 0)
	bus.WriteByte(0xFF00, 7)
	bus.WriteByte(0xFF00, 8)
	bus.WriteByte(0xFF00, 27)
	bus.WriteByte(0xFF00, 127)
	bus.WriteByte(0xFF00, 255)

	expected := "\n\n Z~{0}{7}{8}{27}{127}{255}"
	if got := outBuf.String(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

