package gep9

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type EngineType string

const (
	EngineHatvan       EngineType = "hatvan"
	EngineFlat65280v2 EngineType = "flat65280v2"
	EngineDeep65280v2 EngineType = "deep65280v2"
)

var (
	ErrUserAccessTrap = errors.New("user process attempted access to protected page $FF00..$FFFF")
	ErrKernelIOPanic  = errors.New("kernel accessed unmapped port in page $FF00..$FFFF")
	ErrFlatStrayRead  = errors.New("flat65280v2: stray read in $FF00..$FFEE")
	ErrFlatStrayWrite = errors.New("flat65280v2: stray write in $FF00..$FFFF")
	ErrFlatClockCrash = errors.New("flat65280v2: crash and core dump requested via CLOCK_AND_STOP ($FE)")
	ErrDeepStrayRead  = errors.New("deep65280v2: stray read in $FF00..$FFFF")
	ErrDeepStrayWrite = errors.New("deep65280v2: stray write in $FF00..$FFFF")
)

// Bus manages memory across all 256 tasks, page protection, and I/O devices.
type Bus struct {
	mu sync.Mutex

	Engine EngineType

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

	// Hatvan TaskFlags ($FF2D..$FF2E)
	TaskFlagsTarget byte // $FF2D: target task to configure (default: 1)
	TaskFlags       [256]byte

	// Tunable shared memory curtain ($E000 by default) for Tasks 0, 1, 2
	SharedMemoryCurtain uint16

	// flat65280v2 EMUDSK registers ($FF80..$FF86)
	EmuDskLSN    uint32 // $FF80..$FF82: 24-bit LSN
	EmuDskStatus byte   // $FF83: Command / Status
	EmuDskBuffer uint16 // $FF84..$FF85: 16-bit RAM buffer pointer
	EmuDskDrive  byte   // $FF86: Drive unit selection (0 or 1)

	// flat65280v2 CLOCK_AND_STOP register ($FF87)
	ClockAndStop byte // $FF87: Bit 0 = Tick, Bit 1 = IRQ En, Bits 4..5 = Rate, $FC=exit(0), $FD=exit(1), $FE=crash

	// flat65280v2 ACIA M6850 registers ($FF88..$FF89)
	AciaCtrl byte // $FF88: Control register (Write)

	// deep65280v2 512KB physical RAM and MMU
	PhysRam [524288]byte // 64 8KB blocks (pages 0..63)
	MmuRegs [2][8]byte   // Task 0 and Task 1 DAT registers ($FFA0..$FFAF)
	MmuTask byte         // Active MMU Task (bit 0: $FF91)

	// Callback when IRQ line state changes
	OnIRQChanged func(asserted bool)

	// Exit / Halt ($FF05, $FF87)
	ExitCode int
	OnHalt   func(exitCode int)
	OnCrash  func()
}

// NewBus constructs an initialized Bus with all memory zeroed.
func NewBus() *Bus {
	curtain := uint16(0xE000)
	if s := os.Getenv("HATVAN_SHARED_CURTAIN"); s != "" {
		if v, err := strconv.ParseUint(strings.TrimPrefix(s, "0x"), 16, 16); err == nil {
			curtain = uint16(v)
		}
	}
	b := &Bus{
		Engine:              EngineHatvan,
		CurrentTask:         0,
		TaskFlagsTarget:     1,
		SharedMemoryCurtain: curtain,
		ConsoleOut:          os.Stdout,
		LogOut:              os.Stderr,
	}
	for i := 0; i < 8; i++ {
		b.MmuRegs[0][i] = byte(0x38 + i)
		b.MmuRegs[1][i] = byte(0x38 + i)
	}
	return b
}

