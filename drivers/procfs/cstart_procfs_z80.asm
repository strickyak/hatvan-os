; Hatvan OS PROCFS Driver Task Startup Stub for Z80 (Task 2)
; Resides at $4000 in Task 2 memory.
; Stack starts at $3F00.

    org 0x4000

start:
cstart:
    ld   sp, 0x3F00
    call _main
hang:
    jr   hang

_printf:
    ret

_exit:
__exit:
    jr   hang

f_hal__Sleep:
    out  (0x60), a
    defb 0x0A
    ret
