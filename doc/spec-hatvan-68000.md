# Hatvan OS/K & Hatvan VM/K Specification

**Architecture Specification for Hatvan 68000 Virtual Machine (`gepk`) and Physical Hardware Target**

---

## 1. Overview & Vision

**Hatvan OS/K** (`hatvan-osk`) is a multi-tasking, modular operating system designed for the **Motorola 68000** 16/32-bit microprocessor. It scales the architecture of 8-bit Hatvan OS (Hitachi 6309 / 6809 on `gep9`) into the 32-bit era, combining the modularity of OS-9/68000 and NitrOS-9 with clean Unix-like process isolation. (*"gép" is "machine" in Hungarian, corresponding to kernel `ARCH` constants `'9'` and `'k'`.*)

This specification describes the **Hatvan 68000 Machine** (`gepk`) from two complementary perspectives:
1. **The Software Emulator (`gepk`, formerly `hatvan-vmk`)**: A clean-room, dependency-free virtual machine implemented in the Go programming language, supporting rapid debugging, execution tracing, symbolic listing alignment, and scripted testing.
2. **The Physical Hardware Implementation**: A discrete hardware design based on a physical Motorola 68000 (or 68010) CPU, supported by a CPLD/FPGA memory controller, static RAM, dual-port or DMA bus mastering, and hardware UART/timer peripherals.

---

## 2. CPU Architecture & Execution Model

The system is centered around the standard Motorola 68000 processor (with full forward compatibility for the 68010 and 68020+):

* **Data Bus**: 16-bit bidirectional bus (`D0..D15`).
* **Address Bus**: 24-bit physical addressing (`A1..A23` + `/UDS` and `/LDS` byte strobes), providing a **16 MB physical address space** (`$00000000..$00FFFFFF`).
* **Registers**:
  * Eight 32-bit Data Registers: `D0..D7`
  * Seven 32-bit Address Registers: `A0..A6`
  * Two 32-bit Stack Pointers: `USP` (User Stack Pointer) and `SSP` (Supervisor Stack Pointer), mapped to `A7` depending on the CPU privilege state.
  * 32-bit Program Counter: `PC` (lower 24 bits active on physical 68000).
  * 16-bit Status Register: `SR` (System Byte + User Condition Code Byte `CCR`).
* **Word Alignment**: All 16-bit word and 32-bit longword accesses must be aligned on even byte boundaries. Accessing an odd address on a word or longword initiates a hardware **Address Error** exception (Vector 3).

### Privilege States & Function Codes

The 68000 provides hardware-enforced privilege separation:

| `FC2` | `FC1` | `FC0` | Cycle Type | Privilege Level |
| :---: | :---: | :---: | :--- | :--- |
| 0 | 0 | 1 | User Data | User Mode (`SR[S] = 0`) |
| 0 | 1 | 0 | User Program | User Mode (`SR[S] = 0`) |
| 1 | 0 | 1 | Supervisor Data | Supervisor Mode (`SR[S] = 1`) |
| 1 | 1 | 0 | Supervisor Program | Supervisor Mode (`SR[S] = 1`) |
| 1 | 1 | 1 | CPU Space | Interrupt Acknowledge / Breakpoint |

* **Supervisor State (`SR` bit 13 = 1)**:
  * The Hatvan kernel runs exclusively in Supervisor state.
  * Active stack pointer is `SSP` (`A7`).
  * Full access to all memory-mapped I/O devices, task mapping registers, and privileged instructions (`RTE`, `STOP`, `RESET`, `MOVE to SR`, `MOVE USP`).
* **User State (`SR` bit 13 = 0)**:
  * User applications run in User state.
  * Active stack pointer is `USP` (`A7`).
  * Attempting to execute any privileged instruction triggers an immediate **Privilege Violation** exception (Vector 8).
  * Any memory access to protected supervisor ranges or the memory-mapped I/O page asserts `/BERR`, triggering an immediate **Bus Error** exception (Vector 2).

---

## 3. Tasks and Memory Map

Hatvan OS/K organizes memory into **256 independent task spaces** (`Task 0` through `Task 255`):

