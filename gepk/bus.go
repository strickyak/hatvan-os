package gepk

import (
	"errors"
	"fmt"
	"io"
	"os"
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

	// Turbo9Sim / 68k Console & Timer registers ($FF0000..$FF0008)
	RegStat uint16 // $FF0004: Bit 0 = Timer.Ready, Bit 1 = Term.RxReady
	RegCtrl uint16 // $FF0006: Bit 0 = Ctrl.TimrIRQ, Bit 1 = Ctrl.TermIRQ

	// Console streams
	ConsoleIn  []byte
	ConsoleOut io.Writer
	LogOut     io.Writer
	StdinChan  <-chan byte

	// VM Termination
	ExitCode uint16
	Exited   bool

	// Hatvan Disk I/O ($FF0010..$FF001E)
	DiskDrive  uint16
	DiskSector uint32
	DiskTask   uint16
	DiskAddr   uint32
	DiskStatus uint16
	Disks      [4][]byte

	// Hatvan DMA Copy Engine ($FF0022..$FF0032)
	DmaSrcTask uint16
	DmaSrcAddr uint32
	DmaDstTask uint16
	DmaDstAddr uint32
	DmaCount   uint32
	DmaStatus  uint16

	// Hatvan TaskFlags ($00FF005C)
	TaskFlags [256]byte

	// Callback when highest active interrupt level changes (0 = none, 1..7)
	OnInterruptLevelChanged func(level uint8)
}

// NewBus constructs an initialized Bus with Task 0 allocated.
func NewBus() *Bus {
	b := &Bus{
		CurrentFC:  FCSupervisorProg,
		TaskReg:    1,
		ConsoleOut: os.Stdout,
		LogOut:     os.Stderr,
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
		return b.Tasks[b.TaskReg].readByte(addr)
	}

	// Supervisor Mode (FC2 == 1): targets Task 0
	if addr >= 0x00FF0000 {
		return b.readIOLocked(addr)
	}
	return b.Tasks[0].readByte(addr)
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
	// I/O devices are mapped on 16-bit word boundaries ($00FF0000..$00FF003F)
	base := addr & 0xFFFFFFFE
	isOdd := (addr & 1) != 0

	switch base {
	case 0x00FF0000: // Term.Out (Write-only, reading returns 0)
		return 0

	case 0x00FF0002: // Term.In
		if len(b.ConsoleIn) == 0 && b.StdinChan != nil {
			b.pollStdinInternal()
		}
		if len(b.ConsoleIn) > 0 {
			ch := b.ConsoleIn[0]
			b.ConsoleIn = b.ConsoleIn[1:]
			if len(b.ConsoleIn) == 0 {
				b.RegStat &^= 0x0002
			} else {
				b.RegStat |= 0x0002
			}
			b.evalInterrupts()
			return ch
		}
		if b.StdinChan == nil {
			return 0x04
		}
		return 0 // Non-blocking: 0 if no char ready

	case 0x00FF0004: // Reg.Stat
		if isOdd {
			return byte(b.RegStat)
		}
		return byte(b.RegStat >> 8)

	case 0x00FF0006: // Reg.Ctrl
		if isOdd {
			return byte(b.RegCtrl)
		}
		return byte(b.RegCtrl >> 8)

	case 0x00FF0008: // logchar
		return 0

	case 0x00FF000A: // Exit.Code
		if isOdd {
			return byte(b.ExitCode)
		}
		return byte(b.ExitCode >> 8)

	case 0x00FF0010: // Disk.Drive
		return byte(b.DiskDrive)

	case 0x00FF0014: // Disk.Sector (MSW)
		if isOdd {
			return byte(b.DiskSector >> 16)
		}
		return byte(b.DiskSector >> 24)
	case 0x00FF0016: // Disk.Sector (LSW)
		if isOdd {
			return byte(b.DiskSector)
		}
		return byte(b.DiskSector >> 8)

	case 0x00FF0018: // Disk.Task
		return byte(b.DiskTask)

	case 0x00FF001A: // Disk.Addr (MSW)
		if isOdd {
			return byte(b.DiskAddr >> 16)
		}
		return byte(b.DiskAddr >> 24)
	case 0x00FF001C: // Disk.Addr (LSW)
		if isOdd {
			return byte(b.DiskAddr)
		}
		return byte(b.DiskAddr >> 8)

	case 0x00FF001E: // Disk.CmdSt
		if isOdd {
			st := b.DiskStatus
			b.DiskStatus = 0
			return byte(st)
		}
		return byte(b.DiskStatus >> 8)

	case 0x00FF0020: // Task.Active
		return byte(b.TaskReg)

	case 0x00FF0022: // DMA.SrcTask
		return byte(b.DmaSrcTask)

	case 0x00FF0024: // DMA.SrcAddr (MSW)
		if isOdd {
			return byte(b.DmaSrcAddr >> 16)
		}
		return byte(b.DmaSrcAddr >> 24)
	case 0x00FF0026: // DMA.SrcAddr (LSW)
		if isOdd {
			return byte(b.DmaSrcAddr)
		}
		return byte(b.DmaSrcAddr >> 8)

	case 0x00FF0028: // DMA.DstTask
		return byte(b.DmaDstTask)

	case 0x00FF002A: // DMA.DstAddr (MSW)
		if isOdd {
			return byte(b.DmaDstAddr >> 16)
		}
		return byte(b.DmaDstAddr >> 24)
	case 0x00FF002C: // DMA.DstAddr (LSW)
		if isOdd {
			return byte(b.DmaDstAddr)
		}
		return byte(b.DmaDstAddr >> 8)

	case 0x00FF002E: // DMA.Count (MSW)
		if isOdd {
			return byte(b.DmaCount >> 16)
		}
		return byte(b.DmaCount >> 24)
	case 0x00FF0030: // DMA.Count (LSW)
		if isOdd {
			return byte(b.DmaCount)
		}
		return byte(b.DmaCount >> 8)

	case 0x00FF0032: // DMA.CmdSt
		if isOdd {
			st := b.DmaStatus
			b.DmaStatus = 0
			return byte(st)
		}
		return byte(b.DmaStatus >> 8)

	case 0x00FF005C: // TaskFlagsRegister (0x00FF005C or 0x00FF005D)
		return b.TaskFlags[1]

	case 0x00FF005E: // PurgeTaskMem (0x00FF005E or 0x00FF005F)
		return 0

	default:
		panic(fmt.Errorf("%w: read at unmapped port 0x%06X in Task 0", ErrKernelIOPanic, addr))
	}
}

