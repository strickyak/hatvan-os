; Hatvan OS Kernel Startup Stub & Vector Table for Zilog Z80
; Resides in Task 0 memory space.
; Hardware vectors at $0000 (Reset), $0038 (IM 1 IRQ), $0066 (NMI trap).
; Kernel code starts at $1000 with private kernel stack in $0200..$1000.

    org 0x0000
    jp  cstart

    org 0x0038
    jp  trap_irq

    org 0x0066
    jp  trap_nmi

    org 0x0100
start:
cstart:
    ld   sp, 0xE000
    im   1
    call _main
hang:
    jr   hang

_printf:
    ret

_exit:
__exit:
    ld   a, l
    ld   (0xFF05), a
.exit_hang:
    jr   .exit_hang
