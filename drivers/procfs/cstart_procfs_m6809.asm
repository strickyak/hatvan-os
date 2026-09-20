; Hatvan OS PROCFS Driver Task Startup Stub for M6809 (Task 2)
; Resides at $4000 in Task 2 memory.
; Stack starts at $FE00 (below Red Page $FF00).

    pragma cescapes
    org $4000

cstart:
    lds #$FE00
    clra
    tfr a,dp
    lbsr _main
hang:
    bra hang

_printf:
    rts

__exit:
    bra hang

f_prelude__mul_byte:
    lda 2,s
    ldb 3,s
    mul
    tfr d,x
    rts

f_hal__Sleep:
    swi2
    fcb $0A
    rts
