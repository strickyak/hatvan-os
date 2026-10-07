# Gedankenexperiment: The RCA COSMAC 3604 Architecture

**A Theoretical Exploration of Doubling the RCA CDP1802 Architecture**

---

## 1. Overview & The Hypothesis

The **RCA COSMAC CDP1802** (designed by Joseph Weisbecker in 1974–1976) is celebrated as one of the most eccentric, orthogonal, and minimalist microprocessor architectures in computing history. With its internal matrix of sixteen 16-bit scratchpad registers, interchangeable Program Counter ($P$) and Data Pointer ($X$), single 8-bit accumulator ($D$), and complete absence of a dedicated hardware stack pointer or built-in subroutine instructions, it achieved legendary reliability in deep space probes (Galileo, Magellan) and early microcomputers.

What if RCA—or an adventurous computer architect—had systematically **doubled every single architectural dimension** of the 1802 to create the **COSMAC 3604**?

$$1802 \times 2 = 3604$$

Under this "Double Everything" rule:
* **8-bit Accumulator ($D$)** $\longrightarrow$ **16-bit Accumulator ($D$)**
* **8-bit Arithmetic / Logic** $\longrightarrow$ **16-bit Arithmetic / Logic**
* **16 Scratchpad Registers** $\longrightarrow$ **256 Scratchpad Registers ($R0..R255$)**
* **16-bit Register Width** $\longrightarrow$ **32-bit Register Width**
* **64 KB Address Space** $\longrightarrow$ **4 GB Flat Linear Address Space ($2^{32}$)**
* **256-byte Short Branch Page** $\longrightarrow$ **64 KB Short Branch Page ($2^{16}$)**
* **8-bit Immediate Operands** $\longrightarrow$ **16-bit Immediate Operands**

---

## 2. Architectural Comparison Matrix

| Architectural Parameter | RCA CDP1802 (Original) | COSMAC 3604 ("Doubled") |
| :--- | :--- | :--- |
| **Model Designation** | $1802$ | **$3604$** ($1802 \times 2$) |
| **Data Bus Width** | 8-bit | **16-bit** |
| **Accumulator ($D$)** | 8-bit ($0..255$) | **16-bit** ($0..65535$) |
| **ALU Math Width** | 8-bit with 1-bit $DF$ | **16-bit** with 1-bit $DF$ |
| **Scratchpad Register Count** | $16$ ($R(0)..R(\text{F})$) | **$256$** ($R(0)..R(255)$) |
| **Scratchpad Register Width** | 16-bit (high byte .1, low byte .0) | **32-bit** (upper halfword .1, lower halfword .0) |
| **Total On-Chip Scratchpad RAM** | 32 bytes ($16 \times 2$ bytes) | **1,024 bytes (1 KB)** ($256 \times 4$ bytes) |
| **PC Register Pointer ($P$)** | 4-bit nibble ($0..15$) | **8-bit byte** ($0..255$) |
| **Data Pointer ($X$)** | 4-bit nibble ($0..15$) | **8-bit byte** ($0..255$) |
| **Temporary Register ($T$)** | 8-bit (latches $(X \ll 4) \mid P$) | **16-bit** (latches $(X \ll 8) \mid P$) |
| **Address Bus Width** | 16-bit multiplexed (64 KB) | **32-bit** multiplexed (4 GB linear) |
| **Short Branch Target** | 8-bit (replaces $R(P).0$, 256 B) | **16-bit** (replaces $R(P).0$, 64 KB page) |
| **Long Branch Target** | 16-bit (replaces full $R(P)$) | **32-bit** (replaces full $R(P)$) |
| **Immediate Constants** | 8-bit (`LDI`, `ADI`, `SMI`) | **16-bit** (`LDI`, `ADI`, `SMI`) |
| **Base Instruction Width** | 8-bit (1 byte) | **16-bit (2 bytes)** |

---

## 3. Instruction Encoding & Opcode Space

### 3.1 The 16-Bit Base Instruction Word
On the 1802, instructions fit in a single 8-bit byte:
$$\text{Opcode} = \langle\text{Operation Code: 4 bits}\rangle \parallel \langle\text{Register } N\text{: 4 bits}\rangle$$
Because $N$ is only 4 bits, it can index at most 16 registers ($R0..R\text{F}$).

To address **256 registers**, the register selector $N$ must expand from 4 bits to **8 bits**. Following the doubling principle, the fundamental instruction word doubles from 8 bits to **16 bits**:

