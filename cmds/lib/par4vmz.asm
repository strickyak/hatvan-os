; ==============================================================================
; Hatvan OS/Z: Zilog Z80 Userland Par4 Bytecode Interpreter (par4vmz.asm)
; Native Little-Endian RAM Words, Big-Endian .p4p Bytecode Stream
; ==============================================================================
;
; Register Allocation:
;   BC  = P4PC: Par4 Bytecode Program Counter
;   IX  = P4SP: Par4 16-bit Operand Stack Pointer (grows down from $D000)
;   IY  = P4FP: Par4 Local Frame Pointer (grows down from $F000)
;   SP  = Z80 Hardware Call Stack Pointer (grows down from userSP ~$FDxx)
;   EXX = Used inside loop/block handlers (MEM_COPY, MUL, DIV, etc.) to
;         preserve BC (P4PC) in BC' in 4 T-states while freeing BC/DE/HL.
; ==============================================================================

    org 0x0500

start:
cstart:
    ; Store command-line ArgsPtr (HL) in big-endian at $006C..$006D for sys.Args():
    ld   a, h
    ld   (0x006C), a
    ld   a, l
    ld   (0x006D), a

    jmp  _user_start

; ── Core Dispatch Loop ────────────────────────────────────────────────────────
DispatchNext:
    ld   a, (bc)
    inc  bc
    ld   l, a
    ld   h, 0
    add  hl, hl
    ld   de, OpcodeTable
    add  hl, de
    ld   e, (hl)
    inc  hl
    ld   d, (hl)
    ex   de, hl
    jp   (hl)

; ── Opcode Handlers ───────────────────────────────────────────────────────────

Op_NOP:
    jmp  DispatchNext

Op_PUSH_0:
Op_PUSH_FALSE:
Op_PUSH_NIL:
    dec  ix
    dec  ix
    ld   (ix+0), 0
    ld   (ix+1), 0
    jmp  DispatchNext

Op_PUSH_1:
Op_PUSH_TRUE:
    dec  ix
    dec  ix
    ld   (ix+0), 1
    ld   (ix+1), 0
    jmp  DispatchNext

Op_PUSH_NEG1:
    dec  ix
    dec  ix
    ld   (ix+0), 0xFF
    ld   (ix+1), 0xFF
    jmp  DispatchNext

Op_PUSH_I8:
    ld   a, (bc)
    inc  bc
    ld   e, a
    rla
    sbc  a, a
    ld   d, a
    jmp  _push_de

Op_PUSH_U8:
    ld   a, (bc)
    inc  bc
    ld   e, a
    ld   d, 0
    jmp  _push_de

Op_PUSH_I16:
    ld   a, (bc)
    inc  bc
    ld   d, a
    ld   a, (bc)
    inc  bc
    ld   e, a
_push_de:
    dec  ix
    dec  ix
    ld   (ix+0), e
    ld   (ix+1), d
    jmp  DispatchNext

Op_PUSH_STR:
    ; 16-bit big-endian offset into StringPool
    ld   a, (bc)
    inc  bc
    ld   d, a
    ld   a, (bc)
    inc  bc
    ld   e, a
    ld   hl, (_str_pool_base_addr)
    add  hl, de                 ; HL points to 2-byte big-endian string length
    ld   d, (hl)
    inc  hl
    ld   e, (hl)                ; DE = length
    inc  hl                     ; HL = string data pointer
    ; Push slice [ptr (HL), cap (DE), len (DE)] onto IX:
    push de
    ld   de, 0xFFFA             ; -6
    add  ix, de
    pop  de
    ld   (ix+4), l
    ld   (ix+5), h              ; ptr
    ld   (ix+2), e
    ld   (ix+3), d              ; cap
    ld   (ix+0), e
    ld   (ix+1), d              ; len
    jmp  DispatchNext

Op_PUSH_NIL_SLICE:
    ld   de, 0xFFFA             ; -6
    add  ix, de
    xor  a
    ld   (ix+0), a
    ld   (ix+1), a
    ld   (ix+2), a
    ld   (ix+3), a
    ld   (ix+4), a
    ld   (ix+5), a
    jmp  DispatchNext

Op_POP:
    inc  ix
    inc  ix
    jmp  DispatchNext

Op_POP_SLICE:
    ld   de, 6
    add  ix, de
    jmp  DispatchNext

Op_DUP:
    ld   e, (ix+0)
    ld   d, (ix+1)
    jmp  _push_de

Op_SWAP:
    ld   e, (ix+0)
    ld   d, (ix+1)
    ld   l, (ix+2)
    ld   h, (ix+3)
    ld   (ix+0), l
    ld   (ix+1), h
    ld   (ix+2), e
    ld   (ix+3), d
    jmp  DispatchNext

Op_DUP2:
    ld   e, (ix+0)
    ld   d, (ix+1)
    ld   l, (ix+2)
    ld   h, (ix+3)
    dec  ix
    dec  ix
    dec  ix
    dec  ix
    ld   (ix+2), l
    ld   (ix+3), h
    ld   (ix+0), e
    ld   (ix+1), d
    jmp  DispatchNext

