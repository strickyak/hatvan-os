# Hatvan OS Design Document: Level 3 Driver Tasks & RBF Microkernel Architecture

**Status:** Proposed / In Progress (Phase 1 Complete)  
**Author:** Antigravity / Pair Programming  
**Date:** September 2026  
**Target Systems:** Motorola 6809 (`gep9`), Hitachi 6309, Motorola 68000 (`gepk`)

---

## 1. Executive Summary

As Hatvan OS matures, the Motorola 6809 kernel in **Task 0** is rapidly exhausting its single 64KB address space. Kernel machine code currently spans from `$4000` to `$D955` (~39.3 KB), leaving approximately **9.6 KB of headroom** before colliding with the `$FF00` Red Page (hardware I/O ports and CPU vectors).

To relieve this pressure and establish a scalable OS architecture, Hatvan OS introduces a **"Level 3" Driver Task architecture**. In this model:
1. Device managers and drivers (starting with the Random Block File manager, **RBF**) are relocated from Task 0 into dedicated, isolated 64KB task address spaces.
2. The filesystem driver runs as **Task 1**, displaying in `/proc/p` as `[RBF]`.
3. A new hardware register, **`TaskFlagsRegister` (`$FF2E` on 6809, `$00FF005C` on 68000)**, allows Task 0 to "bless" Task 1 with I/O privileges, granting it direct access to `$FF00..$FFFF` (disk controllers, DMA engines) without kernel mediation.
4. Coarse-grained filesystem requests (including path resolution and directory traversal) are dispatched from Task 0 to Task 1 via a **Page 0 Mailbox** using `TaskFuse` + `RTI`, and completed via `SWI2; fcb F$Sleep`.
5. Moving `rbf` and `path` resolution to Task 1 recovers **~6–7 KB of kernel code space**, nearly doubling available Task 0 headroom.

---

## 2. Background & Motivation

### 2.1 The Historical Context of OS-9 Levels
In the classic Microware OS-9 ecosystem for the 6809:
- **OS-9 Level 1** (e.g. TRS-80 Color Computer 1 & 2): A flat 64KB system without an MMU. The OS kernel, device drivers, descriptors, system buffers, and all user processes competed for the same 64KB of RAM.
- **OS-9 Level 2** (e.g. Color Computer 3): Used a hardware MMU with 8KB DAT (Dynamic Address Translation) blocks to give each user process its own 64KB logical address space. However, **Task 0 remained monolithic**: the kernel (`krn`), file managers (`rbf`, `scf`), device drivers, system tables, and system buffers all had to coexist within Task 0's single 64KB map.
- **The "Level 3" Concept**: Within the 6809 community in the late 1980s, an informal "Level 3" proposal circulated: What if device managers (like RBF) or network stacks were each decoupled into their own dedicated 64KB tasks, communicating via hardware-accelerated message passing? On period hardware, the MMU granularity and context switch latency made this difficult to complete, and the transition to OS-9/68000 superseded it.

### 2.2 The Memory Challenge in Hatvan OS
In Hatvan OS:
- Each task (0 to 255) possesses an independent 64KB address space.
- User processes (Task $\ge 2$) execute with complete 64KB isolation: read-only binaries abutt the Red Page at `$FEFF`, stacks grow downwards from below the code, and data/BSS grows upwards from `$0000`.
- But **Task 0 contains the monolithic kernel**:
  - `cstart` & Vector Table
  - BSS, Global Tables, Kernel Stack: `$0010..$3FFF` (~16 KB)
  - Compiled Kernel Code: `$4000..$D955` (~39.3 KB)
  - Red Page (Hardware I/O & Vectors): `$FF00..$FFFF`
  - **Headroom Remaining:** `$FF00 - $D955 = $25AB` (~9.6 KB).

As network drivers, additional filesystem features, and peripheral managers are authored, Task 0 will breach `$FF00`.

---

## 3. Architecture & Design

### 3.1 Architecture Overview

