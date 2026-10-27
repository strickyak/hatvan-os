# Hatvan OS/0 & Hatvan VM/0 Specification (Intel 8086)

**Architecture & Compiler Specification for the Hatvan Intel 8086 Virtual Machine (`gep0`), Kernel (`kernel/i8086`), and MiniGolf Toolchain (`minigolf -m=8086`, `asm8086`)**

---

## 1. Overview & Architecture Codes

**Hatvan OS/0** extends Hatvan OS to the **Intel 8086 / 8088** 16-bit microprocessor family. While the 8086 has 16-bit general-purpose registers (`AX`, `BX`, `CX`, `DX`, `SI`, `DI`, `BP`, `SP`), its segmented 20-bit address bus (`$00000..$FFFFF`, 1 MB) allows each process to exceed the 64 KB ceiling of 8-bit platforms (`6809`, `Z80`, `1802`, `6502`) without requiring 32-bit pointers.

### 1.1 Reserved x86 Architecture Codes (`'0'`..`'3'`)

Hatvan OS reserves four architecture code letters for the Intel x86 progression:

| Arch Code | ASCII Hex | Target CPU | VM Binary | Command Dir | Binary Suffix |
| :---: | :---: | :--- | :--- | :--- | :--- |
| **`'0'`** | `$30` | **Intel 8086 / 8088** (20-bit real mode) | `gep0` | `/d0/Cmds0` | `.0.decb` |
| **`'1'`** | `$31` | **Intel 80186 / 80188** (reserved) | `gep1` | `/d0/Cmds1` | `.1.decb` |
| **`'2'`** | `$32` | **Intel 80286** (reserved) | `gep2` | `/d0/Cmds2` | `.2.decb` |
| **`'3'`** | `$33` | **Intel 80386** (reserved) | `gep3` | `/d0/Cmds3` | `.3.decb` |

In `kernel/i8086/hal.golf`:
* `const ARCH = '0'`
* `/proc/i` reports `"I8086\n"`.
* Extended DECB executables begin with the 5-byte magic header:
  `[253 ($FD), $00, $00, 'x' ($78), '0' ($30)]`.

---

## 2. Memory Model: Small (Split I&D, 128 KB per Process)

Of the six classic 8086 memory models (`Tiny`, `Small`, `Medium`, `Compact`, `Large`, `Huge`), Hatvan OS/0 and MiniGolf standardize on the **Small (Split Instruction & Data)** memory model, allocating **128 KB (`$00000..$1FFFF`) per task**:

* **Data, Heap & Stack Segment (`$00000..$0FFFF`, 64 KB)**:
  * **`DS = SS = ES = $0000`**
  * Holds `.rodata` (string literals and constant tables), `.data`, `.bss`, `v_prelude.Heap`, the downward-growing hardware stack (`SS:SP`), the Shared Memory Curtain (`$0E000..$0FEFF` in Tasks 0, 1, 2), and the Hardware I/O Page (`$0FF00..$0FFFF` in Task 0).
* **Code Segment (`$10000..$1FFFF`, 64 KB)**:
  * **`CS = $1000`**
  * Holds `.text` (all compiled MiniGolf functions, runtime helpers, and startup/trap assembly).

### 2.1 Why Split I&D (`DS=SS=ES=$0000`, `CS=$1000`)?

1. **16-Bit Native `word` and Pointers**:
   * In MiniGolf, `word` and all pointers (`*T`, `*byte`) are **16-bit** (`sizeof[word]() == 2`), matching the 8086's 16-bit registers (`AX`, `BX`, `CX`, `DX`, `SI`, `DI`, `BP`, `SP`).
   * All function calls and returns are fast 16-bit near `CALL` (`E8 disp16` / `FF /2`) and `RET` (`C3`).
2. **No `SS != DS` Pointer Aliasing Hazard**:
   * On the 8086, `[BP+disp]` defaults to `SS`, `[BX]`/`[SI]`/`[DI]`/`[disp16]` default to `DS`, and string instructions (`MOVSB`, `STOSB`, `CMPSB`) use `ES:[DI]`.
   * Because **`DS == SS == ES == $0000`** is an invariant across all MiniGolf code, stack addresses (`&localVar`), global addresses (`&globalVar`), heap addresses, and `tables.Shared` all reside in the same 64 KB data space and can be dereferenced interchangeably without segment override prefixes.
