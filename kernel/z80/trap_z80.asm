; Hatvan OS Z80 Trap Handler & Context Switcher
; Handles:
;   - NMI System Call Trap ($0066) via Port 60h
;   - Timer / Terminal IM 1 Interrupts ($0038)
;   - Task 1 (RBF) & Task 2 (PROCFS) sleep/wakeup dispatch
;   - Process launch & exit lifecycle

user_sp_table:
    defs 32, 0

saved_kernel_sp_table:
    defs 32, 0

saved_parent_pid_table:
    defs 16, 0

launched_from_kernel:
    defs 16, 0

saved_user_frame_table:
    defs 192, 0

in_kernel:
    defb 1

irq_stack:
    defs 256, 0
irq_stack_top:

call_num:
    defb 0

user_return_pc:
    defw 0

temp_sp:
    defw 0
temp_hl:
    defw 0
temp_de:
    defw 0
temp_bc:
    defw 0
temp_ix:
    defw 0
temp_iy:
    defw 0
temp_af:
    defw 0

; ── NMI System Call Trap Handler (Vector at $0066) ────────────────────────────
trap_nmi:
    ; 1. Immediately save CPU registers into kernel temporaries
    ld   (temp_sp), sp
    ld   (temp_hl), hl
    ld   (temp_de), de
    ld   (temp_bc), bc
    ld   (temp_ix), ix
    ld   (temp_iy), iy
    ex   af, af'          ; user AF now in AF'

    ; 2. Check if trap came from driver tasks (Task 1: RBF, Task 2: PROCFS) returning via hal.Sleep()
    ld   a, (v_proc.CurrentPID)
    cp   1
    jp   z, .handle_driver_return
    cp   2
    jp   z, .handle_driver_return

    ; User process trap (PID >= 3):
    ; Save user SP in user_sp_table[CurrentPID]
    ld   a, (v_proc.CurrentPID)
    add  a, a
    ld   e, a
    ld   d, 0
    ld   hl, user_sp_table
    add  hl, de
    ld   bc, (temp_sp)
    ld   (hl), c
    inc  hl
    ld   (hl), b

    ; Switch to kernel stack for CurrentPID
    ld   hl, saved_kernel_sp_table
    add  hl, de
    ld   e, (hl)
    inc  hl
    ld   d, (hl)
    ld   a, d
    or   e
    jr   nz, .ksp_ok
    ld   de, 0xE000
.ksp_ok:
    ex   de, hl
    ld   sp, hl

    ; Enter kernel mode
    ld   a, 1
    ld   (in_kernel), a

    ; 3. Fetch user return PC from user task memory at temp_sp via DMA:
    ld   a, (v_proc.CurrentPID)
    ld   (0xFF21), a                ; SrcTask
    ld   hl, (temp_sp)
    ld   a, h
    ld   (0xFF22), a
    ld   a, l
    ld   (0xFF23), a                ; SrcAddr (Hi, Lo)
    xor  a
    ld   (0xFF24), a                ; DstTask = 0
    ld   hl, user_return_pc
    ld   a, h
    ld   (0xFF25), a
    ld   a, l
    ld   (0xFF26), a                ; DstAddr (Hi, Lo)
    ld   a, 2
    ld   (0xFF27), a                ; Count = 2
.wait_dma_pc1:
    ld   a, (0xFF27)
    or   a
    jr   z, .wait_dma_pc1

    ; 4. Fetch inline syscall opcode byte from user task RAM at user_return_pc
    ld   a, (v_proc.CurrentPID)
    ld   (0xFF21), a
    ld   hl, (user_return_pc)
    ld   a, h
    ld   (0xFF22), a
    ld   a, l
    ld   (0xFF23), a
    xor  a
    ld   (0xFF24), a
    ld   hl, call_num
    ld   a, h
    ld   (0xFF25), a
    ld   a, l
    ld   (0xFF26), a
    ld   a, 1
    ld   (0xFF27), a
