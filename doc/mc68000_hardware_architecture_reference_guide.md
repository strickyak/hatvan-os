# Motorola MC68000 Hardware & System Design Reference Guide

This reference document details the hardware architecture, memory map conventions, asynchronous bus handshake protocols, exception processing, and clocking/interfacing considerations for the Motorola MC68000 microprocessor (64-pin DIP / 68-pin ceramic/PLCC package).

---

## 1. Pinout & Bus Signal Groups

The MC68000 provides a 32-bit internal architecture with an external **16-bit bidirectional data bus** (`D0`–`D15`) and a **24-bit address bus** capable of directly addressing **16 MB** of physical address space.

```
                     +---------+--+---------+
              D4  1  |                      | 64  D5
              D3  2  |                      | 63  D6
              D2  3  |                      | 62  D7
              D1  4  |                      | 61  D8
              D0  5  |                      | 60  D9
            /AS   6  |                      | 59  D10
           /UDS   7  |                      | 58  D11
           /LDS   8  |                      | 57  D12
            R/W   9  |                      | 56  D13
          /DTACK 10  |                      | 55  D14
             /BG 11  |       MC68000        | 54  D15
           /BGACK 12 |       (64-Pin)       | 53  GND
             /BR 13  |                      | 52  CLK
             VCC 14  |                      | 51  VCC
            /IPL2 15 |                      | 50  A23
            /IPL1 16 |                      | 49  A22
            /IPL0 17 |                      | 48  A21
            /FC2  18 |                      | 47  A20
            /FC1  19 |                      | 46  A19
            /FC0  20 |                      | 45  A18
             A1   21 |                      | 44  A17
             A2   22 |                      | 43  A16
             A3   23 |                      | 42  A15
             A4   24 |                      | 41  A14
             A5   25 |                      | 40  A13
             A6   26 |                      | 39  A12
             A7   27 |                      | 38  A11
             A8   28 |                      | 37  A10
             A9   29 |                      | 36  A9
             A10  30 |                      | 35  GND
             GND  31 |                      | 34  /VMA
             /HALT 32|                      | 33  /VPA
                     +----------------------+
                 Pins 33-34: 6800 Peripheral Control
```

### 1.1 Address Lines ($A_1$–$A_{23}$) and Data Strobes
* **No $A_0$ Pin:** The CPU addresses 16-bit words externally. To access individual bytes within a word, it replaces $A_0$ with two active-low byte selection strobes:
  * $\overline{\text{UDS}}$ (Upper Data Strobe): Validates the **high byte** (`D8`–`D15`, corresponding to even byte addresses).
  * $\overline{\text{LDS}}$ (Lower Data Strobe): Validates the **low byte** (`D0`–`D7`, corresponding to odd byte addresses).
* **Word Transfers:** Both $\overline{\text{UDS}}$ and $\overline{\text{LDS}}$ assert simultaneously.
* **Alignment Requirement:** Word and longword memory transfers must align on even byte boundaries. Attempting an unaligned word/longword access triggers an internal **Address Error exception**.

### 1.2 Function Codes ($FC_0$–$FC_2$)
The CPU outputs 3 function-code bits to indicate the active privilege state and transaction type:

| $FC_2$ | $FC_1$ | $FC_0$ | Cycle Type | Memory Space |
| :---: | :---: | :---: | :--- | :--- |
| 0 | 0 | 1 | User Data | User mode data read/write |
| 0 | 1 | 0 | User Program | User mode instruction fetch |
| 1 | 0 | 1 | Supervisor Data | Supervisor mode data read/write |
| 1 | 1 | 0 | Supervisor Program | Supervisor mode instruction fetch |
| 1 | 1 | 1 | **CPU Space** | Interrupt acknowledge / breakpoint cycle |

---

## 2. The Asynchronous Handshake ($\overline{\text{AS}}$, $\overline{\text{DTACK}}$, $\overline{\text{BERR}}$)

