; Hatvan OS User Process Startup Stub & Syscall Wrappers for Intel 8086
; Prepended before MiniGolf-generated 8086 assembly (Small Split I&D: CS=$1000, DS=SS=ES=$0000).

UF_CC   EQU $0020
UF_A    EQU $0021
UF_B    EQU $0022
UF_DP   EQU $0023
UF_X    EQU $0024       ; $0024..$0025: 16-bit X register in SyscallFrame
UF_Y    EQU $0026       ; $0026..$0027: 16-bit Y register in SyscallFrame
UF_U    EQU $0028       ; $0028..$0029: 16-bit U register in SyscallFrame
ZP_ARGS EQU $002A       ; $002A..$002B: initial argsPtr placed by hal.SetupChildTask

	SEGMENT CODE
	ORG     $0200

user_start:
	CLD
	XOR     AX, AX
	MOV     DS, AX
	MOV     SS, AX
	MOV     ES, AX
	MOV     BP, SP

	; Initialize v_sys.ArgsPtr from ZP_ARGS ($002A..$002B)
	MOV     AX, [ZP_ARGS]
	MOV     [v_sys.ArgsPtr], AX

	STI
	CALL    _main

	MOV     BYTE PTR [UF_B], 0
	INT     $30
	DB      $06             ; F$Exit
.L_user_hang:
	JMP     .L_user_hang

_printf:
	PUSH    BP
	MOV     BP, SP
	SUB     SP, 128
	PUSH    SI
	PUSH    DI

	MOV     SI, [BP+4]
	TEST    SI, SI
	JZ      .L_upf_done
	LEA     DI, [BP-128]
	XOR     CX, CX
.L_upf_loop:
	CMP     CX, 126
	JAE     .L_upf_flush
	LODSB
	TEST    AL, AL
	JZ      .L_upf_flush
	CMP     AL, '%'
	JNE     .L_upf_put
	LODSB
	TEST    AL, AL
	JZ      .L_upf_flush
	CMP     AL, 's'
	JNE     .L_upf_put
	MOV     BX, [BP+6]
	TEST    BX, BX
	JZ      .L_upf_loop
.L_upf_s:
	CMP     CX, 126
	JAE     .L_upf_flush
	MOV     AL, [BX]
	TEST    AL, AL
	JZ      .L_upf_loop
	MOV     [DI], AL
	INC     DI
	INC     CX
	INC     BX
	JMP     .L_upf_s
.L_upf_put:
	MOV     [DI], AL
	INC     DI
	INC     CX
	JMP     .L_upf_loop
.L_upf_flush:
	TEST    CX, CX
	JZ      .L_upf_done
	LEA     AX, [BP-128]
	MOV     BYTE PTR [UF_A], 2
	MOV     [UF_X], AX
	MOV     [UF_Y], CX
	INT     $30
	DB      $8A             ; I$Write
.L_upf_done:
	POP     DI
	POP     SI
	MOV     SP, BP
	POP     BP
	RET

__exit:
_exit:
f_sys__SysExit:
	MOV     BX, SP
	MOV     AL, [BX+2]
	MOV     [UF_B], AL
	INT     $30
	DB      $06             ; F$Exit
.L_exit_hang:
	JMP     .L_exit_hang

; Helper: load arg0 (word) into UF_X, arg1 (byte) into UF_A
; Note: called via CALL, so caller's SP had [ret_ip, arg0, arg1]; inside helper [BX+4]=arg0, [BX+6]=arg1
.L_arg_x_a:
	MOV     BX, SP
	MOV     AX, [BX+4]
	MOV     [UF_X], AX
	MOV     AL, [BX+6]
	MOV     [UF_A], AL
	RET

; Helper: load arg0 (byte) into UF_A, arg1 (word) into UF_X, arg2 (word) into UF_Y
.L_arg_a_x_y:
	MOV     BX, SP
	MOV     AL, [BX+4]
	MOV     [UF_A], AL
	MOV     AX, [BX+6]
	MOV     [UF_X], AX
	MOV     AX, [BX+8]
	MOV     [UF_Y], AX
	RET

; Helper: return UF_A on success, $FF on error (storing UF_B in v_sys.LastErr)
.L_ret_a_or_ff:
	TEST    BYTE PTR [UF_CC], 1
	JNZ     .L_ret_ff_err
	XOR     AH, AH
	MOV     AL, [UF_A]
	RET
