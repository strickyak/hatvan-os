 nam cat
 ttl Cat command for Hatvan OS (OS-9)

Type_Lang equ $11 ; Program + 6809 Machine Code
Attr_Rev  equ $81 ; ReEnt + Rev 1
Edition   equ 1

 mod eom,name,Type_Lang,Attr_Rev,start,size

 org 0
path_num rmb 1
line_buf rmb 128
path_buf rmb 64
stack    rmb 200
size equ .

name fcs /cat/
 fcb Edition

start:
 ; skip leading spaces in parameter string
skip_sp:
 cmpd #0
 beq do_err
 lda ,x
 cmpa #13
 beq do_err
 cmpa #' '
 bne copy_p
 leax 1,x
 subd #1
 bra skip_sp

copy_p:
 leay path_buf,u
copy_loop:
 cmpd #0
 beq open_it
 lda ,x+
 cmpa #' '
 beq open_it
 cmpa #13
 beq open_it
 cmpa #10
 beq open_it
 sta ,y+
 subd #1
 bra copy_loop

open_it:
 lda #13 ; terminate path with CR
 sta ,y
 leax path_buf,u
 lda #1  ; MODE_READ
 swi2
 fcb $84 ; I$Open
 bcs do_err

 sta path_num,u

read_loop:
 lda path_num,u
 leax line_buf,u
 ldy #128
 swi2
 fcb $8B ; I$ReadLn
 bcs read_eof

 ; write line to stdout (path 1)
 lda #1
 leax line_buf,u
 swi2
 fcb $8C ; I$WritLn
 bra read_loop

read_eof:
 lda path_num,u
 swi2
 fcb $8F ; I$Close

 clrb
 swi2
 fcb $06 ; F$Exit

do_err:
 ldb #1
 swi2
 fcb $06 ; F$Exit

 emod
eom equ *
 end
