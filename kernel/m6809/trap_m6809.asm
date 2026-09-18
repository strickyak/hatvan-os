; Hatvan OS M6809 Trap Handler & Vector Table
; Intercepts SWI2 traps from user tasks, extracts register frame,
; decodes inline syscall opcode, and dispatches to MiniGolf syscall handler.

    pragma cescapes

user_sp:
    fdb 0

call_num:
    fcb 0

saved_kernel_sp:
    fdb 0

saved_parent_pid:
    fcb 0

; SWI2 Trap Entry Point (Vector at $FFF4)
; Hardware pushed (PC, U, Y, X, DP, B, A, CC) onto user task stack.
; CPU switched to Task 0 during vector fetch. Register S is user SP.
trap_swi2:
    ; 1. Save user stack pointer
    tfr s,x
    stx user_sp

    ; 2. Switch to kernel stack
    lds saved_kernel_sp

    ; 3. Copy 12-byte user frame from user task RAM into v_syscall.UserFrame
    ; DMA: srcTask = CurrentPID, srcAddr = user_sp, dstTask = 0, dstAddr = UserFrame, count = 12
    ldb v_proc.CurrentPID
    stb $FF21
    ldd user_sp
    std $FF22
    clr $FF24           ; dstTask = 0
    ldd #v_syscall.UserFrame
    std $FF25
    ldb #12             ; 12 bytes
    stb $FF27
.wait_dma1:
    ldb $FF27
    beq .wait_dma1

    ; 4. Fetch inline syscall opcode byte from user task RAM at user PC
    ; User PC is at UserFrame + 10 (big-endian word)
    ldb v_proc.CurrentPID
    stb $FF21
    ldd v_syscall.UserFrame+10
    std $FF22
    clr $FF24
    ldd #call_num
    std $FF25
    ldb #1
    stb $FF27
.wait_dma2:
    ldb $FF27
    beq .wait_dma2

    ; Increment user PC past the inline call_num byte
    ldd v_syscall.UserFrame+10
    addd #1
    std v_syscall.UserFrame+10

    ; 5. Dispatch system call in MiniGolf
    ldb call_num
    jsr f_syscall__Dispatch

    ; Check if process exited (F$Exit called SysExit, which freed paths and set state)
    ldb call_num
    cmpb #$06           ; F$Exit
    beq .process_exited

    ; 6. Copy updated UserFrame (12 bytes) back to user task stack
    clr $FF21           ; srcTask = 0
    ldd #v_syscall.UserFrame
    std $FF22
    ldb v_proc.CurrentPID
    stb $FF24
    ldd user_sp
    std $FF25
    ldb #12
    stb $FF27
.wait_dma3:
    ldb $FF27
    beq .wait_dma3

    ; 7. Restore user stack pointer and arm TaskFuse
    lds user_sp
    ldb v_proc.CurrentPID
    stb $FF20           ; arm TaskFuse with target PID
    rti

.process_exited:
    ; Process exited: restore parent PID and resume caller in Task 0
    lda saved_parent_pid
    sta v_proc.CurrentPID
    rts

; launch_user_process launches user task PID for execution
; Stack frame on entry from MiniGolf:
;   0,s: return PC (2 bytes)
;   2,s: PID (1 byte)
;   3,s: initial SP (2 bytes)
launch_process:
f_hal__LaunchProcess:
    ldb 2,s             ; B = PID
    ldx 3,s             ; X = initial SP
    lda v_proc.CurrentPID
    sta saved_parent_pid
    stb v_proc.CurrentPID
    ; Save current kernel stack pointer for return when process exits
    sts saved_kernel_sp
    tfr x,s             ; S = user SP
    stb $FF20           ; arm TaskFuse with user PID
    rti                 ; RTI switches to user task and launches code!

; Trap stubs for unhandled vectors
trap_swi3:
trap_swi:
trap_nmi:
trap_firq:
trap_irq:
trap_reserved:
    rti
