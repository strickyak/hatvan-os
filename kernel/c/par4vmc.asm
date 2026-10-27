; ==============================================================================
; Hatvan OS/C: RCA COSMAC CDP1802 P-Code Interpreter Engine (par4vmc.asm)
; Common Low-Memory Runtime Template ($0000..$0FFF)
; ==============================================================================
;
; Register Allocation Matrix:
; R0:  Hardware DMA / Boot scratch
; R1:  Hardware Interrupt PC (always points to $0008)
; R2:  Native Call Stack SP (grows downward from $DFFE)
; R3:  R_DISPATCH: P-code dispatch engine PC (P=3)
; R4:  R_EXEC: Opcode handler execution PC (P=4)
; R5:  R_P4PC: Par4 Bytecode Program Counter
; R6:  R_P4SP: Par4 16-bit Operand Stack Pointer (grows down from $B000)
; R7:  R_P4FP: Par4 Frame Pointer / Syscall Register X
; R8:  R_STRPOOL: Task String Pool Base Pointer / Syscall Register Y
; R9:  R_GLOBALS: Task Globals Base Pointer / Syscall Register U
; RA:  R_T0: VM 16-bit Temp 0 / Syscall Register B
; RB:  R_T1: VM 16-bit Temp 1 / Scratch
; RC:  R_TBL_LO: Points to OpcodeTableLo ($0100)
; RD:  R_TBL_HI: Points to OpcodeTableHi ($0200)
; RE:  R_TRAP: Syscall Trap Pointer (always points to $0004)
; RF:  R_SCRATCH: System scratch / memory pointer / untrap fuse
; D:   8-bit Accumulator / Syscall Register A / Return Value
; DF:  Carry Flag / Syscall Error Status (0=OK, 1=Error)
; ==============================================================================

    ORG     $0000

; ── Reset Vector / Task 0 Entry ($0000..$0003) ────────────────────────────────
_reset_vector:
    ; Hardware reset starts with P=0, X=0, R0=$0000.
    ; LBR takes 3 bytes ($0000..$0002), followed by NOP ($0003):
    LBR     _cstart_vmc
    NOP

; ── Syscall Trap Trampoline ($0004..$0005) ────────────────────────────────────
_trap_trampoline:
    SEX     R2                  ; X = 2 (points to M(R2) holding call_num)
    OUT     6                   ; MMU captures syscall ID from BUS, flips CurrentTask = 0!

; ── Syscall Kernel Entry ($0006..$0007) ───────────────────────────────────────
_syscall_kernel_entry:
    ; Next machine cycle after OUT 6 executes at address $0006 in Task 0:
    BR      _syscall_stub       ; Short branch (2 bytes: $0006, $0007) to page 0 stub!

; ── Hardware Interrupt Vector ($0008..$000F) ──────────────────────────────────
_timer_irq_vector:
    ; Hardware /INT forces P=1, X=2, R1=$0008.
    ; gepc MMU forces CurrentTask = 0.
    LBR     _TimerIRQEntry      ; Long branch (3 bytes: $0008..$000A)
    NOP
    NOP
    NOP
    NOP
    NOP                         ; Padding to $0010

; ── Syscall Stub on Page 0 ($0010..$0013) ────────────────────────────────────
_syscall_stub:
    LBR     _SyscallTrapEntry   ; Long branch to main syscall dispatcher
    NOP

; ── Patchable Function Vectors ($0014..$001F) ─────────────────────────────────
_fn_dispatch_syscall:   DW 0    ; Function index of syscall_Dispatch in kernel ($0014)
_fn_schedule_next:      DW 0    ; Function index of proc_ScheduleNext in kernel ($0016)
                        DW 0    ; Reserved ($0018)
                        DW 0    ; Reserved ($001A)
                        DW 0    ; Reserved ($001C)
                        DW 0    ; Reserved ($001E)

; ── User Task Entry Trampoline ($0020..$002F) ─────────────────────────────────
    ORG     $0020
_user_entry:
    LBR     _user_start
    NOP

; ── Synthetic Untrap Bytecode Stub ($0030..$003F) ──────────────────────────────
    ORG     $0030
_un_trap_stub:
    DB      $C1                 ; Op_HATVAN_UNTRAP opcode
    DB      $C1

; ── Low-Memory Supervisor Variables ($0040..$007F) ───────────────────────────
    ORG     $0040
_trap_call_num:     DB 0        ; $0040
_trap_user_a:       DB 0        ; $0041
_trap_user_b:       DB 0        ; $0042
_trap_user_pid:     DB 0        ; $0043
_trap_user_x:       DW 0        ; $0044..$0045
_trap_user_y:       DW 0        ; $0046..$0047
_trap_user_u:       DW 0        ; $0048..$0049
_trap_user_pc:      DW 0        ; $004A..$004B
_trap_ret_a:        DB 0        ; $004C
_trap_ret_b:        DB 0        ; $004D
_trap_ret_err:      DB 0        ; $004E
_trap_unused:       DB 0        ; $004F
_trap_ret_y:        DW 0        ; $0050..$0051

_in_kernel:         DB 1        ; $0052
_curr_task:         DB 0        ; $0053
_str_pool_size:     DW 0        ; $0054..$0055
_globals_count:     DW 0        ; $0056..$0057
_func_count:        DW 0        ; $0058..$0059
_entry_func_idx:    DW 0        ; $005A..$005B
_code_size:         DW 0        ; $005C..$005D
_total_glob_bytes:  DW 0        ; $005E..$005F
_local_vars_size:   DW 0        ; $0060..$0061
_func_table_addr:   DW 0        ; $0062..$0063
_local_vars_addr:   DW 0        ; $0064..$0065
_code_base_addr:    DW 0        ; $0066..$0067
_globals_base_addr: DW 0        ; $0068..$0069
_var_table_ptr:     DW 0        ; $006A..$006B
_task_args_ptr:     DW 0        ; $006C..$006D
_res_launch_r2:     DW 0        ; $006E..$006F
_res_launch_r5:     DW 0        ; $0070..$0071
_res_launch_r6:     DW 0        ; $0072..$0073
_res_launch_r7:     DW 0        ; $0074..$0075
_globals_table_addr: DW 0       ; $0076..$0077
_str_pool_base_addr: DW 0       ; $0078..$0079
_call_saved_sp:      DW 0       ; $007A..$007B
_scratch_word:       DW 0       ; $007C..$007D
_scratch_byte:       DB 0       ; $007E

; ── Task Context Tables ($0080..$00FF) ────────────────────────────────────────
    ORG     $0080
task_r2_table:      DS 32       ; 16 tasks * 2 bytes ($0080..$009F)
    ORG     $00A0
task_r5_table:      DS 32       ; 16 tasks * 2 bytes ($00A0..$00BF)
    ORG     $00C0
task_r6_table:      DS 32       ; 16 tasks * 2 bytes ($00C0..$00DF)
    ORG     $00E0
task_r7_table:      DS 32       ; 16 tasks * 2 bytes ($00E0..$00FF)


; ── Split Jump Tables ($0100..$02FF) ──────────────────────────────────────────
    ORG     $0100
OpcodeTableLo:
    ; Group 1: Stack & Literals (0x00..0x1F)
    DB LOW(Op_NOP), LOW(Op_PUSH_NIL), LOW(Op_PUSH_NIL_SLICE), LOW(Op_PUSH_TRUE)
    DB LOW(Op_PUSH_FALSE), LOW(Op_PUSH_0), LOW(Op_PUSH_1), LOW(Op_PUSH_NEG1)
    DB LOW(Op_PUSH_I8), LOW(Op_PUSH_U8), LOW(Op_PUSH_I16), LOW(Op_PUSH_STR)
    DB LOW(Op_POP), LOW(Op_POP_SLICE), LOW(Op_DUP), LOW(Op_SWAP)
    DB LOW(Op_DUP2), LOW(Op_ROT), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)

    ; Group 2: Variables & Memory (0x20..0x3F)
    DB LOW(Op_LOAD_LOCAL_0), LOW(Op_LOAD_LOCAL_1), LOW(Op_LOAD_LOCAL_2), LOW(Op_LOAD_LOCAL_3)
    DB LOW(Op_STORE_LOCAL_0), LOW(Op_STORE_LOCAL_1), LOW(Op_STORE_LOCAL_2), LOW(Op_STORE_LOCAL_3)
    DB LOW(Op_LOAD_LOCAL), LOW(Op_STORE_LOCAL), LOW(Op_LOAD_GLOBAL), LOW(Op_STORE_GLOBAL)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_ADDR_OF_GLOBAL), LOW(Op_PEEK2), LOW(Op_POKE2), LOW(Op_PEEK1)
    DB LOW(Op_POKE1), LOW(Op_ADDR_OF_LOCAL), LOW(Op_SHL1_ADD), LOW(Op_MEM_COPY)
    DB LOW(Op_MEM_SET), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)

    ; Group 3: Arithmetic & Logic (0x40..0x5F)
    DB LOW(Op_ADD), LOW(Op_SUB), LOW(Op_MUL), LOW(Op_DIV)
    DB LOW(Op_MOD), LOW(Op_NEG), LOW(Op_BIT_AND), LOW(Op_BIT_OR)
    DB LOW(Op_BIT_XOR), LOW(Op_BIT_NOT), LOW(Op_SHL), LOW(Op_SHR)
    DB LOW(Op_CMP_EQ), LOW(Op_CMP_NE), LOW(Op_CMP_LT), LOW(Op_CMP_LE)
    DB LOW(Op_CMP_GT), LOW(Op_CMP_GE), LOW(Op_NOT), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)

    ; Group 4: Control Flow (0x60..0x6F)
    DB LOW(Op_JUMP), LOW(Op_JUMP_IF_TRUE), LOW(Op_JUMP_IF_FALSE), LOW(Op_CALL)
    DB LOW(Op_RET), LOW(Op_RET_SLICE), LOW(Op_RET_VOID), LOW(Op_HALT)
    DB LOW(Op_PANIC), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)

    ; Group 5: Slices & Strings (0x70..0x8F)
    DB LOW(Op_SLICE_NEW), LOW(Op_SLICE_LEN), LOW(Op_SLICE_CAP), LOW(Op_SLICE_SUB)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_STR_CMP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)

    ; Groups 6 & 7: (0x90..0xBF)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)

    ; Group 8: Hatvan OS & Hardware Primitives (0xC0..0xCF)
    DB LOW(Op_HATVAN_TRAP), LOW(Op_HATVAN_UNTRAP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_SYS_EXIT), LOW(Op_IO_PRINT), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)

    ; Remaining opcodes (0xD0..0xFF)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)
    DB LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP), LOW(Op_NOP)

    ORG     $0200
