package gepk

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrAddressError   = errors.New("unaligned word/long access (Address Error, Vector 3)")
	ErrBusError       = errors.New("access fault to protected or unmapped address (Bus Error, Vector 2)")
	ErrUserAccessTrap = errors.New("user process attempted access to supervisor page $FF0000..$FFFFFF")
	ErrKernelIOPanic  = errors.New("kernel accessed unmapped I/O port in $FF0000..$FFFFFF")
)

// Function Codes representing the 68000 bus cycle classification
const (
	FCUserData       = 1 // 001: User Data
	FCUserProgram    = 2 // 010: User Program
	FCSupervisorData = 5 // 101: Supervisor Data
	FCSupervisorProg = 6 // 110: Supervisor Program
	FCCPUSpace       = 7 // 111: CPU Space (Interrupt Acknowledge / Breakpoint)
)

// TaskMemory manages the 16MB address space for a single task using sparse 64KB pages.
type TaskMemory struct {
	Pages [256][]byte
}

func (tm *TaskMemory) readByte(addr uint32) byte {
	p := addr >> 16
	if tm.Pages[p] == nil {
		return 0
	}
	return tm.Pages[p][addr&0xFFFF]
}

func (tm *TaskMemory) writeByte(addr uint32, val byte) {
	p := addr >> 16
	if tm.Pages[p] == nil {
		tm.Pages[p] = make([]byte, 65536)
	}
	tm.Pages[p][addr&0xFFFF] = val
}

// Bus manages memory across all 256 tasks, memory-mapped I/O, and hardware devices.
type Bus struct {
	mu sync.Mutex

	// Tasks[0..255]
	Tasks [256]*TaskMemory

	// Current Function Code output by CPU
	CurrentFC uint8

	// Active User Task selected by $00FF0020
	TaskReg uint8

	// Turbo9Sim / 68k Console & Timer registers ($00FF0000..$00FF0005)
	RegStat byte // $00FF0002: Bit 0 = Timer.Ready, Bit 1 = Term.RxReady
	RegCtrl byte // $00FF0003: Bit 0 = Ctrl.TimrIRQ, Bit 1 = Ctrl.TermIRQ

	// Console streams
	ConsoleIn   []byte
	ConsoleOut  io.Writer
	LogOut      io.Writer
	StdinChan   <-chan byte
	CurlyEscape bool

	// VM Termination
	ExitCode byte // $00FF0005
	Exited   bool

	// Hatvan Disk I/O ($00FF0010..$00FF001B)
	DiskDrive      byte
	DiskStatus     byte   // $00FF0011: Disk.CmdSt
	DiskSector     uint32 // $00FF0014..$00FF0017: 32-bit LSN
	DiskTargetTask byte   // $00FF0018
	DiskTargetAddr uint32 // $00FF0019..$00FF001B: 24-bit
	Disks          [4][]byte

	// Hatvan DMA Copy Engine ($00FF0021..$00FF002F)
	DmaStatus  byte   // $00FF0021: DMA.CmdSt
	DmaSrcTask byte   // $00FF0024
	DmaSrcAddr uint32 // $00FF0025..$00FF0027: 24-bit
	DmaDstTask byte   // $00FF0028
	DmaDstAddr uint32 // $00FF0029..$00FF002B: 24-bit
	DmaCount   uint32 // $00FF002C..$00FF002F: 32-bit

	// Hatvan TaskFlags ($00FF0034..$00FF0036)
	TaskFlagsTarget byte
	TaskFlags       [256]byte

	// Tunable shared memory curtain ($00FE0000 by default) for Tasks 0, 1, 2
	SharedMemoryCurtain uint32

	// Callback when highest active interrupt level changes (0 = none, 1..7)
	OnInterruptLevelChanged func(level uint8)
}

// NewBus constructs an initialized Bus with Task 0 allocated.
func NewBus() *Bus {
	curtain := uint32(0x00FE0000)
	if s := os.Getenv("HATVAN_SHARED_CURTAIN_68K"); s != "" {
		if v, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "$"), 16, 32); err == nil {
			curtain = uint32(v)
		}
	} else if s := os.Getenv("HATVAN_SHARED_CURTAIN"); s != "" {
		if v, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "$"), 16, 32); err == nil {
			curtain = uint32(v)
		}
	}
	b := &Bus{
		CurrentFC:           FCSupervisorProg,
		TaskReg:             1,
		TaskFlagsTarget:     1,
		SharedMemoryCurtain: curtain,
		ConsoleOut:          os.Stdout,
		LogOut:              os.Stderr,
	}
	for i := range b.Tasks {
		b.Tasks[i] = &TaskMemory{}
	}
	return b
}

