; Hatvan OS User Process Startup Stub for Motorola 68000
    org $00000200

start:
cstart:
    move.l  a0, v_sys.ArgsPtr
    jsr     _main
    clr.l   d1
    move.l  #$06, d0
    trap    #0

_printf:
    rts

_exit:
    clr.l   d1
    move.l  #$06, d0
    trap    #0

f_sys__SysOpen:
    move.l  4(sp), a0
    move.l  8(sp), d1
    move.l  #$84, d0
    trap    #0
    bcs     .open_err_k
    move.l  d1, d0
    rts
.open_err_k:
    move.b  d1, v_sys.LastErr
    moveq   #-1, d0
    rts

f_sys__SysCreate:
    move.l  4(sp), a0
    move.l  8(sp), d1
    move.l  12(sp), d2
    move.l  #$83, d0
    trap    #0
    bcs     .create_err_k
    move.l  d1, d0
    rts
.create_err_k:
    move.b  d1, v_sys.LastErr
    moveq   #-1, d0
    rts

f_sys__SysRead:
    move.l  4(sp), d1
    move.l  8(sp), a0
    move.l  12(sp), d2
    move.l  #$89, d0
    trap    #0
    bcs     .read_err_k
    move.l  d2, d0
    rts
.read_err_k:
    move.b  d1, v_sys.LastErr
    moveq   #0, d0
    rts

f_sys__SysReadLn:
    move.l  4(sp), d1
    move.l  8(sp), a0
    move.l  12(sp), d2
    move.l  #$8B, d0
    trap    #0
    bcs     .readln_err_k
    move.l  d2, d0
    rts
.readln_err_k:
    move.b  d1, v_sys.LastErr
    moveq   #0, d0
    rts

f_sys__SysWrite:
    move.l  4(sp), d1
    move.l  8(sp), a0
    move.l  12(sp), d2
    move.l  #$8A, d0
    trap    #0
    bcs     .write_err_k
    move.l  d2, d0
    rts
.write_err_k:
    move.b  d1, v_sys.LastErr
    moveq   #0, d0
    rts

f_sys__SysWritLn:
    move.l  4(sp), d1
    move.l  8(sp), a0
    move.l  12(sp), d2
    move.l  #$8C, d0
    trap    #0
    bcs     .writln_err_k
    move.l  d2, d0
    rts
.writln_err_k:
    move.b  d1, v_sys.LastErr
    moveq   #0, d0
    rts

f_sys__SysClose:
    move.l  4(sp), d1
    move.l  #$8F, d0
    trap    #0
    bcs     .close_err_k
    moveq   #0, d0
    rts
.close_err_k:
    move.b  d1, v_sys.LastErr
    move.l  d1, d0
    rts

f_sys__SysExit:
    move.l  4(sp), d1
    move.l  #$06, d0
    trap    #0
.exit_loop_k:
    bra     .exit_loop_k

f_sys__SysFork:
    move.l  4(sp), a0
    move.l  8(sp), a1
    move.l  12(sp), d2
    move.l  #$03, d0
    trap    #0
    bcs     .fork_err_k
    move.b  d1, d0
    and.l   #$FF, d0
    rts
.fork_err_k:
    move.b  d1, v_sys.LastErr
    moveq   #0, d0
    rts

f_sys__SysWait:
    move.l  #$04, d0
    trap    #0
    bcs     .wait_err_k
    and.l   #$FF, d1
    lsl.w   #8, d1
    and.l   #$FF, d0
    or.w    d1, d0
    rts
.wait_err_k:
    move.b  d0, v_sys.LastErr
    moveq   #0, d0
    rts

f_sys__SysChgDir:
    move.l  4(sp), a0
    move.l  8(sp), d1
    move.l  #$86, d0
    trap    #0
    bcs     .chgdir_err_k
    moveq   #0, d0
    rts
.chgdir_err_k:
    move.b  d1, v_sys.LastErr
    move.b  d1, d0
    and.l   #$FF, d0
    rts

