package gep9

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
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

	// 4b. Task 0 blesses Task 2 via $FF2D = 2, $FF2E = 0x01
	bus.CurrentTask = 0
	bus.WriteByte(0xFF2D, 2)
	bus.WriteByte(0xFF2E, 0x01)
	bus.CurrentTask = 2
	bus.WriteByte(0xFF10, 1) // Access succeeds
	if bus.DiskDrive != 1 {
		t.Fatalf("expected DiskDrive=1 from blessed Task 2, got %d", bus.DiskDrive)
	}

	// 5. Purging Task 1 via $FF2F revokes blessing
	bus.CurrentTask = 0
	bus.WriteByte(0xFF2D, 1)
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

func TestFlat65280v2MemoryMap(t *testing.T) {
	bus := NewBus()
	bus.Engine = EngineFlat65280v2

	// 1. RAM accesses in $0000..$FEFF
	bus.WriteByte(0x0000, 0x12)
	bus.WriteByte(0x1234, 0x56)
	bus.WriteByte(0xFEFF, 0x78)
	if bus.ReadByte(0x0000) != 0x12 || bus.ReadByte(0x1234) != 0x56 || bus.ReadByte(0xFEFF) != 0x78 {
		t.Fatalf("RAM read/write mismatch in flat RAM space")
	}

	// 2. Vectors in $FFF0..$FFFF are readable
	bus.Memory[0][0xFFFE] = 0x10
	bus.Memory[0][0xFFFF] = 0x20
	if bus.ReadByte(0xFFFE) != 0x10 || bus.ReadByte(0xFFFF) != 0x20 {
		t.Fatalf("vector read mismatch")
	}
	if bus.ReadWord(0xFFFE) != 0x1020 {
		t.Fatalf("vector word read mismatch")
	}

	// 3. Stray reads outside valid I/O registers ($FF80..$FF89) must panic with ErrFlatStrayRead
	strayReadAddrs := []uint16{0xFF00, 0xFF01, 0xFF10, 0xFF20, 0xFF7F, 0xFF8A, 0xFFEE, 0xFFEF}
	for _, addr := range strayReadAddrs {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected stray read at 0x%04X to panic", addr)
				}
				err, ok := r.(error)
				if !ok || !errors.Is(err, ErrFlatStrayRead) {
					t.Fatalf("expected ErrFlatStrayRead at 0x%04X, got: %v", addr, r)
				}
			}()
			bus.ReadByte(addr)
		}()
	}

	// 4. Stray writes outside write registers ($FF80..$FF89) must panic with ErrFlatStrayWrite
	strayWriteAddrs := []uint16{0xFF00, 0xFF01, 0xFF05, 0xFF10, 0xFF20, 0xFF7F, 0xFF8A, 0xFFEE, 0xFFEF, 0xFFF0, 0xFFFE, 0xFFFF}
	for _, addr := range strayWriteAddrs {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected stray write at 0x%04X to panic", addr)
				}
				err, ok := r.(error)
				if !ok || !errors.Is(err, ErrFlatStrayWrite) {
					t.Fatalf("expected ErrFlatStrayWrite at 0x%04X, got: %v", addr, r)
				}
			}()
			bus.WriteByte(addr, 0x42)
		}()
	}
}