.wait_dma_op:
    ld   a, (0xFF27)
    or   a
    jr   z, .wait_dma_op

    ; 5. Advance user return PC past the inline byte: user_return_pc += 1
    ld   hl, (user_return_pc)
    inc  hl
    ld   (user_return_pc), hl

    ; Write updated return PC back to user task stack at temp_sp:
    xor  a
    ld   (0xFF21), a
    ld   hl, user_return_pc
    ld   a, h
    ld   (0xFF22), a
    ld   a, l
    ld   (0xFF23), a
    ld   a, (v_proc.CurrentPID)
    ld   (0xFF24), a
    ld   hl, (temp_sp)
    ld   a, h
    ld   (0xFF25), a
    ld   a, l
    ld   (0xFF26), a
    ld   a, 2
    ld   (0xFF27), a
.wait_dma_pc2:
    ld   a, (0xFF27)
    or   a
    jr   z, .wait_dma_pc2

    ; 6. Populate v_syscall.UserFrame:
    ex   af, af'                     ; main AF is now user AF
    ld   (v_syscall.UserFrame + 1), a
    push af
    pop  hl                          ; L is F
    ld   a, l
    ld   (v_syscall.UserFrame + 0), a

    ld   a, (temp_bc + 1)            ; B register
    ld   (v_syscall.UserFrame + 2), a
    xor  a
    ld   (v_syscall.UserFrame + 3), a

    ld   hl, (temp_hl)
    ld   (v_syscall.UserFrame + 4), hl

    ld   hl, (temp_bc)
    ld   (v_syscall.UserFrame + 6), hl

    ld   hl, (temp_ix)
    ld   (v_syscall.UserFrame + 8), hl

    ld   hl, (user_return_pc)
    ld   (v_syscall.UserFrame + 10), hl

    ; 7. Dispatch syscall in MiniGolf: f_syscall__Dispatch(callNum)
    ld   a, (call_num)
    ld   l, a
    ld   h, 0
    push hl
    call f_syscall__Dispatch
    pop  bc

    ; Check if process exited: call_num == 0x06 (F$Exit)
    ld   a, (call_num)
    cp   0x06
    jp   z, .process_exited

    ; 8. Return to user space:
    ; Load user SP for CurrentPID
    ld   a, (v_proc.CurrentPID)
    add  a, a
    ld   e, a
    ld   d, 0
    ld   hl, user_sp_table
    add  hl, de
    ld   e, (hl)
    inc  hl
    ld   d, (hl)
    ex   de, hl
    ld   sp, hl

    ; Arm TaskFuse with CurrentPID
    ld   a, (v_proc.CurrentPID)
    ld   (0xFF20), a

    ; Leaving kernel mode
    xor  a
    ld   (in_kernel), a

    ; Restore registers from UserFrame:
    ld   bc, (v_syscall.UserFrame + 6) ; Y -> BC
    ld   a, (v_syscall.UserFrame + 2)  ; UserFrame.B
    ld   b, a
    ld   de, (temp_de)                 ; DE preserved
    ld   hl, (v_syscall.UserFrame + 4) ; X -> HL
    ld   ix, (v_syscall.UserFrame + 8) ; U -> IX
    ld   iy, (temp_iy)

    ; Restore Carry and A:
    ld   a, (v_syscall.UserFrame + 0)  ; CC
    rra                                ; Carry = CC.0
    ld   a, (v_syscall.UserFrame + 1)  ; A

    ; RETN returns to user task!
    retn

