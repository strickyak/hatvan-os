package gep5

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

// Bus manages memory across all 256 tasks, page protection, and I/O devices for the MOS 6502.
type Bus struct {
	mu sync.Mutex

	// Memory[task][65536]
	Memory [256][65536]byte

	CurrentTask uint8
	LastPC      uint16

	// Turbo9Sim / Hatvan I/O registers ($FF00..$FF03)
	RegStat byte // $FF02: Bit 0 = Timer.Ready, Bit 1 = Term.RxReady
	RegCtrl byte // $FF03: Bit 0 = Ctrl.TimrIRQ, Bit 1 = Ctrl.TermIRQ

	// Console streams
	ConsoleIn   []byte
	ConsoleOut  io.Writer
	LogOut      io.Writer
	StdinChan   <-chan byte
	CurlyEscape bool
	OnLogChar   func(val byte)

	// VM Termination
	ExitCode int
	Halted   bool

	// Hatvan Disk I/O ($FF10..$FF1B)
	DiskDrive      byte      // $FF10: Drive selector (0..3)
	DiskStatus     byte      // $FF11: Disk.CmdSt (0 = busy, 1 = OKAY, >1 = error)
	DiskSector     uint32    // $FF14..$FF17: 32-bit LSN (Big-Endian)
	DiskTargetTask byte      // $FF18: Target task
	DiskTargetAddr uint32    // $FF19..$FF1B: 24-bit task address (Big-Endian)
	Disks          [4][]byte // Up to 4 disk images

	// Hatvan TaskFuse ($FF20)
	TaskFuseArmed  bool
	TaskFuseTarget uint8

	// Hatvan DMA Copy ($FF21..$FF2F)
	DmaStatus  byte   // $FF21: DMA.CmdSt (0 = busy, 1 = OKAY, >1 = error)
	DmaSrcTask byte   // $FF24: Source task
	DmaSrcAddr uint32 // $FF25..$FF27: 24-bit source address (Big-Endian)
	DmaDstTask byte   // $FF28: Destination task
	DmaDstAddr uint32 // $FF29..$FF2B: 24-bit destination address (Big-Endian)
	DmaCount   uint32 // $FF2C..$FF2F: 32-bit transfer count (Big-Endian)

	// Hatvan Shared Curtain ($FF30..$FF33)
	SharedMemoryCurtain uint32 // $FF30..$FF33 (default 0x0000E000)

	// Hatvan TaskFlags ($FF34..$FF36)
	TaskFlagsTarget byte // $FF34: target task to configure (default: 1)
	TaskFlags       [256]byte

	// Hatvan Syscall Trap ($FF60..$FF61)
	TrapSyscallNum  byte // $FF60
	TrapSourceTask  byte // $FF61
	InterruptedTask byte

	// Callbacks
	CPU          any
	OnHalt       func(exitCode int)
	OnIRQChanged func(asserted bool)
	OnNMITrigger func()
}

// NewBus constructs an initialized Bus with Task 0 active.
func NewBus() *Bus {
	curtain := uint32(0x0000E000)
	if s := os.Getenv("HATVAN_SHARED_CURTAIN"); s != "" {
		if v, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "$"), 16, 32); err == nil {
			curtain = uint32(v)
		}
	}
	return &Bus{
		CurrentTask:         0,
		TaskFlagsTarget:     1,
		SharedMemoryCurtain: curtain,
		ConsoleOut:          os.Stdout,
		LogOut:              os.Stderr,
	}
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

// ReadWord reads a 16-bit little-endian word from memory (6502 convention).
func (b *Bus) ReadWord(addr uint16) uint16 {
	b.mu.Lock()
	defer b.mu.Unlock()
	lo := uint16(b.readByteLocked(addr))
	hi := uint16(b.readByteLocked(addr + 1))
	return (hi << 8) | lo
}

