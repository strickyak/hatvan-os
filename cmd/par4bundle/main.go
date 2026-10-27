package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func parseHexOrDec(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	s = strings.TrimPrefix(s, "$")
	val, err := strconv.ParseUint(s, 16, 16)
	if err == nil {
		return uint16(val), nil
	}
	v2, err2 := strconv.ParseUint(s, 10, 16)
	if err2 == nil {
		return uint16(v2), nil
	}
	return 0, fmt.Errorf("invalid address: %q", s)
}

func writeChunk(w io.Writer, tag byte, length uint16, addr uint16, data []byte) error {
	hdr := make([]byte, 5)
	hdr[0] = tag
	binary.BigEndian.PutUint16(hdr[1:3], length)
	binary.BigEndian.PutUint16(hdr[3:5], addr)
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	vmFlag := flag.String("vm", "", "Path to VM binary (par4vmc.bin/par4vmz.bin)")
	vmBaseFlag := flag.String("vm-base", "0x0000", "VM binary load address when raw (default 0x0000)")
	archFlag := flag.String("arch", "c", "Hatvan architecture char for 0xFD magic header (default 'c')")
	p4pFlag := flag.String("p4p", "", "Path to Par4 bytecode binary (*.p4p)")
	outFlag := flag.String("o", "", "Output DECB binary (*.decb)")
	entryFlag := flag.String("entry", "0x0000", "Execution entry address (hex or dec)")
	baseFlag := flag.String("base", "0x2000", "Bytecode load base address (default 0x2000)")

	p4sFlag := flag.String("p4s", "", "Path to Par4 assembly source (*.p4s) for symbol resolution")
	flag.Parse()

	if *p4pFlag == "" || *outFlag == "" {
		fmt.Fprintf(os.Stderr, "Usage: par4bundle -p4p <prog.p4p> [-vm <par4vm.bin>] [-vm-base 0x0000] [-arch c] [-p4s <kernel.p4s>] -o <out.decb> [-entry 0x0000] [-base 0x2000]\n")
		os.Exit(1)
	}

	vmBaseAddr, err := parseHexOrDec(*vmBaseFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid vm-base address: %v\n", err)
		os.Exit(1)
	}

	entryAddr, err := parseHexOrDec(*entryFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid entry address: %v\n", err)
		os.Exit(1)
	}

	baseAddr, err := parseHexOrDec(*baseFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid base address: %v\n", err)
		os.Exit(1)
	}

	p4pData, err := os.ReadFile(*p4pFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading p4p file %q: %v\n", *p4pFlag, err)
		os.Exit(1)
	}

	outFile, err := os.Create(*outFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file %q: %v\n", *outFlag, err)
		os.Exit(1)
	}
	defer outFile.Close()

	// 1. Write Hatvan Executable Magic marker: Tag 0xFD, length 0, addr = ('x' << 8) | arch
	archChar := byte('c')
	if len(*archFlag) > 0 {
		archChar = (*archFlag)[0]
	}
	magicAddr := (uint16('x') << 8) | uint16(archChar)
	if err := writeChunk(outFile, 0xFD, 0, magicAddr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing magic: %v\n", err)
		os.Exit(1)
	}

	// 2. If VM binary specified, load and write as Block 0
	if *vmFlag != "" {
		vmData, err := os.ReadFile(*vmFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading VM file %q: %v\n", *vmFlag, err)
			os.Exit(1)
		}
		if *p4sFlag != "" && len(vmData) >= 0x20 {
			p4sContent, err := os.ReadFile(*p4sFlag)
			if err == nil {
				funcIdx := 0
				for _, line := range strings.Split(string(p4sContent), "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, ".function ") {
						parts := strings.Fields(line)
						if len(parts) >= 2 {
							fnName := parts[1]
							if fnName == "syscall_HandleSyscall" {
								binary.BigEndian.PutUint16(vmData[0x14:0x16], uint16(funcIdx))
								fmt.Printf("par4bundle: patched syscall_HandleSyscall funcIdx=%d at VM 0x0014\n", funcIdx)
							} else if fnName == "proc_ScheduleNext" {
								binary.BigEndian.PutUint16(vmData[0x16:0x18], uint16(funcIdx))
								fmt.Printf("par4bundle: patched proc_ScheduleNext funcIdx=%d at VM 0x0016\n", funcIdx)
							}
						}
						funcIdx++
					}
				}
			}
		}

		// If VM data is a DECB file, extract the raw data chunks
		if len(vmData) >= 5 && vmData[0] == 0x00 {
			// Extract chunk
			chunkLen := binary.BigEndian.Uint16(vmData[1:3])
			chunkAddr := binary.BigEndian.Uint16(vmData[3:5])
			if int(chunkLen)+5 <= len(vmData) {
				vmData = vmData[5 : 5+chunkLen]
				base := chunkAddr
				if err := writeChunk(outFile, 0x00, uint16(len(vmData)), base, vmData); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing VM chunk: %v\n", err)
					os.Exit(1)
				}
			} else {
				if err := writeChunk(outFile, 0x00, uint16(len(vmData)), vmBaseAddr, vmData); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing VM chunk: %v\n", err)
					os.Exit(1)
				}
			}
		} else {
			if err := writeChunk(outFile, 0x00, uint16(len(vmData)), vmBaseAddr, vmData); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing VM chunk: %v\n", err)
				os.Exit(1)
			}
		}
	}

	// 3. Write P-code bytecode chunk at baseAddr (0x1000)
	if err := writeChunk(outFile, 0x00, uint16(len(p4pData)), baseAddr, p4pData); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing bytecode chunk: %v\n", err)
		os.Exit(1)
	}

	// 4. Write DECB Trailer: Tag 0xFF, length 0, addr = entryAddr
	if err := writeChunk(outFile, 0xFF, 0, entryAddr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing trailer: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Bundled %s -> %s (p4p=%d bytes, entry=0x%04X, base=0x%04X)\n",
		*p4pFlag, *outFlag, len(p4pData), entryAddr, baseAddr)
}