OpcodeTableHi:
    ; Group 1: Stack & Literals (0x00..0x1F)
    DB HIGH(Op_NOP), HIGH(Op_PUSH_NIL), HIGH(Op_PUSH_NIL_SLICE), HIGH(Op_PUSH_TRUE)
    DB HIGH(Op_PUSH_FALSE), HIGH(Op_PUSH_0), HIGH(Op_PUSH_1), HIGH(Op_PUSH_NEG1)
    DB HIGH(Op_PUSH_I8), HIGH(Op_PUSH_U8), HIGH(Op_PUSH_I16), HIGH(Op_PUSH_STR)
    DB HIGH(Op_POP), HIGH(Op_POP_SLICE), HIGH(Op_DUP), HIGH(Op_SWAP)
    DB HIGH(Op_DUP2), HIGH(Op_ROT), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)

    ; Group 2: Variables & Memory (0x20..0x3F)
    DB HIGH(Op_LOAD_LOCAL_0), HIGH(Op_LOAD_LOCAL_1), HIGH(Op_LOAD_LOCAL_2), HIGH(Op_LOAD_LOCAL_3)
    DB HIGH(Op_STORE_LOCAL_0), HIGH(Op_STORE_LOCAL_1), HIGH(Op_STORE_LOCAL_2), HIGH(Op_STORE_LOCAL_3)
    DB HIGH(Op_LOAD_LOCAL), HIGH(Op_STORE_LOCAL), HIGH(Op_LOAD_GLOBAL), HIGH(Op_STORE_GLOBAL)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_ADDR_OF_GLOBAL), HIGH(Op_PEEK2), HIGH(Op_POKE2), HIGH(Op_PEEK1)
    DB HIGH(Op_POKE1), HIGH(Op_ADDR_OF_LOCAL), HIGH(Op_SHL1_ADD), HIGH(Op_MEM_COPY)
    DB HIGH(Op_MEM_SET), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)

    ; Group 3: Arithmetic & Logic (0x40..0x5F)
    DB HIGH(Op_ADD), HIGH(Op_SUB), HIGH(Op_MUL), HIGH(Op_DIV)
    DB HIGH(Op_MOD), HIGH(Op_NEG), HIGH(Op_BIT_AND), HIGH(Op_BIT_OR)
    DB HIGH(Op_BIT_XOR), HIGH(Op_BIT_NOT), HIGH(Op_SHL), HIGH(Op_SHR)
    DB HIGH(Op_CMP_EQ), HIGH(Op_CMP_NE), HIGH(Op_CMP_LT), HIGH(Op_CMP_LE)
    DB HIGH(Op_CMP_GT), HIGH(Op_CMP_GE), HIGH(Op_NOT), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)

    ; Group 4: Control Flow (0x60..0x6F)
    DB HIGH(Op_JUMP), HIGH(Op_JUMP_IF_TRUE), HIGH(Op_JUMP_IF_FALSE), HIGH(Op_CALL)
    DB HIGH(Op_RET), HIGH(Op_RET_SLICE), HIGH(Op_RET_VOID), HIGH(Op_HALT)
    DB HIGH(Op_PANIC), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)

    ; Group 5: Slices & Strings (0x70..0x8F)
    DB HIGH(Op_SLICE_NEW), HIGH(Op_SLICE_LEN), HIGH(Op_SLICE_CAP), HIGH(Op_SLICE_SUB)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_STR_CMP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)

    ; Groups 6 & 7: (0x90..0xBF)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)

    ; Group 8: Hatvan OS & Hardware Primitives (0xC0..0xCF)
    DB HIGH(Op_HATVAN_TRAP), HIGH(Op_HATVAN_UNTRAP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_SYS_EXIT), HIGH(Op_IO_PRINT), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)

    ; Remaining opcodes (0xD0..0xFF)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)
    DB HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP), HIGH(Op_NOP)

; ── Core Dispatch Loop ($0300) ────────────────────────────────────────────────
    ORG     $0300
DispatchNext:
    SEX     R2                  ; Ensure X = 2 (SP) is invariant!
    LDA     R5                  ; D = fetch opcode byte, R5++
    PLO     RC                  ; RC.0 = opcode
    PLO     RD                  ; RD.0 = opcode
    LDN     RC                  ; D = OpcodeTableLo[opcode]
    PLO     R4                  ; R4.0 = HandlerLo
    LDN     RD                  ; D = OpcodeTableHi[opcode]
    PHI     R4                  ; R4.1 = HandlerHi
    SEP     R4                  ; Execute handler under P=4!
    BR      DispatchNext        ; Loop back when handler returns via SEP R3

; ── Opcode Handlers (P=4) ─────────────────────────────────────────────────────

Op_NOP:
    SEP     R3

Op_PUSH_0:
Op_PUSH_FALSE:
Op_PUSH_NIL:
    DEC     R6
    LDI     0
    STR     R6
    DEC     R6
    STR     R6
    SEP     R3

Op_PUSH_1:
Op_PUSH_TRUE:
    DEC     R6
    LDI     1
    STR     R6
    DEC     R6
    LDI     0
    STR     R6
    SEP     R3

Op_PUSH_NEG1:
    DEC     R6
    LDI     $FF
    STR     R6
    DEC     R6
    STR     R6
    SEP     R3

Op_PUSH_I8:
    LDA     R5
    PLO     RA
    SHL
    BNF     .L_pos8
    LDI     $FF
    PHI     RA
    BR      .L_push_ra
.L_pos8:
    LDI     0
    PHI     RA
.L_push_ra:
    DEC     R6
    GLO     RA
    STR     R6
    DEC     R6
    GHI     RA
    STR     R6
    SEP     R3

Op_PUSH_U8:
    LDA     R5
    PLO     RA
    LDI     0
    PHI     RA
    BR      .L_push_ra

Op_PUSH_I16:
    LDA     R5
    PHI     RA
    LDA     R5
    PLO     RA
    BR      .L_push_ra

Op_PUSH_STR:
    ; 16-bit offset into StringPool
    LDA     R5
    PHI     RA
    LDA     R5
    PLO     RA                  ; RA = offset

    LOAD    RF, _str_pool_base_addr
    LDA     RF
    PHI     R8
    LDN     RF
    PLO     R8                  ; R8 = string pool base (RD preserved!)

    GLO     R8
    STR     R2
    GLO     RA
    ADD
    PLO     RF
    GHI     R8
    STR     R2
    GHI     RA
    ADC
    PHI     RF                  ; RF = R8 + offset (points to len word)

    LDA     RF
    PHI     RA
    LDA     RF
    PLO     RA                  ; RA = length; RF now points to string data!

    ; Push slice [ptr (RF), cap (RA), len (RA)] onto R6:
    DEC     R6
    GLO     RF
    STR     R6
    DEC     R6
    GHI     RF
    STR     R6                  ; push ptr

    DEC     R6
    GLO     RA
    STR     R6
    DEC     R6
    GHI     RA
    STR     R6                  ; push cap

    DEC     R6
    GLO     RA
    STR     R6
    DEC     R6
    GHI     RA
    STR     R6                  ; push len
    SEP     R3

Op_PUSH_NIL_SLICE:
    LDI     0
    DEC     R6
    STR     R6
    DEC     R6
    STR     R6
    DEC     R6
    STR     R6
    DEC     R6
    STR     R6
    DEC     R6
    STR     R6
    DEC     R6
    STR     R6
    SEP     R3

Op_POP:
    INC     R6
    INC     R6
    SEP     R3

Op_POP_SLICE:
    LDI     6
    GLO     R6
    ADD
    PLO     R6
    GHI     R6
    ADCI    0
    PHI     R6
    SEP     R3

Op_DUP:
    LDA     R6
    PHI     RA
    LDN     R6
    PLO     RA
    DEC     R6                  ; R6 restored
    BR      .L_push_ra

Op_SWAP:
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA                  ; RA = top word
    LDA     R6
    PHI     RB
    LDN     R6
    PLO     RB                  ; RB = 2nd word
    GLO     RA
    STR     R6
    DEC     R6
    GHI     RA
    STR     R6
    DEC     R6
    GLO     RB
    STR     R6
    DEC     R6
    GHI     RB
    STR     R6
    SEP     R3

Op_DUP2:
    ; Duplicate top 2 words: [A, B] -> [A, B, A, B]
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA
    LDA     R6
    PHI     RB
    LDN     R6
    PLO     RB
    DEC     R6
    DEC     R6
    DEC     R6
    ; Push RB, then RA:
    DEC     R6
    GLO     RB
    STR     R6
    DEC     R6
    GHI     RB
    STR     R6
    DEC     R6
    GLO     RA
    STR     R6
    DEC     R6
    GHI     RA
    STR     R6
    SEP     R3

Op_ROT:
    ; Rotate top 3 words: [A, B, C] -> [B, C, A]
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA                  ; RA = A (top)
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB                  ; RB = B
    LDA     R6
    PHI     RF
    LDN     R6
    PLO     RF                  ; RF = C
    GLO     RA
    STR     R6
    DEC     R6
    GHI     RA
    STR     R6
    DEC     R6
    GLO     RF
    STR     R6
    DEC     R6
    GHI     RF
    STR     R6
    DEC     R6
    GLO     RB
    STR     R6
    DEC     R6
    GHI     RB
    STR     R6
    SEP     R3

; ── Local Variable Access Handlers ─────────────────────────────────────────────

Op_LOAD_LOCAL_0:
    LDI     0
    BR      _do_load_local
Op_LOAD_LOCAL_1:
    LDI     1
    BR      _do_load_local
Op_LOAD_LOCAL_2:
    LDI     2
    BR      _do_load_local
Op_LOAD_LOCAL_3:
    LDI     3
    BR      _do_load_local
Op_LOAD_LOCAL:
    LDA     R5                  ; D = slot from bytecode
_do_load_local:
    PLO     RB                  ; RB.0 = target slot
    GLO     R7
    PLO RF
    GHI R7
    PHI RF                           ; RF = frame base R7
    LOAD    RD, _var_table_ptr
    LDA     RD
    PHI RA
    LDN RD
    PLO RA                           ; RA points to var sizes
.L_lld_loop:
    GLO     RB
    BZ .L_lld_done
    LDA     RA                  ; D = size of current slot, RA++
    STR     R2
    GLO     RF
    ADD
    PLO RF
    LDI 0
    STR R2
    GHI RF
    ADC
    PHI RF
    DEC     RB
    BR      .L_lld_loop
.L_lld_done:
    LDN     RA
    SMI     1
    BZ      .L_load_byte
    LDN     RA
    SHR                         ; D = size >> 1 (word count)
    PLO     RB
.L_lld_words:
    LDA     RF
    PHI     RA
    LDA     RF
    PLO     RA
    DEC     R6
    GLO     RA
    STR     R6
    DEC     R6
    GHI     RA
    STR     R6
    DEC     RB
    GLO     RB
    BNZ     .L_lld_words
    LOAD    RC, $0100
    LOAD    RD, $0200
    SEP     R3
.L_load_byte:
    LDN     RF
    PLO     RA
    LDI     0
    PHI     RA
    LOAD    RC, $0100
    LOAD    RD, $0200
    BR      .L_push_ra

Op_STORE_LOCAL_0:
    LDI     0
    BR      _do_store_local
Op_STORE_LOCAL_1:
    LDI     1
    BR      _do_store_local
Op_STORE_LOCAL_2:
    LDI     2
    BR      _do_store_local
Op_STORE_LOCAL_3:
    LDI     3
    BR      _do_store_local
Op_STORE_LOCAL:
    LDA     R5
_do_store_local:
    PLO     RB
    GLO     R7
    PLO RF
    GHI R7
    PHI RF
    LOAD    RD, _var_table_ptr
    LDA     RD
    PHI RA
    LDN RD
    PLO RA
.L_sld_loop:
    GLO     RB
    BZ .L_sld_done
    LDA     RA
    STR     R2
    GLO     RF
    ADD
    PLO RF
    LDI 0
    STR R2
    GHI RF
    ADC
    PHI RF
    DEC     RB
    BR      .L_sld_loop
.L_sld_done:
    LDN     RA
    SMI     1
    BZ      .L_store_byte
    LDN     RA                  ; D = size (even, >= 2)
    STR     R2
    GLO     RF
    ADD
    PLO     RF
    GHI     RF
    ADCI    0
    PHI     RF                  ; RF = addr + size
    LDN     RA
    SHR                         ; D = size >> 1 (word count)
    PLO     RB
.L_sld_words:
    LDA     R6
    PHI     RA
    LDA     R6
    DEC     RF
    STR     RF
    DEC     RF
    GHI     RA
    STR     RF
    DEC     RB
    GLO     RB
    BNZ     .L_sld_words
    LOAD    RC, $0100
    LOAD    RD, $0200
    SEP     R3
.L_store_byte:
    INC     R6                  ; skip high byte of stack word
    LDA     R6                  ; D = low byte
    STR     RF
    LOAD    RC, $0100
    LOAD    RD, $0200
    SEP     R3

Op_ADDR_OF_LOCAL:
    LDA     R5                  ; D = slot
    PLO     RB
    GLO     R7
    PLO RF
    GHI R7
    PHI RF
    LOAD    RD, _var_table_ptr
    LDA     RD
    PHI RA
    LDN RD
    PLO RA
.L_ald_loop:
    GLO     RB
    BZ .L_ald_done
    LDA     RA
    STR     R2
    GLO     RF
    ADD
    PLO RF
    LDI 0
    STR R2
    GHI RF
    ADC
    PHI RF
    DEC     RB
    BR      .L_ald_loop
