# Hatvan OS/C & Hatvan VM/C Specification

**Architecture Specification for Hatvan COSMAC CDP1802 Virtual Machine (`gepc`) and Physical Hardware Target**

---

## 1. Overview & Vision

**Hatvan OS/C** (`hatvan-osc`) extends the Hatvan operating system family to the legendary **RCA COSMAC CDP1802** 8-bit microprocessor. Joining the Hitachi 6309 / Motorola 6809 (`gep9`), Motorola 68000 (`gepk`), and Zilog Z80 (`gepz`), the architecture code **`'c'`** designates the COSMAC machine: **`gepc`**. (*"gép" is "machine" in Hungarian; `'c'` stands for COSMAC.*)

The RCA 1802 is celebrated as the earliest commercial CMOS microprocessor (designed by Joseph Weisbecker in 1974–1976), famous for its radiation-hardened silicon-on-sapphire implementation on deep space probes (Galileo, Magellan, OSCAR satellites) and beloved microcomputers (COSMAC ELF, SuperELF, VIP, Studio II).

Architecturally, the 1802 is Hatvan's least powerful and least capable 8-bit CPU:
* It has **no dedicated Program Counter (PC)**, **no hardware Stack Pointer (SP)**, and **no built-in `CALL` or `RET` instructions**.
* It provides **no 16-bit arithmetic operations** (all 16-bit pointer and index additions must be synthesized byte-by-byte through an 8-bit accumulator).
* It features only **a single 8-bit accumulator (`D`)**, a **1-bit Data Flag (`DF`)**, and an internal **$16 \times 16$-bit Scratchpad Register Matrix** ($R(0)$ through $R(\text{F})$).

Yet, despite these constraints, the 1802 contains an ingenious, orthogonal register structure that makes Hatvan OS's multi-tasking model, clean memory protection, and Unix/OS-9 system call semantics surprisingly elegant:
1. **The Software Virtual Machine (`gepc`)**: A clean-room, dependency-free emulator written in Go. It implements a complete 1802 execution engine, 256 isolated 64 KB task address spaces, standard Hatvan `$FFxx` memory-mapped I/O, Port `OUT 6` and `OUT 7` trap/untrap fuses, hardware flag polling (`EF1`–`EF4`), and symbolic execution tracing.
2. **The Discrete Hardware Target**: A physical system architecture combining a vintage or modern CMOS CDP1802 (or high-speed CDP1802BC) with a CPLD/FPGA memory management unit (MMU), static RAM, octal address latch (74HC573), and standard Hatvan peripherals.

---

## 2. CDP1802 CPU Architecture & Execution Model

The system is centered around the RCA CDP1802 microprocessor running at standard static clock frequencies (DC to 5.0 MHz at $5\text{V}$, or up to 6.4 MHz at $10\text{V}$):

```
+-------------------------------------------------------------------+
|                   CDP1802 Execution Core                          |
|                                                                   |
|   +-------------------+          +----------------------------+   |
|   |  Pointer Nibbles  |          |  Scratchpad Registers (16) |   |
|   |-------------------|          |----------------------------|   |
|   | P (Selects PC)    |--------->| R(0).1  /  R(0).0  (16-bit)|   |
|   | X (Selects Data)  |--------->| R(1).1  /  R(1).0  (16-bit)|   |
|   | N (Selects Reg)   |--------->| ...                        |   |
|   +-------------------+          | R(F).1  /  R(F).0  (16-bit)|   |
|                                  +----------------------------+   |
|   +-------------------+          +----------------------------+   |
|   | D (8-bit Accum)   |<-------->| ALU & Condition Flags      |   |
|   | DF (1-bit Carry)  |          | (IE, Q, T)                 |   |
|   +-------------------+          +----------------------------+   |
+-------------------------------------------------------------------+
```

### 2.1 The Scratchpad Register Matrix ($R(0)$ through $R(\text{F})$)
The core of the 1802 is its sixteen 16-bit general-purpose scratchpad registers. Each register $R(n)$ consists of a high byte $R(n).1$ and a low byte $R(n).0$:
* **$P$ (Program Counter Pointer — 4 bits):** Designates which scratchpad register $R(P)$ currently functions as the Program Counter. When an instruction byte is fetched, memory at address $R(P)$ is read, and $R(P)$ is automatically incremented ($R(P) + 1 \to R(P)$). Changing $P$ via `SEP Rn` ($Dn\text{h}$) causes an instantaneous jump to the address in register $R(n)$.
* **$X$ (Data Pointer — 4 bits):** Designates which scratchpad register $R(X)$ serves as the indirect base pointer for memory-referencing ALU operations, RAM stores, and I/O byte transfers. Modified via `SEX Rn` ($En\text{h}$).
* **$N$ (Register / Opcode Operand — 4 bits):** The lower 4 bits of the instruction opcode, specifying the target register $R(N)$ for increments, decrements, register transfers, or I/O port selection.

