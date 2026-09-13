package vm

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