```
+-------------------------------------------------------------------------+
|                               Task 0: Kernel                            |
|  - Process Scheduler & Tables (ProcTable, PID allocation)               |
|  - System Call Dispatcher (SWI2)                                        |
|  - Path Descriptor Table (PathTable)                                    |
|  - RBF IPC Client Stub                                                  |
+-------------------------------------------------------------------------+
        |  TaskFuse ($FF20 = 1) + RTI                 ^  SWI2 (F$Sleep)
        |  (Request in Page 0 Mailbox)                |  (Response in Mailbox)
        v                                             |
+-------------------------------------------------------------------------+
|                         Task 1: [RBF] Service                           |
|  - TaskFlagsRegister ($FF2E) = $01 (I/O Blessed)                        |
|  - Page 0 IPC Mailbox ($0080..$00BF)                                    |
|  - Coarse-Grained Operations:                                           |
|      * RBFResolvePath (traverses directories without IPC ping-pong)     |
|      * RBFReadSector / RBFWriteSector                                   |
|      * RBFReadFD / RBFReadSuperblock                                    |
|  - Directly controls Disk ($FF10..$FF17) & DMA ($FF21..$FF27)           |
|  - Transfuses sectors directly to user tasks via DiskMemTask ($FF14)    |
+-------------------------------------------------------------------------+
```

### 3.2 Hardware Capability Model (`TaskFlagsRegister`)

Under the standard Hatvan VM security architecture, any read or write to `$FF00..$FFFF` by a task with `TaskID > 0` causes an immediate `ErrUserAccessTrap` panic.

To permit trusted driver tasks to manage hardware:
- **6809 Register:** `$FF2E` (`TaskFlagsRegister`)
- **68000 Register:** `$00FF005C` (`TaskFlagsRegister`)
- **Semantics:**
  - Writing Bit 0 (`$01`, `TASK_FLAG_IO`) blesses Task 1 with I/O privileges.
  - In `ReadByte` / `WriteByte`, accesses to `$FF00..$FFFF` check:
    ```go
    if addr >= 0xFF00 {
        if b.CurrentTask == 0 || (b.TaskFlags[b.CurrentTask]&0x01) != 0 {
            return b.readIO(addr)
        }
        panic(ErrUserAccessTrap)
    }
    ```
  - **Revocation:** When a task is purged via `PurgeTaskMem` (`$FF2F` / `$00FF005E`), `TaskFlags[task]` is automatically cleared to `$00`, preventing recycled tasks from inheriting stale hardware privileges.

### 3.3 The Coarse-Grained Filesystem Service (Avoiding Chatty RPC)

A crucial architectural decision is the **granularity of the service interface**:
- **Discarded Low-Level Approach (Chatty RPC):**
  If Task 1 only exposed `ReadSector(drive, lsn)` and Task 0 performed directory scanning and path parsing, resolving a path like `/d0/Cmds9/ECHO` would require 15–20 back-and-forth context switches between Task 0 and Task 1.
- **Adopted High-Level Approach (Service-Oriented):**
  Task 1 hosts **both the disk driver and the path resolution logic** (`RBFResolvePath`, `RBFScanDirectory`, `RBFMatchName`).
  Task 0 issues a single IPC request:
  ```
  ResolvePath(pathStr) -> (drive, fdLSN, error)
  ```
  Task 1 handles all intermediate directory sector reads internally (directly reading disk hardware into its own local cache) and returns only the final result to Task 0.

### 3.4 Direct Target-Task Disk Transfers

Because Hatvan’s hardware disk controller possesses the **`DiskMemTask` register (`$FF14`)**, Task 1 does not need to read data into its own RAM and then copy it to the user process via DMA:
1. When a user process executes `I$Read`:
2. Task 0 requests Task 1 to read file LSN $L$ into Task $U$ at address $A$.
3. Task 1 sets:
   - `$FF10 = drive`
   - `$FF11..$FF13 = LSN`
   - `$FF14 = targetTask (U)`
   - `$FF15..$FF16 = targetAddr (A)`
   - `$FF17 = 1 (Read)`
