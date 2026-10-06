set -x
build/gepz -disk0=build/disk0.dsk  -task1=build/rbf_z80.decb -task2=build/procfs_z80.decb "$@" build/kernel_z80.decb
