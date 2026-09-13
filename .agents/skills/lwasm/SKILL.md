---
name: lwasm
description: >-
  Instructions, command-line conventions, flags, and syntax guidelines for assembling
  Motorola 6809 and Hitachi 6309 assembly source code using the lwasm assembler.
  Use when assembling 6809/6309 source files, generating DECB or OS-9 binaries,
  creating assembly listing and symbol map files, or troubleshooting lwasm assembly errors.
---

# Assembling with lwasm

`lwasm` is the LWTOOLS cross-assembler for the Motorola 6809 and Hitachi 6309 microprocessors. On this system, `lwasm` is available in `PATH` (at `/home/strick/modoc/coco-shelf/bin/lwasm`).

## Common Invocations

### 1. Assemble to DECB Format (for Hatvan VM)
To assemble a program into Radio Shack Color Computer DECB format (`.decb`) with an assembly listing for debugging:

```bash
lwasm --decb --list=output.list -o output.decb input.asm
```

Alternatively, explicitly using the `--format` flag:
```bash
lwasm --format=decb --list=output.list -o output.decb input.asm
```

### 2. Assemble for Hitachi 6309 CPU
By default, `lwasm` targets the Motorola 6809. Pass `--6309` to enable native Hitachi 6309 opcodes and registers (`W, E, F, V, Q, MD`):

```bash
lwasm --6309 --decb --list=output.list -o output.decb input.asm
```

### 3. Assemble an OS-9 Module
To generate an OS-9 binary module (using `mod ... emod` syntax):

```bash
lwasm --format=os9 --list=module.list -o module.mod module.asm
```

Standard pragmatic flags used across TurbOS and NitrOS-9 builds:
```bash
lwasm --6309 --format=os9 \
  --pragma=pcaspcr,nosymbolcase,condundefzero,undefextern,dollarnotlocal,noforwardrefmax \
  --includedir=. -o module.mod module.asm
```

## Critical Syntax Guidelines & Gotchas

1. **Whitespace Delimits Comments (No Spaces in Operands):**
   In 6809/6309 assembly syntax, a space character terminates the operand field and starts the comment field.
   * **Correct:**
     ```assembly
     fcb 10,0
     pshs a,b,x
     ldd #59*256+1
     ```
   * **Wrong (will cause syntax errors or truncated operands):**
     ```assembly
     fcb 10, 0        ; ERROR: space before 0
     pshs a, b, x     ; ERROR: space before b
     ```

2. **Program Origin and Entry Point:**
   * In DECB binaries, specify the load address with `org <address>`.
   * Specify the start execution address in the `end` directive:
     ```assembly
              org $2000
     start    lda #42
              ...
              end start
     ```
     The address associated with `end` will populate the DECB `$FF` trailer and become the Reset vector in `hatvan-vm`.

3. **PC-Relative Addressing:**
   * Use `,pcr` (or `,pc`) for position-independent memory addressing:
     ```assembly
     leax msg,pcr
     ```

4. **Listing Files (`.list`):**
   * Always generate listings using `--list=<filename>.list`.
   * These listing files contain address mappings and source code lines that `hatvan-vm` parses to annotate execution traces (`--trace`).
   * *Note on wrapper script:* The local `lwasm` executable may run a wrapper that automatically appends `--map` and `--list` flags if not already provided.