```
       Task 0 (Supervisor / Kernel)               Tasks 1..255 (User Mode)
+----------------------------------------+ +----------------------------------------+
| $00FFFFFF                              | | $00FFFFFF                              |
|   Memory-Mapped I/O ($FF0000..$FFFFFF) | |   PROTECTED (Bus Error / Trap)         |
| $00FF0000                              | | $00FF0000                              |
+----------------------------------------+ +----------------------------------------+
| $00FEFFFF                              | | $00FEFFFF                              |
|                                        | |                                        |
|   Kernel RAM / Buffers / Code          | |   User RAM (Code, Data, Stack)         |
|                                        | |   Stack (USP) grows downwards          |
|                                        | |   Data / BSS grows upwards             |
| $00000400                              | | $00000400                              |
+----------------------------------------+ +----------------------------------------+
| $000003FF                              | | $000003FF                              |
|   68000 Vector Table (1024 bytes)      | |   Vector Mirror or User Scratch        |
| $00000000                              | | $00000000                              |
+----------------------------------------+ +----------------------------------------+
```

### Task 0: Supervisor / Kernel Space
* **Vector Table (`$00000000..$000003FF`)**:
  * Vector 0 (`$000000`): Initial Supervisor Stack Pointer (`SSP`).
  * Vector 1 (`$000004`): Initial Program Counter (`PC` / Reset Vector).
  * Vector 2 (`$000008`): Bus Error.
  * Vector 3 (`$00000C`): Address Error.
  * Vector 4 (`$000010`): Illegal Instruction.
  * Vector 8 (`$000020`): Privilege Violation.
  * Vectors 25..31 (`$000064..$00007C`): Level 1 through 7 Autovectors.
  * Vectors 32..47 (`$000080..$0000BC`): `TRAP #0` through `TRAP #15` (system calls).
* **System RAM (`$00000400..$00FEFFFF`)**: Contiguous memory holding kernel globals, process tables, buffers, filesystem cache, and modules.
* **I/O Page (`$00FF0000..$00FFFFFF`)**: The top 64 KB is reserved for device registers.

### Tasks 1..255: User Spaces
* Each user process runs in a dedicated Task Number matching its Process ID (PID).
* Addresses `$00000400..$00FEFFFF` are available for user code, data, heap, and user stack (`USP`).
* **Strict Protection**: Any read, write, or execute access to `$00FF0000..$00FFFFFF` while `FC2 == 0` (User mode) asserts `/BERR` and terminates the process with a fatal core dump.

---

## 4. Task Switching & Trap Mechanics

Because the 68000 features built-in dual stacks and privilege states, the 6809 `TaskFuse` mechanism is retired in favor of a clean, hardware-native model:

### The Task Register (`$00FF0020`)
* Located at `$00FF0020` in Task 0 I/O space (accessible only in Supervisor mode).
* Holds the 8-bit ID of the currently active User Task (`1..255`).
* **Memory Routing Rule**:
  * When the CPU is in **Supervisor State** (`FC2 = 1`), memory accesses route to **Task 0**.
  * When the CPU is in **User State** (`FC2 = 0`), memory accesses route to the task selected by the **Task Register**.

### Entering Kernel Mode (Traps, System Calls, Interrupts)
1. When a user process initiates a system call (`TRAP #0`), encounters a fault (Illegal Instruction, Zero Divide, Bus Error), or receives an interrupt:
2. The 68000 hardware automatically:
   * Switches to Supervisor state (`SR[S] = 1`).
   * Pushes the user's return `PC` (32 bits) and `SR` (16 bits) onto `SSP`.
   * Asserts `FC2 = 1` on the bus.
3. The memory controller detects `FC2 = 1` and routes the vector fetch to Task 0's vector table at `$00000000..$000003FF`.
4. Execution begins in Task 0 using the private supervisor stack `SSP`. The user's registers remain in `D0-D7`/`A0-A6`, while `USP` holds the user stack pointer.

### Returning to User Mode (`RTE`)
1. The kernel prepares the user registers, updates `USP` using `MOVE.L <val>, USP` if necessary, and executes `RTE` (Return from Exception).
2. `RTE` pops `SR` and `PC` from `SSP`.
3. Because the saved `SR` has `S = 0`, the CPU atomically switches back to User state (`FC2 = 0`), restores the `CCR`, and activates `USP`.
4. The memory controller detects `FC2 = 0` and routes subsequent memory accesses to the user task indicated by the Task Register.

---

## 5. Memory-Mapped I/O Device Registers (`$00FF0000..$00FFFFFF`)

All registers are placed on **even 16-bit word boundaries** to comply with 68000 word-alignment rules. Registers may be accessed using byte (`MOVE.B`), word (`MOVE.W`), or longword (`MOVE.L`) operations as specified.

All multi-byte numbers are big-endian.