Op_ROT:
    ; Rotate top 3 words: [C, B, A] (A at ix+0..1, B at ix+2..3, C at ix+4..5) -> [A, C, B]
    ld   e, (ix+0)
    ld   d, (ix+1)              ; DE = A (top)
    ld   l, (ix+2)
    ld   h, (ix+3)              ; HL = B (2nd)
    ld   a, (ix+4)
    ld   (ix+2), a
    ld   a, (ix+5)
    ld   (ix+3), a              ; C -> 2nd
    ld   (ix+4), e
    ld   (ix+5), d              ; A -> 3rd
    ld   (ix+0), l
    ld   (ix+1), h              ; B -> top
    jmp  DispatchNext

; ── Local Variable Access ─────────────────────────────────────────────────────

; Input: A = local slot index
; Output: HL = address of local slot in frame, A = size of slot in bytes
; Preserves: BC (via caller EXX)
_resolve_local_exx:
    push iy
    pop  hl                     ; HL = FP
    ld   de, (_var_table_ptr)
    or   a
    jr   z, .res_loc_done
    ld   b, a
    ld   d, 0
    push de
    ld   de, (_var_table_ptr)
.res_loc_loop:
    ld   a, (de)
    inc  de
    ld   c, a
    push bc
    ld   b, 0
    add  hl, bc
    pop  bc
    djnz .res_loc_loop
    pop  bc                     ; discard saved DE
.res_loc_done:
    ld   a, (de)                ; A = slot size in bytes
    ret

Op_LOAD_LOCAL_0:
    xor  a
    jr   _do_load_local
Op_LOAD_LOCAL_1:
    ld   a, 1
    jr   _do_load_local
Op_LOAD_LOCAL_2:
    ld   a, 2
    jr   _do_load_local
Op_LOAD_LOCAL_3:
    ld   a, 3
    jr   _do_load_local
Op_LOAD_LOCAL:
    ld   a, (bc)
    inc  bc
_do_load_local:
    exx
    call _resolve_local_exx
    cp   1
    jr   z, .load_byte_exx
    srl  a                      ; A = word count (size >> 1)
    ld   b, a
.load_words_exx:
    ld   e, (hl)
    inc  hl
    ld   d, (hl)
    inc  hl
    dec  ix
    dec  ix
    ld   (ix+0), e
    ld   (ix+1), d
    djnz .load_words_exx
    exx
    jp   DispatchNext
.load_byte_exx:
    ld   a, (hl)
    dec  ix
    dec  ix
    ld   (ix+0), a
    ld   (ix+1), 0
    exx
    jp   DispatchNext

; Subroutine: pops top value from IX into local slot A (uses EXX to preserve BC)
_store_local_slot_a:
    exx
    call _resolve_local_exx
    cp   1
    jr   z, .store_byte_exx
    ld   c, a
    ld   b, 0
    add  hl, bc                 ; HL = addr + size
    srl  a                      ; A = word count (size >> 1)
    ld   b, a
.store_words_exx:
    ld   e, (ix+0)
    ld   d, (ix+1)
    inc  ix
    inc  ix
    dec  hl
    ld   (hl), d                ; high byte at addr + 2*k + 1
    dec  hl
    ld   (hl), e                ; low byte at addr + 2*k + 0
    djnz .store_words_exx
    exx
    ret
.store_byte_exx:
    ld   a, (ix+0)
    inc  ix
    inc  ix
    ld   (hl), a
    exx
    ret

Op_STORE_LOCAL_0:
    xor  a
    jr   _do_store_local
Op_STORE_LOCAL_1:
    ld   a, 1
    jr   _do_store_local
Op_STORE_LOCAL_2:
    ld   a, 2
    jr   _do_store_local
Op_STORE_LOCAL_3:
    ld   a, 3
    jr   _do_store_local
Op_STORE_LOCAL:
    ld   a, (bc)
    inc  bc
_do_store_local:
    call _store_local_slot_a
    jp   DispatchNext

Op_ADDR_OF_LOCAL:
    ld   a, (bc)
    inc  bc
    exx
    call _resolve_local_exx
    dec  ix
    dec  ix
    ld   (ix+0), l
    ld   (ix+1), h
    exx
    jp   DispatchNext

; ── Global Variable Access ────────────────────────────────────────────────────

; Reads 16-bit big-endian global index from (BC), advances BC, switches to EXX,
; and resolves HL = global address, A = low byte of global size
_resolve_global_exx:
    ld   a, (bc)
    inc  bc
    ld   d, a
    ld   a, (bc)
    inc  bc
    ld   e, a
    push de
    exx
    pop  bc                     ; BC = global index
    ld   de, (_globals_table_addr)
    ld   hl, (_globals_base_addr)
.res_glob_loop:
    ld   a, b
    or   c
    jr   z, .res_glob_done
    push bc
    ld   a, (de)                ; high byte of size (big-endian in .p4p)
    inc  de
    ld   b, a
    ld   a, (de)                ; low byte of size
    inc  de
    ld   c, a
    add  hl, bc
    pop  bc
    dec  bc
    jr   .res_glob_loop
