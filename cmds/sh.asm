 nam sh
 ttl Userland Shell for Hatvan OS (OS-9 / M6809)

Type_Lang equ $11 ; Program + 6809 Machine Code
Attr_Rev  equ $81 ; ReEnt + Rev 1
Edition   equ 1

 mod eom,name,Type_Lang,Attr_Rev,start,size

 org 0
line_buf  rmb 128
cmd_buf   rmb 32
param_buf rmb 128
is_bg     rmb 1
fg_pid    rmb 1
stack     rmb 256
size equ .

name fcs /sh/
 fcb Edition

prompt_str:
 fcc "$ "
prompt_len equ *-prompt_str

help_msg:
 fcc "Hatvan User Shell Builtins: help, exit, cd, cx"
 fcb 13
help_len equ *-help_msg

err_pfx:
 fcc "ERROR "
err_pfx_len equ *-err_pfx

not_found_msg:
 fcc ": not found"
 fcb 13
not_found_len equ *-not_found_msg

start:
prompt_loop:
 ; 1. Print prompt "$ " to stdout (Path 1) via I$Write ($8A)
 leax prompt_str,pcr
 ldy #prompt_len
 lda #1          ; Path 1
 swi2
 fcb $8A         ; I$Write

 ; 2. Read line from stdin (Path 0) via I$ReadLn ($8B)
 leax line_buf,u
 ldy #127
 clra            ; Path 0
 swi2
 fcb $8B         ; I$ReadLn
 lbcs do_exit     ; EOF or error -> exit shell
 cmpy #0
 lbeq do_exit

 ; 3. Skip leading spaces
 leax line_buf,u
skip_sp:
 lda ,x
 cmpa #' '
 bne check_empty
 leax 1,x
 bra skip_sp

check_empty:
 cmpa #13
 beq prompt_loop
 cmpa #10
 beq prompt_loop
 tsta
 beq prompt_loop

 ; 4. Extract command name into cmd_buf
 leay cmd_buf,u
copy_cmd:
 lda ,x
 beq cmd_done
 cmpa #' '
 beq cmd_done
 cmpa #13
 beq cmd_done
 cmpa #10
 beq cmd_done
 sta ,y+
 leax 1,x
 bra copy_cmd

cmd_done:
 clr ,y          ; null terminate cmd_buf

 ; 5. Skip spaces before parameters
skip_param_sp:
 lda ,x
 cmpa #' '
 bne copy_params_start
 leax 1,x
 bra skip_param_sp

copy_params_start:
 leay param_buf,u
 clra
 clrb            ; D = param length
copy_param_loop:
 lda ,x+
 sta ,y+
 addd #1
 cmpa #13
 beq params_done
 cmpa #10
 beq params_done
 tsta
 beq params_done
 bra copy_param_loop

params_done:
	clr	is_bg,u
	cmpd	#1
	lbls	check_builtins
	pshs	d
	leay	-1,y		; Y points to terminator
	subd	#1		; D is index of terminator
find_bg_loop:
	leay	-1,y
	subd	#1
	beq	not_bg
	lda	,y
	cmpa	#' '
	beq	find_bg_loop
	cmpa	#'&'
	bne	not_bg
	inc	is_bg,u
	lda	#13
	sta	,y
	clr	1,y
	leas	2,s		; discard saved D
	addd	#1		; count includes CR
	bra	check_builtins
not_bg:
	puls	d		; restore original D
check_builtins:
	tfr	d,y		; Y register = param length in D

 ; 6. Check builtins
 ; "exit"
 lda cmd_buf,u
 cmpa #'e'
 bne check_exit_upper
 lda cmd_buf+1,u
 cmpa #'x'
 bne check_help
 lda cmd_buf+2,u
 cmpa #'i'
 bne check_help
 lda cmd_buf+3,u
 cmpa #'t'
 bne check_help
 lda cmd_buf+4,u
 lbeq do_exit

check_exit_upper:
 cmpa #'E'
 bne check_help
 lda cmd_buf+1,u
 cmpa #'X'
 bne check_help
 lda cmd_buf+2,u
 cmpa #'I'
 bne check_help
 lda cmd_buf+3,u
 cmpa #'T'
 bne check_help
 lda cmd_buf+4,u
 lbeq do_exit

check_help:
 ; "help"
 lda cmd_buf,u
 cmpa #'h'
 beq is_help
 cmpa #'H'
 bne check_cd
