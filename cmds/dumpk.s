; Hex dump command for Hatvan OS / M68K
; Dumps file specified in argument: offset followed by 16 hex bytes per line
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
    moveq   #0, d6          ; D6 = file offset (32-bit, starts at 0)

read_loop:
    move.l  d7, d1          ; Path ID
    lea     data_buf, a0    ; 16-byte buffer
    move.l  #16, d2         ; count = 16
    move.l  #$89, d0        ; I$Read
    dc.w    $4E40           ; trap #0
    bcs     read_done       ; EOF or error
    tst.l   d2
    beq     read_done       ; 0 bytes read -> EOF

    move.l  d2, d4          ; D4 = bytes read (1..16)

    ; Format output line in line_buf:
    ; "00000000: 87 CD 00 49 ..."
    lea     line_buf, a1

    ; 1. Format offset (D6.L)
    move.l  d6, d0
    bsr     hex_long

    ; 2. Add separator ": "
    move.b  #':', (a1)+
    move.b  #' ', (a1)+

    ; 3. Format D4 bytes
    lea     data_buf, a2
    move.l  d4, d5          ; D5 = loop counter
format_bytes:
    move.b  (a2)+, d0
    bsr     hex_byte
    move.b  #' ', (a1)+
    subq.l  #1, d5
    bne     format_bytes

    ; 4. Calculate line length: A1 - line_buf
    lea     line_buf, a0
    move.l  a1, d2
    sub.l   a0, d2          ; D2 = line byte count

    ; 5. Print line via I$WritLn ($8C)
    moveq   #1, d1          ; stdout
    move.l  #$8C, d0        ; I$WritLn
    dc.w    $4E40           ; trap #0

    ; 6. Update file offset
    add.l   d4, d6

    ; If we read fewer than 16 bytes, we reached EOF
    cmp.l   #16, d4
    blt     read_done

    bra     read_loop

read_done:
    ; Close file
    move.l  d7, d1
    move.l  #$8F, d0        ; I$Close
    dc.w    $4E40

    ; Exit status 0
    moveq   #0, d1
    move.l  #$06, d0        ; F$Exit
    dc.w    $4E40

open_err:
    lea     msg_open_err, a0
    moveq   #23, d2
    moveq   #1, d1
    move.l  #$8C, d0
    dc.w    $4E40

    moveq   #1, d1
    move.l  #$06, d0
    dc.w    $4E40

usage_err:
    lea     msg_usage, a0
    moveq   #19, d2
    moveq   #1, d1
    move.l  #$8C, d0
    dc.w    $4E40

    moveq   #1, d1
    move.l  #$06, d0
    dc.w    $4E40

; Helper: hex_long (D0.L -> 8 ASCII chars at (A1)+)
hex_long:
    moveq   #7, d3
.hl_loop:
    rol.l   #4, d0
    move.b  d0, d1
    and.b   #$0F, d1
    cmp.b   #9, d1
    bgt     .hl_af
    add.b   #'0', d1
    bra     .hl_st
.hl_af:
    add.b   #'A'-10, d1
.hl_st:
    move.b  d1, (a1)+
    dbra    d3, .hl_loop
    rts

; Helper: hex_byte (D0.B -> 2 ASCII chars at (A1)+)
hex_byte:
    rol.b   #4, d0
    move.b  d0, d1
    and.b   #$0F, d1
    cmp.b   #9, d1
    bgt     .hb1_af
    add.b   #'0', d1
    bra     .hb1_st
.hb1_af:
    add.b   #'A'-10, d1
.hb1_st:
    move.b  d1, (a1)+

    rol.b   #4, d0
    move.b  d0, d1
    and.b   #$0F, d1
    cmp.b   #9, d1
    bgt     .hb2_af
    add.b   #'0', d1
    bra     .hb2_st
.hb2_af:
    add.b   #'A'-10, d1
.hb2_st:
    move.b  d1, (a1)+
    rts

msg_open_err:
    dc.b    "dumpk: cannot open file", 0

msg_usage:
    dc.b    "Usage: dumpk <file>", 0

    even
path_buf:
    ds.b    128

    even
data_buf:
    ds.b    16

    even
line_buf:
    ds.b    80

    end _start
