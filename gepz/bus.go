package gepz

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
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
	ConsoleIn   []byte
	ConsoleOut  io.Writer
	LogOut      io.Writer
	StdinChan   <-chan byte
	CurlyEscape bool

	// VM Termination
	ExitCode int
	Halted   bool

	// Hatvan Disk I/O ($FF10..$FF17)
	DiskDrive   byte      // $FF10
	DiskSector  uint32    // $FF11..$FF13 (24-bit LSN)
	DiskMemTask byte      // $FF14
	DiskMemAddr uint16    // $FF15..$FF16
	DiskStatus  byte      // $FF17 (0 = busy, 1 = OKAY, >1 = error)
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
	LastPC     uint16

	// Hatvan TaskFlags ($FF2D..$FF2E)
	TaskFlagsTarget byte // $FF2D: target task to configure (default: 1)
	TaskFlags       [256]byte

	// Tunable shared memory curtain ($E000 by default) for Tasks 0, 1, 2
	SharedMemoryCurtain uint16

	// Trap State via I/O Port 60h
	TrapPending bool
	TrapVal     byte

	// Callbacks
	CPU          any
	OnHalt       func(exitCode int)
	OnIRQChanged func(asserted bool)
	OnNMITrigger func()
}

// NewBus constructs an initialized Bus with Task 0 allocated.
func NewBus() *Bus {
	curtain := uint16(0xE000)
	if s := os.Getenv("HATVAN_SHARED_CURTAIN"); s != "" {
		if v, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "$"), 16, 16); err == nil {
			curtain = uint16(v)
		}
	}
	b := &Bus{
		CurrentTask:         0,
		TaskFlagsTarget:     1,
		SharedMemoryCurtain: curtain,
		ConsoleOut:          os.Stdout,
		LogOut:              os.Stderr,
	}
	return b
}

func (b *Bus) ReadByte(addr uint16) byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.readByteLocked(addr)
}

func (b *Bus) WriteByte(addr uint16, val byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.writeByteLocked(addr, val)
}

func (b *Bus) ReadWord(addr uint16) uint16 {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Little-endian for Z80 memory
	lo := uint16(b.readByteLocked(addr))
	hi := uint16(b.readByteLocked(addr + 1))
	return (hi << 8) | lo
}

func (b *Bus) WriteWord(addr uint16, val uint16) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Little-endian for Z80 memory
	b.writeByteLocked(addr, byte(val&0xFF))
	b.writeByteLocked(addr+1, byte((val>>8)&0xFF))
}

func (b *Bus) readByteLocked(addr uint16) byte {
	if addr >= 0xFF00 {
		if b.CurrentTask == 0 || (b.TaskFlags[b.CurrentTask]&0x01) != 0 {
			return b.readIOLocked(addr)
		}
		panic(fmt.Errorf("%w: read at 0x%04X in task %d", ErrUserAccessTrap, addr, b.CurrentTask))
	}

	if b.SharedMemoryCurtain > 0 && addr >= b.SharedMemoryCurtain && b.CurrentTask <= 2 {
		return b.Memory[0][addr]
	}

	return b.Memory[b.CurrentTask][addr]
}

func (b *Bus) writeByteLocked(addr uint16, val byte) {
	if addr >= 0xFF00 {
		if b.CurrentTask == 0 || (b.TaskFlags[b.CurrentTask]&0x01) != 0 {
			b.writeIOLocked(addr, val)
			return
		}
		panic(fmt.Errorf("%w: write at 0x%04X in task %d", ErrUserAccessTrap, addr, b.CurrentTask))
	}

	if b.SharedMemoryCurtain > 0 && addr >= b.SharedMemoryCurtain && b.CurrentTask <= 2 {
		b.Memory[0][addr] = val
		return
	}

	b.Memory[b.CurrentTask][addr] = val
}

