package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/strickyak/hatvan-os/gep9"
)

var (
	traceFlag      = flag.Bool("trace", false, "print instruction execution trace")
	maxCyclesFlag  = flag.Uint64("max-cycles", 0, "stop after maximum CPU cycles (0 = unlimited)")
	maxSecondsFlag = flag.Float64("max-seconds", 300, "stop after maximum real-time seconds (0 = unlimited)")
	tickHzFlag     = flag.Int("tick-hz", 5, "timer tick rate in Hz (0 = disabled)")
	cpuClockHz     = flag.Int("cpu-hz", 2000000, "simulated CPU clock speed in Hz (default 2MHz)")
	inputFlag      = flag.String("input", "", "initial console input to feed to the VM (e.g. \"mdir\\n\")")
	disk0Flag      = flag.String("disk0", "", "disk image file for /d0")
	disk1Flag      = flag.String("disk1", "", "disk image file for /d1")
	disk2Flag      = flag.String("disk2", "", "disk image file for /d2")
	disk3Flag      = flag.String("disk3", "", "disk image file for /d3")
	task1Flag       = flag.String("task1", "", "optional binary (DECB) to load into Task 1")
	task2Flag       = flag.String("task2", "", "optional binary (DECB) to load into Task 2")
	hypercallsFlag  = flag.Bool("hypercalls", false, "enable GOMAR-compatible hypercall traps ($12,$21,<hop>)")
	printCyclesFlag = flag.Bool("print-cycles", false, "print total CPU cycles executed on finish")
	curlyEscapeFlag = flag.Bool("curly-escape", true, "escape unusual characters as '{%d}'")
	traceTrapFlag   = flag.Bool("trace-trap", false, "print system call trap trace to stderr")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gep9 [options] <program.decb|system.img> [listing.list ...]\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *traceTrapFlag {
		os.Setenv("HATVAN_TRACE_TRAP", "1")
	}

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	binPath := args[0]
	var listPaths []string
	for _, a := range args[1:] {
		if strings.HasSuffix(a, ".list") || strings.HasSuffix(a, ".lst") || strings.HasSuffix(a, ".listing") {
			listPaths = append(listPaths, a)
		}
	}

	// 1. Initialize Bus and CPU
	bus := gep9.NewBus()
	bus.CurlyEscape = *curlyEscapeFlag
	cpu := gep9.NewCPU(bus)
	cpu.EnableHypercalls = *hypercallsFlag

	// Attach disk images if specified
	attachDisk(bus, 0, *disk0Flag)
	attachDisk(bus, 1, *disk1Flag)
	attachDisk(bus, 2, *disk2Flag)
	attachDisk(bus, 3, *disk3Flag)

	// 2. Load program binary (Raw 64KB .img/.rom or DECB format)
	fileData, err := os.ReadFile(binPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading binary %q: %v\n", binPath, err)
		os.Exit(1)
	}

	var decb *gep9.DECB
	isRaw := strings.HasSuffix(strings.ToLower(binPath), ".img") ||
		strings.HasSuffix(strings.ToLower(binPath), ".rom") ||
		len(fileData) == 65536

	if isRaw {
		if len(fileData) > 65536 {
			fmt.Fprintf(os.Stderr, "Error: raw image %q exceeds 64KB (%d bytes)\n", binPath, len(fileData))
			os.Exit(1)
		}
		if err := bus.LoadRawImage(fileData); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading raw image %q: %v\n", binPath, err)
			os.Exit(1)
		}
		cpu.Reset()
	} else {
		decb, err = gep9.LoadDECBFile(binPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading DECB file %q: %v\n", binPath, err)
			os.Exit(1)
		}
		bus.LoadDECBIntoTask(0, decb)
		cpu.Reset()
		if decb.HasExec {
			cpu.PC = decb.ExecAddr
		}
	}

	// Load Task 1 if specified or companion rbf_6809.decb exists
	task1Path := *task1Flag
	if task1Path == "" {
		candidate := filepath.Join(filepath.Dir(binPath), "rbf_6809.decb")
		if _, err := os.Stat(candidate); err == nil {
			task1Path = candidate
		}
	}
	if task1Path != "" {
		d1, err := gep9.LoadDECBFile(task1Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading Task 1 DECB %q: %v\n", task1Path, err)
			os.Exit(1)
		}
		bus.LoadDECBIntoTask(1, d1)
	}

	// Load Task 2 if specified or companion procfs_6809.decb exists
	task2Path := *task2Flag
	if task2Path == "" {
		candidate := filepath.Join(filepath.Dir(binPath), "procfs_6809.decb")
		if _, err := os.Stat(candidate); err == nil {
			task2Path = candidate
		}
	}
	if task2Path != "" {
		d2, err := gep9.LoadDECBFile(task2Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading Task 2 DECB %q: %v\n", task2Path, err)
			os.Exit(1)
		}
		bus.LoadDECBIntoTask(2, d2)
	}

	// 3. Scan for OS-9 modules in memory
	modules := gep9.ScanOS9Modules(bus.Memory[0][:], 0x0000, 0xFF00)
	modulesByName := make(map[string]*gep9.OS9LoadedModule)
	for _, m := range modules {
		modulesByName[m.Name] = m
	}

	// 4. Load optional .list files and offset-adjust them if matching an OS-9 module
	var listings []*gep9.Listing
	for _, lp := range listPaths {
		l, err := gep9.LoadListingFile(lp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load listing %q: %v\n", lp, err)
			continue
		}
		if mod, ok := modulesByName[l.ModuleName]; ok {
			offsetListing := l.OffsetCopy(mod.BaseAddr)
			listings = append(listings, offsetListing)
		} else {
			listings = append(listings, l)
		}
	}

	// 5. Setup console input
	stdinCh := make(chan byte, 1024)
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				for i := 0; i < n; i++ {
					stdinCh <- buf[i]
				}
			}
			if err != nil {
				close(stdinCh)
				return
			}
		}
	}()

	initialInputPending := false
	if *inputFlag != "" {
		initialInputPending = true
		bus.EnqueueString(unescapeString(*inputFlag))
	} else {
		bus.StdinChan = stdinCh
	}

	// Catch SIGINT cleanly
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\n[gep9: interrupted by signal]")
		printRegisters(cpu)
		os.Exit(0)
	}()

	// 6. Execution loop
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
		pc := cpu.PC

		if *traceFlag {
			src := lookupSource(pc, decb, listings)
			fmt.Fprintf(os.Stderr, "PC=%04X  A=%02X B=%02X X=%04X Y=%04X U=%04X S=%04X CC=%s  %s\n",
				pc, cpu.A, cpu.B, cpu.X, cpu.Y, cpu.U, cpu.S, formatCC(cpu.CC), src)
		}

		c := cpu.Step()
		if c == 0 {
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
				fmt.Fprintf(os.Stderr, "\n[gep9: reached maximum realtime limit of %.1f seconds]\n", *maxSecondsFlag)
				printRegisters(cpu)
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
			fmt.Fprintf(os.Stderr, "\n[gep9: reached maximum cycle limit %d]\n", *maxCyclesFlag)
			break
		}
	}

	if *traceFlag || *printCyclesFlag || os.Getenv("HATVAN_PRINT_CYCLES") != "" {
		fmt.Fprintf(os.Stderr, "[gep9 finished: %d total cycles executed]\n", cpu.Cycles)
	}
	if *traceFlag {
		printRegisters(cpu)
	}

	if cpu.Halted {
		os.Exit(cpu.ExitCode)
	}
}

