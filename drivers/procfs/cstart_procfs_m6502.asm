; Hatvan OS PROCFS Driver Task Startup & Sleep Stub for MOS 6502 (Task 2)

_printf:
	RTS

_exit:
__exit:
.L_procfs_hang:
	JMP     .L_procfs_hang

f_hal__Sleep:
	BRK
	DB      $0A
	RTS
