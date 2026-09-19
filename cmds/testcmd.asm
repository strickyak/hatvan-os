 nam testcmd
 ttl Test OS-9 command for Hatvan OS

Type_Lang equ $11 ; Program + 6809 Machine Code
Attr_Rev  equ $81 ; ReEnt + Rev 1
Edition   equ 1

 mod eom,name,Type_Lang,Attr_Rev,start,200

name fcs /testcmd/
 fcb Edition

start equ *
 lda #'C'
 sta $FF00
 lda #'M'
 sta $FF00
 lda #'D'
 sta $FF00
 lda #':'
 sta $FF00
 lda #'O'
 sta $FF00
 lda #'K'
 sta $FF00
 lda #10
 sta $FF00
 rts

 emod
eom equ *
 end
