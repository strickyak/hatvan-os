; Hatvan OS M6809 Hardware Vector Table ($FFF0..$FFFF)
    pragma cescapes
    org $FFF0

vectors:
    fdb trap_reserved ; $FFF0
    fdb trap_swi3     ; $FFF2
    fdb trap_swi2     ; $FFF4 (OS-9 system call trap)
    fdb trap_firq     ; $FFF6
    fdb trap_irq      ; $FFF8
    fdb trap_swi      ; $FFFA
    fdb trap_nmi      ; $FFFC
    fdb cstart        ; $FFFE (RESET vector)

    end cstart
