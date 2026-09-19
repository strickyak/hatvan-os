# Hatvan gepk: Implementation Overview

This document provides an overview of the clean-room Motorola 68000 CPU emulator and **Hatvan gepk** (`gepk`) virtual machine implemented in Go, complying with [`spec-hatvan-68000.md`](../spec-hatvan-68000.md).

---

## 1. Memory & Bus Subsystem (`gepk/bus.go`)

- **24-bit physical address space** with 256 isolated task spaces (`[256]*TaskMemory`), dynamically allocated via sparse 64 KB pages.
- **Privilege-based memory routing**:
  - Supervisor accesses (`FC2 = 1`) map directly to **Task 0**.
  - User accesses (`FC2 = 0`) map to the active user task configured in the **Task Register** (`$00FF0020`).
- **Strict memory protection**: Any user mode access (`FC2 = 0`) to `$00FF0000..$00FFFFFF` triggers an immediate bus fault (`ErrUserAccessTrap`).
- **Alignment enforcement**: Word and longword accesses to odd addresses assert `ErrAddressError` (Vector 3).
- **I/O Device Registers** (Word-aligned at `$00FF0000..$00FF003F`):
  - Console Out (`$00FF0000`): writing an ASCII byte emits to stdout.
  - Console In (`$00FF0002`): non-blocking read from simulated console keyboard.
  - Status/Control (`$00FF0004` / `$00FF0006`): hardware status and interrupt enable masks.
  - Log Output (`$00FF0008`): writing an ASCII byte emits to stderr.
  - Exit Code (`$00FF000A`): writing a 16-bit status terminates VM execution with that exit code.
  - 32-bit Disk Controller (`$00FF0014..$00FF001E`): sector LSN, target task, memory address, and command/status.
  - Fast DMA Transfer Engine (`$00FF0022..$00FF0032`): high-speed block copies between tasks.
- **Autovector interrupt controller**: Evaluates hardware interrupt lines, prioritizing Level 6 (60 Hz Timer Tick) and Level 4 (Console Receive Ready).

---

## 2. CPU Architecture & Dual Stacks (`gepk/cpu.go`)

- **Registers**:
  - Eight 32-bit Data registers: `D0-D7`.
  - Eight 32-bit Address registers: `A0-A7`.
  - Dual hardware stack pointers: `A7` transparently selects `SSP` (Supervisor Stack Pointer) when `SR[S] = 1` and `USP` (User Stack Pointer) when `SR[S] = 0`.
  - Program Counter: `PC` (24-bit active address space).
- **Status Register (`SR`)**:
  - Full 16-bit register: `T` (Trace), `S` (Supervisor), `IPL[2:0]` (Interrupt Priority Mask), `X` (Extend), `N` (Negative), `Z` (Zero), `V` (Overflow), `C` (Carry).
- **Exception Processing**:
  - Hardware vector table (`$000000..$0003FF`).
  - Automatic `PC` (32-bit) and `SR` (16-bit) supervisor stack frame pushes on traps and exceptions.
  - Automatic privilege transition and stack pointer swaps.

---

## 3. Effective Addressing Engine (`gepk/addressing.go`)

- Implements all 12 Motorola 68000 Effective Addressing modes:
  1. Data Register Direct: `Dn` (mode 0)
  2. Address Register Direct: `An` (mode 1)
  3. Address Register Indirect: `(An)` (mode 2)
  4. Address Register Indirect with Postincrement: `(An)+` (mode 3)
  5. Address Register Indirect with Predecrement: `-(An)` (mode 4)
  6. Address Register Indirect with 16-bit Displacement: `(d16, An)` (mode 5)
  7. Address Register Indirect with Index & 8-bit Displacement: `(d8, An, Xn)` (mode 6)
  8. Absolute Short: `(xxx).W` (mode 7, reg 0, sign-extended to 32 bits)
  9. Absolute Long: `(xxx).L` (mode 7, reg 1)
  10. Program Counter with 16-bit Displacement: `(d16, PC)` (mode 7, reg 2)
  11. Program Counter with Index & 8-bit Displacement: `(d8, PC, Xn)` (mode 7, reg 3)
  12. Immediate Data: `#<data>` (mode 7, reg 4)
