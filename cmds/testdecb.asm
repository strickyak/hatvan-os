 nam testdecb
 ttl Test DECB binary for Hatvan OS
 pragma cescapes
 org $0500
start:
 lda #'D'
 sta $FF00
 lda #'E'
 sta $FF00
 lda #'C'
 sta $FF00
 lda #'B'
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
 end start
