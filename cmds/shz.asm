; Userland Shell command for Hatvan OS / Z80
    org 0x0500

_start:
start:
    ld   sp, 0xDFFF     ; ensure valid stack in user task
prompt_loop:
    ; 1. Print prompt "$ " to stdout (Path 1) via I$Write ($8A)
    ld   hl, prompt_str
    ld   bc, 2          ; count = 2
    ld   a, 1           ; Path 1
    out  (0x60), a
    defb 0x8A           ; I$Write

    ; 2. Read line from stdin (Path 0) via I$ReadLn ($8B)
    ld   hl, line_buf
    ld   bc, 127        ; maxLen = 127
    xor  a              ; Path 0
    out  (0x60), a
    defb 0x8B           ; I$ReadLn
    jp   c, do_exit     ; error -> exit
    ld   a, b
    or   c
    jp   z, do_exit     ; 0 bytes read -> EOF -> exit

    ; 3. Skip leading spaces
    ld   hl, line_buf
skip_sp:
    ld   a, (hl)
    cp   ' '
    jr   nz, check_empty
    inc  hl
    jr   skip_sp

check_empty:
    cp   0x0D
    jr   z, prompt_loop
    cp   0x0A
    jr   z, prompt_loop
    or   a
    jr   z, prompt_loop

    ; 4. Extract command name into cmd_buf
    ld   de, cmd_buf
copy_cmd:
    ld   a, (hl)
    or   a
    jr   z, cmd_done
    cp   ' '
    jr   z, cmd_done
    cp   0x0D
    jr   z, cmd_done
    cp   0x0A
    jr   z, cmd_done
    ld   (de), a
    inc  hl
    inc  de
    jr   copy_cmd

cmd_done:
    xor  a
    ld   (de), a        ; null terminate cmd_buf

    ; 5. Skip spaces before parameters
skip_param_sp:
    ld   a, (hl)
    cp   ' '
    jr   nz, copy_params_start
    inc  hl
    jr   skip_param_sp

copy_params_start:
    ld   de, param_buf
    ld   bc, 0          ; BC = param length counter
copy_params:
    ld   a, (hl)
    ld   (de), a
    inc  hl
    inc  de
    inc  bc
    cp   0x0D
    jr   z, params_done
    cp   0x0A
    jr   z, params_done
    or   a
    jr   z, params_done
    jr   copy_params

params_done:
    xor  a
    ld   (is_bg), a
    ; Check if param length <= 1
    ld   a, b
    or   a
    jr   nz, check_bg
    ld   a, c
    cp   2
    jr   c, check_builtins_z
check_bg:
    ; Check trailing '&'
    push bc
    push de
    dec  de             ; DE points to terminator
find_bg_loop:
    dec  de
    dec  bc
    ld   a, b
    or   c
    jr   z, not_bg
    ld   a, (de)
    cp   ' '
    jr   z, find_bg_loop
    cp   '&'
    jr   nz, not_bg
    ; Found '&'
    ld   a, 1
    ld   (is_bg), a
    ld   a, 10
    ld   (de), a
    inc  de
    xor  a
    ld   (de), a
    pop  de
    pop  bc
    jr   check_builtins_z
not_bg:
    pop  de
    pop  bc

check_builtins_z:
    ; 6. Check builtins
    ; "exit"
    ld   hl, cmd_buf
    ld   a, (hl)
    and  0xDF
    cp   'E'
    jr   nz, check_help
    inc  hl
    ld   a, (hl)
    and  0xDF
    cp   'X'
    jr   nz, check_help
    inc  hl
    ld   a, (hl)
    and  0xDF
    cp   'I'
    jr   nz, check_help
    inc  hl
    ld   a, (hl)
    and  0xDF
    cp   'T'
    jr   nz, check_help
    inc  hl
    ld   a, (hl)
    or   a
    jp   z, do_exit

check_help:
    ; "help"
    ld   hl, cmd_buf
    ld   a, (hl)
    and  0xDF
    cp   'H'
    jr   nz, check_cd
    inc  hl
    ld   a, (hl)
    and  0xDF
    cp   'E'
    jr   nz, check_cd
    inc  hl
    ld   a, (hl)
    and  0xDF
    cp   'L'
    jr   nz, check_cd
    inc  hl
    ld   a, (hl)
    and  0xDF
    cp   'P'
    jr   nz, check_cd
    inc  hl
    ld   a, (hl)
    or   a
    jr   nz, check_cd
    ; Print help message
    ld   hl, help_msg
    ld   bc, 47
    ld   a, 1
    out  (0x60), a
    defb 0x8C           ; I$WritLn
    jp   prompt_loop

