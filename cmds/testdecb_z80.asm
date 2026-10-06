; Test DECB binary for Hatvan OS (Z80)
    org 0x0500
start:
cstart:
    ld   a, 'D'
    out  (0x60), a
    defb 0x06
    ret
    end start
