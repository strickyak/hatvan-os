; Hatvan OS User Process Startup Stub & Syscall Wrappers for Z80
; Standard DECB user binary entry at $0500

    org 0x0500

start:
cstart:
    ld   (v_sys.ArgsPtr), hl
    call _main
    xor  a
    ld   b, a
    out  (0x60), a
    defb 0x06 ; F$Exit
.hang:
    jr   .hang

_printf:
    ld   iy, 0
    add  iy, sp
    ld   l, (iy+2)
    ld   h, (iy+3)
    ld   a, h
    or   l
    jr   z, .printf_arg2

    push hl
    ld   bc, 0
.printf_len1:
    ld   a, (hl)
    or   a
    jr   z, .printf_write1
    inc  hl
    inc  bc
    jr   .printf_len1
.printf_write1:
    pop  hl
    ld   a, b
    or   c
    jr   z, .printf_arg2
    ld   a, 2                    ; stderr
    out  (0x60), a
    defb 0x8A                    ; I$Write

.printf_arg2:
    ld   l, (iy+4)
    ld   h, (iy+5)
    ld   a, h
    or   l
    jr   z, .printf_done

    push hl
    ld   bc, 0
.printf_len2:
    ld   a, (hl)
    or   a
    jr   z, .printf_write2
    inc  hl
    inc  bc
    jr   .printf_len2
.printf_write2:
    pop  hl
    ld   a, b
    or   c
    jr   z, .printf_done
    ld   a, 2                    ; stderr
    out  (0x60), a
    defb 0x8A                    ; I$Write

.printf_done:
    ret

__exit:
_exit:
    ld   iy, 0
    add  iy, sp
    ld   b, (iy+2)
    xor  a
    out  (0x60), a
    defb 0x06                    ; F$Exit
.exit_hang:
    jr   .exit_hang

f_sys__SysOpen:
    ld   iy, 0
    add  iy, sp
    ld   l, (iy+2)
    ld   h, (iy+3)               ; HL = path
    ld   a, (iy+4)               ; A = mode
    out  (0x60), a
    defb 0x84                    ; I$Open
    jr   c, .open_err
    ld   l, a
    ld   h, 0
    ret
.open_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   hl, 0x00FF
    ld   a, 0xFF
    ret

f_sys__SysCreate:
    ld   iy, 0
    add  iy, sp
    ld   l, (iy+2)
    ld   h, (iy+3)               ; HL = path
    ld   a, (iy+4)               ; A = mode
    ld   b, (iy+6)               ; B = attrs
    out  (0x60), a
    defb 0x83                    ; I$Create
    jr   c, .create_err
    ld   l, a
    ld   h, 0
    ret
.create_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   hl, 0x00FF
    ld   a, 0xFF
    ret

f_sys__SysRead:
    ld   iy, 0
    add  iy, sp
    ld   a, (iy+2)               ; A = path
    ld   l, (iy+4)
    ld   h, (iy+5)               ; HL = buf
    ld   c, (iy+6)
    ld   b, (iy+7)               ; BC = maxLen
    out  (0x60), a
    defb 0x89                    ; I$Read
    jr   c, .read_err
    ld   h, b
    ld   l, c
    ret
.read_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   hl, 0
    ret

f_sys__SysReadLn:
    ld   iy, 0
    add  iy, sp
    ld   a, (iy+2)               ; A = path
    ld   l, (iy+4)
    ld   h, (iy+5)               ; HL = buf
    ld   c, (iy+6)
    ld   b, (iy+7)               ; BC = maxLen
    out  (0x60), a
    defb 0x8B                    ; I$ReadLn
    jr   c, .readln_err
    ld   h, b
    ld   l, c
    ret
.readln_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   hl, 0
    ret

f_sys__SysWrite:
    ld   iy, 0
    add  iy, sp
    ld   a, (iy+2)               ; A = path
    ld   l, (iy+4)
    ld   h, (iy+5)               ; HL = buf
    ld   c, (iy+6)
    ld   b, (iy+7)               ; BC = count
    out  (0x60), a
    defb 0x8A                    ; I$Write
    jr   c, .write_err
    ld   h, b
    ld   l, c
    ret