func TestFlat65280v2EMUDSK(t *testing.T) {
	bus := NewBus()
	bus.Engine = EngineFlat65280v2

	disk0 := make([]byte, 1024) // 4 sectors
	disk1 := make([]byte, 512)  // 2 sectors
	bus.Disks[0] = disk0
	bus.Disks[1] = disk1

	// 1. Register read/write
	bus.WriteByte(0xFF80, 0x01)
	bus.WriteByte(0xFF81, 0x02)
	bus.WriteByte(0xFF82, 0x03)
	if bus.ReadByte(0xFF80) != 0x01 || bus.ReadByte(0xFF81) != 0x02 || bus.ReadByte(0xFF82) != 0x03 {
		t.Fatalf("EMUDSK LSN register mismatch")
	}
	if bus.EmuDskLSN != 0x010203 {
		t.Fatalf("expected LSN 0x010203, got 0x%06X", bus.EmuDskLSN)
	}

	bus.WriteByte(0xFF84, 0x20)
	bus.WriteByte(0xFF85, 0x00)
	if bus.ReadByte(0xFF84) != 0x20 || bus.ReadByte(0xFF85) != 0x00 || bus.EmuDskBuffer != 0x2000 {
		t.Fatalf("EMUDSK BUFFER register mismatch")
	}

	bus.WriteByte(0xFF86, 0x01)
	if bus.ReadByte(0xFF86) != 0x01 || bus.EmuDskDrive != 1 {
		t.Fatalf("EMUDSK DRIVE register mismatch")
	}

	// 2. Write Sector to Disk 0
	bus.WriteByte(0xFF86, 0x00) // Drive 0
	bus.WriteByte(0xFF80, 0x00)
	bus.WriteByte(0xFF81, 0x00)
	bus.WriteByte(0xFF82, 0x01) // LSN 1 (offset 256)
	bus.WriteByte(0xFF84, 0x20)
	bus.WriteByte(0xFF85, 0x00) // Buffer $2000

	for i := 0; i < 256; i++ {
		bus.Memory[0][0x2000+uint16(i)] = byte(i ^ 0xAA)
	}
	bus.WriteByte(0xFF83, 1) // Command 1 = Write Sector
	if st := bus.ReadByte(0xFF83); st != 0 {
		t.Fatalf("expected write status 0, got %d", st)
	}
	for i := 0; i < 256; i++ {
		if disk0[256+i] != byte(i^0xAA) {
			t.Fatalf("disk0 sector 1 byte %d mismatch", i)
		}
	}

	// 3. Read Sector from Disk 0 back to memory at $3000
	for i := 0; i < 256; i++ {
		bus.Memory[0][0x3000+uint16(i)] = 0
	}
	bus.WriteByte(0xFF84, 0x30)
	bus.WriteByte(0xFF85, 0x00) // Buffer $3000
	bus.WriteByte(0xFF83, 0)    // Command 0 = Read Sector
	if st := bus.ReadByte(0xFF83); st != 0 {
		t.Fatalf("expected read status 0, got %d", st)
	}
	for i := 0; i < 256; i++ {
		if bus.Memory[0][0x3000+uint16(i)] != byte(i^0xAA) {
			t.Fatalf("RAM $3000+%d mismatch after read", i)
		}
	}

	// 4. Read Sector from Disk 1
	for i := 0; i < 256; i++ {
		disk1[i] = byte(i + 10)
	}
	bus.WriteByte(0xFF86, 0x01) // Drive 1
	bus.WriteByte(0xFF80, 0x00)
	bus.WriteByte(0xFF81, 0x00)
	bus.WriteByte(0xFF82, 0x00) // LSN 0
	bus.WriteByte(0xFF84, 0x40)
	bus.WriteByte(0xFF85, 0x00) // Buffer $4000
	bus.WriteByte(0xFF83, 0)    // Read
	if st := bus.ReadByte(0xFF83); st != 0 {
		t.Fatalf("expected Drive 1 read status 0, got %d", st)
	}
	for i := 0; i < 256; i++ {
		if bus.Memory[0][0x4000+uint16(i)] != byte(i+10) {
			t.Fatalf("RAM $4000+%d mismatch from disk1", i)
		}
	}

	// 5. Error conditions
	// Drive not ready (drive 2)
	bus.WriteByte(0xFF86, 0x02)
	bus.WriteByte(0xFF83, 0)
	if st := bus.ReadByte(0xFF83); st != 2 {
		t.Fatalf("expected status 2 for unattached drive, got %d", st)
	}

	// Sector out of bounds (LSN 10 on Drive 0)
	bus.WriteByte(0xFF86, 0x00)
	bus.WriteByte(0xFF82, 10)
	bus.WriteByte(0xFF83, 0)
	if st := bus.ReadByte(0xFF83); st != 6 {
		t.Fatalf("expected status 6 for out-of-bounds LSN, got %d", st)
	}

	// Buffer crossing into I/O page ($FE80 + 256 = $FF80 >= $FF00)
	bus.WriteByte(0xFF82, 0)
	bus.WriteByte(0xFF84, 0xFE)
	bus.WriteByte(0xFF85, 0x80)
	bus.WriteByte(0xFF83, 0)
	if st := bus.ReadByte(0xFF83); st != 6 {
		t.Fatalf("expected status 6 for buffer in I/O page, got %d", st)
	}

	// Invalid command
	bus.WriteByte(0xFF83, 99)
	if st := bus.ReadByte(0xFF83); st != 254 {
		t.Fatalf("expected status 254 for invalid command, got %d", st)
	}

	// Close command
	bus.WriteByte(0xFF83, 2)
	if st := bus.ReadByte(0xFF83); st != 0 {
		t.Fatalf("expected status 0 for close, got %d", st)
	}
}

