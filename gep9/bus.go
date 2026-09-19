package gep9

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	ErrUserAccessTrap = errors.New("user process attempted access to protected page $FF00..$FFFF")
	ErrKernelIOPanic  = errors.New("kernel accessed unmapped port in page $FF00..$FFFF")
)

// Bus manages memory across all 256 tasks, page protection, and I/O devices.
type Bus struct {
	mu sync.Mutex

	// Memory[task][65536]
	Memory [256][65536]byte

	CurrentTask uint8

	// Turbo9Sim I/O registers ($FF00..$FF03)
	RegStat byte // $FF02: Bit 0 = Timer.Ready, Bit 1 = Term.RxReady
	RegCtrl byte // $FF03: Bit 0 = Ctrl.TimrIRQ, Bit 1 = Ctrl.TermIRQ

	// Console streams
	ConsoleIn  []byte
	ConsoleOut io.Writer
	LogOut     io.Writer
	StdinChan  <-chan byte

	// Hatvan Disk I/O ($FF10..$FF17)
	DiskDrive   byte     // $FF10
	DiskSector  uint32   // $FF11..$FF13 (24-bit LSN)
	DiskMemTask byte     // $FF14
	DiskMemAddr uint16   // $FF15..$FF16
	DiskStatus  byte     // $FF17 (0 = busy, 1 = OKAY, >1 = error)
	Disks       [4][]byte // Up to 4 disk images

	// Hatvan TaskFuse ($FF20)
	TaskFuseArmed  bool
	TaskFuseTarget uint8

	// Hatvan DMA Copy ($FF21..$FF27)
	DmaSrcTask byte   // $FF21
	DmaSrcAddr uint16 // $FF22..$FF23
	DmaDstTask byte   // $FF24
	DmaDstAddr uint16 // $FF25..$FF26
	DmaStatus  byte   // $FF27 (0 = busy, 1 = OKAY, >1 = error)

	// Hatvan TaskFlags ($FF2E)
	TaskFlags [256]byte

	// Callback when IRQ line state changes
	OnIRQChanged func(asserted bool)

	// Exit / Halt ($FF05)
	ExitCode int
	OnHalt   func(exitCode int)
}

// NewBus constructs an initialized Bus with all memory zeroed.
func NewBus() *Bus {
	return &Bus{
		CurrentTask: 0,
		ConsoleOut:  os.Stdout,
		LogOut:      os.Stderr,
	}
}

// ReadByte reads an 8-bit value from the current task's memory space.
func (b *Bus) ReadByte(addr uint16) byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	// In user tasks (Task > 0), access to $FF00..$FFFF is strictly forbidden unless blessed with I/O flag
	if addr >= 0xFF00 {
		if b.CurrentTask == 0 || (b.TaskFlags[b.CurrentTask]&0x01) != 0 {
			return b.readIO(addr)
		}
		panic(fmt.Errorf("%w: read at 0x%04X in task %d", ErrUserAccessTrap, addr, b.CurrentTask))
	}

	return b.Memory[b.CurrentTask][addr]
}

// WriteByte writes an 8-bit value to the current task's memory space.
func (b *Bus) WriteByte(addr uint16, val byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// In user tasks (Task > 0), access to $FF00..$FFFF is strictly forbidden unless blessed with I/O flag
	if addr >= 0xFF00 {
		if b.CurrentTask == 0 || (b.TaskFlags[b.CurrentTask]&0x01) != 0 {
			b.writeIO(addr, val)
			return
		}
		panic(fmt.Errorf("%w: write at 0x%04X in task %d", ErrUserAccessTrap, addr, b.CurrentTask))
	}

	b.Memory[b.CurrentTask][addr] = val
}

// ReadWord reads a big-endian 16-bit word.
func (b *Bus) ReadWord(addr uint16) uint16 {
	hi := uint16(b.ReadByte(addr))
	lo := uint16(b.ReadByte(addr + 1))
	return (hi << 8) | lo
}

// WriteWord writes a big-endian 16-bit word.
func (b *Bus) WriteWord(addr uint16, val uint16) {
	b.WriteByte(addr, byte(val>>8))
	b.WriteByte(addr+1, byte(val))
}

