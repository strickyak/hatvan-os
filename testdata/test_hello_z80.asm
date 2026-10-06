; Test program for Hatvan OS/Z on gepz
        org     $1000
start:
        ld      sp, $8000
        ld      hl, msg
print_loop:
        ld      a, (hl)
        or      a
        jr      z, done
        ld      ($FF00), a      ; Hatvan putchar ($FF00)
        inc     hl
        jr      print_loop
done:
        ld      a, 42
        ld      ($FF05), a      ; Hatvan ExitCode ($FF05)
hang:
        halt
        jr      hang

msg:
        defb    "Hello Hatvan OS/Z from gepz!", 10, 0
        end     start