.res_glob_done:
    inc  de                     ; skip high byte of target global's size
    ld   a, (de)                ; A = low byte of target global's size
    ret

Op_ADDR_OF_GLOBAL:
    call _resolve_global_exx
    dec  ix
    dec  ix
    ld   (ix+0), l
    ld   (ix+1), h
    exx
    jp   DispatchNext

Op_LOAD_GLOBAL:
    call _resolve_global_exx
    cp   1
    jmp  z, .load_byte_exx
    srl  a
    ld   b, a
    jmp  .load_words_exx

Op_STORE_GLOBAL:
    call _resolve_global_exx
    cp   1
    jr   z, .sglob_byte
    ld   c, a
    ld   b, 0
    add  hl, bc                 ; HL = addr + size
    srl  a
    ld   b, a
.sglob_words:
    ld   e, (ix+0)
    ld   d, (ix+1)
    inc  ix
    inc  ix
    dec  hl
    ld   (hl), d
    dec  hl
    ld   (hl), e
    djnz .sglob_words
    exx
    jp   DispatchNext
.sglob_byte:
    ld   a, (ix+0)
    inc  ix
    inc  ix
    ld   (hl), a
    exx
    jp   DispatchNext

; ── Memory PEEK and POKE (Native Little-Endian in RAM) ────────────────────────

Op_PEEK1:
    ld   l, (ix+0)
    ld   h, (ix+1)
    ld   a, (hl)
    ld   (ix+0), a
    ld   (ix+1), 0
    jp   DispatchNext

Op_POKE1:
    ld   a, (ix+0)
    ld   l, (ix+2)
    ld   h, (ix+3)
    ld   (hl), a
    ld   de, 4
    add  ix, de
    jp   DispatchNext

Op_PEEK2:
    ld   l, (ix+0)
    ld   h, (ix+1)
    ld   e, (hl)
    inc  hl
    ld   d, (hl)
    ld   (ix+0), e
    ld   (ix+1), d
    jp   DispatchNext

Op_POKE2:
    ld   e, (ix+0)
    ld   d, (ix+1)
    ld   l, (ix+2)
    ld   h, (ix+3)
    ld   (hl), e
    inc  hl
    ld   (hl), d
    ld   de, 4
    add  ix, de
    jp   DispatchNext

Op_SHL1_ADD:
    ld   l, (ix+0)
    ld   h, (ix+1)              ; HL = idx
    add  hl, hl                 ; HL = idx * 2
    ld   e, (ix+2)
    ld   d, (ix+3)              ; DE = base
    add  hl, de
    inc  ix
    inc  ix
    ld   (ix+0), l
    ld   (ix+1), h
    jp   DispatchNext

Op_MEM_COPY:
    exx
    ld   c, (ix+0)
    ld   b, (ix+1)              ; BC = count
    ld   e, (ix+2)
    ld   d, (ix+3)              ; DE = dst
    ld   l, (ix+4)
    ld   h, (ix+5)              ; HL = src
    ld   a, b
    or   c
    jr   z, .mcopy_done
    ldir
.mcopy_done:
    ld   de, 6
    add  ix, de
    exx
    jp   DispatchNext

Op_MEM_SET:
    exx
    ld   c, (ix+0)
    ld   b, (ix+1)              ; BC = count
    ld   e, (ix+2)              ; E = val byte
    ld   l, (ix+4)
    ld   h, (ix+5)              ; HL = dst
.mset_loop:
    ld   a, b
    or   c
    jr   z, .mset_done
    ld   (hl), e
    inc  hl
    dec  bc
    jr   .mset_loop
.mset_done:
    ld   de, 6
    add  ix, de
    exx
    jp   DispatchNext

; ── Arithmetic & Logic ────────────────────────────────────────────────────────

Op_ADD:
    ld   l, (ix+2)
    ld   h, (ix+3)
    ld   e, (ix+0)
    ld   d, (ix+1)
    add  hl, de
    inc  ix
    inc  ix
    ld   (ix+0), l
    ld   (ix+1), h
    jp   DispatchNext

Op_SUB:
    ld   l, (ix+2)
    ld   h, (ix+3)
    ld   e, (ix+0)
    ld   d, (ix+1)
    and  a
    sbc  hl, de
    inc  ix
    inc  ix
    ld   (ix+0), l
    ld   (ix+1), h
    jp   DispatchNext

Op_BIT_AND:
    ld   a, (ix+2)
    and  (ix+0)
    ld   (ix+2), a
    ld   a, (ix+3)
    and  (ix+1)
    ld   (ix+3), a
    inc  ix
    inc  ix
    jp   DispatchNext

Op_BIT_OR:
    ld   a, (ix+2)
    or   (ix+0)
    ld   (ix+2), a
    ld   a, (ix+3)
    or   (ix+1)
    ld   (ix+3), a
    inc  ix
    inc  ix
    jp   DispatchNext

Op_BIT_XOR:
    ld   a, (ix+2)
    xor  (ix+0)
    ld   (ix+2), a
    ld   a, (ix+3)
    xor  (ix+1)
    ld   (ix+3), a
    inc  ix
    inc  ix
    jp   DispatchNext

