# Hatvan OS Project TODO & Architectural Roadmap

This document captures prioritized feature candidates and future architectural milestones for Hatvan OS (spanning both Motorola 6809 on `gep9` and Motorola 68000 on `gepk`).

---

## Completed Milestones

### Option 3: Full RBF File Writing & File Creation (Completed)
- **Achievements:**
  - Implemented allocation bitmap search, dynamic cluster allocation (`RBFAllocClusters`), and cluster deallocation (`RBFFreeClusters`) in `kernel/common/rbf.golf`.
  - Implemented directory entry insertion with automatic entry reuse and directory growth (`RBFAddDirEntry`), file creation (`RBFCreateFile`), file descriptor serialization (`RBFWriteFD`), and file deletion (`RBFDeleteFile`).
  - Implemented multi-sector and partial-sector file writing (`RBFWriteFile`) with sector caching and read-modify-write preservation of unaligned sector fragments.
  - Added Task 1 RBF command dispatchers for `RBF_CMD_CREATE_FILE` (8), `RBF_CMD_WRITE_FILE` (9), and `RBF_CMD_DELETE_FILE` (10) in `drivers/rbf/main.golf`.
  - Added kernel client IPC wrappers in `kernel/common/rbf_ipc.golf` and wired `path.SysCreate`, `path.SysWrite`, and `path.SysDelete` in `kernel/common/path.golf`.
  - Added OS-9 `I$Create` ($85) and `I$Delete` ($87) system calls to `kernel/common/syscall.golf` and `proc.golf`.
  - Enhanced the shell (`sh.RunCommand`) with output redirection (`> filename`), redirecting process stdout to newly created files.
  - Verified on both Motorola 6809 (`gep9`) and Motorola 68000 (`gepk`) across unit tests (`make test`) and interactive tests (`make test-interactive`).

---

## Active & Candidate Milestones

### Option 1: Move Shell (`sh`) from Kernel to User Space
- **Motivation:** `kernel/common/sh.golf` is compiled directly into the Task 0 kernel binary (~1.5 KB).
- **Goals:**
  - Compile `sh` into standalone executables: `/d0/Cmds9/SH` (OS-9 module) and `/d0/CmdsK/SH` (DECB32 binary).
  - During kernel boot in `main.golf`, after initializing `[RBF]` in Task 1, Task 0 executes `proc.SysFork("/d0/Cmds9/SH", ...)` as **PID 2**.
  - Task 0 becomes a pure microkernel (solely handling system call dispatching, task switching, and IPC), reclaiming 1.5–2 KB in Task 0.

### Option 2: Level 3 SCF Driver Task (Task 2: `[SCF]`)
- **Motivation:** Just as RBF was moved to Task 1, the Level 3 OS-9 vision moves character devices (Sequential Character File manager — terminal I/O, serial, line discipline) into its own dedicated driver task.
- **Goals:**
  - Create `drivers/scf/` running in Task 2.
  - Bless Task 2 via `hal.SetTaskFlags(2, 0x01)` for access to terminal registers (`$FF00..$FF07` / `$00FF0000..$00FF000E`).
  - Move terminal buffering, carriage-return/linefeed mapping, and echo logic into Task 2.
  - `/proc/p` will show:
    ```
      PID  PPID STATE      SP  CMD
        0     0 RUN   $3800  kernel
        1     0 WAIT  $FE00  [RBF]
        2     0 WAIT  $FE00  [SCF]
        3     0 WAIT  $FE00  sh
    ```

### Option 4: Timer-Driven Preemptive Multitasking
- **Motivation:** Hatvan OS currently uses a synchronous fork/wait model where parent processes block until the child calls `F$Exit`.
- **Goals:**
  - Hook the 60 Hz timer interrupt (`$FF02` / Level 6 autovector) in `trap_m6809.asm` and `trap_m68k.s`.
  - Implement round-robin scheduling across all processes in state `PROC_RUNNING` in `ProcTable`.
  - Add background execution support in the shell (e.g. `cmd &`).

### Option 5: OS-9 Level 1 Binary Compatibility Expansion
- **Motivation:** `/d0/Cmds9` has been populated with Level 1 OS-9 binaries from `os9-6809-level1.zip` (`asm`, `basic09`, `edit`, `list`, `ident`, `dump`, `merge`, etc.).
- **Goals:**
  - Test executing stock Level 1 binaries and audit missing system calls (`F$Link`, `F$Load`, `F$Mem`, `F$SPrior`, `I$GetStt`, `I$SetStt`).
  - Add stubs or implementations in `kernel/common/syscall.golf` to allow standard OS-9 tools to run unmodified.
