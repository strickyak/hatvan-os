; ══════════════════════════════════════════════════════════════════════════════
; Hatvan OS Trap, Interrupt & Context Switch Handlers for Intel 8086 (Task 0)
; ══════════════════════════════════════════════════════════════════════════════

	SEGMENT CODE

; ── DMA Helpers for User Syscall Register Block ($0020..$0029) ───────────────
; dma_read_user_frame: AL = user task ID -> copies 10 bytes from Task AL:$0020 to Task 0:v_syscall.UserFrame
dma_read_user_frame:
	MOV     [$FF24], AL                     ; SrcTask = AL
	MOV     BYTE PTR [$FF25], 0             ; SrcPage = 0 (Data Segment)
	MOV     BYTE PTR [$FF26], $00
	MOV     BYTE PTR [$FF27], $20           ; SrcAddr = $0020
	MOV     BYTE PTR [$FF28], 0             ; DstTask = 0
	MOV     BYTE PTR [$FF29], 0             ; DstPage = 0
	MOV     BYTE PTR [$FF2A], >v_syscall.UserFrame
	MOV     BYTE PTR [$FF2B], <v_syscall.UserFrame
	MOV     BYTE PTR [$FF2C], 0
	MOV     BYTE PTR [$FF2D], 0
	MOV     BYTE PTR [$FF2E], 0
	MOV     BYTE PTR [$FF2F], 10            ; Count = 10 bytes
	MOV     BYTE PTR [$FF21], 1             ; Execute DMA
.L_wait_dma_r:
	MOV     AL, [$FF21]
	TEST    AL, AL
	JZ      .L_wait_dma_r
	RET

; dma_write_user_frame: AL = user task ID -> copies 10 bytes from Task 0:v_syscall.UserFrame to Task AL:$0020
dma_write_user_frame:
	MOV     [$FF28], AL                     ; DstTask = AL
	MOV     BYTE PTR [$FF29], 0             ; DstPage = 0 (Data Segment)
	MOV     BYTE PTR [$FF2A], $00
	MOV     BYTE PTR [$FF2B], $20           ; DstAddr = $0020
	MOV     BYTE PTR [$FF24], 0             ; SrcTask = 0
	MOV     BYTE PTR [$FF25], 0             ; SrcPage = 0
	MOV     BYTE PTR [$FF26], >v_syscall.UserFrame
	MOV     BYTE PTR [$FF27], <v_syscall.UserFrame
	MOV     BYTE PTR [$FF2C], 0
	MOV     BYTE PTR [$FF2D], 0
	MOV     BYTE PTR [$FF2E], 0
	MOV     BYTE PTR [$FF2F], 10            ; Count = 10 bytes
	MOV     BYTE PTR [$FF21], 1             ; Execute DMA
.L_wait_dma_w:
	MOV     AL, [$FF21]
	TEST    AL, AL
	JZ      .L_wait_dma_w
	RET

; ── Software Syscall & Driver Sleep Handler (Vector 30h at $00C0) ────────────
trap_syscall:
	MOV     [temp_sp], SP
	MOV     [temp_bp], BP

	MOV     AL, [$FF60]
	MOV     [call_num], AL
	MOV     AL, [$FF61]
	MOV     [trap_src_task], AL

	; Check if trap came from driver task (Task 1: RBF or Task 2: PROCFS) via hal.Sleep()
	CMP     AL, 1
	JE      .L_driver_sleep_return
	CMP     AL, 2
	JE      .L_driver_sleep_return

	; User process syscall trap (PID >= 3):
	MOV     BL, [v_proc.CurrentPID]
	XOR     BH, BH
	SHL     BX, 1
	MOV     AX, [temp_sp]
	MOV     [user_sp_table + BX], AX
	MOV     AX, [temp_bp]
	MOV     [user_bp_table + BX], AX

	; Switch to Task 0 kernel hardware stack
	MOV     AX, [saved_kernel_sp_table + BX]
	TEST    AX, AX
	JNZ     .L_ksp_ok
	MOV     AX, $E000