### 2.2 ALU and Working Registers
* **$D$ (Data Register / Accumulator — 8 bits):** Central accumulator for all byte-level ALU arithmetic, bitwise logic, and memory load/store operations.
* **$DF$ (Data Flag / Carry — 1 bit):** Holds the carry, borrow, or shifted-out bit from ALU operations.
* **$T$ (Temporary Register — 8 bits):** Latches the combined state of $(X, P)$ (upper nibble $= X$, lower nibble $= P$) whenever a hardware interrupt or `MARK` instruction is executed.
* **$IE$ (Interrupt Enable — 1 bit):** Controls maskable interrupt handling ($1 = \text{enabled}$, $0 = \text{disabled}$).
* **$Q$ (Output Flip-Flop — 1 bit):** An internal single-bit register driven to physical pin 4 ($Q$). Set via `SEQ` ($7B\text{h}$) and cleared via `REQ` ($7A\text{h}$).

### 2.3 Bus Multiplexing & Machine Cycle Timing
The 1802 uses an 8-bit bidirectional data bus (`BUS0..BUS7`) and a multiplexed 16-bit address bus on 8 physical pins (`A0..A7`):
* Every machine cycle comprises **8 clock periods ($8 \times T$)**.
* Most instructions take **2 machine cycles** (16 clock ticks): $S0$ (Fetch) followed by $S1$ (Execute).
* Long branches and skips (`LBR`, `LBZ`, `LSKP`, etc.) take **3 machine cycles** (24 clock ticks): $S0 \to S1 \to S1$.

```
State    |                  S0 (Fetch)                 |                S1 (Execute)               |
T-Clock  | T1 | T2 | T3 | T4 | T5 | T6 | T7 | T8 | T1 | T2 | T3 | T4 | T5 | T6 | T7 | T8 |
          ________                                  ________
TPA      _|        |_______________________________|        |_______________________________
                         ________                                  ________
TPB      ________________|        |_______________________________|        |________________
         ==============+===========================+==============+=========================
A0-A7    == High Addr  |===== Low Address =========+== High Addr  |===== Low Address =======
         ==============+===========================+==============+=========================
```

* **Timing Pulse A ($TPA$):** Pulses high during clock period $T2$ of every machine cycle, strobing the high-order address byte ($A_8..A_{15}$) onto pins `A0..A7`. A transparent octal latch (74HC573) latches $A_8..A_{15}$ on the falling edge of $TPA$.
* **Low Address Phase ($T3..T8$):** The CPU outputs the lower address byte ($A_0..A_7$) directly on pins `A0..A7`, where it remains stable for the remainder of the cycle.
* **Timing Pulse B ($TPB$):** Pulses high during $T6$ and $T7$, confirming that lower address and data lines are stable. Serves as the write or I/O strobe.
* **State Codes ($SC0, SC1$):** External pins indicate active processor state:
  * $SC1=0, SC0=0$: **$S0$ (Fetch)**
  * $SC1=0, SC0=1$: **$S1$ (Execute)**
  * $SC1=1, SC0=0$: **$S2$ (DMA)**
  * $SC1=1, SC0=1$: **$S3$ (Interrupt Response)**

---

## 3. Tasks and Memory Organization

Hatvan OS/C organizes physical memory into **256 independent task spaces** (`Task 0` through `Task 255`), managed by the Hatvan MMU:

