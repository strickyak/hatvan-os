# Zilog Z80 Hardware & System Design Reference Guide

This reference document details critical hardware interfaces, memory maps, bus cycle decoding, and clocking considerations for system designers and firmware engineers building or interfacing with the Z80 microprocessor.

---

## 1. Special Fixed Memory Addresses

The Z80 features a set of dedicated hardware addresses located primarily in page zero ($0000\text{h}$–$00FF\text{h}$) of memory. These locations serve as hardwired vector targets for resets, non-maskable interrupts, restart instructions, and interrupt handlers.

### 1.1 The Reset Vector

* **Address:** `$0000\text{h}$`
* **Trigger:** A low signal asserted on the `$\overline{\text{RESET}}$` line for at least 3 full clock periods ($3 \times T\text{-states}$).
* **CPU Behavior:**
  * Program Counter ($PC$) is cleared to `$0000\text{h}$`.
  * Interrupt flip-flops $IFF_1$ and $IFF_2$ are reset to `0` (interrupts disabled).
  * Interrupt Mode is defaulted to **Mode 0**.
  * Register $I$ (Interrupt Vector Base) is cleared to `$00\text{h}$`.
  * Register $R$ (Refresh Counter) is cleared to `$00\text{h}$`.
  * The address and data buses enter a high-impedance state while `$\overline{\text{RESET}}$` is held low, and all control outputs become inactive.
* **Design Rule:** A non-volatile bootloader (ROM, Flash, or EEPROM) or active memory swap hardware must be mapped at `$0000\text{h}$` upon startup.

---

### 1.2 Restart (`RST`) Instruction Vectors

The Z80 provides eight 1-byte call instructions: `RST 00h` through `RST 38h`. Each pushes the current Program Counter to the stack and vectors execution to a 3-bit offset in page zero:

| Instruction | Hex Opcode | Target Vector Address | Typical Firmware / System Usage |
| :--- | :--- | :--- | :--- |
| `RST 00h` | `C7h` | `$0000\text{h}$` | Software warm restart / re-initialization |
| `RST 08h` | `CFh` | `$0008\text{h}$` | Fast dispatch / lightweight runtime helper |
| `RST 10h` | `D7h` | `$0010\text{h}$` | Character output / buffer routines |
| `RST 18h` | `DFh` | `$0018\text{h}$` | Memory comparison / pointer arithmetic helper |
| `RST 20h` | `E7h` | `$0020\text{h}$` | Expression evaluator / interpreter dispatcher |
| `RST 28h` | `EFh` | `$0028\text{h}$` | Math subroutines (CP/M and floating-point packages) |
| `RST 30h` | `F7h` | `$0030\text{h}$` | Extended system hooks / auxiliary traps |
| `RST 38h` | `FFh` | `$0038\text{h}$` | **Default target for Interrupt Mode 1** |

* **Spacing Constraint:** Because vectors are separated by only 8 bytes, typical firmware places a short jump (`JP target` or `JR target`) at each vector unless the handler fits completely within 8 bytes.

---

### 1.3 Non-Maskable Interrupt (NMI) Vector

* **Address:** `$0066\text{h}$`
* **Trigger:** Negative edge (high-to-low transition) on the `$\overline{\text{NMI}}$` pin.
* **CPU Behavior:**
  * Cannot be disabled in software via `DI`.
  * Finishes the currently executing instruction.
  * Pushes the current Program Counter onto the stack ($SP \leftarrow SP - 2$).
  * Preserves interrupt status: $IFF_1$ is cleared (preventing maskable interrupts during the NMI handler), while $IFF_2$ retains the prior state of $IFF_1$.
  * Executes code starting at `$0066\text{h}$`.
* **Exit Instruction:** `RETN` (Return from NMI). This opcode automatically copies $IFF_2$ back into $IFF_1$, restoring the original maskable interrupt state.

---

## 2. Maskable Interrupt Modes (`IM 0`, `IM 1`, `IM 2`)

Maskable interrupts are triggered by pulling the `$\overline{\text{INT}}$` line low. They are acknowledged only if interrupts are enabled ($IFF_1 = 1$, controlled via `EI` and `DI`).

```
                    +--------------------+
                    |  INT asserted low  |
                    +---------+----------+
                              |
               +--------------+--------------+
               |                             |
          [IFF1 == 0]                   [IFF1 == 1]
               |                             |
        (Ignored until                 Acknowledge:
        EI is executed)           Assert M1 and IORQ low
                                             |
                  +--------------------------+--------------------------+
                  |                          |                          |
             [Mode 0]                   [Mode 1]                   [Mode 2]
                  |                          |                          |
         Read instruction           Ignore data bus;            Read vector byte
         from data bus and          Force jump to               from bus; form
         execute it (e.g. RST)      address 0038h               pointer: [I : Vector]
```