func (b *Bus) readIO(addr uint16) byte {
	// Vectors $FFF0..$FFFF are stored in Task 0 RAM
	if addr >= 0xFFF0 {
		return b.Memory[0][addr]
	}

	switch addr {
	case 0xFF00: // Term.Out
		return 0

	case 0xFF01: // Term.In
		if len(b.ConsoleIn) == 0 && b.StdinChan != nil {
			b.pollStdinLocked()
		}
		if len(b.ConsoleIn) > 0 {
			ch := b.ConsoleIn[0]
			b.ConsoleIn = b.ConsoleIn[1:]
			if len(b.ConsoleIn) == 0 {
				b.RegStat &^= 0x02 // Clear Term.RxReady
			} else {
				b.RegStat |= 0x02
			}
			b.evalIRQ()
			return ch
		}
		if b.StdinChan == nil {
			return 0x04
		}
		return 0 // Non-blocking: returns 0 if no character ready

	case 0xFF02: // Reg.Stat
		if len(b.ConsoleIn) == 0 && b.StdinChan != nil {
			b.pollStdinLocked()
		}
		return b.RegStat

	case 0xFF03: // Reg.Ctrl
		return b.RegCtrl

	case 0xFF04: // logchar
		return 0

	case 0xFF05: // exit code
		return byte(b.ExitCode)

	case 0xFF10:
		return b.DiskDrive
	case 0xFF11:
		return byte(b.DiskSector >> 16)
	case 0xFF12:
		return byte(b.DiskSector >> 8)
	case 0xFF13:
		return byte(b.DiskSector)
	case 0xFF14:
		return b.DiskMemTask
	case 0xFF15:
		return byte(b.DiskMemAddr >> 8)
	case 0xFF16:
		return byte(b.DiskMemAddr)
	case 0xFF17:
		st := b.DiskStatus
		b.DiskStatus = 0 // Reset on read
		return st

	case 0xFF20:
		if b.TaskFuseArmed {
			return b.TaskFuseTarget
		}
		return 0

	case 0xFF21:
		return b.DmaSrcTask
	case 0xFF22:
		return byte(b.DmaSrcAddr >> 8)
	case 0xFF23:
		return byte(b.DmaSrcAddr)
	case 0xFF24:
		return b.DmaDstTask
	case 0xFF25:
		return byte(b.DmaDstAddr >> 8)
	case 0xFF26:
		return byte(b.DmaDstAddr)
	case 0xFF27:
		st := b.DmaStatus
		b.DmaStatus = 0 // Reset on read
		return st

	case 0xFF2E: // TaskFlagsRegister
		return b.TaskFlags[1]

	case 0xFF2F: // PurgeTaskMem
		return 0

	default:
		panic(fmt.Errorf("%w: read at unmapped 0x%04X in Task 0", ErrKernelIOPanic, addr))
	}
}

func (b *Bus) writeIO(addr uint16, val byte) {
	// Vectors $FFF0..$FFFF can be configured in Task 0 RAM
	if addr >= 0xFFF0 {
		b.Memory[0][addr] = val
		return
	}

	switch addr {
	case 0xFF00: // Term.Out (putchar)
		if b.ConsoleOut != nil {
			b.ConsoleOut.Write([]byte{val})
		}

	case 0xFF01: // Term.In (read only)
		// Ignored on write

	case 0xFF02: // Reg.Stat: writing a 1 clears corresponding flag
		b.RegStat &^= (val & 0x03)
		if len(b.ConsoleIn) > 0 {
			b.RegStat |= 0x02
		}
		b.evalIRQ()

	case 0xFF03: // Reg.Ctrl: update control bits
		b.RegCtrl = val & 0x03
		b.evalIRQ()

	case 0xFF04: // logchar
		if b.LogOut != nil {
			b.LogOut.Write([]byte{val})
		}

	case 0xFF05: // exit code
		b.ExitCode = int(val)
		if b.OnHalt != nil {
			b.OnHalt(int(val))
		}

	case 0xFF10:
		b.DiskDrive = val & 0x03
	case 0xFF11:
		b.DiskSector = (b.DiskSector & 0x0000FFFF) | (uint32(val) << 16)
	case 0xFF12:
		b.DiskSector = (b.DiskSector & 0x00FF00FF) | (uint32(val) << 8)
	case 0xFF13:
		b.DiskSector = (b.DiskSector & 0x00FFFF00) | uint32(val)
	case 0xFF14:
		b.DiskMemTask = val
	case 0xFF15:
		b.DiskMemAddr = (b.DiskMemAddr & 0x00FF) | (uint16(val) << 8)
	case 0xFF16:
		b.DiskMemAddr = (b.DiskMemAddr & 0xFF00) | uint16(val)
	case 0xFF17: // Disk command: 1=Read, 2=Write
		b.executeDiskCommand(val)

	case 0xFF20: // TaskFuse
		b.TaskFuseArmed = true
		b.TaskFuseTarget = val

	case 0xFF21:
		b.DmaSrcTask = val
	case 0xFF22:
		b.DmaSrcAddr = (b.DmaSrcAddr & 0x00FF) | (uint16(val) << 8)
	case 0xFF23:
		b.DmaSrcAddr = (b.DmaSrcAddr & 0xFF00) | uint16(val)
	case 0xFF24:
		b.DmaDstTask = val
	case 0xFF25:
		b.DmaDstAddr = (b.DmaDstAddr & 0x00FF) | (uint16(val) << 8)
	case 0xFF26:
		b.DmaDstAddr = (b.DmaDstAddr & 0xFF00) | uint16(val)
	case 0xFF27: // DMA Copy command: length 1..255, 0 = 256
		b.executeDMACopy(val)

	case 0xFF2E: // TaskFlagsRegister: sets flags for Task 1 (Bit 0 = allow I/O)
		b.TaskFlags[1] = val

	case 0xFF2F: // PurgeTaskMem: zero task memory if val != 0
		if val != 0 {
			b.purgeTaskMem(val)
		}

	default:
		panic(fmt.Errorf("%w: write at unmapped 0x%04X in Task 0", ErrKernelIOPanic, addr))
	}
}