4. The virtual disk controller DMAs the sector **directly into the user task's memory space in a single hardware cycle**.

### 3.5 Inter-Process Communication (IPC) Protocol

#### Page 0 Mailbox Layout
Task 1 reserves `$0080..$00BF` (64 bytes) in Page 0 as a fixed IPC Mailbox:

| Offset | Field | Type | Description |
|---|---|---|---|
| `$0080` | `Cmd` | `uint8` | `1`=ReadSector, `2`=WriteSector, `3`=ResolvePath, `4`=ReadFD, `5`=ReadSuperblock |
| `$0081` | `Drive` | `uint8` | Drive number (`0..3`) |
| `$0082` | `TargetTask` | `uint8` | Target memory task ID for sector data |
| `$0083` | `Status` | `uint8` | Output: `0`=OK, non-zero error code |
| `$0084..$0085` | `TargetAddr` | `uint16` | Target memory address in `TargetTask` |
| `$0086..$0088` | `LSN` | `uint24` | 24-bit Logical Sector Number |
| `$0089..$008A` | `Param` | `uint16` | Auxiliary parameter (cwd/cxd LSN, mode) |
| `$008B..$00AA` | `PathStr` | `[32]byte` | Null-terminated path string / segment |

#### Invocation Sequence
1. **Setup:** Task 0 formats the request and copies it into Task 1's Mailbox via DMA:
   ```asm
   ; Copy 48-byte request to Task 1 at $0080
   clr $FF21          ; srcTask = 0
   ldd #request_buf
   std $FF22          ; srcAddr
   lda #1
   sta $FF24          ; dstTask = 1
   ldd #$0080
   std $FF25          ; dstAddr
   ldb #48
   stb $FF27          ; trigger DMA
   ```
2. **Dispatch:** Task 0 arms TaskFuse and returns from interrupt:
   ```asm
   lda #1
   sta $FF20          ; TaskFuse = Task 1
   rti                ; Switches to Task 1
   ```
3. **Execution:** Task 1 processes the request, writes results to `$0083`, and executes:
   ```asm
   swi2
   fcb $0A            ; F$Sleep
   ```
4. **Resumption:** The hardware vector fetch drops into Task 0's `trap_swi2`. The trap handler identifies `CurrentPID == 1 && call_num == $0A`, extracts the response status from `$0083`, and resumes the kernel caller.

### 3.6 Re-entrant Trap Dispatching

When a user process (PID 2) invokes `I$Read`, the CPU is already in a kernel trap frame. When Task 0 subsequently switches to Task 1, a second nested context transition occurs.

To prevent corruption of user register frames:
- The kernel trap handler maintains an `RBF_Active` flag.
- When a trap occurs with `CurrentPID == 1`, `trap_swi2` bypasses user syscall table dispatching and jumps directly to `rbf_return_handler`:
  ```asm
  trap_swi2:
      clra
      tfr a,dp
      ldb v_proc.CurrentPID
      cmpb #1                 ; Is it Task 1 (RBF)?
      beq .handle_rbf_return
      ; ... standard user syscall handling ...
  ```

---

## 4. Process Table & `/proc/p` Integration

- During `proc.ProcInit()`:
  - Task 1 is pre-allocated:
    - `PID = 1`
    - `PPID = 0` (Parent is kernel)
    - `State = PROC_WAITING`
    - `CmdName = "[RBF]"`
- User applications (such as the interactive root shell `sh`) allocate starting at `PID = 2`.
- Inspection via `cat /proc/p` shows:
  ```
    PID  PPID STATE      SP  CMD
      0     0 RUN   $3800  kernel
      1     0 WAIT  $FE00  [RBF]
      2     0 WAIT  $FE00  sh
      3     2 RUN   $FD00  cat
  ```

---

## 5. Discarded Alternatives

