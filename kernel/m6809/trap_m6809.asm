; Hatvan OS M6809 Trap Handler & Vector Table
; Intercepts SWI2 traps from user tasks, extracts register frame,
; decodes inline syscall opcode, and dispatches to MiniGolf syscall handler.

    pragma cescapes

user_sp_table:
    fdb 0,0,0,0,0,0,0,0

saved_kernel_sp_table:
    fdb 0,0,0,0,0,0,0,0

saved_parent_pid_table:
    fcb 0,0,0,0,0,0,0,0

saved_user_frame_table:
    fill 0,96

call_num:
    fcb 0

; SWI2 Trap Entry Point (Vector at $FFF4)
; Hardware pushed (PC, U, Y, X, DP, B, A, CC) onto user task stack.
; CPU switched to Task 0 during vector fetch. Register S is user SP.
trap_swi2:
    ; 0. Reset Direct Page to page 0 for kernel execution
    clra
    tfr a,dp

    ; 1. Save user stack pointer in user_sp_table[CurrentPID]
    tfr s,x
    ldb v_proc.CurrentPID
    lslb
    ldy #user_sp_table
    stx b,y

    ; 2. Switch to kernel stack
    ldy #saved_kernel_sp_table
    lds b,y
    lsrb

    ; 3. Copy 12-byte user frame from user task RAM into v_syscall.UserFrame
    ; DMA: srcTask = CurrentPID, srcAddr = user_sp_table[CurrentPID], dstTask = 0, dstAddr = UserFrame, count = 12
    ldb v_proc.CurrentPID
    stb $FF21
    lslb
    ldy #user_sp_table
    ldd b,y
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
    ldd >v_syscall.UserFrame+10
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
    ldd >v_syscall.UserFrame+10
    addd #1
    std >v_syscall.UserFrame+10

    ; 5. Dispatch system call in MiniGolf
    ldb call_num
    pshs b              ; preserve call_num for this frame across nested calls
    jsr f_syscall__Dispatch
    puls b              ; restore call_num for this frame

    ; Check if process exited (F$Exit called SysExit, which freed paths and set state)
    cmpb #$06           ; F$Exit
    beq .process_exited

    ; 6. Copy updated UserFrame (12 bytes) back to user task stack
    clr $FF21           ; srcTask = 0
    ldd #v_syscall.UserFrame
    std $FF22
    ldb v_proc.CurrentPID
    stb $FF24
    lslb
    ldy #user_sp_table
    ldd b,y
    std $FF25
    ldb #12
    stb $FF27
.wait_dma3:
    ldb $FF27
    beq .wait_dma3

    ; 7. Restore user stack pointer and arm TaskFuse
    ldb v_proc.CurrentPID
    lslb
    ldy #user_sp_table
    lds b,y
    lsrb
    stb $FF20           ; arm TaskFuse with target PID
    rti

.process_exited:
    ; Process exited: restore parent PID and resume caller in Task 0
    ldb v_proc.CurrentPID
    ldx #saved_parent_pid_table
    lda b,x
    sta v_proc.CurrentPID

    ; Restore parent's UserFrame from saved_user_frame_table
    ldb #12
    mul                 ; D = parentPID * 12
    ldx #saved_user_frame_table
    leax d,x
    ldy #v_syscall.UserFrame
    ldd ,x++
    std ,y++
    ldd ,x++
    std ,y++
    ldd ,x++
    std ,y++
    ldd ,x++
    std ,y++
    ldd ,x++
    std ,y++
    ldd ,x++
    std ,y++

    rts

; launch_user_process launches user task PID for execution
; Stack frame on entry from MiniGolf:
;   0,s: return PC (2 bytes)
;   2,s: PID (1 byte)
;   3,s: initial SP (2 bytes)
launch_process:
f_hal__LaunchProcess:
    ; Save parent's UserFrame into saved_user_frame_table
    lda v_proc.CurrentPID
    ldb #12
    mul                 ; D = parentPID * 12
    ldx #saved_user_frame_table
    leax d,x
    ldy #v_syscall.UserFrame
    ldd ,y++
    std ,x++
    ldd ,y++
    std ,x++
    ldd ,y++
    std ,x++
    ldd ,y++
    std ,x++
    ldd ,y++
    std ,x++
    ldd ,y++
    std ,x++

    ldb 2,s             ; B = PID
    ldx 3,s             ; X = initial SP
    lda v_proc.CurrentPID

    ldy #saved_parent_pid_table
    sta b,y

    lslb
    ldy #saved_kernel_sp_table
    sts b,y
    lsrb

    stb v_proc.CurrentPID
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