func TestFlat65280v2ACIA(t *testing.T) {
	bus := NewBus()
	bus.Engine = EngineFlat65280v2

	outBuf := new(bytes.Buffer)
	bus.ConsoleOut = outBuf

	var irqState bool
	bus.OnIRQChanged = func(asserted bool) {
		irqState = asserted
	}

	// 1. Initial status: TDRE (0x02) is set, RDRF (0x01) and IRQ (0x80) are clear
	if st := bus.ReadByte(0xFF88); st != 0x02 {
		t.Fatalf("expected initial ACIA status 0x02, got 0x%02X", st)
	}
	if irqState {
		t.Fatalf("expected IRQ false initially")
	}

	// 2. Reading data when empty returns 0
	if ch := bus.ReadByte(0xFF89); ch != 0 {
		t.Fatalf("expected 0 on empty ACIA read, got %02X", ch)
	}

	// 3. Writing data outputs to ConsoleOut
	bus.WriteByte(0xFF89, 'H')
	bus.WriteByte(0xFF89, 'i')
	bus.WriteByte(0xFF89, 13) // newline
	if outBuf.String() != "Hi\n" {
		t.Fatalf("expected 'Hi\\n', got %q", outBuf.String())
	}

	// 4. Receiving data without interrupt (RIE = 0)
	bus.EnqueueString("OK")
	if st := bus.ReadByte(0xFF88); st != 0x03 { // TDRE | RDRF
		t.Fatalf("expected status 0x03 after enqueue, got 0x%02X", st)
	}
	if irqState {
		t.Fatalf("expected IRQ false when RIE is 0")
	}

	if ch := bus.ReadByte(0xFF89); ch != 'O' {
		t.Fatalf("expected 'O', got %c", ch)
	}
	if st := bus.ReadByte(0xFF88); st != 0x03 {
		t.Fatalf("expected status 0x03 while 'K' pending, got 0x%02X", st)
	}
	if ch := bus.ReadByte(0xFF89); ch != 'K' {
		t.Fatalf("expected 'K', got %c", ch)
	}
	if st := bus.ReadByte(0xFF88); st != 0x02 {
		t.Fatalf("expected status 0x02 when empty, got 0x%02X", st)
	}

	// 5. Receiving data with interrupt (RIE = 1, bit 7 of Control register)
	bus.WriteByte(0xFF88, 0x80) // Enable RIE
	if irqState {
		t.Fatalf("expected IRQ false with empty input")
	}

	bus.EnqueueKey('X')
	if !irqState {
		t.Fatalf("expected IRQ true after enqueuing with RIE enabled")
	}
	if st := bus.ReadByte(0xFF88); st != 0x83 { // IRQ | TDRE | RDRF
		t.Fatalf("expected status 0x83, got 0x%02X", st)
	}

	// Read data -> should deassert IRQ
	if ch := bus.ReadByte(0xFF89); ch != 'X' {
		t.Fatalf("expected 'X', got %c", ch)
	}
	if irqState {
		t.Fatalf("expected IRQ false after reading last character")
	}
	if st := bus.ReadByte(0xFF88); st != 0x02 {
		t.Fatalf("expected status 0x02 after reading, got 0x%02X", st)
	}

	// 6. Master Reset ($03) clears RIE and deasserts IRQ
	bus.WriteByte(0xFF88, 0x80) // Enable RIE
	bus.EnqueueKey('Y')
	if !irqState {
		t.Fatalf("expected IRQ true")
	}
	bus.WriteByte(0xFF88, 0x03) // Master Reset
	if irqState {
		t.Fatalf("expected IRQ false after master reset")
	}
	if bus.AciaCtrl != 0 {
		t.Fatalf("expected AciaCtrl=0 after master reset, got 0x%02X", bus.AciaCtrl)
	}
}

