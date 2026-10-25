; Hatvan OS MOS 6502 Trap Handler, Context Switcher & Hardware Vectors
; Resides in Task 0 memory space.
; Hardware vectors at $FFFA (NMI), $FFFC (RESET), $FFFE (IRQ / BRK).

; Per-task saved hardware stack pointer ($0100 + SP)
user_hw_sp_table:
	DS      16

; Per-task saved 18-byte Zero-Page C runtime state (ZP_SP..ZP_RF: $00..$11)
saved_kernel_zp_table:
	DS      288

saved_kernel_hw_sp_table:
	DS      16

saved_parent_pid_table:
	DS      16

launched_from_kernel:
	DS      16

saved_user_frame_table:
	DS      256

irq_saved_zp:
	DS      16

in_kernel:
	DB      1

call_num:
	DB      0

trap_src_task:
	DB      0

temp_a:
	DB      0
temp_x:
	DB      0
temp_y:
	DB      0
temp_hw_sp:
	DB      0
temp_pid:
	DB      0

init_rti_frame:
	DB      $24, $00, $00

; ── Kernel Reset Entry Point (Vector at $FFFC) ───────────────────────────────
kernel_reset:
	SEI
	CLD
	LDX     #$FF
	TXS

	; Initialize Task 0 software C stack below shared curtain ($E000)
	LDA     #<$E000
	STA     ZP_SP
	STA     ZP_FP
	LDA     #>$E000
	STA     ZP_SP+1
	STA     ZP_FP+1

	LDA     #1
	STA     in_kernel

	JSR     _main
.L_khang:
	JMP     .L_khang

_printf:
	RTS

_exit:
__exit:
	LDY     #0
	LDA     (ZP_SP),Y
	STA     $FF05
.L_exit_hang:
	JMP     .L_exit_hang

; ── Helper: DMA Copy 10-byte UserFrame from Task A ($0020) to Task 0 ─────────
dma_read_user_frame:
	STA     $FF24           ; DmaSrcTask
	LDA     #0
	STA     $FF25
	STA     $FF26
	LDA     #$20
	STA     $FF27           ; SrcAddr = $000020
	LDA     #0
	STA     $FF28           ; DmaDstTask = 0
	STA     $FF29
	LDA     #>v_syscall.UserFrame
	STA     $FF2A
	LDA     #<v_syscall.UserFrame
	STA     $FF2B
	LDA     #0
	STA     $FF2C
	STA     $FF2D
	STA     $FF2E
	LDA     #10
	STA     $FF2F
	LDA     #1
	STA     $FF21
.L_wait_druf:
	LDA     $FF21
	BEQ     .L_wait_druf
	RTS

; ── Helper: DMA Copy 10-byte UserFrame from Task 0 to Task A ($0020) ─────────
dma_write_user_frame:
	STA     $FF28           ; DmaDstTask
	LDA     #0
	STA     $FF29
	STA     $FF2A
	LDA     #$20
	STA     $FF2B           ; DstAddr = $000020
	LDA     #0
	STA     $FF24           ; DmaSrcTask = 0
	STA     $FF25
	LDA     #>v_syscall.UserFrame
	STA     $FF26
	LDA     #<v_syscall.UserFrame
	STA     $FF27
	LDA     #0
	STA     $FF2C
	STA     $FF2D
	STA     $FF2E
	LDA     #10
	STA     $FF2F
	LDA     #1
	STA     $FF21
.L_wait_dwuf:
	LDA     $FF21
	BEQ     .L_wait_dwuf
	RTS

; ── IRQ / BRK Handler (Vector at $FFFE) & NMI Handler (Vector at $FFFA) ──────
trap_nmi:
trap_irq:
	STA     temp_a
	STX     temp_x
	STY     temp_y
	TSX
	STX     temp_hw_sp

	; Distinguish hardware IRQ ($FF61 == $FF) from software BRK ($FF61 == 0..15)
	LDA     $FF61
	CMP     #$FF
	BNE     .L_sw_brk
	JMP     .L_hw_irq
.L_sw_brk:

	; Software BRK trap: read syscall number ($FF60) and source task ($FF61)
	LDA     $FF60
	STA     call_num
	LDA     $FF61
	STA     trap_src_task

	; Check if trap came from driver task (Task 1: RBF or Task 2: PROCFS) via hal.Sleep()
	CMP     #1
	BEQ     .L_driver_sleep_return
	CMP     #2
	BEQ     .L_driver_sleep_return

	; User process syscall trap (PID >= 3):
	LDX     v_proc.CurrentPID
	LDA     temp_hw_sp
	STA     user_hw_sp_table,X

	; Switch to Task 0 hardware stack
	LDA     saved_kernel_hw_sp_table,X
	BNE     .L_khwsp_ok
	LDA     #$FF
