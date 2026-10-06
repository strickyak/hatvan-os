# Hatvan Unified Hardware I/O Architecture Specification

**Architecture Specification for Hatvan Operating System Virtual Machines & Discrete Hardware**  
*Covers Motorola 6809 / Hitachi 6309 (`gep9`), Zilog Z80 (`gepz`), and Motorola 68000 (`gepk`)*

---

## 1. Overview & Principles

Hatvan OS provides a unified, cross-architecture hardware I/O and MMU abstraction across both 8-bit small-memory processors and 16/32-bit large-memory processors.

The peripheral and MMU design adheres to the following core principles:
1. **Identical Functional Semantics**: Console UART, system timer, disk controller, DMA block transfer engine, MMU task-fuse mechanism, and task capability security flags operate identically across all CPU architectures.
2. **Deterministic Driver Tasks (Level 3 Architecture)**: Trusted userland service tasks (such as RBF filesystem on Task 1 and PROCFS on Task 2) can be "blessed" by the kernel with direct I/O privileges, allowing them to communicate directly with hardware controllers without kernel context-switch mediation.
3. **Natural Architectural Alignment**:
   - **Small-Memory Architectures (16-bit address space, e.g. 6809, Z80)**: Place the 256-byte I/O page at the top of memory: **`$FF00..$FFFF`**. Registers are tightly packed single-byte or 16-bit big-endian fields.
   - **Small-Memory Architectures (16-bit address space, e.g. 6809, Z80)**: Place the 256-byte I/O page at the top of memory: **`$FF00..$FFFF`**. Registers are tightly packed single-byte or 16-bit big-endian fields.
   - **Large-Memory Architectures (24-bit / 32-bit address space, e.g. 68000)**: Place the I/O block at the top of physical memory: **`$00FF0000..$00FFFFFF`**. Every 8-bit port at `$FF00 + offset` on 6809/Z80 maps deterministically to a 16-byte slot at:
     $$\text{Port}_{\text{68k}} = \$00\text{FF}0000 + 16 \times \text{offset} = \$00\text{FF0}xx0$$
     where $\text{offset} = \text{Port}_{\text{6809}} - \$FF00$. This 16-byte slot allocation ensures inherent 16-bit and 32-bit natural alignment, allows single-byte fields to expand to 32 bits, and provides dedicated command/status offsets without register crowding.

---

## 2. Master I/O Register Comparison Table