```
        Task 0 (Supervisor / Kernel)               Tasks 1..255 (User Mode)
+----------------------------------------+ +----------------------------------------+
| $FFFF                                  | | $FFFF                                  |
|   Memory-Mapped I/O ($FF00..$FFFF)     | |   PROTECTED (Fatal Hardware Trap)      |
| $FF00                                  | | $FF00                                  |
+----------------------------------------+ +----------------------------------------+
| $FEFF                                  | | $FEFF                                  |
|   Shared Memory Curtain ($E000..$FEFF) | |   User RAM (Code, Data, Stack)         |
|   (Shared by Tasks 0, 1, and 2)        | |   Stack (SP = R2) grows downwards      |
| $E000                                  | |   Heap / Data grows upwards            |
+----------------------------------------+ |                                        |
| $DFFF                                  | |                                        |
|   Kernel RAM / Buffers / Text          | |                                        |
| $0100                                  | | $0100                                  |
+----------------------------------------+ +----------------------------------------+
| $00FF                                  | | $00FF                                  |
|   Hardware Vectors & Trampolines       | |   User Scratchpad / Trampoline         |
|   $0000: Reset Entry (R0 PC)           | |   $0004: Syscall Entry Trampoline      |
|   $0004: Syscall Trap Vector (RE PC)   | |                                        |
|   $0008: Hardware IRQ Handler (R1 PC)  | |                                        |
| $0000                                  | | $0000                                  |
+----------------------------------------+ +----------------------------------------+
```

### 3.1 Task 0: Supervisor / Kernel Space
* **Vector & Entry Area (`$0000..$00FF`)**:
  * `$0000`: Hardware Reset Entry Point. System boots with $P=0, X=0, R(0)=0000\text{h}$.
  * `$0004`: Kernel Syscall Entry Point (invoked when $R(\text{E})$ becomes PC).
  * `$0008`: Maskable Hardware Interrupt Entry Point (invoked when `/INT` forces $P=1, X=2, R(1)$ PC).
* **Kernel Memory (`$0100..$DFFF`)**: Static RAM holding kernel code, global structures, process tables, buffers, and private kernel stack.
* **Shared Memory Curtain (`$E000..$FEFF`)**:
  * Shared across **Task 0 (Kernel)**, **Task 1 (RBF Disk Driver)**, and **Task 2 (PROCFS Driver)**.
  * Hosts `SharedTables`: `ProcTable[16]`, `PathTable[16]`, `RBFMailbox[64]`, and `ProcfsMailbox[64]`.
* **Memory-Mapped I/O Page (`$FF00..$FFFF`)**: Dedicated to Hatvan hardware registers.

### 3.2 Tasks 1..255: User Spaces
* Each user process runs in an isolated Task Number matching its Process ID (PID).
* Memory `$0000..$FEFF` is private to each user process.
* **Strict Page Protection**: Any memory read, write, or opcode fetch to `$FF00..$FFFF` while in a user task ($N \ge 1$, unless blessed by `TaskFlags`) triggers an immediate fatal hardware memory protection trap, terminating the process with a core dump.

---

## 4. Memory-Mapped I/O Page (`$FF00..$FFFF` in Task 0)

The memory-mapped register interface in Page `$FF` is identical in offset, function, and byte-ordering to the Hitachi 6309 (`gep9`) and Zilog Z80 (`gepz`) architectures:

| Address | Register Name | R/W | Description |
| :--- | :--- | :---: | :--- |
| **`$FF00`** | `putchar` (`Term.Out`) | W | Write ASCII byte to standard console output. |
| **`$FF01`** | `getchar` (`Term.In`) | R | Read ASCII byte from console keyboard (non-blocking; returns `0` if empty). |
| **`$FF02`** | `Reg.Stat` | R/W | Status Register. Bit 0 = `Timer.Ready`. Bit 1 = `Term.RxReady`. |
| **`$FF03`** | `Reg.Ctrl` | R/W | Control Register. Bit 0 = `Ctrl.TimrIRQ` (1 = enable 60Hz IRQ). Bit 1 = `Ctrl.TermIRQ`. |
| **`$FF04`** | `logchar` | W | Write ASCII byte to kernel debug log (`os.Stderr`). |
| **`$FF05`** | `ExitCode` | R/W | Writing halts the virtual machine and exits with the written status code. |
| **`$FF10`** | `Disk.Drive` | R/W | Disk drive selector (`0`..`3`, corresponding to `/d0`..`/d3`). |
| **`$FF11..$FF13`** | `Disk.Sector` | R/W | 24-bit Logical Sector Number (LSN), big-endian. |
| **`$FF14`** | `Disk.Task` | R/W | Target memory Task ID for disk sector transfer. |
| **`$FF15..$FF16`** | `Disk.Addr` | R/W | 16-bit target buffer address in destination task (big-endian). |
| **`$FF17`** | `Disk.CmdStat` | R/W | Write: `1 = Read`, `2 = Write`. Read: `0 = Busy`, `1 = OKAY`, `>1 = Error`. |
| **`$FF20`** | `TaskFuse` | W | Latches target user task number for returning from kernel mode. |
| **`$FF21`** | `Dma.SrcTask` | R/W | Source task ID for cross-task block transfer. |
| **`$FF22..$FF23`** | `Dma.SrcAddr` | R/W | 16-bit source memory address (big-endian). |
| **`$FF24`** | `Dma.DstTask` | R/W | Destination task ID. |
| **`$FF25..$FF26`** | `Dma.DstAddr` | R/W | 16-bit destination memory address (big-endian). |
| **`$FF27`** | `Dma.CountStat` | R/W | Write: byte count (`1..255`, or `0 = 256 bytes`). Initiates copy. Read: `0 = Busy`, `1 = OKAY`. |
| **`$FF28`** | `TrapSyscallNum` | R | Latches the syscall opcode transmitted during the user's `OUT 6` trap cycle. |
| **`$FF2D`** | `TaskFlagsTarget`| W | Selects target task ID for capability flag configuration. |
| **`$FF2E`** | `TaskFlags` | R/W | Bit 0 = `I/O Privilege` (allows Tasks 1 & 2 to access `$FF00..$FFFF` directly). |
| **`$FF2F`** | `PurgeTaskMem` | W | Zeroes/frees the specified task's 64 KB memory space and revokes privileges. |