is_help:
 lda cmd_buf+1,u
 anda #$DF
 cmpa #'E'
 bne check_cd
 lda cmd_buf+2,u
 anda #$DF
 cmpa #'L'
 bne check_cd
 lda cmd_buf+3,u
 anda #$DF
 cmpa #'P'
 bne check_cd
 lda cmd_buf+4,u
 bne check_cd
 ; Print help
 leax help_msg,pcr
 ldy #help_len
 lda #1
 swi2
 fcb $8C         ; I$WritLn
 lbra prompt_loop

check_cd:
 ; "cd"
 lda cmd_buf,u
 anda #$DF
 cmpa #'C'
 bne check_cx
 lda cmd_buf+1,u
 anda #$DF
 cmpa #'D'
 bne check_cx
 lda cmd_buf+2,u
 bne check_cx
 ; Do I$ChgDir with MODE_READ (1)
 leax param_buf,u
 lda #1
 swi2
 fcb $86         ; I$ChgDir
 lbcc prompt_loop
 lbsr print_error
 lbra prompt_loop

check_cx:
 ; "cx"
 lda cmd_buf,u
 anda #$DF
 cmpa #'C'
 bne do_fork
 lda cmd_buf+1,u
 anda #$DF
 cmpa #'X'
 bne do_fork
 lda cmd_buf+2,u
 bne do_fork
 ; Do I$ChgDir with MODE_EXEC ($40)
 leax param_buf,u
 lda #$40
 swi2
 fcb $86         ; I$ChgDir
 lbcc prompt_loop
 lbsr print_error
 lbra prompt_loop

do_fork:
	; External command execution via F$Fork ($03)
	; X = cmd_buf
	; Y = param length
	; U = param_buf
	pshs	u		; Preserve shell data pointer
	leax	cmd_buf,u
	leau	param_buf,u
	clra			; type/lang = any
	clrb			; mem size = default
	swi2
	fcb	$03		; F$Fork
	lbcs	fork_fail

	; Child PID in A
	sta	fg_pid,u
	lda	is_bg,u
	lbne	fork_bg_done

	; Foreground wait loop: wait until child with PID == fg_pid terminates
wait_fg_loop:
	swi2
	fcb	$04		; F$Wait
	lbcs	wait_fail	; Error (no more children)
	cmpa	fg_pid,u
	bne	wait_fg_loop	; Reaped accumulated background zombie! Ignore and keep waiting!

	; Foreground child finished! Status in B
	puls	u
	tstb
	lbeq	prompt_loop
	lbsr	print_error
	lbra	prompt_loop

wait_fail:
	puls	u
	lbra	prompt_loop

fork_bg_done:
	puls	u
	lbra	prompt_loop

fork_fail:
 puls u          ; Restore shell data pointer
 ; Print cmd_buf + ": not found\n"
 leax cmd_buf,u
 bsr print_sz
 leax not_found_msg,pcr
 ldy #not_found_len
 lda #1
 swi2
 fcb $8C
 lbra prompt_loop

do_exit:
 clrb            ; exit status 0
 swi2
 fcb $06         ; F$Exit

; Helper: print "ERROR " followed by B in decimal and newline
print_error:
 pshs b
 leax err_pfx,pcr
 ldy #err_pfx_len
 lda #1
 swi2
 fcb $8A         ; I$Write
 puls b

 ; Print B in decimal
 tfr b,a
 bsr print_byte_dec
 lda #10
 pshs a
 leax ,s
 ldy #1
 lda #1
 swi2
 fcb $8A         ; I$Write
 leas 1,s
 rts

print_byte_dec:
 clr ,-s         ; null terminator at bottom of stack
 cmpa #0
 bne pbd_loop
 lda #'0'
 pshs a
 bra pbd_print

pbd_loop:
 tsta
 beq pbd_print
 ; Repeated subtraction: A / 10
 ldb #0
pbd_sub:
 cmpa #10
 blt pbd_rem
 suba #10
 incb
 bra pbd_sub
pbd_rem:
 adda #'0'
 pshs a
 tfr b,a
 bra pbd_loop

pbd_print:
 leax ,s
 bsr print_sz
pbd_cleanup:
 puls a
 tsta
 bne pbd_cleanup
 rts

print_sz:
 ; print null-terminated string at X
 pshs x
 ldy #0
psz_len:
 lda ,x+
 beq psz_do
 leay 1,y
 bra psz_len
psz_do:
 puls x
 cmpy #0
 beq psz_ret
 lda #1
 swi2
 fcb $8A
psz_ret:
 rts

 emod
eom equ *
 end