Unlike synchronous CPUs that sample data at a rigid, predetermined clock tick, the MC68000 uses a fully **asynchronous request/acknowledge handshake**.

```
         S0   S1   S2   S3   S4   S5   S6   S7
CLK    : __/--\__/--\__/--\__/--\__/--\__/--\__/--\_
A1-A23 : ======<=============== Valid ==============>
/AS    : -----------XXXXXXXX\_______________________
/UDS,/LDS: ---------XXXXXXXX\_______________________
R/W    : ======\____________________________________ (Read: High / Write: Low)
/DTACK : -----------------------XXXXXXXX\___________ (From peripheral)
Data   : -----------------------------<=== Valid ===> (Sampled during S6)
```

### 2.1 The Bus Transfer Protocol (Read Cycle Example)
1. **State S0 & S1:** Clock starts cycle. CPU asserts address lines $A_1$–$A_{23}$ and function code lines $FC_0$–$FC_2$.
2. **State S2:** The CPU asserts **$\overline{\text{AS}}$** (Address Strobe low), signaling that the address bus is valid and stable. It sets $R/\overline{W}$ high (Read) or low (Write).
3. **State S3 & S4:** The CPU asserts $\overline{\text{UDS}}$ and/or $\overline{\text{LDS}}$. 
4. **State S4/S5 Wait State Insertion:** The CPU samples the **$\overline{\text{DTACK}}$** (Data Transfer Acknowledge) input on the falling edge of S4.
   * If $\overline{\text{DTACK}}$ is **high**, the processor inserts wait states ($T_W$) indefinitely.
   * If $\overline{\text{DTACK}}$ is **asserted low**, execution proceeds to S6.
5. **State S6:** The CPU latches data from `D0`–`D15` into its internal register.
6. **State S7:** The CPU negates $\overline{\text{AS}}$, $\overline{\text{UDS}}$, and $\overline{\text{LDS}}$. In response, the slave device must negate $\overline{\text{DTACK}}$, ending the bus cycle.

### 2.2 Error and Retry Lines ($\overline{\text{BERR}}$ and $\overline{\text{HALT}}$)
* **$\overline{\text{BERR}}$ (Bus Error):** If an address decoder selects an unpopulated memory space or a hardware fault occurs, external logic (e.g., a watchdog timer) must assert $\overline{\text{BERR}}$ low. The CPU terminates the cycle and invokes the Bus Error exception handler.
* **Bus Retry:** If external logic pulls both **$\overline{\text{BERR}}$ and $\overline{\text{HALT}}$** low simultaneously, the processor aborts the cycle and automatically retries the identical transfer once the lines are released.

---

## 3. Special Memory Addresses & Exception Vectors

The MC68000 reserves the lowest 1024 bytes of memory (**`$000000` to `$0003FF`**) for its 256-entry **Exception Vector Table**. Each vector is a 4-byte (32-bit) longword absolute address.

### 3.1 Primary Hardware Vectors

