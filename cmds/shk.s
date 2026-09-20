; Userland Shell command for Hatvan OS / M68K
    org $00000200

_start:
prompt_loop:
    ; 1. Print prompt "$ " to stdout (Path 1) via I$Write ($8A)
    lea     prompt_str, a0
    move.l  #2, d2          ; count = 2
    moveq   #1, d1          ; Path 1
    move.l  #$8A, d0        ; I$Write
    dc.w    $4E40

    ; 2. Read line from stdin (Path 0) via I$ReadLn ($8B)
    lea     line_buf, a0
    move.l  #127, d2        ; maxLen = 127
    moveq   #0, d1          ; Path 0
    move.l  #$8B, d0        ; I$ReadLn
    dc.w    $4E40
    bcs     do_exit
    tst.l   d2
    beq     do_exit

    ; 3. Skip leading spaces
    lea     line_buf, a0
skip_sp:
    move.b  (a0)+, d0
    cmp.b   #' ', d0
    beq     skip_sp
    subq.l  #1, a0

    ; Check empty line
    move.b  (a0), d0
    beq     prompt_loop
    cmp.b   #$0D, d0
    beq     prompt_loop
    cmp.b   #$0A, d0
    beq     prompt_loop

    ; 4. Extract command name into cmd_buf
    lea     cmd_buf, a1
copy_cmd:
    move.b  (a0)+, d0
    beq     cmd_done
    cmp.b   #' ', d0
    beq     cmd_done
    cmp.b   #$0D, d0
    beq     cmd_done
    cmp.b   #$0A, d0
    beq     cmd_done
    move.b  d0, (a1)+
    bra     copy_cmd

cmd_done:
    clr.b   (a1)            ; null terminate
    subq.l  #1, a0

    ; 5. Skip spaces before parameters
skip_param_sp:
    move.b  (a0)+, d0
    cmp.b   #' ', d0
    beq     skip_param_sp
    subq.l  #1, a0

    ; Copy parameters into param_buf
    lea     param_buf, a1
    moveq   #0, d4          ; d4 = length counter
copy_params:
    move.b  (a0)+, d0
    move.b  d0, (a1)+
    addq.l  #1, d4
    cmp.b   #$0D, d0
    beq     params_done
    cmp.b   #$0A, d0
    beq     params_done
    tst.b   d0
    beq     params_done
    bra     copy_params

params_done:

    ; 6. Check builtins
    ; "exit"
    lea     cmd_buf, a1
    move.b  (a1), d0
    and.b   #$DF, d0
    cmp.b   #'E', d0
    bne     check_help
    move.b  1(a1), d0
    and.b   #$DF, d0
    cmp.b   #'X', d0
    bne     check_help
    move.b  2(a1), d0
    and.b   #$DF, d0
    cmp.b   #'I', d0
    bne     check_help
    move.b  3(a1), d0
    and.b   #$DF, d0
    cmp.b   #'T', d0
    bne     check_help
    tst.b   4(a1)
    beq     do_exit

check_help:
    ; "help"
    lea     cmd_buf, a1
    move.b  (a1), d0
    and.b   #$DF, d0
    cmp.b   #'H', d0
    bne     check_cd
    move.b  1(a1), d0
    and.b   #$DF, d0
    cmp.b   #'E', d0
    bne     check_cd
    move.b  2(a1), d0
    and.b   #$DF, d0
    cmp.b   #'L', d0
    bne     check_cd
    move.b  3(a1), d0
    and.b   #$DF, d0
    cmp.b   #'P', d0
    bne     check_cd
    tst.b   4(a1)
    bne     check_cd
    ; Print help message
    lea     help_msg, a0
    move.l  #47, d2
    moveq   #1, d1          ; stdout
    move.l  #$8C, d0        ; I$WritLn
    dc.w    $4E40
    bra     prompt_loop

check_cd:
    ; "cd"
    lea     cmd_buf, a1
    move.b  (a1), d0
    and.b   #$DF, d0
    cmp.b   #'C', d0
    bne     check_cx
    move.b  1(a1), d0
    and.b   #$DF, d0
    cmp.b   #'D', d0
    bne     check_cx
    tst.b   2(a1)
    bne     check_cx
    ; Call I$ChgDir with MODE_READ (1)
    lea     param_buf, a0
    moveq   #1, d1          ; Mode = 1
    move.l  #$86, d0        ; I$ChgDir
    dc.w    $4E40
    bcc     prompt_loop
    bsr     print_error
    bra     prompt_loop