### Alternative 1: Fine-Grained Sector-Level Block RPC
- **Idea:** Keep all path resolution, directory parsing, and file descriptors in Task 0; only move `hal.DiskTransfer` into Task 1.
- **Why Discarded:** Moving only sector transfers saves less than 1 KB in Task 0, while introducing dozens of context switches for every directory walk. It exacerbates IPC overhead without solving memory pressure.

### Alternative 2: MMU Bank-Switching in Task 0
- **Idea:** Introduce a banked memory window (e.g. `$8000..$BFFF`) in Task 0, switching in different code banks for kernel, drivers, and userland.
- **Why Discarded:** Violates Hatvan OS's core architectural principle of simple, flat 64KB task spaces. Requires banking hardware logic, compiler trampolines, and inter-bank call management, adding severe complexity to both the emulator and MiniGolf compiler.

### Alternative 3: Stripping the Kernel Shell to Userland Only
- **Idea:** Leave RBF in Task 0 and simply compile `sh` as a separate userland binary in `/d0/Cmds9/sh`.
- **Why Discarded:** Moving `sh` only frees ~1.5 KB. While desirable as an independent cleanup, it does not provide a scalable foundation for future drivers (SCF, console graphics, sound, networking).

### Alternative 4: Register-Only IPC Calling Convention
- **Idea:** Pass all IPC parameters purely in 6809 registers (`D, X, Y, U`) without a Page 0 Mailbox.
- **Why Discarded:** The 6809 has only four 16-bit registers (D, X, Y, U). Simultaneously passing a 24-bit LSN (3 bytes), drive (1 byte), target task (1 byte), target address (2 bytes), transfer length (2 bytes), and path string pointer (2 bytes) requires 11 bytes. Cramming this into registers requires multiple awkward setup instructions and restricts future extensibility.

---

## 6. Implementation Roadmap

### Phase 1: Virtual Machine Capability Support [COMPLETED]
- [x] Add `TaskFlags [256]byte` to `gep9/bus.go` and `gepk/bus.go`.
- [x] Implement `$FF2E` (`gep9`) and `$00FF005C` (`gepk`) `TaskFlagsRegister`.
- [x] Update `ReadByte` / `WriteByte` to permit `$FF00..$FFFF` access when `TaskFlags[task] & 0x01 != 0`.
- [x] Auto-revoke task capability flags in `PurgeTaskMem` (`$FF2F` / `$00FF005E`).
- [x] Unit test suite in `gep9/cpu_test.go` and `gepk/bus_test.go`.
- [x] Document registers in `doc/spec-hatvan-6309.md` and `doc/spec-hatvan-68000.md`.

### Phase 2: Kernel Trap & IPC Bridge [COMPLETED]
- [x] Implement `f_hal__RBFCall` assembly stub in `kernel/m6809/trap_m6809.asm` and `kernel/m68k/trap_m68k.s`.
- [x] Add re-entrant return dispatching in `trap_swi2` (`SWI2; fcb $0A`) and `trap_0` (`TRAP #0`, D0=10) for `CurrentPID == 1`.
- [x] Define 64-byte Page 0 Mailbox structure in `kernel/common/rbf_ipc.golf` with 32-bit address and LSN support.

### Phase 3: RBF Driver Extraction & Service Task [COMPLETED]
- [x] Author standalone Task 1 service loop in `drivers/rbf/main.golf` and assembly entry stubs (`cstart_rbf_m6809.asm`, `cstart_rbf_m68k.s`).
- [x] Move path resolution and directory traversal routines into Task 1.
- [x] Update `proc.ProcInit` to allocate PID 1 as `[RBF]`, bless Task 1 via `hal.SetTaskFlags(0x01)`, and initialize the driver task.

### Phase 4: Full System Verification [COMPLETED]
- [x] Verify `/proc/p` reports PID 1 as `[RBF]` in state `WAIT`.
- [x] Verify end-to-end file reading, binary launching, and shell commands (`ECHO`, `CAT`) transparently route via Task 1.
- [x] Automated (`make test`) and interactive (`make test-interactive`) test suites pass cleanly on both M6809 (`gep9`) and M68000 (`gepk`).