// WriteWord writes a 16-bit little-endian word into memory (6502 convention).
func (b *Bus) WriteWord(addr uint16, val uint16) {
	b.mu.Lock()
	defer b.mu.Unlock()
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

	curtain := uint16(b.SharedMemoryCurtain & 0xFFFF)
	if curtain > 0 && addr >= curtain && b.CurrentTask <= 2 {
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

	curtain := uint16(b.SharedMemoryCurtain & 0xFFFF)
	if curtain > 0 && addr >= curtain && b.CurrentTask <= 2 {
		b.Memory[0][addr] = val
		return
	}

	b.Memory[b.CurrentTask][addr] = val
}

func (b *Bus) readIOLocked(addr uint16) byte {
	// Hardware vectors $FFF0..$FFFF are stored in Task 0 RAM
	if addr >= 0xFFF0 {
		return b.Memory[0][addr]
	}

	switch addr {
	case 0xFF00: // putchar read
		return 0
	case 0xFF01: // getchar
		if len(b.ConsoleIn) == 0 && b.StdinChan != nil {
			b.pollStdinInternal()
		}
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
			return 0x04 // EOF
		}
		return 0
	case 0xFF02: // RegStat
		if len(b.ConsoleIn) == 0 && b.StdinChan != nil {
			b.pollStdinInternal()
		}
		return b.RegStat
	case 0xFF03: // RegCtrl
		return b.RegCtrl
	case 0xFF04: // Log.Out
		return 0
	case 0xFF05: // ExitCode
		return byte(b.ExitCode)

	case 0xFF10:
		return b.DiskDrive
	case 0xFF11:
		st := b.DiskStatus
		if st != 0 {
			b.DiskStatus = 0
		}
		return st
	case 0xFF12, 0xFF13:
		return 0
	case 0xFF14:
		return byte(b.DiskSector >> 24)
	case 0xFF15:
		return byte(b.DiskSector >> 16)
	case 0xFF16:
		return byte(b.DiskSector >> 8)
	case 0xFF17:
		return byte(b.DiskSector)
	case 0xFF18:
		return b.DiskTargetTask
	case 0xFF19:
		return byte(b.DiskTargetAddr >> 16)
	case 0xFF1A:
		return byte(b.DiskTargetAddr >> 8)
	case 0xFF1B:
		return byte(b.DiskTargetAddr)

	case 0xFF20:
		if b.TaskFuseArmed {
			return b.TaskFuseTarget
		}
		return b.CurrentTask
	case 0xFF21:
		st := b.DmaStatus
		if st != 0 {
			b.DmaStatus = 0
		}
		return st
	case 0xFF22, 0xFF23:
		return 0
	case 0xFF24:
		return b.DmaSrcTask
	case 0xFF25:
		return byte(b.DmaSrcAddr >> 16)
	case 0xFF26:
		return byte(b.DmaSrcAddr >> 8)
	case 0xFF27:
		return byte(b.DmaSrcAddr)
	case 0xFF28:
		return b.DmaDstTask
	case 0xFF29:
		return byte(b.DmaDstAddr >> 16)
	case 0xFF2A:
		return byte(b.DmaDstAddr >> 8)
	case 0xFF2B:
		return byte(b.DmaDstAddr)
	case 0xFF2C:
		return byte(b.DmaCount >> 24)
	case 0xFF2D:
		return byte(b.DmaCount >> 16)
	case 0xFF2E:
		return byte(b.DmaCount >> 8)
	case 0xFF2F:
		return byte(b.DmaCount)

	case 0xFF30:
		return byte(b.SharedMemoryCurtain >> 24)
	case 0xFF31:
		return byte(b.SharedMemoryCurtain >> 16)
	case 0xFF32:
		return byte(b.SharedMemoryCurtain >> 8)
	case 0xFF33:
		return byte(b.SharedMemoryCurtain)
	case 0xFF34:
		return b.TaskFlagsTarget
	case 0xFF35:
		return b.TaskFlags[b.TaskFlagsTarget]
	case 0xFF36:
		return 0

	case 0xFF60:
		return b.TrapSyscallNum
	case 0xFF61:
		return b.TrapSourceTask

	default:
		return 0
	}
}

