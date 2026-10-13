#!/usr/bin/env bash
# scripts/test-expect.sh - System tests driving run*.sh interactively via Expect
set -eo pipefail

START_TIME=$(date +%s)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "============================================================"
echo "  Hatvan OS Interactive Expect System Tests"
echo "============================================================"

# Ensure expect is installed
if ! command -v expect >/dev/null 2>&1; then
    echo "Error: expect binary not found in PATH." >&2
    exit 1
fi

# Ensure test scripts are executable
chmod +x "$REPO_DIR"/tests/test_run*.exp

echo ""
echo "[1/3] Running 6809 system test (run9.sh)..."
"$REPO_DIR/tests/test_run9.exp"
echo "  -> 6809 (run9.sh): PASS"

echo ""
echo "[2/3] Running Z80 system test (runz.sh)..."
"$REPO_DIR/tests/test_runz.exp"
echo "  -> Z80 (runz.sh): PASS"

echo ""
echo "[3/3] Running 68000 system test (runk.sh)..."
"$REPO_DIR/tests/test_runk.exp"
echo "  -> 68000 (runk.sh): PASS"

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
echo "============================================================"
echo "  ALL EXPECT SYSTEM TESTS PASSED in ${ELAPSED}s"
echo "============================================================"