func TestFlat65280v2CPUExecution(t *testing.T) {
	bus := NewBus()
	bus.Engine = EngineFlat65280v2

	outBuf := new(bytes.Buffer)
	bus.ConsoleOut = outBuf

	// Create a disk with character 'Z' at sector 0 byte 0
	disk := make([]byte, 512)
	disk[0] = 'Z'
	bus.Disks[0] = disk

	// Assembly program at $1000:
	//   CLR  >$FF86     ; Drive 0
	//   CLR  >$FF80     ; LSN hi = 0
	//   CLR  >$FF81     ; LSN mid = 0
	//   CLR  >$FF82     ; LSN lo = 0
	//   LDX  #$2000
	//   STX  >$FF84     ; Buffer = $2000
	//   CLR  >$FF83     ; Command 0 = Read Sector
	//   LDA  >$2000     ; Read byte from buffer
	//   STA  >$FF89     ; Transmit to ACIA data register
	//   FCB  $12,$21,$03 ; Hypercall 3 = Exit(A)
	prog := []byte{
		0x7F, 0xFF, 0x86, // CLR $FF86
		0x7F, 0xFF, 0x80, // CLR $FF80
		0x7F, 0xFF, 0x81, // CLR $FF81
		0x7F, 0xFF, 0x82, // CLR $FF82
		0x8E, 0x20, 0x00, // LDX #$2000
		0xBF, 0xFF, 0x84, // STX $FF84
		0x7F, 0xFF, 0x83, // CLR $FF83
		0xB6, 0x20, 0x00, // LDA $2000
		0xB7, 0xFF, 0x89, // STA $FF89
		0x1F, 0x89,       // TFR A,B
		0x4F,             // CLRA
		0x12, 0x21, 107,  // Hypercall 107 = Exit(D)
	}

	copy(bus.Memory[0][0x1000:], prog)
	// Set Reset vector to $1000
	bus.Memory[0][0xFFFE] = 0x10
	bus.Memory[0][0xFFFF] = 0x00

	cpu := NewCPU(bus)
	cpu.EnableHypercalls = true
	cpu.Reset()

	for !cpu.Halted {
		cpu.Step()
	}

	if outBuf.String() != "Z" {
		t.Fatalf("expected 'Z' on console out, got %q", outBuf.String())
	}
	if cpu.ExitCode != 'Z' {
		t.Fatalf("expected exit code 'Z' (%d), got %d", 'Z', cpu.ExitCode)
	}
}