// ReadByte reads an 8-bit byte at addr using CurrentFC privilege.
func (b *Bus) ReadByte(addr uint32) byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.readByteLocked(addr)
}

// WriteByte writes an 8-bit byte at addr using CurrentFC privilege.
func (b *Bus) WriteByte(addr uint32, val byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.writeByteLocked(addr, val)
}

// ReadWord reads a big-endian 16-bit word. Unaligned accesses trigger ErrAddressError.
func (b *Bus) ReadWord(addr uint32) uint16 {
	if (addr & 1) != 0 {
		panic(fmt.Errorf("%w: read word at 0x%08X", ErrAddressError, addr))
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	hi := uint16(b.readByteLocked(addr))
	lo := uint16(b.readByteLocked(addr + 1))
	return (hi << 8) | lo
}

// WriteWord writes a big-endian 16-bit word. Unaligned accesses trigger ErrAddressError.
func (b *Bus) WriteWord(addr uint32, val uint16) {
	if (addr & 1) != 0 {
		panic(fmt.Errorf("%w: write word at 0x%08X", ErrAddressError, addr))
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	b.writeByteLocked(addr, byte(val>>8))
	b.writeByteLocked(addr+1, byte(val))
}

// ReadLong reads a big-endian 32-bit longword. Unaligned accesses trigger ErrAddressError.
func (b *Bus) ReadLong(addr uint32) uint32 {
	if (addr & 1) != 0 {
		panic(fmt.Errorf("%w: read long at 0x%08X", ErrAddressError, addr))
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	b0 := uint32(b.readByteLocked(addr))
	b1 := uint32(b.readByteLocked(addr + 1))
	b2 := uint32(b.readByteLocked(addr + 2))
	b3 := uint32(b.readByteLocked(addr + 3))
	return (b0 << 24) | (b1 << 16) | (b2 << 8) | b3
}

// WriteLong writes a big-endian 32-bit longword. Unaligned accesses trigger ErrAddressError.
func (b *Bus) WriteLong(addr uint32, val uint32) {
	if (addr & 1) != 0 {
		panic(fmt.Errorf("%w: write long at 0x%08X", ErrAddressError, addr))
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	b.writeByteLocked(addr, byte(val>>24))
	b.writeByteLocked(addr+1, byte(val>>16))
	b.writeByteLocked(addr+2, byte(val>>8))
	b.writeByteLocked(addr+3, byte(val))
}

func (b *Bus) readByteLocked(addr uint32) byte {
	addr &= 0x00FFFFFF // 24-bit physical address space

	// User Mode (FC2 == 0)
	if (b.CurrentFC & 0x04) == 0 {
		if addr >= 0x00FF0000 {
			if (b.TaskFlags[b.TaskReg] & 0x01) != 0 {
				return b.readIOLocked(addr)
			}
			panic(fmt.Errorf("%w: user read at 0x%06X in task %d", ErrUserAccessTrap, addr, b.TaskReg))
		}
		if b.SharedMemoryCurtain > 0 && addr >= b.SharedMemoryCurtain && b.TaskReg <= 2 {
			return b.Tasks[0].readByte(addr)
		}
		return b.Tasks[b.TaskReg].readByte(addr)
	}

	// Supervisor Mode (FC2 == 1): targets Task 0
	if addr >= 0x00FF0000 {
		return b.readIOLocked(addr)
	}
	return b.Tasks[0].readByte(addr)
}

// ReadUserByte safely reads a byte from a user task's memory space.
func (b *Bus) ReadUserByte(task uint8, addr uint32) byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	addr &= 0x00FFFFFF
	if addr >= 0x00FF0000 {
		return 0
	}
	if b.SharedMemoryCurtain > 0 && addr >= b.SharedMemoryCurtain && task <= 2 {
		return b.Tasks[0].readByte(addr)
	}
	return b.Tasks[task].readByte(addr)
}

func (b *Bus) writeByteLocked(addr uint32, val byte) {
	addr &= 0x00FFFFFF // 24-bit physical address space

	// User Mode (FC2 == 0)
	if (b.CurrentFC & 0x04) == 0 {
		if addr >= 0x00FF0000 {
			if (b.TaskFlags[b.TaskReg] & 0x01) != 0 {
				b.writeIOLocked(addr, val)
				return
			}
			panic(fmt.Errorf("%w: user write at 0x%06X in task %d", ErrUserAccessTrap, addr, b.TaskReg))
		}
		if b.SharedMemoryCurtain > 0 && addr >= b.SharedMemoryCurtain && b.TaskReg <= 2 {
			b.Tasks[0].writeByte(addr, val)
			return
		}
		b.Tasks[b.TaskReg].writeByte(addr, val)
		return
	}

	// Supervisor Mode (FC2 == 1): targets Task 0
	if addr >= 0x00FF0000 {
		b.writeIOLocked(addr, val)
		return
	}
	b.Tasks[0].writeByte(addr, val)
}

func (b *Bus) readIOLocked(addr uint32) byte {
	offset := addr & 0xFFFF

	switch offset {
	case 0x0000: // Term.Out
		return 0

	case 0x0001: // Term.In
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
			return 0x04
		}
		return 0 // Non-blocking: 0 if no char ready

	case 0x0002: // Reg.Stat
		return b.RegStat

	case 0x0003: // Reg.Ctrl
		return b.RegCtrl

	case 0x0004: // logchar
		return 0

	case 0x0005: // Exit.Code
		return b.ExitCode

	case 0x0010: // Disk.Drive
		return b.DiskDrive

	case 0x0011: // Disk.CmdSt
		st := b.DiskStatus
		b.DiskStatus = 0
		return st

	case 0x0012, 0x0013:
		return 0 // Reserved

	case 0x0014: // Disk.Sector (32-bit Big-Endian LSN)
		return byte(b.DiskSector >> 24)
	case 0x0015:
		return byte(b.DiskSector >> 16)
	case 0x0016:
		return byte(b.DiskSector >> 8)
	case 0x0017:
		return byte(b.DiskSector)

	case 0x0018: // Disk.Target ([Task:8][Addr:24])
		return b.DiskTargetTask
	case 0x0019:
		return byte(b.DiskTargetAddr >> 16)
	case 0x001A:
		return byte(b.DiskTargetAddr >> 8)
	case 0x001B:
		return byte(b.DiskTargetAddr)

	case 0x0020: // Task.Active (TaskReg)
		return b.TaskReg

	case 0x0021: // DMA.CmdSt
		st := b.DmaStatus
		b.DmaStatus = 0
		return st

	case 0x0022, 0x0023:
		return 0 // Reserved

	case 0x0024: // DMA.Src ([SrcTask:8][SrcAddr:24])
		return b.DmaSrcTask
	case 0x0025:
		return byte(b.DmaSrcAddr >> 16)
	case 0x0026:
		return byte(b.DmaSrcAddr >> 8)
	case 0x0027:
		return byte(b.DmaSrcAddr)

	case 0x0028: // DMA.Dst ([DstTask:8][DstAddr:24])
		return b.DmaDstTask
	case 0x0029:
		return byte(b.DmaDstAddr >> 16)
	case 0x002A:
		return byte(b.DmaDstAddr >> 8)
	case 0x002B:
		return byte(b.DmaDstAddr)

	case 0x002C: // DMA.Count (32-bit Big-Endian)
		return byte(b.DmaCount >> 24)
	case 0x002D:
		return byte(b.DmaCount >> 16)
	case 0x002E:
		return byte(b.DmaCount >> 8)
	case 0x002F:
		return byte(b.DmaCount)

	case 0x0030: // SharedMemoryCurtain (32-bit Big-Endian)
		return byte(b.SharedMemoryCurtain >> 24)
	case 0x0031:
		return byte(b.SharedMemoryCurtain >> 16)
	case 0x0032:
		return byte(b.SharedMemoryCurtain >> 8)
	case 0x0033:
		return byte(b.SharedMemoryCurtain)

	case 0x0034: // TaskFlagsTarget
		return b.TaskFlagsTarget

	case 0x0035: // TaskFlagsRegister
		return b.TaskFlags[b.TaskFlagsTarget]

	case 0x0036: // PurgeTaskMem
		return 0

	case 0x0040, 0x0041, 0x0042, 0x0043, 0x0044, 0x0045, 0x0046, 0x0047:
		return 0 // EMBIGGEN MMAP stub on 68k

	default:
		panic(fmt.Errorf("%w: read at unmapped port 0x%06X in Task 0", ErrKernelIOPanic, addr))
	}
}

func (b *Bus) writeIOLocked(addr uint32, val byte) {
	offset := addr & 0xFFFF

	switch offset {
	case 0x0000: // Term.Out
		if b.ConsoleOut != nil {
			if val == 10 || val == 13 {
				b.ConsoleOut.Write([]byte{'\n'})
			} else if b.CurlyEscape && !(val >= 32 && val <= 126) {
				fmt.Fprintf(b.ConsoleOut, "{%d}", val)
			} else {
				b.ConsoleOut.Write([]byte{val})
			}
		}

	case 0x0001: // Term.In (read only)
		// Ignored

	case 0x0002: // Reg.Stat (write 1 to clear)
		b.RegStat &^= (val & 0x03)
		if len(b.ConsoleIn) > 0 {
			b.RegStat |= 0x02
		}
		b.evalInterrupts()

	case 0x0003: // Reg.Ctrl
		b.RegCtrl = val & 0x03
		b.evalInterrupts()

	case 0x0004: // logchar
		if b.LogOut != nil {
			b.LogOut.Write([]byte{val})
		}

	case 0x0005: // Exit.Code
		b.ExitCode = val
		b.Exited = true

	case 0x0010: // Disk.Drive
		b.DiskDrive = val & 0x03

	case 0x0011: // Disk.CmdSt
		if val != 0 {
			b.executeDiskCommand(val)
		}

	case 0x0012, 0x0013:
		// Reserved

	case 0x0014:
		b.DiskSector = (b.DiskSector & 0x00FFFFFF) | (uint32(val) << 24)
	case 0x0015:
		b.DiskSector = (b.DiskSector & 0xFF00FFFF) | (uint32(val) << 16)
	case 0x0016:
		b.DiskSector = (b.DiskSector & 0xFFFF00FF) | (uint32(val) << 8)
	case 0x0017:
		b.DiskSector = (b.DiskSector & 0xFFFFFF00) | uint32(val)

	case 0x0018:
		b.DiskTargetTask = val
	case 0x0019:
		b.DiskTargetAddr = (b.DiskTargetAddr & 0x00FFFF) | (uint32(val) << 16)
	case 0x001A:
		b.DiskTargetAddr = (b.DiskTargetAddr & 0xFF00FF) | (uint32(val) << 8)
	case 0x001B:
		b.DiskTargetAddr = (b.DiskTargetAddr & 0xFFFF00) | uint32(val)

	case 0x0020: // Task.Active (TaskReg)
		b.TaskReg = val

	case 0x0021: // DMA.CmdSt
		if val == 1 {
			b.executeDMACopy()
		}

	case 0x0022, 0x0023:
		// Reserved

	case 0x0024:
		b.DmaSrcTask = val
	case 0x0025:
		b.DmaSrcAddr = (b.DmaSrcAddr & 0x00FFFF) | (uint32(val) << 16)
	case 0x0026:
		b.DmaSrcAddr = (b.DmaSrcAddr & 0xFF00FF) | (uint32(val) << 8)
	case 0x0027:
		b.DmaSrcAddr = (b.DmaSrcAddr & 0xFFFF00) | uint32(val)

	case 0x0028:
		b.DmaDstTask = val
	case 0x0029:
		b.DmaDstAddr = (b.DmaDstAddr & 0x00FFFF) | (uint32(val) << 16)
	case 0x002A:
		b.DmaDstAddr = (b.DmaDstAddr & 0xFF00FF) | (uint32(val) << 8)
	case 0x002B:
		b.DmaDstAddr = (b.DmaDstAddr & 0xFFFF00) | uint32(val)

	case 0x002C:
		b.DmaCount = (b.DmaCount & 0x00FFFFFF) | (uint32(val) << 24)
	case 0x002D:
		b.DmaCount = (b.DmaCount & 0xFF00FFFF) | (uint32(val) << 16)
	case 0x002E:
		b.DmaCount = (b.DmaCount & 0xFFFF00FF) | (uint32(val) << 8)
	case 0x002F:
		b.DmaCount = (b.DmaCount & 0xFFFFFF00) | uint32(val)

	case 0x0030:
		b.SharedMemoryCurtain = (b.SharedMemoryCurtain & 0x00FFFFFF) | (uint32(val) << 24)
	case 0x0031:
		b.SharedMemoryCurtain = (b.SharedMemoryCurtain & 0xFF00FFFF) | (uint32(val) << 16)
	case 0x0032:
		b.SharedMemoryCurtain = (b.SharedMemoryCurtain & 0xFFFF00FF) | (uint32(val) << 8)
	case 0x0033:
		b.SharedMemoryCurtain = (b.SharedMemoryCurtain & 0xFFFFFF00) | uint32(val)

	case 0x0034:
		b.TaskFlagsTarget = val
	case 0x0035:
		b.TaskFlags[b.TaskFlagsTarget] = val
	case 0x0036:
		if val != 0 {
			b.purgeTaskMem(val)
		}

	case 0x0040, 0x0041, 0x0042, 0x0043, 0x0044, 0x0045, 0x0046, 0x0047:
		// EMBIGGEN MMAP stub on 68k

	default:
		panic(fmt.Errorf("%w: write at unmapped port 0x%06X in Task 0", ErrKernelIOPanic, addr))
	}
}

func (b *Bus) evalInterrupts() {
	var level uint8 = 0

	// Level 6: 60Hz Timer Tick
	if (b.RegStat & b.RegCtrl & 0x01) != 0 {
		level = 6
	} else if (b.RegStat & b.RegCtrl & 0x02) != 0 {
		// Level 4: Console Input (Term.RxReady)
		level = 4
	}

	if b.OnInterruptLevelChanged != nil {
		b.OnInterruptLevelChanged(level)
	}
}

// TimerTick signals a 60Hz clock interrupt (Level 6).
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
	b.RegStat |= 0x0002 // Term.RxReady
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
		b.RegStat |= 0x0002
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
				b.RegStat |= 0x0002
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
				b.RegStat |= 0x0002
				b.evalInterrupts()
			}
			return
		}
	}
}