.L_ald_done:
    DEC     R6
    GLO RF
    STR R6
    DEC R6
    GHI RF
    STR R6
    LOAD    RC, $0100
    LOAD RD, $0200
    SEP     R3


Op_ADDR_OF_GLOBAL:
    ; Argument is 16-bit global index
    LDA     R5
    PHI     RB
    LDA     R5
    PLO     RB                  ; RB = global index

    ; GlobalsTable address from _globals_table_addr
    LOAD    RF, _globals_table_addr
    LDA     RF
    PHI     RA
    LDN     RF
    PLO     RA
    GLO     R9
    PLO     RF
    GHI     R9
    PHI     RF                  ; RF = R9 (Globals Data Base)

.L_glob_loop:
    GLO     RB
    BNZ     .L_glob_step
    GHI     RB
    BZ      .L_glob_done
.L_glob_step:
    LDA     RA                  ; D = high byte of size, RA++
    STR     R2                  ; M(R2) = high byte
    LDA     RA                  ; D = low byte of size, RA++
    DEC     R2
    STR     R2                  ; M(R2) = low byte, M(R2+1) = high byte
    GLO     RF
    ADD
    PLO     RF
    INC     R2                  ; points back to high byte
    GHI     RF
    ADC
    PHI     RF
    DEC     RB
    BR      .L_glob_loop

.L_glob_done:
    ; Push address RF onto R6:
    DEC     R6
    GLO     RF
    STR     R6
    DEC     R6
    GHI     RF
    STR     R6
    SEP     R3

Op_LOAD_GLOBAL:
    ; Reads 16-bit global index, loads word from R9+offset
    LDA     R5
    PHI     RB
    LDA     R5
    PLO     RB
    LOAD    RF, _globals_table_addr
    LDA     RF
    PHI     RA
    LDN     RF
    PLO     RA
    GLO     R9
    PLO     RF
    GHI     R9
    PHI     RF
.L_lglob_loop:
    GLO     RB
    BNZ     .L_lglob_step
    GHI     RB
    BZ      .L_lglob_done
.L_lglob_step:
    LDA     RA                  ; D = high byte of size, RA++
    STR     R2                  ; M(R2) = high byte
    LDA     RA                  ; D = low byte of size, RA++
    DEC     R2
    STR     R2                  ; M(R2) = low byte, M(R2+1) = high byte
    GLO     RF
    ADD
    PLO     RF
    INC     R2                  ; points back to high byte
    GHI     RF
    ADC
    PHI     RF
    DEC     RB
    BR      .L_lglob_loop
.L_lglob_done:
    INC     RA                  ; skip high byte of size
    LDN     RA                  ; D = low byte of size
    SMI     1
    BZ      .L_lglob_byte
    LDN     RA
    SHR                         ; D = size >> 1 (word count)
    PLO     RB
.L_lglob_words:
    LDA     RF
    PHI     RA
    LDA     RF
    PLO     RA
    DEC     R6
    GLO     RA
    STR     R6
    DEC     R6
    GHI     RA
    STR     R6
    DEC     RB
    GLO     RB
    BNZ     .L_lglob_words
    SEP     R3
.L_lglob_byte:
    LDN     RF
    PLO     RA
    LDI     0
    PHI     RA
    BR      .L_push_ra

Op_STORE_GLOBAL:
    LDA     R5
    PHI     RB
    LDA     R5
    PLO     RB
    LOAD    RF, _globals_table_addr
    LDA     RF
    PHI     RA
    LDN     RF
    PLO     RA
    GLO     R9
    PLO     RF
    GHI     R9
    PHI     RF
.L_sglob_loop:
    GLO     RB
    BNZ     .L_sglob_step
    GHI     RB
    BZ      .L_sglob_done
.L_sglob_step:
    LDA     RA                  ; D = high byte of size, RA++
    STR     R2                  ; M(R2) = high byte
    LDA     RA                  ; D = low byte of size, RA++
    DEC     R2
    STR     R2                  ; M(R2) = low byte, M(R2+1) = high byte
    GLO     RF
    ADD
    PLO     RF
    INC     R2                  ; points back to high byte
    GHI     RF
    ADC
    PHI     RF
    DEC     RB
    BR      .L_sglob_loop
.L_sglob_done:
    INC     RA                  ; skip high byte of size
    LDN     RA                  ; D = low byte of size
    SMI     1
    BZ      .L_sglob_byte
    LDN     RA                  ; D = size (even, >= 2)
    STR     R2
    GLO     RF
    ADD
    PLO     RF
    GHI     RF
    ADCI    0
    PHI     RF                  ; RF = addr + size
    LDN     RA
    SHR                         ; D = size >> 1 (word count)
    PLO     RB
.L_sglob_words:
    LDA     R6
    PHI     RA
    LDA     R6
    DEC     RF
    STR     RF
    DEC     RF
    GHI     RA
    STR     RF
    DEC     RB
    GLO     RB
    BNZ     .L_sglob_words
    SEP     R3
.L_sglob_byte:
    INC     R6
    LDA     R6
    STR     RF
    SEP     R3

; ── Memory PEEK and POKE ──────────────────────────────────────────────────────

Op_PEEK1:
    LDA     R6
    PHI     RF
    LDA     R6
    PLO     RF                  ; RF = addr
    LDN     RF
    PLO     RA
    LDI     0
    PHI     RA
    BR      .L_push_ra

Op_POKE1:
    ; Stack: [addr, val] -> val at top, addr below
    INC     R6                  ; skip high byte of val word
    LDA     R6
    PLO     RA                  ; RA.0 = val byte
    LDA     R6
    PHI     RF
    LDA     R6
    PLO     RF                  ; RF = addr
    GLO     RA
    STR     RF                  ; store byte to memory at RF
    SEP     R3

Op_PEEK2:
    LDA     R6
    PHI     RF
    LDA     R6
    PLO     RF
    LDA     RF
    PHI     RA
    LDN     RF
    PLO     RA
    BR      .L_push_ra

Op_POKE2:
    ; Stack: [addr, val] -> val at top, addr below
    LDA     R6
    PHI RA
    LDA R6
    PLO RA                           ; RA = val word
    LDA     R6
    PHI RF
    LDA R6
    PLO RF                           ; RF = addr
    GHI     RA
    STR     RF                  ; store high byte
    INC     RF
    GLO     RA
    STR     RF                  ; store low byte
    SEP     R3

Op_SHL1_ADD:
    ; [base, idx] -> [base + idx * 2]
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB                  ; RB = idx
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA                  ; RA = base

    GLO     RB
    SHL
    PLO     RB
    GHI     RB
    SHLC
    PHI     RB                  ; RB = idx * 2

    GLO     RB
    STR     R2
    GLO     RA
    ADD
    PLO     RA
    GHI     RB
    STR     R2
    GHI     RA
    ADC
    PHI     RA
    BR      .L_push_ra

Op_MEM_COPY:
    ; [src, dst, count] -> []
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA                  ; RA = count
    LDA     R6
    PHI     RF
    LDA     R6
    PLO     RF                  ; RF = dst
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB                  ; RB = src

.L_mcopy_loop:
    GLO     RA
    BNZ     .L_mcopy_byte
    GHI     RA
    BZ      .L_mcopy_done
.L_mcopy_byte:
    LDA     RB
    STR     RF
    INC     RF
    DEC     RA
    BR      .L_mcopy_loop
.L_mcopy_done:
    SEP     R3

Op_MEM_SET:
    ; [dst, val, count] -> []
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA                  ; RA = count
    INC     R6
    LDA     R6
    PLO     RB                  ; RB.0 = val byte
    LDA     R6
    PHI     RF
    LDA     R6
    PLO     RF                  ; RF = dst

.L_mset_loop:
    GLO     RA
    BNZ     .L_mset_byte
    GHI     RA
    BZ      .L_mset_done
.L_mset_byte:
    GLO     RB
    STR     RF
    INC     RF
    DEC     RA
    BR      .L_mset_loop
.L_mset_done:
    SEP     R3

; ── Arithmetic & Logical Operations ───────────────────────────────────────────

Op_ADD:
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB                  ; RB = 2nd operand
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA                  ; RA = 1st operand

    GLO     RB
    STR     R2
    GLO     RA
    ADD
    PLO     RA
    GHI     RB
    STR     R2
    GHI     RA
    ADC
    PHI     RA
    BR      .L_push_ra

Op_SUB:
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB                  ; RB = 2nd operand (subtrahend)
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA                  ; RA = 1st operand (minuend)

    GLO     RB
    STR     R2
    GLO     RA
    SM
    PLO     RA
    GHI     RB
    STR     R2
    GHI     RA
    SMB
    PHI     RA
    BR      .L_push_ra

Op_BIT_AND:
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA

    GLO     RB
    STR     R2
    GLO     RA
    AND
    PLO     RA
    GHI     RB
    STR     R2
    GHI     RA
    AND
    PHI     RA
    BR      .L_push_ra

Op_BIT_OR:
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA

    GLO     RB
    STR     R2
    GLO     RA
    OR
    PLO     RA
    GHI     RB
    STR     R2
    GHI     RA
    OR
    PHI     RA
    BR      .L_push_ra

Op_BIT_XOR:
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA

    GLO     RB
    STR     R2
    GLO     RA
    XOR
    PLO     RA
    GHI     RB
    STR     R2
    GHI     RA
    XOR
    PHI     RA
    BR      .L_push_ra

Op_BIT_NOT:
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA
    GLO     RA
    XRI     $FF
    PLO     RA
    GHI     RA
    XRI     $FF
    PHI     RA
    BR      .L_push_ra

Op_SHL:
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB                  ; RB.0 = count
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA                  ; RA = value

.L_shl_loop:
    GLO     RB
    BZ      .L_push_ra
    GLO     RA
    SHL
    PLO     RA
    GHI     RA
    SHLC
    PHI     RA
    DEC     RB
    BR      .L_shl_loop

Op_SHR:
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB                  ; RB.0 = count
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA                  ; RA = value

.L_shr_loop:
    GLO     RB
    BZ      .L_push_ra
    GHI     RA
    SHR
    PHI     RA
    GLO     RA
    SHRC
    PLO     RA
    DEC     RB
    BR      .L_shr_loop

Op_NEG:
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA
    GLO     RA
    STR     R2
    LDI     0
    SM
    PLO     RA
    GHI     RA
    STR     R2
    LDI     0
    SMB
    PHI     RA
    BR      .L_push_ra

Op_MUL:
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA
    ; Multiply RA by RB (result into RA):
    ; Push RB on stack
    GLO     RB
    STXD
    GHI RB
    STXD
    INC R2
    INC R2
    LOAD    RF, 0               ; high accumulator
    LDI     16
    PLO     RB                  ; bit counter = 16
.L_mul16_loop:
    GLO     RA
    SHR
    BNF     .L_mul16_no_add
    GLO     RF
    ADD
    PLO RF
    DEC     R2
    GHI     RF
    ADC
    PHI RF
    INC     R2
    GHI     RF
    SHRC
    PHI RF
    GLO     RF
    SHRC
    PLO RF
    GHI     RA
    SHRC
    PHI RA
    GLO     RA
    SHRC
    PLO RA
    BR      .L_mul16_next
.L_mul16_no_add:
    GHI     RF
    SHR
    PHI RF
    GLO     RF
    SHRC
    PLO RF
    GHI     RA
    SHRC
    PHI RA
    GLO     RA
    SHRC
    PLO RA
.L_mul16_next:
    DEC     RB
    GLO     RB
    BNZ     .L_mul16_loop
    BR      .L_push_ra

Op_DIV:
    LDA     R6
    PHI RB
    LDA R6
    PLO RB                           ; divisor
    LDA     R6
    PHI RA
    LDA R6
    PLO RA                           ; dividend
    ; Unsigned division RA / RB -> quotient RA, remainder RF
    GLO     RB
    STXD
    GHI RB
    STXD
    INC R2
    INC R2
    LOAD    RF, 0
    LDI     16
    PLO     RB