3. **Rule for String Literals and Read-Only Constants**:
   * **All string literals (`v_str_const_*`), panic messages, and constant arrays MUST be placed in the Data Segment (`$0xxxx`, `DS = $0000`), never in the Code Segment (`$1xxxx`, `CS = $1000`).**
   * This guarantees that `(*byte)("hello")` is a standard 16-bit `DS` offset readable by `peekb(word(p))` using `[BX]`/`[SI]` without `CS:` overrides.
4. **Full 64 KB for Code + Full 64 KB for Data/Stack**:
   * On 8-bit targets, large programs like `gsh2` overflowed because ~57 KB of code and ~8 KB of heap shared a single 64 KB space.
   * On 8086 Small model, `.text` has a dedicated 64 KB at `$10000..$1FFFF`, while `.rodata` + `.data` + `.bss` + heap (~10 KB even in `gsh2`) has a dedicated 64 KB at `$00000..$0FFFF`, leaving >45 KB of free stack space in `DS`.

---

## 3. Per-Task 128 KB Memory Map (`$00000..$1FFFF`)

The `gep0` virtual machine supports **16 (or up to 256) isolated task spaces**. Each task has a 128 KB address space (`$00000..$1FFFF`):

```
                    CODE SEGMENT: CS = $1000 ($10000..$1FFFF)
         Task 0 / 1 / 2 (Kernel & Drivers)          Tasks 3..15 (User Processes)
+----------------------------------------+ +----------------------------------------+
| $1FFFF  (1000:FFFF)                    | | $1FFFF  (1000:FFFF)                    |
|                                        | |                                        |
|   Executable Code (.text, up to 63.5K) | |   Executable Code (.text, up to 63.5K) |
|   cstart, MiniGolf functions, runtime  | |   user_start, MiniGolf functions       |
|                                        | |                                        |
| $10200  (1000:0200)  Default Entry IP  | | $10200  (1000:0200)  Default Entry IP  |
+----------------------------------------+ +----------------------------------------+
| $10000..$101FF  Reserved (Null IP=0)   | | $10000..$101FF  Reserved (Null IP=0)   |
+========================================+ +========================================+

              DATA & STACK SEGMENT: DS = SS = ES = $0000 ($00000..$0FFFF)
         Task 0 / 1 / 2 (Kernel & Drivers)          Tasks 3..15 (User Processes)
+----------------------------------------+ +----------------------------------------+
| $0FFFF  (0000:FFFF)                    | | $0FFFF  (0000:FFFF)                    |
|   Hardware I/O Page ($0FF00..$0FFFF)   | |   PROTECTED REDPAGE ($0FF00..$0FFFF)   |
|   (Task 0, or TaskFlags I/O bit = 1)   | |   (Any user read/write faults)         |
| $0FF00  (0000:FF00)                    | | $0FF00  (0000:FF00)                    |
+----------------------------------------+ +----------------------------------------+
| $0FEFF  (0000:FEFF)                    | | $0FEFF  (0000:FEFF)                    |
|   Shared Memory Curtain                | |   User Command-Line Args ($0FE00..FF)  |
|   ($0E000..$0FEFF, 7,936 bytes)        | +----------------------------------------+
|   Shared across Tasks 0, 1, and 2      | | $0FDFF  (0000:FDFF)                    |
|   Holds tables.Shared (SharedTables)   | |   User Stack (SS:SP grows downward)    |
| $0E000  (0000:E000)                    | |         |                              |
+----------------------------------------+ |         v                              |
| $0DFFF  (0000:DFFF)                    | |         ^                              |
|   Kernel / Driver Stack (SS:SP)        | |         |                              |
|         |  (grows downward)            | |   User Globals (.rodata, .data, .bss,  |
|         v                              | |   v_prelude.Heap) starting at $00200   |
|   Globals (.rodata, .data, .bss)       | |                                        |
|   Task 0 starts at $00400              | |                                        |
|   Tasks 1 & 2 start at $00200          | |                                        |
| $00400 / $00200                        | | $00200  (0000:0200)                    |
+----------------------------------------+ +----------------------------------------+
| $003FF  (0000:03FF)                    | | $001FF  (0000:01FF)                    |
|   8086 Interrupt Vector Table (IVT)    | |   User Zero Page & Syscall Scratch     |
|   256 vectors x 4 bytes (IP, CS)       | |   $0020..$0029: User Syscall Registers |
| $00000  (0000:0000)                    | | $00000  (0000:0000)                    |
+----------------------------------------+ +----------------------------------------+
```

