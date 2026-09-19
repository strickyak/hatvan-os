# Hatvan OS

Hatvan is an operating system for a Virtual Machine based on the Hitachi
6309 8-bit/16-bit CPU with specialized I/O devices in the $FF00 page.

## Hatvan 6809 Virtual Machine (`gep9`)

The `gep9` emulator (formerly Hatvan VM; "gép" means machine in Hungarian, architecture `'9'`) is a CPU with the Hitachi 6309 instruction set
and specialized devices in the $FF00 I/O page when in Task 0.

### Tasks and Memory Protection

Task numbers are uint8_t (0 to 255). There is always a current task number.
The VM boots into Task 0.

Each task number has an independent 64K memory space:
* **Task 0 (Kernel Mode):**
  * Runs the operating system kernel.
  * The top page (`$FF00..$FFFF`) is mapped to memory-mapped I/O devices and 6309 hardware vectors.
  * Any read or write to unmapped or unused portions within `$FF00..$FFFF` triggers an unrecoverable hardware PANIC.
* **Tasks 1..255 (User Mode):**
  * Each user process runs in its own non-zero Task Number, matching its Process ID (PID).
  * It gets its own 64K of RAM.
  * Any read, write, or execute access to the top page (`$FF00..$FFFF`) is strictly forbidden and causes immediate process termination and a core dump.
  * Hardware CPU traps on native 6309 illegal instructions and divide-by-zero errors also cause immediate process termination and a core dump.

### Character IO & Hardware Timer (Turbo9Sim Compatible)

Console character I/O uses non-blocking hardware polling with kernel-level busy-waiting. Addresses `$FF00..$FF03` match the Turbo9Sim architecture:

* `$FF00` : `putchar` (`Term.Out`) : any ASCII byte written here goes to the console
* `$FF01` : `getchar` (`Term.In`) : any read returns the next char typed, or 0 if no character is available.
  * A typed ASCII `NUL` (`0x00`) does not occur on the console.
  * Stalling CPU clock inputs indefinitely while waiting for user input is not viable (especially on real hardware), so the hardware immediately returns 0 when no character is ready.
  * For the `/term` driver, if it reads 0 from the hardware during a blocking read, it busy-waits in a loop until a non-zero character is received.
* `$FF02` : `Reg.Stat` (Read/Write) : Status register:
  * Bit 0 (`$01`): `Timer.Ready` (set when timer tick occurs; write 1 to bit 0 to clear)
  * Bit 1 (`$02`): `Term.RxReady` (set when keyboard character is ready; write 1 to bit 1 to clear)
* `$FF03` : `Reg.Ctrl` (Read/Write) : Control register:
  * Bit 0 (`$01`): `Ctrl.TimrIRQ` (1 = enable 60Hz timer interrupt to CPU IRQ)
  * Bit 1 (`$02`): `Ctrl.TermIRQ` (1 = enable keyboard receive interrupt to CPU IRQ)
* `$FF04` : `logchar` : any ASCII byte written here goes to a log file

### Disk IO

Disk I/O uses a non-blocking hardware interface with kernel-level busy-waiting for completion. All multi-byte numbers are big-endian.

* Sector size is 256 bytes.
* Four attached drives are supported: drive numbers 0 to 3 (mapped to `/d0` .. `/d3`).
* `$FF10` : drive number (0 to 3)
* `$FF1[123]` : Sector Number (24-bit LSN)
* `$FF1[456]` : Memory Address (Task, High, Low)
* `$FF17` : Write Command: 1 = Read, 2 = Write. Writing initiates the operation and sets status to 0 (busy).
* `$FF17` : Read Status:
  * `0` = Busy (operation in progress; kernel busy-waits)
  * `1` = OKAY (completed successfully)
  * `>1` (other) = Error code
  * Reading a non-zero status resets status back to 0.

### Task Fuse & Trap Mechanics

* **Entering Kernel Mode (Exceptions / Traps / SWI2):**
  * On an interrupt or SWI2 instruction in user mode, the 6309 pushes CPU registers (`PC, U, Y, X, DP, B, A, CC`, and native `E, F`) onto the stack in the user process space.
  * During the cycle when vector fetch occurs `(BA,BS) == (0,1)`, the current task number is switched to 0.
  * The CPU fetches the interrupt/trap vector from Task 0's vector table at `$FFF0..$FFFF`.
  * The kernel in Task 0 maintains its own private kernel stack.
  * The kernel reads and writes the user's saved register frame and syscall arguments using the fast DMA memory copy engine.