- **Hardware Quirk**: In modes `(A7)+` and `-(A7)`, byte operations (`.B`) postincrement or predecrement by **2** (instead of 1) to maintain word alignment on the active stack.
- **Two-phase Commit**: Postincrement side-effects are deferred until after operand reads and writes to eliminate double-increment bugs during read-modify-write operations.

---

## 4. ALU, CCR Flags, & Arithmetic (`gepk/alu.go`)

- **Bit-accurate flag calculations** for all condition codes (`X`, `N`, `Z`, `V`, `C`).
- **Basic Arithmetic & Logic**: `ADD`, `SUB`, `CMP`, `NEG`, `NOT`, `AND`, `OR`, `EOR`, `TST`, `CLR`.
- **Multi-Precision Arithmetic**: `ADDX`, `SUBX`, `NEGX` strictly adhere to the M68000 architectural rule where the `Z` flag is cleared on non-zero results, but preserved unchanged on zero results.
- **Shifts and Rotates**: `ASL`, `ASR`, `LSL`, `LSR`, `ROL`, `ROR`, `ROXL`, `ROXR` with accurate carry and extend behavior.
- **Multiplication & Division**: `MULU`, `MULS`, `DIVU`, `DIVS` (with overflow flag `V` and Zero Divide exception Vector 5).
- **Decimal Arithmetic**: Packed BCD operations `ABCD` and `SBCD`.
- **Condition Code Evaluation**: Evaluates all 16 condition codes (0..15) for branches, conditional sets (`Scc`), and loop decrements (`DBcc`).

---

## 5. Instruction Decoder & Dispatcher (`gepk/ops.go`, `gepk/dispatch.go`)

Covers all 16 opcode groups (`0` through `F`):

| Group | Instructions |
| :--- | :--- |
| **0 (`0000`)** | Static/Dynamic bit operations (`BTST`, `BSET`, `BCLR`, `BCHG`), Immediate ALU (`ADDI`, `SUBI`, `ANDI`, `ORI`, `EORI`, `CMPI`), `MOVEP`, and `ORI`/`ANDI`/`EORI` to `CCR`/`SR`. |
| **1 (`0001`)** | `MOVE.B` |
| **2 (`0010`)** | `MOVE.L`, `MOVEA.L` |
| **3 (`0011`)** | `MOVE.W`, `MOVEA.W` |
| **4 (`0100`)** | `LEA`, `PEA`, `MOVEM`, `LINK`, `UNLK`, `SWAP`, `EXT.W`, `EXT.L`, `TAS`, `TRAP #0..15`, `TRAPV`, `CHK`, `JMP`, `JSR`, `RTS`, `RTE`, `RTR`, `MOVE USP`, `MOVE to/from SR`, `MOVE to CCR`, `RESET`, `STOP`, `NOP`. |
| **5 (`0101`)** | `ADDQ`, `SUBQ`, `Scc`, `DBcc`. |
| **6 (`0110`)** | `BRA`, `BSR`, `Bcc` (all 14 conditions: `HI`, `LS`, `CC`, `CS`, `NE`, `EQ`, `VC`, `VS`, `PL`, `MI`, `GE`, `LT`, `GT`, `LE`). |
| **7 (`0111`)** | `MOVEQ #<data>, Dn`. |
| **8 (`1000`)** | `OR`, `DIVU`, `DIVS`, `SBCD`. |
| **9 (`1001`)** | `SUB`, `SUBA`, `SUBX`. |
| **A (`1010`)** | Line 1010 Emulator Exception (Vector 10). |
| **B (`1011`)** | `CMP`, `CMPA`, `CMPM`, `EOR`. |
| **C (`1100`)** | `AND`, `MULU`, `MULS`, `ABCD`, `EXG`. |
| **D (`1101`)** | `ADD`, `ADDA`, `ADDX`. |
| **E (`1110`)** | Shifts and rotates (register and memory variants). |
| **F (`1111`)** | Line 1111 Emulator Exception (Vector 11). |