---

## 4. Hardware I/O Page (`$0FF00..$0FFFF`) & 8-Bit Register Access

### 4.1 Location of the I/O Page

The Hatvan memory-mapped I/O page resides at **`$0FF00..$0FFFF`** (`0000:FF00..FFFF`), the top 256 bytes of the `$0xxxx` Data Segment:
* Because `DS = $0000`, `vpeekb(0xFF00)` and `vpokeb(0xFF00, val)` address `$0FF00` directly using standard 16-bit `word` constants (`PORT_TERMOUT = 0xFF00`, `SHARED_MEM_CURTAIN = 0xE000`), identical to the `6809`, `Z80`, `1802`, and `6502` ports.
* For convenience, the `gep0` bus may also alias physical `$FFF00..$FFFFF` (`FFFF:00F0` / `F000:FF00`) to the same I/O page in Task 0, and treat both `$0FF00..$0FFFF` and `$FFF00..$FFFFF` as protected redpages in user tasks.

### 4.2 8-Bit Memory-Mapped Registers & Endianness

* **Byte Access (`vpeekb` / `vpokeb`)**: The 8086 natively supports 8-bit byte loads (`MOV AL, [BX]` / `MOV AL, [disp16]`) and 8-bit byte stores (`MOV [BX], AL` / `MOV [disp16], AL`) at any even or odd byte address without alignment restrictions.
* **Big-Endian 32-Bit I/O Registers**: The 8086 is **little-endian**, whereas the 32-bit Hatvan I/O registers (`DISKSEC`, `DISKTARGET`, `DMASRC`, `DMADST`, `DMACNT`, `SHAREDCURTAIN`) are **big-endian** (`[Task:8][Page:8][AddrHi:8][AddrLo:8]`). Just like `kernel/z80/hal.golf` and `kernel/m6502/hal.golf`, `kernel/i8086/hal.golf` must read/write these 4-byte registers byte-by-byte using `vpokeb`/`vpeekb` rather than `vpokew`.

### 4.3 Register Map (`$0FF00..$0FFFF` in Task 0)

| Address | Register Name | R/W | Description |
| :--- | :--- | :---: | :--- |
| **`$FF00`** | `PORT_TERMOUT` | W | Write ASCII byte to console stdout. |
| **`$FF01`** | `PORT_TERMIN` | R | Read ASCII byte from console stdin (non-blocking; `0` if empty). |
| **`$FF02`** | `PORT_REGSTAT` | R/W | Status: Bit 0 = Timer IRQ pending (write 1 to clear), Bit 1 = Rx IRQ pending. |
| **`$FF03`** | `PORT_REGCTRL` | R/W | Control: Bit 0 = Timer IRQ enable, Bit 1 = Rx IRQ enable. |
| **`$FF04`** | `PORT_LOGCHAR` | W | Write ASCII byte to kernel debug log (`stderr`). |
| **`$FF05`** | `PORT_EXIT` | R/W | Write halts `gep0` VM and exits with status code. |
| **`$FF10`** | `PORT_DISKDRV` | R/W | Disk drive selector (`0..3`). |
| **`$FF11`** | `PORT_DISKCMD` | R/W | Write: `1 = Read`, `2 = Write`. Read: `0 = Busy`, `1 = OK`, `>1 = Error`. |
| **`$FF14..$FF17`** | `PORT_DISKSEC` | R/W | 32-bit LSN (Big-Endian). |
| **`$FF18..$FF1B`** | `PORT_DISKTARGET` | R/W | 32-bit `[Task:8][Page:8][AddrHi:8][AddrLo:8]` (Big-Endian). |
| **`$FF20`** | `PORT_TASKFUSE` | W | Arms `TaskFuse` with target task ID; switches task on next `IRET`. |
| **`$FF21`** | `PORT_DMACMD` | R/W | Write `1` to execute DMA copy. Read `1` when complete. |
| **`$FF24..$FF27`** | `PORT_DMASRC` | R/W | 32-bit `[SrcTask:8][SrcPage:8][SrcAddrHi:8][SrcAddrLo:8]` (Big-Endian). |
| **`$FF28..$FF2B`** | `PORT_DMADST` | R/W | 32-bit `[DstTask:8][DstPage:8][DstAddrHi:8][DstAddrLo:8]` (Big-Endian). |
| **`$FF2C..$FF2F`** | `PORT_DMACNT` | R/W | 32-bit byte count (Big-Endian). |
| **`$FF30..$FF33`** | `PORT_SHAREDCURTAIN` | R/W | 32-bit shared curtain address (default `$0000E000`). |
| **`$FF34`** | `PORT_TASKFLAGSTASK` | R/W | Selects task ID for `PORT_TASKFLAGS`. |
| **`$FF35`** | `PORT_TASKFLAGS` | R/W | Bit 0 = I/O privilege for driver tasks (Tasks 1 & 2). |
| **`$FF36`** | `PORT_PURGETASKMEM` | W | Clears all memory (`$00000..$1FFFF`) and flags of specified task. |
| **`$FF60`** | `PORT_TRAPNUM` | R | Latched syscall/trap number from most recent `INT 30h`. |
| **`$FF61`** | `PORT_TRAPTASK` | R | Source task ID (`0..15`) of `INT 30h`, or `$FF` on hardware IRQ. |