* **Returning to User Mode (TaskFuse & RTI):**
  * `$FF20` : TaskFuse : When a byte is written to `$FF20`, the target task number is latched into the fuse.
  * Inside the subsequent `RTI` instruction, the VM switches the active task to the latched user task number *before* pulling registers, so `RTI` loads the registers directly from the user process stack space.

### Memory Copies

A hardware DMA copy engine enables cross-task and intra-task block transfers:

* `$FF2[123]` : Source Address (Task, High, Low)
* `$FF2[456]` : Destination Address (Task, High, Low)
* `$FF27` : Write Command: Copy-bytes length: number of bytes (1 to 255, or 0 means 256) to copy. Writing initiates the copy and sets status to 0 (busy).
* `$FF27` : Read Status:
  * `0` = Busy (copy in progress; kernel busy-waits)
  * `1` = OKAY (completed successfully)
  * `>1` (other) = Error (causes an unrecoverable kernel PANIC)
  * Reading a non-zero status resets status back to 0.

* `$FF2E` : `TaskFlagsRegister` : Sets runtime capability flags for Task 1. Writing Bit 0 (`$01`) blesses Task 1 with I/O privileges, allowing it to directly access the hardware I/O page (`$FF00..$FFFF`) without triggering a user protection trap (`ErrUserAccessTrap`). Reading returns the current flags for Task 1.
* `$FF2F` : `PurgeTaskMem` : Writing a non-zero task number zeroes that task's entire 64K memory space (or frees it in sparse VM implementations) and clears any runtime task capability flags (revoking I/O privileges). Writing 0 is ignored (Task 0 kernel memory is protected).

This engine is used by the kernel to inspect the post-SWI2 syscall byte in user code, pass buffers between user space and kernel space, and access the user register frame.

## Hatvan OS Features

* **Binary compatibility** for user programs with OS-9 for the 6809 and NitrOS-9 operating systems, as long as they only use supported features. Basic09 should be made to work.
* **OS-9 Filesystem:** Supports OS-9 RBF (Random Block File) filesystem format on four attached disk drives (`/d0` through `/d3`).
* **Unified byte-oriented I/O**, as in Unix:
  * **Binary vs. Line-oriented I/O:**
    * Binary mode calls (`I$Read` and `I$Write`) perform exact, uninterpreted byte transfers with no ambiguity, translation, or substitution.
    * Line-oriented calls (`I$ReadLn` and `I$WritLn`) interpret line endings according to a per-process state variable (e.g. defaulting to `\n`, but allowing `\r` for legacy OS-9 programs such as Basic09). Input accepts either line ending whenever possible.
* **Device-prefixed paths:** File paths begin with the name of the disk or terminal device (e.g. `/d0/...`, `/term`), as in OS-9.
* **Standard Paths:** Per-process Stdin (0), Stdout (1), and Stderr (2), plus stdlog (path 3).
* **User Process Layout (64K):**
  * Read-only Binary program loaded in high memory (below `$FF00`).
  * Data/BSS starts at 0 and grows upwards.
  * Command parameters and stack start high (below the binary) and stack grows downwards.
  * 256-byte page granularity for memory regions.
* **Process Lifecycle:**
  * Uses `F$Fork` and `F$Chain` from OS-9.
  * `F$Fork` launches a child process in a new Task/PID. Execution is synchronous: the parent is suspended until the child terminates (e.g. calls `F$Exit`). Consequently, `F$Wait` always returns immediately with the dead child's exit status.
  * `F$Chain` replaces the running process image in the current task.
* **Recursive Shell**, as in Unix (synchronously executing child commands).

### Hatvan OS Non-Features (For Now)

* No async background processes: no concurrent fork, wait, kill.
* No pipes.
* No user IDs. No file permissions.
* No walltime. No file timestamps.
* Simple I/O: no ioctls, GetStat, or SetStat calls for now.
* No dynamic filesystem mounting/unmounting (fixed 4 attached OS-9 drives).
* No `brk` or dynamic memory allocation syscalls, because a process already has a full 64K space.

### Hatvan OS System Calls

For traps from User Mode into Kernel Mode, Hatvan uses a subset of the user-mode system calls of the NitrOS-9 kernel.
Calls use `SWI2` followed immediately by a single inline byte identifying the system call number (`fcb <call_num>`).
Registers are passed and returned in CPU registers. The kernel fetches the inline syscall number and copies memory buffers across address spaces using the DMA copy engine.

Hatvan OS does not use in-kernel traps, like OS-9 does. Because there are no modules in the kernel—everything is compiled together and whole-program optimized—no in-kernel traps are needed.

