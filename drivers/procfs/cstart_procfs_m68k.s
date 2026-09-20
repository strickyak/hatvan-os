; Hatvan OS PROCFS Driver Task Startup Stub for Motorola 68000 (Task 2)
    org $00001000

cstart:
    lea     $0007FE00, sp
    jsr     _main
hang:
    bra     hang

_printf:
    rts

_exit:
    bra     _exit

f_hal__Sleep:
    move.l  d2, -(sp)
    move.l  d3, -(sp)
    move.l  d4, -(sp)
    move.l  d5, -(sp)
    move.l  d6, -(sp)
    move.l  d7, -(sp)
    move.l  a2, -(sp)
    move.l  a3, -(sp)
    move.l  a4, -(sp)
    move.l  a5, -(sp)
    move.l  a6, -(sp)

    move.l  #10, d0
    dc.w    $4E40

    move.l  (sp)+, a6
    move.l  (sp)+, a5
    move.l  (sp)+, a4
    move.l  (sp)+, a3
    move.l  (sp)+, a2
    move.l  (sp)+, d7
    move.l  (sp)+, d6
    move.l  (sp)+, d5
    move.l  (sp)+, d4
    move.l  (sp)+, d3
    move.l  (sp)+, d2
    rts