---

## 5. Scratchpad Register Allocation Matrix

Because the 1802 lacks dedicated registers for PC, SP, and addressing, we define a standardized **Register Allocation Matrix** across the 16 scratchpad registers ($R(0)$ through $R(\text{F})$). This guarantees seamless compatibility with Hatvan OS-9 calling conventions, the MiniGolf compiler runtime, and standard COSMAC call/return linkage:

| Register | Native 1802 Role | Hatvan OS Syscall Role | 6809 OS-9 Equivalent | Functional Description & Justification |
| :---: | :--- | :--- | :---: | :--- |
| **$R(0)$** | Reset / DMA Pointer | Hardware DMA Pointer | — | Hardwired by 1802 reset ($P=0, X=0, R(0)=0000\text{h}$) and hardware DMA. |
| **$R(1)$** | Interrupt PC | Hardware Interrupt PC | — | Hardwired by 1802 `/INT` ($P \leftarrow 1$). Receives 60Hz timer & console IRQs. |
| **$R(2)$** | Stack Pointer (`SP`) | Process Stack Pointer | **`S`** | Standard 1802 stack pointer (grows downwards). Forced as $X=2$ on `/INT`. |
| **$R(3)$** | Program Counter (`PC`) | Main Program Counter | **`PC`** | Standard 1802 user and kernel code execution Program Counter ($P=3$). |
| **$R(4)$** | Call Engine Pointer | SCRT Call Routine | — | Subroutine call pointer. Executing `SEP R4` initiates a function call. |
| **$R(5)$** | Return Engine Pointer| SCRT Return Routine | — | Subroutine return pointer. Executing `SEP R5` returns from a function. |
| **$R(6)$** | Linkage Register | Subroutine Linkage | — | Used by SCRT to pass return address and parameter lists. |
| **$R(7)$** | General Register 7 | **Syscall Buffer / Path Pointer** | **`X`** | Primary 16-bit memory pointer argument (`I$Read`, `I$Write`, `I$Open`). |
| **$R(8)$** | General Register 8 | **Syscall Byte Count / Length** | **`Y`** | 16-bit length/count argument for I/O operations. |
| **$R(9)$** | General Register 9 | **Syscall Parameter Block Pointer** | **`U`** | 16-bit pointer for auxiliary structs and parameters (e.g. `F$Fork`). |
| **$R(\text{A})$** | General Register A | **Syscall Secondary Return Value** | **`B`** | Secondary 16-bit value or byte error code; maps to secondary return register. |
| **$R(\text{B})$** | General Register B | **Frame Pointer (`FP`)** | — | Compiler base pointer for local stack frame variable indexing. |
| **$R(\text{C})$** | General Register C | Compiler Temporary 0 (`T0`) | — | High-performance 16-bit expression evaluation scratch register. |
| **$R(\text{D})$** | General Register D | Compiler Temporary 1 (`T1`) | — | High-performance 16-bit expression evaluation scratch register. |
| **$R(\text{E})$** | Trap Pointer | **Syscall Entry PC (`R_TRAP`)** | — | Preloaded to point to `$0004`. Executing `SEP RE` triggers the `OUT 6` trap! |
| **$R(\text{F})$** | Untrap Linkage | **Task Return Fuse (`R_UNTRAP`)** | — | Used by kernel to strobe `OUT 7` and return to user task. |
| **`D`** | Accumulator (8-bit) | **Primary Byte Argument / Result** | **`A`** | Path ID on entry; Return value or Error Code on exit. |
| **`DF`** | Data Flag (1-bit) | **Syscall Error Status (Carry)** | **`CC.C`** | $DF = 0$: Success. $DF = 1$: Error code returned in $D$. |

