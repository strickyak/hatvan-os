# Optimizing the RCA COSMAC CDP1802 Backend for Hatvan OS

**Analysis of Binary Sizes, Code Expansion Multipliers, and Optimization Strategies for Tasks 0, 1, and 2**

---

## 1. Executive Summary & Binary Size Reality

When compiling Hatvan OS Tasks 0, 1, and 2 with the MiniGolf compiler targeting the RCA COSMAC CDP1802 (`-m=1802`) and assembling with `asm1802`, the resulting binary sizes exceed the physical 64 KB memory capacity of the architecture:

| Task | Component | Motorola 6809 | Zilog Z80 | **RCA CDP1802** | Available RAM on 1802 | Status on 1802 |
| :---: | :--- | :---: | :---: | :---: | :---: | :---: |
| **Task 0** | **Kernel** (`main.golf`) | 26 KB | 44 KB | **126 KB** (128,220 B) | ~55 KB ($0100..$DFFF) | ❌ **FATAL (+130% overflow)** |
| **Task 1** | **RBF Driver** (`drivers/rbf`) | 12 KB | 25 KB | **61 KB** (62,250 B) | 56 KB ($0000..$DFFF) | ❌ **OVERFLOW (+9% overflow)** |
| **Task 2** | **PROCFS Driver** (`drivers/procfs`) | 2.9 KB | 6.7 KB | **15 KB** (15,105 B) | 56 KB ($0000..$DFFF) |  **FITS (27% capacity)** |

### Key Observations
* **Task 0 (Kernel)** at **128,220 bytes (~126 KB)** is nearly **double the entire 64 KB physical address space** of the 1802 processor. In a system where `$E000..$FEFF` is reserved for shared driver curtain tables and `$FF00..$FFFF` is memory-mapped I/O, the kernel has at most ~55 KB of available space.
* **Task 1 (RBF Disk Driver)** at **62,250 bytes (~61 KB)** overflows past the `$E000` shared memory curtain by approximately **5.5 KB**.
* **Task 2 (PROCFS Driver)** at **15,105 bytes (~15 KB)** fits within the 56 KB driver ceiling, but is still **$5.2\times$ larger** than the 6809 implementation (2.9 KB) and **$2.2\times$ larger** than the Z80 implementation (6.7 KB).

The **1802 Code Expansion Multiplier** is approximately **$4.8\times$ vs. 6809** and **$2.6\times$ vs. Z80**.

---

## 2. Root Cause Analysis: Why 1802 Code Explodes

Inspecting the 100,587 lines of generated assembly in `build/kernel_1802.asm` reveals five primary architectural friction points between the MiniGolf compiler and the 1802 execution model:

### 2.1 Lack of Displacement Addressing for Local Variables
The 1802 has no instructions for indexing a base register with a constant offset (e.g. `ldd 4,s` on 6809 or `ld l, (ix+4)` on Z80).
To read a single 16-bit local variable at offset 5 from the frame pointer $R(\text{B})$:

```assembly
    ; Reading a 16-bit variable at RB + 5:
    GLO     RB          ; 1 byte
    ADI     5           ; 2 bytes: add low byte
    PLO     RC          ; 1 byte: store low pointer
    GHI     RB          ; 1 byte
    ADCI    0           ; 2 bytes: add carry
    PHI     RC          ; 1 byte: store high pointer
    LDA     RC          ; 1 byte: read high byte, advance RC
    PHI     R7          ; 1 byte
    LDN     RC          ; 1 byte: read low byte
    PLO     R7          ; 1 byte
    ; TOTAL: 10 instructions, 12 bytes!
```
Compare this to 6809:
```assembly
    ldd     5,u         ; 1 instruction, 2 bytes
```
And Z80:
```assembly
    ld      l, (ix+5)   ; 2 instructions, 6 bytes
    ld      h, (ix+6)
```
On the 1802, local variable indexing is **$6\times$ larger than 6809** in machine code.

