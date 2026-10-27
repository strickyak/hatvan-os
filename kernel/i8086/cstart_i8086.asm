; ══════════════════════════════════════════════════════════════════════════════
; Hatvan OS Kernel Startup for Intel 8086 (Task 0, Small Model CS=$1000, DS=$0000)
; ══════════════════════════════════════════════════════════════════════════════

	SEGMENT CODE
	ORG     $0200

kernel_reset:
	CLI
	CLD

	; Initialize Data, Stack, and Extra segments to $0000 ($00000..$0FFFF)
	XOR     AX, AX
	MOV     DS, AX
	MOV     SS, AX
	MOV     ES, AX

	; Task 0 hardware stack starts at $0E000 (just below Shared Curtain $0E000..$0FEFF)
	MOV     SP, $E000
	MOV     BP, SP

	; Install 8086 Interrupt Vector Table (IVT) entries in Task 0 at $0000..$03FF:
	; Vector 08h ($0020..$0023): Hardware Timer / Terminal IRQ -> 1000:trap_hw_irq
	MOV     WORD PTR [$0020], trap_hw_irq
	MOV     WORD PTR [$0022], $1000

	; Vector 30h ($00C0..$00C3): Software Syscall / Driver Sleep Trap -> 1000:trap_syscall
	MOV     WORD PTR [$00C0], trap_syscall
	MOV     WORD PTR [$00C2], $1000

	MOV     BYTE PTR [in_kernel], 1

	; Jump into MiniGolf kernel main
	CALL    f_main__main

	; Halt VM on return
	MOV     BYTE PTR [$FF05], 0
.L_kernel_halt:
	HLT
	JMP     .L_kernel_halt

; ── Runtime Support for MiniGolf _panic / _printf / _exit ────────────────────
_printf:
	PUSH    BP
	MOV     BP, SP
	PUSH    SI
	PUSH    BX

	MOV     SI, [BP+4]
	TEST    SI, SI
	JZ      .L_kpf_done
.L_kpf_loop:
	LODSB
	TEST    AL, AL
	JZ      .L_kpf_done
	CMP     AL, '%'
	JNE     .L_kpf_out
	LODSB
	TEST    AL, AL
	JZ      .L_kpf_done
	CMP     AL, 's'
	JNE     .L_kpf_out
	MOV     BX, [BP+6]
	TEST    BX, BX
	JZ      .L_kpf_loop
.L_kpf_str:
	MOV     AL, [BX]
	TEST    AL, AL
	JZ      .L_kpf_loop
	MOV     [$FF04], AL
	INC     BX
	JMP     .L_kpf_str
.L_kpf_out:
	MOV     [$FF04], AL
	JMP     .L_kpf_loop
.L_kpf_done:
	POP     BX
	POP     SI
	POP     BP
	RET

_exit:
__exit:
	MOV     BP, SP
	MOV     AL, [BP+2]
	MOV     [$FF05], AL
.L_kexit_loop:
	HLT
	JMP     .L_kexit_loop