### 2.1 Interrupt Mode 0 (`IM 0`)
* **Behavior:** Intel 8080 compatibility mode.
* **Mechanism:** The CPU asserts both `$\overline{\text{M1}}$` and `$\overline{\text{IORQ}}$` low. External hardware must place an executable instruction opcode onto the data bus (`D0`–`D7`).
* **Typical Implementation:** The interrupting device drives a single-byte `RST xx` opcode (`C7h`–`FFh`), causing an immediate jump to the corresponding restart vector. Alternatively, a 3-byte `CALL nn` instruction can be driven over consecutive clock cycles, though this requires more complex sequencing hardware.

### 2.2 Interrupt Mode 1 (`IM 1`)
* **Behavior:** Automatic fixed-vector mode.
* **Mechanism:** When `$\overline{\text{INT}}$` is recognized, the CPU ignores whatever data is on the data bus and automatically vectors to `$0038\text{h}$`.
* **Hardware Implication:** Ideal for simple systems. No external bus-driving logic is required; pull-up resistors on the data bus are sufficient.

### 2.3 Interrupt Mode 2 (`IM 2`)
* **Behavior:** Vectored indirect jump mode, designed for high-performance peripheral suites (like Z80-PIO, SIO, CTC).
* **Address Generation:**
  1. The software initializes the high-order byte using the `LD I, A` instruction.
  2. The interrupting peripheral places an 8-bit vector onto the data bus during the interrupt acknowledge cycle.
  3. The CPU constructs a 16-bit table pointer:
     $$\text{Pointer Address} = (I \times 256) + \text{Vector Byte}$$
  4. The low bit of the vector byte must be `0` because it indexes a 16-bit target address stored low-byte first.
  5. The CPU reads the two contiguous bytes at that pointer and jumps to that target address.

---

## 3. Bus Cycles and Control Line Decoding

The Z80 uses combinations of five active-low status and control lines to signal the intent of a bus cycle:

* `$\overline{\text{M1}}$`: Machine Cycle 1 (Opcode Fetch or Interrupt Acknowledge)
* `$\overline{\text{MREQ}}$`: Memory Request (Address bus holds a valid memory address)
* `$\overline{\text{IORQ}}$`: I/O Request (Address bus holds an I/O port address)
* `$\overline{\text{RD}}$`: Read (CPU reading data from memory or I/O)
* `$\overline{\text{WR}}$`: Write (CPU driving data to memory or I/O)
* `$\overline{\text{RFSH}}$`: Refresh (Low during DRAM refresh cycle)

### 3.1 Truth Table for Bus Operations

| Operation Type | $\overline{\text{M1}}$ | $\overline{\text{MREQ}}$ | $\overline{\text{IORQ}}$ | $\overline{\text{RD}}$ | $\overline{\text{WR}}$ | $\overline{\text{RFSH}}$ | Address Bus ($A_{15}\text{--}A_0$) | Data Bus ($D_7\text{--}D_0$) |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :--- | :--- |
| **Opcode Fetch** | **0** | **0** | 1 | **0** | 1 | 1 | Holds Program Counter ($PC$) | Reading opcode |
| **DRAM Refresh** | 1 | **0** | 1 | 1 | 1 | **0** | $A_6\text{--}A_0$ holds 7-bit $R$ register | High-Z (float) |
| **Memory Read** | 1 | **0** | 1 | **0** | 1 | 1 | Target memory address | Reading data |
| **Memory Write** | 1 | **0** | 1 | 1 | **0** | 1 | Target memory address | Driving valid data |
| **I/O Read (`IN`)** | 1 | 1 | **0** | **0** | 1 | 1 | Port on $A_7\text{--}A_0$; Reg on $A_{15}\text{--}A_8$ | Reading port data |
| **I/O Write (`OUT`)**| 1 | 1 | **0** | 1 | **0** | 1 | Port on $A_7\text{--}A_0$; Reg on $A_{15}\text{--}A_8$ | Driving port data |
| **Interrupt Ack** | **0** | 1 | **0** | 1 | 1 | 1 | Indeterminate / Float | Reading interrupt vector |

---

### 3.2 Glitch-Free Decoding Logic

