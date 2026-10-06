#!/usr/bin/env bash
# scripts/test-quick.sh - Fast, thorough test suite (<1 minute) focusing on Z80
set -eo pipefail

START_TIME=$(date +%s)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
MINIGOLF_DIR="$(cd "$REPO_DIR/../minigolf" && pwd)"

echo "============================================================"
echo "  Hatvan OS / MiniGolf Fast Test Suite (Z80 Focus)"
echo "============================================================"

# 1. MiniGolf Z80 tests
echo ""
echo "[1/6] Running MiniGolf Z80 tests (Assembler, Triangles, Golf & C suites)..."
(
    cd "$MINIGOLF_DIR"
    go test -count=1 -run ".*z80.*|.*Z80.*" .
    go test -count=1 ./cmd/asmz80
)
echo "  -> MiniGolf Z80 test suite: PASS"

# 2. Gepz VM unit tests
echo ""
echo "[2/6] Running Gepz VM unit tests..."
(
    cd "$REPO_DIR"
    go test -count=1 ./gepz
)
echo "  -> Gepz VM tests: PASS"

# 3. Build Hatvan OS targets
echo ""
echo "[3/6] Building Hatvan OS targets (VMs, kernels, cmds, disk)..."
make -C "$REPO_DIR" vms kernels cmds disk
echo "  -> Build: OK"

# 4. Hatvan OS Z80 Boot Test
echo ""
echo "[4/6] Testing Hatvan OS Z80 kernel boot & exit..."
"$REPO_DIR/build/gepz" -disk0="$REPO_DIR/build/disk0.dsk" -input="exit\n" "$REPO_DIR/build/kernel_z80.decb" >/dev/null
echo "  -> Z80 Kernel Boot: PASS"

# 5. Hatvan OS Z80 Interactive Test
echo ""
echo "[5/6] Testing Hatvan OS Z80 interactive shell & commands..."
INTERACTIVE_INPUT="help\npwd\npwx\nECHO hello from z80 userspace\nECHO redirection works on z80 > /d0/redirz.txt\nCAT /d0/redirz.txt\nGECHO hello from gecho z80\nGCAT /d0/redirz.txt\nGDIR\nGDUMP /CmdsZ/ECHO\nGSEEKTEST\nSH\nhelp\nECHO nested shell z80\nCAT /nonexistent\nECHO background z80 &\nCAT /proc/p\nexit\nCAT /nonexistent\nexit\n"
"$REPO_DIR/build/gepz" -disk0="$REPO_DIR/build/disk0.dsk" -input="$INTERACTIVE_INPUT" "$REPO_DIR/build/kernel_z80.decb" >/dev/null
echo "  -> Z80 Interactive Userspace: PASS"

# 6. Multi-architecture Smoke Tests (6809 and 68k)
echo ""
echo "[6/6] Running multi-architecture smoke tests (6809 & 68k)..."
"$REPO_DIR/build/gep9" --disk0="$REPO_DIR/build/disk0.dsk" --input="exit\n" "$REPO_DIR/build/kernel_6809.decb" >/dev/null
"$REPO_DIR/build/gepk" -disk0="$REPO_DIR/build/disk0.dsk" -input="exit\n" "$REPO_DIR/build/kernel_68k.srec" >/dev/null
echo "  -> 6809 & 68k Smoke: PASS"

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
echo "============================================================"
echo "  Z80 Binary Sizes:"
ls -lh "$REPO_DIR/build/kernel_z80.decb" "$REPO_DIR/build/rbf_z80.decb" "$REPO_DIR/build/procfs_z80.decb" "$REPO_DIR/cmds/gsh.z.decb" | awk '{print "    " $9 ": " $5}'
echo "============================================================"
echo "  ALL TESTS PASSED in ${ELAPSED}s"
echo "============================================================"