```
Address     Width  Name        Description
----------  -----  ----------  -------------------------------------------------------
$00FF0000   W/B    Term.Out    Console Output (write ASCII char to stdout)
$00FF0002   W/B    Term.In     Console Input (read non-blocking ASCII char from stdin)
$00FF0004   W      Reg.Stat    Interrupt / Device Status Register
$00FF0006   W      Reg.Ctrl    Interrupt Control Register
$00FF0008   W/B    logchar     Log Output (write ASCII char to stderr)
$00FF000A   W      Exit.Code   VM Termination Code (write halts VM; read returns status)

$00FF0010   W      Disk.Drive  Disk Drive Selector (0 to 3)
$00FF0012   W      (reserved)  Reserved (must be 0)
$00FF0014   L      Disk.Sector 32-bit Logical Sector Number (LSN)
$00FF0018   W      Disk.Task   Target Task ID (0 to 255)
$00FF001A   L      Disk.Addr   32-bit Target Memory Address in Disk.Task
$00FF001E   W      Disk.CmdSt  Disk Command / Status Register

$00FF0020   W      Task.Active Active User Task Register (1 to 255)

$00FF0022   W      DMA.SrcTask DMA Source Task ID (0 to 255)
$00FF0024   L      DMA.SrcAddr DMA 32-bit Source Address
$00FF0028   W      DMA.DstTask DMA Destination Task ID (0 to 255)
$00FF002A   L      DMA.DstAddr DMA 32-bit Destination Address
$00FF002E   L      DMA.Count   DMA Byte Count (1 to 16,777,216 bytes)
$00FF0032   W      DMA.CmdSt   DMA Command / Status Register
```

### Character Console & 60Hz Timer (`$00FF0000..$00FF000A`)
* **`Term.Out` (`$00FF0000`)**: Writing an ASCII byte transmits it to standard output.
* **`Term.In` (`$00FF0002`)**: Reading returns the next character from non-blocking stdin (cooked line mode), or returns `0` if no character is ready.
* **`Reg.Stat` (`$00FF0004`)**:
  * Bit 0 (`$0001`): `Timer.Ready` (set on 60Hz timer tick; write 1 to clear).
  * Bit 1 (`$0002`): `Term.RxReady` (set when console input is ready; write 1 to clear).
* **`Reg.Ctrl` (`$00FF0006`)**:
  * Bit 0 (`$0001`): `Ctrl.TimrIRQ` (1 = enable timer interrupt).
  * Bit 1 (`$0002`): `Ctrl.TermIRQ` (1 = enable console receive interrupt).
* **`logchar` (`$00FF0008`)**: Writing an ASCII byte sends it to standard error / emulator log.
* **`Exit.Code` (`$00FF000A`)**: Writing a 16-bit status terminates emulator execution with that exit code.

### Interrupt Architecture
Hatvan VM/K supports two interrupt routing options.

We will use Option A, Direct Autovectors.

#### Option A: Direct 68000 Autovectors (Standard)
* **60Hz Timer $\rightarrow$ Interrupt Level 6 (Autovector 30 at `$00000078`)**:
  * Delivers highest priority to system scheduling and preemptive time slicing.
  * No status polling is required; the vector directs straight to the clock handler.
* **Console Input $\rightarrow$ Interrupt Level 4 (Autovector 28 at `$00000070`)**:
  * Vectors directly to the console receive ISR.
  * Can be masked independently by setting `SR[I2:I0] >= 4`.

#### Option B: Unified Polled Vector (Turbo9Sim / OS-9 Port Compatibility)
* Both timer and console input assert **Interrupt Level 2 (Autovector 26 at `$00000068`)**.
* The ISR reads `Reg.Stat` to distinguish `Timer.Ready` and `Term.RxReady`.

### Disk Subsystem (`$00FF0010..$00FF001E`)
* Sector Size: **512 bytes** (or 256 bytes for legacy OS-9 RBF images).
* Supports up to 4 attached images (`/d0` through `/d3`).
* **`Disk.Sector`**: Full 32-bit LSN, supporting multi-terabyte disk images.
* **`Disk.CmdSt` (`$00FF001E`)**:
  * Write: `1 = Read Sector`, `2 = Write Sector`. Initiates transfer and sets status to `0` (busy).
  * Read:
    * `0` = Busy (transfer in progress).
    * `1` = OKAY (transfer completed successfully).
    * `>1` = Error code (e.g. invalid drive, sector out of bounds).
    * Reading non-zero status automatically clears the register back to `0`.