Op_BIT_NOT:
    ld   a, (ix+0)
    cpl
    ld   (ix+0), a
    ld   a, (ix+1)
    cpl
    ld   (ix+1), a
    jp   DispatchNext

Op_NEG:
    xor  a
    sub  (ix+0)
    ld   (ix+0), a
    sbc  a, a
    sub  (ix+1)
    ld   (ix+1), a
    jp   DispatchNext

Op_SHL:
    ld   a, (ix+0)              ; count
    ld   l, (ix+2)
    ld   h, (ix+3)              ; value
    inc  ix
    inc  ix
    or   a
    jr   z, .shl_done
.shl_loop:
    add  hl, hl
    dec  a
    jr   nz, .shl_loop
.shl_done:
    ld   (ix+0), l
    ld   (ix+1), h
    jp   DispatchNext

Op_SHR:
    ld   a, (ix+0)              ; count
    ld   l, (ix+2)
    ld   h, (ix+3)              ; value
    inc  ix
    inc  ix
    or   a
    jr   z, .shr_done
.shr_loop:
    srl  h
    rr   l
    dec  a
    jr   nz, .shr_loop
.shr_done:
    ld   (ix+0), l
    ld   (ix+1), h
    jp   DispatchNext

Op_MUL:
    exx
    ld   e, (ix+0)
    ld   d, (ix+1)              ; DE = multiplier
    ld   c, (ix+2)
    ld   b, (ix+3)              ; BC = multiplicand
    ld   hl, 0
    ld   a, 16
.mul_loop:
    add  hl, hl
    rl   e
    rl   d
    jr   nc, .mul_no_add
    add  hl, bc
.mul_no_add:
    dec  a
    jr   nz, .mul_loop
    inc  ix
    inc  ix
    ld   (ix+0), l
    ld   (ix+1), h
    exx
    jp   DispatchNext

_divmod16:
    ld   e, (ix+0)
    ld   d, (ix+1)              ; DE = divisor
    ld   c, (ix+2)
    ld   b, (ix+3)              ; BC = dividend
    ld   hl, 0                  ; HL = remainder
    ld   a, 16
.div_loop:
    sla  c
    rl   b
    adc  hl, hl
    or   a
    sbc  hl, de
    jr   nc, .div_bit1
    add  hl, de
    jr   .div_next
.div_bit1:
    inc  c
.div_next:
    dec  a
    jr   nz, .div_loop
    ret

Op_DIV:
    exx
    call _divmod16
    inc  ix
    inc  ix
    ld   (ix+0), c
    ld   (ix+1), b
    exx
    jp   DispatchNext

Op_MOD:
    exx
    call _divmod16
    inc  ix
    inc  ix
    ld   (ix+0), l
    ld   (ix+1), h
    exx
    jp   DispatchNext

; ── Comparisons ───────────────────────────────────────────────────────────────

Op_CMP_EQ:
    ld   l, (ix+2)
    ld   h, (ix+3)
    ld   e, (ix+0)
    ld   d, (ix+1)
    and  a
    sbc  hl, de
    jr   z, _cmp_true
    jr   _cmp_false

Op_CMP_NE:
    ld   l, (ix+2)
    ld   h, (ix+3)
    ld   e, (ix+0)
    ld   d, (ix+1)
    and  a
    sbc  hl, de
    jr   nz, _cmp_true
    jr   _cmp_false

Op_CMP_LT:
    ld   l, (ix+2)
    ld   h, (ix+3)
    ld   e, (ix+0)
    ld   d, (ix+1)
    and  a
    sbc  hl, de
    jr   c, _cmp_true
    jr   _cmp_false

Op_CMP_GE:
    ld   l, (ix+2)
    ld   h, (ix+3)
    ld   e, (ix+0)
    ld   d, (ix+1)
    and  a
    sbc  hl, de
    jr   nc, _cmp_true
    jr   _cmp_false

Op_CMP_GT:
    ld   l, (ix+0)
    ld   h, (ix+1)
    ld   e, (ix+2)
    ld   d, (ix+3)
    and  a
    sbc  hl, de
    jr   c, _cmp_true
    jr   _cmp_false

Op_CMP_LE:
    ld   l, (ix+0)
    ld   h, (ix+1)
    ld   e, (ix+2)
    ld   d, (ix+3)
    and  a
    sbc  hl, de
    jr   nc, _cmp_true
    jr   _cmp_false

_cmp_true:
    inc  ix
    inc  ix
    ld   (ix+0), 1
    ld   (ix+1), 0
    jp   DispatchNext

_cmp_false:
    inc  ix
    inc  ix
    ld   (ix+0), 0
    ld   (ix+1), 0
    jp   DispatchNext

Op_NOT:
    ld   a, (ix+0)
    or   (ix+1)
    ld   a, 0
    jr   nz, .not_store
    inc  a
.not_store:
    ld   (ix+0), a
    ld   (ix+1), 0
    jp   DispatchNext

; ── Control Flow ──────────────────────────────────────────────────────────────