.L_ret_ff_err:
	MOV     AL, [UF_B]
	MOV     [v_sys.LastErr], AL
	MOV     AX, $00FF
	RET

; Helper: return UF_Y (16-bit) on success, 0 on error (storing UF_B in v_sys.LastErr)
.L_ret_y_or_0:
	TEST    BYTE PTR [UF_CC], 1
	JNZ     .L_ret_0_err
	MOV     AX, [UF_Y]
	RET
.L_ret_0_err:
	MOV     AL, [UF_B]
	MOV     [v_sys.LastErr], AL
	XOR     AX, AX
	RET

; Helper: return 0 on success, UF_B on error (storing UF_B in v_sys.LastErr)
.L_ret_0_or_b:
	TEST    BYTE PTR [UF_CC], 1
	JNZ     .L_ret_b_err
	XOR     AX, AX
	RET
.L_ret_b_err:
	XOR     AH, AH
	MOV     AL, [UF_B]
	MOV     [v_sys.LastErr], AL
	RET

f_sys__SysOpen:
	CALL    .L_arg_x_a
	INT     $30
	DB      $84             ; I$Open
	JMP     .L_ret_a_or_ff

f_sys__SysCreate:
	CALL    .L_arg_x_a
	MOV     BX, SP
	MOV     AL, [BX+6]
	MOV     [UF_B], AL
	INT     $30
	DB      $83             ; I$Create
	JMP     .L_ret_a_or_ff

f_sys__SysRead:
	CALL    .L_arg_a_x_y
	INT     $30
	DB      $89             ; I$Read
	JMP     .L_ret_y_or_0

f_sys__SysReadLn:
	CALL    .L_arg_a_x_y
	INT     $30
	DB      $8B             ; I$ReadLn
	JMP     .L_ret_y_or_0

f_sys__SysWrite:
	CALL    .L_arg_a_x_y
	INT     $30
	DB      $8A             ; I$Write
	JMP     .L_ret_y_or_0

f_sys__SysWritLn:
	CALL    .L_arg_a_x_y
	INT     $30
	DB      $8C             ; I$WritLn
	JMP     .L_ret_y_or_0

f_sys__SysClose:
	MOV     BX, SP
	MOV     AL, [BX+2]
	MOV     [UF_A], AL
	INT     $30
	DB      $8F             ; I$Close
	JMP     .L_ret_0_or_b

f_sys__SysDup:
	MOV     BX, SP
	MOV     AL, [BX+2]
	MOV     [UF_A], AL
	INT     $30
	DB      $82             ; I$Dup
	JMP     .L_ret_a_or_ff

f_sys__SysDelete:
	MOV     BX, SP
	MOV     AX, [BX+2]
	MOV     [UF_X], AX
	INT     $30
	DB      $87             ; I$Delete
	JMP     .L_ret_0_or_b

f_sys__SysFork:
	MOV     BX, SP
	MOV     AX, [BX+2]      ; cmd
	MOV     [UF_X], AX
	MOV     AX, [BX+4]      ; params
	MOV     [UF_U], AX
	MOV     AX, [BX+6]      ; paramLen
	MOV     [UF_Y], AX
	INT     $30
	DB      $03             ; F$Fork
	TEST    BYTE PTR [UF_CC], 1
	JNZ     .L_ret_0_err
	XOR     AH, AH
	MOV     AL, [UF_A]
	RET

f_sys__SysWait:
	INT     $30
	DB      $04             ; F$Wait
	TEST    BYTE PTR [UF_CC], 1
	JNZ     .L_ret_0_err
	MOV     AH, [UF_A]      ; high byte = child PID
	MOV     AL, [UF_B]      ; low byte = exit status
	RET

f_sys__SysChgDir:
	CALL    .L_arg_x_a
	INT     $30
	DB      $86             ; I$ChgDir
	JMP     .L_ret_0_or_b

f_sys__SysSeek:
	MOV     BX, SP
	MOV     AL, [BX+2]      ; path
	MOV     [UF_A], AL
	MOV     WORD PTR [UF_X], 0
	MOV     AX, [BX+4]      ; pos
	MOV     [UF_U], AX
	INT     $30
	DB      $88             ; I$Seek
	JMP     .L_ret_0_or_b