.L_div16_loop:
    GLO     RA
    SHL
    PLO RA
    GHI     RA
    SHLC
    PHI RA
    GLO     RF
    SHLC
    PLO RF
    GHI     RF
    SHLC
    PHI RF
    GLO     RF
    SM
    PLO R0
    DEC     R2
    GHI     RF
    SMB
    INC     R2
    BNF     .L_div16_no_sub
    PHI     RF
    GLO     R0
    PLO RF
    GLO     RA
    ORI 1
    PLO RA
.L_div16_no_sub:
    DEC     RB
    GLO     RB
    BNZ     .L_div16_loop
    BR      .L_push_ra

Op_MOD:
    LDA     R6
    PHI RB
    LDA R6
    PLO RB
    LDA     R6
    PHI RA
    LDA R6
    PLO RA
    GLO     RB
    STXD
    GHI RB
    STXD
    INC R2
    INC R2
    LOAD    RF, 0
    LDI     16
    PLO     RB
.L_mod16_loop:
    GLO     RA
    SHL
    PLO RA
    GHI     RA
    SHLC
    PHI RA
    GLO     RF
    SHLC
    PLO RF
    GHI     RF
    SHLC
    PHI RF
    GLO     RF
    SM
    PLO R0
    DEC     R2
    GHI     RF
    SMB
    INC     R2
    BNF     .L_mod16_no_sub
    PHI     RF
    GLO     R0
    PLO RF
    GLO     RA
    ORI 1
    PLO RA
.L_mod16_no_sub:
    DEC     RB
    GLO     RB
    BNZ     .L_mod16_loop
    ; Remainder in RF -> move to RA:
    GLO     RF
    PLO RA
    GHI     RF
    PHI RA
    BR      .L_push_ra

; ── Comparisons ───────────────────────────────────────────────────────────────

Op_CMP_EQ:
    LDA     R6
    PHI RB
    LDA R6
    PLO RB
    LDA     R6
    PHI RA
    LDA R6
    PLO RA
    GLO     RA
    STR R2
    GLO RB
    SM
    BNZ .L_cmp_false
    GHI     RA
    STR R2
    GHI RB
    SM
    BNZ .L_cmp_false
    LDI     1
    BR .L_push_bool
.L_cmp_false:
    LDI     0
    BR .L_push_bool

Op_CMP_NE:
    LDA     R6
    PHI RB
    LDA R6
    PLO RB
    LDA     R6
    PHI RA
    LDA R6
    PLO RA
    GLO     RA
    STR R2
    GLO RB
    SM
    BNZ .L_cmp_true
    GHI     RA
    STR R2
    GHI RB
    SM
    BNZ .L_cmp_true
    LDI     0
    BR .L_push_bool
.L_cmp_true:
    LDI     1
    BR .L_push_bool

Op_CMP_LT:
    ; RA < RB
    LDA     R6
    PHI RB
    LDA R6
    PLO RB
    LDA     R6
    PHI RA
    LDA R6
    PLO RA
    GLO     RB
    STR R2
    GLO RA
    SM
    GHI     RB
    STR R2
    GHI RA
    SMB
    BDF     .L_cmp_false        ; DF=1 -> RA >= RB (not LT)
    LDI     1
    BR .L_push_bool

Op_CMP_GE:
    ; RA >= RB
    LDA     R6
    PHI RB
    LDA R6
    PLO RB
    LDA     R6
    PHI RA
    LDA R6
    PLO RA
    GLO     RB
    STR R2
    GLO RA
    SM
    GHI     RB
    STR R2
    GHI RA
    SMB
    BNF     .L_cmp_false        ; DF=0 -> borrow (RA < RB)
    LDI     1
    BR .L_push_bool

Op_CMP_GT:
    ; RA > RB (RB < RA)
    LDA     R6
    PHI RB
    LDA R6
    PLO RB
    LDA     R6
    PHI RA
    LDA R6
    PLO RA
    GLO     RA
    STR R2
    GLO RB
    SM
    GHI     RA
    STR R2
    GHI RB
    SMB
    BDF     .L_cmp_false
    LDI     1
    BR .L_push_bool

Op_CMP_LE:
    ; RA <= RB (RB >= RA)
    LDA     R6
    PHI RB
    LDA R6
    PLO RB
    LDA     R6
    PHI RA
    LDA R6
    PLO RA
    GLO     RA
    STR R2
    GLO RB
    SM
    GHI     RA
    STR R2
    GHI RB
    SMB
    BNF     .L_cmp_false
    LDI     1
    BR .L_push_bool

Op_NOT:
    LDA     R6
    PHI RA
    LDA R6
    PLO RA
    GLO     RA
    BNZ .L_cmp_false
    GHI     RA
    BNZ .L_cmp_false
    LDI     1
    BR .L_push_bool

.L_push_bool:
    DEC     R6
    STR     R6
    DEC     R6
    LDI     0
    STR     R6
    SEP     R3

; ── Control Flow ──────────────────────────────────────────────────────────────

Op_JUMP:
    LDA     R5
    PHI     RA
    LDA     R5
    PLO     RA                  ; RA = signed 16-bit offset
    GLO     RA
    STR     R2
    GLO     R5
    ADD
    PLO     R5
    GHI     RA
    STR     R2
    GHI     R5
    ADC
    PHI     R5
    SEP     R3

Op_JUMP_IF_TRUE:
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA
    GLO     RA
    BNZ     Op_JUMP
    GHI     RA
    BNZ     Op_JUMP
    INC     R5                  ; skip 2-byte branch offset
    INC     R5
    SEP     R3

Op_JUMP_IF_FALSE:
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA
    GLO     RA
    BNZ     .L_skip_jump
    GHI     RA
    BNZ     .L_skip_jump
    BR      Op_JUMP
.L_skip_jump:
    INC     R5
    INC     R5
    SEP     R3

Op_CALL:
    ; 16-bit function index in bytecode
    LDA     R5
    PHI     RB
    LDA     R5
    PLO     RB                  ; RB = funcIdx

_do_call_rb:
    ; Push caller state onto R2:
    ; 1. R5 (ReturnPC)
    GLO     R5
    STXD
    GHI     R5
    STXD

    ; 2. R7 (Caller FP)
    GLO     R7
    STXD
    GHI     R7
    STXD

    ; 3. _var_table_ptr
    LOAD    RD, _var_table_ptr
    LDA     RD
    PHI     RA
    LDN     RD
    PLO     RA
    GLO     RA
    STXD
    GHI     RA
    STXD

    ; Save caller_saved_sp (R2) in _call_saved_sp:
    LOAD    RD, _call_saved_sp
    GHI     R2
    STR     RD
    INC     RD
    GLO     R2
    STR     RD

    ; Resolve function entry: FuncTable + funcIdx * 10
    ; Multiply funcIdx by 10:
    GLO     RB
    SHL
    PLO     RA
    GHI     RB
    SHLC
    PHI     RA
    GLO     RA
    SHL
    PLO     R0
    GHI     RA
    SHLC
    PHI     R0
    GLO     R0
    SHL
    PLO     R0
    GHI     R0
    SHLC
    PHI     R0
    GLO     RA
    STR     R2
    GLO     R0
    ADD
    PLO     R0
    GHI     RA
    STR     R2
    GHI     R0
    ADC
    PHI     R0

    LOAD    RD, _func_table_addr
    LDA     RD
    PHI     RF
    LDN     RD
    PLO     RF
    GLO     R0
    STR     R2
    GLO     RF
    ADD
    PLO     RF
    GHI     R0
    STR     R2
    GHI     RF
    ADC
    PHI     RF

    ; RF points to FuncTable[funcIdx]:
    ; [0]=argCount, [1]=localCount, [2..3]=frameSize, [4..5]=codeOffset, [8..9]=varTableOff
    LDA     RF
    PLO     R0                  ; R0.0 = argCount
    INC     RF                  ; skip localCount
    LDA     RF
    PHI     RB
    LDA     RF
    PLO     RB                  ; RB = frameSize
    LDA     RF
    PHI     RA
    LDA     RF
    PLO     RA                  ; RA = codeOffset
    ; Compute target R5 = CodeBase + codeOffset (RA):
    LOAD    RD, _code_base_addr
    LDA     RD
    PHI     R5
    LDN     RD
    PLO     R5
    GLO     RA
    STR     R2
    GLO     R5
    ADD
    PLO     R5
    GHI     RA
    STR     R2
    GHI     R5
    ADC
    PHI     R5

    ; RF points to FuncTable[funcIdx] + 6 (codeSize, 2 bytes)
    INC     RF
    INC     RF                  ; skip 2 codeSize bytes
    LDA     RF
    PHI     RA
    LDN     RF
    PLO     RA                  ; RA = varTableOff

    ; Update _var_table_ptr = LocalVarsTable + varTableOff (RA):
    LOAD    RD, _local_vars_addr
    LDA     RD
    PHI     RF
    LDN     RD
    PLO     RF
    GLO     RA
    STR     R2
    GLO     RF
    ADD
    PLO     RF
    GHI     RA
    STR     R2
    GHI     RF
    ADC
    PHI     RF
    LOAD    RD, _var_table_ptr
    GHI     RF
    STR     RD
    INC     RD
    GLO     RF
    STR     RD


    ; Restore caller_saved_sp from _call_saved_sp into R2:
    LOAD    RD, _call_saved_sp
    LDA     RD
    PHI     R2
    LDN     RD
    PLO     R2

    ; Allocate frame: R7 = R2 - frameSize (RB)
    GLO     RB
    STR     R2
    GLO     R2
    SM
    PLO     R7
    GHI     RB
    STR     R2
    GHI     R2
    SMB
    PHI     R7

    ; Save caller_saved_sp (R2) at Callee R7 - 2:
    DEC     R7
    GLO     R2
    STR     R7
    DEC     R7
    GHI     R2
    STR     R7
    INC     R7
    INC     R7

    ; Set Callee R2 = R7 - 3:
    GLO     R7
    SMI     3
    PLO     R2
    GHI     R7
    SMBI    0
    PHI     R2

    ; Pop arguments into parameter slots in reverse order (argCount-1 down to 0):
.L_pop_args_loop:
    GLO     R0
    BZ      .L_call_done
    DEC     R0                  ; R0.0 = arg index
    GLO     R0
    PLO     RB
    GLO     R7
    PLO     RF
    GHI     R7
    PHI     RF
    LOAD    RD, _var_table_ptr
    LDA     RD
    PHI     RA
    LDN     RD
    PLO     RA
.L_arg_res_loop:
    GLO     RB
    BZ      .L_arg_res_done
    LDA     RA
    STR     R2
    GLO     RF
    ADD
    PLO     RF
    LDI     0
    STR     R2
    GHI     RF
    ADC
    PHI     RF
    DEC     RB
    BR      .L_arg_res_loop
.L_arg_res_done:
    LDN     RA
    SMI     1
    BZ      .L_arg_byte
    LDN     RA                  ; D = slot size (even, >= 2)
    STR     R2
    GLO     RF
    ADD
    PLO     RF
    GHI     RF
    ADCI    0
    PHI     RF                  ; RF = addr + size
    LDN     RA
    SHR                         ; D = size >> 1 (word count)
    PLO     RB
.L_arg_words:
    LDA     R6
    PHI     RA
    LDA     R6
    DEC     RF
    STR     RF
    DEC     RF
    GHI     RA
    STR     RF
    DEC     RB
    GLO     RB
    BNZ     .L_arg_words
    BR      .L_pop_args_loop
.L_arg_byte:
    INC     R6
    LDA     R6
    STR     RF
    BR      .L_pop_args_loop

.L_call_done:
    LOAD    RC, $0100
    LOAD    RD, $0200
    SEP     R3

Op_RET:
Op_RET_VOID:
Op_RET_SLICE:
    ; 1. Restore R2 from Callee R7 - 2:
    DEC     R7
    LDN     R7
    PLO     R2                  ; R2.0 = GLO caller_saved_sp
    DEC     R7
    LDN     R7
    PHI     R2                  ; R2.1 = GHI caller_saved_sp

    ; 2. Pop Caller VarTablePtr:
    INC     R2
    LDA     R2
    PHI     RA
    LDN     R2
    PLO     RA
    LOAD    RD, _var_table_ptr
    GHI     RA
    STR     RD
    INC     RD
    GLO     RA
    STR     RD

    ; 3. Pop Caller FP (R7):
    INC     R2
    LDA     R2
    PHI     R7
    LDN     R2
    PLO     R7

    ; 4. Pop Caller Return PC (R5):
    INC     R2
    LDA     R2
    PHI     R5
    LDN     R2
    PLO     R5

    LOAD    RC, $0100
    LOAD    RD, $0200
    SEP     R3