---

### 2.2 16-Bit Pointer and Constant Loading (`LOAD Rn, addr`)
Every 16-bit address or immediate value requires 4 instructions and 6 bytes:
```assembly
    LDI     HIGH(addr)  ; 2 bytes
    PHI     Rn          ; 1 byte
    LDI     LOW(addr)   ; 2 bytes
    PLO     Rn          ; 1 byte
```
Because the compiler frequently loads addresses of globals, string literals, jump tables, and hardware ports, this 6-byte expansion appears thousands of times across the binary.

---

### 2.3 Synthesized 16-Bit ALU Arithmetic
The 1802 has only an 8-bit accumulator ($D$) and no 16-bit arithmetic instructions (`add hl, de` on Z80 or `addr d, x` on 6809).
Adding two 16-bit variables ($R7 = R7 + R8$) requires spilling to scratch RAM or running byte-by-byte through $D$:
```assembly
    GLO     R7
    STR     R2          ; push low byte
    GLO     R8
    ADD                 ; D = low sum, DF = carry
    PLO     R7
    GHI     R7
    STR     R2          ; push high byte
    GHI     R8
    ADC                 ; D = high sum + carry
    PHI     R7
    ; 10 instructions, 10 bytes!
```

---

### 2.4 Heavy Stack Frame Prologue & Epilogue
Because the 1802 has no hardware stack instructions for 16-bit registers (`PSHS` on 6809 or `PUSH` on Z80), every function with local variables must execute a manual frame setup:

```assembly
; Function Prologue (12 bytes):
    GLO     RB
    STXD                ; push RB.0 to stack R2
    GHI     RB
    STXD                ; push RB.1 to stack R2
    GHI     R2
    PHI     RB          ; RB = R2 (set frame pointer)
    GLO     R2
    PLO     RB

; Function Epilogue (16 bytes):
    GHI     RB
    PHI     R2          ; R2 = RB (deallocate locals)
    GLO     RB
    PLO     R2
    INC     R2          ; pop RB.1
    LDA     R2
    PHI     RB
    LDN     R2          ; pop RB.0
    PLO     RB
    SEP     R5          ; return via SCRT
```
For small leaf functions (like `_putchar`, `_getchar`, or simple getters), the prologue and epilogue dwarf the actual function body by a factor of 4.

---

### 2.5 SCRT Function Linkage Overhead
Every subroutine call is mediated by RCA's Standard Call and Return Technique (SCRT):
```assembly
    SEP     R4          ; 1 byte: Switch P to _CALL engine
    DW      target      ; 2 bytes: Inlined target address
```
While compact at the call site (3 bytes), the SCRT engine itself and the stack manipulation to save and restore the caller's linkage register ($R6$) add significant overhead to every call/return sequence.

---

## 3. Priority Optimization Roadmap

To shrink the 1802 kernel from **126 KB down to below 50 KB** (a $\sim 60\%$ reduction), optimizations should be tackled in order of impact:

```
+-------------------------------------------------------------------+
|               1802 Code Size Optimization Priority                |
+-------------------------------------------------------------------+
|  1. Fastcall Register Argument Passing       [-25% to -30% size]  |
|  2. Leaf Function Frame Elision              [-12% to -15% size]  |
|  3. Dedicated Scratch Register for Offsets   [-10% to -15% size]  |
|  4. Helper Routines for 16-bit ALU Ops       [-8% to -12% size]   |
|  5. Stripping Debug Printf & Kernel Strings  [-5% to -8% size]    |
+-------------------------------------------------------------------+
```

### Strategy 1: Fastcall Register Calling Convention (Estimated Gain: 25–30%)
* **Current Behavior**: Function arguments are pushed onto the stack ($R2$) and then loaded by the callee using $R(\text{B}) + \text{offset}$ arithmetic.
* **Optimization**:
  * **Argument 1 (16-bit / pointer)**: Passed directly in **$R(7)$**.
  * **Argument 2 (16-bit / pointer)**: Passed directly in **$R(8)$**.
  * **Byte Arguments**: Passed in **$D$**.
  * **Return Value**: Returned in **$R(7)$** (or **$D$** if byte).