---

## 6. S-Records & Disassembly (`gepk/srec.go`, `gepk/disasm.go`)

- **Motorola S-Record Loader**: Automatically parses `.s19`, `.s28`, `.s37`, and `.srec` files, verifying checksums and extracting entry point addresses from `S7`/`S8`/`S9` records.
- **Instruction Disassembler**: Real-time disassembly engine formatting opcodes, effective addresses, and branch targets for `--trace` diagnostic logging.

---

## 7. CLI Executable (`cmd/gepk/main.go`)

- **Command Line Flags**:
  - `--trace`: output instruction execution trace to `stderr`.
  - `--max-cycles`: limit maximum simulated cycles (0 = unlimited).
  - `--max-seconds`: real-time timeout limit in seconds (default 300).
  - `--tick-hz`: timer interrupt rate in Hz (default 60).
  - `--cpu-hz`: simulated CPU clock frequency (default 8,000,000 Hz).
  - `--input`: initial console input string queued to simulated stdin.
  - `--disk0..3`: attached raw disk image files.
  - `--base`: base memory loading address for raw binary images.
  - `--print-cycles`: print total cycles executed on termination.
- **Console Terminal Integration**: Non-blocking cooked stdin channel handoff after queued `--input` exhaustion.

---

## 8. Verification Results

Unit and integration tests pass cleanly:

```
=== RUN   TestAddressingDirect
--- PASS: TestAddressingDirect (0.00s)
=== RUN   TestAddressingIndirect
--- PASS: TestAddressingIndirect (0.00s)
=== RUN   TestAddressingPostincPredec
--- PASS: TestAddressingPostincPredec (0.00s)
=== RUN   TestAddressingDisplacementAndIndex
--- PASS: TestAddressingDisplacementAndIndex (0.00s)
=== RUN   TestAddressingSpecialModes
--- PASS: TestAddressingSpecialModes (0.00s)
=== RUN   TestALUAddSubFlags
--- PASS: TestALUAddSubFlags (0.00s)
=== RUN   TestALUAddXSubXZeroFlag
--- PASS: TestALUAddXSubXZeroFlag (0.00s)
=== RUN   TestEvaluateCondition
--- PASS: TestEvaluateCondition (0.00s)
=== RUN   TestALUShiftsRotates
--- PASS: TestALUShiftsRotates (0.00s)
=== RUN   TestALUMulDiv
--- PASS: TestALUMulDiv (0.00s)
=== RUN   TestALUBCD
--- PASS: TestALUBCD (0.00s)
=== RUN   TestBusMemoryAccess
--- PASS: TestBusMemoryAccess (0.00s)
=== RUN   TestBusAddressError
--- PASS: TestBusAddressError (0.00s)
=== RUN   TestBusUserAccessProtection
--- PASS: TestBusUserAccessProtection (0.00s)
=== RUN   TestBusTaskRouting
--- PASS: TestBusTaskRouting (0.00s)
=== RUN   TestBusDMAEngine
--- PASS: TestBusDMAEngine (0.00s)
=== RUN   TestCPUReset
--- PASS: TestCPUReset (0.00s)
=== RUN   TestCPUPrivilegeSwap
--- PASS: TestCPUPrivilegeSwap (0.00s)
=== RUN   TestCPUTrapAndRTE
--- PASS: TestCPUTrapAndRTE (0.00s)
=== RUN   TestCPUEndToEndProgram
--- PASS: TestCPUEndToEndProgram (0.00s)
=== RUN   TestLoadSRecords
--- PASS: TestLoadSRecords (0.00s)
=== RUN   TestHatvanVMKHello
--- PASS: TestHatvanVMKHello (0.01s)
PASS
```
