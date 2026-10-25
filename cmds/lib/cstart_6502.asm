; Hatvan OS User Process Startup Stub & Syscall Wrappers for MOS 6502
; Appended after MiniGolf-generated 6502 assembly; overrides entry point via END user_start.

UF_CC   = $20
UF_A    = $21
UF_B    = $22
UF_DP   = $23
UF_X    = $24   ; $24..$25: 16-bit X register in SyscallFrame
UF_Y    = $26   ; $26..$27: 16-bit Y register in SyscallFrame
UF_U    = $28   ; $28..$29: 16-bit U register in SyscallFrame
ZP_ARGS = $2A   ; $2A..$2B: initial argsPtr placed by hal.SetupChildTask

user_start:
	CLD
	LDX     #$FF
	TXS

	; Initialize v_sys.ArgsPtr, ZP_SP, and ZP_FP from ZP_ARGS ($002A..$002B)
	LDA     ZP_ARGS
	STA     v_sys.ArgsPtr
	STA     ZP_SP
	STA     ZP_FP
	LDA     ZP_ARGS+1
	STA     v_sys.ArgsPtr+1
	STA     ZP_SP+1
	STA     ZP_FP+1

	CLI
	JSR     _main

	LDA     #0
	STA     UF_B
	BRK
	DB      $06             ; F$Exit
.L_user_hang:
	JMP     .L_user_hang

_printf:
	RTS

__exit:
_exit:
f_sys__SysExit:
	LDY     #0
	LDA     (ZP_SP),Y
	STA     UF_B
	BRK
	DB      $06             ; F$Exit
.L_exit_hang:
	JMP     .L_exit_hang

; Helper: load arg0 (word) into UF_X, arg1 (byte) into UF_A
.L_arg_x_a:
	LDY     #0
	LDA     (ZP_SP),Y
	STA     UF_X
	INY
	LDA     (ZP_SP),Y
	STA     UF_X+1
	INY
	LDA     (ZP_SP),Y
	STA     UF_A
	RTS

; Helper: load arg0 (byte) into UF_A, arg1 (word) into UF_X, arg2 (word) into UF_Y
.L_arg_a_x_y:
	LDY     #0
	LDA     (ZP_SP),Y
	STA     UF_A
	LDY     #2
	LDA     (ZP_SP),Y
	STA     UF_X
	INY
	LDA     (ZP_SP),Y
	STA     UF_X+1
	INY
	LDA     (ZP_SP),Y
	STA     UF_Y
	INY
	LDA     (ZP_SP),Y
	STA     UF_Y+1
	RTS

; Helper: return UF_A on success, $FF on error (storing UF_B in v_sys.LastErr)
.L_ret_a_or_ff:
	LDA     UF_CC
	AND     #1
	BNE     .L_ret_ff_err
	LDA     UF_A
	STA     ZP_R0
	LDA     #0
	STA     ZP_R0+1
	RTS
.L_ret_ff_err:
	LDA     UF_B
	STA     v_sys.LastErr
	LDA     #$FF
	STA     ZP_R0
	LDA     #0
	STA     ZP_R0+1
	RTS

; Helper: return UF_Y (16-bit) on success, 0 on error (storing UF_B in v_sys.LastErr)
.L_ret_y_or_0:
	LDA     UF_CC
	AND     #1
	BNE     .L_ret_0_err
	LDA     UF_Y
	STA     ZP_R0
	LDA     UF_Y+1
	STA     ZP_R0+1
	RTS
.L_ret_0_err:
	LDA     UF_B
	STA     v_sys.LastErr
	LDA     #0
	STA     ZP_R0
	STA     ZP_R0+1
	RTS

; Helper: return 0 on success, UF_B on error (storing UF_B in v_sys.LastErr)
.L_ret_0_or_b:
	LDA     UF_CC
	AND     #1
	BNE     .L_ret_b_err
	LDA     #0
	STA     ZP_R0
	STA     ZP_R0+1
	RTS
.L_ret_b_err:
	LDA     UF_B
	STA     v_sys.LastErr
	STA     ZP_R0
	LDA     #0
	STA     ZP_R0+1
	RTS

f_sys__SysOpen:
	JSR     .L_arg_x_a
	BRK
	DB      $84             ; I$Open
	JMP     .L_ret_a_or_ff

f_sys__SysCreate:
	JSR     .L_arg_x_a
	LDY     #4
	LDA     (ZP_SP),Y
	STA     UF_B
	BRK
	DB      $83             ; I$Create
	JMP     .L_ret_a_or_ff

f_sys__SysRead:
	JSR     .L_arg_a_x_y
	BRK
	DB      $89             ; I$Read
	JMP     .L_ret_y_or_0

f_sys__SysReadLn:
	JSR     .L_arg_a_x_y
	BRK
	DB      $8B             ; I$ReadLn
	JMP     .L_ret_y_or_0

f_sys__SysWrite:
	JSR     .L_arg_a_x_y
	BRK
	DB      $8A             ; I$Write
	JMP     .L_ret_y_or_0

f_sys__SysWritLn:
	JSR     .L_arg_a_x_y
	BRK
	DB      $8C             ; I$WritLn
	JMP     .L_ret_y_or_0

f_sys__SysClose:
	LDY     #0
	LDA     (ZP_SP),Y
	STA     UF_A
	BRK
	DB      $8F             ; I$Close
	JMP     .L_ret_0_or_b

f_sys__SysDup:
	LDY     #0
	LDA     (ZP_SP),Y
	STA     UF_A
	BRK
	DB      $82             ; I$Dup
	JMP     .L_ret_a_or_ff

f_sys__SysDelete:
	LDY     #0
	LDA     (ZP_SP),Y
	STA     UF_X
	INY
	LDA     (ZP_SP),Y
	STA     UF_X+1
	BRK
	DB      $87             ; I$Delete
	JMP     .L_ret_0_or_b

f_sys__SysFork:
	LDY     #0
	LDA     (ZP_SP),Y
	STA     UF_X
	INY
	LDA     (ZP_SP),Y
	STA     UF_X+1
	INY
	LDA     (ZP_SP),Y
	STA     UF_U
	INY
	LDA     (ZP_SP),Y
	STA     UF_U+1
	INY
	LDA     (ZP_SP),Y
	STA     UF_Y
	INY
	LDA     (ZP_SP),Y
	STA     UF_Y+1
	BRK
	DB      $03             ; F$Fork
	LDA     UF_CC
	AND     #1
	BNE     .L_ret_0_err
	LDA     UF_A
	STA     ZP_R0
	LDA     #0
	STA     ZP_R0+1
	RTS

f_sys__SysWait:
	BRK
	DB      $04             ; F$Wait
	LDA     UF_CC
	AND     #1
	BNE     .L_ret_0_err
	LDA     UF_B            ; low byte = exit status
	STA     ZP_R0
	LDA     UF_A            ; high byte = child PID
	STA     ZP_R0+1
	RTS

f_sys__SysChgDir:
	JSR     .L_arg_x_a
	BRK
	DB      $86             ; I$ChgDir
	JMP     .L_ret_0_or_b

f_sys__SysSeek:
	LDY     #0
	LDA     (ZP_SP),Y
	STA     UF_A
	LDA     #0
	STA     UF_X
	STA     UF_X+1
	LDY     #2
	LDA     (ZP_SP),Y
	STA     UF_U
	INY
	LDA     (ZP_SP),Y
	STA     UF_U+1
	BRK
	DB      $88             ; I$Seek
	JMP     .L_ret_0_or_b

	END     user_start