func (b *Bus) readIOLocked(addr uint16) byte {
	switch addr {
	case 0xFF00: // putchar read (echo last or status)
		return 0
	case 0xFF01: // getchar
		if len(b.ConsoleIn) > 0 {
			ch := b.ConsoleIn[0]
			b.ConsoleIn = b.ConsoleIn[1:]
			if len(b.ConsoleIn) == 0 {
				b.RegStat &^= 0x02
			} else {
				b.RegStat |= 0x02
			}
			b.evalInterrupts()
			return ch
		}
		if b.StdinChan == nil {
			return 0x04
		}
		return 0
	case 0xFF02: // RegStat
		return b.RegStat
	case 0xFF03: // RegCtrl
		return b.RegCtrl
	case 0xFF05: // ExitCode
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
		if st != 0 {
			b.DiskStatus = 0
		}
		return st
	case 0xFF20:
		if b.TaskFuseArmed {
			return b.TaskFuseTarget
		}
		return b.CurrentTask
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
		if st != 0 {
			b.DmaStatus = 0
		}
		return st
	case 0xFF2D:
		return b.TaskFlagsTarget
	case 0xFF2E:
		return b.TaskFlags[b.TaskFlagsTarget]
	case 0xFF60:
		return b.TrapVal
	default:
		// Return 0 for other unmapped ports to prevent panic unless strictly requested
		return 0
	}
}

func (b *Bus) writeIOLocked(addr uint16, val byte) {
	switch addr {
	case 0xFF00: // putchar
		if b.ConsoleOut != nil {
			b.ConsoleOut.Write([]byte{val})
		}
	case 0xFF02: // RegStat
		b.RegStat &^= val
		b.evalInterrupts()
	case 0xFF03: // RegCtrl
		b.RegCtrl = val
		b.evalInterrupts()
	case 0xFF04: // logchar
		if b.LogOut != nil {
			b.LogOut.Write([]byte{val})
		}
	case 0xFF05: // ExitCode
		b.ExitCode = int(val)
		b.Halted = true
		if b.OnHalt != nil {
			b.OnHalt(b.ExitCode)
		}
	case 0xFF10:
		b.DiskDrive = val
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
	case 0xFF17:
		b.executeDiskCommandLocked(val)
	case 0xFF20:
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
	case 0xFF27:
		b.executeDmaCopyLocked(val)
	case 0xFF2D:
		b.TaskFlagsTarget = val
	case 0xFF2E:
		b.TaskFlags[b.TaskFlagsTarget] = val
	case 0xFF2F:
		if val > 0 {
			b.Memory[val] = [65536]byte{}
			b.TaskFlags[val] = 0
		}
	}
}

func (b *Bus) executeDiskCommandLocked(cmd byte) {
	if b.DiskDrive >= 4 || b.Disks[b.DiskDrive] == nil {
		b.DiskStatus = 2 // Error: drive not ready
		return
	}
	disk := b.Disks[b.DiskDrive]
	offset := int64(b.DiskSector) * 256
	if offset < 0 || offset+256 > int64(len(disk)) {
		b.DiskStatus = 3 // Error: sector out of range
		return
	}

	task := b.DiskMemTask
	addr := b.DiskMemAddr

	switch cmd {
	case 1: // Read Sector
		for i := 0; i < 256; i++ {
			tgtAddr := addr + uint16(i)
			if b.SharedMemoryCurtain > 0 && tgtAddr >= b.SharedMemoryCurtain && task <= 2 {
				b.Memory[0][tgtAddr] = disk[offset+int64(i)]
			} else {
				b.Memory[task][tgtAddr] = disk[offset+int64(i)]
			}
		}
		b.DiskStatus = 1 // OKAY
	case 2: // Write Sector
		for i := 0; i < 256; i++ {
			srcAddr := addr + uint16(i)
			if b.SharedMemoryCurtain > 0 && srcAddr >= b.SharedMemoryCurtain && task <= 2 {
				disk[offset+int64(i)] = b.Memory[0][srcAddr]
			} else {
				disk[offset+int64(i)] = b.Memory[task][srcAddr]
			}
		}
		b.DiskStatus = 1 // OKAY
	default:
		b.DiskStatus = 4 // Error: unknown command
	}
	if os.Getenv("TRACE_DISK") != "" {
		fmt.Fprintf(os.Stderr, "[DISK] cmd=%d drv=%d sec=%d task=%d addr=0x%04X -> st=%d\n",
			cmd, b.DiskDrive, b.DiskSector, task, addr, b.DiskStatus)
	}
}