Op_HALT:
    LDI     0
    LOAD    RF, $FF05
    STR     RF
.L_halt_hang:
    BR      .L_halt_hang

Op_PANIC:
    LOAD    RF, $FF00
    LDI     'P'
    STR RF
    LDI     'A'
    STR RF
    LDI     'N'
    STR RF
    LDI     'I'
    STR RF
    LDI     'C'
    STR RF
    LDI     '\n'
    STR RF
    LDI     1
    LOAD    RF, $FF05
    STR     RF
.L_panic_hang:
    BR      .L_panic_hang

; ── Slices & Strings ──────────────────────────────────────────────────────────

Op_SLICE_NEW:
    ; [ptr, cap, len] already on stack!
    SEP     R3

Op_SLICE_LEN:
    ; Top word on R6 is len: [ptr, cap, len] -> [len]
    LDA     R6
    PHI RA
    LDA R6
    PLO RA                           ; RA = len
    INC     R6
    INC R6
    INC R6
    INC R6                           ; pop cap, ptr
    BR      .L_push_ra

Op_SLICE_CAP:
    INC     R6
    INC R6                           ; skip len
    LDA     R6
    PHI RA
    LDA R6
    PLO RA                           ; RA = cap
    INC     R6
    INC R6                           ; skip ptr
    BR      .L_push_ra

Op_SLICE_SUB:
    ; [slice(ptr, cap, len), start, end] -> [new_slice(ptr+start, cap-start, end-start)]
    LDA     R6
    PHI RB
    LDA R6
    PLO RB                           ; RB = end
    LDA     R6
    PHI RA
    LDA R6
    PLO RA                           ; RA = start
    ; Pop old slice:
    LDA     R6
    PHI R0
    LDA R6
    PLO R0                           ; R0 = old len
    LDA     R6
    PHI RF
    LDA R6
    PLO RF                           ; RF = old cap
    LDA     R6
    PHI RD
    LDA R6
    PLO RD                           ; RD = old ptr (preserve R5 = VM PC!)
    ; Compute new len = end - start:
    GLO     RA
    STR R2
    GLO RB
    SM
    PLO R0
    GHI RA
    STR R2
    GHI RB
    SMB
    PHI R0
    ; Compute new cap = old cap - start:
    GLO     RA
    STR R2
    GLO RF
    SM
    PLO RF
    GHI RA
    STR R2
    GHI RF
    SMB
    PHI RF
    ; Compute new ptr = old ptr + start:
    GLO     RA
    STR R2
    GLO RD
    ADD
    PLO RD
    GHI RA
    STR R2
    GHI RD
    ADC
    PHI RD
    ; Push new slice [ptr (RD), cap (RF), len (R0)]:
    DEC     R6
    GLO RD
    STR R6
    DEC R6
    GHI RD
    STR R6
    DEC     R6
    GLO RF
    STR R6
    DEC R6
    GHI RF
    STR R6
    DEC     R6
    GLO R0
    STR R6
    DEC R6
    GHI R0
    STR R6
    LOAD    RD, $0200
    SEP     R3

Op_STR_CMP:
    ; Pop slice2 (bPtr, bCap, bLen) and slice1 (aPtr, aCap, aLen)
    ; Push 0 if a == b, $FFFF (-1) if a < b, $0001 (+1) if a > b
    LDA     R6
    PHI     RB
    LDA     R6
    PLO     RB                  ; RB = bLen
    INC     R6
    INC     R6                  ; skip bCap
    LDA     R6
    PHI     RD
    LDA     R6
    PLO     RD                  ; RD = bPtr
    LDA     R6
    PHI     RA
    LDA     R6
    PLO     RA                  ; RA = aLen
    INC     R6
    INC     R6                  ; skip aCap
    LDA     R6
    PHI     RF
    LDA     R6
    PLO     RF                  ; RF = aPtr
.L_strcmp_loop:
    GLO     RA
    BNZ     .L_strcmp_a_nonempty
    GHI     RA
    BNZ     .L_strcmp_a_nonempty
    ; aLen == 0: check bLen
    GLO     RB
    BNZ     .L_strcmp_lt
    GHI     RB
    BNZ     .L_strcmp_lt
    ; Both aLen == 0 and bLen == 0 -> equal (0)
    LDI     0
    PLO     RA
    PHI     RA
    LOAD    RD, $0200
    BR      .L_push_ra
.L_strcmp_a_nonempty:
    GLO     RB
    BNZ     .L_strcmp_both_nonempty
    GHI     RB
    BZ      .L_strcmp_gt
.L_strcmp_both_nonempty:
    LDA     RD                  ; D = *bPtr, RD++
    STR     R2                  ; M(R2) = *bPtr
    LDA     RF                  ; D = *aPtr, RF++
    SM                          ; D = *aPtr - *bPtr, DF=1 if *aPtr >= *bPtr
    BZ      .L_strcmp_next
    BDF     .L_strcmp_gt
.L_strcmp_lt:
    LDI     $FF
    PLO     RA
    PHI     RA
    LOAD    RD, $0200
    BR      .L_push_ra
.L_strcmp_gt:
    LDI     1
    PLO     RA
    LDI     0
    PHI     RA
    LOAD    RD, $0200
    BR      .L_push_ra
.L_strcmp_next:
    DEC     RA
    DEC     RB
    BR      .L_strcmp_loop

; ── Hatvan OS Primitives ──────────────────────────────────────────────────────

Op_HATVAN_TRAP:
    LDA     R5                  ; D = call_num
    PLO     RF                  ; save call_num in RF.0

    ; Check supervisor dispatch traps in Task 0:
    GLO     RF
    SMI $F0
    BZ _do_launch_process
    GLO     RF
    SMI $F1
    BZ _do_rbf_call
    GLO     RF
    SMI $F2
    BZ _do_procfs_call
    GLO     RF
    SMI $F3
    BZ _do_init_rbf
    GLO     RF
    SMI $F4
    BZ _do_init_procfs

    ; Regular user / driver syscall trap (or sleep):
    ; Pop U (R9), Y (R8), X (R0), B (RA), A (D) from R6:
    LDA     R6
    PHI     R9
    LDA     R6
    PLO     R9                           ; U
    LDA     R6
    PHI     R8
    LDA     R6
    PLO     R8                           ; Y
    LDA     R6
    PHI     R0
    LDA     R6
    PLO     R0                           ; X (preserved in R0, R7 untouched!)
    INC     R6
    LDA     R6
    PLO     RA                           ; B (low byte of word)
    INC     R6
    LDA     R6                           ; A (D)
    PLO     RB                           ; RB.0 = A (preserved across trap!)

    ; Save call_num at M(R2-1) for OUT 6:
    DEC     R2
    GLO     RF
    STR     R2
    ; Trigger trap fuse via RE ($0004):
    LOAD    RE, $0004
    SEP     RE
    LOAD    RC, $0100
    LOAD    RD, $0200
    LOAD    R3, DispatchNext
    SEP     R3

Op_SYS_EXIT:
    INC     R6
    LDA     R6                  ; D = exit code
    LOAD    RF, $FF05
    STR     RF
.L_exit_hang:
    BR      .L_exit_hang

Op_IO_PRINT:
    ; Pop string slice: [ptr, cap, len]
    ; Print via I$WritLn (0x8C) to STDOUT (Path 1)
    LDA     R6
    PHI R8
    LDA R6
    PLO R8                           ; R8 = len
    INC     R6
    INC R6                           ; skip cap
    LDA     R6
    PHI R7
    LDA R6
    PLO R7                           ; R7 = ptr
    LDI     1                           ; Path 1 (STDOUT)
    LOAD    RE, $0004
    SEP     RE
    LOAD    RC, $0100
    LOAD    RD, $0200
    LOAD    RE, $0004
    SEP     R3

; ── Supervisor Handlers in Task 0 ─────────────────────────────────────────────

_do_launch_process:
    ; Pop arguments from R6: [pid, 0, pc, sp, 0]
    INC     R6
    INC R6                           ; pop U (0)
    INC     R6
    INC R6                           ; pop sp
    LDA     R6
    PHI RA
    LDA R6
    PLO RA                           ; RA = pc (keep R7 frame pointer intact!)
    INC     R6
    INC R6                           ; pop B (0)
    INC     R6
    LDA R6                           ; D = pid
    PLO RB                           ; RB.0 = pid (save immediately before D is clobbered!)

    ; Save Task 0 caller context in kernel_launch_r*_table[pid]:
    GLO     RB
    SHL
    PLO     RF                          ; RF.0 = pid * 2

    LOAD    RD, kernel_launch_r2_table
    GLO     RF
    PLO     RD
    GHI     R2
    STR     RD
    INC     RD
    GLO     R2
    STR     RD

    LOAD    RD, kernel_launch_r5_table
    GLO     RF
    ADI     $20
    PLO     RD
    GHI     R5
    STR     RD
    INC     RD
    GLO     R5
    STR     RD

    LOAD    RD, kernel_launch_r6_table
    GLO     RF
    ADI     $40
    PLO     RD
    GHI     R6
    STR     RD
    INC     RD
    GLO     R6
    STR     RD

    LOAD    RD, kernel_launch_r7_table
    GLO     RF
    ADI     $60
    PLO     RD
    GHI     R7
    STR     RD
    INC     RD
    GLO     R7
    STR     RD

    ; Also save into task_table[0]:
    LOAD    RD, task_r2_table
    GHI     R2
    STR RD
    INC RD
    GLO R2
    STR RD
    LOAD    RD, task_r5_table
    GHI     R5
    STR RD
    INC RD
    GLO R5
    STR RD
    LOAD    RD, task_r6_table
    GHI     R6
    STR RD
    INC RD
    GLO R6
    STR RD
    LOAD    RD, task_r7_table
    GHI     R7
    STR RD
    INC RD
    GLO R7
    STR RD

    ; Save pid in _curr_task and _trap_user_pid:
    LOAD    RD, _curr_task
    GLO     RB
    STR     RD
    LOAD    RD, _trap_user_pid
    GLO     RB
    STR     RD

    ; Set in_kernel = 0
    LOAD    RD, _in_kernel
    LDI     0
    STR     RD

    ; Arm TaskFuse with target pid:
    LOAD    RD, $FF20
    GLO     RB
    STR     RD

    ; Check if target is a newly spawned task (pc == 0 or pc == $0020):
    GHI     RA
    BNZ     .L_resume_existing
    GLO     RA
    BZ      .L_start_new_task
    SMI     $20
    BNZ     .L_resume_existing

.L_start_new_task:
    ; Start new task at _user_entry ($0020) under P=0:
    LOAD    R0, $0020
    SEP     R0                          ; MMU flips to target task under P=0!

.L_resume_existing:
    ; Restore target task's context from Page 0 task tables:
    GLO     RB
    SHL
    PLO     RA                          ; RA.0 = pid * 2
    LDI     0
    PHI     RD

    GLO     RA
    ADI     $80                         ; task_r2_table ($0080)
    PLO     RD
    LDA     RD
    PHI     R2
    LDN     RD
    PLO     R2

    GLO     RA
    ADI     $C0                         ; task_r6_table ($00C0)
    PLO     RD
    LDA     RD
    PHI     R6
    LDN     RD
    PLO     R6

    GLO     RA
    ADI     $E0                         ; task_r7_table ($00E0)
    PLO     RD
    LDA     RD
    PHI     R7
    LDN     RD
    PLO     R7

    GLO     RA
    ADI     $A0                         ; task_r5_table ($00A0)
    PLO     RD
    LDA     RD
    PHI     R5
    LDN     RD
    PLO     R5

    LOAD    R3, _resume_user_task
    SEP     R3                          ; MMU flips to target task under P=3, enters _resume_user_task!