.handle_driver_return:
    ; Driver called hal.Sleep() (out 60h, a; defb 0x0A).
    ; Hardware pushed return PC on driver task stack at temp_sp.
    ; 1. Fetch return PC via DMA
    ld   a, (v_proc.CurrentPID)
    ld   (0xFF21), a
    ld   hl, (temp_sp)
    ld   a, h
    ld   (0xFF22), a
    ld   a, l
    ld   (0xFF23), a
    xor  a
    ld   (0xFF24), a
    ld   hl, user_return_pc
    ld   a, h
    ld   (0xFF25), a
    ld   a, l
    ld   (0xFF26), a
    ld   a, 2
    ld   (0xFF27), a
.wait_drv_dma1:
    ld   a, (0xFF27)
    or   a
    jr   z, .wait_drv_dma1

    ; 2. Advance PC past 0x0A: user_return_pc += 1
    ld   hl, (user_return_pc)
    inc  hl
    ld   (user_return_pc), hl

    ; 3. Write updated PC back to driver stack at temp_sp
    xor  a
    ld   (0xFF21), a
    ld   hl, user_return_pc
    ld   a, h
    ld   (0xFF22), a
    ld   a, l
    ld   (0xFF23), a
    ld   a, (v_proc.CurrentPID)
    ld   (0xFF24), a
    ld   hl, (temp_sp)
    ld   a, h
    ld   (0xFF25), a
    ld   a, l
    ld   (0xFF26), a
    ld   a, 2
    ld   (0xFF27), a
.wait_drv_dma2:
    ld   a, (0xFF27)
    or   a
    jr   z, .wait_drv_dma2

    ; 4. Save driver SP into user_sp_table[CurrentPID]
    ld   a, (v_proc.CurrentPID)
    add  a, a
    ld   e, a
    ld   d, 0
    ld   hl, user_sp_table
    add  hl, de
    ld   bc, (temp_sp)
    ld   (hl), c
    inc  hl
    ld   (hl), b

    ; 5. Restore kernel SP from saved_kernel_sp_table[CurrentPID]
    ld   hl, saved_kernel_sp_table
    add  hl, de
    ld   e, (hl)
    inc  hl
    ld   d, (hl)
    ex   de, hl
    ld   sp, hl

    ; 6. Restore caller PID and caller IX from kernel stack
    pop  hl
    ld   a, l
    ld   (v_proc.CurrentPID), a
    pop  ix

    ; 7. Return to DriverCall caller in Task 0!
    ret

.process_exited:
    ld   a, (v_proc.CurrentPID)
    ld   c, a
    ld   b, 0
    ld   hl, launched_from_kernel
    add  hl, bc
    ld   a, (hl)
    or   a
    jp   z, .no_caller_waiting

    ; Clear launched_from_kernel[CurrentPID]
    xor  a
    ld   (hl), a

    ; Restore parent PID
    ld   hl, saved_parent_pid_table
    add  hl, bc
    ld   a, (hl)
    ld   (v_proc.CurrentPID), a

    ; Restore parent's UserFrame from saved_user_frame_table[parentPID * 12]
    ld   l, a
    ld   h, 0
    add  hl, hl                      ; * 2
    add  hl, hl                      ; * 4
    ld   c, l
    ld   b, h
    add  hl, hl                      ; * 8
    add  hl, bc                      ; * 12
    ld   de, saved_user_frame_table
    add  hl, de                      ; HL = source
    ld   de, v_syscall.UserFrame     ; DE = destination
    ld   bc, 12
    ldir

    ; Return to caller of LaunchProcess!
    pop  ix
    ret

.no_caller_waiting:
    ; Background process exited: schedule next
    call f_proc__ScheduleNext
    ; Next PID returned in L
    ld   a, l
    ld   (v_proc.CurrentPID), a

    ; Load user SP for next PID
    add  a, a
    ld   e, a
    ld   d, 0
    ld   hl, user_sp_table
    add  hl, de
    ld   e, (hl)
    inc  hl
    ld   d, (hl)
    ex   de, hl
    ld   sp, hl

    ; Arm TaskFuse
    ld   a, (v_proc.CurrentPID)
    ld   (0xFF20), a

    xor  a
    ld   (in_kernel), a
    retn