func (b *Bus) evalIRQ() {
	asserted := (b.RegStat & b.RegCtrl & 0x03) != 0
	if b.OnIRQChanged != nil {
		b.OnIRQChanged(asserted)
	}
}

// EnqueueKey simulates a user typing a key.
func (b *Bus) EnqueueKey(ch byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch == '\n' {
		ch = '\r'
	}
	b.ConsoleIn = append(b.ConsoleIn, ch)
	b.RegStat |= 0x02 // Term.RxReady
	b.evalIRQ()
}

// EnqueueString enqueues characters into ConsoleIn, translating '\n' to '\r' (OS-9 carriage return).
func (b *Bus) EnqueueString(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\n' {
			ch = '\r'
		}
		b.ConsoleIn = append(b.ConsoleIn, ch)
	}
	if len(b.ConsoleIn) > 0 {
		b.RegStat |= 0x02 // Term.RxReady
		b.evalIRQ()
	}
}

// ConsoleInEmpty returns true if ConsoleIn has no pending characters.
func (b *Bus) ConsoleInEmpty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.ConsoleIn) == 0
}

// PollStdin non-blockingly drains available characters from StdinChan into ConsoleIn.
func (b *Bus) PollStdin() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pollStdinLocked()
}

func (b *Bus) pollStdinLocked() {
	if b.StdinChan == nil {
		return
	}
	added := false
	for {
		select {
		case ch, ok := <-b.StdinChan:
			if !ok {
				b.StdinChan = nil
				b.ConsoleIn = append(b.ConsoleIn, 0x04)
				b.RegStat |= 0x02
				b.evalIRQ()
				return
			}
			if ch == '\n' {
				ch = '\r'
			}
			b.ConsoleIn = append(b.ConsoleIn, ch)
			added = true
		default:
			if added || len(b.ConsoleIn) > 0 {
				b.RegStat |= 0x02 // Term.RxReady
				b.evalIRQ()
			}
			return
		}
	}
}

// LoadRawImage zeroes Task 0 memory and loads a 64KB raw image directly into Task 0.
func (b *Bus) LoadRawImage(data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := range b.Memory[0] {
		b.Memory[0][i] = 0
	}
	copy(b.Memory[0][:], data)
	return nil
}

// TimerTick signals a 60Hz timer tick.
func (b *Bus) TimerTick() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.RegStat |= 0x01 // Timer.Ready
	b.evalIRQ()
}

func (b *Bus) executeDiskCommand(cmd byte) {
	drv := int(b.DiskDrive)
	if drv >= len(b.Disks) || b.Disks[drv] == nil {
		b.DiskStatus = 2 // Error: drive not ready
		return
	}
	disk := b.Disks[drv]
	offset := int(b.DiskSector) * 256
	if offset+256 > len(disk) {
		b.DiskStatus = 3 // Error: sector out of range
		return
	}

	task := b.DiskMemTask
	addr := b.DiskMemAddr

	switch cmd {
	case 1: // Read sector from disk into memory
		for i := 0; i < 256; i++ {
			b.Memory[task][addr+uint16(i)] = disk[offset+i]
		}
		b.DiskStatus = 1 // OKAY

	case 2: // Write sector from memory to disk
		for i := 0; i < 256; i++ {
			disk[offset+i] = b.Memory[task][addr+uint16(i)]
		}
		b.DiskStatus = 1 // OKAY

	default:
		b.DiskStatus = 4 // Error: unknown command
	}
}

func (b *Bus) executeDMACopy(lengthByte byte) {
	count := int(lengthByte)
	if count == 0 {
		count = 256
	}

	srcTask := b.DmaSrcTask
	srcAddr := b.DmaSrcAddr
	dstTask := b.DmaDstTask
	dstAddr := b.DmaDstAddr

	for i := 0; i < count; i++ {
		val := b.Memory[srcTask][srcAddr+uint16(i)]
		b.Memory[dstTask][dstAddr+uint16(i)] = val
	}

	b.DmaStatus = 1 // OKAY
}

func (b *Bus) purgeTaskMem(task uint8) {
	if task == 0 {
		return // Never purge Task 0 (kernel)
	}
	b.TaskFlags[task] = 0
	for i := range b.Memory[task] {
		b.Memory[task][i] = 0
	}
}

// LoadDECBIntoTask zeroes memory and loads a DECB structure into the specified task.
func (b *Bus) LoadDECBIntoTask(task uint8, d *DECB) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Zero target task memory
	for i := range b.Memory[task] {
		b.Memory[task][i] = 0
	}

	for _, seg := range d.Segments {
		for i, v := range seg.Data {
			b.Memory[task][seg.Addr+uint16(i)] = v
		}
	}

	if d.HasExec && task == 0 {
		// Set Reset Vector at $FFFE..$FFFF
		binary.BigEndian.PutUint16(b.Memory[0][0xFFFE:0x10000], d.ExecAddr)
	}
}