.L_khwsp_ok:
	TAX
	TXS

	LDA     #1
	STA     in_kernel

	; Copy user syscall registers ($0020..$0029) into v_syscall.UserFrame
	LDA     v_proc.CurrentPID
	JSR     dma_read_user_frame

	; Call f_syscall__Dispatch(call_num)
	LDA     call_num
	PHA
	STA     ZP_R0
	LDA     #0
	STA     ZP_R0+1
	JSR     __push_r0
	JSR     f_syscall__Dispatch
	LDY     #2
	JSR     __add_sp_y

	; Restore this task's call_num and check if process exited ($06 = F$Exit)
	PLA
	STA     call_num
	CMP     #$06
	BEQ     .L_process_exited

	; Copy updated v_syscall.UserFrame back to user task ($0020..$0029)
	LDA     v_proc.CurrentPID
	JSR     dma_write_user_frame

	; Restore user hardware stack pointer and return via TaskFuse + RTI
	LDX     v_proc.CurrentPID
	LDA     user_hw_sp_table,X
	TAX
	TXS

	LDA     v_proc.CurrentPID
	STA     $FF20           ; Arm TaskFuse
	LDA     #0
	STA     in_kernel
	RTI

.L_driver_sleep_return:
	; Driver called hal.Sleep() (BRK #$0A)
	LDX     v_proc.CurrentPID
	LDA     temp_hw_sp
	STA     user_hw_sp_table,X

	; Restore kernel hardware stack pointer
	LDA     saved_kernel_hw_sp_table,X
	TAX
	TXS

	; Pull caller PID from kernel hardware stack
	PLA
	STA     v_proc.CurrentPID
	LDA     #1
	STA     in_kernel
	RTS

.L_process_exited:
	LDX     v_proc.CurrentPID
	STX     temp_pid
	LDA     launched_from_kernel,X
	BEQ     .L_no_caller_waiting

	; Clear launched_from_kernel[CurrentPID]
	LDA     #0
	STA     launched_from_kernel,X

	; Restore parent PID
	LDA     saved_parent_pid_table,X
	STA     v_proc.CurrentPID

	; Restore parent's v_syscall.UserFrame (10 bytes) from saved_user_frame_table[parentPID * 16]
	ASL     A
	ASL     A
	ASL     A
	ASL     A
	TAY
	LDX     #0
.L_restore_uf:
	LDA     saved_user_frame_table,Y
	STA     v_syscall.UserFrame,X
	INY
	INX
	CPX     #10
	BNE     .L_restore_uf

	; Restore kernel hardware SP saved in LaunchProcess (for child PID)
	LDX     temp_pid
	LDA     saved_kernel_hw_sp_table,X
	TAX
	TXS
	LDA     #1
	STA     in_kernel
	RTS

.L_no_caller_waiting:
	JSR     f_proc__ScheduleNext
	LDA     ZP_R0
	STA     v_proc.CurrentPID
	TAX
	LDA     user_hw_sp_table,X
	TAX
	TXS
	LDA     v_proc.CurrentPID
	STA     $FF20
	LDA     #0
	STA     in_kernel
	RTI

; ── Hardware Timer / Terminal IRQ Handler ────────────────────────────────────
.L_hw_irq:
	LDA     in_kernel
	BNE     .L_irq_in_kernel

	; Interrupted in user mode: save user hardware SP
	LDX     v_proc.CurrentPID
	LDA     temp_hw_sp
	STA     user_hw_sp_table,X

	; Switch to Task 0 kernel hardware stack
	LDA     saved_kernel_hw_sp_table,X
	BNE     .L_irq_khwsp_ok
	LDA     #$FF
.L_irq_khwsp_ok:
	TAX
	TXS

	; Save Task 0 ZP scratch ($04..$11)
	LDX     #13
.L_irq_save_zp:
	LDA     $04,X
	STA     irq_saved_zp,X
	DEX
	BPL     .L_irq_save_zp

	LDA     #1
	STA     in_kernel

	; Check Terminal Rx interrupt (bit 1 of $FF02)
	LDA     $FF02
	AND     #$02
	BEQ     .L_u_no_rx
	JSR     f_dev__TermRxInterrupt

.L_u_no_rx:
	LDA     $FF02
	AND     #$01
	BEQ     .L_u_no_timer
	LDA     #$01
	STA     $FF02           ; Acknowledge timer

.L_u_no_timer:
	; Restore Task 0 ZP scratch ($04..$11)
	LDX     #13
.L_irq_rest_zp:
	LDA     irq_saved_zp,X
	STA     $04,X
	DEX
	BPL     .L_irq_rest_zp

	LDX     v_proc.CurrentPID
	LDA     user_hw_sp_table,X
	TAX
	TXS
	LDA     v_proc.CurrentPID
	STA     $FF20
	LDA     #0
	STA     in_kernel
	LDA     temp_y
	TAY
	LDA     temp_x
	TAX
	LDA     temp_a
	RTI

.L_irq_in_kernel:
	LDA     $FF02
	AND     #$01
	BEQ     .L_k_no_timer
	LDA     #$01
	STA     $FF02
.L_k_no_timer:
	LDA     temp_y
	TAY
	LDA     temp_x
	TAX
	LDA     temp_a
	RTI

; ── Driver & Process Lifecycle Primitives ────────────────────────────────────

f_hal__RBFCall:
	LDA     v_proc.CurrentPID
	PHA
	TSX
	STX     saved_kernel_hw_sp_table+1

	LDA     #1
	STA     v_proc.CurrentPID
	LDX     user_hw_sp_table+1
	TXS
	STA     $FF20           ; Arm TaskFuse for Task 1
	RTI

f_hal__InitRBF:
	LDY     #0
	LDA     (ZP_SP),Y
	STA     init_rti_frame+1
	INY
	LDA     (ZP_SP),Y
	STA     init_rti_frame+2
	LDA     #$24
	STA     init_rti_frame

	; DMA copy 3-byte RTI frame [P, PC_lo, PC_hi] to Task 1 at $01FD..$01FF
	LDA     #0
	STA     $FF24
	STA     $FF25
	LDA     #>init_rti_frame
	STA     $FF26
	LDA     #<init_rti_frame
	STA     $FF27
	LDA     #1
	STA     $FF28
	LDA     #0
	STA     $FF29
	LDA     #$01
	STA     $FF2A
	LDA     #$FD
	STA     $FF2B
	LDA     #0
	STA     $FF2C
	STA     $FF2D
	STA     $FF2E
	LDA     #3
	STA     $FF2F
	LDA     #1
	STA     $FF21
.L_wait_init_rbf:
	LDA     $FF21
	BEQ     .L_wait_init_rbf

	LDA     #$FC
	STA     user_hw_sp_table+1
	JSR     f_hal__RBFCall
	RTS

f_hal__ProcfsCall:
	LDA     v_proc.CurrentPID
	PHA
	TSX
	STX     saved_kernel_hw_sp_table+2

	LDA     #2
	STA     v_proc.CurrentPID
	LDX     user_hw_sp_table+2
	TXS
	STA     $FF20           ; Arm TaskFuse for Task 2
	RTI

f_hal__InitProcfs:
	LDY     #0
	LDA     (ZP_SP),Y
	STA     init_rti_frame+1
	INY
	LDA     (ZP_SP),Y
	STA     init_rti_frame+2
	LDA     #$24
	STA     init_rti_frame

	; DMA copy 3-byte RTI frame [P, PC_lo, PC_hi] to Task 2 at $01FD..$01FF
	LDA     #0
	STA     $FF24
	STA     $FF25
	LDA     #>init_rti_frame
	STA     $FF26
	LDA     #<init_rti_frame
	STA     $FF27
	LDA     #2
	STA     $FF28
	LDA     #0
	STA     $FF29
	LDA     #$01
	STA     $FF2A
	LDA     #$FD
	STA     $FF2B
	LDA     #0
	STA     $FF2C
	STA     $FF2D
	STA     $FF2E
	LDA     #3
	STA     $FF2F
	LDA     #1
	STA     $FF21
.L_wait_init_procfs:
	LDA     $FF21
	BEQ     .L_wait_init_procfs

	LDA     #$FC
	STA     user_hw_sp_table+2
	JSR     f_hal__ProcfsCall
	RTS

f_hal__Sleep:
	BRK
	DB      $0A
	RTS

f_hal__LaunchProcess:
	; Args at (ZP_SP): +0=pid, +2=sp, +4=pc
	LDY     #0
	LDA     (ZP_SP),Y
	STA     temp_pid

	; Save parent's v_syscall.UserFrame (10 bytes) into saved_user_frame_table[parentPID * 16]
	LDA     v_proc.CurrentPID
	ASL     A
	ASL     A
	ASL     A
	ASL     A
	TAY
	LDX     #0
.L_save_uf:
	LDA     v_syscall.UserFrame,X
	STA     saved_user_frame_table,Y
	INY
	INX
	CPX     #10
	BNE     .L_save_uf

	LDX     temp_pid
	LDA     v_proc.CurrentPID
	STA     saved_parent_pid_table,X
	LDA     #1
	STA     launched_from_kernel,X

	TSX
	TXA
	LDX     temp_pid
	STA     saved_kernel_hw_sp_table,X

	LDA     user_hw_sp_table,X
	BNE     .L_lp_sp_ok
	LDA     #$FC
	STA     user_hw_sp_table,X
.L_lp_sp_ok:
	STX     v_proc.CurrentPID
	TAX
	TXS

	LDA     v_proc.CurrentPID
	STA     $FF20
	LDA     #0
	STA     in_kernel
	RTI

f_hal__InitUserContext:
	LDY     #0
	LDA     (ZP_SP),Y
	TAX
	LDA     #$FC
	STA     user_hw_sp_table,X
	RTS

f_hal__FreeUserContext:
	LDY     #0
	LDA     (ZP_SP),Y
	TAX
	LDA     #0
	STA     launched_from_kernel,X
	STA     user_hw_sp_table,X
	STA     saved_kernel_hw_sp_table,X
	RTS

; ── Hardware Vectors ($FFFA..$FFFF) ──────────────────────────────────────────
	ORG     $FFFA
	DW      trap_nmi
	DW      kernel_reset
	DW      trap_irq

	END     kernel_reset