---

## 5. Interrupts, Syscall Traps (`INT 30h`), and `IRET` TaskFuse

### 5.1 8086 Interrupt Vector Table (`$00000..$003FF` in Task 0)

Unlike the 6502 or Z80, the 8086 provides 256 distinct 4-byte vectors (`[IP_lo, IP_hi, CS_lo, CS_hi]`) in Task 0 at `$00000..$003FF`:
* **Vector `08h` (`$00020..$00023`)**: Hardware Timer / Terminal Rx IRQ handler (`trap_hw_irq`, `CS = $1000`).
* **Vector `30h` (`$000C0..$000C3`)**: Software Syscall & Driver Sleep Trap handler (`trap_syscall`, `CS = $1000`).

### 5.2 Syscall & Driver Trap Mechanism (`INT 30h` + `DB call_num`)

To match the inline syscall convention used on 6809 (`SWI2; FCB n`), Z80 (`OUT (60h),A; DB n`), and 6502 (`BRK; DB n`), userland syscall wrappers and driver `hal.Sleep()` execute:

```x86asm
    INT 30h
    DB  call_num
```

When the `gep0` CPU executes `INT 30h` (opcode `CD 30`):
1. It reads the inline byte `call_num` at `CS:IP` in the current task and increments `IP` by 1 (so the saved return `IP` points to the instruction after `DB call_num`).
2. It latches `TrapCallNum = call_num` at `$FF60` and `TrapSourceTask = CurrentTask` at `$FF61`.
3. It pushes `FLAGS`, `CS` (`$1000`), and the updated `IP` (6 bytes total) onto the current task's stack (`SS:SP` in Task $N$), and clears `IF` and `TF`.
4. It switches `CurrentTask = 0` and loads `IP` and `CS` (`$1000`) from Task 0's Vector `30h` (`$000C0..$000C3`).

### 5.3 User Syscall Register Block (`$0020..$0029` in User Data Segment)

Just as on the 1802 and 6502 ports, `cmds/lib/cstart_8086.asm` marshals syscall arguments through the 10-byte zero-page block at `$0020..$0029` in the user task's Data Segment (`DS = $0000`), matching `v_syscall.UserFrame`:

| Offset | Size | Field | Role |
| :--- | :---: | :--- | :--- |
| **`$0020`** | 1 | `Flags` (`CC`) | Bit 0 = Carry flag (`1` on error, `0` on success) |
| **`$0021`** | 1 | `A` | Path ID / Open mode / Primary byte result |
| **`$0022`** | 1 | `B` | Error code / Exit status |
| **`$0023`** | 1 | `DP` | Padding (`0`) |
| **`$0024..$0025`** | 2 | `X` | Buffer or pathname pointer (Big-Endian in `UserFrame` or native word) |
| **`$0026..$0027`** | 2 | `Y` | Byte count / Seek hi word |
| **`$0028..$0029`** | 2 | `U` | Aux pointer / Seek lo word |