### Fast DMA Copy Engine (`$00FF0022..$00FF0032`)
A hardware DMA engine allows fast inter-task and intra-task block transfers without software copy loops:
* **`DMA.Count`**: 32-bit transfer size (up to full 16 MB task spaces).
* **`DMA.CmdSt` (`$00FF0032`)**:
  * Write: `1 = Start Transfer`. Initiates transfer and sets status to `0` (busy).
  * Read: `0 = Busy`, `1 = OKAY`, `>1 = Error`.

---

## 6. Physical Hardware Realization

The Hatvan VM/K specification is designed for straightforward hardware construction using accessible, standard components:

```
                          +-------------------+
                          |  Motorola 68000   |
                          |  (DIP64 / PLCC68) |
                          +---------+---------+
                                    |
          +-------------------------+-------------------------+
          | Address (A1..A23)       | Data (D0..D15)          | Control
          |                         |                         | (AS, R/W, UDS, LDS, FC0..2)
          v                         v                         v
+-------------------+     +-------------------+     +--------------------+
|  CPLD / FPGA      |     |  High/Low SRAM    |     |  Peripherals       |
|  Memory & Task    |     |  (512K to 16MB)   |     |  (UART / 68681,    |
|  Controller       |     |                   |     |   SD/IDE, Timer)   |
+---------+---------+     +-------------------+     +--------------------+
          |
          +---> Task Bank Lines (High physical address lines to RAM)
          +---> /DTACK generation (Zero-wait-state bus cycling)
          +---> /BERR generation (User mode access to I/O)
          +---> /IPL0..2 & /VPA (Autovectored interrupts)
```

### 1. CPU
* MC68000P8, P10, or P12 (8, 10, or 12.5 MHz) in a 64-pin DIP or 68-pin PLCC package.
* Alternatively, a pin-compatible MC68010, which adds the Vector Base Register (`VBR`) and loop mode acceleration.

### 2. CPLD / FPGA Memory & Bus Controller
A single low-cost CPLD (such as an Altera/Intel MAX II, Xilinx XC95144XL, or Lattice MachXO2) implements:
* **Address Decoding**: Matches `$00FF0000..$00FFFFFF` for I/O devices.
* **Task Bank Generator**: Combines `FC2` and the internal 8-bit `Task.Active` register to output upper physical address lines to RAM chips. When `FC2 == 1`, Task 0 is selected.
* **Bus Error (`/BERR`) Generator**: Asserts `/BERR` if `$00FF0000..$00FFFFFF` is accessed while `FC2 == 0`.
* **DTACK Generator**: Provides asynchronous `/DTACK` handshake signals to the 68000.
* **Interrupt Logic**: Prioritizes timer and UART signals, asserts `/IPL0..IPL2`, and asserts `/VPA` for automatic vector generation.

### 3. DMA & Bus Arbitration
* Uses the 68000's native bus arbitration lines: `/BR` (Bus Request), `/BG` (Bus Grant), and `/BGACK` (Bus Grant Acknowledge).
* The DMA engine requests bus ownership, halts the 68000, transfers blocks at maximum memory bandwidth, and releases the bus.

---

## 7. Software Emulator (`gepk`) in Go

The software emulator `gepk` (formerly `hatvan-vmk`) follows the design principles established in `gep9`:

### Clean-Room Go Implementation
* Entirely written from scratch in standard Go without third-party or GPL-tainted dependencies.
* **Memory Model**:
  ```go
  type MemorySpace [65536][]byte // Sparse 64KB page table per task
  type Bus struct {
      Tasks       [256]*MemorySpace
      CurrentTask uint8
      TaskReg     uint8
      RegStat     uint16
      RegCtrl     uint16
      ConsoleIn   []byte
      ConsoleOut  io.Writer
      LogOut      io.Writer
      StdinChan   <-chan byte
  }
  ```
  Sparse page allocation prevents allocating 256 × 16 MB (4 GB) of host RAM, allocating 64 KB pages on demand.

### Non-Blocking Cooked Stdin
* A background goroutine reads lines from `os.Stdin` in cooked mode, delivering input via a thread-safe Go channel.
* The CPU execution loop polls the channel non-blockingly, converting `\n` to carriage return (`\r`) for terminal input compatibility.
* When `--input="command\n"` is passed, stdin is gated until all initial input has been consumed by the guest OS.

