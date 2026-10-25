; Hatvan OS RBF Driver Task Startup & Sleep Stub for MOS 6502 (Task 1)

_printf:
	RTS

_exit:
__exit:
.L_rbf_hang:
	JMP     .L_rbf_hang

f_hal__Sleep:
	BRK
	DB      $0A
	RTS
