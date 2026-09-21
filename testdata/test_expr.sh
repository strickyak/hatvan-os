echo --- testing gtest ---
GTEST 1 -eq 1 && echo 1_eq_1_ok
GTEST 1 -eq 2 || echo 1_neq_2_ok
GTEST abc = abc && echo abc_eq_abc_ok
GTEST abc != def && echo abc_neq_def_ok
GTEST -z "" && echo z_empty_ok
GTEST -n "hello" && echo n_hello_ok
GTEST -f /d0/Cmds9/ECHO && echo f_echo_ok
GTEST -d /d0/Cmds9 && echo d_cmds9_ok
GTEST ! -f /d0/nonexistent && echo not_nonexistent_ok
GTEST [ 5 -gt 3 ] && echo bracket_ok
GTEST 1 -eq 1 -a 2 -eq 2 && echo and_ok
GTEST 1 -eq 2 -o 3 -eq 3 && echo or_ok

echo --- testing gexpr ---
GEXPR 2 + 3
GEXPR 10 - 4
GEXPR 3 \* 7
GEXPR 20 / 4
GEXPR 23 % 5
GEXPR 2 + 3 \* 4
GEXPR \( 2 + 3 \) \* 4
GEXPR 5 = 5
GEXPR 5 \< 10
GEXPR 5 \> 10
GEXPR length hello
GEXPR 0 \| 42
GEXPR 10 \& 20
GEXPR 0 \& 20
echo all_tests_passed