Op_JUMP:
    ld   a, (bc)
    inc  bc
    ld   h, a
    ld   a, (bc)
    inc  bc
    ld   l, a
    add  hl, bc
    ld   b, h
    ld   c, l
    jp   DispatchNext

Op_JUMP_IF_TRUE:
    ld   a, (ix+0)
    or   (ix+1)
    inc  ix
    inc  ix
    jr   nz, Op_JUMP
    inc  bc
    inc  bc
    jp   DispatchNext

Op_JUMP_IF_FALSE:
    ld   a, (ix+0)
    or   (ix+1)
    inc  ix
    inc  ix
    jr   z, Op_JUMP
    inc  bc
    inc  bc
    jp   DispatchNext

Op_CALL:
    ld   a, (bc)
    inc  bc
    ld   d, a
    ld   a, (bc)
    inc  bc
    ld   e, a                   ; DE = funcIdx, BC = ReturnPC

    ; Push caller context onto Z80 hardware stack SP:
    push bc                     ; 1. ReturnPC
    push iy                     ; 2. CallerFP
    ld   hl, (_var_table_ptr)
    push hl                     ; 3. CallerVarTablePtr

    ; Resolve FuncTable + funcIdx * 10:
    ld   h, d
    ld   l, e
    add  hl, hl                 ; * 2
    ld   d, h
    ld   e, l
    add  hl, hl                 ; * 4
    add  hl, hl                 ; * 8
    add  hl, de                 ; * 10
    ld   de, (_func_table_addr)
    add  hl, de                 ; HL = &FuncTable[funcIdx]

    ld   a, (hl)                ; [0] = argCount
    ld   (_call_arg_count), a
    inc  hl
    inc  hl                     ; skip [1] = localCount

    ; [2..3] = frameSize (big-endian)
    ld   d, (hl)
    inc  hl
    ld   e, (hl)
    inc  hl
    push hl                     ; save &FuncTable[funcIdx]+4
    push iy
    pop  hl
    and  a
    sbc  hl, de                 ; CalleeFP = CallerFP - frameSize
    push hl
    pop  iy
    pop  hl                     ; restore &FuncTable[funcIdx]+4

    ; [4..5] = codeOffset (big-endian)
    ld   d, (hl)
    inc  hl
    ld   e, (hl)
    inc  hl
    inc  hl                     ; skip [6..7] = codeSize
    inc  hl
    push hl                     ; save &FuncTable[funcIdx]+8
    ld   hl, (_code_base_addr)
    add  hl, de
    ld   b, h
    ld   c, l                   ; BC = new P4PC
    pop  hl

    ; [8..9] = varTableOff (big-endian)
    ld   d, (hl)
    inc  hl
    ld   e, (hl)
    ld   hl, (_local_vars_addr)
    add  hl, de
    ld   (_var_table_ptr), hl

    ; Pop arguments into parameter slots (argCount-1 down to 0):
.call_pop_args:
    ld   a, (_call_arg_count)
    or   a
    jp   z, DispatchNext
    dec  a
    ld   (_call_arg_count), a
    call _store_local_slot_a
    jr   .call_pop_args

Op_RET:
Op_RET_VOID:
Op_RET_SLICE:
    pop  hl
    ld   (_var_table_ptr), hl
    pop  iy
    pop  bc
    jp   DispatchNext

Op_HALT:
    xor  a
    ld   b, a
    out  (0x60), a
    defb 0x06                   ; F$Exit(0)
.halt_loop:
    jr   .halt_loop

Op_PANIC:
    ld   hl, _panic_msg
    ld   bc, 6
    ld   a, 2                   ; STDERR
    out  (0x60), a
    defb 0x8A                   ; I$Write
    xor  a
    ld   b, 1
    out  (0x60), a
    defb 0x06                   ; F$Exit(1)
.panic_loop:
    jr   .panic_loop

_panic_msg:
    defb "PANIC\n"

; ── Slices & Strings ──────────────────────────────────────────────────────────

Op_SLICE_NEW:
    jp   DispatchNext

Op_SLICE_LEN:
    ld   e, (ix+0)
    ld   d, (ix+1)
    ld   (ix+4), e
    ld   (ix+5), d
    ld   de, 4
    add  ix, de
    jp   DispatchNext

Op_SLICE_CAP:
    ld   e, (ix+2)
    ld   d, (ix+3)
    ld   (ix+4), e
    ld   (ix+5), d
    ld   de, 4
    add  ix, de
    jp   DispatchNext

Op_SLICE_SUB:
    ; [ptr, cap, len, start, end] -> [ptr+start, cap-start, end-start]
    ld   e, (ix+2)
    ld   d, (ix+3)              ; DE = start
    ; new_len = end - start -> store at ix+4..5:
    ld   l, (ix+0)
    ld   h, (ix+1)              ; HL = end
    and  a
    sbc  hl, de
    ld   (ix+4), l
    ld   (ix+5), h
    ; new_cap = old_cap - start -> store at ix+6..7:
    ld   l, (ix+6)
    ld   h, (ix+7)
    and  a
    sbc  hl, de
    ld   (ix+6), l
    ld   (ix+7), h
    ; new_ptr = old_ptr + start -> store at ix+8..9:
    ld   l, (ix+8)
    ld   h, (ix+9)
    add  hl, de
    ld   (ix+8), l
    ld   (ix+9), h
    ld   de, 4
    add  ix, de
    jp   DispatchNext

