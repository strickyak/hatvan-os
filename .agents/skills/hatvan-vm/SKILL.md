---
name: hatvan-vm
description: >-
  Procedures, command-line usage, hardware specifications, and debugging instructions
  for running 6809 and 6309 binaries in the Hatvan Virtual Machine (hatvan-vm).
  Use when building the VM, running test programs or OS kernels (such as TurbOS),
  configuring hardware I/O or disks, analyzing instruction traces, or debugging execution.
---

# Running and Testing with Hatvan VM (`hatvan-vm`)

`hatvan-vm` is a clean-room virtual machine written in Go, implementing the Hitachi 6309 CPU with 256 isolated 64KB task spaces and specialized hardware devices mapped into `$FF00..$FFFF` in Task 0.

## Building and Testing the VM

From the repository root (`/home/strick/github.com/strickyak/hatvan-os`):

```bash
# Run all unit tests
go test -v ./vm/...

# Build the emulator binary
go build -o hatvan-vm ./cmd/hatvan-vm
```

## Running Binaries

The emulator accepts a primary `.decb` file or raw 64KB `.img` system image, and optional `lwasm` `.list` assembly listings for symbolic trace annotation:

```bash
./hatvan-vm [options] <program.decb|system.img> [listing.list ...]
```

### Command-Line Options

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--trace` | `false` | Enables full instruction execution tracing (PC, registers, CC, source line). |
| `--input <str>` | `""` | Pre-enqueues console input string (e.g. `--input="mdir\n"`). `\n` is translated to OS-9 `\r`. |
| `--max-cycles <N>`| `0` | Halts simulation after $N$ CPU cycles (0 = unlimited). Ideal for non-interactive tests. |
| `--max-seconds <N>`| `300` | Halts simulation after $N$ real-time seconds (0 = unlimited). |
| `--tick-hz <N>` | `60` | Frequency in Hz for hardware timer ticks (sets `Timer.Ready` in `$FF02`). Set to `0` to disable. |
| `--cpu-hz <N>` | `2000000` | Simulated CPU clock frequency in Hz (default 2 MHz). Used to scale timer ticks per cycle. |
| `--disk0 <path>` | `""` | Attaches a raw disk image file (256 bytes/sector) to `/d0` (`$FF10 = 0`). |
| `--disk1`..`--disk3` | `""` | Attaches disk images to `/d1`, `/d2`, and `/d3`. |

> [!NOTE]
> When loading a raw 64KB OS-9/TurbOS image (`.img`), `hatvan-vm` automatically scans memory for valid OS-9 module headers. Any `.list` files passed on the command line that match module names (`kernel`, `shell`, `mdir`, etc.) are automatically shifted by the module's in-memory base address for seamless source tracing.

### End-to-End Workflow Example

1. **Assemble a program:**
   ```bash
   lwasm --decb --list=hello.list -o hello.decb hello.asm
   ```

2. **Run without trace (standard output):**
   ```bash
   ./hatvan-vm hello.decb
   ```

3. **Run with symbolic execution trace:**
   ```bash
   ./hatvan-vm --trace --max-cycles=500 hello.decb hello.list
   ```

   **Trace Output Format:**
   ```
   PC=2000  A=00 B=00 X=0000 Y=0000 U=0000 S=0000 CC=[.F.I....]  (8) start    leax msg,pcr
   PC=2004  A=00 B=00 X=2010 Y=0000 U=0000 S=0000 CC=[.F.I....]  (9) loop     lda ,x+
   PC=2006  A=48 B=00 X=2011 Y=0000 U=0000 S=0000 CC=[.F.I....]  (10) beq done
   PC=2008  A=48 B=00 X=2011 Y=0000 U=0000 S=0000 CC=[.F.I....]  (11) sta TermOut
   ```

## Hardware Device Reference (Task 0 at `$FF00..$FFFF`)

* **`$FF00` (`putchar` / `Term.Out`):** Write ASCII character to console standard out.
* **`$FF01` (`getchar` / `Term.In`):** Read ASCII character from console standard in. Non-blocking; returns `0` if empty.
* **`$FF02` (`Reg.Stat` - Turbo9Sim):** Status register.
  * Bit 0 (`$01`): `Timer.Ready` (set on timer tick; write 1 to clear).
  * Bit 1 (`$02`): `Term.RxReady` (set when keyboard character is ready; write 1 to clear).
* **`$FF03` (`Reg.Ctrl` - Turbo9Sim):** Control register.
  * Bit 0 (`$01`): `Ctrl.TimrIRQ` (1 = timer generates CPU IRQ).
  * Bit 1 (`$02`): `Ctrl.TermIRQ` (1 = keyboard input generates CPU IRQ).
* **`$FF04` (`logchar`):** Write ASCII character to debug log file (`os.Stderr`).
* **`$FF10..$FF17` (Disk I/O):**
  * `$FF10`: Drive number (0 to 3).
  * `$FF11..$FF13`: 24-bit sector number (LSN).
  * `$FF14..$FF16`: Target memory address (`Task`, `High`, `Low`).
  * `$FF17`: Write command (`1 = Read`, `2 = Write`); Read status (`0 = Busy`, `1 = OKAY`, `>1 = Error`).
* **`$FF20` (`TaskFuse`):** Write user task number. Inside the subsequent `RTI`, active task switches before pulling registers.
* **`$FF21..$FF27` (DMA Memory Copy):** Fast block transfers between tasks.
  * `$FF21..$FF23`: Source (`Task`, `High`, `Low`).
  * `$FF24..$FF26`: Destination (`Task`, `High`, `Low`).
  * `$FF27`: Write length (`1..255`, or `0 = 256`); Read status (`0 = Busy`, `1 = OKAY`, `>1 = Error / Panic`).
* **`$FFF0..$FFFF` (Vectors):** Interrupt and reset vectors in Task 0 RAM. `$FFFE` contains the Reset vector set by the DECB file.

## Protection Rules
* Any access to `$FF00..$FFFF` in a user task (Task $\ge 1$) triggers an immediate fatal user trap.
* Any read or write to unmapped ports within `$FF00..$FFFF` in Task 0 triggers an unrecoverable kernel panic.