check_cd:
    ; "cd"
    ld   hl, cmd_buf
    ld   a, (hl)
    and  0xDF
    cp   'C'
    jr   nz, check_cx
    inc  hl
    ld   a, (hl)
    and  0xDF
    cp   'D'
    jr   nz, check_cx
    inc  hl
    ld   a, (hl)
    or   a
    jr   nz, check_cx
    ; Call I$ChgDir with MODE_READ (1)
    ld   hl, param_buf
    ld   a, 1           ; Mode = 1
    out  (0x60), a
    defb 0x86           ; I$ChgDir
    jp   nc, prompt_loop
    ld   a, b
    call print_error
    jp   prompt_loop

check_cx:
    ; "cx"
    ld   hl, cmd_buf
    ld   a, (hl)
    and  0xDF
    cp   'C'
    jr   nz, do_fork
    inc  hl
    ld   a, (hl)
    and  0xDF
    cp   'X'
    jr   nz, do_fork
    inc  hl
    ld   a, (hl)
    or   a
    jr   nz, do_fork
    ; Call I$ChgDir with MODE_EXEC ($40)
    ld   hl, param_buf
    ld   a, 0x40        ; Mode = 0x40
    out  (0x60), a
    defb 0x86           ; I$ChgDir
    jp   nc, prompt_loop
    ld   a, b
    call print_error
    jp   prompt_loop

do_fork:
    ; Call F$Fork ($03)
    ; HL = cmd_buf
    ; IX = param_buf
    ; BC = param_buf length
    ld   hl, cmd_buf
    ld   de, param_buf
    push de
    pop  ix
    xor  a
    out  (0x60), a
    defb 0x03           ; F$Fork
    jr   c, fork_fail

    ; Child PID in A
    ld   (fg_pid), a
    ld   a, (is_bg)
    or   a
    jp   nz, prompt_loop

wait_fg_loop:
    ; Call F$Wait ($04)
    xor  a
    out  (0x60), a
    defb 0x04           ; F$Wait
    jp   c, prompt_loop

    ; Child PID in A, status in B
    ld   c, a
    ld   a, (fg_pid)
    cp   c
    jr   nz, wait_fg_loop

    ; Reaped child: check status in B
    ld   a, b
    or   a
    jp   z, prompt_loop

    ; Nonzero status: print "ERROR %d\n"
    ld   a, b
    call print_error
    jp   prompt_loop

fork_fail:
    ; Print cmd_buf + ": not found\n"
    ld   hl, cmd_buf
    call print_sz
    ld   hl, not_found_msg
    ld   bc, 12
    ld   a, 1
    out  (0x60), a
    defb 0x8C           ; I$WritLn
    jp   prompt_loop

do_exit:
    ld   b, 0           ; status 0
    xor  a
    out  (0x60), a
    defb 0x06           ; F$Exit
.hang:
    halt
    jr   .hang

; Print "ERROR " followed by A in decimal and newline
print_error:
    push af
    ld   hl, err_pfx
    ld   bc, 6
    ld   a, 1
    out  (0x60), a
    defb 0x8A           ; I$Write

    pop  af
    call print_dec

    ld   hl, newline_str
    ld   bc, 1
    ld   a, 1
    out  (0x60), a
    defb 0x8A           ; I$Write
    ret

; Convert byte in A to decimal string at num_buf and print via print_sz
print_dec:
    ld   hl, num_buf + 7
    xor  a
    ld   (hl), a
    pop  de             ; return address
    push de             ; restore stack
    ; value in A
    or   a
    jr   nz, .pd_loop
    dec  hl
    ld   a, '0'
    ld   (hl), a
    jp   print_sz
.pd_loop:
    or   a
    jp   z, print_sz
    ; Div A by 10:
    ld   c, a
    ld   b, 0
.div10:
    ld   a, c
    cp   10
    jr   c, .div10_done
    sub  10
    ld   c, a
    inc  b
    jr   .div10
.div10_done:
    ld   a, c
    add  a, '0'
    dec  hl
    ld   (hl), a
    ld   a, b
    jr   .pd_loop

; Print null-terminated string at HL via I$Write
print_sz:
    push hl
    ld   bc, 0
.psz_len:
    ld   a, (hl)
    or   a
    jr   z, .psz_write
    inc  hl
    inc  bc
    jr   .psz_len
.psz_write:
    pop  hl
    ld   a, b
    or   c
    ret  z
    ld   a, 1           ; stdout
    out  (0x60), a
    defb 0x8A           ; I$Write
    ret

prompt_str:
    defb "$ ", 0

help_msg:
    defb "Hatvan User Shell Builtins: help, exit, cd, cx", 10, 0

err_pfx:
    defb "ERROR ", 0

not_found_msg:
    defb ": not found", 10, 0

newline_str:
    defb 10, 0

line_buf:
    defs 128, 0
cmd_buf:
    defs 32, 0
param_buf:
    defs 128, 0
is_bg:
    defb 0
fg_pid:
    defb 0
num_buf:
    defs 16, 0

    end start