check_cx:
    ; "cx"
    lea     cmd_buf, a1
    move.b  (a1), d0
    and.b   #$DF, d0
    cmp.b   #'C', d0
    bne     do_fork
    move.b  1(a1), d0
    and.b   #$DF, d0
    cmp.b   #'X', d0
    bne     do_fork
    tst.b   2(a1)
    bne     do_fork
    ; Call I$ChgDir with MODE_EXEC ($40)
    lea     param_buf, a0
    moveq   #$40, d1        ; Mode = MODE_EXEC
    move.l  #$86, d0        ; I$ChgDir
    dc.w    $4E40
    bcc     prompt_loop
    bsr     print_error
    bra     prompt_loop

do_fork:
    ; Call F$Fork ($03)
    ; A0 = cmd_buf
    ; A1 = param_buf
    ; D2 = param length (d4)
    lea     cmd_buf, a0
    lea     param_buf, a1
    move.l  d4, d2
    move.l  #$03, d0        ; F$Fork
    dc.w    $4E40
    bcs     fork_fail

    ; Call F$Wait ($04)
    move.l  #$04, d0        ; F$Wait
    dc.w    $4E40
    bcs     prompt_loop

    ; Return status in D0.B
    tst.b   d0
    beq     prompt_loop

    ; Nonzero status: print "ERROR %d\n"
    move.b  d0, d1
    bsr     print_error
    bra     prompt_loop

fork_fail:
    ; Print cmd_buf + ": not found\n"
    lea     cmd_buf, a0
    bsr     print_sz
    lea     not_found_msg, a0
    move.l  #12, d2
    moveq   #1, d1
    move.l  #$8C, d0
    dc.w    $4E40
    bra     prompt_loop

do_exit:
    moveq   #0, d1          ; status 0
    move.l  #$06, d0        ; F$Exit
    dc.w    $4E40
    rts

; Print "ERROR " followed by D1.B in decimal and newline
print_error:
    move.l  d1, -(sp)
    lea     err_pfx, a0
    move.l  #6, d2
    moveq   #1, d1
    move.l  #$8A, d0        ; I$Write
    dc.w    $4E40

    move.l  (sp)+, d1
    and.l   #$FF, d1
    bsr     print_dec

    lea     newline_str, a0
    move.l  #1, d2
    moveq   #1, d1
    move.l  #$8A, d0
    dc.w    $4E40
    rts

; Print decimal value in D1
print_dec:
    lea     num_buf+8, a0
    clr.b   -(a0)
    tst.l   d1
    bne     pd_loop
    move.b  #'0', -(a0)
    bra     pd_done
pd_loop:
    tst.l   d1
    beq     pd_done
    divu    #10, d1
    swap    d1              ; remainder in low word
    add.b   #'0', d1
    move.b  d1, -(a0)
    clr.w   d1
    swap    d1              ; quotient in low word
    bra     pd_loop
pd_done:
    bsr     print_sz
    rts

print_sz:
    move.l  a0, -(sp)
    moveq   #0, d2
psz_cnt:
    tst.b   (a0)+
    beq     psz_do
    addq.l  #1, d2
    bra     psz_cnt
psz_do:
    move.l  (sp)+, a0
    tst.l   d2
    beq     psz_ret
    moveq   #1, d1
    move.l  #$8A, d0        ; I$Write
    dc.w    $4E40
psz_ret:
    rts

prompt_str:
    dc.b    "$ ", 0

help_msg:
    dc.b    "Hatvan User Shell Builtins: help, exit, cd, cx", 10, 0

err_pfx:
    dc.b    "ERROR ", 0

not_found_msg:
    dc.b    ": not found", 10, 0

newline_str:
    dc.b    10, 0

    even
line_buf:
    ds.b    128
cmd_buf:
    ds.b    32
param_buf:
    ds.b    128
num_buf:
    ds.b    16

    end _start