_do_rbf_call:
    ; Pop 5 words from R6 (arguments to hatvan_trap):
    GLO     R6
    ADI     10
    PLO     R6
    GHI     R6
    ADCI    0
    PHI     R6
    LDI     1
    BR      _switch_to_driver

_do_procfs_call:
    ; Pop 5 words from R6 (arguments to hatvan_trap):
    GLO     R6
    ADI     10
    PLO     R6
    GHI     R6
    ADCI    0
    PHI     R6
    LDI     2
    BR      _switch_to_driver

_switch_to_driver:
    PLO     RB                          ; RB.0 = target driver task (1 or 2)
    ; Save Task 0 context into task_table[0]:
    LOAD    RD, task_r2_table
    GHI     R2
    STR RD
    INC RD
    GLO R2
    STR RD
    LOAD    RD, task_r5_table
    GHI     R5
    STR RD
    INC RD
    GLO R5
    STR RD
    LOAD    RD, task_r6_table
    GHI     R6
    STR RD
    INC RD
    GLO R6
    STR RD
    LOAD    RD, task_r7_table
    GHI     R7
    STR RD
    INC RD
    GLO R7
    STR RD

    LOAD    RD, _curr_task
    GLO     RB
    STR     RD

    LOAD    RD, _in_kernel
    LDI     0
    STR     RD

    LOAD    RD, $FF20                   ; TaskFuse
    GLO     RB
    STR     RD

    ; Load driver context from Page 0 task tables:
    GLO     RB
    SHL
    PLO     RA                          ; RA.0 = PID * 2
    LDI     0
    PHI     RD

    GLO     RA
    ADI     $80                         ; task_r2_table ($0080)
    PLO     RD
    LDA     RD
    PHI     R2
    LDN     RD
    PLO     R2

    GLO     RA
    ADI     $A0                         ; task_r5_table ($00A0)
    PLO     RD
    LDA     RD
    PHI     R5
    LDN     RD
    PLO     R5

    GLO     RA
    ADI     $C0                         ; task_r6_table ($00C0)
    PLO     RD
    LDA     RD
    PHI     R6
    LDN     RD
    PLO     R6

    GLO     RA
    ADI     $E0                         ; task_r7_table ($00E0)
    PLO     RD
    LDA     RD
    PHI     R7
    LDN     RD
    PLO     R7

    ; Push 1 dummy return value onto Driver Task R6 for POP:
    LDI     0
    DEC     R6
    STR     R6
    DEC     R6
    STR     R6

    LOAD    R3, _resume_user_task
    SEP     R3                          ; MMU flips to driver under P=3, enters _resume_user_task!

_do_init_rbf:
    ; Pop 5 words from R6 (arguments to hatvan_trap):
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6

    ; Save Task 0 context into task_table[0]:
    LOAD    RD, task_r2_table
    GHI     R2
    STR     RD
    INC     RD
    GLO     R2
    STR     RD
    LOAD    RD, task_r5_table
    GHI     R5
    STR     RD
    INC     RD
    GLO     R5
    STR     RD
    LOAD    RD, task_r6_table
    GHI     R6
    STR     RD
    INC     RD
    GLO     R6
    STR     RD
    LOAD    RD, task_r7_table
    GHI     R7
    STR     RD
    INC     RD
    GLO     R7
    STR     RD

    LOAD    RD, _curr_task
    LDI     1
    STR     RD
    LOAD    RD, _in_kernel
    LDI     0
    STR     RD

    ; Arm TaskFuse with target Task 1:
    LOAD    RD, $FF20
    LDI     1
    STR     RD

    ; Initial execution in Task 1 starts at _user_entry ($0020) under P=0:
    LOAD    R0, $0020
    SEP     R0

_do_init_procfs:
    ; Pop 5 words from R6:
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6
    INC     R6

    ; Save Task 0 context into task_table[0]:
    LOAD    RD, task_r2_table
    GHI     R2
    STR     RD
    INC     RD
    GLO     R2
    STR     RD
    LOAD    RD, task_r5_table
    GHI     R5
    STR     RD
    INC     RD
    GLO     R5
    STR     RD
    LOAD    RD, task_r6_table
    GHI     R6
    STR     RD
    INC     RD
    GLO     R6
    STR     RD
    LOAD    RD, task_r7_table
    GHI     R7
    STR     RD
    INC     RD
    GLO     R7
    STR     RD

    LOAD    RD, _curr_task
    LDI     2
    STR     RD
    LOAD    RD, _in_kernel
    LDI     0
    STR     RD

    ; Arm TaskFuse with target Task 2:
    LOAD    RD, $FF20
    LDI     2
    STR     RD

    ; Initial execution in Task 2 starts at _user_entry ($0020) under P=0:
    LOAD    R0, $0020
    SEP     R0

; ── Syscall Kernel Trap Handler (_SyscallTrapEntry) ───────────────────────────
_SyscallTrapEntry:
    ; Entered at $0006 with P=E!
    LOAD    R4, .L_trap_p4_start
    SEP     R4                          ; Switch P from E to 4!
.L_trap_p4_start:
    LOAD    RE, $0004                   ; Restore RE = $0004 for next trap!
    SEX     R2                          ; Ensure X = 2 (native stack pointer)!

    ; Check if caller is Driver Task 1 or 2 returning from Sleep():
    LOAD    RD, $FF61
    LDN     RD
    PLO     RD
    BNZ     .L_trap_pid_ok
    LOAD    RD, _curr_task
    LDN     RD
    PLO     RD
.L_trap_pid_ok:
    GLO     RD
    SMI     1
    BZ      .L_driver_sleep_save
    GLO     RD
    SMI     2
    BZ      .L_driver_sleep_save
    BR      .L_user_trap_save

.L_driver_sleep_save:
    ; Caller is Driver Task (1 or 2) yielding back to Task 0!
    LOAD    RD, $FF61
    LDN     RD
    PLO     RB
    BNZ     .L_drv_pid_ok
    LOAD    RD, _curr_task
    LDN     RD
    PLO     RB
.L_drv_pid_ok:
    GLO     RB
    SHL
    PLO     RA                          ; RA.0 = caller PID * 2
    LDI     0
    PHI     RD

    GLO     RA
    ADI     $80                         ; task_r2_table ($0080)
    PLO     RD
    GHI     R2
    STR     RD
    INC     RD
    GLO     R2
    STR     RD

    GLO     RA
    ADI     $A0                         ; task_r5_table ($00A0)
    PLO     RD
    GHI     R5
    STR     RD
    INC     RD
    GLO     R5
    STR     RD

    GLO     RA
    ADI     $C0                         ; task_r6_table ($00C0)
    PLO     RD
    GHI     R6
    STR     RD
    INC     RD
    GLO     R6
    STR     RD

    GLO     RA
    ADI     $E0                         ; task_r7_table ($00E0)
    PLO     RD
    GHI     R7
    STR     RD
    INC     RD
    GLO     R7
    STR     RD

    ; Return to Task 0 caller without touching _trap_user_* or _trap_call_num:
    BR      .L_driver_returned

.L_user_trap_save:
    ; 1. Save user trap parameters BEFORE clobbering D, RA, R8, R9, R0, RB:
    ; RB.0 = user A, RA.0 = user B, R0 = user X, R8 = user Y, R9 = user U, R5 = user PC
    LOAD    RD, _trap_user_a
    GLO     RB
    STR     RD
    LOAD    RD, _trap_user_b
    GLO     RA
    STR     RD
    LOAD    RD, _trap_user_x
    GHI     R0
    STR     RD
    INC     RD
    GLO     R0
    STR     RD
    LOAD    RD, _trap_user_y
    GHI     R8
    STR     RD
    INC     RD
    GLO     R8
    STR     RD
    LOAD    RD, _trap_user_u
    GHI     R9
    STR     RD
    INC     RD
    GLO     R9
    STR     RD
    LOAD    RD, _trap_user_pc
    GHI     R5
    STR     RD
    INC     RD
    GLO     R5
    STR     RD

    ; 2. Save caller context (clean R2, R5, R6, clean R7) into task_table[_curr_task]:
    LOAD    RD, $FF61
    LDN     RD
    PLO     RB                          ; RB.0 = caller PID from MMU TrapSourceTask
    BNZ     .L_caller_pid_valid
    LOAD    RD, _curr_task
    LDN     RD
    PLO     RB
.L_caller_pid_valid:
    LOAD    RD, _curr_task
    GLO     RB
    STR     RD
    LOAD    RD, _trap_user_pid
    GLO     RB
    STR     RD
    GLO     RB
    SHL
    PLO     RA                          ; RA.0 = caller PID * 2
    LDI     0
    PHI     RD

    GLO     RA
    ADI     $80                         ; task_r2_table ($0080)
    PLO     RD
    GHI     R2
    STR     RD
    INC     RD
    GLO     R2
    STR     RD

    GLO     RA
    ADI     $A0                         ; task_r5_table ($00A0)
    PLO     RD
    GHI     R5
    STR     RD
    INC     RD
    GLO     R5
    STR     RD

    GLO     RA
    ADI     $C0                         ; task_r6_table ($00C0)
    PLO     RD
    GHI     R6
    STR     RD
    INC     RD
    GLO     R6
    STR     RD

    GLO     RA
    ADI     $E0                         ; task_r7_table ($00E0)
    PLO     RD
    GHI     R7
    STR     RD
    INC     RD
    GLO     R7
    STR     RD

    ; Read callNum from $FF60:
    LOAD    RD, $FF60
    LDN     RD
    PLO     RB                          ; RB.0 = callNum
    LOAD    RD, _trap_call_num
    GLO     RB
    STR RD

    ; User task syscall (PID >= 3):
    ; Switch to Task 0 kernel context:
    LOAD    RD, _in_kernel
    LDI     1
    STR     RD
    LOAD    RD, _curr_task
    LDI     0
    STR     RD

    LOAD    RD, task_r2_table
    LDA     RD
    PHI R2
    LDN RD
    PLO R2
    LOAD    RD, task_r6_table
    LDA     RD
    PHI R6
    LDN RD
    PLO R6
    LOAD    RD, task_r7_table
    LDA     RD
    PHI R7
    LDN RD
    PLO R7

    ; Push caller PID and callNum onto Task 0 R2 to preserve across nested syscalls:
    LOAD    RD, _trap_user_pid
    LDN     RD
    STXD
    LOAD    RD, _trap_call_num
    LDN     RD
    STXD

    ; Push HandleSyscall arguments onto Task 0 operand stack R6:
    ; HandleSyscall(callNum byte, a byte, b byte, x word, y word, u word, pc word)
    ; Push callNum:
    LOAD    RD, _trap_call_num
    LDN RD
    PLO RA
    LDI 0
    PHI RA
    DEC R6
    GLO RA
    STR R6
    DEC R6
    GHI RA
    STR R6
    ; Push a:
    LOAD    RD, _trap_user_a
    LDN RD
    PLO RA
    LDI 0
    PHI RA
    DEC R6
    GLO RA
    STR R6
    DEC R6
    GHI RA
    STR R6
    ; Push b:
    LOAD    RD, _trap_user_b
    LDN RD
    PLO RA
    LDI 0
    PHI RA
    DEC R6
    GLO RA
    STR R6
    DEC R6
    GHI RA
    STR R6
    ; Push x:
    LOAD    RD, _trap_user_x
    LDA RD
    PHI RA
    LDN RD
    PLO RA
    DEC R6
    GLO RA
    STR R6
    DEC R6
    GHI RA
    STR R6
    ; Push y:
    LOAD    RD, _trap_user_y
    LDA RD
    PHI RA
    LDN RD
    PLO RA
    DEC R6
    GLO RA
    STR R6
    DEC R6
    GHI RA
    STR R6
    ; Push u:
    LOAD    RD, _trap_user_u
    LDA RD
    PHI RA
    LDN RD
    PLO RA
    DEC R6
    GLO RA
    STR R6
    DEC R6
    GHI RA
    STR R6
    ; Push pc:
    LOAD    RD, _trap_user_pc
    LDA RD
    PHI RA
    LDN RD
    PLO RA
    DEC R6
    GLO RA
    STR R6
    DEC R6
    GHI RA
    STR R6

    ; Restore Task 0's R8 and R9:
    LOAD    RD, _str_pool_base_addr
    LDA     RD
    PHI     R8
    LDN     RD
    PLO     R8
    LOAD    RD, _globals_base_addr
    LDA     RD
    PHI     R9
    LDN     RD
    PLO     R9

    ; Set Return PC = _un_trap_stub ($0030)
    LOAD    R5, _un_trap_stub

    ; Function index of syscall_HandleSyscall from vector $0014:
    LOAD    RD, _fn_dispatch_syscall
    LDA     RD
    PHI     RB
    LDN     RD
    PLO     RB                           ; RB = funcIdx of HandleSyscall

    LBR     _do_call_rb