Op_STR_CMP:
    ; Stack: [aPtr(ix+10..11), aCap(ix+8..9), aLen(ix+6..7), bPtr(ix+4..5), bCap(ix+2..3), bLen(ix+0..1)]
    exx
    ld   e, (ix+10)
    ld   d, (ix+11)             ; DE = aPtr
    ld   l, (ix+4)
    ld   h, (ix+5)              ; HL = bPtr
    ld   c, (ix+6)
    ld   b, (ix+7)              ; BC = aLen
.strcmp_loop:
    ld   a, b
    or   c
    jr   nz, .strcmp_a_nonempty
    ; aLen == 0: check bLen at ix+0..1
    ld   a, (ix+0)
    or   (ix+1)
    jr   nz, .strcmp_lt
    ; Both empty -> equal (0)
    ld   de, 0
    jr   .strcmp_done
.strcmp_a_nonempty:
    ld   a, (ix+0)
    or   (ix+1)
    jr   z, .strcmp_gt
    ; Compare *aPtr (DE) vs *bPtr (HL):
    ld   a, (de)
    cp   (hl)
    jr   nz, .strcmp_diff
    inc  de
    inc  hl
    dec  bc                     ; aLen--
    ; Decrement 16-bit bLen at ix+0..1:
    ld   a, (ix+0)
    or   a
    jr   nz, .strcmp_dec_lo
    dec  (ix+1)
.strcmp_dec_lo:
    dec  (ix+0)
    jr   .strcmp_loop
.strcmp_diff:
    jr   c, .strcmp_lt
.strcmp_gt:
    ld   de, 1
    jr   .strcmp_done
.strcmp_lt:
    ld   de, 0xFFFF
.strcmp_done:
    ld   hl, 10
    ex   de, hl                 ; HL = result, DE = 10
    add  ix, de
    ld   (ix+0), l
    ld   (ix+1), h
    exx
    jp   DispatchNext

; ── Hatvan OS Syscall Primitives ──────────────────────────────────────────────

Op_HATVAN_TRAP:
    ld   a, (bc)
    inc  bc
    ld   (_trap_opcode_byte), a

    ; Save VM registers on Z80 hardware stack SP (immune to child processes):
    push bc                     ; save P4PC
    push iy                     ; save P4FP

    ; Read 5 syscall arguments from IX:
    ;   (ix+0..1) = regU
    ;   (ix+2..3) = regY
    ;   (ix+4..5) = regX
    ;   (ix+6..7) = regB
    ;   (ix+8..9) = regA
    ld   e, (ix+0)
    ld   d, (ix+1)              ; DE = regU
    ld   c, (ix+2)
    ld   b, (ix+3)              ; BC = regY
    ld   a, (ix+6)              ; A = low(regB)
    or   a
    jr   z, .trap_keep_b
    ld   b, a                   ; B = low(regB) when non-zero
.trap_keep_b:
    ld   l, (ix+4)
    ld   h, (ix+5)              ; HL = regX
    ld   a, (ix+8)              ; A = low(regA)

    ; Advance IX by +2 (pop 5 words, push 4 return words -> net +2 bytes) and save on SP:
    inc  ix
    inc  ix
    push ix                     ; save new P4SP on Z80 stack

    ; Set IX = regU (DE) for the Hatvan syscall:
    push de
    pop  ix

    ; Invoke NMI syscall trampoline:
    call _trap_trampoline

    ; Preserve syscall return registers (A, B, C, Carry) across stack pops:
    ld   d, a                   ; D = returned A
    ld   e, 0                   ; E = err (0 if Carry=0, 1 if Carry=1)
    jr   nc, .trap_ok
    ld   e, 1
    ld   d, b                   ; On error, sys.golf reads res_A as OS-9 error code (UserFrame.B)!
.trap_ok:
    ld   h, b
    ld   l, c                   ; HL = returned BC (UserFrame.Y, H = returned B)

    pop  ix                     ; restore P4SP
    pop  iy                     ; restore P4FP
    pop  bc                     ; restore P4PC

    ; Store [res_A, res_B, res_Y, err] onto IX:
    ;   (ix+6..7) = res_A
    ;   (ix+4..5) = res_B
    ;   (ix+2..3) = res_Y
    ;   (ix+0..1) = err
    ld   (ix+6), d
    ld   (ix+7), 0
    ld   (ix+4), h
    ld   (ix+5), 0
    ld   (ix+2), l
    ld   (ix+3), h
    ld   (ix+0), e
    ld   (ix+1), 0
    jp   DispatchNext

_trap_trampoline:
    out  (0x60), a
_trap_opcode_byte:
    defb 0
    ret

Op_SYS_EXIT:
    ld   b, (ix+0)
    inc  ix
    inc  ix
    xor  a
    out  (0x60), a
    defb 0x06                   ; F$Exit
.exit_loop:
    jr   .exit_loop