### Command-Line Interface
```bash
gepk [options] <kernel.img|program.s37> [listing.list ...]
```
* `--trace`: Prints 68000 instruction execution traces (PC, D0-D7, A0-A7, SR) to `stderr`.
* `--max-cycles <N>`: Halts simulation after $N$ clock cycles.
* `--max-seconds <N>`: Halts simulation after $N$ real-time wall-clock seconds (default 300).
* `--input <str>`: Pre-enqueues command strings to the console input buffer.
* `--disk0`..`--disk3`: Attaches raw disk images.

---

## 8. Binary and Executable Formats

`gepk` supports three binary distribution formats:

### 1. Direct System Image (`.img`)
* Raw 24-bit binary memory image overlaying Task 0.
* **Offset `$00000000`**: 32-bit Initial Supervisor Stack Pointer (`SSP`).
* **Offset `$00000004`**: 32-bit Initial Program Counter (`PC` / Reset Vector).
* Loaded directly into Task 0; CPU execution begins at the vector target.

### 2. Motorola S-Records (`.s28` / `.s37`)
* Industry-standard hex format for the 68000 family:
  * `S2` records for 24-bit addresses, `S3` records for 32-bit addresses.
  * `S7` / `S8` records defining the execution start address.

### 3. Extended DECB (`.decb`) with 32-Bit Paging (`SET_HIGH16_ADDR32`)
* Keeps headers strictly **5 bytes long**, identical to the 6809 format:
  ```
  [Type: uint8] [Length: uint16 BE] [Address/Arg: uint16 BE] [Payload: Length bytes]
  ```
* In order to load memory into addresses above `$00FFFF` across the 24-bit / 32-bit address space, the format defines an address prefix command code:
  * **`SET_HIGH16_ADDR32 = 254` (`$FE`)**:
    * `Length`: Always `0` (`$0000` — no payload).
    * `Address`: Remembered as the high 16 bits of the 32-bit address (`High16 = Address`).
    * **Semantics**: Effective for all following headers, until another `SET_HIGH16_ADDR32` header changes it.
    * **Initialization**: The parser initializes `High16` to `$0000` at the start of a binary file.
* For all subsequent chunks (e.g. `$00` `DECB_DATA`, `$FF` `DECB_EXEC`, or source line/symbol metadata), the target 32-bit address is formed by:
  $$\text{TargetAddr32} = (\text{High16} \ll 16) \mid \text{Address}$$
* **Cross-Page Spilling in `$00` `DECB_DATA`**:
  * A block of data in a `$00` `DECB_DATA` chunk is explicitly **allowed to spill into the next 64 KB page** if `Address + Length > $10000`.
  * Allowing data chunks to span past a 64 KB boundary simplifies binary producers (assemblers, linkers, converters) by eliminating the requirement to chop contiguous sections into boundary-aligned sub-chunks, placing the handling responsibility onto consumers/loaders.
  * **Persistence**: Spilling into the next page **does not change** the remembered `High16` value. Subsequent headers continue to use the active `High16` prefix until another `SET_HIGH16_ADDR32` header explicitly changes it.
* Any parser that does not recognize type 254 safely skips it (since `Length` is 0). This preserves uniform 5-byte headers across both 6809 and 68000 toolchains while supporting arbitrary 32-bit addresses.

---

## 9. Hatvan OS/K Kernel Services

* **System Call Interface**: Uses `TRAP #0`. Registers `D0-D7` and `A0-A6` pass arguments. `D0` holds the system call number (compatible with OS-9/68000 function codes).
* **Process Model**:
  * Single active user task per PID.
  * `F$Fork` creates a child process in a free Task ID.
  * `F$Exit` returns exit code to waiting parent and frees the task space.
  * Parent processes wait synchronously via `F$Wait`.
* **Standard I/O Paths**:
  * Path 0: `stdin` (`/term`)
  * Path 1: `stdout` (`/term`)
  * Path 2: `stderr` (`/term`)
  * Path 3: `stdlog` (`logchar`)

---

## 10. Summary of Architectural Evolutions (6809 $\rightarrow$ 68000)

1. **Vectors**: Inverted from top-of-RAM (`$FFF0`) to bottom-of-RAM (`$00000000`), matching 68000 architecture.
2. **TaskFuse Retired**: Hardware `SR[S]`, `SSP`, `USP`, and `RTE` replace software stack-switching tricks.
3. **I/O Relocation**: Moved to top of 24-bit space (`$00FF0000..$00FFFFFF`) on 16-bit word-aligned boundaries.
4. **Hardware Protection**: Native Function Codes (`FC2`) and `SR[S]` hardware signals enforce memory protection, replacing software address checks.
5. **Autovectors**: Hardware 7-level priority interrupts replace polled IRQs for clock and console.