To generate standard system control strobes for external chips (e.g., SRAM, Flash, UARTs), combine the control signals using standard 74HC-series logic gates:

$$\overline{\text{MEMR}} = \overline{\text{MREQ}} \lor \overline{\text{RD}} \quad (\text{Active Low: Read Memory})$$

$$\overline{\text{MEMW}} = \overline{\text{MREQ}} \lor \overline{\text{WR}} \quad (\text{Active Low: Write Memory})$$

$$\overline{\text{IOR}} = \overline{\text{IORQ}} \lor \overline{\text{RD}} \quad (\text{Active Low: Read I/O Port})$$

$$\overline{\text{IOW}} = \overline{\text{IORQ}} \lor \overline{\text{WR}} \quad (\text{Active Low: Write I/O Port})$$

```
          MREQ -----|\
                    | >o--- MEMR (Active Low)
            RD -----|/

          MREQ -----|\
                    | >o--- MEMW (Active Low)
            WR -----|/

          IORQ -----|\
                    | >o--- IOR (Active Low)
            RD -----|/

          IORQ -----|\
                    | >o--- IOW (Active Low)
            WR -----|/
```

> **Design Tip:** Using OR gates (such as a 74HC32) with active-low inputs produces an active-low output that asserts **only** when both signals are concurrently low.

---

## 4. Understanding I/O Address Mirroring (`IN` & `OUT`)

A common trap for hardware designers involves how the 16-bit address bus behaves during `IN` and `OUT` instructions:

1. **Immediate Addressing (`IN A, (n)` / `OUT (n), A`):**
   * The 8-bit port address $n$ is placed on **$A_0$–$A_7$**.
   * The contents of the **Accumulator ($A$)** are placed simultaneously on **$A_8$–$A_{15}$**.
2. **Register Indirect Addressing (`IN r, (C)` / `OUT (C), r`):**
   * The port address in register **$C$** is placed on **$A_0$–$A_7$**.
   * The contents of register **$B$** are placed simultaneously on **$A_8$–$A_{15}$**.

If designing a minimal I/O system, decode only the lower 8 bits of the address bus ($A_0$–$A_7$). Be aware that software utilizing `B` as an outer loop counter (e.g., block transfer instructions like `OTIR` or `INIR`) will cause lines $A_8$–$A_{15}$ to decrement on every single iteration.

---

## 5. Clocking, Timing, and Hardware Traps

### 5.1 Clock Signal Requirements
* **Waveform:** Pure single-phase square wave.
* **Voltage Levels:** For vintage NMOS parts (Z8400), clock high voltage ($V_{IH}$) requires a minimum of **$V_{CC} - 0.6\text{V}$** (typically $\ge 4.4\text{V}$ on a $+5\text{V}$ rail). Standard 74LS TTL outputs do not pull high enough to reliably clock an NMOS Z80. Use a CMOS driver (e.g., 74HCT, 74HC, or a dedicated oscillator with rail-to-rail output) or add a $1\text{k}\Omega$ pull-up resistor to the clock line.
* **CMOS Variants (Z84C00):** Can tolerate standard full-swing CMOS inputs and can be fully halted down to DC ($0\text{ Hz}$) without losing internal register contents.
* **NMOS Variants:** Have a minimum operating frequency (typically between $250\text{ kHz}$ and $500\text{ kHz}$). Stopping the clock on an NMOS variant causes dynamic storage nodes in the internal execution pipeline to decay, corrupting registers.

### 5.2 The Opcode Fetch ($M_1$) Cycle Speed Penalty
The $M_1$ opcode fetch is the most timing-critical cycle in the processor:
* A standard memory read or write cycle lasts **3 clock periods ($3 \times T\text{-states}$)**.
* An $M_1$ fetch cycle allocates only **about 1.5 clock periods** from the time the address stabilizes to the time data must be valid at the CPU pins, because $T_3$ and $T_4$ are reserved for DRAM refresh.
* **Hardware Warning:** Slow memories (like older EEPROMs) may fail during an $M_1$ fetch while working fine during standard data reads. If using slow memory, use the $\overline{\text{WAIT}}$ line during $M_1$ to inject an extra wait state.

### 5.3 Built-in I/O Wait State
* During every I/O cycle (`$\overline{\text{IORQ}}$` asserted), the Z80 **automatically inserts one wait state ($T_W$)** between $T_2$ and $T_3$.
* This extends all I/O read and write operations to a minimum of **4 $T$-states**, giving slower peripherals extra time to decode addresses and latch data without external wait-state generation circuitry.