.L_driver_returned:
    ; Driver Task 1 or 2 called hal.Sleep()!
    LOAD    RD, _in_kernel
    LDI     1
    STR     RD
    LOAD    RD, _curr_task
    LDI     0
    STR     RD

    LOAD    RD, task_r2_table
    LDA     RD
    PHI R2
    LDN RD
    PLO R2
    LOAD    RD, task_r5_table
    LDA     RD
    PHI R5
    LDN RD
    PLO R5
    LOAD    RD, task_r6_table
    LDA     RD
    PHI R6
    LDN RD
    PLO R6
    LOAD    RD, task_r7_table
    LDA     RD
    PHI R7
    LDN RD
    PLO R7

    LOAD    RD, _str_pool_base_addr
    LDA     RD
    PHI     R8
    LDN     RD
    PLO     R8
    LOAD    RD, _globals_base_addr
    LDA     RD
    PHI     R9
    LDN     RD
    PLO     R9

    ; Push 1 dummy return value onto Task 0 R6 for POP:
    LDI     0
    DEC     R6
    STR     R6
    DEC     R6
    STR     R6

    LOAD    RC, $0100
    LOAD    RD, $0200
    LOAD    R3, DispatchNext
    SEP     R3

; ── Op_HATVAN_UNTRAP ($C1) ───────────────────────────────────────────────────
Op_HATVAN_UNTRAP:
    ; HandleSyscall returned [resA, resB, resY, err] on R6 in Task 0!
    ; 1. Copy debug return values into _trap_ret_* using RF without modifying R6:
    GLO     R6
    PLO     RF
    GHI     R6
    PHI     RF
    INC     RF                          ; RF points to err (R6+1)
    LOAD    RD, _trap_ret_err
    LDA     RF                          ; D = err, RF points to resY hi (R6+2)
    STR     RD
    LOAD    RD, _trap_ret_y
    LDA     RF                          ; D = resY hi, RF points to resY lo (R6+3)
    STR     RD
    INC     RD
    LDA     RF                          ; D = resY lo, RF points to resB hi (R6+4)
    STR     RD
    INC     RF                          ; RF points to resB lo (R6+5)
    LOAD    RD, _trap_ret_b
    LDA     RF                          ; D = resB, RF points to resA hi (R6+6)
    STR     RD
    INC     RF                          ; RF points to resA lo (R6+7)
    LOAD    RD, _trap_ret_a
    LDN     RF                          ; D = resA
    STR     RD

    ; 2. Pop callNum and caller PID preserved on Task 0 R2:
    INC     R2                          ; R2 points to callNum
    LDA     R2                          ; D = callNum, R2 increments to caller PID
    PLO     RF                          ; RF.0 = callNum
    LDN     R2                          ; D = caller PID, R2 remains at Task 0 SP
    PLO     RB                          ; RB.0 = caller PID

    ; Check if callNum was F$Exit ($06):
    GLO     RF
    SMI     $06
    BZ      .L_user_exited

    ; 3. Not F$Exit: Return to user task!
    GLO     RB
    SHL
    PLO     RA                          ; RA.0 = user PID * 2

    ; 4. Get user saved R6 from task_r6_table, decrement by 8, and save back:
    LDI     0
    PHI     RD
    GLO     RA
    ADI     $C0                         ; task_r6_table ($00C0)
    PLO     RD
    LDA     RD
    PHI     RF
    LDN     RD
    PLO     RF                          ; RF = user saved R6
    GLO     RF
    SMI     8
    PLO     RF
    GHI     RF
    SMBI    0
    PHI     RF                          ; RF = new user R6 (allocated 8 bytes for 4 return words)
    GLO     RF
    STR     RD                          ; update task_r6_table low byte (RD points to task_r6_table + pid*2 + 1)
    DEC     RD
    GHI     RF
    STR     RD                          ; update task_r6_table high byte

    ; 5. DMA 8 bytes from Task 0 R6 to user PID at new R6 (RF):
    LOAD    RD, $FF24                   ; PORT_DMASRC
    LDI     0
    STR     RD                          ; DMASRC task = 0
    INC     RD
    LDI     0
    STR     RD                          ; DMASRC page = 0
    INC     RD
    GHI     R6
    STR     RD                          ; DMASRC addr hi
    INC     RD
    GLO     R6
    STR     RD                          ; DMASRC addr lo

    LOAD    RD, $FF28                   ; PORT_DMADST
    GLO     RB
    STR     RD                          ; DMADST task = user PID
    INC     RD
    LDI     0
    STR     RD                          ; DMADST page = 0
    INC     RD
    GHI     RF
    STR     RD                          ; DMADST addr hi
    INC     RD
    GLO     RF
    STR     RD                          ; DMADST addr lo

    LOAD    RD, $FF2C                   ; PORT_DMACNT
    LDI     0
    STR     RD
    INC     RD
    LDI     0
    STR     RD
    INC     RD
    LDI     0
    STR     RD
    INC     RD
    LDI     8
    STR     RD                          ; DMACNT = 8 bytes

    LOAD    RD, $FF21                   ; PORT_DMACMD
    LDI     1
    STR     RD                          ; Execute DMA copy!

    ; 6. Clean 8 bytes off Task 0 R6:
    GLO     R6
    ADI     8
    PLO     R6
    GHI     R6
    ADCI    0
    PHI     R6

    ; 7. Restore Task 0 clean context in task_table[0] from kernel_launch_r*_table[pid]:
    LOAD    RD, kernel_launch_r2_table
    GLO     RA
    PLO     RD
    LDA     RD
    PHI     R0
    LDN     RD
    PLO     R0
    LOAD    RD, task_r2_table
    GHI     R0
    STR     RD
    INC     RD
    GLO     R0
    STR     RD

    LOAD    RD, kernel_launch_r5_table
    GLO     RA
    ADI     $20
    PLO     RD
    LDA     RD
    PHI     R0
    LDN     RD
    PLO     R0
    LOAD    RD, task_r5_table
    GHI     R0
    STR     RD
    INC     RD
    GLO     R0
    STR     RD

    LOAD    RD, kernel_launch_r6_table
    GLO     RA
    ADI     $40
    PLO     RD
    LDA     RD
    PHI     R0
    LDN     RD
    PLO     R0
    LOAD    RD, task_r6_table
    GHI     R0
    STR     RD
    INC     RD
    GLO     R0
    STR     RD

    LOAD    RD, kernel_launch_r7_table
    GLO     RA
    ADI     $60
    PLO     RD
    LDA     RD
    PHI     R0
    LDN     RD
    PLO     R0
    LOAD    RD, task_r7_table
    GHI     R0
    STR     RD
    INC     RD
    GLO     R0
    STR     RD

    ; 8. Update _curr_task and arm TaskFuse with user PID:
    LOAD    RD, _curr_task
    GLO     RB
    STR     RD
    LOAD    RD, $FF20                   ; Arm TaskFuse
    GLO     RB
    STR     RD

    ; 8. Restore user task context from Page 0 task tables:
    LDI     0
    PHI     RD

    GLO     RA
    ADI     $80                         ; task_r2_table ($0080)
    PLO     RD
    LDA     RD
    PHI     R2
    LDN     RD
    PLO     R2

    GLO     RA
    ADI     $A0                         ; task_r5_table ($00A0)
    PLO     RD
    LDA     RD
    PHI     R5
    LDN     RD
    PLO     R5

    GLO     RA
    ADI     $C0                         ; task_r6_table ($00C0)
    PLO     RD
    LDA     RD
    PHI     R6
    LDN     RD
    PLO     R6

    GLO     RA
    ADI     $E0                         ; task_r7_table ($00E0)
    PLO     RD
    LDA     RD
    PHI     R7
    LDN     RD
    PLO     R7

    LOAD    RD, _in_kernel
    LDI     0
    STR     RD

    ; 9. Switch to user task at _resume_user_task under P=3!
    LOAD    R3, _resume_user_task
    SEP     R3                          ; MMU flips to user task at _resume_user_task!

_resume_user_task:
    ; Runs in target task under P=3!
    ; Restore target task's StringPool (R8) and Globals (R9):
    LOAD    RD, _str_pool_base_addr
    LDA     RD
    PHI     R8
    LDN     RD
    PLO     R8
    LOAD    RD, _globals_base_addr
    LDA     RD
    PHI     R9
    LDN     RD
    PLO     R9
    LOAD    RC, $0100
    LOAD    RD, $0200
    LOAD    RE, $0004
    LBR     DispatchNext                ; Direct branch to DispatchNext under P=3!

.L_user_exited:
    ; Process exited! Restore Task 0's LaunchProcess caller context for this PID:
    GLO     RB
    SHL
    PLO     RA                          ; RA.0 = caller PID * 2

    LOAD    RD, kernel_launch_r2_table
    GLO     RA
    PLO     RD
    LDA     RD
    PHI     R2
    LDN     RD
    PLO     R2

    LOAD    RD, kernel_launch_r5_table
    GLO     RA
    ADI     $20
    PLO     RD
    LDA     RD
    PHI     R5
    LDN     RD
    PLO     R5

    LOAD    RD, kernel_launch_r6_table
    GLO     RA
    ADI     $40
    PLO     RD
    LDA     RD
    PHI     R6
    LDN     RD
    PLO     R6

    LOAD    RD, kernel_launch_r7_table
    GLO     RA
    ADI     $60
    PLO     RD
    LDA     RD
    PHI     R7
    LDN     RD
    PLO     R7

    LOAD    RD, _str_pool_base_addr
    LDA     RD
    PHI     R8
    LDN     RD
    PLO     R8
    LOAD    RD, _globals_base_addr
    LDA     RD
    PHI     R9
    LDN     RD
    PLO     R9

    ; Ensure Task 0 state:
    LOAD    RD, _curr_task
    LDI     0
    STR     RD
    LOAD    RD, _in_kernel
    LDI     1
    STR     RD

    ; Push 1 dummy return value onto Task 0 R6 for POP:
    LDI     0
    DEC     R6
    STR     R6
    DEC     R6
    STR     R6

    LOAD    RC, $0100
    LOAD    RD, $0200
    LOAD    RE, $0004
    LOAD    R3, DispatchNext
    SEP     R3                          ; Returns to LaunchProcess caller in Task 0!

; ── Timer Interrupt Handler (_TimerIRQEntry) ──────────────────────────────────
_TimerIRQEntry:
    ; Hardware forces P=1, X=2, IE=0.
    ; Save T, D, RF, and DF on native stack R2:
    DEC     R2
    SAV                                 ; M(R2) = T (P and X before interrupt)
    DEC     R2
    STR     R2                          ; M(R2-1) = D
    GHI     RF
    DEC     R2
    STR     R2                          ; M(R2-2) = RF.1
    GLO     RF
    DEC     R2
    STR     R2                          ; M(R2-3) = RF.0
    LDI     0
    SHLC
    DEC     R2
    STR     R2                          ; M(R2-4) = DF

    ; Acknowledge timer IRQ ($FF02):
    LOAD    RF, $FF02
    LDI     1
    STR     RF

    ; Restore DF, RF, and D:
    LDA     R2                          ; D = saved DF
    SHR                                 ; DF restored!
    LDA     R2
    PLO     RF                          ; RF.0 restored!
    LDA     R2
    PHI     RF                          ; RF.1 restored!
    LDA     R2                          ; D restored! R2 now points to saved T!

    ; RET reads T from M(R2), restores P and X, sets IE=1, and increments R2:
    RET
    ; R1 points here after RET. On the next interrupt, P=1 fetches this LBR:
    LBR     _TimerIRQEntry