; ── Process Lifecycle & Driver Invocation Primitives ──────────────────────────
f_hal__RBFCall:
    push ix
    ; Push caller PID onto kernel stack
    ld   a, (v_proc.CurrentPID)
    ld   l, a
    ld   h, 0
    push hl

    ; CurrentPID = 1
    ld   a, 1
    ld   (v_proc.CurrentPID), a

    ; Save kernel stack pointer in saved_kernel_sp_table[1]
    ld   (saved_kernel_sp_table + 2), sp

    ; Load Task 1 stack pointer from user_sp_table[1]
    ld   sp, (user_sp_table + 2)

    ; Arm TaskFuse with target 1
    ld   a, 1
    ld   (0xFF20), a

    ; RETN switches to Task 1!
    retn

f_hal__InitRBF:
    push ix
    ld   hl, 4
    add  hl, sp
    ld   e, (hl)
    inc  hl
    ld   d, (hl)
    ld   (user_return_pc), de

    ; DMA copy 2 bytes of entryPC to Task 1 at 0x3EFE
    xor  a
    ld   (0xFF21), a
    ld   hl, user_return_pc
    ld   a, h
    ld   (0xFF22), a
    ld   a, l
    ld   (0xFF23), a
    ld   a, 1
    ld   (0xFF24), a
    ld   a, 0x3E
    ld   (0xFF25), a
    ld   a, 0xFE
    ld   (0xFF26), a
    ld   a, 2
    ld   (0xFF27), a
.wait_init_rbf:
    ld   a, (0xFF27)
    or   a
    jr   z, .wait_init_rbf

    ld   hl, 0x3EFE
    ld   (user_sp_table + 2), hl

    ; Call RBF once to let it initialize and enter hal.Sleep()
    call f_hal__RBFCall
    pop  ix
    ret

f_hal__ProcfsCall:
    push ix
    ; Push caller PID onto kernel stack
    ld   a, (v_proc.CurrentPID)
    ld   l, a
    ld   h, 0
    push hl

    ; CurrentPID = 2
    ld   a, 2
    ld   (v_proc.CurrentPID), a

    ; Save kernel stack pointer in saved_kernel_sp_table[2]
    ld   (saved_kernel_sp_table + 4), sp

    ; Load Task 2 stack pointer from user_sp_table[2]
    ld   sp, (user_sp_table + 4)

    ; Arm TaskFuse with target 2
    ld   a, 2
    ld   (0xFF20), a

    ; RETN switches to Task 2!
    retn

f_hal__InitProcfs:
    push ix
    ld   hl, 4
    add  hl, sp
    ld   e, (hl)
    inc  hl
    ld   d, (hl)
    ld   (user_return_pc), de

    ; DMA copy 2 bytes of entryPC to Task 2 at 0x3EFE
    xor  a
    ld   (0xFF21), a
    ld   hl, user_return_pc
    ld   a, h
    ld   (0xFF22), a
    ld   a, l
    ld   (0xFF23), a
    ld   a, 2
    ld   (0xFF24), a
    ld   a, 0x3E
    ld   (0xFF25), a
    ld   a, 0xFE
    ld   (0xFF26), a
    ld   a, 2
    ld   (0xFF27), a
.wait_init_procfs:
    ld   a, (0xFF27)
    or   a
    jr   z, .wait_init_procfs

    ld   hl, 0x3EFE
    ld   (user_sp_table + 4), hl

    ; Call PROCFS once to let it initialize and enter hal.Sleep()
    call f_hal__ProcfsCall
    pop  ix
    ret

f_hal__Sleep:
    out  (0x60), a
    defb 0x0A
    ret

