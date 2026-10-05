# RCA CDP1802 COSMAC Hardware & Architecture Reference Guide

This reference document details the hardware architecture, register structure, bus multiplexing protocols, state timing, and electrical interfacing considerations for the RCA CDP1802 COSMAC 8-bit microprocessor (40-pin DIP package).

## 1. Internal Architecture & The Scratchpad Register Matrix

Unlike traditional contemporary microprocessors (e.g., 6502, Z80, 8085), the RCA 1802 does not have a single dedicated Program Counter (PC), hardware stack pointer, or index register. Instead, CPU execution and memory indexing revolve entirely around an internal **$16 \times 16$-bit Scratchpad Register Matrix** ($R(0)$ through $R(\text{F})$) controlled by three 4-bit register selector pointers:

```
+-------------------------------------------------------------+
|                     1802 Execution Core                     |
|                                                             |
|   +-------------------+    +----------------------------+   |
|   |  Pointer Nibbles  |    |  Scratchpad Registers (16) |   |
|   |-------------------|    |----------------------------|   |
|   | P (Selects PC)    |--->| R(0).0  /  R(0).1  (16-bit)|   |
|   | X (Selects Data)  |--->| R(1).0  /  R(1).1  (16-bit)|   |
|   | N (Selects Reg)   |--->| ...                        |   |
|   +-------------------+    | R(F).0  /  R(F).1  (16-bit)|   |
|                            +----------------------------+   |
|   +-------------------+    +----------------------------+   |
|   | D (8-bit Accum)   |<-->| ALU & Condition Flags      |   |
|   | DF (1-bit Carry)  |    | (IE, Q, T)                 |   |
|   +-------------------+    +----------------------------+   |
+-------------------------------------------------------------+
```

### 1.1 The Pointer Nibbles
* **$P$ (Program Counter Pointer — 4 bits):** Points to the specific scratchpad register $R(P)$ currently serving as the Program Counter. Changing $P$ via the `SEP Rn` ($7n\text{h}$) instruction causes an immediate jump to the address contained in register $R(n)$.
* **$X$ (Data Pointer — 4 bits):** Points to the register $R(X)$ used as the indirect base pointer for memory-referencing ALU operations, RAM stores, and peripheral transfers. Modified via `SEX Rn` ($En\text{h}$).
* **$N$ (Instruction Operand / Register Select — 4 bits):** Holds the lower 4 bits of the currently fetched instruction opcode, indicating the target register $R(N)$ for register transfers, increments, decrements, and branches.

### 1.2 Auxiliary Internal Registers
* **$D$ (Data Register / Accumulator — 8 bits):** Central working register for all byte-level ALU arithmetic, logic, and external loads.
* **$DF$ (Data Flag / Carry — 1 bit):** Holds the arithmetic carry, borrow, or shift out from $D$.
* **$T$ (Temporary Register — 8 bits):** Latches the combined prior states of $X$ and $P$ (upper nibble $= X$, lower nibble $= P$) whenever a hardware interrupt is accepted.
* **$IE$ (Interrupt Enable — 1 bit):** Controls maskable interrupt handling ($1 = \text{enabled}$, $0 = \text{disabled}$).

### 1.3 Subroutine Linkage (SCRT: Standard Call and Return Technique)
Because there are no built-in `CALL` or `RET` instructions, standard programs assign specific registers to implement a call engine:
* $R(3)$: Main Program Counter (points to active instruction stream).
* $R(4)$: Standard Call Routine pointer.
* $R(5)$: Standard Return Routine pointer.
* $R(2)$: Stack Pointer (points to memory stack).
* $R(6)$: Parameter and Return Address linkage register.

A call is made by pointing $P$ to $R(4)$ via `SEP R4`, which pushes the old $R(3)$ value to the stack and re-points $R(3)$ to the subroutine target address.

---

## 2. Hardwired Vectors and Operating Conditions

The 1802 does not use an interrupt vector table in RAM. Hardware events directly manipulate $P$, $X$, and $R(0)$:

| Hardware Event | Hardware Vector Action | Description / Firmware Initialization Rules |
| :--- | :--- | :--- |
| **Hardware Reset** | Forces **$P = 0$**, **$X = 0$**, **$R(0) = 0000\text{h}$** | Execution begins at address **`$0000`** with $R(0)$ acting as both the Program Counter and the indirect data pointer. The reset stub sets up pointers ($R(2), R(3)$, etc.) and executes `SEP R3` to hand control to main firmware. |
| **Maskable Interrupt (`/INT` low)** | Forces **$P = 1$**, **$X = 2$** | Halts current execution, saves $(X, P)$ into $T$, resets $IE = 0$, and transfers the PC to **$R(1)$**. Execution resumes at the address preloaded in $R(1)$ with $R(2)$ serving as the data/stack pointer. |
| **DMA In Service (`/DMA-IN` low)** | Memory addressed via **$R(0)$** | The CPU writes external data directly to the memory address pointed to by $R(0)$, increments $R(0)$, and acknowledges external hardware without software execution cycles. |
| **DMA Out Service (`/DMA-OUT` low)** | Memory addressed via **$R(0)$** | The CPU reads a byte from the memory address pointed to by $R(0)$, places it on the data bus, increments $R(0)$, and acknowledges external hardware. |

---

## 3. Bus Multiplexing, State Timing, and Cycle Decoding

The CDP1802 uses a **multiplexed 16-bit address bus** on 8 physical pins (`A0`–`A7`) to fit a 40-pin DIP layout. Every machine cycle spans **8 clock periods ($8 \times T$)**, split into a high-order address phase and a low-order data phase.

```
CLK      __   __   __   __   __   __   __   __   __   __   __   __   __   __   __   __
       _|  |_|  |_|  |_|  |_|  |_|  |_|  |_|  |_|  |_|  |_|  |_|  |_|  |_|  |_|  |_|  |_
State   |                  S0 (Fetch)                 |                S1 (Execute)               |
T-Clock | T1 | T2 | T3 | T4 | T5 | T6 | T7 | T8 | T1 | T2 | T3 | T4 | T5 | T6 | T7 | T8 |
         ________                                  ________
TPA    _|        |_______________________________|        |_______________________________
                        ________                                  ________
TPB    ________________|        |_______________________________|        |________________
       ==============+===========================+==============+=========================
A0-A7  == High Addr  |===== Low Address =========+== High Addr  |===== Low Address =======
       ==============+===========================+==============+=========================
```

### 3.1 State Codes ($SC0, SC1$)
Two state-code output pins define the active processor phase on every cycle:

| $SC1$ | $SC0$ | Machine Cycle | Operations Performed |
| :---: | :---: | :--- | :--- |
| **$0$** | **$0$** | **$S0$ (Fetch)** | Opcode is fetched from memory address $R(P)$. |
| **$0$** | **$1$** | **$S1$ (Execute)** | The fetched instruction executes (memory access, ALU, branch, I/O). |
| **$1$** | **$0$** | **$S2$ (DMA)** | External DMA transfer occurs; memory addressed via $R(0)$. |
| **$1$** | **$1$** | **$S3$ (Interrupt)** | Hardware forces $T \leftarrow (X, P)$, sets $P=1, X=2, IE=0$. |

### 3.2 Address Demultiplexing ($TPA$ and $TPB$)
* **Timing Pulse A ($TPA$):** Pulses high during clock period $T2$ of **every machine cycle**. It strobes the **high-order address byte** ($A_8$–$A_{15}$) onto pins `A0`–`A7`.
  * **External Demuxing:** Wire pins `A0`–`A7` to a 74HC573 or 74HC373 transparent octal latch, with the latch enable ($LE$) connected directly to $TPA$. The latch captures $A_8$–$A_{15}$ on the falling edge of $TPA$.
