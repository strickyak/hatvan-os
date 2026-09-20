; Hatvan OS Kernel Startup & TRAP #0 Handler for Motorola 68000
    org $00000000

vectors:
    dc.l    $00080000       ; Vector 0: Initial SSP ($00080000 = 512 KB RAM top)
    dc.l    cstart          ; Vector 1: Initial PC (Reset entry)
    dc.l    trap_unhandled  ; Vector 2: Bus Error
    dc.l    trap_unhandled  ; Vector 3: Address Error
    dc.l    trap_unhandled  ; Vector 4: Illegal Instruction
    dc.l    trap_unhandled  ; Vector 5: Zero Divide
    dc.l    trap_unhandled  ; Vector 6: CHK
    dc.l    trap_unhandled  ; Vector 7: TRAPV
    dc.l    trap_unhandled  ; Vector 8: Privilege Violation
    dc.l    trap_unhandled  ; Vector 9: Trace
    dc.l    trap_unhandled  ; Vector 10: Line 1010
    dc.l    trap_unhandled  ; Vector 11: Line 1111
    dc.l    trap_unhandled  ; Vector 12..23: Reserved
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled  ; Vector 24: Spurious Interrupt
    dc.l    trap_unhandled  ; Vector 25: Level 1 Autovector
    dc.l    trap_unhandled  ; Vector 26: Level 2 Autovector
    dc.l    trap_unhandled  ; Vector 27: Level 3 Autovector
    dc.l    trap_unhandled  ; Vector 28: Level 4 Autovector (Terminal Rx)
    dc.l    trap_unhandled  ; Vector 29: Level 5 Autovector
    dc.l    trap_level6     ; Vector 30: Level 6 Autovector (Timer)
    dc.l    trap_unhandled  ; Vector 31: Level 7 Autovector
    dc.l    trap_0          ; Vector 32: TRAP #0 (Hatvan OS system call)
    ; Vectors 33..47: TRAP #1..#15
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled
    dc.l    trap_unhandled

    org $00001000

cstart:
    lea     $00080000, sp
    jsr     _main
hang:
    bra     hang

_printf:
    rts

_exit:
    move.l  4(sp), d0
    move.b  d0, $00FF000A
.exit_hang:
    bra     .exit_hang

trap_unhandled:
    rte

saved_kernel_sp_table_m68k:
    dc.l    0, 0, 0, 0, 0, 0, 0, 0
    dc.l    0, 0, 0, 0, 0, 0, 0, 0

saved_parent_pid_table_m68k:
    dc.b    0, 0, 0, 0, 0, 0, 0, 0
    dc.b    0, 0, 0, 0, 0, 0, 0, 0
    even

in_kernel_m68k:
    dc.b    1
    even

; Per-process user register context table for preemptive multitasking:
; 16 entries * 72 bytes = 1152 bytes
; Layout:
;   0: D0..D7 (32 bytes)
;  32: A0..A6 (28 bytes)
;  60: USP   (4 bytes)
;  64: SR    (2 bytes)
;  66: PC    (4 bytes)
;  70: pad   (2 bytes)
user_context_table_m68k:
    ds.b    1152

saved_kernel_sp_rbf_m68k:
    dc.l    0
saved_task1_sp_m68k:
    dc.l    0
saved_task1_pc_m68k:
    dc.l    0
saved_task1_sr_m68k:
    dc.w    0
    even

