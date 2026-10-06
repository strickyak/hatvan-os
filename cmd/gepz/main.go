package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/strickyak/hatvan-os/gepz"
)

var (
	traceFlag         = flag.Bool("trace", false, "print instruction execution trace to stderr")
	maxCyclesFlag     = flag.Uint64("max-cycles", 0, "stop after maximum CPU cycles (0 = unlimited)")
	maxSecondsFlag    = flag.Float64("max-seconds", 300, "stop after maximum real-time seconds (0 = unlimited)")
	tickHzFlag        = flag.Int("tick-hz", 60, "timer tick rate in Hz (0 = disabled)")
	cpuClockHz        = flag.Int("cpu-hz", 4000000, "simulated CPU clock speed in Hz (default 4MHz)")
	inputFlag         = flag.String("input", "", "initial console input to feed to the VM (e.g. \"mdir\\n\")")
	disk0Flag         = flag.String("disk0", "", "disk image file for drive 0")
	disk1Flag         = flag.String("disk1", "", "disk image file for drive 1")
	disk2Flag         = flag.String("disk2", "", "disk image file for drive 2")
	disk3Flag         = flag.String("disk3", "", "disk image file for drive 3")
	baseAddrFlag      = flag.Uint("base", 0, "base memory address for raw binary image (default 0)")
	printCyclesFlag   = flag.Bool("print-cycles", false, "print total CPU cycles executed on finish")
	sharedCurtainFlag = flag.String("shared-curtain", "0xE000", "shared memory curtain address for tasks 0, 1, and 2")
	task1Flag         = flag.String("task1", "", "optional binary (DECB) to load into Task 1")
	task2Flag         = flag.String("task2", "", "optional binary (DECB) to load into Task 2")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gepz [options] <program.decb|program.bin>\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	binPath := args[0]

	// 1. Initialize Bus and CPU
	bus := gepz.NewBus()
	if *sharedCurtainFlag != "" {
		s := strings.TrimPrefix(*sharedCurtainFlag, "0x")
		s = strings.TrimPrefix(s, "$")
		if v, err := strconv.ParseUint(s, 16, 16); err == nil {
			bus.SharedMemoryCurtain = uint16(v)
		}
	}
	cpu := gepz.NewCPU(bus)
	bus.OnHalt = func(exitCode int) {
		cpu.Halted = true
	}

	attachDisk(bus, 0, *disk0Flag)
	attachDisk(bus, 1, *disk1Flag)
	attachDisk(bus, 2, *disk2Flag)
	attachDisk(bus, 3, *disk3Flag)

	// 2. Load program binary
	file, err := os.Open(binPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening binary %q: %v\n", binPath, err)
		os.Exit(1)
	}
	defer file.Close()

	var loaded *gepz.LoadedBinary
	lowerPath := strings.ToLower(binPath)
	if strings.HasSuffix(lowerPath, ".bin") || strings.HasSuffix(lowerPath, ".rom") || strings.HasSuffix(lowerPath, ".img") {
		loaded, err = bus.LoadRawBinary(file, uint16(*baseAddrFlag))
	} else {
		loaded, err = bus.LoadFile(binPath)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading binary %q: %v\n", binPath, err)
		os.Exit(1)
	}

	cpu.Reset()
	if loaded != nil && loaded.HasEntry {
		cpu.PC = loaded.EntryPC
	}

	// Load Task 1 if specified or companion rbf_z80.decb exists
	task1Path := *task1Flag
	if task1Path == "" {
		candidate := filepath.Join(filepath.Dir(binPath), "rbf_z80.decb")
		if _, err := os.Stat(candidate); err == nil {
			task1Path = candidate
		}
	}
	if task1Path != "" {
		_, err = bus.LoadFileIntoTask(1, task1Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading Task 1 binary %q: %v\n", task1Path, err)
			os.Exit(1)
		}
	}

	// Load Task 2 if specified or companion procfs_z80.decb exists
	task2Path := *task2Flag
	if task2Path == "" {
		candidate := filepath.Join(filepath.Dir(binPath), "procfs_z80.decb")
		if _, err := os.Stat(candidate); err == nil {
			task2Path = candidate
		}
	}
	if task2Path != "" {
		_, err = bus.LoadFileIntoTask(2, task2Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading Task 2 binary %q: %v\n", task2Path, err)
			os.Exit(1)
		}
	}

	// 3. Configure console input
	stdinCh := make(chan byte, 1024)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			b, err := reader.ReadByte()
			if err != nil {
				close(stdinCh)
				return
			}
			stdinCh <- b
		}
	}()

	initialInputPending := false
	if *inputFlag != "" {
		bus.EnqueueString(unescapeString(*inputFlag))
		initialInputPending = true
	} else {
		bus.StdinChan = stdinCh
	}

	// 4. Trace setup
	traceTaskStr := os.Getenv("TRACE_TASK")
	if *traceFlag || traceTaskStr != "" {
		cpu.OnInstruction = func(pc uint16, op byte) {
			if traceTaskStr != "" {
				taskNum, _ := strconv.Atoi(traceTaskStr)
				if bus.CurrentTask != uint8(taskNum) {
					return
				}
			}
			disasm, _ := gepz.Disasm(pc, func(a uint16) byte {
				return bus.ReadByte(a)
			})
			fmt.Fprintf(os.Stderr, "[T%d:%04X] %s  %-16s (op=%02X)\n", bus.CurrentTask, pc, cpu.DumpState(), disasm, op)
		}
	}

	// 5. Timer setup
	var timerTicker *time.Ticker
	if *tickHzFlag > 0 {
		timerInterval := time.Second / time.Duration(*tickHzFlag)
		timerTicker = time.NewTicker(timerInterval)
		defer timerTicker.Stop()
	}

	// 6. Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	startTime := time.Now()
	var timeoutChan <-chan time.Time
	if *maxSecondsFlag > 0 {
		timeoutChan = time.After(time.Duration(*maxSecondsFlag * float64(time.Second)))
	}

	// 7. Execution loop
	stepBatch := 5000
	for !cpu.Halted && !bus.Halted {
		// Execute batch of instructions
		for i := 0; i < stepBatch && !cpu.Halted && !bus.Halted; i++ {
			cpu.Step()
			if *maxCyclesFlag > 0 && cpu.Cycles >= *maxCyclesFlag {
				fmt.Fprintf(os.Stderr, "gepz: reached maximum cycles (%d) at %s\n", *maxCyclesFlag, cpu.DumpState())
				cpu.Halted = true
				break
			}
		}

		// Check console input
		if initialInputPending {
			if bus.ConsoleInEmpty() {
				initialInputPending = false
				bus.StdinChan = stdinCh
				bus.PollStdin()
			}
		} else {
			bus.PollStdin()
		}

		// Check timer ticks
		if timerTicker != nil {
			select {
			case <-timerTicker.C:
				bus.RegStat |= 0x01 // Timer.Ready
				if (bus.RegCtrl & 0x01) != 0 {
					if bus.OnIRQChanged != nil {
						bus.OnIRQChanged(true)
					}
				}
			default:
			}
		}

		// Check timeout
		if timeoutChan != nil {
			select {
			case <-timeoutChan:
				fmt.Fprintf(os.Stderr, "gepz: reached maximum execution time (%.1fs)\n", *maxSecondsFlag)
				cpu.Halted = true
			default:
			}
		}

		// Check OS signals
		select {
		case <-sigChan:
			fmt.Fprintf(os.Stderr, "\ngepz: interrupted by signal\n")
			cpu.Halted = true
		default:
		}
	}

	elapsed := time.Since(startTime)
	if *printCyclesFlag {
		mhz := float64(cpu.Cycles) / elapsed.Seconds() / 1e6
		fmt.Fprintf(os.Stderr, "Total cycles: %d in %.2fs (%.2f MHz)\n", cpu.Cycles, elapsed.Seconds(), mhz)
	}

	os.Exit(bus.ExitCode)
}

func attachDisk(bus *gepz.Bus, drive int, path string) {
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not open disk image %q for drive %d: %v\n", path, drive, err)
		return
	}
	if filepath.Ext(path) == "" {
		// ensure non-empty
	}
	bus.Disks[drive] = data
}

func unescapeString(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\r`, "\r")
	s = strings.ReplaceAll(s, `\t`, "\t")
	s = strings.ReplaceAll(s, `\d`, "\x04")
	s = strings.ReplaceAll(s, `\x04`, "\x04")
	return s
}

