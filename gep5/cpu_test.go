package gep5

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func Test6502BasicExecutionAndAddressing(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// Load a small program at $0200:
	//   CLD                 ; D8
	//   CLC                 ; 18
	//   LDA #40             ; A9 28
	//   ADC #2              ; 69 02
	//   STA $10             ; 85 10
	//   LDX #$05            ; A2 05
	//   STA $20,X           ; 95 20  -> writes 42 to $0025
	//   LDA #$00            ; A9 00
	//   STA $30             ; 85 30
	//   LDA #$12            ; A9 12
	//   STA $31             ; 85 31  -> pointer ($30) = $1200
	//   LDY #$04            ; A0 04
	//   LDA $10             ; A5 10
	//   STA ($30),Y         ; 91 30  -> writes 42 to $1204
	//   STA $FF05           ; 8D 05 FF -> exit(42)
	code := []byte{
		0xD8,
		0x18,
		0xA9, 40,
		0x69, 2,
		0x85, 0x10,
		0xA2, 0x05,
		0x95, 0x20,
		0xA9, 0x00,
		0x85, 0x30,
		0xA9, 0x12,
		0x85, 0x31,
		0xA0, 0x04,
		0xA5, 0x10,
		0x91, 0x30,
		0x8D, 0x05, 0xFF,
	}
	for i, b := range code {
		bus.WriteByte(uint16(0x0200+i), b)
	}
	bus.WriteWord(VecReset, 0x0200)
	cpu.Reset()

	for !cpu.Halted {
		cpu.Step()
	}

	if bus.ExitCode != 42 {
		t.Fatalf("expected ExitCode 42, got %d", bus.ExitCode)
	}
	if bus.ReadByte(0x0010) != 42 {
		t.Fatalf("expected $0010 == 42, got %d", bus.ReadByte(0x0010))
	}
	if bus.ReadByte(0x0025) != 42 {
		t.Fatalf("expected $0025 == 42, got %d", bus.ReadByte(0x0025))
	}
	if bus.ReadByte(0x1204) != 42 {
		t.Fatalf("expected $1204 == 42, got %d", bus.ReadByte(0x1204))
	}
}

func Test6502JSRAndRTS(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// $0200: JSR $0300; INX; STX $FF05
	// $0300: LDX #99; RTS
	bus.WriteByte(0x0200, 0x20) // JSR $0300
	bus.WriteWord(0x0201, 0x0300)
	bus.WriteByte(0x0203, 0xE8) // INX
	bus.WriteByte(0x0204, 0x8E) // STX $FF05
	bus.WriteWord(0x0205, 0xFF05)

	bus.WriteByte(0x0300, 0xA2) // LDX #99
	bus.WriteByte(0x0301, 99)
	bus.WriteByte(0x0302, 0x60) // RTS

	bus.WriteWord(VecReset, 0x0200)
	cpu.Reset()

	for !cpu.Halted {
		cpu.Step()
	}
	if bus.ExitCode != 100 {
		t.Fatalf("expected ExitCode 100, got %d", bus.ExitCode)
	}
}

func Test6502BRKTaskFuseAndRTI(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// Set IRQ/BRK vector at $FFFE..$FFFF in Task 0 to $0800
	bus.WriteWord(VecIRQ, 0x0800)

	// Kernel trap handler in Task 0 at $0800:
	//   LDA $FF60       ; AD 60 FF (read BRK signature byte)
	//   STA $0050       ; 85 50    (store in Task 0 ZP)
	//   LDA $FF61       ; AD 61 FF (read TrapSourceTask)
	//   STA $FF20       ; 8D 20 FF (arm TaskFuse back to source task)
	//   RTI             ; 40
	handler := []byte{
		0xAD, 0x60, 0xFF,
		0x85, 0x50,
		0xAD, 0x61, 0xFF,
		0x8D, 0x20, 0xFF,
		0x40,
	}
	for i, b := range handler {
		bus.Memory[0][0x0800+i] = b
	}

	// User program in Task 1 at $0400:
	//   BRK $37         ; 00 37 (syscall 0x37)
	//   LDA #$55        ; A9 55
	//   STA $0010       ; 85 10
	userProg := []byte{
		0x00, 0x37,
		0xA9, 0x55,
		0x85, 0x10,
	}
	for i, b := range userProg {
		bus.Memory[1][0x0400+i] = b
	}

	bus.CurrentTask = 1
	cpu.SP = 0xFF
	cpu.PC = 0x0400

	// 1. Execute BRK $37 in Task 1 -> switches to Task 0 at $0800
	cpu.Step()
	if bus.CurrentTask != 0 {
		t.Fatalf("expected BRK to switch to Task 0, got Task %d", bus.CurrentTask)
	}
	if cpu.PC != 0x0800 {
		t.Fatalf("expected PC=0x0800 in handler, got 0x%04X", cpu.PC)
	}

	// 2. Execute handler in Task 0 through RTI
	for i := 0; i < 5; i++ {
		cpu.Step()
	}

	if bus.Memory[0][0x0050] != 0x37 {
		t.Fatalf("expected Task 0 $0050 to record BRK signature 0x37, got 0x%02X", bus.Memory[0][0x0050])
	}
	if bus.CurrentTask != 1 {
		t.Fatalf("expected RTI with TaskFuse to return to Task 1, got Task %d", bus.CurrentTask)
	}
	if cpu.PC != 0x0402 {
		t.Fatalf("expected PC=0x0402 after RTI, got 0x%04X", cpu.PC)
	}

	// 3. Execute remaining user instructions in Task 1
	cpu.Step() // LDA #$55
	cpu.Step() // STA $0010
	if bus.Memory[1][0x0010] != 0x55 {
		t.Fatalf("expected Task 1 $0010 == 0x55, got 0x%02X", bus.Memory[1][0x0010])
	}
}