func (b *Bus) executeDiskCommand(cmd byte) {
	drv := int(b.DiskDrive)
	if drv >= len(b.Disks) || b.Disks[drv] == nil {
		b.DiskStatus = 2 // Drive not ready
		return
	}
	disk := b.Disks[drv]
	sectorSize := 256
	offset := int(b.DiskSector) * sectorSize
	if offset+sectorSize > len(disk) {
		b.DiskStatus = 3 // Sector out of range
		return
	}

	task := uint8(b.DiskTargetTask)
	addr := b.DiskTargetAddr

	switch cmd {
	case 1: // Read sector from disk to task memory
		for i := 0; i < sectorSize; i++ {
			b.Tasks[task].writeByte(addr+uint32(i), disk[offset+i])
		}
		b.DiskStatus = 1 // OKAY

	case 2: // Write sector from task memory to disk
		for i := 0; i < sectorSize; i++ {
			disk[offset+i] = b.Tasks[task].readByte(addr + uint32(i))
		}
		b.DiskStatus = 1 // OKAY

	default:
		b.DiskStatus = 4 // Unknown command
	}
}

func (b *Bus) executeDMACopy() {
	count := int(b.DmaCount)
	if count <= 0 {
		b.DmaStatus = 1
		return
	}

	srcTask := uint8(b.DmaSrcTask)
	dstTask := uint8(b.DmaDstTask)
	srcAddr := b.DmaSrcAddr
	dstAddr := b.DmaDstAddr

	readTaskByte := func(task uint8, addr uint32) byte {
		if b.SharedMemoryCurtain > 0 && addr >= b.SharedMemoryCurtain && addr < 0x00FF0000 && task <= 2 {
			return b.Tasks[0].readByte(addr)
		}
		return b.Tasks[task].readByte(addr)
	}
	writeTaskByte := func(task uint8, addr uint32, val byte) {
		if b.SharedMemoryCurtain > 0 && addr >= b.SharedMemoryCurtain && addr < 0x00FF0000 && task <= 2 {
			b.Tasks[0].writeByte(addr, val)
			return
		}
		b.Tasks[task].writeByte(addr, val)
	}

	for i := 0; i < count; i++ {
		val := readTaskByte(srcTask, srcAddr+uint32(i))
		writeTaskByte(dstTask, dstAddr+uint32(i), val)
	}

	b.DmaStatus = 1 // OKAY
}

func (b *Bus) purgeTaskMem(task uint8) {
	if task == 0 {
		return // Never purge Task 0 (kernel)
	}
	b.TaskFlags[task] = 0
	tm := b.Tasks[task]
	if tm == nil {
		return
	}
	for i := range tm.Pages {
		tm.Pages[i] = nil
	}
}

// LoadRawImage loads a binary memory image directly into Task 0 starting at baseAddr.
func (b *Bus) LoadRawImage(baseAddr uint32, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, v := range data {
		b.Tasks[0].writeByte(baseAddr+uint32(i), v)
	}
	return nil
}

// LoadS37 parses and loads Motorola S-Records (S0, S1, S2, S3, S7, S8, S9).
func (b *Bus) LoadS37(r io.Reader) (entryAddr uint32, hasEntry bool, err error) {
	return b.LoadSRecords(r)
}