| Vector Number | Byte Offset (Hex) | Assignment | Description / Usage |
| :---: | :---: | :--- | :--- |
| **0** | `$000000`–`$000003` | **Reset: Initial Interrupt Stack Pointer (ISP)** | Hardwired value loaded into $A_7$ ($SSP$) at reset |
| **1** | `$000004`–`$000007` | **Reset: Initial Program Counter (PC)** | Starting execution address after reset |
| **2** | `$000008`–`$00000B` | **Bus Error** | Asserted via $\overline{\text{BERR}}$ pin or watchdog timer timeout |
| **3** | `$00000C`–`$00000F` | **Address Error** | Unaligned word/longword access to odd byte address |
| **4** | `$000010`–`$000013` | **Illegal Instruction** | Unimplemented/invalid opcode execution |
| **5** | `$000014`–`$000017` | **Zero Divide** | Executing `DIVS` or `DIVU` with divisor = 0 |
| **6** | `$000018`–`$00001B` | `CHK` Instruction | Register out of bounds via `CHK` |
| **7** | `$00001C`–`$00001F` | `TRAPV` Instruction | Overflow trap when $V$ bit in status register is set |
| **8** | `$000020`–`$000023` | **Privilege Violation** | Executing a supervisor-only opcode in user mode |
| **9** | `$000024`–`$000027` | **Trace** | Single-step debugging trap (when $T$-bit is enabled) |
| **10** | `$000028`–`$00002B` | **Line 1010 Emulator** | Opcode begins with `%1010` (Unimplemented instructions) |
| **11** | `$00002C`–`$00002F` | **Line 1111 Emulator** | Opcode begins with `%1111` (Coprocessor / FPU hooks) |
| **12–14** | `$000030`–`$00003B` | *Reserved* | Reserved by Motorola |
| **15** | `$00003C`–`$00003F` | **Uninitialized Interrupt** | Peripheral interrupts before its vector register is programmed |
| **16–23** | `$000040`–`$00005F` | *Reserved* | Reserved by Motorola |
| **24** | `$000060`–`$000063` | **Spurious Interrupt** | Interrupt acknowledged, but neither $\overline{\text{DTACK}}$ nor $\overline{\text{VPA}}$ returned |
| **25–31** | `$000064`–`$00007F` | **Autovector Interrupts 1–7** | Automatically selected when peripheral returns $\overline{\text{VPA}}$ |
| **32–47** | `$000080`–`$0000BF` | **`TRAP #0` through `TRAP #15`** | Software system calls and OS traps |
| **48–63** | `$0000C0`–`$0000FF` | *Reserved* | Reserved by Motorola |
| **64–255** | `$000100`–`$0003FF` | **User Vectored Interrupts** | Peripheral places vector number ($40\text{h}$–$FF\text{h}$) on bus |

---

## 4. Interrupt Architecture

The MC68000 supports **7 priority levels of interrupts** encoded on three active-low pins: $\overline{\text{IPL0}}$, $\overline{\text{IPL1}}$, and $\overline{\text{IPL2}}$.

```
   Priority Level   \IPL2   \IPL1   \IPL0    Classification
   --------------------------------------------------------
   0 (None)           1       1       1      Normal execution
   1                  1       1       0      Maskable
   2                  1       0       1      Maskable
   3                  1       0       0      Maskable
   4                  0       1       1      Maskable
   5                  0       1       0      Maskable
   6                  0       0       1      Maskable
   7 (Highest)        0       0       0      Non-Maskable (Level 7 NMI)
```

* **Masking Rules:** A pending interrupt level is acknowledged only if its priority value is strictly greater than the 3-bit interrupt mask field ($I_2, I_1, I_0$) in the Status Register ($SR$).
* **Level 7 Exemption:** Priority Level 7 is **non-maskable (NMI)**. It executes even if the Status Register mask is set to `7`.

### 4.1 The Interrupt Acknowledge (IACK) Cycle
When servicing an interrupt:
1. The CPU initiates a special read cycle setting function codes $FC_2, FC_1, FC_0 = 1, 1, 1$ (CPU Space).
2. Address lines $A_1$–$A_3$ output the interrupt priority level being serviced ($1$ to $7$). Lines $A_4$–$A_{23}$ are driven high.
3. The peripheral must respond in one of two ways:
   * **Vectored Interrupt:** The peripheral drives an 8-bit vector number ($64$–$255$) onto `D0`–`D7` and asserts **$\overline{\text{DTACK}}$**. The CPU reads the vector and jumps through the corresponding entry in the table.
   * **Autovectored Interrupt:** The peripheral asserts **$\overline{\text{VPA}}$** (Valid Peripheral Address) low instead of $\overline{\text{DTACK}}$. The CPU ignores the data bus and routes execution directly to the designated Autovector entry (Vectors 25–31).

---

## 5. Byte & Word Access Decoding Logic

Because the MC68000 uses independent byte strobes rather than a dedicated $A_0$ pin, memory and peripheral chips must be connected based on their word width.