; ── Task 0 Kernel Bootloader (_cstart_vmc) ────────────────────────────────────
_cstart_vmc:
    LOAD    R1, $0008           ; IRQ vector
    LOAD    RE, $0004           ; Trap vector
    LOAD    RC, $0100           ; OpcodeTableLo
    LOAD    RD, $0200           ; OpcodeTableHi
    LOAD    R2, $DFFE           ; Native stack pointer
    SEX     R2
    LOAD    R6, $B000           ; Par4 operand stack
    LOAD    R7, 0               ; Frame pointer

    ; Set _curr_task = 0, _in_kernel = 1:
    LOAD    RF, _curr_task
    LDI     0
    STR     RF
    LOAD    RF, _in_kernel
    LDI     1
    STR     RF

    ; Fall through to _user_start to parse kernel's .p4p at $1000:

; ── Bytecode Parser & Task Startup (_user_start) ──────────────────────────────
_user_start:
    LOAD    R2, $DFFE
    SEX     R2

    ; Check magic "P4P\x01" at $2000:
    LOAD    RF, $2000
    LDA     RF
    SMI $50
    BNZ _halt_bad_magic
    LDA     RF
    SMI $34
    BNZ _halt_bad_magic
    LDA     RF
    SMI $50
    BNZ _halt_bad_magic
    LDA     RF
    SMI $01
    BNZ _halt_bad_magic

    ; Skip flags (offset 4, 5)
    INC     RF
    INC     RF

    ; Read stringPoolSize (offset 6, 7) into _str_pool_size
    LOAD    RD, _str_pool_size
    LDA     RF
    STR RD
    INC RD
    LDA RF
    STR RD

    ; Read globalsCount (offset 8, 9) into _globals_count
    LOAD    RD, _globals_count
    LDA     RF
    STR RD
    INC RD
    LDA RF
    STR RD

    ; Read funcCount (offset 10, 11) into _func_count
    LOAD    RD, _func_count
    LDA     RF
    STR RD
    INC RD
    LDA RF
    STR RD

    ; Read entryFuncIdx (offset 12, 13) into _entry_func_idx
    LOAD    RD, _entry_func_idx
    LDA     RF
    STR RD
    INC RD
    LDA RF
    STR RD

    ; Read codeSize (offset 14, 15) into _code_size
    LOAD    RD, _code_size
    LDA     RF
    STR RD
    INC RD
    LDA RF
    STR RD

    ; RF is now at GlobalsTable (Base + 16):
    LOAD    RD, _globals_table_addr
    GHI     RF
    STR     RD
    INC     RD
    GLO     RF
    STR     RD

    ; Sum sizes of all globals to find _total_glob_bytes:
    LOAD    R8, 0
    LOAD    RD, _globals_count
    LDA     RD
    PHI RB
    LDN RD
    PLO RB
.L_sum_globals_loop:
    GLO     RB
    BNZ .L_sum_g_step
    GHI     RB
    BZ  .L_sum_globals_done
.L_sum_g_step:
    LDA     RF                  ; D = high byte of size, RF++
    STR     R2                  ; M(R2) = high byte
    LDA     RF                  ; D = low byte of size, RF++
    DEC     R2
    STR     R2                  ; M(R2) = low byte, M(R2+1) = high byte
    GLO     R8
    ADD
    PLO     R8
    INC     R2                  ; points back to high byte
    GHI     R8
    ADC
    PHI     R8
    DEC     RB
    BR      .L_sum_globals_loop
.L_sum_globals_done:
    LOAD    RD, _total_glob_bytes
    GHI     R8
    STR RD
    INC RD
    GLO R8
    STR RD

    ; Now RF is at StringPoolBase:
    LOAD    RD, _str_pool_base_addr
    GHI     RF
    STR     RD
    INC     RD
    GLO     RF
    STR     RD
    GLO     RF
    PLO R8
    GHI     RF
    PHI R8

    ; FuncTable = StringPoolBase (RF) + stringPoolSize (_str_pool_size)
    LOAD    RD, _str_pool_size
    LDA     RD
    PHI RA
    LDN RD
    PLO RA
    GLO     RA
    STR R2
    GLO RF
    ADD
    PLO RF
    GHI RA
    STR R2
    GHI RF
    ADC
    PHI RF
    LOAD    RD, _func_table_addr
    GHI     RF
    STR RD
    INC RD
    GLO RF
    STR RD

    ; LocalVarsTable = FuncTable (RF) + funcCount * 10
    LOAD    RD, _func_count
    LDA     RD
    PHI RB
    LDN RD
    PLO RB
    GLO     RB
    SHL
    PLO RA
    GHI RB
    SHLC
    PHI RA
    GLO     RA
    SHL
    PLO R4
    GHI RA
    SHLC
    PHI R4
    GLO     R4
    SHL
    PLO R4
    GHI R4
    SHLC
    PHI R4
    GLO     RA
    STR R2
    GLO R4
    ADD
    PLO R4
    GHI RA
    STR R2
    GHI R4
    ADC
    PHI R4
    GLO     R4
    STR R2
    GLO RF
    ADD
    PLO RF
    GHI R4
    STR R2
    GHI RF
    ADC
    PHI RF
    LOAD    RD, _local_vars_addr
    GHI     RF
    STR RD
    INC RD
    GLO RF
    STR RD

    ; Calculate localVarsSize: sum of (argCount + localCount) across all functions
    LOAD    RD, _func_table_addr
    LDA     RD
    PHI RA
    LDN RD
    PLO RA
    LOAD    RD, _func_count
    LDA     RD
    PHI RB
    LDN RD
    PLO RB
    LOAD    R4, 0
.L_sum_lvars_loop:
    GLO     RB
    BNZ .L_sum_lv_step
    GHI     RB
    BZ  .L_sum_lvars_done
.L_sum_lv_step:
    LDA     RA                  ; argCount
    STR     R2
    GLO     R4
    ADD
    PLO R4
    LDI 0
    STR R2
    GHI R4
    ADC
    PHI R4
    LDA     RA                  ; localCount
    STR     R2
    GLO     R4
    ADD
    PLO R4
    LDI 0
    STR R2
    GHI R4
    ADC
    PHI R4
    INC     RA
    INC RA
    INC RA
    INC RA
    INC     RA
    INC RA
    INC RA
    INC RA
    DEC     RB
    BR      .L_sum_lvars_loop
.L_sum_lvars_done:
    LOAD    RD, _local_vars_size
    GHI     R4
    STR RD
    INC RD
    GLO R4
    STR RD

    ; CodeBase = LocalVarsTable (_local_vars_addr) + localVarsSize (R4)
    LOAD    RD, _local_vars_addr
    LDA     RD
    PHI RF
    LDN RD
    PLO RF
    GLO     R4
    STR R2
    GLO RF
    ADD
    PLO RF
    GHI R4
    STR R2
    GHI RF
    ADC
    PHI RF
    LOAD    RD, _code_base_addr
    GHI     RF
    STR RD
    INC RD
    GLO RF
    STR RD

    ; GlobalsBase = CodeBase (RF) + codeSize (_code_size)
    LOAD    RD, _code_size
    LDA     RD
    PHI RA
    LDN RD
    PLO RA
    GLO     RA
    STR R2
    GLO RF
    ADD
    PLO RF
    GHI RA
    STR R2
    GHI RF
    ADC
    PHI RF
    GLO     RF
    PLO R9
    GHI     RF
    PHI R9
    LOAD    RD, _globals_base_addr
    GHI     R9
    STR RD
    INC RD
    GLO R9
    STR RD

    ; Zero out Globals data: _total_glob_bytes at R9
    LOAD    RD, _total_glob_bytes
    LDA     RD
    PHI RB
    LDN RD
    PLO RB
    GLO     R9
    PLO RF
    GHI     R9
    PHI RF
.L_zero_globals_loop:
    GLO     RB
    BNZ .L_zero_g_step
    GHI     RB
    BZ  .L_zero_globals_done
.L_zero_g_step:
    LDI     0
    STR RF
    INC RF
    DEC     RB
    BR      .L_zero_globals_loop
.L_zero_globals_done:

    ; Parse Entry Function: FuncTable + entryFuncIdx * 10
    LOAD    RD, _entry_func_idx
    LDA     RD
    PHI RB
    LDN RD
    PLO RB
    GLO     RB
    SHL
    PLO RA
    GHI RB
    SHLC
    PHI RA
    GLO     RA
    SHL
    PLO R4
    GHI RA
    SHLC
    PHI R4
    GLO     R4
    SHL
    PLO R4
    GHI R4
    SHLC
    PHI R4
    GLO     RA
    STR R2
    GLO R4
    ADD
    PLO R4
    GHI RA
    STR R2
    GHI R4
    ADC
    PHI R4
    LOAD    RD, _func_table_addr
    LDA     RD
    PHI RF
    LDN RD
    PLO RF
    GLO     R4
    STR R2
    GLO RF
    ADD
    PLO RF
    GHI R4
    STR R2
    GHI RF
    ADC
    PHI RF
    INC     RF
    INC RF
    LDA     RF
    PHI RB
    LDA RF
    PLO RB                           ; RB = frameSize
    LDA     RF
    PHI R5
    LDA RF
    PLO R5                           ; R5 = codeOffset
    INC     RF
    INC RF
    LDA     RF
    PHI R4
    LDA RF
    PLO R4                           ; R4 = varTableOff

    ; _var_table_ptr = LocalVarsTable (_local_vars_addr) + varTableOff (R4)
    LOAD    RD, _local_vars_addr
    LDA     RD
    PHI RA
    LDN RD
    PLO RA
    GLO     R4
    STR R2
    GLO RA
    ADD
    PLO RA
    GHI R4
    STR R2
    GHI RA
    ADC
    PHI RA
    LOAD    RD, _var_table_ptr
    GHI     RA
    STR RD
    INC RD
    GLO RA
    STR RD

    ; R5 = CodeBase (_code_base_addr) + codeOffset (R5)
    LOAD    RD, _code_base_addr
    LDA     RD
    PHI RA
    LDN RD
    PLO RA
    GLO     RA
    STR R2
    GLO R5
    ADD
    PLO R5
    GHI RA
    STR R2
    GHI R5
    ADC
    PHI R5

    ; Invariant Registers & Native Call Stack:
    LOAD    R1, $0008           ; IRQ vector
    LOAD    RE, $0004           ; Trap vector
    LOAD    RC, $0100           ; OpcodeTableLo
    LOAD    RD, $0200           ; OpcodeTableHi
    LOAD    R6, $B000           ; Par4 operand stack
    LOAD    R2, $DFFE           ; Native call stack
    SEX     R2

    ; Allocate frame on R2 for entry function:
    GLO     RB
    STR     R2
    GLO     R2
    SM
    PLO     R7
    GHI     RB
    STR     R2
    GHI     R2
    SMB
    PHI     R7

    ; Save caller_saved_sp ($DFFE) at Callee R7 - 2:
    DEC     R7
    LDI     $FE
    STR     R7
    DEC     R7
    LDI     $DF
    STR     R7
    INC     R7
    INC     R7

    ; Set Callee R2 = R7 - 3:
    GLO     R7
    SMI     3
    PLO     R2
    GHI     R7
    SMBI    0
    PHI     R2

    ; Enter dispatch!
    LOAD    R3, DispatchNext
    SEP     R3

_halt_bad_magic:
    LDI     '!'
    LOAD    RF, $FF00
    STR     RF
    LDI     1
    LOAD    RF, $FF05
    STR     RF
.L_halt_bad_loop:
    BR      .L_halt_bad_loop

    ORG     $1500
kernel_launch_r2_table: DS 32       ; 16 tasks * 2 bytes ($1500..$151F)
kernel_launch_r5_table: DS 32       ; 16 tasks * 2 bytes ($1520..$153F)
kernel_launch_r6_table: DS 32       ; 16 tasks * 2 bytes ($1540..$155F)
kernel_launch_r7_table: DS 32       ; 16 tasks * 2 bytes ($1560..$157F)

    ORG     $1FFF
    DB      0