.L_ksp_ok:
	MOV     SP, AX
	MOV     BP, [saved_kernel_bp_table + BX]

	MOV     BYTE PTR [in_kernel], 1

	; Copy user syscall registers ($0020..$0029) into v_syscall.UserFrame
	MOV     AL, [v_proc.CurrentPID]
	CALL    dma_read_user_frame

	; Preserve this task's call_num on Task 0's stack across f_syscall__Dispatch
	XOR     AX, AX
	MOV     AL, [call_num]
	PUSH    AX                              ; Save call_num across potential child F$Exit
	PUSH    AX                              ; Arg 1: call_num
	CALL    f_syscall__Dispatch
	ADD     SP, 2

	; Restore this task's call_num and check if process exited ($06 = F$Exit)
	POP     AX
	MOV     [call_num], AL
	CMP     AL, $06
	JE      .L_process_exited

	; Copy updated v_syscall.UserFrame back to user task ($0020..$0029)
	MOV     AL, [v_proc.CurrentPID]
	CALL    dma_write_user_frame

	; Restore user SP and BP and return via TaskFuse + IRET
	MOV     BL, [v_proc.CurrentPID]
	XOR     BH, BH
	SHL     BX, 1
	MOV     SP, [user_sp_table + BX]
	MOV     BP, [user_bp_table + BX]

	MOV     AL, [v_proc.CurrentPID]
	MOV     [$FF20], AL                     ; Arm TaskFuse
	MOV     BYTE PTR [in_kernel], 0
	IRET

.L_driver_sleep_return:
	; Driver called hal.Sleep() (INT 30h; DB $0A)
	MOV     BL, [v_proc.CurrentPID]
	XOR     BH, BH
	SHL     BX, 1
	MOV     AX, [temp_sp]
	MOV     [user_sp_table + BX], AX
	MOV     AX, [temp_bp]
	MOV     [user_bp_table + BX], AX

	; Restore kernel SP and BP
	MOV     SP, [saved_kernel_sp_table + BX]
	MOV     BP, [saved_kernel_bp_table + BX]

	; Pop caller PID and saved BP from kernel stack
	POP     AX
	MOV     [v_proc.CurrentPID], AL
	POP     BP
	MOV     BYTE PTR [in_kernel], 1
	RET

.L_process_exited:
	MOV     BL, [v_proc.CurrentPID]
	XOR     BH, BH
	MOV     [temp_pid], BL
	MOV     AL, [launched_from_kernel + BX]
	TEST    AL, AL
	JZ      .L_no_caller_waiting

	; Clear launched_from_kernel[CurrentPID]
	MOV     BYTE PTR [launched_from_kernel + BX], 0

	; Restore parent PID
	MOV     AL, [saved_parent_pid_table + BX]
	MOV     [v_proc.CurrentPID], AL

	; Restore parent's v_syscall.UserFrame (10 bytes) from saved_user_frame_table[parentPID * 16]
	XOR     AH, AH
	MOV     CL, 4
	SHL     AX, CL
	MOV     SI, saved_user_frame_table
	ADD     SI, AX
	MOV     DI, v_syscall.UserFrame
	MOV     CX, 10
	CLD
	REP     MOVSB

	; Restore kernel SP and BP saved in LaunchProcess (for child PID)
	MOV     BL, [temp_pid]
	XOR     BH, BH
	SHL     BX, 1
	MOV     SP, [saved_kernel_sp_table + BX]
	MOV     BP, [saved_kernel_bp_table + BX]
	MOV     BYTE PTR [in_kernel], 1
	RET

.L_no_caller_waiting:
	CALL    f_proc__ScheduleNext
	MOV     [v_proc.CurrentPID], AL
	MOV     BL, AL
	XOR     BH, BH
	SHL     BX, 1
	MOV     SP, [user_sp_table + BX]
	MOV     BP, [user_bp_table + BX]
	MOV     AL, [v_proc.CurrentPID]
	MOV     [$FF20], AL
	MOV     BYTE PTR [in_kernel], 0
	IRET

; ── Hardware Timer / Terminal IRQ Handler (Vector 08h at $0020) ──────────────
trap_hw_irq:
	CMP     BYTE PTR [in_kernel], 0
	JNE     .L_irq_in_kernel

	; Interrupted in user mode: save all user registers into Task 0 globals
	MOV     [irq_saved_ax], AX
	MOV     [irq_saved_bx], BX
	MOV     [irq_saved_cx], CX
	MOV     [irq_saved_dx], DX
	MOV     [irq_saved_si], SI
	MOV     [irq_saved_di], DI
	MOV     [irq_saved_bp], BP

	MOV     BL, [v_proc.CurrentPID]
	XOR     BH, BH
	SHL     BX, 1
	MOV     [user_sp_table + BX], SP

	; Switch to Task 0 kernel stack
	MOV     AX, [saved_kernel_sp_table + BX]
	TEST    AX, AX
	JNZ     .L_irq_ksp_ok
	MOV     AX, $E000
.L_irq_ksp_ok:
	MOV     SP, AX
	MOV     BP, [saved_kernel_bp_table + BX]

	MOV     BYTE PTR [in_kernel], 1

	; Check Terminal Rx interrupt (bit 1 of $FF02)
	MOV     AL, [$FF02]
	TEST    AL, $02
	JZ      .L_u_no_rx
	CALL    f_dev__TermRxInterrupt