trap_0:
    move.b  #1, in_kernel_m68k  ; Enter kernel mode
    ; Check if trap came from Task 1 (RBF driver returning via F$Sleep)
    cmp.b   #1, v_proc.CurrentPID
    beq     .m68k_rbf_return

    ; On entry:
    ; Hardware pushed:
    ;   0(sp): SR (16-bit word)
    ;   2(sp): PC (32-bit long)
    ; CPU is in Supervisor mode (Task 0).
    move.l  d0, -(sp)
    move.l  d1, -(sp)
    move.l  d2, -(sp)
    move.l  d3, -(sp)
    move.l  d4, -(sp)
    move.l  d5, -(sp)
    move.l  d6, -(sp)
    move.l  d7, -(sp)
    move.l  a0, -(sp)
    move.l  a1, -(sp)
    move.l  a2, -(sp)
    move.l  a3, -(sp)
    move.l  a4, -(sp)
    move.l  a5, -(sp)
    move.l  a6, -(sp)

    ; Map user registers to v_syscall.UserFrame:
    ; User D0: callNum
    ; User D1: A (low byte = path ID or status)
    ; User A0: X (buffer/path pointer)
    ; User D2: Y (count)
    ; User A1: U (params)
    move.b  d1, v_syscall.UserFrame+1
    move.l  a0, v_syscall.UserFrame+4
    move.l  d2, v_syscall.UserFrame+8
    move.l  a1, v_syscall.UserFrame+12

    ; Check if process exited (F$Exit = 6)
    cmp.l   #6, d0
    beq     .m68k_process_exited

    ; MiniGolf Syscall Dispatch takes callNum in D0
    move.l  d0, -(sp)
    jsr     f_syscall__Dispatch
    lea     4(sp), sp

    ; Update return values in saved register frame on stack:
    ; Sp points to a6.
    ; 56(sp) is D0, 52(sp) is D1, 48(sp) is D2
    ; 60(sp) is SR (word), 62(sp) is PC (long)
    move.b  v_syscall.UserFrame+1, 55(sp) ; D1 = A (path ID)
    move.b  v_syscall.UserFrame+2, 59(sp) ; D0 = B (status)
    move.l  v_syscall.UserFrame+8, 48(sp) ; D2 = Y (count)

    ; Update Carry bit in saved SR:
    move.b  v_syscall.UserFrame, d0
    and.l   #1, d0
    beq     .no_carry
    move.b  v_syscall.UserFrame+2, 55(sp) ; D1 = B (error code)
    move.b  v_syscall.UserFrame+2, 59(sp) ; D0 = B (error code)
    or.w    #1, 60(sp)
    bra     .done_cc
.no_carry:
    and.w   #$FFFE, 60(sp)
.done_cc:

    ; Restore user registers and return to user task
    move.l  (sp)+, a6
    move.l  (sp)+, a5
    move.l  (sp)+, a4
    move.l  (sp)+, a3
    move.l  (sp)+, a2
    move.l  (sp)+, a1
    move.l  (sp)+, a0
    move.l  (sp)+, d7
    move.l  (sp)+, d6
    move.l  (sp)+, d5
    move.l  (sp)+, d4
    move.l  (sp)+, d3
    move.l  (sp)+, d2
    move.l  (sp)+, d1
    move.l  (sp)+, d0
    clr.b   in_kernel_m68k      ; Returning to user mode
    rte

.m68k_process_exited:
    ; Process exited: call SysExit(status = d1.B)
    move.l  d1, -(sp)
    jsr     f_proc__SysExit
    lea     4(sp), sp

    ; Child PID is currently in v_proc.CurrentPID
    moveq   #0, d0
    move.b  v_proc.CurrentPID, d0

    ; Restore parent PID from saved_parent_pid_table_m68k[childPID]
    lea     saved_parent_pid_table_m68k, a0
    move.b  0(a0, d0.w), v_proc.CurrentPID
    move.b  v_proc.CurrentPID, $00FF0020

    ; Restore kernel stack and return to LaunchProcess caller
    lea     saved_kernel_sp_table_m68k, a0
    lsl.l   #2, d0
    move.l  0(a0, d0.w), sp
    movem.l (sp)+, d2-d7/a2-a6
    rts

.m68k_rbf_return:
    ; Task 1 returned via trap #0.
    ; SP points to SR (word) and PC (long) pushed by trap #0.
    ; Save Task 1 USP
    move.l  usp, a0
    move.l  a0, saved_task1_sp_m68k

    ; Pop SR and PC into saved variables
    move.w  (sp)+, saved_task1_sr_m68k
    move.l  (sp)+, saved_task1_pc_m68k

    ; Restore kernel stack pointer
    move.l  saved_kernel_sp_rbf_m68k, sp

    ; Restore caller PID from kernel stack
    move.b  (sp), v_proc.CurrentPID
    addq.l  #2, sp

    ; Restore TaskReg to caller PID
    move.b  v_proc.CurrentPID, $00FF0020

    ; Restore Task 0 callee-saved registers
    move.l  (sp)+, a6
    move.l  (sp)+, a5
    move.l  (sp)+, a4
    move.l  (sp)+, a3
    move.l  (sp)+, a2
    move.l  (sp)+, d7
    move.l  (sp)+, d6
    move.l  (sp)+, d5
    move.l  (sp)+, d4
    move.l  (sp)+, d3
    move.l  (sp)+, d2

    ; Return to RBFCall caller in Task 0!
    rts