// ReadByte reads an 8-bit value from the current task's memory space.
func (b *Bus) ReadByte(addr uint16) byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.Engine == EngineDeep65280v2 {
		if addr >= 0xFF00 {
			return b.readIODeep(addr)
		}
		if addr >= 0xFE00 {
			// Locked $FExx page: always physical block $3F offset $1Exx
			return b.PhysRam[0x70000+uint32(addr)]
		}
		slot := addr >> 13
		page := b.MmuRegs[b.MmuTask&1][slot] & 0x3F
		physAddr := (uint32(page) << 13) | uint32(addr&0x1FFF)
		return b.PhysRam[physAddr]
	}

	if b.Engine == EngineFlat65280v2 {
		if addr >= 0xFF00 {
			return b.readIOFlat(addr)
		}
		return b.Memory[0][addr]
	}

	// In user tasks (Task > 0), access to $FF00..$FFFF is strictly forbidden unless blessed with I/O flag
	if addr >= 0xFF00 {
		if b.CurrentTask == 0 || (b.TaskFlags[b.CurrentTask]&0x01) != 0 {
			return b.readIO(addr)
		}
		panic(fmt.Errorf("%w: read at 0x%04X in task %d", ErrUserAccessTrap, addr, b.CurrentTask))
	}

	// Shared Memory Curtain for Tasks 0, 1, and 2
	if b.SharedMemoryCurtain > 0 && addr >= b.SharedMemoryCurtain && b.CurrentTask <= 2 {
		return b.Memory[0][addr]
	}

	return b.Memory[b.CurrentTask][addr]
}

// WriteByte writes an 8-bit value to the current task's memory space.
func (b *Bus) WriteByte(addr uint16, val byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.Engine == EngineDeep65280v2 {
		if addr >= 0xFF00 {
			b.writeIODeep(addr, val)
			return
		}
		if addr >= 0xFE00 {
			// Locked $FExx page: always physical block $3F offset $1Exx
			b.PhysRam[0x70000+uint32(addr)] = val
			return
		}
		slot := addr >> 13
		page := b.MmuRegs[b.MmuTask&1][slot] & 0x3F
		physAddr := (uint32(page) << 13) | uint32(addr&0x1FFF)
		b.PhysRam[physAddr] = val
		return
	}

	if b.Engine == EngineFlat65280v2 {
		if addr >= 0xFF00 {
			b.writeIOFlat(addr, val)
			return
		}
		b.Memory[0][addr] = val
		return
	}

	// In user tasks (Task > 0), access to $FF00..$FFFF is strictly forbidden unless blessed with I/O flag
	if addr >= 0xFF00 {
		if b.CurrentTask == 0 || (b.TaskFlags[b.CurrentTask]&0x01) != 0 {
			b.writeIO(addr, val)
			return
		}
		panic(fmt.Errorf("%w: write at 0x%04X in task %d", ErrUserAccessTrap, addr, b.CurrentTask))
	}

	// Shared Memory Curtain for Tasks 0, 1, and 2
	if b.SharedMemoryCurtain > 0 && addr >= b.SharedMemoryCurtain && b.CurrentTask <= 2 {
		b.Memory[0][addr] = val
		return
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

	case 0xFF28:
		return byte(b.SharedMemoryCurtain >> 8)
	case 0xFF29:
		return byte(b.SharedMemoryCurtain)

	case 0xFF2D:
		return b.TaskFlagsTarget
	case 0xFF2E: // TaskFlagsRegister
		return b.TaskFlags[b.TaskFlagsTarget]

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
			if val == 10 || val == 13 {
				b.ConsoleOut.Write([]byte{'\n'})
			} else if b.CurlyEscape && !(val >= 32 && val <= 126) {
				fmt.Fprintf(b.ConsoleOut, "{%d}", val)
			} else {
				b.ConsoleOut.Write([]byte{val})
			}
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

	case 0xFF28:
		b.SharedMemoryCurtain = (b.SharedMemoryCurtain & 0x00FF) | (uint16(val) << 8)
	case 0xFF29:
		b.SharedMemoryCurtain = (b.SharedMemoryCurtain & 0xFF00) | uint16(val)

	case 0xFF2D: // TaskFlagsTarget: target task to configure
		b.TaskFlagsTarget = val
	case 0xFF2E: // TaskFlagsRegister: sets flags for TaskFlagsTarget (Bit 0 = allow I/O)
		b.TaskFlags[b.TaskFlagsTarget] = val

	case 0xFF2F: // PurgeTaskMem: zero task memory if val != 0
		if val != 0 {
			b.purgeTaskMem(val)
		}

	default:
		panic(fmt.Errorf("%w: write at unmapped 0x%04X in Task 0", ErrKernelIOPanic, addr))
	}
}

