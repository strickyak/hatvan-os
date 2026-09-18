; Hatvan OS Kernel Startup Stub for M6809
; Places kernel code at $4000, reserving $0010..$1FFF for BSS/globals
; and $2000..$3800 for the kernel stack.

    pragma cescapes
    org $4000

cstart:
    lds #$3800
    lbsr _main
hang:
    bra hang

_printf:
    rts

__exit:
    stb $FF05
    bra hang

f_prelude__mul_byte:
    lda 2,s
    ldb 3,s
    mul
    tfr d,x
    rts

    end cstart
