set -x
build/gep0 -disk0=build/disk0.dsk -task1=build/rbf_8086.decb -task2=build/procfs_8086.decb "$@" build/kernel_8086.decb