func (b *Bus) writeIOLocked(addr uint32, val byte) {
	base := addr & 0xFFFFFFFE
	isOdd := (addr & 1) != 0

	switch base {
	case 0x00FF0000: // Term.Out
		if b.ConsoleOut != nil {
			b.ConsoleOut.Write([]byte{val})
		}

	case 0x00FF0002: // Term.In (read only)
		// Ignored

	case 0x00FF0004: // Reg.Stat (write 1 to clear)
		if isOdd {
			b.RegStat &^= uint16(val) & 0x0003
		} else {
			b.RegStat &^= (uint16(val) << 8) & 0x0003
		}
		if len(b.ConsoleIn) > 0 {
			b.RegStat |= 0x0002
		}
		b.evalInterrupts()

	case 0x00FF0006: // Reg.Ctrl
		if isOdd {
			b.RegCtrl = (b.RegCtrl & 0xFF00) | uint16(val)
		} else {
			b.RegCtrl = (b.RegCtrl & 0x00FF) | (uint16(val) << 8)
		}
		b.evalInterrupts()

	case 0x00FF0008: // logchar
		if b.LogOut != nil {
			b.LogOut.Write([]byte{val})
		}

	case 0x00FF000A: // Exit.Code
		if isOdd {
			b.ExitCode = (b.ExitCode & 0xFF00) | uint16(val)
		} else {
			b.ExitCode = (b.ExitCode & 0x00FF) | (uint16(val) << 8)
		}
		b.Exited = true

	case 0x00FF0010: // Disk.Drive
		b.DiskDrive = uint16(val & 0x03)

	case 0x00FF0014: // Disk.Sector (MSW)
		if isOdd {
			b.DiskSector = (b.DiskSector & 0xFF00FFFF) | (uint32(val) << 16)
		} else {
			b.DiskSector = (b.DiskSector & 0x00FFFFFF) | (uint32(val) << 24)
		}
	case 0x00FF0016: // Disk.Sector (LSW)
		if isOdd {
			b.DiskSector = (b.DiskSector & 0xFFFFFF00) | uint32(val)
		} else {
			b.DiskSector = (b.DiskSector & 0xFFFF00FF) | (uint32(val) << 8)
		}

	case 0x00FF0018: // Disk.Task
		b.DiskTask = uint16(val)

	case 0x00FF001A: // Disk.Addr (MSW)
		if isOdd {
			b.DiskAddr = (b.DiskAddr & 0xFF00FFFF) | (uint32(val) << 16)
		} else {
			b.DiskAddr = (b.DiskAddr & 0x00FFFFFF) | (uint32(val) << 24)
		}
	case 0x00FF001C: // Disk.Addr (LSW)
		if isOdd {
			b.DiskAddr = (b.DiskAddr & 0xFFFFFF00) | uint32(val)
		} else {
			b.DiskAddr = (b.DiskAddr & 0xFFFF00FF) | (uint32(val) << 8)
		}

	case 0x00FF001E: // Disk.CmdSt
		if val != 0 {
			b.executeDiskCommand(val)
		}

	case 0x00FF0020: // Task.Active
		b.TaskReg = val

	case 0x00FF0022: // DMA.SrcTask
		b.DmaSrcTask = uint16(val)

	case 0x00FF0024: // DMA.SrcAddr (MSW)
		if isOdd {
			b.DmaSrcAddr = (b.DmaSrcAddr & 0xFF00FFFF) | (uint32(val) << 16)
		} else {
			b.DmaSrcAddr = (b.DmaSrcAddr & 0x00FFFFFF) | (uint32(val) << 24)
		}
	case 0x00FF0026: // DMA.SrcAddr (LSW)
		if isOdd {
			b.DmaSrcAddr = (b.DmaSrcAddr & 0xFFFFFF00) | uint32(val)
		} else {
			b.DmaSrcAddr = (b.DmaSrcAddr & 0xFFFF00FF) | (uint32(val) << 8)
		}

	case 0x00FF0028: // DMA.DstTask
		b.DmaDstTask = uint16(val)

	case 0x00FF002A: // DMA.DstAddr (MSW)
		if isOdd {
			b.DmaDstAddr = (b.DmaDstAddr & 0xFF00FFFF) | (uint32(val) << 16)
		} else {
			b.DmaDstAddr = (b.DmaDstAddr & 0x00FFFFFF) | (uint32(val) << 24)
		}
	case 0x00FF002C: // DMA.DstAddr (LSW)
		if isOdd {
			b.DmaDstAddr = (b.DmaDstAddr & 0xFFFFFF00) | uint32(val)
		} else {
			b.DmaDstAddr = (b.DmaDstAddr & 0xFFFF00FF) | (uint32(val) << 8)
		}

	case 0x00FF002E: // DMA.Count (MSW)
		if isOdd {
			b.DmaCount = (b.DmaCount & 0xFF00FFFF) | (uint32(val) << 16)
		} else {
			b.DmaCount = (b.DmaCount & 0x00FFFFFF) | (uint32(val) << 24)
		}
	case 0x00FF0030: // DMA.Count (LSW)
		if isOdd {
			b.DmaCount = (b.DmaCount & 0xFFFFFF00) | uint32(val)
		} else {
			b.DmaCount = (b.DmaCount & 0xFFFF00FF) | (uint32(val) << 8)
		}

	case 0x00FF0032: // DMA.CmdSt
		if val != 0 {
			b.executeDMACopy()
		}

	case 0x00FF005C: // TaskFlagsRegister (0x00FF005C or 0x00FF005D): sets flags for Task 1 (Bit 0 = allow I/O)
		b.TaskFlags[1] = val

	case 0x00FF005E: // PurgeTaskMem (0x00FF005E or 0x00FF005F)
		if val != 0 {
			b.purgeTaskMem(val)
		}

	default:
		panic(fmt.Errorf("%w: write at unmapped port 0x%06X in Task 0", ErrKernelIOPanic, addr))
	}
}

func (b *Bus) evalInterrupts() {
	var level uint8 = 0

	// Level 6: 60Hz Timer Tick
	if (b.RegStat & b.RegCtrl & 0x0001) != 0 {
		level = 6
	} else if (b.RegStat & b.RegCtrl & 0x0002) != 0 {
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

	b.RegStat |= 0x0001 // Timer.Ready
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

	task := uint8(b.DiskTask)
	addr := b.DiskAddr

	switch cmd {
	case 1: // Read sector from disk to task memory
		for i := 0; i < sectorSize; i++ {
			b.Tasks[task].writeByte(addr+uint32(i), disk[offset+i])
		}
		b.DiskStatus = 1 // OKAY

	case 2: // Write sector from task memory to disk
		for i := 0; i < sectorSize; i++ {
			disk[offset+i] = b.Tasks[task].readByte(addr+uint32(i))
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

	for i := 0; i < count; i++ {
		val := b.Tasks[srcTask].readByte(srcAddr + uint32(i))
		b.Tasks[dstTask].writeByte(dstAddr+uint32(i), val)
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