### Syscall Register Mapping in `SyscallFrame`
In the Hatvan OS kernel's common syscall layer, `SyscallFrame` is populated on trap entry as follows:

```go
type SyscallFrame struct {
    CC byte // Maps to 1802 DF (Data Flag / Carry in bit 0)
    A  byte // Maps to 1802 D (Accumulator / Path ID / mode)
    B  byte // Maps to 1802 R(A).0 (Secondary byte operand / error code)
    DP byte // Unused on 1802 (set to 0)
    X  word // Maps to 1802 R(7) (Buffer / Path pointer)
    Y  word // Maps to 1802 R(8) (Byte count / length)
    U  word // Maps to 1802 R(9) (Parameter block pointer)
    PC word // Maps to 1802 R(3) (User return address)
}
```

---

## 6. Trap and Untrap Mechanics (`OUT 6` & `OUT 7` Fuses)

The central architectural challenge of the CDP1802 is its complete lack of user/supervisor privilege modes, trap instructions (`SWI`, `TRAP`, `SYSCALL`), or automatic vector pushes. Hatvan OS/C solves this cleanly through a hardware/software co-design utilizing **Port `OUT 6` (Trap Fuse)** and **Port `OUT 7` (Untrap Fuse)**.

### 6.1 The Hardware Architecture of `OUT 6` and `OUT 7`
The CDP1802 provides three dedicated physical I/O selection pins: **$N0, N1, N2$**.
When the CPU executes `OUT n` ($61\text{h}..67\text{h}$):
1. The memory byte at address $R(X)$ is driven onto the data bus `BUS0..BUS7`.
2. $R(X)$ is automatically incremented ($R(X) + 1 \to R(X)$).
3. The lower 3 bits of the opcode ($n = 1..7$) are driven onto lines $N0, N1, N2$.
4. Timing Pulse B ($TPB$) pulses high during $T6$ and $T7$.

The Hatvan MMU monitors lines $N0..N2$ and qualifies them with $TPB$:
* **Port 6 ($N2=1, N1=1, N0=0$): The Syscall Trap Fuse**.
* **Port 7 ($N2=1, N1=1, N0=1$): The Syscall Untrap Fuse**.

---

### 6.2 The `OUT 6` Syscall Trap (User $\to$ Kernel)

#### Step 1: User Code Invocation
To issue a system call, user code places arguments into $D, R(7), R(8), R(9)$ and executes:

```assembly
    SEP  RE             ; 1 byte: Opcode $DE -> Switch P to R(E) (Syscall Pointer)
    .db  CALL_NUM       ; 1 byte: Inline Syscall ID (e.g. $84 = I$Open, $8A = I$Write)
    ; User execution resumes here after syscall completion
```

#### Step 2: The Syscall Trampoline
Every user task has $R(\text{E})$ preloaded with the address of the standard Syscall Trampoline at `$0004`:

```assembly
    ; Trampoline at address $0004 (shared entry convention):
    SEX  R3             ; 1 byte: Opcode $E3 -> Set X = 3 (Point R(X) to user PC R(3)!)
    OUT  6              ; 1 byte: Opcode $66 -> Strobe Port 6!
```

#### Step 3: The 1-Byte Hardware Magic of `OUT 6`
When `OUT 6` executes with $X=3$:
1. **Reads the Syscall Code**: The CPU reads the byte at $M(R(3))$—which is precisely the inline `CALL_NUM`!
2. **Drives Data Bus**: The `CALL_NUM` byte is placed on the external data bus `BUS0..BUS7`.
3. **Advances User PC**: The CPU automatically increments $R(3)$ ($R(3) + 1 \to R(3)$). Register $R(3)$ now points directly to the user's return address past the inline byte!
4. **Strobes Port 6**: Lines $N2=1, N1=1, N0=0$ go active on $TPB$.

#### Step 4: The MMU Task Flip
Upon detecting `OUT 6`:
1. The MMU captures the byte on `BUS0..BUS7` into hardware register `$FF28` (`TrapSyscallNum`).
2. The MMU captures the current task number as `TrapSourceTask`.
3. The MMU **immediately flips the active task to Task 0 (Kernel Mode)**!