func Test6502TimerIRQAndDMACopy(t *testing.T) {
	bus := NewBus()
	cpu := NewCPU(bus)

	// Test DMA copy from Task 0 $1000 to Task 2 $2000
	copy(bus.Memory[0][0x1000:], []byte{0xCA, 0xFE, 0xBA, 0xBE})
	bus.WriteByte(0xFF24, 0)    // srcTask = 0
	bus.WriteByte(0xFF25, 0x00) // srcAddr = $001000
	bus.WriteByte(0xFF26, 0x10)
	bus.WriteByte(0xFF27, 0x00)
	bus.WriteByte(0xFF28, 2)    // dstTask = 2
	bus.WriteByte(0xFF29, 0x00) // dstAddr = $002000
	bus.WriteByte(0xFF2A, 0x20)
	bus.WriteByte(0xFF2B, 0x00)
	bus.WriteByte(0xFF2C, 0) // count = 4
	bus.WriteByte(0xFF2D, 0)
	bus.WriteByte(0xFF2E, 0)
	bus.WriteByte(0xFF2F, 4)
	bus.WriteByte(0xFF21, 1) // trigger DMA

	if !bytes.Equal(bus.Memory[2][0x2000:0x2004], []byte{0xCA, 0xFE, 0xBA, 0xBE}) {
		t.Fatalf("DMA copy mismatch: % X", bus.Memory[2][0x2000:0x2004])
	}

	// Test Timer IRQ
	bus.WriteWord(VecIRQ, 0x0900)
	// IRQ handler at $0900: LDA $FF02; STA $FF02; INC $0040; RTI
	irqCode := []byte{
		0xAD, 0x02, 0xFF,
		0x8D, 0x02, 0xFF,
		0xE6, 0x40,
		0x40,
	}
	copy(bus.Memory[0][0x0900:], irqCode)

	// Main code at $0200: LDA #$01; STA $FF03; CLI; NOP
	mainCode := []byte{
		0xA9, 0x01,
		0x8D, 0x03, 0xFF,
		0x58,
		0xEA,
	}
	copy(bus.Memory[0][0x0200:], mainCode)
	bus.WriteWord(VecReset, 0x0200)
	cpu.Reset()

	cpu.Step() // LDA #$01
	cpu.Step() // STA $FF03
	cpu.Step() // CLI
	bus.TimerTick()
	cpu.Step() // Services IRQ -> jumps to $0900
	if cpu.PC != 0x0900 {
		t.Fatalf("expected IRQ to vector to $0900, got 0x%04X", cpu.PC)
	}
	cpu.Step() // LDA $FF02
	cpu.Step() // STA $FF02
	cpu.Step() // INC $0040
	cpu.Step() // RTI
	if bus.Memory[0][0x0040] != 1 {
		t.Fatalf("expected IRQ counter $0040 == 1, got %d", bus.Memory[0][0x0040])
	}
	if cpu.PC != 0x0206 {
		t.Fatalf("expected RTI to return to $0206, got 0x%04X", cpu.PC)
	}
}

func TestDECBLoaderMagicX5(t *testing.T) {
	bus := NewBus()
	var buf bytes.Buffer
	// Magic header: 0xFD, 0x00, 0x00, 'x', '5'
	buf.Write([]byte{0xFD, 0x00, 0x00, 'x', '5'})
	// Data chunk at $0300: LDA #7; STA $FF05
	data := []byte{0xA9, 0x07, 0x8D, 0x05, 0xFF}
	buf.WriteByte(0x00)
	binary.Write(&buf, binary.BigEndian, uint16(len(data)))
	binary.Write(&buf, binary.BigEndian, uint16(0x0300))
	buf.Write(data)
	// Trailer at $0300
	buf.Write([]byte{0xFF, 0x00, 0x00, 0x03, 0x00})

	lb, err := bus.LoadDECB(&buf)
	if err != nil {
		t.Fatalf("LoadDECB failed: %v", err)
	}
	if !lb.HasEntry || lb.EntryPC != 0x0300 {
		t.Fatalf("unexpected entry: %+v", lb)
	}

	cpu := NewCPU(bus)
	for !cpu.Halted {
		cpu.Step()
	}
	if bus.ExitCode != 7 {
		t.Fatalf("expected ExitCode 7, got %d", bus.ExitCode)
	}
}
