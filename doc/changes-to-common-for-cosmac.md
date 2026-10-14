# Changes to Hatvan Common and Subsystems for RCA COSMAC CDP1802

This document records all changes made outside of 1802-specific code (including Hatvan OS common kernel code, shared drivers, userspace commands, build automation, and the MiniGolf toolchain) in support of bringing the RCA COSMAC CDP1802 port (`gepc`) to interactive stability.

---

## 1. Hatvan OS Common Kernel (`kernel/common/`)

### 1.1 Parent Process ID Preservation Across `hal.LaunchProcess`
* **Affected Files**:
  * [`kernel/common/proc.golf`](file:///home/strick/github.com/strickyak/hatvan-os/kernel/common/proc.golf) (`SysWait` and `Yield`)
* **Problem & Root Cause**:
  * When a child process terminates, `HandleSyscall` traps into Task 0 with caller PID set to the child PID (`trapPID`, e.g., 4).
  * `proc.SysExit` marks the child as `PROC_ZOMBIE`. When `hal.LaunchProcess` returned to `SysWait`, `CurrentPID` remained set to the child PID (4) rather than the waiting parent (e.g., 3).
  * `SysWait` then executed `reapChild(p, byte(i))`, which called:
    ```golf
    ProcFree(cpid)
    SetProcState(CurrentPID, PROC_RUNNING)
    ```
  * Because `CurrentPID` was 4, `reapChild` freed PID 4 and immediately set `ProcTable[4].State = PROC_RUNNING`, reviving the reaped child process while leaving the parent waiting.
  * Subsequent calls to `SysWait` observed `ProcTable[4]` in `PROC_RUNNING` and re-launched the zombie process in an infinite loop.
* **Fix**:
  * Preserved `CurrentPID` across `hal.LaunchProcess` in both `SysWait` and `Yield`:
    ```golf
    SetProcState(CurrentPID, PROC_WAITING)
    prevPID := CurrentPID
    hal.LaunchProcess(byte(i), p.SavedSP, p.SavedPC)
    CurrentPID = prevPID
    if p.State == PROC_ZOMBIE {
        return reapChild(p, byte(i))
    }
    ```
  * Critically, `CurrentPID` is *not* modified before entering `hal.LaunchProcess`, ensuring compatibility with 6809, 68K, and Z80 assembly implementations that inspect `CurrentPID` on entry to identify the parent process.

### 1.2 Chunked Cross-Task DMA Buffer Transfers
* **Affected Files**:
  * [`kernel/common/dev.golf`](file:///home/strick/github.com/strickyak/hatvan-os/kernel/common/dev.golf) (`TermRead`, `TermWrite`, `LogWrite`)
* **Problem & Root Cause**:
  * Terminal I/O and diagnostic logging previously performed cross-task transfers one byte at a time:
    ```golf
    hal.DMACopy(task, srcAddr + i, 0, word(&c), 1)
    ```
  * Each 1-byte transfer incurred hardware MMU setup and synchronization overhead, causing significant terminal latency during interactive shell operations and directory listings.
* **Fix**:
  * Implemented 64-byte chunked streaming buffers:
    ```golf
    var writeBuf [64]byte
    offset := word(0)
    for offset < count {
        chunk := count - offset
        if chunk > 64 {
            chunk = 64
        }
        hal.DMACopy(task, srcAddr + offset, 0, word(&writeBuf[0]), chunk)
        for j := word(0); j < chunk; j++ {
            hal.PutChar(writeBuf[j])
        }
        offset += chunk
    }
    ```
  * Terminal throughput and responsiveness increased significantly across all userland commands.

### 1.3 Stack Frame Reductions (Eliminating Stack-Local Array Buffers)
* **Affected Files**:
  * [`kernel/common/proc.golf`](file:///home/strick/github.com/strickyak/hatvan-os/kernel/common/proc.golf)
  * [`kernel/common/loader.golf`](file:///home/strick/github.com/strickyak/hatvan-os/kernel/common/loader.golf)
  * [`kernel/common/rbf.golf`](file:///home/strick/github.com/strickyak/hatvan-os/kernel/common/rbf.golf)
  * [`kernel/common/sh.golf`](file:///home/strick/github.com/strickyak/hatvan-os/kernel/common/sh.golf)
* **Problem & Root Cause**:
  * Several functions allocated local array buffers directly on the machine call stack (e.g., `var comp [16]byte`, `var compBuf [32]byte`, `var hdr [13]byte`, `var buf [4]byte`).
  * On architectures or virtual machines with small call stack allocations or byte-offset stack indexing, local arrays caused frame bloat and potential stack collisions.
* **Fix**:
  * Replaced local stack buffers with static package-level variables:
    * `var procComp [16]byte` in `proc.golf`
    * `var os9Hdr [13]byte` in `loader.golf`
    * `var rbfCompBuf [32]byte` in `rbf.golf`
    * `var printDecBuf [4]byte` in `sh.golf`

### 1.4 Unified Syscall Trap Interface (`HandleSyscall`)
* **Affected Files**:
  * [`kernel/common/syscall.golf`](file:///home/strick/github.com/strickyak/hatvan-os/kernel/common/syscall.golf)
* **Details**:
  * Added `HandleSyscall`:
    ```golf
    func HandleSyscall(callNum byte, a byte, b byte, x word, y word, u word, pc word) (byte, byte, word, byte)
    ```
  * Provides a clean architectural bridge from low-level register/frame unpacking into `Dispatch(callNum)`.
  * Detects trap source PID from hardware port (`0x0043` / `PORT_TRAPSRCTASK`), updates `proc.CurrentPID`, and returns `(resA, resB, resY, ccErr)`.

### 1.5 Execution Directory & Diagnostic Logging at Boot
* **Affected Files**:
  * [`kernel/common/main.golf`](file:///home/strick/github.com/strickyak/hatvan-os/kernel/common/main.golf)
  * [`kernel/common/tables.golf`](file:///home/strick/github.com/strickyak/hatvan-os/kernel/common/tables.golf)
* **Details**:
  * Added `hal.ARCH == 'c'` check to switch working directory to `/d0/CmdsC` on boot (joining `/d0/Cmds9`, `/d0/CmdsK`, and `/d0/CmdsZ`).
  * Added 3-digit numeric error logging to stderr upon directory switch failure.
  * Updated shared memory curtain sizing validation in `tables.golf` to enforce limits for `hal.ARCH == 'c'`.

---

## 2. Common Drivers & Userspace Commands

### 2.1 PROCFS Driver Optimizations
* **Affected Files**:
  * [`drivers/procfs/main.golf`](file:///home/strick/github.com/strickyak/hatvan-os/drivers/procfs/main.golf)
* **Details**:
  * Replaced stack-local `var tmp [4]byte` in `bufAppendDec` with static `var decTmp [4]byte`.
  * Replaced `klib.streq(subpath, (*byte)("p"))` on mailbox path string with direct character comparisons (`peekb(subpath) == 'p' && peekb(subpath+1) == 0`).

### 2.2 Stack Buffer Bounds in Userland Commands
* **Affected Files**:
  * [`cmds/gsh.golf`](file:///home/strick/github.com/strickyak/hatvan-os/cmds/gsh.golf)
  * [`cmds/gsh2.golf`](file:///home/strick/github.com/strickyak/hatvan-os/cmds/gsh2.golf)
  * [`cmds/gcat.golf`](file:///home/strick/github.com/strickyak/hatvan-os/cmds/gcat.golf)
  * [`cmds/lib/sys.golf`](file:///home/strick/github.com/strickyak/hatvan-os/cmds/lib/sys.golf)
* **Details**:
  * Scaled 256-byte stack buffers (`itemsStr`, `bodyStr`, `condStr`, `segBuf`, `expBuf`, `contBuf`, `buf`) down to 240 bytes with buffer limit bounds set to 238–239 bytes to prevent single-function stack frames from exceeding 256-byte boundaries.
  * Added `[GSH LINE=...]` trace logging in `gsh` when interactive or script commands are read, enabling reliable expect test synchronization.

---

## 3. Build System & Scripting Integration

### 3.1 Makefile
* **Affected Files**:
  * [`Makefile`](file:///home/strick/github.com/strickyak/hatvan-os/Makefile)
* **Details**:
  * Added definitions and build rules for `PAR4ASM` (`$(BUILD_DIR)/par4asm`) and `PAR4BUNDLE` (`$(BUILD_DIR)/par4bundle`).
  * Added target pattern rules for `$(CMDS_GOLF_C)` (`cmds/*.c.decb`).
  * Updated `make disk` to create `/CmdsC` on `disk0.dsk` and install all 1802 command binaries.
  * Added `-tick-hz=0` to the 1802 VM invocation under `make test`.

### 3.2 Build Command Helper (`build-cmd.sh`)
* **Affected Files**:
  * [`build-cmd.sh`](file:///home/strick/github.com/strickyak/hatvan-os/build-cmd.sh)
* **Details**:
  * Added automated checks and compilation for `par4asm` and `par4bundle`.
  * Added 1802 compilation pipeline compiling `.golf` command source with `minigolf -m par4`, assembling with `par4asm`, and bundling with `par4bundle` into executable `.c.decb` packages.

---

## 4. MiniGolf Toolchain (`../minigolf`)

* **Par4 CLI Tools, Standard Library, and Documentation (Commit `60f95aa`)**:
  * **`cmd/par4asm/main.go`**: Par4 assembler converting assembly `.p4s` text to binary `.p4p` bytecode.
  * **`cmd/par4dis/main.go`**: Par4 bytecode disassembler.
  * **`cmd/par4vmp/main.go`**: Standalone Go reference interpreter for Par4 bytecode.
  * **`doc/par4-language-documentation.md`**: Specification, bytecode format, and opcode definitions for the Par4 VM.
  * **`par4-lib/`**: Standard library modules (`prelude.par4`, `slice.par4`, `smap.par4`, `btree.par4`, `rbtree.par4`, `regexp.par4`).
