#!/bin/bash
set -e
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DISK="${FUZIX_DISK:-/home/strick/nand/github.com/EtchedPixels/FUZIX/Images/multicomp09/disk.img}"
KERNEL="${FUZIX_KERNEL:-/home/strick/nand/github.com/EtchedPixels/FUZIX/Kernel/fuzix.bin}"

exec "$DIR/build/gep9" -engine multicomp09fuzix -disk0 "$DISK" -curly-escape=false "$@" "$KERNEL"