#### Step 5: Kernel Entry in Task 0
The next machine cycle ($S0$) fetches the next opcode from $M(R(\text{E}))$ **in Task 0 memory space**!
In Task 0, address `$0006` contains:

```assembly
    ; Located at address $0006 in Task 0 (immediately following OUT 6):
    LBR  _SyscallEntry  ; Long branch to kernel C/Golf syscall dispatcher!
```

The CPU branches to `_SyscallEntry` with:
* Current Task $= 0$ (Kernel Mode).
* $R(3)$ holding the exact user return address.
* $R(2)$ holding the user's stack pointer.
* $R(7)..R(\text{D})$ and $D$ holding all user arguments.
* `$FF28` holding the syscall number.

```
+-----------------------------------------------------------------------------------+
| 1. User executes:                                                                 |
|       SEP  RE             ; P = E (points to trampoline at $0004)                 |
|       .db  CALL_NUM       ; Inline syscall code at R(3)                           |
+-----------------------------------------+-----------------------------------------+
                                          |
                                          v
+-----------------------------------------------------------------------------------+
| 2. Trampoline at $0004:                                                           |
|       SEX  R3             ; X = 3                                                 |
|       OUT  6              ; M(R3) -> BUS, R3 = R3 + 1, Strobe Port 6!             |
+-----------------------------------------+-----------------------------------------+
                                          |
                                          v
+-----------------------------------------------------------------------------------+
| 3. Hatvan MMU:                                                                    |
|    * Latches CALL_NUM from BUS into $FF28.                                        |
|    * Latches TrapSourceTask = CurrentTask.                                        |
|    * Flips CurrentTask = 0 (Kernel Mode)!                                         |
+-----------------------------------------+-----------------------------------------+
                                          |
                                          v
+-----------------------------------------------------------------------------------+
| 4. Task 0 Execution:                                                              |
|    * Next fetch from R(E) in Task 0 executes: LBR _SyscallEntry.                  |
|    * Kernel processes syscall using DMA engine and driver tables.                 |
+-----------------------------------------------------------------------------------+
```

---

### 6.3 The `OUT 7` Syscall Untrap (Kernel $\to$ User)

When the kernel finishes servicing the system call, it returns to the user process using the **`OUT 7` TaskFlip Fuse**.

#### Step 1: Return Value Preparation
The kernel prepares return registers:
* **Success**: $DF = 0$ (Carry cleared), primary return value in $D$.
* **Error**: $DF = 1$ (Carry set), error code in $D$.
* User registers $R(7)..R(\text{D})$ updated with any return pointers/counts.

#### Step 2: Arming the TaskFlip Fuse via `OUT 7`
In Task 0, the kernel points $R(X)$ to a memory byte containing the target user task ID (or `$FF20`), and executes `OUT 7`:

```assembly
    ; In Task 0 return path:
    SEX  RF                 ; Point R(X) to R(F), where R(F) points to target_task_id
    OUT  7                  ; 1 byte: Opcode $67 -> Strobe Port 7!
```

When `OUT 7` executes:
1. The target task ID is driven onto `BUS0..BUS7`.
2. The MMU captures `TargetTaskID` into the `TaskFuse` register.
3. The MMU **arms the TaskFlip fuse to trigger on the very next `SEP` instruction**.

#### Step 3: Returning Control with `SEP R3`
The kernel immediately executes:

```assembly
    SEP  R3                 ; 1 byte: Opcode $D3 -> Switch P back to User PC R(3)!
```

During the execution phase ($S1$) of `SEP R3`:
1. The CPU updates internal register $P \leftarrow 3$.
2. The MMU detects the armed fuse and switches the memory bus: **`CurrentTask = TargetTaskID`**!
3. The very next fetch cycle ($S0$) reads from address $R(3)$ in **User Task space**!
4. The user program resumes execution at the instruction immediately following `.db CALL_NUM`, with all return values and flags intact!

---

### 6.4 Hardware Maskable Interrupts (`/INT`)

External hardware interrupts (60Hz timer ticks and keyboard character arrival) are handled through the 1802's native interrupt mechanism:

1. When `Reg.Ctrl` enables interrupts and an event occurs, the Hatvan hardware asserts the physical **`/INT`** line low.
2. At the completion of the current instruction, the 1802 enters State **$S3$ (Interrupt Response)**:
   * Saves $(X, P)$ into internal register $T$: $T \leftarrow (X, P)$.
   * Automatically sets **$P = 1$** and **$X = 2$**.
   * Resets Interrupt Enable **$IE = 0$**.