Op_IO_PRINT:
    push bc
    push iy
    ld   c, (ix+0)
    ld   b, (ix+1)              ; BC = len
    ld   l, (ix+4)
    ld   h, (ix+5)              ; HL = ptr
    ld   de, 6
    add  ix, de
    push ix
    ld   a, 1                   ; STDOUT
    out  (0x60), a
    defb 0x8C                   ; I$WritLn
    pop  ix
    pop  iy
    pop  bc
    jp   DispatchNext

; ── Bytecode Parser & Task Startup (_user_start) ──────────────────────────────

; Helper: reads 16-bit big-endian word from (HL), advances HL by 2, returns in DE
_read_be16:
    ld   d, (hl)
    inc  hl
    ld   e, (hl)
    inc  hl
    ret

_user_start:
    ; Verify magic "P4P\x01" at $2000:
    ld   hl, 0x2000
    ld   a, (hl)
    cp   0x50
    jp   nz, _halt_bad_magic
    inc  hl
    ld   a, (hl)
    cp   0x34
    jp   nz, _halt_bad_magic
    inc  hl
    ld   a, (hl)
    cp   0x50
    jp   nz, _halt_bad_magic
    inc  hl
    ld   a, (hl)
    cp   0x01
    jp   nz, _halt_bad_magic
    inc  hl

    ; Skip flags (offset 4, 5):
    inc  hl
    inc  hl

    ; Read stringPoolSize (offset 6, 7):
    call _read_be16
    ld   (_str_pool_size), de

    ; Read globalsCount (offset 8, 9):
    call _read_be16
    ld   (_globals_count), de

    ; Read funcCount (offset 10, 11):
    call _read_be16
    ld   (_func_count), de

    ; Read entryFuncIdx (offset 12, 13):
    call _read_be16
    ld   (_entry_func_idx), de

    ; Read codeSize (offset 14, 15):
    call _read_be16
    ld   (_code_size), de

    ; HL is now at GlobalsTable ($2010):
    ld   (_globals_table_addr), hl

    ; Sum sizes of all globals into _total_glob_bytes:
    ld   bc, (_globals_count)
    ld   ix, 0                  ; IX accumulates total_glob_bytes
.sum_glob_loop:
    ld   a, b
    or   c
    jr   z, .sum_glob_done
    call _read_be16             ; DE = global size
    add  ix, de
    dec  bc
    jr   .sum_glob_loop
.sum_glob_done:
    ld   (_total_glob_bytes), ix

    ; HL is now at StringPoolBase:
    ld   (_str_pool_base_addr), hl

    ; FuncTable = StringPoolBase + _str_pool_size:
    ld   de, (_str_pool_size)
    add  hl, de
    ld   (_func_table_addr), hl

    ; LocalVarsTable = FuncTable + _func_count * 10:
    push hl
    ld   hl, (_func_count)
    add  hl, hl                 ; * 2
    ld   d, h
    ld   e, l
    add  hl, hl                 ; * 4
    add  hl, hl                 ; * 8
    add  hl, de                 ; * 10
    ex   de, hl                 ; DE = _func_count * 10
    pop  hl
    add  hl, de
    ld   (_local_vars_addr), hl

    ; Sum (argCount + localCount) across all functions in FuncTable:
    ld   hl, (_func_table_addr)
    ld   bc, (_func_count)
    ld   ix, 0                  ; IX accumulates localVarsSize
.sum_lvars_loop:
    ld   a, b
    or   c
    jr   z, .sum_lvars_done
    ld   e, (hl)                ; argCount
    ld   d, 0
    add  ix, de
    inc  hl
    ld   e, (hl)                ; localCount
    add  ix, de
    ld   de, 9
    add  hl, de                 ; advance to next 10-byte FuncTable entry
    dec  bc
    jr   .sum_lvars_loop
.sum_lvars_done:
    ld   (_local_vars_size), ix

    ; CodeBase = LocalVarsTable + _local_vars_size:
    ld   hl, (_local_vars_addr)
    push ix
    pop  de
    add  hl, de
    ld   (_code_base_addr), hl

    ; GlobalsBase = CodeBase + _code_size:
    ld   de, (_code_size)
    add  hl, de
    ld   (_globals_base_addr), hl

    ; Zero Globals data (_total_glob_bytes at GlobalsBase):
    ld   bc, (_total_glob_bytes)
.zero_glob_loop:
    ld   a, b
    or   c
    jr   z, .zero_glob_done
    ld   (hl), 0
    inc  hl
    dec  bc
    jr   .zero_glob_loop
