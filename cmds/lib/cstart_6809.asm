; Hatvan OS User Process Startup Stub for M6809
; Standard DECB user binary entry at $0500

    pragma cescapes
    org $0500

start:
cstart:
    stx v_sys.ArgsPtr
    lbsr _main
    clrb
    swi2
    fcb $06 ; F$Exit

_printf:
    ldx 2,s
    cmpx #0
    beq .printf_arg1
    ldy #0
.printf_len:
    tst ,x+
    beq .printf_write
    leay 1,y
    bra .printf_len
.printf_write:
    cmpy #0
    beq .printf_arg1
    ldx 2,s
    lda #2 ; stderr
    swi2
    fcb $8A ; I$Write
.printf_arg1:
    ldx 4,s
    cmpx #0
    beq .printf_ret
    ldy #0
.printf_len2:
    tst ,x+
    beq .printf_write2
    leay 1,y
    bra .printf_len2
.printf_write2:
    cmpy #0
    beq .printf_ret
    ldx 4,s
    lda #2 ; stderr
    swi2
    fcb $8A ; I$Write
.printf_ret:
    rts

__exit:
    clrb
    swi2
    fcb $06 ; F$Exit

f_prelude__mul_byte:
    lda 2,s
    ldb 3,s
    mul
    tfr d,x
    rts

f_sys__SysOpen:
    ldx 2,s
    lda 4,s
    swi2
    fcb $84 ; I$Open
    bcs .open_err
    tfr a,b
    clra
    tfr d,x
    rts
.open_err:
    stb v_sys.LastErr
    ldb #$FF
    clra
    tfr d,x
    rts

f_sys__SysCreate:
    ldx 2,s
    lda 4,s
    ldb 5,s
    swi2
    fcb $83 ; I$Create
    bcs .create_err
    tfr a,b
    clra
    tfr d,x
    rts
.create_err:
    stb v_sys.LastErr
    ldb #$FF
    clra
    tfr d,x
    rts

f_sys__SysRead:
    lda 2,s
    ldx 3,s
    ldy 5,s
    swi2
    fcb $89 ; I$Read
    bcs .read_err
    tfr y,d
    tfr d,x
    rts
.read_err:
    stb v_sys.LastErr
    ldd #0
    ldx #0
    rts

f_sys__SysReadLn:
    lda 2,s
    ldx 3,s
    ldy 5,s
    swi2
    fcb $8B ; I$ReadLn
    bcs .readln_err
    tfr y,d
    tfr d,x
    rts
.readln_err:
    stb v_sys.LastErr
    ldd #0
    ldx #0
    rts

f_sys__SysWrite:
    lda 2,s
    ldx 3,s
    ldy 5,s
    swi2
    fcb $8A ; I$Write
    bcs .write_err
    tfr y,d
    tfr d,x
    rts
.write_err:
    stb v_sys.LastErr
    ldd #0
    ldx #0
    rts

f_sys__SysWritLn:
    lda 2,s
    ldx 3,s
    ldy 5,s
    swi2
    fcb $8C ; I$WritLn
    bcs .writln_err
    tfr y,d
    tfr d,x
    rts
.writln_err:
    stb v_sys.LastErr
    ldd #0
    ldx #0
    rts

f_sys__SysClose:
    lda 2,s
    swi2
    fcb $8F ; I$Close
    bcs .close_err
    clra
    clrb
    ldx #0
    rts
.close_err:
    stb v_sys.LastErr
    clra
    tfr d,x
    rts

f_sys__SysDup:
    lda 2,s
    swi2
    fcb $82 ; I$Dup
    bcs .dup_err
    tfr a,b
    clra
    tfr d,x
    rts
.dup_err:
    stb v_sys.LastErr
    ldb #$FF
    clra
    tfr d,x
    rts

f_sys__SysDelete:
    ldx 2,s
    swi2
    fcb $87 ; I$Delete
    bcs .del_err
    clra
    clrb
    ldx #0
    rts
.del_err:
    stb v_sys.LastErr
    clra
    tfr d,x
    rts

f_sys__SysExit:
    ldb 2,s
    swi2
    fcb $06 ; F$Exit
.exit_loop:
    bra .exit_loop

f_sys__SysFork:
    pshs    u,y
    ldx     6,s         ; cmd
    ldu     8,s         ; params
    ldy     10,s        ; paramLen
    clra
    clrb
    swi2
    fcb     $03         ; F$Fork
    bcs     .fork_err
    tfr     a,b
    clra
    tfr     d,x
    puls    u,y,pc
.fork_err:
    stb     v_sys.LastErr
    clra
    clrb
    tfr     d,x
    puls    u,y,pc

f_sys__SysWait:
    swi2
    fcb     $04         ; F$Wait
    bcs     .wait_err
    tfr     d,x
    rts
.wait_err:
    stb     v_sys.LastErr
    clra
    clrb
    tfr     d,x
    rts

f_sys__SysChgDir:
    ldx     2,s
    lda     4,s
    swi2
    fcb     $86         ; I$ChgDir
    bcs     .chgdir_err
    clra
    clrb
    tfr     d,x
    rts
.chgdir_err:
    stb     v_sys.LastErr
    clra
    tfr     d,x
    rts