func TestFlat65280v2ClockAndStopRegister(t *testing.T) {
	bus := NewBus()
	bus.Engine = EngineFlat65280v2

	// 1. Initial value is 0
	if got := bus.ReadByte(0xFF87); got != 0x00 {
		t.Fatalf("expected initial CLOCK_AND_STOP to be 0, got 0x%02X", got)
	}

	// 2. User writes normal bits (bits 7..1) with low bit = 0
	bus.WriteByte(0xFF87, 0x20) // Rate 6 Hz, low bit 0
	if got := bus.ReadByte(0xFF87); got != 0x20 {
		t.Fatalf("expected 0x20, got 0x%02X", got)
	}
	if hz := bus.ClockRateHz(); hz != 6.0 {
		t.Fatalf("expected 6.0 Hz, got %f", hz)
	}
	if cpt := bus.ClockCyclesPerTick(2000000); cpt != 333333 {
		t.Fatalf("expected 333333 cycles per tick, got %d", cpt)
	}

	// 3. Test rates 60Hz ($00), 50Hz ($10), 0.1Hz ($30)
	bus.WriteByte(0xFF87, 0x00)
	if bus.ClockRateHz() != 60.0 || bus.ClockCyclesPerTick(2000000) != 33333 {
		t.Fatalf("expected 60 Hz / 33333 cpt")
	}
	bus.WriteByte(0xFF87, 0x10)
	if bus.ClockRateHz() != 50.0 || bus.ClockCyclesPerTick(2000000) != 40000 {
		t.Fatalf("expected 50 Hz / 40000 cpt")
	}
	bus.WriteByte(0xFF87, 0x30)
	if bus.ClockRateHz() != 0.1 || bus.ClockCyclesPerTick(2000000) != 20000000 {
		t.Fatalf("expected 0.1 Hz / 20000000 cpt")
	}

	// 4. Clock tick sets low bit (firing condition)
	bus.ClockTick()
	if got := bus.ReadByte(0xFF87); got != 0x31 {
		t.Fatalf("expected 0x31 after ClockTick, got 0x%02X", got)
	}

	// 5. Write with low bit = 0 preserves firing condition
	bus.WriteByte(0xFF87, 0x12) // Rate 50 Hz, IRQ enable, low bit = 0
	if got := bus.ReadByte(0xFF87); got != 0x13 {
		t.Fatalf("expected 0x13 (bit 0 preserved when writing low bit 0), got 0x%02X", got)
	}

	// 6. Write with low bit = 1 turns off firing condition
	bus.WriteByte(0xFF87, 0x13) // low bit = 1
	if got := bus.ReadByte(0xFF87); got != 0x12 {
		t.Fatalf("expected 0x12 (bit 0 cleared when writing low bit 1), got 0x%02X", got)
	}

	// Writing 0x01 resets firing condition and clears upper bits
	bus.ClockTick()
	if got := bus.ReadByte(0xFF87); got != 0x13 {
		t.Fatalf("expected 0x13 after ClockTick, got 0x%02X", got)
	}
	bus.WriteByte(0xFF87, 0x01)
	if got := bus.ReadByte(0xFF87); got != 0x00 {
		t.Fatalf("expected 0x00 after writing 0x01, got 0x%02X", got)
	}

	// 7. Special values with explanation printing
	outBuf := new(bytes.Buffer)
	logBuf := new(bytes.Buffer)
	bus.ConsoleOut = outBuf
	bus.LogOut = logBuf

	// $FC -> exit(0)
	halted := false
	haltCode := -1
	bus.OnHalt = func(code int) {
		halted = true
		haltCode = code
	}
	bus.WriteByte(0xFF87, 0xFC)
	if !halted || haltCode != 0 {
		t.Fatalf("expected halt with 0 on 0xFC, got halted=%v code=%d", halted, haltCode)
	}
	expectedFC := "\n *** Exiting with status 0 due to CLOCK_AND_STOP command $FC.\n"
	if !strings.Contains(outBuf.String(), expectedFC) || !strings.Contains(logBuf.String(), expectedFC) {
		t.Fatalf("expected explanation in outBuf and logBuf, got out=%q log=%q", outBuf.String(), logBuf.String())
	}

	// $FD -> exit(1)
	outBuf.Reset()
	logBuf.Reset()
	halted = false
	haltCode = -1
	bus.WriteByte(0xFF87, 0xFD)
	if !halted || haltCode != 1 {
		t.Fatalf("expected halt with 1 on 0xFD, got halted=%v code=%d", halted, haltCode)
	}
	expectedFD := "\n *** Exiting with status 1 due to CLOCK_AND_STOP command $FD.\n"
	if !strings.Contains(outBuf.String(), expectedFD) || !strings.Contains(logBuf.String(), expectedFD) {
		t.Fatalf("expected explanation in outBuf and logBuf, got out=%q log=%q", outBuf.String(), logBuf.String())
	}

	// $FE -> crash hook called
	outBuf.Reset()
	logBuf.Reset()
	crashed := false
	bus.OnCrash = func() {
		crashed = true
	}
	bus.WriteByte(0xFF87, 0xFE)
	if !crashed {
		t.Fatalf("expected OnCrash to be called on 0xFE")
	}
	expectedFE := "\n *** Crashing due to CLOCK_AND_STOP command $FE.\n"
	if !strings.Contains(outBuf.String(), expectedFE) || !strings.Contains(logBuf.String(), expectedFE) {
		t.Fatalf("expected explanation in outBuf and logBuf, got out=%q log=%q", outBuf.String(), logBuf.String())
	}
}

