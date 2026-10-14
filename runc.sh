#!/bin/sh
set -x
build/gepc -tick-hz=0 -disk0=build/disk0.dsk -task1=build/rbf_1802.decb -task2=build/procfs_1802.decb "$@" build/kernel_1802.decb