*(Note: Since both `SyscallFrame` in MiniGolf and the 8086 CPU are little-endian, storing 16-bit `word` values at `$0024`, `$0026`, `$0028` in native little-endian order allows a single 10-byte DMA transfer between user `$0020..$0029` and kernel `v_syscall.UserFrame`!)*

### 5.4 Returning to a Task via `PORT_TASKFUSE` (`$FF20`) and `IRET`

To launch or resume Task $N$:
1. While in Task 0, the kernel restores `SP` from `user_sp_table[N]`, writes $N$ to `$FF20` (`PORT_TASKFUSE`), and executes **`IRET`** (opcode `CF`).
2. When `gep0` executes `IRET` with `TaskFuse` armed:
   * It switches `CurrentTask = TaskFuseTarget` ($N$).
   * It pops `IP`, `CS` (`$1000`), and `FLAGS` (6 bytes) from Task $N$'s stack (`SS:SP`).
   * Execution resumes at `CS:IP` in Task $N$ with `DS = SS = ES = $0000`.

---

## 6. MiniGolf Compiler (`minigolf -m=8086`) Specification

This section defines the contract for the MiniGolf 8086 backend (`-m=8086` or `-m=0`) and the `asm8086` assembler (`minigolf/cmd/asm8086`).

### 6.1 Command-Line Interface

* **Compiler**:
  ```bash
  minigolf -m=8086 [-global_var_offset <N>] [-I <dir> ...] -o <out.asm> <main.golf>
  ```
  *(Accepts `-m=8086`, `-m=i8086`, or `-m=0`.)*
* **Assembler**:
  ```bash
  asm8086 [-l <out.list>] -o <out.decb> <in.asm>
  ```

### 6.2 Type Sizes & Representation

| MiniGolf Type | Size (Bytes) | Alignment | Representation |
| :--- | :---: | :---: | :--- |
| `byte` | 1 | 1 | Unsigned 8-bit (`uint8`), zero-extended to 16 bits when widened to `word`. |
| `bool` | 1 | 1 | `0` (`false`) or `1` (`true`). |
| `word` | 2 | 1 | 16-bit (`uint16` / `int16`), **Little-Endian**. |
| `*T` (pointer) | 2 | 1 | 16-bit offset in `DS` (`$0000..$FFFF`), **Little-Endian**. |
| `string` / `[]T` | 6 | 1 | `{ Ptr *byte, Len word, Cap word }` (6 bytes, matching `6809`/`Z80`/`6502` prelude). |

### 6.3 Register Conventions & ABI

* **Segment Registers (Invariant)**:
  * `CS = $1000` (Code Segment, `$10000..$1FFFF`).
  * `DS = SS = ES = $0000` (Data / Stack / Extra Segment, `$00000..$0FFFF`).
  * Direction Flag `DF = 0` (`CLD` at startup; string instructions auto-increment `SI`/`DI`).
* **General-Purpose Registers**:
  * `AX` (`AH:AL`): Primary 16-bit/8-bit expression accumulator and return register.
  * `BX` (`BH:BL`): Base address register (`[BX]`) and scratch register.
  * `CX` (`CH:CL`): Shift count (`CL`), loop/repeat count (`REP MOVSB`), and scratch register.
  * `DX` (`DH:DL`): Secondary 16-bit operand, high word for `MUL`/`DIV`/`XOR DX,DX`, and scratch register.
  * `SI`, `DI`: Source and destination index registers (`[SI]`, `[DI]`); scratch (or local register allocation).
  * `BP`: Frame pointer (`SS:BP`). **Callee-saved** (`PUSH BP; MOV BP, SP` on entry, `MOV SP, BP; POP BP` on exit).
  * `SP`: Hardware stack pointer (`SS:SP`).