f_hal__RBFCall:
    ; Save Task 0 callee-saved registers
    move.l  d2, -(sp)
    move.l  d3, -(sp)
    move.l  d4, -(sp)
    move.l  d5, -(sp)
    move.l  d6, -(sp)
    move.l  d7, -(sp)
    move.l  a2, -(sp)
    move.l  a3, -(sp)
    move.l  a4, -(sp)
    move.l  a5, -(sp)
    move.l  a6, -(sp)

    ; Push caller PID onto kernel stack (word-aligned)
    subq.l  #2, sp
    move.b  v_proc.CurrentPID, (sp)

    ; Set CurrentPID = 1 (Task 1: RBF)
    move.b  #1, v_proc.CurrentPID

    ; Save kernel stack pointer
    move.l  sp, saved_kernel_sp_rbf_m68k

    ; Set TaskReg to Task 1 ($00FF0020)
    move.b  #1, $00FF0020

    ; Set USP to Task 1 stack pointer
    move.l  saved_task1_sp_m68k, a0
    move.l  a0, usp

    ; Push Task 1 PC and SR for RTE
    move.l  saved_task1_pc_m68k, -(sp)
    move.w  saved_task1_sr_m68k, -(sp)
    rte

f_hal__InitRBF:
    ; 4(sp) is entryPC
    move.l  4(sp), saved_task1_pc_m68k
    move.l  #$0007FE00, saved_task1_sp_m68k
    move.w  #$0000, saved_task1_sr_m68k

    ; Call RBF once so it runs its init and enters hal.Sleep()
    jsr     f_hal__RBFCall
    rts

f_hal__Sleep:
    move.l  #10, d0
    trap    #0
    rts

f_hal__InitUserContext:
    ; 4(sp) = PID
    ; 8(sp) = SP (paramAddr)
    ; 12(sp) = PC (entryPC)
    move.l  4(sp), d0
    mulu    #72, d0
    lea     user_context_table_m68k, a0
    add.l   d0, a0

    moveq   #0, d1
    move.l  d1, 0(a0)
    move.l  d1, 4(a0)
    move.l  d1, 8(a0)
    move.l  d1, 12(a0)
    move.l  d1, 16(a0)
    move.l  d1, 20(a0)
    move.l  d1, 24(a0)
    move.l  d1, 28(a0)

    move.l  8(sp), 32(a0)       ; A0 = paramAddr
    move.l  d1, 36(a0)
    move.l  d1, 40(a0)
    move.l  d1, 44(a0)
    move.l  d1, 48(a0)
    move.l  d1, 52(a0)
    move.l  d1, 56(a0)

    move.l  8(sp), d2
    sub.l   #128, d2
    and.l   #$FFFFFFFE, d2
    move.l  d2, 60(a0)          ; USP
    move.w  #$0000, 64(a0)      ; SR
    move.l  12(sp), 66(a0)      ; PC
    rts

f_hal__FreeUserContext:
    ; 4(sp) = PID
    move.l  4(sp), d0
    mulu    #72, d0
    lea     user_context_table_m68k, a0
    clr.l   66(a0, d0.l)
    lea     saved_kernel_sp_table_m68k, a1
    move.l  4(sp), d0
    lsl.l   #2, d0
    clr.l   0(a1, d0.l)
    rts

f_hal__LaunchProcess:
    ; MiniGolf caller pushed:
    ;   0(sp): return PC (4 bytes)
    ;   4(sp): PID (4 bytes)
    ;   8(sp): SP (4 bytes)
    ;   12(sp): PC (4 bytes)
    move.l  4(sp), d0           ; d0 = PID
    move.l  8(sp), a0           ; a0 = paramAddr
    move.l  12(sp), d1          ; d1 = initial PC

    ; Save parent's callee-saved registers on kernel stack
    movem.l d2-d7/a2-a6, -(sp)

    ; Save parent PID in saved_parent_pid_table_m68k[childPID]
    lea     saved_parent_pid_table_m68k, a1
    move.b  v_proc.CurrentPID, 0(a1, d0.w)

    ; Save kernel SP in saved_kernel_sp_table_m68k[childPID]
    lea     saved_kernel_sp_table_m68k, a1
    move.l  d0, d2
    lsl.l   #2, d2
    move.l  sp, 0(a1, d2.w)

    move.b  d0, v_proc.CurrentPID
    move.b  d0, $00FF0020       ; Set TaskReg to user PID

    ; Check if user_context_table_m68k[d0] already has a valid PC
    moveq   #0, d2
    move.b  d0, d2
    mulu    #72, d2
    lea     user_context_table_m68k, a1
    add.l   d2, a1

    tst.l   66(a1)
    bne     .launch_resume_existing

    ; Fresh launch:
    move.l  a0, 32(a1)          ; A0 = paramAddr
    move.l  a0, d2
    sub.l   #128, d2
    and.l   #$FFFFFFFE, d2
    move.l  d2, 60(a1)          ; USP
    move.w  #$0000, 64(a1)      ; SR
    move.l  d1, 66(a1)          ; PC