func unescapeString(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\r`, "\r")
	s = strings.ReplaceAll(s, `\t`, "\t")
	s = strings.ReplaceAll(s, `\d`, "\x04")
	s = strings.ReplaceAll(s, `\x04`, "\x04")
	return s
}

func attachDisk(b *gep9.Bus, drive int, path string) {
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

func lookupSource(pc uint16, decb *gep9.DECB, listings []*gep9.Listing) string {
	// 1. Check DECB absolute source lines
	if decb != nil {
		if line, ok := decb.AbsLines[pc]; ok {
			return fmt.Sprintf("(%d) %s", line.LineNum, line.Text)
		}
	}
	// 2. Check external .list files
	for _, l := range listings {
		if line, ok := l.LinesByAddr[pc]; ok {
			if l.ModuleName != "" {
				return fmt.Sprintf("[%s:%d] %s", l.ModuleName, line.LineNum, line.Text)
			}
			return fmt.Sprintf("(%d) %s", line.LineNum, line.Text)
		}
	}
	// 3. Check DECB absolute symbols
	if decb != nil {
		if sym, ok := decb.AbsSymbolsByAddr[pc]; ok {
			return fmt.Sprintf("<%s>", sym)
		}
	}
	return ""
}

func formatCC(cc byte) string {
	chars := []byte("EFHINZVC")
	var sb strings.Builder
	sb.WriteByte('[')
	for i := 7; i >= 0; i-- {
		if (cc & (1 << i)) != 0 {
			sb.WriteByte(chars[7-i])
		} else {
			sb.WriteByte('.')
		}
	}
	sb.WriteByte(']')
	return sb.String()
}

func printRegisters(cpu *gep9.CPU) {
	fmt.Fprintf(os.Stderr, "CPU State: PC=%04X A=%02X B=%02X X=%04X Y=%04X U=%04X S=%04X DP=%02X CC=%02X %s MD=%02X\n",
		cpu.PC, cpu.A, cpu.B, cpu.X, cpu.Y, cpu.U, cpu.S, cpu.DP, cpu.CC, formatCC(cpu.CC), cpu.MD)
}
