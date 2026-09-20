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

f_sys__SysExit:
    ldb 2,s
    swi2
    fcb $06 ; F$Exit
.exit_loop:
    bra .exit_loop