```
+-------------------------------+-------------------------------+
|         Opcode (8 bits)       |      Register N (8 bits)      |
|           OP0 .. OP7          |           N0 .. N7            |
+-------------------------------+-------------------------------+
```

* **Byte 0 (Opcode)**: 256 distinct primary operation codes (expanded from the 1802's 16 primary opcode classes).
* **Byte 1 (Register Index $N$)**: Direct, orthogonal selection of any register $R(0)$ through $R(255)$.

### 3.2 Pointer Expansion: $P$, $X$, and $T$
* **$P$ (Program Counter Selector)**: Expands to an **8-bit register**. Executing `SEP Rn` ($Dn\text{h}$) loads $n$ into $P$, instantaneously selecting register $R(n)$ as the active Program Counter.
* **$X$ (Data Pointer Selector)**: Expands to an **8-bit register**. Executing `SEX Rn` ($En\text{h}$) points all indirect memory loads, stores, and ALU operations to register $R(n)$.
* **$T$ (Temporary Register)**: On the 1802, $T$ latched the combined state of $(X, P)$ as $(X \ll 4) \mid P$ into an 8-bit byte. On the 3604, $T$ expands to **16 bits**, latching:
  $$T \longleftarrow (X \ll 8) \mid P$$
  Because the data bus and accumulator are 16 bits wide, $T$ can be pushed to the stack or read into $D$ in a single 16-bit operation (`SAV` / `MARK`).

---

## 4. Register Architecture & The 1 KB Scratchpad Matrix

The core of the 3604 is an on-chip $256 \times 32$-bit dual-ported static RAM array providing **1,024 bytes (1 KB)** of high-speed registers:

```
Register   | Bits 31..16 (High Halfword .1) | Bits 15..0 (Low Halfword .0)
-----------+--------------------------------+------------------------------
R(0)       |           R(0).1               |           R(0).0
R(1)       |           R(1).1               |           R(1).0
R(2)       |           R(2).1               |           R(2).0
...        |             ...                |             ...
R(255)     |          R(255).1              |          R(255).0
```

Each register $R(n)$ is a full 32-bit linear memory pointer capable of addressing anywhere in the 4 GB physical address space.

### 4.1 Halfword Register Transfers (`GLO`, `GHI`, `PLO`, `PHI`)
Because the accumulator $D$ is 16 bits and registers are 32 bits, the 1802's byte transfers naturally evolve into **16-bit halfword transfers**:

* **`GLO Rn` (Get Low Halfword)**:
  $$D \longleftarrow R(n)[15:0]$$
* **`GHI Rn` (Get High Halfword)**:
  $$D \longleftarrow R(n)[31:16]$$
* **`PLO Rn` (Put Low Halfword)**:
  $$R(n)[15:0] \longleftarrow D$$
* **`PHI Rn` (Put High Halfword)**:
  $$R(n)[31:16] \longleftarrow D$$

#### Synthesizing a 32-Bit Pointer
Loading a full 32-bit address into register $R(84)$ takes just four instructions (8 bytes total):
```assembly
    LDI  $C000       ; Load upper 16 bits into D
    PHI  R84         ; R84[31:16] = $C000
    LDI  $1234       ; Load lower 16 bits into D
    PLO  R84         ; R84[15:0]  = $1234
    ; R84 now holds $C0001234
```

---

## 5. Memory Model & Branch Mechanics

### 5.1 4 GB Flat Address Space
With 32-bit registers, the 3604 natively addresses:
$$2^{32} = 4,294,967,296 \text{ bytes (4 Gigabytes)}$$
Memory-mapped I/O, device registers, and peripherals can reside in the upper addresses (e.g. `$FFFF0000..$FFFFFFFF`), leaving gigabytes of contiguous RAM for user code, data, and buffers without requiring complex bank switching or segmentation.

### 5.2 Short Branches: 64 KB Scope
On the 1802, short branches (`BR`, `BZ`, `BNZ`, `BDF`, etc.) are notoriously restrictive: they only replace the lower 8 bits of $R(P)$, confining branches to the current 256-byte page.

On the 3604, a short branch replaces the **lower 16 bits ($R(P)[15:0]$)** while leaving the upper 16 bits ($R(P)[31:16]$) untouched:
$$R(P)[15:0] \longleftarrow \text{target}_{16}$$

* **Short Branch Range**: **64 Kilobytes** ($0000\text{h}..FFFF\text{h}$ within the current 64 KB block).
* **Impact**: Almost all application code, subroutines, loops, and driver routines fit comfortably inside a single 64 KB page. Software rarely needs to emit 32-bit long branches.

### 5.3 Long Branches: 4 GB Scope
Long branches (`LBR`, `LBZ`, `LBNZ`, `LBDF`, etc.) read a full 32-bit big-endian address from the instruction stream and replace all 32 bits of $R(P)$:
$$R(P) \longleftarrow \text{target}_{32}$$
This allows an instantaneous branch to any location in the entire 4 GB address space.

---

## 6. Arithmetic Logic Unit (ALU) & Condition Flags

The 3604 ALU operates on **16-bit words**:
* **Accumulator ($D$)**: 16 bits ($0..65535$).
* **Data Flag ($DF$)**: 1-bit Carry / Borrow / Shift-out.
* **Borrow Convention**: Preserves the native 1802 convention ($DF=1$ indicates NO BORROW; $DF=0$ indicates BORROW).

### ALU Instruction Set
| Mnemonic | Operation | Description |
| :--- | :--- | :--- |
| **`ADD`** | $D \leftarrow D + M(R(X))$ | 16-bit add memory word to $D$; $DF \leftarrow \text{carry} \ge 65536$. |
| **`ADI imm16`** | $D \leftarrow D + \text{imm}_{16}$ | 16-bit immediate add. |
| **`ADC`** | $D \leftarrow D + M(R(X)) + DF$ | 16-bit add with carry. |
| **`ADCI imm16`**| $D \leftarrow D + \text{imm}_{16} + DF$| 16-bit immediate add with carry. |
| **`SM`** | $D \leftarrow D - M(R(X))$ | 16-bit subtract memory from $D$; $DF=1$ if no borrow ($D \ge M$). |
| **`SMI imm16`** | $D \leftarrow D - \text{imm}_{16}$ | 16-bit immediate subtract from $D$. |
| **`SMB`** | $D \leftarrow D - M(R(X)) - (1 - DF)$ | 16-bit subtract memory with borrow. |
| **`SD`** | $D \leftarrow M(R(X)) - D$ | 16-bit subtract $D$ from memory; $DF=1$ if no borrow ($M \ge D$). |
| **`SDI imm16`** | $D \leftarrow \text{imm}_{16} - D$ | 16-bit immediate subtract $D$ from constant. |
| **`SDB`** | $D \leftarrow M(R(X)) - D - (1 - DF)$ | 16-bit reverse subtract with borrow. |
| **`AND` / `ANI`**| $D \leftarrow D \ \& \ M(R(X)) \text{ or } \text{imm}_{16}$ | 16-bit bitwise AND. |
| **`OR` / `ORI`** | $D \leftarrow D \mid M(R(X)) \text{ or } \text{imm}_{16}$ | 16-bit bitwise OR. |
| **`XOR` / `XRI`**| $D \leftarrow D \oplus M(R(X)) \text{ or } \text{imm}_{16}$ | 16-bit bitwise XOR. |
| **`SHR`** | $DF \leftarrow D[0];\ D \leftarrow D \gg 1$ | 16-bit logical right shift. |
| **`SHRC`** | $D \leftarrow (D \gg 1) \mid (DF \ll 15)$ | 16-bit right shift through carry. |
| **`SHL`** | $DF \leftarrow D[15];\ D \leftarrow (D \ll 1) \& 0xFFFF$ | 16-bit logical left shift. |
| **`SHLC`** | $D \leftarrow ((D \ll 1) \mid DF) \& 0xFFFF$ | 16-bit left shift through carry. |

---

## 7. The Superpowers of the 3604 Architecture

Scaling the 1802 to 3604 produces several remarkable properties not found in traditional 16-bit or 32-bit architectures:

### 7.1 Instant 256-Way Coroutines & Hardware Multitasking
Because any of the 256 scratchpad registers can act as the Program Counter ($R(P)$), context switching between threads requires **zero stack pushes and zero register saves**:
```assembly
    ; Thread A is running with P = 10 (PC is R10).
    ; To yield control to Thread B (whose PC is R25):
    SEP  R25            ; Switch P to 25!
    ; Next machine cycle fetches instruction from R25 in Thread B!
```
* **256 Hardware Threads**: Up to 256 threads, coroutines, or event handlers can run concurrently.
* **Instantaneous Context Switch**: Switching tasks takes exactly 1 instruction (2 bytes, 2 machine cycles).
* **Private State**: Each thread can own its own dedicated registers (e.g. Thread $k$ uses $R(k \times 8)..R(k \times 8 + 7)$).

### 7.2 The Death of Register Starvation
In the standard 1802, the biggest programming bottleneck is register exhaustion:
* 7 registers are reserved for system roles ($R0$: DMA, $R1$: IRQ, $R2$: SP, $R3$: PC, $R4$: Call, $R5$: Ret, $R6$: Linkage).
* Only 9 registers remain for all local variables, parameters, array indices, and temporary expression evaluations.

With 256 registers, **register starvation disappears entirely**:
* Compilers can allocate large blocks of registers per subroutine.
* High-performance leaf functions can execute in private register partitions without ever touching memory.
* Subroutine call/return linkage does not require complex stack-spilling engines like SCRT.

### 7.3 Built-in 1 KB Scratchpad Cache
The $256 \times 32$-bit matrix acts as **1,024 bytes of zero-wait-state on-chip memory**. Table lookups, ring buffers, circular queues, and state machine descriptors can be held directly in registers and referenced using `GLO`, `GHI`, `INC`, and `DEC` without external bus memory cycles.

---

## 8. Why Was It Never Built Historically?

If the 3604 is so mathematically elegant, why did RCA never manufacture it?

### 8.1 The Silicon Transistor Budget (1976–1982)
* **The 1802 (1976)**: Fabricated in RCA's radiation-hardened Silicon-on-Sapphire (SOS) CMOS process, the 1802 contained approximately **5,000 transistors**.
* **The 3604 Register Array**: A dual-ported $256 \times 32$-bit SRAM array requires **8,192 memory bits**. In static CMOS, an SRAM cell requires 6 transistors:
  $$8,192 \times 6 = 49,152 \text{ transistors}$$
  With address decoders, sense amplifiers, and bus muxes, the register file alone would exceed **60,000 transistors**—more than a complete Motorola 68000 (68,000 transistors in 1979) or Intel 8086 (29,000 transistors in 1978).
* In the late 1970s, dedicating that much silicon solely to scratchpad registers on a CMOS process was commercially unviable.

### 8.2 The 1980s Single-Accumulator Bottleneck
By the time semiconductor fabrication could easily accommodate 100,000+ transistors (mid-1980s: Motorola 68020, Intel 80386, ARM1, MIPS R2000):
* Architectural philosophy had overwhelmingly shifted toward **orthogonal load/store RISC**: 16 or 32 general-purpose registers where **any** register could serve as an accumulator, source, or destination (e.g. `ADD R1, R2, R3`).
* Having 256 registers but still forcing every arithmetic operation to funnel through a single accumulator ($D$) would have been viewed as an unacceptable compiler bottleneck.

### 8.3 RCA's Historical Successors (1804 / 1805 / 1806)
Instead of doubling the architecture, RCA made incremental improvements in the late 1970s and early 1980s:
* **CDP1804**: Added on-chip 2 KB mask ROM, 64 bytes RAM, and an 8-bit timer/counter.
* **CDP1805 / 1806**: Added 32 new instructions under a `68h` prefix (including 16-bit register loads `RLDI`, register increments, block copies, and subroutine branches `SCAL`/`SRET`), but retained the 8-bit accumulator, 16 registers, and 64 KB address space.
* RCA eventually sold its semiconductor division to General Electric (and later Harris / Intersil), ending active architectural development of the COSMAC family.

---

## 9. Modern Implementation: FPGA Soft-Core & VM Potential

While economically impractical in 1980, the **COSMAC 3604** is extraordinarily well-suited for modern implementation:

1. **FPGA Realization**:
   * On modern FPGAs (Xilinx Artix-7, Lattice ECP5, Intel Cyclone), the 1 KB register file maps into a single 9-Kb Block RAM (BRAM).
   * The entire CPU core can easily synthesize in under 800 LUTs, clocking at 100+ MHz.
2. **Virtual Machine Target**:
   * A software emulator (written in Go or C, following the style of `gepc`) can implement the 3604 in fewer than 1,000 lines of code.
   * With 4 GB address space and 256 registers, it could serve as a unique, ultra-lightweight bytecode machine or experimental multitasking microkernel testbed.

---

## 10. Summary

The **COSMAC 3604** thought experiment reveals that the architectural DNA of Joseph Weisbecker’s 1802 scales into 16-bit and 32-bit domains with exceptional mathematical symmetry. By doubling the accumulator to 16 bits and registers to 256 32-bit pointers, it eliminates the 1802's historic weaknesses (register starvation, 256-byte page boundaries) while elevating its greatest strength—single-instruction register-driven program counter swapping—into an unprecedented 256-way zero-overhead multitasking engine.