.L_u_no_rx:
	MOV     AL, [$FF02]
	TEST    AL, $01
	JZ      .L_u_no_timer
	MOV     BYTE PTR [$FF02], $01           ; Acknowledge timer

.L_u_no_timer:
	MOV     BL, [v_proc.CurrentPID]
	XOR     BH, BH
	SHL     BX, 1
	MOV     SP, [user_sp_table + BX]

	MOV     AL, [v_proc.CurrentPID]
	MOV     [$FF20], AL                     ; Arm TaskFuse
	MOV     BYTE PTR [in_kernel], 0

	MOV     BP, [irq_saved_bp]
	MOV     DI, [irq_saved_di]
	MOV     SI, [irq_saved_si]
	MOV     DX, [irq_saved_dx]
	MOV     CX, [irq_saved_cx]
	MOV     BX, [irq_saved_bx]
	MOV     AX, [irq_saved_ax]
	IRET

.L_irq_in_kernel:
	PUSH    AX
	PUSH    BX
	PUSH    CX
	PUSH    DX
	PUSH    SI
	PUSH    DI
	PUSH    BP

	MOV     AL, [$FF02]
	TEST    AL, $01
	JZ      .L_k_no_timer
	MOV     BYTE PTR [$FF02], $01           ; Acknowledge timer
.L_k_no_timer:
	MOV     AL, [$FF02]
	TEST    AL, $02
	JZ      .L_k_no_rx
	CALL    f_dev__TermRxInterrupt
.L_k_no_rx:
	POP     BP
	POP     DI
	POP     SI
	POP     DX
	POP     CX
	POP     BX
	POP     AX
	IRET

; ── Driver & Process Context Switch Primitives ───────────────────────────────
f_hal__RBFCall:
	PUSH    BP
	XOR     AX, AX
	MOV     AL, [v_proc.CurrentPID]
	PUSH    AX
	MOV     [saved_kernel_sp_table + 2], SP
	MOV     [saved_kernel_bp_table + 2], BP

	MOV     BYTE PTR [v_proc.CurrentPID], 1
	MOV     SP, [user_sp_table + 2]
	MOV     BP, [user_bp_table + 2]
	MOV     AL, 1
	MOV     [$FF20], AL                     ; Arm TaskFuse for Task 1
	IRET

f_hal__InitRBF:
	PUSH    BP
	MOV     BP, SP
	MOV     AX, [BP+4]                      ; entryPC
	MOV     [init_iret_frame], AX
	MOV     WORD PTR [init_iret_frame+2], $1000
	MOV     WORD PTR [init_iret_frame+4], $F002

	; DMA copy 6-byte IRET frame to Task 1 at $DFA0..$DFA5
	MOV     BYTE PTR [$FF24], 0
	MOV     BYTE PTR [$FF25], 0
	MOV     BYTE PTR [$FF26], >init_iret_frame
	MOV     BYTE PTR [$FF27], <init_iret_frame
	MOV     BYTE PTR [$FF28], 1
	MOV     BYTE PTR [$FF29], 0
	MOV     BYTE PTR [$FF2A], $DF
	MOV     BYTE PTR [$FF2B], $A0
	MOV     BYTE PTR [$FF2C], 0
	MOV     BYTE PTR [$FF2D], 0
	MOV     BYTE PTR [$FF2E], 0
	MOV     BYTE PTR [$FF2F], 6
	MOV     BYTE PTR [$FF21], 1
.L_wait_init_rbf:
	MOV     AL, [$FF21]
	TEST    AL, AL
	JZ      .L_wait_init_rbf

	MOV     WORD PTR [user_sp_table + 2], $DFA0
	MOV     WORD PTR [user_bp_table + 2], $DFA6
	CALL    f_hal__RBFCall
	POP     BP
	RET

f_hal__ProcfsCall:
	PUSH    BP
	XOR     AX, AX
	MOV     AL, [v_proc.CurrentPID]
	PUSH    AX
	MOV     [saved_kernel_sp_table + 4], SP
	MOV     [saved_kernel_bp_table + 4], BP

	MOV     BYTE PTR [v_proc.CurrentPID], 2
	MOV     SP, [user_sp_table + 4]
	MOV     BP, [user_bp_table + 4]
	MOV     AL, 2
	MOV     [$FF20], AL                     ; Arm TaskFuse for Task 2
	IRET