* **Low Address Phase ($T3$–$T8$):** Immediately after $TPA$ falls, the CPU outputs the lower 8 bits ($A_0$–$A_7$) directly onto pins `A0`–`A7`, where they remain stable for the rest of the cycle.
* **Timing Pulse B ($TPB$):** Pulses high during clock period $T6$ and $T7$. It confirms that the lower address and data lines are stable and serves as the write or I/O strobe.

### 3.3 Memory Control Lines
* **$\overline{\text{MRD}}$ (Memory Read — Active Low):** Low during all read cycles (Fetch, Execute-Read, DMA-Out). High during writes.
* **$\overline{\text{MWR}}$ (Memory Write — Active Low):** Pulses low during memory write cycles coincident with $TPB$.

```
Address Decoding Logic:
  CPU A0-A7  -------------------------> SRAM / ROM A0-A7
  CPU A0-A7  ---->[ 74HC573 Latch ]---> SRAM / ROM A8-A15
  CPU TPA    ---->[   Latch LE    ]
  CPU /MRD   -------------------------> SRAM / ROM /OE
  CPU /MWR   -------------------------> SRAM /WE
```

---

## 4. Hardware I/O Architecture

The 1802 provides three distinct peripheral control mechanisms: testable flag inputs, a programmable output flag, and 3-bit port-select strobes.

```
       +---------------------------------------------+
       |             RCA CDP1802 I/O Lines           |
       |                                             |
       |   EF1-EF4  ------> [ Testable Input Flags ] |
       |   Q        <------ [ Latched Output Line  ] |
       |   N0-N2    <------ [ 3-Bit Port Strobes   ] |
       |   /INT     ------> [ Level-Sensitive IRQ  ] |
       |   /DMA-IN  ------> [ High-Priority Input  ] |
       |   /DMA-OUT ------> [ High-Priority Output ] |
       +---------------------------------------------+
```

### 4.1 External Flag Inputs ($\overline{\text{EF1}}$ through $\overline{\text{EF4}}$)
* Four active-low hardware input pins directly polled by branch instructions:
  * `B1` / `BN1` ($34\text{h}$ / $3C\text{h}$): Branch if $\overline{\text{EF1}}$ active / inactive.
  * `B2` / `BN2` ($35\text{h}$ / $3D\text{h}$): Branch if $\overline{\text{EF2}}$ active / inactive.
  * `B3` / `BN3` ($36\text{h}$ / $3E\text{h}$): Branch if $\overline{\text{EF3}}$ active / inactive.
  * `B4` / `BN4` ($37\text{h}$ / $3F\text{h}$): Branch if $\overline{\text{EF4}}$ active / inactive.
* These flags require zero address decoding and no RAM or register overhead, making them ideal for bit-banging serial input lines, sensor triggers, or pushbuttons.

### 4.2 The $Q$ Output Line
* An internal single-bit flip-flop driven to external pin 4 ($Q$).
* Controlled via two dedicated 1-byte opcodes:
  * `SEQ` ($7B\text{h}$): Set $Q = 1$ (high).
  * `REQ` ($7A\text{h}$): Reset $Q = 0$ (low).
* Used for bit-banged serial transmission (TX), piezo tone generation, and hardware status indicators.

### 4.3 I/O Device Selection ($N0, N1, N2$)
* When executing an `INP 1`–`INP 7` ($69\text{h}$–$6F\text{h}$) or `OUT 1`–`OUT 7` ($61\text{h}$–$67\text{h}$) instruction, the lower 3 bits of the opcode are placed onto the external output lines **$N0, N1,$ and $N2$**.
* An inactive line condition ($N0=N1=N2=0$) indicates no active I/O operation.
* An external 3-to-8 decoder (e.g., 74HC138) qualified by $TPB$ decodes lines $N0$–$N2$ into 7 discrete input and 7 discrete output port select strobes:
  * **Input (`INP`):** The peripheral drives 8-bit data onto `BUS0`–`BUS7`. The CPU stores this byte simultaneously into the accumulator $D$ and memory location $R(X)$.
  * **Output (`OUT`):** The CPU drives data from memory location $R(X)$ onto `BUS0`–`BUS7` and increments $R(X)$. The peripheral latches the bus on the rising edge of $TPB$.