* **Function Calling Convention**:
  * **Hardware Stack Frames**: The 8086 uses the native hardware stack (`SS:SP` and `SS:BP`) for call frames, local variables, and arguments (just like `Z80` uses `IX`/`SP` and `M68000` uses `A6`/`A7`). No software stack is needed.
  * **Arguments**: Pushed onto the stack by the caller before `CALL`, so the first parameter is at `[BP+4]` (`[BP+0]` is saved `BP`, `[BP+2]` is the 16-bit return `IP`). Caller cleans up arguments after `CALL` with `ADD SP, <arg_bytes>`.
  * **Return Values**:
    * `byte` / `bool`: returned in `AL` (zero-extended to `AX` with `MOV AH, 0` when needed).
    * `word` / `*T`: returned in `AX`.
    * Composite types (`struct`, `string`, `slice` > 2 bytes): returned via caller-allocated space / hidden pointer or stack slot, matching MiniGolf's existing Z80/6502 convention.
  * **Symbol Naming**: Matches existing MiniGolf backends:
    * Functions: `f_<pkg>__<FuncName>` (e.g., `f_main__main`, `f_syscall__Dispatch`, `f_hal__LaunchProcess`).
    * Globals: `v_<pkg>.<VarName>` (e.g., `v_proc.CurrentPID`, `v_syscall.UserFrame`, `v_sys.ArgsPtr`).
    * String Constants: `v_str_const_<N>`.

### 6.4 Required Section Placement (`SEGMENT CODE` vs. `SEGMENT DATA`)

Because code (`CS = $1000`) and data (`DS = $0000`) live in separate 64 KB segments, the MiniGolf 8086 backend **must** emit directives that separate executable instructions from data:

```x86asm
    SEGMENT CODE
    ; All functions (f_*) and runtime helper routines go here:
f_main__main:
    PUSH BP
    MOV  BP, SP
    ...
    MOV  SP, BP
    POP  BP
    RET

    SEGMENT DATA
    ORG  $0200          ; (or -global_var_offset N, e.g. $0400 for kernel)
    ; All global variables (v_*), string constants (v_str_const_*),
    ; and panic message strings (.Lpanic*) MUST go in SEGMENT DATA:
v_str_const_1:
    DB   "hello", 0
v_proc.CurrentPID:
    DS   1
```

> [!IMPORTANT]
> Never emit string literals, `.Lpanic` strings, or lookup tables into `SEGMENT CODE`. All data read via pointers must reside in `SEGMENT DATA` (`DS = $0000`).

---

## 7. `asm8086` Assembler & Extended DECB Output Format

### 7.1 Assembly Syntax & Directives

* **Syntax**: Standard Intel 8086 syntax (`OP DST, SRC`):
  * Register/Immediate: `MOV AX, 1234`, `MOV AL, $FF`, `ADD SP, 6`
  * Direct Memory (`DS:[disp16]`): `MOV AL, [v_proc.CurrentPID]`, `MOV [0xFF00], AL`
  * Indirect / Base+Displacement: `MOV AX, [BX]`, `MOV AX, [BP-4]`, `MOV BYTE PTR [BP+6], AL`, `LEA AX, [BP-16]`
  * Explicit Operand Size Qualifiers: `BYTE PTR` and `WORD PTR` where neither operand is a sized register (e.g., `MOV BYTE PTR [BX], 0`, `CMP WORD PTR [BP-2], 0`).
* **Directives**:
  * `SEGMENT CODE` / `SEGMENT DATA`: Switches active segment between Code (`High16 = $0001`, `CS = $1000`) and Data (`High16 = $0000`, `DS = $0000`). Each segment maintains its own location counter, or uses explicit `ORG`.
  * `ORG <expr>`: Sets the 16-bit offset within the currently active segment.
  * `EQU`: Symbol constant definition (`PORT_TERMOUT EQU $FF00`).
  * `DB`: Byte or ASCII string data (`DB $01, "hatvan", 0`).
  * `DW`: 16-bit Little-Endian word (`DW $1234`, `DW label`).
  * `DS <count>` (or `RMB` / `RESB`): Reserves `<count>` zero-initialized bytes.
  * `END [entry_label]`: Marks end of assembly and records the execution entry offset (`IP`).