| Register Name | 6809 (`gep9`) | Z80 (`gepz`) | 68000 (`gepk`) | Width (68k) | R/W | Function / Description |
| :--- | :---: | :---: | :---: | :---: | :---: | :--- |
| **Console & System Control** | | | | | | |
| `Term.Out` | `$FF00` | `$FF00` | `$00FF0000` | Byte / Word | W | Console putchar (write ASCII character to stdout) |
| `Term.In` | `$FF01` | `$FF01` | `$00FF0010` | Byte / Word | R | Console getchar (read non-blocking ASCII character) |
| `Reg.Stat` | `$FF02` | `$FF02` | `$00FF0020` | 16-bit Word | R/W | UART/Timer Status: Bit 0 = `Timer.Ready`, Bit 1 = `Term.RxReady` |
| `Reg.Ctrl` | `$FF03` | `$FF03` | `$00FF0030` | 16-bit Word | R/W | IRQ Control: Bit 0 = `TimrIRQ`, Bit 1 = `TermIRQ` |
| `logchar` | `$FF04` | `$FF04` | `$00FF0040` | Byte / Word | W | Diagnostic log output (emitted to stderr / debug trace) |
| `Exit.Code` | `$FF05` | `$FF05` | `$00FF0050` | 16-bit Word | W/R | VM halt / termination exit code |
| **Disk Subsystem** | | | | | | |
| `Disk.Drive` | `$FF10` | `$FF10` | `$00FF0100` | Byte / Word | R/W | Drive unit selector (`0..3` for `/d0` through `/d3`) |
| `Disk.Sector` | `$FF11..$FF13` | `$FF11..$FF13` | `$00FF0110` | 32-bit Long | R/W | Logical Sector Number (LSN, 24-bit on 8-bit, 32-bit on 68k) |
| `Disk.Task` | `$FF14` | `$FF14` | `$00FF0140` | Byte / Word | R/W | Target Task ID for DMA sector transfer (`0..255`) |
| `Disk.Addr` | `$FF15..$FF16` | `$FF15..$FF16` | `$00FF0150` | 32-bit Long | R/W | Buffer pointer in `Disk.Task` (16-bit on 8-bit, 32-bit on 68k) |
| `Disk.CmdSt` | `$FF17` | `$FF17` | `$00FF0170` | Byte / Word | R/W | Command / Status (Write: 1=Read, 2=Write; Read: 0=Busy, 1=OK, >1=Err) |
| **MMU & Fast DMA Engine** | | | | | | |
| `TaskFuse` / `Task.Active` | `$FF20` | `$FF20` | `$00FF0200` | Byte / Word | R/W | Armed Task Switch fuse / Active User Task selector |
| `DMA.SrcTask` | `$FF21` | `$FF21` | `$00FF0210` | Byte / Word | R/W | DMA Source Task ID (`0..255`) |
| `DMA.SrcAddr` | `$FF22..$FF23` | `$FF22..$FF23` | `$00FF0220` | 32-bit Long | R/W | DMA Source Memory Address (16-bit on 8-bit, 32-bit on 68k) |
| `DMA.DstTask` | `$FF24` | `$FF24` | `$00FF0240` | Byte / Word | R/W | DMA Destination Task ID (`0..255`) |
| `DMA.DstAddr` | `$FF25..$FF26` | `$FF25..$FF26` | `$00FF0250` | 32-bit Long | R/W | DMA Destination Memory Address (16-bit on 8-bit, 32-bit on 68k) |
| `DMA.Count` | `$FF27` | `$FF27` | `$00FF0270` | 32-bit Long | R/W | Byte Count (`1..255`, 0=256 on 8-bit; up to 16 MB on 68k) |
| `DMA.CmdSt` | *(in `$FF27`)* | *(in `$FF27`)* | `$00FF0274` | Byte / Word | R/W | DMA Command / Status (1=Start; Read: 0=Busy, 1=OK, >1=Err) |
| `SharedMemoryCurtain` | `$FF28..$FF29` | `$FF28..$FF29` | `$00FF0280` | 32-bit Long | R/W | Shared memory boundary (`$E000` on 8-bit, `$00FE0000` on 68k) |
| **Task Capability Flags** | | | | | | |
| `TaskFlagsTarget` | `$FF2D` | `$FF2D` | `$00FF02D0` | Byte / Word | R/W | Target task selector (`0..255`) for capability configuration |
| `TaskFlagsRegister` | `$FF2E` | `$FF2E` | `$00FF02E0` | Byte / Word | R/W | Capability flags for selected task: Bit 0 = `TASK_FLAG_IO` |
| `PurgeTaskMem` | `$FF2F` | `$FF2F` | `$00FF02F0` | Byte / Word | W | Zeroes/frees task RAM and clears capability flags |

---

## 3. Subsystem Specifications

### 3.1 Character Console & 60Hz Timer
* **Console I/O**:
  * Writing to `Term.Out` outputs an ASCII byte to standard output.
  * Reading from `Term.In` retrieves the next pending character from standard input (non-blocking). If no input is ready, it returns `0`.
* **Status & Interrupt Control (`Reg.Stat` / `Reg.Ctrl`)**:
  * `Reg.Stat` Bit 0 (`$01`): `Timer.Ready` — asserts on every 60Hz timer tick. Writing `1` clears the flag.
  * `Reg.Stat` Bit 1 (`$02`): `Term.RxReady` — asserts when keyboard/terminal input is ready. Writing `1` clears the flag.
  * `Reg.Ctrl` Bit 0 (`$01`): `TimrIRQ` — enables timer interrupts.
  * `Reg.Ctrl` Bit 1 (`$02`): `TermIRQ` — enables terminal input interrupts.
* **Diagnostics & VM Exit**:
  * Writing an ASCII character to `logchar` routes output to the emulator's diagnostic trace log.
  * Writing an exit code to `Exit.Code` halts the virtual machine and returns the status to the host operating system.

### 3.2 Disk Subsystem
* Supports up to 4 virtual disk drives (`/d0` through `/d3`).
* Sector size: **512 bytes** (or 256 bytes for legacy OS-9 RBF images).
* Direct Task-to-Task Transfers:
  The hardware disk controller features a target task selector (`Disk.Task`) and buffer address pointer (`Disk.Addr`). When a user process requests a read, the driver task (Task 1) sets `Disk.Task` to the user's task ID and `Disk.Addr` to the user's buffer, streaming sector data directly into user RAM without intermediate kernel buffering.
* `Disk.CmdSt`:
  * Write `1`: Read sector from disk into `[Disk.Task : Disk.Addr]`.
  * Write `2`: Write sector from `[Disk.Task : Disk.Addr]` to disk.
  * Read: `0` = Busy, `1` = OKAY, `>1` = Error code. Reading non-zero status automatically clears the register to `0`.