f_hal__InitProcfs:
	PUSH    BP
	MOV     BP, SP
	MOV     AX, [BP+4]                      ; entryPC
	MOV     [init_iret_frame], AX
	MOV     WORD PTR [init_iret_frame+2], $1000
	MOV     WORD PTR [init_iret_frame+4], $F002

	; DMA copy 6-byte IRET frame to Task 2 at $DFA0..$DFA5
	MOV     BYTE PTR [$FF24], 0
	MOV     BYTE PTR [$FF25], 0
	MOV     BYTE PTR [$FF26], >init_iret_frame
	MOV     BYTE PTR [$FF27], <init_iret_frame
	MOV     BYTE PTR [$FF28], 2
	MOV     BYTE PTR [$FF29], 0
	MOV     BYTE PTR [$FF2A], $DF
	MOV     BYTE PTR [$FF2B], $A0
	MOV     BYTE PTR [$FF2C], 0
	MOV     BYTE PTR [$FF2D], 0
	MOV     BYTE PTR [$FF2E], 0
	MOV     BYTE PTR [$FF2F], 6
	MOV     BYTE PTR [$FF21], 1
.L_wait_init_procfs:
	MOV     AL, [$FF21]
	TEST    AL, AL
	JZ      .L_wait_init_procfs

	MOV     WORD PTR [user_sp_table + 4], $DFA0
	MOV     WORD PTR [user_bp_table + 4], $DFA6
	CALL    f_hal__ProcfsCall
	POP     BP
	RET

f_hal__Sleep:
	INT     $30
	DB      $0A
	RET

f_hal__LaunchProcess:
	; Args on stack: [SP+2]=pid, [SP+4]=sp, [SP+6]=pc
	MOV     BX, SP
	MOV     AL, [BX+2]
	MOV     [temp_pid], AL

	; Save parent's v_syscall.UserFrame (10 bytes) into saved_user_frame_table[parentPID * 16]
	XOR     AX, AX
	MOV     AL, [v_proc.CurrentPID]
	MOV     CL, 4
	SHL     AX, CL
	MOV     DI, saved_user_frame_table
	ADD     DI, AX
	MOV     SI, v_syscall.UserFrame
	MOV     CX, 10
	CLD
	REP     MOVSB

	MOV     BL, [temp_pid]
	XOR     BH, BH
	MOV     AL, [v_proc.CurrentPID]
	MOV     [saved_parent_pid_table + BX], AL
	MOV     BYTE PTR [launched_from_kernel + BX], 1

	SHL     BX, 1
	MOV     [saved_kernel_sp_table + BX], SP
	MOV     [saved_kernel_bp_table + BX], BP

	MOV     AX, [user_sp_table + BX]
	TEST    AX, AX
	JNZ     .L_lp_sp_ok
	MOV     AX, $FDFA
	MOV     [user_sp_table + BX], AX
.L_lp_sp_ok:
	MOV     SP, AX
	MOV     BP, [user_bp_table + BX]

	MOV     AL, [temp_pid]
	MOV     [v_proc.CurrentPID], AL
	MOV     [$FF20], AL                     ; Arm TaskFuse
	MOV     BYTE PTR [in_kernel], 0
	IRET

f_hal__InitUserContext:
	; Args: [SP+2]=pid, [SP+4]=sp, [SP+6]=pc
	MOV     SI, SP
	MOV     BL, [SI+2]
	XOR     BH, BH
	SHL     BX, 1
	MOV     AX, [SI+4]
	MOV     [user_sp_table + BX], AX
	ADD     AX, 6
	MOV     [user_bp_table + BX], AX
	RET

f_hal__FreeUserContext:
	; Args: [SP+2]=pid
	MOV     SI, SP
	MOV     BL, [SI+2]
	XOR     BH, BH
	MOV     BYTE PTR [launched_from_kernel + BX], 0
	SHL     BX, 1
	MOV     WORD PTR [user_sp_table + BX], 0
	MOV     WORD PTR [user_bp_table + BX], 0
	MOV     WORD PTR [saved_kernel_sp_table + BX], 0
	MOV     WORD PTR [saved_kernel_bp_table + BX], 0
	RET

; ── Kernel Trap & Context Switch State (in SEGMENT DATA) ─────────────────────
	SEGMENT DATA

in_kernel:
	DS      1
call_num:
	DS      1
trap_src_task:
	DS      1
temp_pid:
	DS      1
temp_sp:
	DS      2
temp_bp:
	DS      2

irq_saved_ax:
	DS      2
irq_saved_bx:
	DS      2
irq_saved_cx:
	DS      2
irq_saved_dx:
	DS      2
irq_saved_si:
	DS      2
irq_saved_di:
	DS      2
irq_saved_bp:
	DS      2

user_sp_table:
	DS      32
user_bp_table:
	DS      32
saved_kernel_sp_table:
	DS      32
saved_kernel_bp_table:
	DS      32
launched_from_kernel:
	DS      16
saved_parent_pid_table:
	DS      16
saved_user_frame_table:
	DS      256
init_iret_frame:
	DS      6

	END     kernel_reset
