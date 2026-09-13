---
name: turbos
description: >-
  Architecture, memory map, module layouts, boot sequence, and debugging instructions
  for running TurbOS (NitrOS-9 / OS-9 compatible kernel by Boisy Pitre) on the Turbo9Sim port and Hatvan VM.
  Use when analyzing TurbOS binaries (.img and .img.rom), building and running TurbOS kernels,
  understanding OS-9 module headers, or troubleshooting TurbOS execution and device drivers.
---

# TurbOS Architecture & Turbo9Sim Port Guide

TurbOS is an open-source, modular, re-entrant operating system kernel for the 6809 and 6309 microprocessors created by Boisy Pitre. It is binary-compatible with Microware OS-9 Level 1 and NitrOS-9.

On this system, the Turbo9Sim port is located at:
`/home/strick/modoc/coco-shelf/turbos/ports/turbo9sim`

---

## 1. Binary Image Layouts (`.img` vs. `.img.rom`)

### A. The 64KB System Image (`turbos_dev.img`)
The `.img` file is exactly 65,536 bytes ($2^{16}$) and overlays directly onto the 64KB memory space of Task 0:

| Address Range | Size | Description |
| :--- | :--- | :--- |
| **`$0000..$001F`** | 32 bytes | Direct Page scratchpad & CPU registers. |
| **`$0020..$0111`** | 242 bytes| System Globals (`D.FMBM` through `D.XNMI`). |
| **`$0100..$0111`** | 18 bytes | Vector trampolines (`D.XSWI3`, `D.XSWI2`, `D.XFIRQ`, `D.XIRQ`, `D.XSWI`, `D.XNMI`). |
| **`$0200..$021F`** | 32 bytes | Free Memory Allocation Bitmap (1 bit = 256-byte page). |
| **`$0220..$0221`** | 2 bytes  | IOMan I/O entry vector pointer. |
| **`$0222..$0291`** | 112 bytes| System Service Dispatch Table (56 vector pointers). |
| **`$0292..$02FF`** | 110 bytes| User Service Dispatch Table. |
| **`$0300..$03FF`** | 256 bytes| Module Directory (up to 64 module header pointer entries). |
| **`$0400..$04FF`** | 256 bytes| Kernel System Stack (`S`). |
| **`$0500..$D4DD`** | ~53 KB   | Free RAM for process allocation and module loading. |
| **`$D4DE..$FEFF`** | 10,786 B | **ROM Modules** (identical to `turbos_dev.img.rom`). |
| **`$FF00..$FFEF`** | 240 bytes| Memory-Mapped I/O page (Turbo9Sim device registers). |
| **`$FFF0..$FFFF`** | 16 bytes | **Hardware Vectors** (SWI, IRQ, FIRQ, and Reset vector). |

### B. The Packed ROM File (`turbos_dev.img.rom`)
The `.rom` file contains the raw concatenated OS-9 modules (10,786 bytes) that get placed immediately below `$FF00` (from `$D4DE` to `$FEFF`).

You can inspect the modules, editions, sizes, and CRCs using `os9 ident`:
```bash
os9 ident /home/strick/modoc/coco-shelf/turbos/ports/turbo9sim/turbos_dev.img.rom
```

The 13 packed modules in `turbos_dev` are:
1. `kernel` (`$0D4E` bytes, exec offset `$0014` -> resets to `$D4F2`)
2. `init` (`$0036` bytes, system configuration & feature flags)
3. `tk` (`$0195` bytes, tkt9sim timer tick driver)
4. `ioman` (`$070A` bytes, OS-9 I/O manager)
5. `scf` (`$06D0` bytes, Sequential Character File manager)
6. `scvt` (`$0107` bytes, virtual terminal driver using `$FF00..$FF03`)
7. `term` (`$003C` bytes, terminal descriptor)
8. `go` (`$0032` bytes, simple initial loop program)
9. `shell` (`$060F` bytes, interactive command shell)
10. `mfree` (`$0128` bytes, reports free memory)
11. `mdir` (`$01BD` bytes, lists modules in module directory)
12. `procs` (`$0279` bytes, lists active processes)
13. `sleep` (`$004D` bytes, sleeps for specified ticks)

---

## 2. Boot Sequence Flow

1. **Hardware Reset Vector (`$FFFE..$FFFF`):**
   * Reads `$D4F2` (calculated as `ModTop + $0014` execution offset).
2. **System Globals Initialization:**
   * Clears memory from `$0020` to `$0400` with zeroes.
   * Configures Free Memory Bitmap (`$0200..$021F`).
   * Copies vector trampolines to `$0100..$0111` (`D.XSWI3`).
3. **Module Directory Scanning (`ValMods`):**
   * The kernel takes `ModTop` (`$D4DE`) and scans linearly up to `MappedIOStart` (`$FF00`).
   * Validates OS-9 module headers (`$87 $CD`), header parity, and 24-bit CRCs.
   * Registers all 13 modules into the Module Directory (`$0300..$03FF`).
4. **Subsystem Linking & Start:**
   * Links to `init` (`F$Link`).
   * Links to `tk` clock driver, installing 60Hz ticker interrupt handler (`$FF02` / `$FF03`).
   * Unmasks CPU interrupts (`ANDCC #^FlagI`).
   * Launches the initial command (e.g. `shell`).

---

## 3. Terminal & Console I/O Details

* **Registers:**
  * `$FF00` (`Term.Out`): Write character to console.
  * `$FF01` (`Term.In`): Read character from keyboard (returns 0 if empty).
  * `$FF02` (`Reg.Stat`): Bit 0 = `Timer.Ready`, Bit 1 = `Term.RxReady`.
  * `$FF03` (`Reg.Ctrl`): Bit 0 = `Ctrl.TimrIRQ`, Bit 1 = `Ctrl.TermIRQ`.
* **Line Endings:**
  * TurbOS SCF and `term` descriptor use `C$CR` (`$0D` / `\r`) as the End-Of-Record (`PD.EOR`).
  * In `hatvan-vm`, any incoming newline `\n` (`$0A`) is translated to `\r` (`$0D`) so standard command lines like `mdir\n` are properly terminated and executed by `shell`.