---

## 5. Control Lines & Processor Modes

The operating mode is configured using two input pins: **$\overline{\text{CLEAR}}$** (pin 3) and **$\overline{\text{WAIT}}$** (pin 2).

| $\overline{\text{CLEAR}}$ | $\overline{\text{WAIT}}$ | Operational Mode | Hardware Action |
| :---: | :---: | :--- | :--- |
| **$0$** | **$0$** | **LOAD** | Halts CPU execution. Enables front-panel memory loading via DMA. Strobing $\overline{\text{DMA-IN}}$ writes data from the bus into memory at $R(0)$, and $R(0)$ increments automatically. |
| **$0$** | **$1$** | **RESET** | Resets internal registers: $P=0, X=0, R(0)=0000\text{h}, Q=0, IE=1$. The data bus enters high-impedance. |
| **$1$** | **$0$** | **PAUSE** | Freezes the internal clock generator. CPU stops running, but all internal registers and state remain preserved. |
| **$1$** | **$1$** | **RUN** | Normal instruction execution. |

---

## 6. Electrical Characteristics & Hardware Design Rules

### 6.1 Static Operation
The CDP1802 is constructed with static CMOS logic. The clock can be slowed or completely halted at **$0\text{ Hz}$ (DC)** without losing internal registers or scratchpad memory.

### 6.2 Operating Specifications

| Parameter | CDP1802A (Standard CMOS) | CDP1802AC / BC (High-Speed CMOS) |
| :--- | :--- | :--- |
| **Supply Voltage ($V_{DD}$)** | $+4\text{V}$ to $+10.5\text{V}$ | $+4\text{V}$ to $+6.5\text{V}$ (typically $+5.0\text{V}$) |
| **Max Clock at $V_{DD} = 5\text{V}$** | $\approx 3.2\text{ MHz}$ | $\approx 5.0\text{ MHz}$ |
| **Max Clock at $V_{DD} = 10\text{V}$** | $\approx 6.4\text{ MHz}$ | Not rated |
| **Logic High Input ($V_{IH}$)** | $0.7 \times V_{DD}$ (min $3.5\text{V}$ at $5\text{V}$) | $0.7 \times V_{DD}$ (min $3.5\text{V}$ at $5\text{V}$) |
| **Logic Low Input ($V_{IL}$)** | $0.3 \times V_{DD}$ (max $1.5\text{V}$ at $5\text{V}$) | $0.3 \times V_{DD}$ (max $1.5\text{V}$ at $5\text{V}$) |
| **Quiescent Current ($I_{DD}$)** | $< 100\ \mu\text{A}$ at $5\text{V}$ | $< 50\ \mu\text{A}$ at $5\text{V}$ |

### 6.3 Interfacing Rules
1. **Clock Input Voltage:** Because $V_{IH}$ is $0.7 \times V_{DD}$ ($3.5\text{V}$ at $5\text{V}$), classic 74LS TTL circuits cannot drive the clock pin directly. Use 74HCT/74HC logic or attach a $2.2\text{ k}\Omega$ pull-up resistor to $+5\text{V}$ on the clock line.
2. **Crystal Oscillator:** An internal inverter connects pin 1 (`XTAL`) to pin 39 (`CLOCK`). To drive using a crystal, connect a parallel-resonant fundamental crystal across pins 1 and 39 with two $22\text{ pF}$ load capacitors to ground and a $10\text{ M}\Omega$ feedback resistor across the crystal pins.
3. **Unused Inputs:** Never leave inputs floating. Tie unused interrupt and DMA inputs ($\overline{\text{INT}}$, $\overline{\text{DMA-IN}}$, $\overline{\text{DMA-OUT}}$) and unused flag pins ($\overline{\text{EF1}}$–$\overline{\text{EF4}}$) through $10\text{ k}\Omega$ pull-up resistors to $V_{DD}$.