func (b *Bus) evalIRQ() {
	var asserted bool
	if b.Engine == EngineFlat65280v2 || b.Engine == EngineDeep65280v2 {
		rdrf := len(b.ConsoleIn) > 0
		rie := (b.AciaCtrl & 0x80) != 0
		aciaIRQ := rdrf && rie
		clockIRQ := (b.ClockAndStop&0x01 != 0) && (b.ClockAndStop&0x02 != 0)
		asserted = aciaIRQ || clockIRQ
	} else {
		asserted = (b.RegStat & b.RegCtrl & 0x03) != 0
	}
	if b.OnIRQChanged != nil {
		b.OnIRQChanged(asserted)
	}
}

// TimerTick signals a 60Hz timer tick.
func (b *Bus) TimerTick() {
	if b.Engine == EngineFlat65280v2 || b.Engine == EngineDeep65280v2 {
		return // Handled via ClockTick in flat65280v2 and deep65280v2
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	b.RegStat |= 0x01 // Timer.Ready
	b.evalIRQ()
}

func (b *Bus) readIOFlat(addr uint16) byte {
	if addr >= 0xFFF0 {
		return b.Memory[0][addr]
	}

	switch {
	case addr >= 0xFF80 && addr <= 0xFF86:
		return b.readEmuDsk(addr)
	case addr == 0xFF87:
		return b.readClockAndStop()
	case addr == 0xFF88:
		return b.readAciaStatus()
	case addr == 0xFF89:
		return b.readAciaData()
	default:
		panic(fmt.Errorf("%w: read at 0x%04X", ErrFlatStrayRead, addr))
	}
}

func (b *Bus) writeIOFlat(addr uint16, val byte) {
	switch {
	case addr >= 0xFF80 && addr <= 0xFF86:
		b.writeEmuDsk(addr, val)
	case addr == 0xFF87:
		b.writeClockAndStop(val)
	case addr == 0xFF88:
		b.writeAciaCtrl(val)
	case addr == 0xFF89:
		b.writeAciaData(val)
	default:
		panic(fmt.Errorf("%w: write at 0x%04X", ErrFlatStrayWrite, addr))
	}
}

func (b *Bus) readIODeep(addr uint16) byte {
	if addr >= 0xFFF0 {
		return b.PhysRam[0x70000+uint32(addr)]
	}

	switch {
	case addr >= 0xFF80 && addr <= 0xFF86:
		return b.readEmuDsk(addr)
	case addr == 0xFF87:
		return b.readClockAndStop()
	case addr == 0xFF88:
		return b.readAciaStatus()
	case addr == 0xFF89:
		return b.readAciaData()
	case addr >= 0xFF00 && addr <= 0xFF03:
		return 0 // PIA0 dummy
	case addr == 0xFF90:
		return 0 // GIME Init0
	case addr == 0xFF92 || addr == 0xFF93:
		return 0 // GIME IrqEnR / Status
	case addr == 0xFF91:
		return b.MmuTask
	case addr >= 0xFFA0 && addr <= 0xFFA7:
		return b.MmuRegs[0][addr-0xFFA0]
	case addr >= 0xFFA8 && addr <= 0xFFAF:
		return b.MmuRegs[1][addr-0xFFA8]
	case addr == 0xFFD8 || addr == 0xFFDE:
		return 0 // SAM speed / mode
	default:
		panic(fmt.Errorf("%w: read at 0x%04X", ErrDeepStrayRead, addr))
	}
}

func (b *Bus) writeIODeep(addr uint16, val byte) {
	if addr >= 0xFFF0 {
		b.PhysRam[0x70000+uint32(addr)] = val
		return
	}

	switch {
	case addr >= 0xFF00 && addr <= 0xFF03:
		// PIA0 dummy - ignored
	case addr >= 0xFF80 && addr <= 0xFF86:
		b.writeEmuDsk(addr, val)
	case addr == 0xFF87:
		b.writeClockAndStop(val)
	case addr == 0xFF88:
		b.writeAciaCtrl(val)
	case addr == 0xFF89:
		b.writeAciaData(val)
	case addr == 0xFF90:
		// GIME Init0 - ignored in deep65280v2
	case addr == 0xFF91:
		b.MmuTask = val & 0x01
	case addr == 0xFF92 || addr == 0xFF93:
		// GIME IrqEnR / Status - ignored
	case addr >= 0xFFA0 && addr <= 0xFFA7:
		b.MmuRegs[0][addr-0xFFA0] = val & 0x3F
	case addr >= 0xFFA8 && addr <= 0xFFAF:
		b.MmuRegs[1][addr-0xFFA8] = val & 0x3F
	case addr == 0xFFD8 || addr == 0xFFDE:
		// SAM speed / mode - ignored
	default:
		panic(fmt.Errorf("%w: write at 0x%04X", ErrDeepStrayWrite, addr))
	}
}

func (b *Bus) readEmuDsk(addr uint16) byte {
	switch addr {
	case 0xFF80:
		return byte(b.EmuDskLSN >> 16)
	case 0xFF81:
		return byte(b.EmuDskLSN >> 8)
	case 0xFF82:
		return byte(b.EmuDskLSN)
	case 0xFF83:
		return b.EmuDskStatus
	case 0xFF84:
		return byte(b.EmuDskBuffer >> 8)
	case 0xFF85:
		return byte(b.EmuDskBuffer)
	case 0xFF86:
		return b.EmuDskDrive
	default:
		return 0
	}
}

func (b *Bus) writeEmuDsk(addr uint16, val byte) {
	switch addr {
	case 0xFF80:
		b.EmuDskLSN = (b.EmuDskLSN & 0x00FFFF) | (uint32(val) << 16)
	case 0xFF81:
		b.EmuDskLSN = (b.EmuDskLSN & 0xFF00FF) | (uint32(val) << 8)
	case 0xFF82:
		b.EmuDskLSN = (b.EmuDskLSN & 0xFFFF00) | uint32(val)
	case 0xFF83:
		b.executeEmuDskCommand(val)
	case 0xFF84:
		b.EmuDskBuffer = (b.EmuDskBuffer & 0x00FF) | (uint16(val) << 8)
	case 0xFF85:
		b.EmuDskBuffer = (b.EmuDskBuffer & 0xFF00) | uint16(val)
	case 0xFF86:
		b.EmuDskDrive = val
	}
}

func (b *Bus) executeEmuDskCommand(cmd byte) {
	drv := int(b.EmuDskDrive)
	if drv < 0 || drv >= len(b.Disks) || b.Disks[drv] == nil {
		b.EmuDskStatus = 2 // Drive not enabled / not ready
		return
	}
	disk := b.Disks[drv]
	offset := int(b.EmuDskLSN) * 256
	if offset+256 > len(disk) {
		b.EmuDskStatus = 6 // Seek / range error
		return
	}

	bufAddr := b.EmuDskBuffer

	switch cmd {
	case 0: // Read Sector
		for i := 0; i < 256; i++ {
			targetAddr := bufAddr + uint16(i)
			if targetAddr >= 0xFF00 {
				b.EmuDskStatus = 6 // Buffer cannot cross into I/O page
				return
			}
			if b.Engine == EngineDeep65280v2 {
				if targetAddr >= 0xFE00 {
					b.PhysRam[0x70000+uint32(targetAddr)] = disk[offset+i]
				} else {
					slot := targetAddr >> 13
					page := b.MmuRegs[b.MmuTask&1][slot] & 0x3F
					physAddr := (uint32(page) << 13) | uint32(targetAddr&0x1FFF)
					b.PhysRam[physAddr] = disk[offset+i]
				}
			} else {
				b.Memory[0][targetAddr] = disk[offset+i]
			}
		}
		b.EmuDskStatus = 0

	case 1: // Write Sector
		for i := 0; i < 256; i++ {
			srcAddr := bufAddr + uint16(i)
			if srcAddr >= 0xFF00 {
				b.EmuDskStatus = 6
				return
			}
			if b.Engine == EngineDeep65280v2 {
				if srcAddr >= 0xFE00 {
					disk[offset+i] = b.PhysRam[0x70000+uint32(srcAddr)]
				} else {
					slot := srcAddr >> 13
					page := b.MmuRegs[b.MmuTask&1][slot] & 0x3F
					physAddr := (uint32(page) << 13) | uint32(srcAddr&0x1FFF)
					disk[offset+i] = b.PhysRam[physAddr]
				}
			} else {
				disk[offset+i] = b.Memory[0][srcAddr]
			}
		}
		b.EmuDskStatus = 0

	case 2: // Close
		b.EmuDskStatus = 0

	default:
		b.EmuDskStatus = 254 // Invalid command
	}
}

func (b *Bus) readAciaStatus() byte {
	st := byte(0x02) // Bit 1 = TDRE (always 1)
	if len(b.ConsoleIn) == 0 && b.StdinChan != nil {
		b.pollStdinLocked()
	}
	if len(b.ConsoleIn) > 0 {
		st |= 0x01 // Bit 0 = RDRF
	}
	if (st&0x01) != 0 && (b.AciaCtrl&0x80) != 0 {
		st |= 0x80 // Bit 7 = IRQ flag
	}
	return st
}

func (b *Bus) readAciaData() byte {
	if len(b.ConsoleIn) == 0 && b.StdinChan != nil {
		b.pollStdinLocked()
	}
	var ch byte
	if len(b.ConsoleIn) > 0 {
		ch = b.ConsoleIn[0]
		b.ConsoleIn = b.ConsoleIn[1:]
	}
	b.evalIRQ()
	return ch
}

func (b *Bus) writeAciaCtrl(val byte) {
	if (val & 0x03) == 0x03 {
		// Master Reset: clears internal registers, disables interrupts
		b.AciaCtrl = 0
	} else {
		b.AciaCtrl = val
	}
	b.evalIRQ()
}

func (b *Bus) writeAciaData(val byte) {
	if b.ConsoleOut != nil {
		if val == 10 || val == 13 {
			b.ConsoleOut.Write([]byte{'\n'})
		} else if b.CurlyEscape && !(val >= 32 && val <= 126) {
			fmt.Fprintf(b.ConsoleOut, "{%d}", val)
		} else {
			b.ConsoleOut.Write([]byte{val})
		}
	}
}

func (b *Bus) readClockAndStop() byte {
	return b.ClockAndStop
}

func (b *Bus) printStopExplanation(msg string) {
	if b.ConsoleOut != nil {
		fmt.Fprint(b.ConsoleOut, msg)
		if f, ok := b.ConsoleOut.(interface{ Sync() error }); ok {
			_ = f.Sync()
		}
		if f, ok := b.ConsoleOut.(interface{ Flush() error }); ok {
			_ = f.Flush()
		}
	}
	if b.LogOut != nil && b.LogOut != b.ConsoleOut {
		fmt.Fprint(b.LogOut, msg)
		if f, ok := b.LogOut.(interface{ Sync() error }); ok {
			_ = f.Sync()
		}
		if f, ok := b.LogOut.(interface{ Flush() error }); ok {
			_ = f.Flush()
		}
	}
	_ = os.Stdout.Sync()
	_ = os.Stderr.Sync()
}

func (b *Bus) writeClockAndStop(val byte) {
	switch val {
	case 0xFC:
		b.printStopExplanation("\n *** Exiting with status 0 due to CLOCK_AND_STOP command $FC.\n")
		b.ExitCode = 0
		if b.OnHalt != nil {
			b.OnHalt(0)
		} else {
			os.Exit(0)
		}
		return
	case 0xFD:
		b.printStopExplanation("\n *** Exiting with status 1 due to CLOCK_AND_STOP command $FD.\n")
		b.ExitCode = 1
		if b.OnHalt != nil {
			b.OnHalt(1)
		} else {
			os.Exit(1)
		}
		return
	case 0xFE:
		b.printStopExplanation("\n *** Crashing due to CLOCK_AND_STOP command $FE.\n")
		if b.OnCrash != nil {
			b.OnCrash()
			return
		}
		_ = syscall.Kill(os.Getpid(), syscall.SIGABRT)
		panic(ErrFlatClockCrash)
	}

	newLow := b.ClockAndStop & 0x01
	if (val & 0x01) != 0 {
		newLow = 0
	}
	b.ClockAndStop = (val & 0xFE) | newLow
	b.evalIRQ()
}

// ClockTick signals a clock tick for the flat65280v2 engine.
func (b *Bus) ClockTick() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.ClockAndStop |= 0x01
	b.evalIRQ()
}

