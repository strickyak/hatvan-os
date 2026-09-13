* Test program for hatvan-vm
* Prints 'Hello World!' to $FF00 and enters CWAI / sync loop

TermOut  equ $FF00

         org $2000

start    leax msg,pcr
loop     lda ,x+
         beq done
         sta TermOut
         bra loop

done     sync
         bra done

msg      fcc "Hello from Hatvan-VM!"
         fcb 10,0

         end start