* **Impact**: Eliminates both the parameter push sequences at the call site and the expensive 10-instruction offset reads inside the callee for 80% of all functions.

---

### Strategy 2: Leaf Function Frame Elision (Estimated Gain: 12–15%)
* **Current Behavior**: Every function unconditionally saves and restores $R(\text{B})$, sets up a new stack frame, and exits via SCRT `SEP R5`.
* **Optimization**:
  * Functions that do not call any other functions (leaf functions) and use 2 or fewer local variables do not allocate an $R(\text{B})$ frame.
  * Direct return via `SEP R6` or the linkage register without stack manipulation.
* **Impact**: Saves 28 bytes of prologue/epilogue per leaf function.

---

### Strategy 3: Dedicated Scratch Register for Frame Offsets (Estimated Gain: 10–15%)
* **Current Behavior**: Computing $R(\text{B}) + \text{offset}$ recalculates both the high byte and low byte through $D$ with `ADI` and `ADCI 0`.
* **Optimization**:
  * For small offsets ($\le 15$), if $R(\text{B})$ does not cross a 256-byte boundary, only the low byte needs adjustment:
    ```assembly
    GLO     RB
    ADI     offset
    PLO     RC
    GHI     RB
    PHI     RC          ; Skip ADCI when page crossing is impossible
    ```
  * Or pre-index using `IRX` / register increments.

---

### Strategy 4: Out-of-Line 16-Bit Arithmetic Helpers (Estimated Gain: 8–12%)
* **Current Behavior**: Every 16-bit add, subtract, and comparison is inlined as 8–12 instructions.
* **Optimization**:
  * Implement compact out-of-line helpers for common 16-bit primitives:
    * `__add16`: $R7 = R7 + R8$
    * `__sub16`: $R7 = R7 - R8$
    * `__cmp16`: Compare $R7$ and $R8$, set $DF$ and condition
  * A 3-byte call (`SEP R4; DW __add16`) replaces 10–12 bytes of inlined code.

---

### Strategy 5: Kernel Partitioning & String Stripping (Estimated Gain: 5–8%)
* **Current Behavior**: Task 0 contains extensive diagnostic format strings, error messages, and full `_printf` formatting.
* **Optimization**:
  * Strip verbose debug strings in release builds.
  * Move non-essential kernel subsystems (e.g. secondary filesystem drivers) into dedicated driver tasks (Task 3, Task 4), taking advantage of Hatvan's 256-task MMU isolation.

---

## 4. Target Size Projections

With the proposed optimizations implemented:

| Component | Current 1802 Size | Projected Optimized Size | Limit | Result |
| :--- | :---: | :---: | :---: | :---: |
| **Task 0 (Kernel)** | 126 KB | **~42–48 KB** | 55 KB |  **FITS** |
| **Task 1 (RBF Driver)**| 61 KB | **~20–24 KB** | 56 KB |  **FITS** |
| **Task 2 (PROCFS)** | 15 KB | **~5–6 KB** | 56 KB |  **FITS** |

---

## 5. Conclusion

The RCA CDP1802 is fundamentally an 8-bit pointer machine without 16-bit arithmetic or indexed addressing. Unoptimized code generation results in a steep **$\sim 4.8\times$ code expansion multiplier** compared to orthogonal CISC architectures like the Motorola 6809. 

However, because the majority of the expansion is concentrated in **frame-pointer offset arithmetic** and **parameter passing on the stack**, adopting a **Fastcall register-argument convention** and **leaf frame elision** will cut binary size by more than half, allowing all three Hatvan core tasks to fit comfortably inside their 64 KB memory spaces.