// ClockRateHz returns the configured clock rate in Hz for flat65280v2.
func (b *Bus) ClockRateHz() float64 {
	switch b.ClockAndStop & 0x30 {
	case 0x00:
		return 60.0
	case 0x10:
		return 50.0
	case 0x20:
		return 6.0
	case 0x30:
		return 0.1
	default:
		return 60.0
	}
}

// ClockCyclesPerTick returns the number of CPU cycles per clock tick given cpuHz.
func (b *Bus) ClockCyclesPerTick(cpuHz int) uint64 {
	if cpuHz <= 0 {
		return 0
	}
	switch b.ClockAndStop & 0x30 {
	case 0x00:
		return uint64(cpuHz / 60)
	case 0x10:
		return uint64(cpuHz / 50)
	case 0x20:
		return uint64(cpuHz / 6)
	case 0x30:
		return uint64(cpuHz) * 10
	default:
		return uint64(cpuHz / 60)
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
// For deep65280v2, it also zeroes PhysRam, resets MMU registers, copies the 64KB image
// to physical pages $38..$3F (0x70000..0x7FFFF), and copies Slot 0 to physical page $00 (0x00000..0x01FFF).
// Slot 0 MMU register ($FFA0/$FFA8) is initialized to block $00, matching the legacy Level 2 SysBlock requirement.
func (b *Bus) LoadRawImage(data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := range b.Memory[0] {
		b.Memory[0][i] = 0
	}
	copy(b.Memory[0][:], data)

	for i := range b.PhysRam {
		b.PhysRam[i] = 0
	}
	copy(b.PhysRam[0x70000:], data)
	if len(data) >= 0x2000 {
		copy(b.PhysRam[0x00000:], data[:0x2000])
	}
	b.MmuTask = 0
	b.MmuRegs[0][0] = 0x00
	b.MmuRegs[1][0] = 0x00
	for i := 1; i < 8; i++ {
		b.MmuRegs[0][i] = byte(0x38 + i)
		b.MmuRegs[1][i] = byte(0x38 + i)
	}

	return nil
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

func (b *Bus) readTaskByte(task byte, addr uint16) byte {
	if b.SharedMemoryCurtain > 0 && addr >= b.SharedMemoryCurtain && addr < 0xFF00 && task <= 2 {
		return b.Memory[0][addr]
	}
	return b.Memory[task][addr]
}

func (b *Bus) writeTaskByte(task byte, addr uint16, val byte) {
	if b.SharedMemoryCurtain > 0 && addr >= b.SharedMemoryCurtain && addr < 0xFF00 && task <= 2 {
		b.Memory[0][addr] = val
		return
	}
	b.Memory[task][addr] = val
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
		val := b.readTaskByte(srcTask, srcAddr+uint16(i))
		b.writeTaskByte(dstTask, dstAddr+uint16(i), val)
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