.launch_resume_existing:
    move.l  60(a1), a2
    move.l  a2, usp

    move.l  66(a1), -(sp)       ; PC
    move.w  64(a1), -(sp)       ; SR

    movem.l 0(a1), d0-d7/a0-a6

    clr.b   in_kernel_m68k      ; Entering user mode
    rte

trap_level6:
    ; Acknowledge Level 6 timer interrupt
    move.w  #1, $00FF0004

    ; Save all registers onto supervisor stack
    movem.l d0-d7/a0-a6, -(sp)

    ; Check if we were in kernel mode
    tst.b   in_kernel_m68k
    bne     .l6_fast_return

    ; Check S-bit in saved SR: SR is at 60(sp)
    btst    #5, 60(sp)
    bne     .l6_fast_return

    ; Check CurrentPID: if <= 1 (kernel or RBF), do not preempt
    cmp.b   #1, v_proc.CurrentPID
    bls     .l6_fast_return

    ; Preempt user process CurrentPID!
    moveq   #0, d0
    move.b  v_proc.CurrentPID, d0
    mulu    #72, d0
    lea     user_context_table_m68k, a0
    add.l   d0, a0

    ; Copy registers from stack to user_context_table_m68k[CurrentPID]
    move.l  0(sp), 0(a0)        ; d0
    move.l  4(sp), 4(a0)        ; d1
    move.l  8(sp), 8(a0)        ; d2
    move.l  12(sp), 12(a0)      ; d3
    move.l  16(sp), 16(a0)      ; d4
    move.l  20(sp), 20(a0)      ; d5
    move.l  24(sp), 24(a0)      ; d6
    move.l  28(sp), 28(a0)      ; d7
    move.l  32(sp), 32(a0)      ; a0
    move.l  36(sp), 36(a0)      ; a1
    move.l  40(sp), 40(a0)      ; a2
    move.l  44(sp), 44(a0)      ; a3
    move.l  48(sp), 48(a0)      ; a4
    move.l  52(sp), 52(a0)      ; a5
    move.l  56(sp), 56(a0)      ; a6

    move.l  usp, a1
    move.l  a1, 60(a0)          ; USP
    move.w  60(sp), 64(a0)      ; SR
    move.l  62(sp), 66(a0)      ; PC

    ; Pop saved frame (66 bytes) from supervisor stack
    lea     66(sp), sp

    ; Enter kernel mode for scheduler
    move.b  #1, in_kernel_m68k
    jsr     f_proc__ScheduleNext
    ; Next PID in D0.B!

    ; Check if next process is different
    cmp.b   v_proc.CurrentPID, d0
    beq     .l6_resume_same

    ; If switching to nextPID (in D0): check saved_kernel_sp_table_m68k[nextPID]
    moveq   #0, d1
    move.b  d0, d1
    lsl.l   #2, d1
    lea     saved_kernel_sp_table_m68k, a0
    tst.l   0(a0, d1.l)
    bne     .l6_ksp_ok

    ; Inherit kernel SP and parent PID from oldPID
    moveq   #0, d2
    move.b  v_proc.CurrentPID, d2
    lsl.l   #2, d2
    move.l  0(a0, d2.l), 0(a0, d1.l)

    move.b  v_proc.CurrentPID, d2
    lea     saved_parent_pid_table_m68k, a1
    move.b  0(a1, d2.w), d3
    move.b  d0, d2
    move.b  d3, 0(a1, d2.w)

.l6_ksp_ok:
    move.b  d0, v_proc.CurrentPID
    move.b  d0, $00FF0020       ; Set TaskReg to nextPID

.l6_resume_same:
    ; Restore context of CurrentPID
    moveq   #0, d0
    move.b  v_proc.CurrentPID, d0
    mulu    #72, d0
    lea     user_context_table_m68k, a0
    add.l   d0, a0

    move.l  60(a0), a1
    move.l  a1, usp

    ; Push PC and SR onto supervisor stack for RTE
    move.l  66(a0), -(sp)       ; PC
    move.w  64(a0), -(sp)       ; SR

    movem.l 0(a0), d0-d7/a0-a6

    clr.b   in_kernel_m68k
    rte

.l6_fast_return:
    movem.l (sp)+, d0-d7/a0-a6
    rte
