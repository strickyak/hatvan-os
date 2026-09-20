package gepk

import (
	"bytes"
	"testing"
)

func TestBusMemoryAccess(t *testing.T) {
	bus := NewBus()
	bus.CurrentFC = FCSupervisorData

	// Byte access
	bus.WriteByte(0x1000, 0x42)
	if got := bus.ReadByte(0x1000); got != 0x42 {
		t.Fatalf("ReadByte(0x1000) = 0x%02X, want 0x42", got)
	}

	// Word access
	bus.WriteWord(0x2000, 0x1234)
	if got := bus.ReadWord(0x2000); got != 0x1234 {
		t.Fatalf("ReadWord(0x2000) = 0x%04X, want 0x1234", got)
	}

	// Long access
	bus.WriteLong(0x3000, 0xDEADBEEF)
	if got := bus.ReadLong(0x3000); got != 0xDEADBEEF {
		t.Fatalf("ReadLong(0x3000) = 0x%08X, want 0xDEADBEEF", got)
	}
}

func TestBusAddressError(t *testing.T) {
	bus := NewBus()
	bus.CurrentFC = FCSupervisorData

	assertPanic := func(fn func(), desc string) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for %s, but did not panic", desc)
			}
		}()
		fn()
	}

	// Word read at odd address
	assertPanic(func() { bus.ReadWord(0x1001) }, "ReadWord(0x1001)")

	// Word write at odd address
	assertPanic(func() { bus.WriteWord(0x1001, 0x1234) }, "WriteWord(0x1001)")

	// Long read at odd address
	assertPanic(func() { bus.ReadLong(0x1001) }, "ReadLong(0x1001)")

	// Long write at odd address
	assertPanic(func() { bus.WriteLong(0x1001, 0x12345678) }, "WriteLong(0x1001)")
}

func TestBusUserAccessProtection(t *testing.T) {
	bus := NewBus()
	bus.CurrentFC = FCUserData // User data access

	assertPanic := func(fn func(), desc string) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for %s, but did not panic", desc)
			}
		}()
		fn()
	}

	// User cannot access I/O page $FF0000..$FFFFFF
	assertPanic(func() { bus.ReadByte(0x00FF0000) }, "User ReadByte($FF0000)")
	assertPanic(func() { bus.WriteByte(0x00FF0000, 0x55) }, "User WriteByte($FF0000)")
}

func TestBusTaskFlagsIOBlessing(t *testing.T) {
	bus := NewBus()

	assertPanic := func(fn func(), desc string) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for %s, but did not panic", desc)
			}
		}()
		fn()
	}

	// 1. Task 1 in User mode unblessed: accessing $FF0000 panics
	bus.CurrentFC = FCUserData
	bus.TaskReg = 1
	assertPanic(func() { bus.ReadByte(0x00FF0000) }, "Unblessed Task 1 ReadByte")
	assertPanic(func() { bus.WriteByte(0x00FF0000, 0x41) }, "Unblessed Task 1 WriteByte")

	// 2. Supervisor blesses Task 1 via $00FF005C = 0x01
	bus.CurrentFC = FCSupervisorData
	bus.WriteByte(0x00FF005C, 0x01)
	if got := bus.ReadByte(0x00FF005C); got != 0x01 {
		t.Fatalf("ReadByte(0x00FF005C) = 0x%02X, want 0x01", got)
	}

	// 3. Task 1 in User mode now has I/O access
	bus.CurrentFC = FCUserData
	bus.TaskReg = 1
	bus.WriteByte(0x00FF0010, 0x03) // Set DiskDrive to 3
	if bus.DiskDrive != 3 {
		t.Fatalf("expected DiskDrive=3 from blessed Task 1, got %d", bus.DiskDrive)
	}

	// 4. Task 2 in User mode remains unblessed and panics
	bus.TaskReg = 2
	assertPanic(func() { bus.ReadByte(0x00FF0000) }, "Unblessed Task 2 ReadByte")

	// 5. Purging Task 1 revokes blessing
	bus.CurrentFC = FCSupervisorData
	bus.WriteByte(0x00FF005E, 0x01) // Purge Task 1
	if got := bus.ReadByte(0x00FF005C); got != 0x00 {
		t.Fatalf("ReadByte(0x00FF005C) after purge = 0x%02X, want 0x00", got)
	}

	// Task 1 now panics again
	bus.CurrentFC = FCUserData
	bus.TaskReg = 1
	assertPanic(func() { bus.ReadByte(0x00FF0000) }, "Task 1 ReadByte after purge")
}