func TestFlat65280v2ClockIRQ(t *testing.T) {
	bus := NewBus()
	bus.Engine = EngineFlat65280v2
	cpu := NewCPU(bus)

	if cpu.irqLine {
		t.Fatalf("expected initial irqLine false")
	}

	// 1. Tick without IRQ enable ($02) does not assert IRQ
	bus.ClockTick()
	if (bus.ClockAndStop & 0x01) == 0 {
		t.Fatalf("expected tick bit set")
	}
	if cpu.irqLine {
		t.Fatalf("expected irqLine false when IRQ disabled ($02 is 0)")
	}

	// 2. Enabling IRQ while tick is already set asserts IRQ
	bus.WriteByte(0xFF87, 0x02) // bit 1 = 1, bit 0 = 0 (so tick bit 0 is preserved!)
	if !cpu.irqLine {
		t.Fatalf("expected irqLine true after enabling IRQ with tick pending")
	}

	// 3. Acknowledging tick with low bit 1 deasserts IRQ
	bus.WriteByte(0xFF87, 0x03) // acknowledge tick while keeping IRQ enable set
	if cpu.irqLine {
		t.Fatalf("expected irqLine false after acknowledging tick")
	}
	if bus.ReadByte(0xFF87) != 0x02 {
		t.Fatalf("expected register to be 0x02, got 0x%02X", bus.ReadByte(0xFF87))
	}

	// 4. Tick with IRQ enable asserted asserts IRQ
	bus.ClockTick()
	if !cpu.irqLine {
		t.Fatalf("expected irqLine true on ClockTick when enabled")
	}

	// 5. Test combined ACIA and Clock IRQ
	// Enable ACIA IRQ and enqueue char
	bus.WriteByte(0xFF88, 0x80) // RIE = 1
	bus.EnqueueKey('X')
	// Both ACIA and Clock have IRQ active
	if !cpu.irqLine {
		t.Fatalf("expected irqLine true with both ACIA and clock")
	}
	// Acknowledge Clock
	bus.WriteByte(0xFF87, 0x03)
	// IRQ should still be active because ACIA is pending
	if !cpu.irqLine {
		t.Fatalf("expected irqLine true while ACIA character is still pending")
	}
	// Read ACIA data
	ch := bus.ReadByte(0xFF89)
	if ch != 'X' {
		t.Fatalf("expected 'X', got %q", ch)
	}
	// Now both are cleared, IRQ line should be false
	if cpu.irqLine {
		t.Fatalf("expected irqLine false after both clock and ACIA cleared")
	}
}

func TestFlat65280v2ClockCPUExecution(t *testing.T) {
	bus := NewBus()
	bus.Engine = EngineFlat65280v2
	bus.ConsoleOut = new(bytes.Buffer)
	bus.LogOut = new(bytes.Buffer)
	cpu := NewCPU(bus)

	// Memory layout:
	//   $1000: Main code
	//     LDA #$02         ; 86 02
	//     STA $FF87        ; B7 FF 87 (enable clock IRQ)
	//     ANDCC #$EF       ; 1C EF (enable IRQs by clearing FlagI)
	//   .loop:
	//     BRA .loop        ; 20 FE
	//
	//   $2000: Clock ISR
	//     LDA $FF87        ; B6 FF 87
	//     ORA #$01         ; 8A 01
	//     STA $FF87        ; B7 FF 87 (turn off firing condition)
	//     INC $0050        ; 7C 00 50 (count tick)
	//     RTI              ; 3B
	//
	//   Vectors:
	//     $FFF8: $2000 (IRQ)
	//     $FFFE: $1000 (RESET)

	mainCode := []byte{
		0x86, 0x02,
		0xB7, 0xFF, 0x87,
		0x1C, 0xEF,
		0x20, 0xFE,
	}
	isrCode := []byte{
		0xB6, 0xFF, 0x87,
		0x8A, 0x01,
		0xB7, 0xFF, 0x87,
		0x7C, 0x00, 0x50,
		0x3B,
	}

	copy(bus.Memory[0][0x1000:], mainCode)
	copy(bus.Memory[0][0x2000:], isrCode)

	// Set hardware stack
	cpu.S = 0x0500

	// Set vectors
	bus.Memory[0][0xFFF8] = 0x20
	bus.Memory[0][0xFFF9] = 0x00
	bus.Memory[0][0xFFFE] = 0x10
	bus.Memory[0][0xFFFF] = 0x00

	cpu.Reset()
	// Step through initialization until loop is reached
	for cpu.PC != 0x1007 {
		cpu.Step()
	}

	if (cpu.CC & FlagI) != 0 {
		t.Fatalf("expected FlagI cleared in CC")
	}

	// Now trigger a clock tick
	bus.ClockTick()
	if !cpu.irqLine {
		t.Fatalf("expected irqLine true after ClockTick")
	}

	// Step CPU: it should take IRQ, execute ISR, increment $0050, acknowledge tick, and RTI
	steps := 0
	for bus.Memory[0][0x0050] == 0 && steps < 100 {
		cpu.Step()
		steps++
	}

	if bus.Memory[0][0x0050] != 1 {
		t.Fatalf("expected $0050 to be incremented to 1 by clock ISR, got %d", bus.Memory[0][0x0050])
	}
	if cpu.irqLine {
		t.Fatalf("expected irqLine deasserted after ISR acknowledged tick")
	}

	// Test special stop value $FC execution by CPU
	// Overwrite loop with LDA #$FC; STA $FF87
	stopProg := []byte{0x86, 0xFC, 0xB7, 0xFF, 0x87}
	copy(bus.Memory[0][cpu.PC:], stopProg)

	for !cpu.Halted {
		cpu.Step()
	}
	if !cpu.Halted || cpu.ExitCode != 0 {
		t.Fatalf("expected CPU halted with exit code 0 on $FC, got halted=%v exit=%d", cpu.Halted, cpu.ExitCode)
	}

	// Test $FD execution
	cpu.Halted = false
	stopProg1 := []byte{0x86, 0xFD, 0xB7, 0xFF, 0x87}
	copy(bus.Memory[0][0x1050:], stopProg1)
	cpu.PC = 0x1050
	for !cpu.Halted {
		cpu.Step()
	}
	if !cpu.Halted || cpu.ExitCode != 1 {
		t.Fatalf("expected CPU halted with exit code 1 on $FD, got halted=%v exit=%d", cpu.Halted, cpu.ExitCode)
	}
}

