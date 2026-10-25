set -x
build/gep5 -disk0=build/disk0.dsk  -task1=build/rbf_6502.decb -task2=build/procfs_6502.decb "$@" build/kernel_6502.decb
