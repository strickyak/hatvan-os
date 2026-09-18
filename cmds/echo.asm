 nam echo
 ttl Echo command for Hatvan OS

Type_Lang equ $11 ; Program + 6809 Machine Code
Attr_Rev  equ $81 ; ReEnt + Rev 1
Edition   equ 1

 mod eom,name,Type_Lang,Attr_Rev,start,size

 org 0
stack rmb 200
size equ .

name fcs /echo/
 fcb Edition

start:
 ; Skip leading spaces in parameter string
skip_sp:
 cmpy #0
 beq do_nl
 lda ,x
 cmpa #13        ; CR?
 beq do_nl
 cmpa #' '
 bne do_print
 leax 1,x
 leay -1,y
 bra skip_sp

do_nl:
 leax nl_msg,pcr
 ldy #1
 lda #1          ; Path 1: stdout
 swi2
 fcb $8C         ; I$WritLn
 bra do_exit

do_print:
 lda #1          ; Path 1: stdout
 swi2
 fcb $8C         ; I$WritLn

do_exit:
 clrb            ; Exit status 0
 swi2
 fcb $06         ; F$Exit

nl_msg:
 fcb 13

 emod
eom equ *
 end