* **Automatic Conditional Jump Relaxation (`Jcc`)**:
  * On the 8086, conditional jumps (`JE`/`JZ`, `JNE`/`JNZ`, `JL`, `JLE`, `JG`, `JGE`, `JB`/`JC`, `JBE`, `JA`, `JAE`, `JS`, `JNS`) only support an 8-bit signed displacement (`-128..+127`).
  * Whenever a `Jcc target` displacement exceeds `-128..+127`, **`asm8086` must automatically relax it** into the inverted short jump over a 3-byte near jump (5 bytes total):
    ```x86asm
    J!cc $+5        ; 2 bytes: skip JMP if condition is false
    JMP  NEAR target ; 3 bytes: E9 disp16
    ```
  * Unconditional `JMP target` defaults to `JMP NEAR` (`E9 disp16`, 3 bytes) or `JMP SHORT` (`EB disp8`, 2 bytes) when in range.

### 7.2 Extended DECB (`.decb`) Output Layout

`asm8086` outputs a single Extended DECB binary containing both segments using the standard Hatvan `SET_HIGH16_ADDR32` (`Tag = 254` / `$FE`) prefix:

1. **Architecture Magic Record (`Tag = 253` / `$FD`)**:
   ```
   FD 00 00 78 30       ; Tag=253, Len=0, 'x' (0x78), '0' (0x30 = Intel 8086)
   ```
2. **Code Segment Prefix & Data Chunks (`High16 = $0001` $\rightarrow$ `$10000..$1FFFF`)**:
   ```
   FE 00 00 00 01       ; Tag=254 (SET_HIGH16_ADDR32), Len=0, High16 = $0001
   00 <len_hi> <len_lo> <addr_hi> <addr_lo> <code bytes...>
   ```
3. **Data Segment Prefix & Data Chunks (`High16 = $0000` $\rightarrow$ `$00000..$0FFFF`)**:
   ```
   FE 00 00 00 00       ; Tag=254 (SET_HIGH16_ADDR32), Len=0, High16 = $0000
   00 <len_hi> <len_lo> <addr_hi> <addr_lo> <data/rodata/bss bytes...>
   ```
4. **Execution Postamble (`Tag = 255` / `$FF`)**:
   ```
   FF 00 00 <entry_hi> <entry_lo>   ; Entry IP offset within CS=$1000 (e.g. $0200)
   ```

> [!NOTE]
> Because `kernel/common/loader.golf` (`LoadDECB`) already tracks `high16` from Tag `254` and passes `page := byte(high16 & 0xFF)` to `path.SysReadPage` $\rightarrow$ `hal.DMACopyPage`, the Hatvan kernel loads both segments (`page = 1` into `$1xxxx` and `page = 0` into `$0xxxx`) without any changes to `loader.golf`!

---

## 8. Implementation Roadmap

1. **Phase 1 — MiniGolf 8086 Backend & `asm8086` (`../minigolf`)**:
   * Implement `minigolf -m=8086` (emitting `SEGMENT CODE` and `SEGMENT DATA`) and `minigolf/cmd/asm8086` (with `Jcc` branch relaxation and multi-segment DECB output).
   * Verify with unit tests in `minigolf`.
2. **Phase 2 — `gep0` Virtual Machine (`gep0/`, `cmd/gep0/`)**:
   * Implement clean-room 8086 CPU emulator (`gep0/cpu.go`, `gep0/ops.go`, `gep0/modrm.go`, `gep0/alu.go`, `gep0/disasm.go`), 16-task $\times$ 128 KB (or 1 MB) bus (`gep0/bus.go`), `$0FFxx` I/O page, `INT 30h` trap handling, and `IRET` `TaskFuse`.
3. **Phase 3 — Hatvan OS/0 Kernel & Drivers (`kernel/i8086/`)**:
   * Implement `kernel/i8086/hal.golf`, `kernel/i8086/cstart_i8086.asm`, `kernel/i8086/trap_i8086.asm`, `drivers/rbf/cstart_rbf_i8086.asm`, and `drivers/procfs/cstart_procfs_i8086.asm`.
4. **Phase 4 — Userland Commands (`/d0/Cmds0`) & System Tests (`run0.sh`)**:
   * Implement `cmds/lib/cstart_8086.asm`, update `build-cmd.sh` and `Makefile` to build `*.0.decb` (including `gsh` and `gsh2`) into `/d0/Cmds0`, and verify with `make test-8086`.
