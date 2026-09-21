echo --- while with break ---
while true; do
  gecho while_break_1
  break
  gecho while_break_bad
done

echo --- while false ---
while false; do
  gecho while_false_bad
done
gecho while_false_ok

echo --- while with shift ---
while gtest $# -gt 0; do
  gecho arg_$1
  shift
done
gecho shift_done

echo --- continue in for ---
for y in 1 2 3 4; do
  gtest $y = 2 && continue
  gtest $y = 3 && continue
  gecho for_continue_$y
done

echo --- continue in while ---
flag=first
while gtrue; do
  gtest $flag = second && break
  flag=second
  gecho will_continue
  continue
  gecho bad_after_continue
done
gecho continue_while_ok

echo --- nested break 2 ---
for o in 1 2 3; do
  for i in a b c; do
    gecho nest_$o$i
    break 2
  done
done
gecho break2_done

echo --- nested continue 2 ---
for a in 1 2; do
  cflag=first
  while gtrue; do
    gtest $cflag = second && break
    cflag=second
    gecho continue2_step_$a
    continue 2
    gecho bad_continue2
  done
done
gecho continue2_done

echo --- done all while tests ---
