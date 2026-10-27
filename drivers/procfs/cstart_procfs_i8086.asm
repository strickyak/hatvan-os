; Hatvan OS PROCFS Driver Task Startup & Sleep Stub for Intel 8086 (Task 2)
; Loaded into Code Segment CS=$1000 at ORG $0200; Data/Stack Segment DS=SS=ES=$0000.

	SEGMENT CODE
	ORG     $0200

driver_start:
	CLD
	XOR     AX, AX
	MOV     DS, AX
	MOV     SS, AX
	MOV     ES, AX
	MOV     SP, $DFA6
	MOV     BP, SP
	CALL    f_main__main
.L_procfs_hang:
	JMP     .L_procfs_hang

_printf:
	RET

_exit:
__exit:
	JMP     .L_procfs_hang

f_hal__Sleep:
	INT     $30
	DB      $0A
	RET