.zero_glob_done:

    ; Parse Entry Function: FuncTable + _entry_func_idx * 10:
    ld   hl, (_entry_func_idx)
    add  hl, hl                 ; * 2
    ld   d, h
    ld   e, l
    add  hl, hl                 ; * 4
    add  hl, hl                 ; * 8
    add  hl, de                 ; * 10
    ld   de, (_func_table_addr)
    add  hl, de                 ; HL = &FuncTable[entryFuncIdx]
    inc  hl                     ; skip argCount
    inc  hl                     ; skip localCount

    ; Read frameSize (big-endian):
    call _read_be16             ; DE = frameSize
    push hl
    ld   hl, 0xF000
    and  a
    sbc  hl, de
    push hl
    pop  iy                     ; IY (P4FP) = $F000 - frameSize
    pop  hl

    ; Read codeOffset (big-endian):
    call _read_be16             ; DE = codeOffset
    inc  hl                     ; skip codeSize hi
    inc  hl                     ; skip codeSize lo
    push hl
    ld   hl, (_code_base_addr)
    add  hl, de
    ld   b, h
    ld   c, l                   ; BC (P4PC) = CodeBase + codeOffset
    pop  hl

    ; Read varTableOff (big-endian):
    call _read_be16             ; DE = varTableOff
    ld   hl, (_local_vars_addr)
    add  hl, de
    ld   (_var_table_ptr), hl

    ; Initialize Par4 Operand Stack IX = $D000:
    ld   ix, 0xD000

    ; Push synthetic caller frame onto Z80 SP so RET from main triggers Op_HALT:
    ld   hl, _halt_stub
    push hl                     ; ReturnPC
    ld   hl, 0xF000
    push hl                     ; CallerFP
    ld   hl, 0
    push hl                     ; CallerVarTablePtr

    jp   DispatchNext

_halt_stub:
    defb 0x67                   ; Op_HALT

_halt_bad_magic:
    ld   b, 1
    xor  a
    out  (0x60), a
    defb 0x06                   ; F$Exit(1)
.bad_loop:
    jr   .bad_loop

; ── VM State Variables ────────────────────────────────────────────────────────
_str_pool_size:     defw 0
_globals_count:     defw 0
_func_count:        defw 0
_entry_func_idx:    defw 0
_code_size:         defw 0
_total_glob_bytes:  defw 0
_local_vars_size:   defw 0
_globals_table_addr: defw 0
_str_pool_base_addr: defw 0
_func_table_addr:   defw 0
_local_vars_addr:   defw 0
_code_base_addr:    defw 0
_globals_base_addr: defw 0
_var_table_ptr:     defw 0
_call_arg_count:    defb 0

; ── 256-Entry Opcode Jump Table ───────────────────────────────────────────────
OpcodeTable:
    ; Group 1: Stack & Literals (0x00..0x1F)
    defw Op_NOP, Op_PUSH_NIL, Op_PUSH_NIL_SLICE, Op_PUSH_TRUE
    defw Op_PUSH_FALSE, Op_PUSH_0, Op_PUSH_1, Op_PUSH_NEG1
    defw Op_PUSH_I8, Op_PUSH_U8, Op_PUSH_I16, Op_PUSH_STR
    defw Op_POP, Op_POP_SLICE, Op_DUP, Op_SWAP
    defw Op_DUP2, Op_ROT, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP

    ; Group 2: Variables & Memory (0x20..0x3F)
    defw Op_LOAD_LOCAL_0, Op_LOAD_LOCAL_1, Op_LOAD_LOCAL_2, Op_LOAD_LOCAL_3
    defw Op_STORE_LOCAL_0, Op_STORE_LOCAL_1, Op_STORE_LOCAL_2, Op_STORE_LOCAL_3
    defw Op_LOAD_LOCAL, Op_STORE_LOCAL, Op_LOAD_GLOBAL, Op_STORE_GLOBAL
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_ADDR_OF_GLOBAL, Op_PEEK2, Op_POKE2, Op_PEEK1
    defw Op_POKE1, Op_ADDR_OF_LOCAL, Op_SHL1_ADD, Op_MEM_COPY
    defw Op_MEM_SET, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP

    ; Group 3: Arithmetic & Logic (0x40..0x5F)
    defw Op_ADD, Op_SUB, Op_MUL, Op_DIV
    defw Op_MOD, Op_NEG, Op_BIT_AND, Op_BIT_OR
    defw Op_BIT_XOR, Op_BIT_NOT, Op_SHL, Op_SHR
    defw Op_CMP_EQ, Op_CMP_NE, Op_CMP_LT, Op_CMP_LE
    defw Op_CMP_GT, Op_CMP_GE, Op_NOT, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP

    ; Group 4: Control Flow (0x60..0x6F)
    defw Op_JUMP, Op_JUMP_IF_TRUE, Op_JUMP_IF_FALSE, Op_CALL
    defw Op_RET, Op_RET_SLICE, Op_RET_VOID, Op_HALT
    defw Op_PANIC, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP

    ; Group 5: Slices & Strings (0x70..0x8F)
    defw Op_SLICE_NEW, Op_SLICE_LEN, Op_SLICE_CAP, Op_SLICE_SUB
    defw Op_NOP, Op_NOP, Op_NOP, Op_STR_CMP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP

    ; Groups 6 & 7: (0x90..0xBF)
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP

    ; Group 8: Hatvan OS & Hardware Primitives (0xC0..0xCF)
    defw Op_HATVAN_TRAP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_SYS_EXIT, Op_IO_PRINT, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP

    ; Remaining opcodes (0xD0..0xFF)
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP
    defw Op_NOP, Op_NOP, Op_NOP, Op_NOP

    end start
