; Cat command for Hatvan OS / M68K
; Reads file specified in argument line by line and prints to stdout
    org $00000200

_start:
    ; Skip leading spaces
skip_sp:
    move.b  (a0)+, d0
    cmp.b   #' ', d0
    beq     skip_sp
    cmp.b   #',', d0
    beq     skip_sp
    subq.l  #1, a0

    ; Copy path parameter to path_buf
    lea     path_buf, a1
    moveq   #0, d5          ; D5 = length counter
copy_path:
    move.b  (a0)+, d0
    cmp.b   #' ', d0
    beq     path_done
    cmp.b   #',', d0
    beq     path_done
    cmp.b   #$0D, d0        ; CR
    beq     path_done
    cmp.b   #$0A, d0        ; LF
    beq     path_done
    tst.b   d0              ; NUL
    beq     path_done
    move.b  d0, (a1)+
    addq.l  #1, d5
    bra     copy_path
path_done:
    clr.b   (a1)            ; null terminate

    ; If no path argument was provided, print usage
    tst.l   d5
    beq     usage_err

    ; Open file via I$Open ($84) with MODE_READ (1)
    moveq   #1, d1          ; Mode = MODE_READ ($01)
    lea     path_buf, a0
    move.l  #$84, d0        ; I$Open
    dc.w    $4E40           ; trap #0
    bcs     open_err

    move.l  d1, d7          ; D7 = file Path ID

read_loop:
    move.l  d7, d1          ; Path ID
    lea     line_buf, a0    ; Line buffer
    move.l  #128, d2        ; maxLen = 128
    move.l  #$8B, d0        ; I$ReadLn
    dc.w    $4E40           ; trap #0
    bcs     read_done       ; EOF or error
    tst.l   d2
    beq     read_done       ; 0 bytes read -> EOF

    ; Write line to stdout (Path 1) via I$WritLn ($8C)
    move.l  d2, d4          ; D4 = bytes read
    moveq   #1, d1          ; Path 1 (stdout)
    lea     line_buf, a0
    move.l  d4, d2          ; count
    move.l  #$8C, d0        ; I$WritLn
    dc.w    $4E40           ; trap #0

    bra     read_loop

read_done:
    ; Close file via I$Close ($8F)
    move.l  d7, d1
    move.l  #$8F, d0        ; I$Close
    dc.w    $4E40           ; trap #0

    ; Exit success
    clr.l   d1
    move.l  #$06, d0        ; F$Exit
    dc.w    $4E40
    rts

usage_err:
    lea     msg_usage, a0
    move.l  #17, d2
    bra     print_err_and_exit

open_err:
    lea     msg_openerr, a0
    move.l  #16, d2
    bra     print_err_and_exit

print_err_and_exit:
    moveq   #2, d1          ; Path 2 (stderr)
    move.l  #$8C, d0        ; I$WritLn
    dc.w    $4E40
    moveq   #1, d1          ; exit code 1
    move.l  #$06, d0        ; F$Exit
    dc.w    $4E40
    rts

msg_usage:
    dc.b    "Usage: CAT <file>", 10, 0
msg_openerr:
    dc.b    "CAT: cannot open", 10, 0

    even
path_buf:
    ds.b    128
line_buf:
    ds.b    128

    end _start