func (b *Bus) writeIOLocked(addr uint16, val byte) {
	// Hardware vectors $FFF0..$FFFF are stored in Task 0 RAM
	if addr >= 0xFFF0 {
		b.Memory[0][addr] = val
		return
	}

	switch addr {
	case 0xFF00: // putchar
		if b.ConsoleOut != nil {
			if val == 10 || val == 13 {
				b.ConsoleOut.Write([]byte{'\n'})
			} else if b.CurlyEscape && !(val >= 32 && val <= 126) {
				fmt.Fprintf(b.ConsoleOut, "{%d}", val)
			} else {
				b.ConsoleOut.Write([]byte{val})
			}
		}
	case 0xFF01:
		// Read-only
	case 0xFF02: // RegStat (write 1 to clear bit)
		b.RegStat &^= val
		b.evalInterrupts()
	case 0xFF03: // RegCtrl
		b.RegCtrl = val
		b.evalInterrupts()
	case 0xFF04: // logchar
		if b.OnLogChar != nil {
			b.OnLogChar(val)
		}
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
	case 0xFF11: // Disk.CmdSt: 1=Read, 2=Write
		b.executeDiskCommandLocked(val)
	case 0xFF12, 0xFF13:
		// Reserved
	case 0xFF14:
		b.DiskSector = (b.DiskSector & 0x00FFFFFF) | (uint32(val) << 24)
	case 0xFF15:
		b.DiskSector = (b.DiskSector & 0xFF00FFFF) | (uint32(val) << 16)
	case 0xFF16:
		b.DiskSector = (b.DiskSector & 0xFFFF00FF) | (uint32(val) << 8)
	case 0xFF17:
		b.DiskSector = (b.DiskSector & 0xFFFFFF00) | uint32(val)
	case 0xFF18:
		b.DiskTargetTask = val
	case 0xFF19:
		b.DiskTargetAddr = (b.DiskTargetAddr & 0x00FFFF) | (uint32(val) << 16)
	case 0xFF1A:
		b.DiskTargetAddr = (b.DiskTargetAddr & 0xFF00FF) | (uint32(val) << 8)
	case 0xFF1B:
		b.DiskTargetAddr = (b.DiskTargetAddr & 0xFFFF00) | uint32(val)

	case 0xFF20:
		b.TaskFuseArmed = true
		b.TaskFuseTarget = val
	case 0xFF21: // DMA.CmdSt
		if val == 1 {
			b.executeDmaCopyLocked()
		}
	case 0xFF22, 0xFF23:
		// Reserved
	case 0xFF24:
		b.DmaSrcTask = val
	case 0xFF25:
		b.DmaSrcAddr = (b.DmaSrcAddr & 0x00FFFF) | (uint32(val) << 16)
	case 0xFF26:
		b.DmaSrcAddr = (b.DmaSrcAddr & 0xFF00FF) | (uint32(val) << 8)
	case 0xFF27:
		b.DmaSrcAddr = (b.DmaSrcAddr & 0xFFFF00) | uint32(val)
	case 0xFF28:
		b.DmaDstTask = val
	case 0xFF29:
		b.DmaDstAddr = (b.DmaDstAddr & 0x00FFFF) | (uint32(val) << 16)
	case 0xFF2A:
		b.DmaDstAddr = (b.DmaDstAddr & 0xFF00FF) | (uint32(val) << 8)
	case 0xFF2B:
		b.DmaDstAddr = (b.DmaDstAddr & 0xFFFF00) | uint32(val)
	case 0xFF2C:
		b.DmaCount = (b.DmaCount & 0x00FFFFFF) | (uint32(val) << 24)
	case 0xFF2D:
		b.DmaCount = (b.DmaCount & 0xFF00FFFF) | (uint32(val) << 16)
	case 0xFF2E:
		b.DmaCount = (b.DmaCount & 0xFFFF00FF) | (uint32(val) << 8)
	case 0xFF2F:
		b.DmaCount = (b.DmaCount & 0xFFFFFF00) | uint32(val)

	case 0xFF30:
		b.SharedMemoryCurtain = (b.SharedMemoryCurtain & 0x00FFFFFF) | (uint32(val) << 24)
	case 0xFF31:
		b.SharedMemoryCurtain = (b.SharedMemoryCurtain & 0xFF00FFFF) | (uint32(val) << 16)
	case 0xFF32:
		b.SharedMemoryCurtain = (b.SharedMemoryCurtain & 0xFFFF00FF) | (uint32(val) << 8)
	case 0xFF33:
		b.SharedMemoryCurtain = (b.SharedMemoryCurtain & 0xFFFFFF00) | uint32(val)

	case 0xFF34:
		b.TaskFlagsTarget = val
	case 0xFF35:
		b.TaskFlags[b.TaskFlagsTarget] = val
	case 0xFF36:
		if val > 0 {
			b.Memory[val] = [65536]byte{}
			b.TaskFlags[val] = 0
		}

	case 0xFF60:
		b.TrapSyscallNum = val
	case 0xFF61:
		b.TrapSourceTask = val
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

	task := b.DiskTargetTask
	addr := uint16(b.DiskTargetAddr & 0xFFFF)
	curtain := uint16(b.SharedMemoryCurtain & 0xFFFF)

	switch cmd {
	case 1: // Read Sector
		for i := 0; i < 256; i++ {
			tgtAddr := addr + uint16(i)
			if curtain > 0 && tgtAddr >= curtain && task <= 2 {
				b.Memory[0][tgtAddr] = disk[offset+int64(i)]
			} else {
				b.Memory[task][tgtAddr] = disk[offset+int64(i)]
			}
		}
		b.DiskStatus = 1 // OKAY
	case 2: // Write Sector
		for i := 0; i < 256; i++ {
			srcAddr := addr + uint16(i)
			if curtain > 0 && srcAddr >= curtain && task <= 2 {
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

func (b *Bus) executeDmaCopyLocked() {
	count := b.DmaCount
	if count == 0 {
		b.DmaStatus = 1
		return
	}
	srcTask := b.DmaSrcTask
	srcAddr := uint16(b.DmaSrcAddr & 0xFFFF)
	dstTask := b.DmaDstTask
	dstAddr := uint16(b.DmaDstAddr & 0xFFFF)
	curtain := uint16(b.SharedMemoryCurtain & 0xFFFF)

	for i := uint32(0); i < count; i++ {
		sAddr := srcAddr + uint16(i)
		dAddr := dstAddr + uint16(i)

		var val byte
		if curtain > 0 && sAddr >= curtain && srcTask <= 2 {
			val = b.Memory[0][sAddr]
		} else {
			val = b.Memory[srcTask][sAddr]
		}

		if curtain > 0 && dAddr >= curtain && dstTask <= 2 {
			b.Memory[0][dAddr] = val
		} else {
			b.Memory[dstTask][dAddr] = val
		}
	}
	b.DmaStatus = 1 // OKAY

	if os.Getenv("TRACE_DMA") != "" && ((dstTask == 1 && dstAddr == 0x0080) || (srcTask == 1 && srcAddr == 0x0080)) {
		mb := b.Memory[1][0x0080 : 0x0080+64]
		pathStr := string(bytes.TrimRight(mb[24:56], "\x00"))
		fmt.Fprintf(os.Stderr, "[DMA-MB] srcTask=%d srcAddr=0x%04X dstTask=%d dstAddr=0x%04X cmd=%d drv=%d st=%d path=%q\n",
			srcTask, srcAddr, dstTask, dstAddr, mb[0], mb[1], mb[3], pathStr)
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

// TimerTick signals a periodic timer tick.
func (b *Bus) TimerTick() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.RegStat |= 0x01 // Timer.Ready
	b.evalInterrupts()
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