f_hal__LaunchProcess:
    ; Stack on entry:
    ; 0(sp)..1(sp): return address
    ; 2(sp): PID (byte, widened to 2 bytes)
    ; 4(sp): initial SP / paramAddr (word, 2 bytes)
    ; 6(sp): initial PC (word, 2 bytes)
    push ix
    ld   hl, 4
    add  hl, sp
    ld   a, (hl)                   ; child PID
    inc  hl
    inc  hl
    ld   e, (hl)                   ; DE = initial SP
    inc  hl
    ld   d, (hl)
    inc  hl
    ld   c, (hl)                   ; BC = initial PC
    inc  hl
    ld   b, (hl)
    push bc
    pop  hl                        ; HL = initial PC
    ld   (temp_de), de             ; save initial SP in temp_de

    ; 1. Save parent's UserFrame (12 bytes) into saved_user_frame_table[parentPID * 12]
    push af
    push de
    push hl
    ld   a, (v_proc.CurrentPID)
    ld   l, a
    ld   h, 0
    add  hl, hl                      ; * 2
    add  hl, hl                      ; * 4
    ld   c, l
    ld   b, h
    add  hl, hl                      ; * 8
    add  hl, bc                      ; * 12
    ld   de, saved_user_frame_table
    add  hl, de                      ; HL = destination table entry
    ex   de, hl                      ; DE = destination
    ld   hl, v_syscall.UserFrame     ; HL = source
    ld   bc, 12
    ldir

    pop  hl                          ; restore HL = initial PC
    pop  de                          ; restore DE = initial SP
    pop  af                          ; restore A = child PID

    ; 2. Record parent PID and launched_from_kernel
    ld   c, a                        ; C = child PID
    ld   b, 0
    ld   hl, saved_parent_pid_table
    add  hl, bc
    ld   a, (v_proc.CurrentPID)
    ld   (hl), a

    ld   hl, launched_from_kernel
    add  hl, bc
    ld   a, 1
    ld   (hl), a

    ; 3. Save kernel SP in saved_kernel_sp_table[childPID]
    ld   hl, saved_kernel_sp_table
    add  hl, bc
    add  hl, bc
    ld   (temp_sp), sp
    ld   a, (temp_sp)
    ld   (hl), a
    inc  hl
    ld   a, (temp_sp + 1)
    ld   (hl), a

    ; 4. Check if child already has a user SP in user_sp_table[childPID]
    ld   hl, user_sp_table
    add  hl, bc
    add  hl, bc
    ld   e, (hl)
    inc  hl
    ld   d, (hl)
    ld   a, d
    or   e
    jr   nz, .sp_already_set
    ; If not set, use initial SP from arguments
    ld   de, (temp_de)
.sp_already_set:

    ; 5. Set CurrentPID = childPID
    ld   a, c
    ld   (v_proc.CurrentPID), a

    ; 6. Load user SP
    ex   de, hl
    ld   sp, hl
    ex   de, hl

    ; 7. Arm TaskFuse with childPID
    ld   a, c
    ld   (0xFF20), a

    ; 8. Setup initial registers:
    ; HL = initial SP + 2 (the paramAddr!)
    inc  de
    inc  de
    ld   h, d
    ld   l, e
    ld   bc, 0
    ld   de, 0
    ld   ix, 0
    ld   iy, 0
    xor  a

    ; 9. Leaving kernel mode, RETN to user!
    ld   (in_kernel), a
    retn

f_hal__InitUserContext:
    ; InitUserContext(pid byte, sp word, pc word)
    ; 2(sp) = pid
    ; 4(sp) = sp
    ; 6(sp) = pc
    push hl
    push de
    ld   hl, 6
    add  hl, sp
    ld   a, (hl) ; pid
    add  a, a
    ld   e, a
    ld   d, 0
    push hl
    ld   hl, user_sp_table
    add  hl, de
    ex   de, hl  ; DE = &user_sp_table[pid]
    pop  hl
    inc  hl
    inc  hl      ; HL = &(sp)
    ld   a, (hl)
    ld   (de), a
    inc  hl
    inc  de
    ld   a, (hl)
    ld   (de), a
    pop  de
    pop  hl
    ret