## Extended DECB Binary Format (`.decb`)

`gep9` and associated tools use an extended version of Radio Shack's Color Computer DECB binary format. All chunks strictly follow the uniform 5-byte header convention:

```
[Type: uint8] [Length: uint16 big-endian] [Address/Arg: uint16 big-endian] [Payload: Length bytes]
```

Any parser encountering an unrecognized chunk `Type` can safely skip `Length` bytes to proceed to the next chunk.

### Chunk Types

* **`$00` : `DECB_DATA` (Standard Binary Data):**
  * `Address`: 16-bit destination load address in memory.
  * `Length`: Number of binary data bytes ($N$).
  * `Payload`: Raw $N$ bytes of machine code / data loaded into RAM.

* **`$FF` : `DECB_EXEC` (Execution Entry Point):**
  * `Address`: 16-bit execution start address (becomes the Reset Vector).
  * `Length`: `00 00` (no payload).

* **`$01` : `DECB_SRC_ABS` (Absolute Source Line):**
  * `Address`: 16-bit absolute PC address.
  * `Payload`: `[LineNum: uint16] [SourceText: UTF-8 string]`. Attaches the assembly source line text to an absolute address.

* **`$02` : `DECB_MOD_DEF` (OS-9 Module Declaration):**
  * `Address`: 16-bit module size ($M$).
  * `Payload`: `[CRC: 3 bytes] [ExecOffset: uint16] [Name: UTF-8 string]`. Defines the active OS-9 module context (matching Borges `<name>.<size><crc>` identification) for subsequent module-relative chunks.

* **`$03` : `DECB_SRC_REL` (OS-9 Module-Relative Source Line):**
  * `Address`: 16-bit offset from start of the active OS-9 module (`0` to `M-1`).
  * `Payload`: `[LineNum: uint16] [SourceText: UTF-8 string]`. Attaches the assembly source line to an offset within the most recently declared module.

* **`$04` : `DECB_SYM_ABS` (Absolute Symbol):**
  * `Address`: 16-bit absolute address.
  * `Payload`: `[SymbolName: UTF-8 string]`.

* **`$05` : `DECB_SYM_REL` (Module-Relative Symbol):**
  * `Address`: 16-bit offset within active OS-9 module.
  * `Payload`: `[SymbolName: UTF-8 string]`.

* **`$06` : `DECB_SRC_FILE` (Source File Declaration):**
  * `Address`: 16-bit File ID ($1, 2, \dots$).
  * `Payload`: `[FilePath: UTF-8 string]`.

## Open Questions and Postponed Decisions

1. **OS-9 Module Structure:**
   * Traditional OS-9 executables are self-contained modules (`$87 $CD` header, name, CRC, exec offset, data storage).
   * It is currently undecided whether Hatvan will require this module structure, make it optional, or discard it in favor of flat binaries.
   * Basic09 relies on `F$Link` and `F$Load` to find runtime components (`RunB`, `InSys`, `SysLib`) and procedure modules. How Basic09 procedure linking will be supported without full OS-9 module management remains to be resolved.

2. **Line Endings Configuration (`\n` vs. `\r`):**
   * Binary I/O (`I$Read` / `I$Write`) is strictly uninterpreted raw bytes.
   * Line-oriented I/O (`I$ReadLn` / `I$WritLn`) uses the per-process state variable.
   * Details to finalize: the default setting for newly spawned processes, how a process toggles or queries this state (e.g., environment variable, header flag, or lightweight SetStat option), and exact line-termination parsing rules on input.

3. **Process Arguments & Calling Conventions:**
   * OS-9 programs expect register `U` pointing to data storage, `X` pointing to a CR-terminated parameter string, and `Y` containing the parameter length.
   * Unix programs expect `argc` and `argv[]` pointer arrays on the stack.
   * Determine whether Hatvan should populate both conventions on process startup.

4. **I/O and DMA Enhancements (Interrupt-driven vs. Polling):**
   * Hardware console, disk I/O, and DMA memory copies currently rely on non-blocking hardware interfaces with kernel-level busy-waiting for non-zero status.
   * Interrupt-driven I/O or voluntary task yielding while waiting on disk, console, or DMA is postponed until concurrent/background multitasking is considered.

5. **Additional Filesystems and Mount Management:**
   * Mount and unmount operations are postponed; system starts with fixed `/d0`..`/d3` OS-9 RBF drives.
   * Support for other filesystems (e.g., FAT, Unix v6/v7) may be explored later.