3. The Hatvan MMU monitors state lines $SC1, SC0$. When state $S3$ is detected ($SC1=1, SC0=1$):
   * The MMU immediately forces **`CurrentTask = 0`**!
4. Instruction fetch begins with $P=1$ from address $R(1)$ in **Task 0**.
5. The kernel interrupt service routine processes the timer/console event.
6. To return to the interrupted user task, the kernel restores state via:
   ```assembly
   SEX  RF
   OUT  7                  ; Arm TaskFlip fuse to return to interrupted task
   RET                     ; Opcode $70: Restores (X, P) from T, sets IE = 1!
   ```
   The MMU flips back to the user task coincident with `RET`, restoring execution seamlessly.

---

## 7. Hardware Flag Inputs ($\overline{\text{EF1}}$–$\overline{\text{EF4}}$) & Output Flag ($Q$)

The CDP1802 features four direct, active-low flag input pins ($\overline{\text{EF1}}$ through $\overline{\text{EF4}}$) and one latched output pin ($Q$). In Hatvan OS/C, these pins are wired directly to hardware subsystem status lines, allowing single-instruction, zero-memory-overhead polling:

| 1802 Pin | Hatvan Hardware Signal | Polling Instruction | Functional Meaning |
| :---: | :--- | :--- | :--- |
| **$\overline{\text{EF1}}$** | `Term.RxReady` | `B1 label` / `BN1 label` | Branch if keyboard character is ready / empty. |
| **$\overline{\text{EF2}}$** | `Disk.Ready` | `B2 label` / `BN2 label` | Branch if disk command has completed (`Disk.CmdStat != 0`). |
| **$\overline{\text{EF3}}$** | `Timer.Ready` | `B3 label` / `BN3 label` | Branch if 60 Hz timer tick has occurred. |
| **$\overline{\text{EF4}}$** | `DMA.Ready` | `B4 label` / `BN4 label` | Branch if DMA cross-task copy has completed. |
| **$Q$** | `LED / Trace Toggle` | `SEQ` ($7B\text{h}$) / `REQ` ($7A\text{h}$) | Latched diagnostic output; toggles execution trace or heartbeat LED. |

### Advantage for Hatvan Drivers
In device drivers (e.g. console `/term` or `/d0`), waiting for keyboard input or disk completion does not require reading memory-mapped I/O registers:
```assembly
.wait_rx:
    BN1  .wait_rx       ; 2 bytes, 16 cycles: Loop until character is ready!
    LDI  $FF            ; Read character from $FF01
    PHI  R7
    LDI  $01
    PLO  R7
    LDN  R7             ; D = getchar
```
This is substantially smaller and faster than polling memory-mapped registers in software.

---

## 8. Subroutine Linkage & MiniGolf Calling Conventions

Because the 1802 lacks hardware `CALL` and `RET` instructions, standard subroutine calls use RCA's **Standard Call and Return Technique (SCRT)**:

### 8.1 The SCRT Engine ($R(4)$ and $R(5)$)
Every task initializes $R(4)$ to the `_CALL` routine and $R(5)$ to the `_RET` routine:

```assembly
; Calling a subroutine 'target':
    SEP  R4             ; P = 4 (Executes _CALL)
    .dw  target         ; Inlined target address
    ; Return resumes here
```

#### The `_CALL` Routine ($R(4)$):
1. $R(3)$ points to `.dw target`.
2. Reads target address from $M(R(3))$ into $R(6)$.
3. Pushes old $R(6)$ or caller's return address to stack $R(2)$.
4. Copies target into $R(3)$.
5. Executes `SEP R3` to enter the subroutine with $P=3$.

#### The `_RET` Routine ($R(5)$):
1. Pops saved return address from stack $R(2)$ into $R(3)$.
2. Executes `SEP R3` to resume caller with $P=3$.

### 8.2 MiniGolf Fastcall Convention
To avoid excessive SCRT stack pushes for short leaf functions:
* **Argument 1 (16-bit / pointer)**: Passed in **$R(7)$** (low byte in $R(7).0$, high in $R(7).1$).
* **Argument 2 (16-bit or 8-bit)**: Passed in **$R(8)$** (or **$D$** if byte).
* **Return Value**: 16-bit return in **$R(7)$**; 8-bit return in **$D$**.
* **Leaf Functions**: Functions that make no calls can execute with direct `SEP R6` linkage, avoiding stack manipulation entirely.

---

## 9. Software Emulator Architecture: `gepc`