f_hal__FreeUserContext:
    ; FreeUserContext(pid byte)
    ; 2(sp) = pid
    push hl
    push de
    push bc
    ld   hl, 8
    add  hl, sp
    ld   a, (hl) ; pid
    ld   c, a
    ld   b, 0
    ld   hl, launched_from_kernel
    add  hl, bc
    ld   (hl), 0

    add  a, a
    ld   e, a
    ld   d, 0
    ld   hl, user_sp_table
    add  hl, de
    ld   (hl), 0
    inc  hl
    ld   (hl), 0

    ld   hl, saved_kernel_sp_table
    add  hl, de
    ld   (hl), 0
    inc  hl
    ld   (hl), 0
    pop  bc
    pop  de
    pop  hl
    ret

; ── IM 1 Interrupt Handler (Vector at $0038) ──────────────────────────────────
trap_irq:
    ; Hardware pushed PC onto stack and vectored to 0x0038
    ex   af, af'
    ld   a, (in_kernel)
    or   a
    jp   nz, .irq_kernel_mode

    ; Interrupted in user mode:
    ld   (temp_sp), sp
    ld   (temp_hl), hl
    ld   (temp_de), de
    ld   (temp_bc), bc
    ld   (temp_ix), ix
    ld   (temp_iy), iy

    ; Switch to IRQ stack
    ld   sp, irq_stack_top
    ld   a, 1
    ld   (in_kernel), a

    ; Save user SP into user_sp_table[CurrentPID]
    ld   a, (v_proc.CurrentPID)
    add  a, a
    ld   e, a
    ld   d, 0
    ld   hl, user_sp_table
    add  hl, de
    ld   bc, (temp_sp)
    ld   (hl), c
    inc  hl
    ld   (hl), b

    ; Check Terminal Rx interrupt (bit 1 of $FF02)
    ld   a, (0xFF02)
    and  2
    jr   z, .u_no_rx
    call f_dev__TermRxInterrupt

.u_no_rx:
    ; Check Timer interrupt (bit 0 of $FF02)
    ld   a, (0xFF02)
    and  1
    jr   z, .u_no_timer

    ; Acknowledge timer
    ld   a, 1
    ld   (0xFF02), a

    ; If CurrentPID <= 1, do not preempt
    ld   a, (v_proc.CurrentPID)
    cp   2
    jr   c, .u_no_timer

    ; Call scheduler
    call f_proc__ScheduleNext
    ; Next PID in L
    ld   a, l
    ld   (v_proc.CurrentPID), a

.u_no_timer:
    ; Resume process:
    ld   a, (v_proc.CurrentPID)
    add  a, a
    ld   e, a
    ld   d, 0
    ld   hl, user_sp_table
    add  hl, de
    ld   e, (hl)
    inc  hl
    ld   d, (hl)
    ex   de, hl
    ld   sp, hl

    ; Arm TaskFuse
    ld   a, (v_proc.CurrentPID)
    ld   (0xFF20), a

    ; Restore registers
    ld   iy, (temp_iy)
    ld   ix, (temp_ix)
    ld   bc, (temp_bc)
    ld   de, (temp_de)
    ld   hl, (temp_hl)
    ex   af, af'

    xor  a
    ld   (in_kernel), a
    ei
    retn

.irq_kernel_mode:
    ; Interrupted in kernel mode: only check devices, don't preempt
    ld   a, (0xFF02)
    and  2
    jr   z, .k_no_rx
    ld   (temp_hl), hl
    call f_dev__TermRxInterrupt
    ld   hl, (temp_hl)

.k_no_rx:
    ld   a, (0xFF02)
    and  1
    jr   z, .k_no_timer
    ld   a, 1
    ld   (0xFF02), a

.k_no_timer:
    ex   af, af'
    ei
    reti
