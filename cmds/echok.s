; Echo command for Hatvan OS / M68K
; Reads parameter string at (A0), prints via I$WritLn, and exits with status 0.
    org $00000200

_start:
    ; Skip leading spaces
skip_sp:
    move.b  (a0)+, d0
    cmp.b   #$20, d0
    beq     skip_sp
    subq.l  #1, a0

    ; Find length of parameter string until CR ($0D), LF ($0A), or NUL (0)
    move.l  a0, a1
    moveq   #0, d2
find_len:
    move.b  (a1)+, d0
    cmp.b   #$0D, d0
    beq     found_end
    cmp.b   #$0A, d0
    beq     found_end
    tst.b   d0
    beq     found_end
    addq.l  #1, d2
    bra     find_len

found_end:
    ; D2 holds length of string to print

    ; Call I$WritLn ($8C)
    ; D0 = $8C
    ; D1 = 1 (Path 1: stdout)
    ; A0 = buffer pointer
    ; D2 = byte count
    moveq   #1, d1
    move.l  #$8C, d0
    trap    #0

    ; Call F$Exit ($06) with status 0
    ; D0 = $06
    ; D1 = 0
    moveq   #0, d1
    move.l  #$06, d0
    trap    #0

    end _start
