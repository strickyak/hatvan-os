; Hatvan OS M6809 Trap Handler & Vector Table
; Intercepts SWI2 traps from user tasks, extracts register frame,
; decodes inline syscall opcode, and dispatches to MiniGolf syscall handler.

    pragma cescapes

user_sp_table:
    fill 0,32

saved_kernel_sp_table:
    fill 0,32

saved_parent_pid_table:
    fill 0,16

saved_user_frame_table:
    fill 0,192

call_num:
    fcb 0

rbf_temp_pc:
    fdb 0

rbf_init_frame:
    fcb $D0             ; CC: all flags set, interrupts masked
    fcb 0               ; A
    fcb 0               ; B
    fcb 0               ; DP
    fdb 0               ; X
    fdb 0               ; Y
    fdb 0               ; U
    fdb $4000           ; PC

; SWI2 Trap Entry Point (Vector at $FFF4)
; Hardware pushed (PC, U, Y, X, DP, B, A, CC) onto user task stack.
; CPU switched to Task 0 during vector fetch. Register S is user SP.
trap_swi2:
    ; 0. Reset Direct Page to page 0 for kernel execution
    clra
    tfr a,dp

    ; Check if trap came from Task 1 (RBF driver returning via F$Sleep)
    ldb v_proc.CurrentPID
    cmpb #1
    lbeq .handle_rbf_return

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

.handle_rbf_return:
    ; Hardware pushed 12-byte RTI frame on Task 1 stack.
    ; Register S is Task 1 SP. PC on Task 1 stack is at S + 10.
    ; Advance PC past the $0A inline opcode byte:
    lda #1
    sta $FF21           ; srcTask = 1
    tfr s,d
    addd #10
    std $FF22           ; srcAddr = S + 10
    clr $FF24           ; dstTask = 0
    ldd #rbf_temp_pc
    std $FF25           ; dstAddr = rbf_temp_pc
    ldb #2
    stb $FF27           ; DMA read 2 bytes
.rbf_ret_dma1:
    ldb $FF27
    beq .rbf_ret_dma1

    ldd rbf_temp_pc
    addd #1
    std rbf_temp_pc

    clr $FF21           ; srcTask = 0
    ldd #rbf_temp_pc
    std $FF22           ; srcAddr = rbf_temp_pc
    lda #1
    sta $FF24           ; dstTask = 1
    tfr s,d
    addd #10
    std $FF25           ; dstAddr = S + 10
    ldb #2
    stb $FF27           ; DMA write 2 bytes
.rbf_ret_dma2:
    ldb $FF27
    beq .rbf_ret_dma2

    ; Save Task 1 SP in user_sp_table[1]
    tfr s,x
    ldy #user_sp_table
    stx 2,y

    ; Restore kernel stack pointer from saved_kernel_sp_table[1]
    ldy #saved_kernel_sp_table
    lds 2,y

    ; Restore caller PID from kernel stack
    puls a
    sta v_proc.CurrentPID

    ; Return to RBFCall caller in Task 0!
    rts

f_hal__RBFCall:
    ; Push caller PID onto kernel stack
    lda v_proc.CurrentPID
    pshs a

    ; Set CurrentPID = 1 (Task 1: RBF)
    lda #1
    sta v_proc.CurrentPID

    ; Save kernel stack pointer in saved_kernel_sp_table[1]
    ldy #saved_kernel_sp_table
    sts 2,y

    ; Load Task 1 stack pointer from user_sp_table[1]
    ldy #user_sp_table
    lds 2,y

    ; Arm TaskFuse with target task 1
    lda #1
    sta $FF20

    ; RTI switches to Task 1!
    rti

f_hal__InitRBF:
    ; Stack on entry from MiniGolf:
    ;   0,s: return PC (2 bytes)
    ;   2,s: entryPC (2 bytes)
    ldd 2,s
    std rbf_init_frame+10

    ; DMA initial 12-byte RTI frame to Task 1 at $FDF4
    clr $FF21
    ldd #rbf_init_frame
    std $FF22
    lda #1
    sta $FF24
    ldd #$FDF4
    std $FF25
    ldb #12
    stb $FF27
.rbf_init_dma:
    ldb $FF27
    beq .rbf_init_dma

    ; user_sp_table[1] = $FDF4
    ldd #$FDF4
    ldy #user_sp_table
    std 2,y

    ; Call RBF once to let it initialize and enter hal.Sleep()
    jsr f_hal__RBFCall
    rts

f_hal__Sleep:
    swi2
    fcb $0A
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