.write_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   hl, 0
    ret

f_sys__SysWritLn:
    ld   iy, 0
    add  iy, sp
    ld   a, (iy+2)               ; A = path
    ld   l, (iy+4)
    ld   h, (iy+5)               ; HL = buf
    ld   c, (iy+6)
    ld   b, (iy+7)               ; BC = count
    out  (0x60), a
    defb 0x8C                    ; I$WritLn
    jr   c, .writln_err
    ld   h, b
    ld   l, c
    ret
.writln_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   hl, 0
    ret

f_sys__SysClose:
    ld   iy, 0
    add  iy, sp
    ld   a, (iy+2)               ; A = path
    out  (0x60), a
    defb 0x8F                    ; I$Close
    jr   c, .close_err
    xor  a
    ld   hl, 0
    ret
.close_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   l, b
    ld   h, 0
    ret

f_sys__SysDup:
    ld   iy, 0
    add  iy, sp
    ld   a, (iy+2)               ; A = path
    out  (0x60), a
    defb 0x82                    ; I$Dup
    jr   c, .dup_err
    ld   l, a
    ld   h, 0
    ret
.dup_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   hl, 0x00FF
    ld   a, 0xFF
    ret

f_sys__SysDelete:
    ld   iy, 0
    add  iy, sp
    ld   l, (iy+2)
    ld   h, (iy+3)               ; HL = path
    xor  a
    out  (0x60), a
    defb 0x87                    ; I$Delete
    jr   c, .del_err
    xor  a
    ld   hl, 0
    ret
.del_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   l, b
    ld   h, 0
    ret

f_sys__SysExit:
    ld   iy, 0
    add  iy, sp
    ld   b, (iy+2)               ; B = status
    xor  a
    out  (0x60), a
    defb 0x06                    ; F$Exit
.sysexit_hang:
    jr   .sysexit_hang

f_sys__SysFork:
    push ix                      ; preserve caller's frame pointer
    ld   iy, 0
    add  iy, sp
    ; stack layout: [saved IX (2 bytes)], [return address (2 bytes)], [args...]
    ld   l, (iy+4)
    ld   h, (iy+5)               ; HL = cmd
    ld   e, (iy+6)
    ld   d, (iy+7)               ; DE = params
    ld   c, (iy+8)
    ld   b, (iy+9)               ; BC = paramLen
    push de
    pop  ix                      ; IX = params (maps to UserFrame.U)
    xor  a
    out  (0x60), a
    defb 0x03                    ; F$Fork
    pop  ix                      ; restore caller's frame pointer
    jr   c, .fork_err
    ld   l, a
    ld   h, 0
    ret
.fork_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   hl, 0
    ret

f_sys__SysWait:
    xor  a
    out  (0x60), a
    defb 0x04                    ; F$Wait
    jr   c, .wait_err
    ld   h, a                    ; H = child PID
    ld   l, b                    ; L = exit status
    ret
.wait_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   hl, 0
    ret

f_sys__SysChgDir:
    ld   iy, 0
    add  iy, sp
    ld   l, (iy+2)
    ld   h, (iy+3)               ; HL = path
    ld   a, (iy+4)               ; A = mode
    out  (0x60), a
    defb 0x86                    ; I$ChgDir
    jr   c, .chgdir_err
    xor  a
    ld   hl, 0
    ret
.chgdir_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   l, b
    ld   h, 0
    ret

f_sys__SysSeek:
    push ix                      ; preserve caller's frame pointer
    ld   iy, 0
    add  iy, sp
    ; stack layout: [saved IX (2 bytes)], [return address (2 bytes)], [args...]
    ld   a, (iy+4)               ; A = path
    ld   l, (iy+6)
    ld   h, (iy+7)               ; HL = pos
    push hl
    pop  ix                      ; IX = pos (maps to UserFrame.U)
    out  (0x60), a
    defb 0x88                    ; I$Seek
    pop  ix                      ; restore caller's frame pointer
    jr   c, .seek_err
    xor  a
    ld   hl, 0
    ret
.seek_err:
    ld   a, b
    ld   (v_sys.LastErr), a
    ld   l, b
    ld   h, 0
    ret
