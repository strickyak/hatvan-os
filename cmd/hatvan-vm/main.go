package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/strickyak/hatvan-os/vm"
)

var (
	traceFlag     = flag.Bool("trace", false, "print instruction execution trace")
	maxCyclesFlag = flag.Uint64("max-cycles", 0, "stop after maximum CPU cycles (0 = unlimited)")
	tickHzFlag    = flag.Int("tick-hz", 60, "timer tick rate in Hz (0 = disabled)")
	cpuClockHz    = flag.Int("cpu-hz", 2000000, "simulated CPU clock speed in Hz (default 2MHz)")
	disk0Flag     = flag.String("disk0", "", "disk image file for /d0")
	disk1Flag     = flag.String("disk1", "", "disk image file for /d1")
	disk2Flag     = flag.String("disk2", "", "disk image file for /d2")
	disk3Flag     = flag.String("disk3", "", "disk image file for /d3")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: hatvan-vm [options] <program.decb> [listing.list ...]\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	decbPath := args[0]
	var listPaths []string
	for _, a := range args[1:] {
		if strings.HasSuffix(a, ".list") || strings.HasSuffix(a, ".lst") || strings.HasSuffix(a, ".listing") {
			listPaths = append(listPaths, a)
		}
	}

	// 1. Initialize Bus and CPU
	bus := vm.NewBus()
	cpu := vm.NewCPU(bus)

	// Attach disk images if specified
	attachDisk(bus, 0, *disk0Flag)
	attachDisk(bus, 1, *disk1Flag)
	attachDisk(bus, 2, *disk2Flag)
	attachDisk(bus, 3, *disk3Flag)

	// 2. Load DECB file
	decb, err := vm.LoadDECBFile(decbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading DECB file %q: %v\n", decbPath, err)
		os.Exit(1)
	}

	bus.LoadDECBIntoTask(0, decb)

	// 3. Load optional .list files
	var listings []*vm.Listing
	for _, lp := range listPaths {
		l, err := vm.LoadListingFile(lp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load listing %q: %v\n", lp, err)
		} else {
			listings = append(listings, l)
		}
	}

	// 4. Reset CPU to loaded Reset Vector
	cpu.Reset()
	if decb.HasExec {
		cpu.PC = decb.ExecAddr
	}

	// Catch SIGINT cleanly
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\n[hatvan-vm: interrupted by signal]")
		printRegisters(cpu)
		os.Exit(0)
	}()

	// 5. Execution loop
	var cyclesSinceTick uint64
	var cyclesPerTick uint64
	if *tickHzFlag > 0 && *cpuClockHz > 0 {
		cyclesPerTick = uint64(*cpuClockHz / *tickHzFlag)
	}

	for !cpu.Halted {
		pc := cpu.PC

		if *traceFlag {
			src := lookupSource(pc, decb, listings)
			fmt.Printf("PC=%04X  A=%02X B=%02X X=%04X Y=%04X U=%04X S=%04X CC=%s  %s\n",
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

		if *maxCyclesFlag > 0 && cpu.Cycles >= *maxCyclesFlag {
			fmt.Fprintf(os.Stderr, "\n[hatvan-vm: reached maximum cycle limit %d]\n", *maxCyclesFlag)
			break
		}
	}

	if *traceFlag {
		fmt.Printf("[hatvan-vm finished: %d total cycles executed]\n", cpu.Cycles)
		printRegisters(cpu)
	}
}

func attachDisk(b *vm.Bus, drive int, path string) {
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

func lookupSource(pc uint16, decb *vm.DECB, listings []*vm.Listing) string {
	// 1. Check DECB absolute source lines
	if line, ok := decb.AbsLines[pc]; ok {
		return fmt.Sprintf("(%d) %s", line.LineNum, line.Text)
	}
	// 2. Check external .list files
	for _, l := range listings {
		if line, ok := l.LinesByAddr[pc]; ok {
			return fmt.Sprintf("(%d) %s", line.LineNum, line.Text)
		}
	}
	// 3. Check DECB absolute symbols
	if sym, ok := decb.AbsSymbolsByAddr[pc]; ok {
		return fmt.Sprintf("<%s>", sym)
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

func printRegisters(cpu *vm.CPU) {
	fmt.Printf("CPU State: PC=%04X A=%02X B=%02X X=%04X Y=%04X U=%04X S=%04X DP=%02X CC=%02X %s MD=%02X\n",
		cpu.PC, cpu.A, cpu.B, cpu.X, cpu.Y, cpu.U, cpu.S, cpu.DP, cpu.CC, formatCC(cpu.CC), cpu.MD)
}