### 3.3 Fast DMA Block Transfer Engine
* Enables rapid cross-task and intra-task block transfers without software copy loops.
* Source buffer is addressed by `[DMA.SrcTask : DMA.SrcAddr]`.
* Destination buffer is addressed by `[DMA.DstTask : DMA.DstAddr]`.
* On 8-bit machines, writing `DMA.Count` (`$FF27`) immediately initiates the copy and sets status. On 68000, `DMA.Count` is a 32-bit register at `$00FF0270` and writing `1` to `DMA.CmdSt` (`$00FF0274`) initiates the transfer.

### 3.4 Shared Memory Curtain
* Tasks 0 (Kernel), 1 (RBF Driver), and 2 (PROCFS Driver) share memory above the curtain address:
  - Small architectures: `$E000..$FEFF` (default boundary `$E000`).
  - 68000: `$00FE0000..$00FEFFFF` (default boundary `$00FE0000`).
* Reads and writes above the curtain by Tasks 1 and 2 directly alias Task 0 memory, providing zero-copy IPC, shared process tables, and shared path descriptors.
* User tasks (Task $\ge 3$) cannot access memory across the curtain; their entire address space remains strictly isolated.

### 3.5 MMU Task Capability Flags & Security Architecture
* By default, any attempt by a user task (`TaskID > 0`) to access the hardware I/O page triggers an immediate fatal protection trap (`ErrUserAccessTrap` on 8-bit, hardware `/BERR` on 68000).
* **Two-Register Configuration Protocol**:
  1. The kernel writes the target Task ID (`0..255`) to `TaskFlagsTarget` (`$FF2D` / `$00FF02D0`).
  2. The kernel writes the capability byte to `TaskFlagsRegister` (`$FF2E` / `$00FF02E0`).
* **Capability Flags**:
  * **Bit 0 (`$01`, `TASK_FLAG_IO`)**: Blesses the selected task with I/O privileges, granting it direct access to device registers without kernel mediation.
  * Bits 1..7: Reserved for future capabilities (e.g. raw disk vs filesystem, direct DMA).
* **Automatic Privilege Revocation**:
  When a task exits or is terminated, writing its Task ID to `PurgeTaskMem` (`$FF2F` / `$00FF02F0`) zeroes its memory and automatically resets `TaskFlags[task] = 0`, preventing recycled tasks from inheriting stale hardware privileges.

---

## 4. Architecture-Specific Differences for Large-Memory Architectures (68000)

While functional semantics are identical, the Motorola 68000 (`gepk`) differs from the 8-bit implementations in the following specific ways:

### 1. I/O Block Relocation to 24-Bit / 32-Bit Top Page
* **6809 / Z80**: The I/O block occupies the top 256 bytes of the 16-bit address space: **`$FF00..$FFFF`**.
* **68000**: The I/O block occupies the top 64 KB page of the 24-bit physical address space: **`$00FF0000..$00FFFFFF`** (specifically `$00FF0000..$00FF02FF`).

### 2. 16-Byte Slot Allocation and Word Alignment Constraints
* The 68000 hardware architecture requires all 16-bit word and 32-bit longword memory transfers to occur on **even byte addresses** (`A0 = 0`). Attempting an unaligned word/long access generates a fatal hardware **Address Error exception**.
* To provide ample room for 16-bit and 32-bit expanded registers without address crowding, each 1-byte register offset on 6809 (`$FF00 + offset`) expands to a dedicated **16-byte slot** on 68000:
  $$\text{Port}_{\text{68k}} = \$00\text{FF}0000 + 16 \times \text{offset} = \$00\text{FF0}xx0$$
  * `$00FF0000`: `Term.Out`
  * `$00FF0010`: `Term.In`
  * `$00FF0020`: `Reg.Stat`
  * `$00FF0030`: `Reg.Ctrl`
  * `$00FF0040`: `logchar`
  * `$00FF0050`: `Exit.Code`
  * `$00FF0100`: `Disk.Drive`
  * `$00FF0110`: `Disk.Sector`
  * `$00FF0140`: `Disk.Task`
  * `$00FF0150`: `Disk.Addr`
  * `$00FF0170`: `Disk.CmdSt`
  * `$00FF0200`: `TaskFuse` / `Task.Active`
  * `$00FF0210`: `DMA.SrcTask`
  * `$00FF0220`: `DMA.SrcAddr`
  * `$00FF0240`: `DMA.DstTask`
  * `$00FF0250`: `DMA.DstAddr`
  * `$00FF0270`: `DMA.Count` (`$00FF0270`) & `DMA.CmdSt` (`$00FF0274`)
  * `$00FF0280`: `SharedMemoryCurtain`
  * `$00FF02D0`: `TaskFlagsTarget`
  * `$00FF02E0`: `TaskFlagsRegister`
  * `$00FF02F0`: `PurgeTaskMem`