func TestDeep65280v2MMUAndIO(t *testing.T) {
	bus := NewBus()
	bus.Engine = EngineDeep65280v2

	// Verify initial MMU mapping: Task 0 has pages $38..$3F
	if bus.MmuTask != 0 {
		t.Fatalf("expected initial MmuTask = 0, got %d", bus.MmuTask)
	}
	for i := 0; i < 8; i++ {
		if bus.MmuRegs[0][i] != byte(0x38+i) {
			t.Fatalf("expected MmuRegs[0][%d] = 0x%02X, got 0x%02X", i, 0x38+i, bus.MmuRegs[0][i])
		}
		if bus.MmuRegs[1][i] != byte(0x38+i) {
			t.Fatalf("expected MmuRegs[1][%d] = 0x%02X, got 0x%02X", i, 0x38+i, bus.MmuRegs[1][i])
		}
	}

	// Test write through slot 0 (mapped to block $38 = phys 0x70000)
	bus.WriteByte(0x0100, 0x42)
	if bus.PhysRam[0x70000+0x0100] != 0x42 {
		t.Fatalf("expected PhysRam[0x70100] = 0x42, got 0x%02X", bus.PhysRam[0x70000+0x0100])
	}
	if bus.ReadByte(0x0100) != 0x42 {
		t.Fatalf("expected ReadByte(0x0100) = 0x42, got 0x%02X", bus.ReadByte(0x0100))
	}

	// Change Task 0 slot 0 to point to block $05 (phys 0x0A000)
	bus.WriteByte(0xFFA0, 0x05)
	if bus.MmuRegs[0][0] != 0x05 {
		t.Fatalf("expected MmuRegs[0][0] = 5, got %d", bus.MmuRegs[0][0])
	}
	// Slot 0 should now read from block $05
	bus.PhysRam[0x05*8192+0x0100] = 0x99
	if bus.ReadByte(0x0100) != 0x99 {
		t.Fatalf("expected ReadByte(0x0100) = 0x99, got 0x%02X", bus.ReadByte(0x0100))
	}

	// Test Task switch via $FF91
	bus.WriteByte(0xFF91, 0x01)
	if bus.MmuTask != 1 {
		t.Fatalf("expected MmuTask = 1, got %d", bus.MmuTask)
	}
	// Task 1 slot 0 is still block $38
	if bus.ReadByte(0x0100) != 0x42 {
		t.Fatalf("expected ReadByte(0x0100) in Task 1 = 0x42, got 0x%02X", bus.ReadByte(0x0100))
	}

	// Map Task 1 slot 0 to block $10 via $FFA8
	bus.WriteByte(0xFFA8, 0x10)
	bus.PhysRam[0x10*8192+0x0100] = 0x77
	if bus.ReadByte(0x0100) != 0x77 {
		t.Fatalf("expected ReadByte(0x0100) in Task 1 = 0x77, got 0x%02X", bus.ReadByte(0x0100))
	}

	// Switch back to Task 0
	bus.WriteByte(0xFF91, 0x00)
	if bus.ReadByte(0x0100) != 0x99 {
		t.Fatalf("expected ReadByte(0x0100) back in Task 0 = 0x99, got 0x%02X", bus.ReadByte(0x0100))
	}

	// Test locked $FExx page:
	// Slot 7 is $E000..$FFFF. Map slot 7 to block $02
	bus.WriteByte(0xFFA7, 0x02)
	bus.PhysRam[0x02*8192+0x0050] = 0x11 // $E050 -> block 2 offset $0050
	bus.PhysRam[0x02*8192+0x1E50] = 0x22 // $FE50 in block 2
	bus.PhysRam[0x3F*8192+0x1E50] = 0x33 // $FE50 in block $3F

	// Reading $E050 should read block 2
	if bus.ReadByte(0xE050) != 0x11 {
		t.Fatalf("expected ReadByte(0xE050) = 0x11, got 0x%02X", bus.ReadByte(0xE050))
	}
	// Reading $FE50 MUST read from block $3F (locked page), NOT block 2!
	if bus.ReadByte(0xFE50) != 0x33 {
		t.Fatalf("expected ReadByte(0xFE50) = 0x33 (locked $FExx page), got 0x%02X", bus.ReadByte(0xFE50))
	}
	// Writing to $FE50 must write to block $3F
	bus.WriteByte(0xFE50, 0xAA)
	if bus.PhysRam[0x3F*8192+0x1E50] != 0xAA {
		t.Fatalf("expected PhysRam[block $3F + $1E50] = 0xAA, got 0x%02X", bus.PhysRam[0x3F*8192+0x1E50])
	}
	if bus.PhysRam[0x02*8192+0x1E50] != 0x22 {
		t.Fatalf("expected PhysRam[block 2 + $1E50] untouched (0x22), got 0x%02X", bus.PhysRam[0x02*8192+0x1E50])
	}

	// Test vectors $FFF0..$FFFF always map to block $3F
	bus.WriteByte(0xFFFE, 0xC0)
	bus.WriteByte(0xFFFF, 0xDE)
	if bus.PhysRam[0x3F*8192+0x1FFE] != 0xC0 || bus.PhysRam[0x3F*8192+0x1FFF] != 0xDE {
		t.Fatalf("expected vectors in PhysRam block $3F, got %02X%02X", bus.PhysRam[0x3F*8192+0x1FFE], bus.PhysRam[0x3F*8192+0x1FFF])
	}
	if bus.ReadByte(0xFFFE) != 0xC0 || bus.ReadByte(0xFFFF) != 0xDE {
		t.Fatalf("expected ReadByte vectors = C0DE, got %02X%02X", bus.ReadByte(0xFFFE), bus.ReadByte(0xFFFF))
	}

	// Test LoadRawImage
	rawImage := make([]byte, 65536)
	rawImage[0x1000] = 0x55
	rawImage[0xFE20] = 0x66
	bus.LoadRawImage(rawImage)
	if bus.MmuTask != 0 {
		t.Fatalf("expected LoadRawImage to reset MmuTask to 0")
	}
	if bus.PhysRam[0x70000+0x1000] != 0x55 {
		t.Fatalf("expected PhysRam to contain rawImage[0x1000]")
	}
	if bus.ReadByte(0x1000) != 0x55 {
		t.Fatalf("expected ReadByte(0x1000) = 0x55 after LoadRawImage, got 0x%02X", bus.ReadByte(0x1000))
	}
	if bus.ReadByte(0xFE20) != 0x66 {
		t.Fatalf("expected ReadByte(0xFE20) = 0x66 after LoadRawImage, got 0x%02X", bus.ReadByte(0xFE20))
	}
}