func (b *Bus) executeDmaCopyLocked(count byte) {
	n := int(count)
	if n == 0 {
		n = 256
	}
	srcTask := b.DmaSrcTask
	srcAddr := b.DmaSrcAddr
	dstTask := b.DmaDstTask
	dstAddr := b.DmaDstAddr

	for i := 0; i < n; i++ {
		sAddr := srcAddr + uint16(i)
		dAddr := dstAddr + uint16(i)

		var val byte
		if b.SharedMemoryCurtain > 0 && sAddr >= b.SharedMemoryCurtain && srcTask <= 2 {
			val = b.Memory[0][sAddr]
		} else {
			val = b.Memory[srcTask][sAddr]
		}

		if b.SharedMemoryCurtain > 0 && dAddr >= b.SharedMemoryCurtain && dstTask <= 2 {
			b.Memory[0][dAddr] = val
		} else {
			b.Memory[dstTask][dAddr] = val
		}
	}
	b.DmaStatus = 1 // OKAY

	if os.Getenv("TRACE_DMA") != "" && ((dstTask == 1 && dstAddr == 0x0080) || (srcTask == 1 && srcAddr == 0x0080)) {
		mb := b.Memory[1][0x0080 : 0x0080+64]
		pathStr := string(bytes.TrimRight(mb[24:56], "\x00"))
		fmt.Fprintf(os.Stderr, "[DMA-MB] srcTask=%d srcAddr=0x%04X dstTask=%d dstAddr=0x%04X cmd=%d drv=%d st=%d path=%q fdLSN=%d\n",
			srcTask, srcAddr, dstTask, dstAddr, mb[0], mb[1], mb[3], pathStr, (uint16(mb[56])<<8)|uint16(mb[57]))
	}
}

// ReadPort reads from an 8-bit or 16-bit I/O port.
func (b *Bus) ReadPort(port uint16) byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	port8 := byte(port & 0xFF)
	switch port8 {
	case 0x60:
		return b.TrapVal
	}
	return 0
}

// WritePort writes to an 8-bit or 16-bit I/O port.
func (b *Bus) WritePort(port uint16, val byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	port8 := byte(port & 0xFF)
	switch port8 {
	case 0x60: // Syscall Trap Port
		b.TrapPending = true
		b.TrapVal = val
		if b.OnNMITrigger != nil {
			b.OnNMITrigger()
		}
	}
}

func (b *Bus) ReadUserString(task uint8, addr uint16) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var sb strings.Builder
	for i := 0; i < 256; i++ {
		c := b.Memory[task][addr+uint16(i)]
		if c == 0 || c == '\r' || c == '\n' {
			break
		}
		if (c & 0x80) != 0 {
			sb.WriteByte(c & 0x7F)
			break
		}
		sb.WriteByte(c)
	}
	return sb.String()
}

func (b *Bus) evalInterrupts() {
	asserted := (b.RegStat & b.RegCtrl & 0x03) != 0
	if b.OnIRQChanged != nil {
		b.OnIRQChanged(asserted)
	}
}

// EnqueueKey simulates typing a key, translating '\n' to '\r'.
func (b *Bus) EnqueueKey(ch byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch == '\n' {
		ch = '\r'
	}
	b.ConsoleIn = append(b.ConsoleIn, ch)
	b.RegStat |= 0x02 // Term.RxReady
	b.evalInterrupts()
}

// EnqueueString enqueues multiple characters into ConsoleIn.
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
		b.RegStat |= 0x02
		b.evalInterrupts()
	}
}

// ConsoleInEmpty returns true if no console input characters are pending.
func (b *Bus) ConsoleInEmpty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.ConsoleIn) == 0
}

// PollStdin non-blockingly drains available characters from StdinChan into ConsoleIn.
func (b *Bus) PollStdin() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pollStdinInternal()
}

func (b *Bus) pollStdinInternal() {
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
				b.evalInterrupts()
				return
			}
			if ch == '\n' {
				ch = '\r'
			}
			b.ConsoleIn = append(b.ConsoleIn, ch)
			added = true
		default:
			if added || len(b.ConsoleIn) > 0 {
				b.RegStat |= 0x02
				b.evalInterrupts()
			}
			return
		}
	}
}