### 3. 32-Bit Pointers and Counters
* **`Disk.Sector`**:
  * 6809 / Z80: 24-bit LSN represented across 3 consecutive bytes (`$FF11..$FF13`), addressing up to 8 GB (16M $\times$ 512B).
  * 68000: Full 32-bit LSN longword at `$00FF0110`, addressing up to 2 TB.
* **`Disk.Addr`**:
  * 6809 / Z80: 16-bit memory pointer (`$FF15..$FF16`) in target task space.
  * 68000: Full 32-bit memory pointer longword at `$00FF0150`.
* **DMA Addressing & Transfer Count**:
  * 6809 / Z80: `DMA.SrcAddr` and `DMA.DstAddr` are 16-bit; `DMA.Count` is an 8-bit count (`$FF27`, where 0 represents 256 bytes).
  * 68000: `DMA.SrcAddr` (`$00FF0220`) and `DMA.DstAddr` (`$00FF0250`) are 32-bit longwords; `DMA.Count` (`$00FF0270`) is a 32-bit longword capable of copying entire multi-megabyte address spaces in a single operation.
* **Dedicated DMA Command Register (`DMA.CmdSt`)**:
  * 6809 / Z80: Writing `$FF27` sets length and simultaneously triggers the DMA transfer.
  * 68000: `DMA.Count` at `$00FF0270` is separated from `DMA.CmdSt` at `$00FF0274`. Writing `1` to `DMA.CmdSt` initiates the transfer.
* **`SharedMemoryCurtain`**:
  * 6809 / Z80: 16-bit boundary (`$FF28..$FF29`, defaulting to `$E000`).
  * 68000: 32-bit boundary (`$00FF0280`, defaulting to `$00FE0000`).

### 4. Hardware Privilege Model & Bus Error (`/BERR`) Assertion
* On the 68000, CPU privilege state is explicitly signaled via function code pins:
  * `FC2 = 1`: Supervisor Mode.
  * `FC2 = 0`: User Mode.
* When `FC2 == 0` (User Mode), any access to `$00FF0000..$00FFFFFF` by an unblessed task asserts the hardware `/BERR` line, generating a fatal 68000 Bus Error exception (Vector 2).
* On 6809 and Z80, user mode is tracked by software MMU task registers (`CurrentTask > 0`), triggering an `ErrUserAccessTrap`.

### 5. Interrupt Vectoring: Direct Autovectors vs Polled IRQ
* **68000 (Direct Autovectors)**:
  * Level 6 (`$00000078`, Autovector 30): 60Hz System Timer tick. Direct vector to clock scheduler with highest priority; requires no status polling.
  * Level 4 (`$00000070`, Autovector 28): Console UART receive ready. Direct vector to input handler.
* **6809**: Single maskable `IRQ` (vector at `$FFF8`) or `FIRQ` (`$FFF6`). ISR polls `Reg.Stat` to distinguish timer from keyboard.
* **Z80**: Single maskable `INT` using Interrupt Mode 1 (`IM 1`, vector at `$0038`). ISR polls `Reg.Stat` to distinguish interrupt causes.

### 6. Task Switch Execution Trigger
* **68000**: Armed `TaskFuse` triggers on the execution of the **`RTE`** (Return from Exception) instruction.
* **6809**: Armed `TaskFuse` triggers on the execution of the **`RTI`** (Return from Interrupt) instruction.
* **Z80**: Armed `TaskFuse` triggers on the execution of the **`RETN`** (Return from NMI) instruction.

---

## 5. Architecture-Specific Differences for 6809 and Z80 Outside `$FF00..$FF2F`

1. **Z80 Syscall Port 60h Trap Latch (`$FF60`)**:
   - The Z80 lacks a native syscall instruction with inline argument bytes like 6809's `SWI2 <call_num>`.
   - Instead, Z80 user code executes `OUT (0x60), A; DEFB <call_num>`. Port `0x60` latches the accumulator and triggers an immediate Non-Maskable Interrupt (`NMI`) entering Task 0.
   - In `gepz`, memory address **`$FF60`** is mapped to read this latched syscall value.
2. **6809 Vintage NitrOS-9 EMUDSK Compatibility (`$FF80..$FF86`)**:
   - `gep9` contains optional hardware decoding for the legacy NitrOS-9 `flat65280v2` / `deep65280v2` emulator disk registers at `$FF80..$FF86` to run vintage OS-9 disks.
   - `gepz` and `gepk` are pure Hatvan systems and omit this legacy peripheral.
3. **Hardware Interrupt Vector Locations**:
   - **6809**: High memory: `$FFF0..$FFFF` (stored in Task 0 RAM).
   - **Z80**: Low memory: `$0000` (Reset), `$0038` (IM 1 IRQ), `$0066` (NMI).
   - **68000**: Vector table in Task 0 low RAM: `$00000000..$000003FF` (Vectors 0..255).
