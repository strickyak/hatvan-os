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
    dc.l    trap_unhandled  ; Vector 30: Level 6 Autovector (Timer)
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

saved_kernel_sp_m68k:
    dc.l    0

saved_parent_pid_m68k:
    dc.b    0
    even

trap_0:
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
    move.b  v_syscall.UserFrame+1, 59(sp) ; D0 = A (return value)
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
    rte

.m68k_process_exited:
    ; Process exited: call SysExit(status = d1.B)
    move.l  d1, -(sp)
    jsr     f_proc__SysExit
    lea     4(sp), sp

    ; Restore kernel stack and parent PID, return to LaunchProcess caller
    move.l  saved_kernel_sp_m68k, sp
    move.b  saved_parent_pid_m68k, v_proc.CurrentPID
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
    move.b  v_proc.CurrentPID, saved_parent_pid_m68k
    move.b  d0, v_proc.CurrentPID
    move.l  sp, saved_kernel_sp_m68k

    ; Reserve stack space below parameter string for USP:
    move.l  a0, a1
    sub.l   #128, a1
    and.l   #$FFFFFFFE, a1
    ; move.l a1, usp ($4E61)
    dc.w    $4E61
    move.b  d0, $00FF0020       ; Set TaskReg to user PID

    ; Enter user mode via RTE (A0 still points to paramAddr):
    move.l  d1, -(sp)           ; Push initial user PC
    move.w  #$0000, -(sp)       ; Push initial user SR (User mode, interrupts enabled)
    rte
