package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/strickyak/hatvan-os/gepk"
)

var (
	traceFlag       = flag.Bool("trace", false, "print instruction execution trace to stderr")
	maxCyclesFlag   = flag.Uint64("max-cycles", 0, "stop after maximum CPU cycles (0 = unlimited)")
	maxSecondsFlag  = flag.Float64("max-seconds", 300, "stop after maximum real-time seconds (0 = unlimited)")
	tickHzFlag      = flag.Int("tick-hz", 60, "timer tick rate in Hz (0 = disabled)")
	cpuClockHz      = flag.Int("cpu-hz", 8000000, "simulated CPU clock speed in Hz (default 8MHz)")
	inputFlag       = flag.String("input", "", "initial console input to feed to the VM (e.g. \"mdir\\n\")")
	disk0Flag       = flag.String("disk0", "", "disk image file for drive 0")
	disk1Flag       = flag.String("disk1", "", "disk image file for drive 1")
	disk2Flag       = flag.String("disk2", "", "disk image file for drive 2")
	disk3Flag       = flag.String("disk3", "", "disk image file for drive 3")
	baseAddrFlag    = flag.Uint("base", 0, "base memory address for raw binary image (default 0)")
	printCyclesFlag = flag.Bool("print-cycles", false, "print total CPU cycles executed on finish")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gepk [options] <program.s37|program.img>\n\nOptions:\n")
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
	bus := gepk.NewBus()
	cpu := gepk.NewCPU(bus)

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

	lowerPath := strings.ToLower(binPath)
	isSRec := strings.HasSuffix(lowerPath, ".s37") ||
		strings.HasSuffix(lowerPath, ".s28") ||
		strings.HasSuffix(lowerPath, ".s19") ||
		strings.HasSuffix(lowerPath, ".srec") ||
		strings.HasSuffix(lowerPath, ".s")

	if isSRec {
		entryAddr, hasEntry, err := bus.LoadSRecords(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading S-Records from %q: %v\n", binPath, err)
			os.Exit(1)
		}
		cpu.Reset()
		if hasEntry {
			cpu.PC = entryAddr
		}
	} else {
		// Raw binary image
		data, err := io.ReadAll(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading raw image %q: %v\n", binPath, err)
			os.Exit(1)
		}
		if err := bus.LoadRawImage(uint32(*baseAddrFlag), data); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading raw image %q: %v\n", binPath, err)
			os.Exit(1)
		}
		cpu.Reset()
		if *baseAddrFlag != 0 {
			cpu.PC = uint32(*baseAddrFlag)
		}
	}

	// 3. Setup console input
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

	// 4. Handle OS signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\n[gepk: interrupted by signal]")
		fmt.Fprintln(os.Stderr, cpu.String())
		os.Exit(0)
	}()

	// 5. Execution loop
	var cyclesSinceTick uint64
	var cyclesPerTick uint64
	var cyclesSinceInputCheck uint64
	if *tickHzFlag > 0 && *cpuClockHz > 0 {
		cyclesPerTick = uint64(*cpuClockHz / *tickHzFlag)
	}

	startTime := time.Now()
	var maxDuration time.Duration
	if *maxSecondsFlag > 0 {
		maxDuration = time.Duration(*maxSecondsFlag * float64(time.Second))
	}

	for !cpu.Halted {
		if bus.ExitCode != 0 {
			break
		}

		if *traceFlag {
			disasm, _ := cpu.Disassemble(cpu.PC)
			fmt.Fprintf(os.Stderr, "PC=%06X SR=%04X [S=%d X=%d N=%d Z=%d V=%d C=%d] D0=%08X D1=%08X A0=%08X A7=%08X  %s\n",
				cpu.PC, cpu.SR,
				boolToInt((cpu.SR&gepk.FlagS) != 0),
				boolToInt((cpu.SR&gepk.FlagX) != 0),
				boolToInt((cpu.SR&gepk.FlagN) != 0),
				boolToInt((cpu.SR&gepk.FlagZ) != 0),
				boolToInt((cpu.SR&gepk.FlagV) != 0),
				boolToInt((cpu.SR&gepk.FlagC) != 0),
				cpu.D[0], cpu.D[1], cpu.A[0], cpu.A[7],
				disasm,
			)
		}

		c := cpu.Step()
		if (c == 0 && cpu.Halted) || bus.Exited {
			break
		}

		if cyclesPerTick > 0 {
			cyclesSinceTick += uint64(c)
			if cyclesSinceTick >= cyclesPerTick {
				cyclesSinceTick -= cyclesPerTick
				bus.TimerTick()
			}
		}

		cyclesSinceInputCheck += uint64(c)
		if cyclesSinceInputCheck >= 1024 {
			cyclesSinceInputCheck = 0
			if maxDuration > 0 && time.Since(startTime) >= maxDuration {
				fmt.Fprintf(os.Stderr, "\n[gepk: reached maximum realtime limit of %.1f seconds]\n", *maxSecondsFlag)
				break
			}
			if initialInputPending {
				if bus.ConsoleInEmpty() {
					initialInputPending = false
					bus.StdinChan = stdinCh
					bus.PollStdin()
				}
			} else {
				bus.PollStdin()
			}
		}

		if *maxCyclesFlag > 0 && cpu.Cycles >= *maxCyclesFlag {
			fmt.Fprintf(os.Stderr, "\n[gepk: reached maximum cycle limit %d]\n", *maxCyclesFlag)
			break
		}
	}

	if *traceFlag || *printCyclesFlag || os.Getenv("HATVAN_PRINT_CYCLES") != "" {
		fmt.Fprintf(os.Stderr, "[gepk finished: %d total cycles executed]\n", cpu.Cycles)
	}
	if *traceFlag {
		fmt.Fprintln(os.Stderr, cpu.String())
	}

	if bus.ExitCode != 0 {
		os.Exit(int(bus.ExitCode))
	}
}

func unescapeString(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\r`, "\r")
	s = strings.ReplaceAll(s, `\t`, "\t")
	return s
}

func attachDisk(b *gepk.Bus, drive int, path string) {
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot load disk image %q for drive %d: %v\n", path, drive, err)
		return
	}
	b.Disks[drive] = data
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