The Hatvan 1802 Virtual Machine (`gepc`, architecture code `'c'`) is implemented in clean Go within the `hatvan-os` repository.

### 9.1 Command-Line Usage
```bash
gepc [options] <kernel_1802.decb> [disk0.dsk [disk1.dsk ...]]
```

#### Supported Flags
* `-trace`: Enables symbolic assembly instruction trace printing to stderr.
* `-trace-reg`: Dumps all 16 scratchpad registers ($R(0)..R(\text{F})$), $D, DF, P, X$ on each instruction.
* `-disk`: Traces disk read/write commands and sector transfers.
* `-dma`: Traces cross-task DMA memory copy transfers.
* `-max-cycles <N>`: Terminates execution after $N$ machine cycles.
* `-curt`: Enables curly-brace escaping for non-printable ASCII terminal output.

### 9.2 Extended DECB Binary Format (`.c.decb`)
Like `gep9`, `gepk`, and `gepz`, `gepc` loads executables using the extended DECB format:
* **`$00` (`DECB_DATA`)**: Destination 16-bit load address, byte length, and machine code payload.
* **`$FF` (`DECB_EXEC`)**: Execution start address (preloads $R(3)$ and $R(0)$).
* **`$01..$06`**: Source lines, symbol tables, and module metadata for source-level debugging.

---

## 10. Physical Hardware Implementation Blueprint

A discrete hardware implementation of the Hatvan OS/C system pairs standard, off-the-shelf components:

```
+------------------+         +--------------------+         +-------------------+
|  RCA CDP1802BC   |         | 74HC573 Latch      |         | Static RAM (SRAM) |
|  Microprocessor  |--A0-A7->| (Captures A8-A15   |--A8-A15>| 512 KB / 16 MB    |
|                  |         |  on TPA falling)   |         |                   |
|                  |--A0-A7-------------------------------->| A0-A7             |
|  BUS0..BUS7 <====|=======================================>| D0..D7            |
|                  |                                        |                   |
|  TPA, TPB ------->|                                        | /CE, /OE, /WE     |
|  /MRD, /MWR ----->|         +--------------------+         |                   |
|  N0, N1, N2 ----->|========>| Hatvan CPLD MMU    |-------->| High Bank Lines   |
|  SC0, SC1 ------->|========>| (ATF1508 / XC9572) |         +-------------------+
|  /INT <-----------|<--------|                    |
|  EF1-EF4 <--------|<--------| Port 6 (Trap Fuse) |
+------------------+         | Port 7 (Untrap)    |
                             | Task Registers     |
                             +--------------------+
```

### Components
1. **CPU**: RCA CDP1802BC (or Intersil CDP1802AC) in 40-pin DIP, clocked at 4.0 MHz.
2. **Address Latch**: 74HC573 transparent octal latch, with $LE$ wired to CPU $TPA$.
3. **CPLD / FPGA MMU**: Microchip ATF1508AS or Xilinx XC9572XL:
   * Latches $N0..N2$ on $TPB$.
   * Implements `OUT 6` (Trap Fuse) and `OUT 7` (Untrap Fuse).
   * Decodes `$FF00..$FFFF` memory-mapped I/O page.
   * Provides 8-bit Task Register driving SRAM bank address lines $A_{16}..A_{23}$.
   * Drives $\overline{\text{EF1}}$–$\overline{\text{EF4}}$ flags from peripheral status bits.
4. **SRAM**: 512 KB AS6C4008 (32 tasks $\times$ 64 KB) or 16 MB AS6C1616 (256 tasks $\times$ 64 KB).
5. **UART / Console**: Memory-mapped at `$FF00..$FF01` and wired to $\overline{\text{EF1}}$ ready flag.

---

## 11. Conclusion & Next Steps

This strawman proposal demonstrates that the RCA CDP1802 COSMAC architecture—despite its primitive single-accumulator, no-call design—can host Hatvan OS with full memory isolation, preemptive multi-tasking, and standard OS-9 system call compatibility.

By leveraging:
1. **`OUT 6` with $X=3$**: Reading the inline syscall byte, advancing the user PC, and flipping to Task 0 in a single 1-byte opcode.
2. **`OUT 7` with `SEP R3`**: Latching the target task and flipping back to user space during the program counter register select.
3. **Hardware flags $\overline{\text{EF1}}$–$\overline{\text{EF4}}$**: Enabling single-instruction status polling for console, disk, timer, and DMA.

Hatvan OS/C achieves complete architectural harmony with the 6309, 68000, and Z80 ports.
