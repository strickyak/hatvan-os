; Directory listing command for Hatvan OS / M68K
; Lists files in current working directory (like /bin/ls .)
    org $00000200

_start:
    ; Open current directory "." with mode $81 (MODE_READ | MODE_DIR)
    moveq   #0, d1
    move.b  #$81, d1        ; Mode = MODE_READ | MODE_DIR ($81)
    lea     path_dot, a0    ; Path = "."
    move.l  #$84, d0        ; I$Open
    trap    #0
    bcs     open_err

    move.l  d1, d7          ; D7 = directory Path ID

read_loop:
    ; Read one 32-byte directory entry via I$Read ($89)
    move.l  d7, d1          ; Path ID
    lea     dirent_buf, a0  ; 32-byte buffer
    moveq   #32, d2         ; count = 32
    move.l  #$89, d0        ; I$Read
    trap    #0
    bcs     read_done       ; EOF or error
    cmp.l   #32, d2
    blt     read_done

    ; Check entry validity
    lea     dirent_buf, a0
    move.b  (a0), d0
    beq     read_loop       ; empty slot (0)
    cmp.b   #$E5, d0        ; deleted slot ($E5)
    beq     read_loop
    and.b   #$7F, d0        ; mask high bit (handles both '.' and '..')
    cmp.b   #'.', d0        ; dot entry (. or .. or hidden)
    beq     read_loop

    ; Extract filename (up to 29 bytes; last byte has bit 7 set)
    lea     name_buf, a1
    moveq   #0, d2          ; d2 = length counter
    moveq   #28, d3         ; d3 = max 29 chars (0..28)
extract_loop:
    move.b  (a0)+, d0
    move.b  d0, d1
    and.b   #$7F, d1        ; clear bit 7 for ASCII
    move.b  d1, (a1)+
    addq.l  #1, d2
    tst.b   d0              ; was bit 7 set?
    bmi     print_entry     ; bit 7 set indicates last char of name
    dbra    d3, extract_loop

print_entry:
    ; Append newline to name_buf
    move.b  #10, (a1)+
    addq.l  #1, d2

    ; Print filename via I$WritLn ($8C)
    ; D0 = $8C, D1 = 1 (stdout), A0 = name_buf, D2 = length
    lea     name_buf, a0
    moveq   #1, d1          ; stdout
    move.l  #$8C, d0        ; I$WritLn
    trap    #0

    bra     read_loop

read_done:
    ; Close directory path
    move.l  d7, d1
    move.l  #$8F, d0        ; I$Close
    trap    #0

    ; Exit with status 0
    moveq   #0, d1
    move.l  #$06, d0        ; F$Exit
    trap    #0

open_err:
    lea     err_msg, a0
    moveq   #16, d2
    moveq   #1, d1
    move.l  #$8C, d0
    trap    #0

    moveq   #1, d1
    move.l  #$06, d0
    trap    #0

path_dot:
    dc.b    ".", 0

err_msg:
    dc.b    "dir: cannot open", 0

    even
dirent_buf:
    ds.b    32

    even
name_buf:
    ds.b    32

    end _start