func TestBusTaskRouting(t *testing.T) {
	bus := NewBus()

	// Write to Task 0 in Supervisor mode
	bus.CurrentFC = FCSupervisorData
	bus.WriteLong(0x00010000, 0xAAAAAAAA)

	// Switch TaskReg to Task 2
	bus.WriteWord(0x00FF0020, 0x0002)

	// Write to Task 2 in User mode at same logical address
	bus.CurrentFC = FCUserData
	bus.WriteLong(0x00010000, 0xBBBBBBBB)

	// Read back in User mode (should read Task 2)
	if got := bus.ReadLong(0x00010000); got != 0xBBBBBBBB {
		t.Fatalf("Task 2 ReadLong = 0x%08X, want 0xBBBBBBBB", got)
	}

	// Read back in Supervisor mode (should read Task 0)
	bus.CurrentFC = FCSupervisorData
	if got := bus.ReadLong(0x00010000); got != 0xAAAAAAAA {
		t.Fatalf("Task 0 ReadLong = 0x%08X, want 0xAAAAAAAA", got)
	}
}

func TestBusDMAEngine(t *testing.T) {
	bus := NewBus()
	bus.CurrentFC = FCSupervisorData

	// Setup data in Task 1 at 0x1000
	bus.Tasks[1].writeByte(0x1000, 0x11)
	bus.Tasks[1].writeByte(0x1001, 0x22)
	bus.Tasks[1].writeByte(0x1002, 0x33)
	bus.Tasks[1].writeByte(0x1003, 0x44)

	// Program DMA registers: copy 4 bytes from Task 1:0x1000 to Task 2:0x2000
	bus.WriteWord(0x00FF0022, 0x0001)     // Src Task 1
	bus.WriteLong(0x00FF0024, 0x00001000) // Src Addr
	bus.WriteWord(0x00FF0028, 0x0002)     // Dst Task 2
	bus.WriteLong(0x00FF002A, 0x00002000) // Dst Addr
	bus.WriteLong(0x00FF002E, 0x00000004) // Count 4 bytes
	bus.WriteWord(0x00FF0032, 0x0001)     // Trigger DMA command

	// Check status
	stat := bus.ReadWord(0x00FF0032)
	if stat != 1 {
		t.Fatalf("DMA status = %d, want 1 (OKAY)", stat)
	}

	// Verify Task 2 received bytes
	if b := bus.Tasks[2].readByte(0x2000); b != 0x11 {
		t.Fatalf("Task 2 [0x2000] = 0x%02X, want 0x11", b)
	}
	if b := bus.Tasks[2].readByte(0x2001); b != 0x22 {
		t.Fatalf("Task 2 [0x2001] = 0x%02X, want 0x22", b)
	}
	if b := bus.Tasks[2].readByte(0x2002); b != 0x33 {
		t.Fatalf("Task 2 [0x2002] = 0x%02X, want 0x33", b)
	}
	if b := bus.Tasks[2].readByte(0x2003); b != 0x44 {
		t.Fatalf("Task 2 [0x2003] = 0x%02X, want 0x44", b)
	}
}

func TestBusCurlyEscapeAndNewline(t *testing.T) {
	bus := NewBus()
	bus.CurrentFC = FCSupervisorData
	outBuf := new(bytes.Buffer)
	bus.ConsoleOut = outBuf

	// Without CurlyEscape:
	// Newline translation (13 -> \n, 10 -> \n)
	bus.WriteByte(0x00FF0000, 13)
	bus.WriteByte(0x00FF0000, 10)
	bus.WriteByte(0x00FF0000, 'A')
	bus.WriteByte(0x00FF0000, 7)
	if got := outBuf.String(); got != "\n\nA\x07" {
		t.Fatalf("expected \\n\\nA\\x07, got %q", got)
	}

	// With CurlyEscape:
	outBuf.Reset()
	bus.CurlyEscape = true
	bus.WriteByte(0x00FF0000, 13)
	bus.WriteByte(0x00FF0000, 10)
	bus.WriteByte(0x00FF0000, ' ')
	bus.WriteByte(0x00FF0000, 'Z')
	bus.WriteByte(0x00FF0000, '~')
	bus.WriteByte(0x00FF0000, 0)
	bus.WriteByte(0x00FF0000, 7)
	bus.WriteByte(0x00FF0000, 8)
	bus.WriteByte(0x00FF0000, 27)
	bus.WriteByte(0x00FF0000, 127)
	bus.WriteByte(0x00FF0000, 255)

	expected := "\n\n Z~{0}{7}{8}{27}{127}{255}"
	if got := outBuf.String(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