### 5.1 Connecting 8-Bit Peripherals and Memory
To connect two 8-bit SRAM chips across the 16-bit bus:
* Connect $A_1$ of the CPU to $A_0$ of both RAM chips, $A_2$ to $A_1$, etc.
* Connect the **Upper Byte RAM** to `D8`–`D15` and chip select / write enable qualified by $\overline{\text{UDS}}$.
* Connect the **Lower Byte RAM** to `D0`–`D7` and chip select / write enable qualified by $\overline{\text{LDS}}$.

```
CPU Signals                  Upper Byte (Even Addresses)
----------------             +-------------------------+
D8-D15  ====================>| D0-D7                   |
A1-Axx  ====================>| A0-A(xx-1)              |
/UDS    ----------+--------->| /CS (or /WE)            |
R/W     ---+      |          +-------------------------+
           |      |
           |      |          Lower Byte (Odd Addresses)
           |      |          +-------------------------+
D0-D7   ===|======|=========>| D0-D7                   |
A1-Axx  ===|======|=========>| A0-A(xx-1)              |
/LDS    ---|------|----+---->| /CS (or /WE)            |
           |      |          +-------------------------+
           +------+---------> Common R/W
```

### 5.2 Glitch-Free Memory Strobes
Generate clean chip enables and write strobes using combinational logic:

$$
\overline{\text{WE}_{\text{UPPER}}} = \overline{\text{UDS}} \lor \text{Write} \quad (\text{Low when } \overline{\text{UDS}}=0 \text{ and } R/\overline{W}=0)
$$

$$
\overline{\text{WE}_{\text{LOWER}}} = \overline{\text{LDS}} \lor \text{Write} \quad (\text{Low when } \overline{\text{LDS}}=0 \text{ and } R/\overline{W}=0)
$$

---

## 6. Interfacing 6800 Synchronous Peripherals

To maintain backward compatibility with legacy 8-bit Motorola 6800 peripherals (such as the MC6821 PIA or MC6850 ACIA), the MC68000 includes dedicated synchronization hardware:

* **$E$ (Enable Clock):** Fixed output clock running at **$\frac{1}{10}$th of the MC68000 master clock frequency** with a $60/40$ duty cycle.
* **$\overline{\text{VPA}}$ (Valid Peripheral Address):** Driven low by address decoding logic to tell the CPU that the selected address belongs to an 8-bit 6800 peripheral.
* **$\overline{\text{VMA}}$ (Valid Memory Address):** Asserted low by the CPU to indicate synchronization with the $E$ clock, providing the required setup and hold times for legacy synchronous chips.

---

## 7. Power-On Reset & Bootstrapping Circuitry

### 7.1 Reset Timing
* To guarantee a cold reset, **both $\overline{\text{RESET}}$ and $\overline{\text{HALT}}$ must be pulled low simultaneously** for a minimum of **$100\text{ ms}$** after $V_{CC}$ reaches specification.
* Holding $\overline{\text{RESET}}$ low without asserting $\overline{\text{HALT}}$ executes a software-initiated reset cycle (`RESET` instruction), which turns the $\overline{\text{RESET}}$ pin into an **output** to reset external peripherals while leaving internal CPU registers untouched.

### 7.2 Boot-ROM Overlay Hardware Trap
Because the CPU reads vectors 0 and 1 from `$000000`–`$000007` upon boot:
* System designers must place a ROM at address `$000000` initially.
* However, most operating systems (Unix, AmigaOS, Atari TOS) expect high-speed read/write RAM at `$000000` to quickly dynamically patch the exception table.
* **Solution:** Implement a simple 74LS74 flip-flop circuit ("Boot Flip-Flop") that maps ROM to `$000000` upon hardware reset. After the CPU reads the initial ISP and PC, an initial write access or a specific address strobe trips the flip-flop, permanently swapping physical RAM into `$000000` and relocating ROM to its permanent base (e.g., `$E00000` or `$FC0000`).