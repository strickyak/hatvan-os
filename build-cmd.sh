#!/bin/sh
# build-cmd.sh - Framework to compile MiniGolf commands into *.9.decb and *.k.decb
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$SCRIPT_DIR"
BUILD_DIR="$REPO_DIR/build"
CMDS_DIR="$REPO_DIR/cmds"
MINIGOLF_DIR="$(cd "$REPO_DIR/../minigolf" 2>/dev/null && pwd || true)"

mkdir -p "$BUILD_DIR"

# 1. Ensure MiniGolf compiler is built
MINIGOLF="$BUILD_DIR/minigolf"
if [ ! -x "$MINIGOLF" ]; then
    echo "==> Building minigolf..."
    if [ -n "$MINIGOLF_DIR" ] && [ -d "$MINIGOLF_DIR" ]; then
        (cd "$MINIGOLF_DIR" && go build -o "$MINIGOLF" .)
    else
        echo "Error: Cannot locate minigolf repository directory." >&2
        exit 1
    fi
fi

# 2. Ensure asm6809 assembler is built
ASM6809="$BUILD_DIR/asm6809"
if [ ! -x "$ASM6809" ]; then
    if [ -n "$MINIGOLF_DIR" ] && [ -f "$MINIGOLF_DIR/cmd/asm6809/main.go" ]; then
        echo "==> Building asm6809..."
        (cd "$MINIGOLF_DIR" && go build -o "$ASM6809" ./cmd/asm6809)
    elif which asm6809 >/dev/null 2>&1; then
        ASM6809="$(which asm6809)"
    fi
fi

# 3. Ensure asm68k assembler is built
ASM68K="$BUILD_DIR/asm68k"
if [ ! -x "$ASM68K" ]; then
    echo "==> Building asm68k..."
    if [ -n "$MINIGOLF_DIR" ] && [ -d "$MINIGOLF_DIR/cmd/asm68k" ]; then
        (cd "$MINIGOLF_DIR" && go build -o "$ASM68K" ./cmd/asm68k)
    else
        echo "Error: Cannot locate asm68k source in $MINIGOLF_DIR/cmd/asm68k" >&2
        exit 1
    fi
fi

# 4. Ensure asmz80 assembler is built
ASMZ80="$BUILD_DIR/asmz80"
if [ ! -x "$ASMZ80" ]; then
    echo "==> Building asmz80..."
    if [ -n "$MINIGOLF_DIR" ] && [ -d "$MINIGOLF_DIR/cmd/asmz80" ]; then
        (cd "$MINIGOLF_DIR" && go build -o "$ASMZ80" ./cmd/asmz80)
    else
        echo "Error: Cannot locate asmz80 source in $MINIGOLF_DIR/cmd/asmz80" >&2
        exit 1
    fi
fi

SREC2DECB="$REPO_DIR/scripts/srec2decb.py"

TARGETS="$@"
if [ -z "$TARGETS" ]; then
    TARGETS="gecho gcat gdir gdump gsh gsh2 gtest gexpr gtrue gfalse"
fi

for TARGET in $TARGETS; do
    # Resolve source path and base command name
    case "$TARGET" in
        *.golf)
            SRC="$TARGET"
            BASE="$(basename "$TARGET" .golf)"
            DIR="$(dirname "$TARGET")"
            ;;
        *)
            if [ -f "$CMDS_DIR/$TARGET.golf" ]; then
                SRC="$CMDS_DIR/$TARGET.golf"
                DIR="$CMDS_DIR"
            elif [ -f "$TARGET.golf" ]; then
                SRC="$TARGET.golf"
                DIR="."
            elif [ -f "$TARGET" ]; then
                SRC="$TARGET"
                DIR="$(dirname "$TARGET")"
            else
                echo "Error: Cannot find source file for '$TARGET'" >&2
                exit 1
            fi
            BASE="$(basename "$SRC" .golf)"
            ;;
    esac

    MANAGED_FLAG_9=""
    MANAGED_FLAG_K=""
    MANAGED_FLAG_Z=""
    OPT_FLAGS_9=""
    OPT_FLAGS_K=""
    OPT_FLAGS_Z=""
    GLOBAL_OFFSET=45056
    if grep -q 'import "mem"\|import "smap"' "$SRC" 2>/dev/null || [ "$BASE" = "gsh2" ]; then
        MANAGED_FLAG_9="-I $CMDS_DIR/managed -D prelude.HEAP_SIZE=8000"
        MANAGED_FLAG_K="-I $CMDS_DIR/managed -D prelude.HEAP_SIZE=64000"
        MANAGED_FLAG_Z="-I $CMDS_DIR/managed -D prelude.HEAP_SIZE=8000"
        #yak# OPT_FLAGS_9="-no-slotsharing6809 -no-stackalloc -no-inline -no-leaf-opt6809"
        #yak# OPT_FLAGS_K="-no-stackalloc -no-inline"
        GLOBAL_OFFSET=48128
    fi

    echo "=== Building $BASE ($SRC) ==="

    # --- Motorola 6809 Target (*.9.decb) ---
    echo "  [6809] Compiling with MiniGolf..."
    "$MINIGOLF" -m M6809 \
        -global_var_offset $GLOBAL_OFFSET \
        $MANAGED_FLAG_9 $OPT_FLAGS_9 \
        -I "$CMDS_DIR/lib" \
        -I "$REPO_DIR/kernel/common" \
        -o "$BUILD_DIR/$BASE.9.asm" \
        "$SRC"

    cat "$CMDS_DIR/lib/cstart_6809.asm" "$BUILD_DIR/$BASE.9.asm" > "$BUILD_DIR/full_$BASE.9.asm"
    echo "    end start" >> "$BUILD_DIR/full_$BASE.9.asm"

    echo "  [6809] Assembling with asm6809..."
    "$ASM6809" -decb -l "$BUILD_DIR/$BASE.9.list" -o "$BUILD_DIR/$BASE.9.decb" "$BUILD_DIR/full_$BASE.9.asm"
    cp -f "$BUILD_DIR/$BASE.9.decb" "$DIR/$BASE.9.decb"

    # --- Motorola 68000 Target (*.k.decb) ---
    echo "  [68K]  Compiling with MiniGolf..."
    "$MINIGOLF" -m=k \
        $MANAGED_FLAG_K $OPT_FLAGS_K \
        -I "$CMDS_DIR/lib" \
        -I "$REPO_DIR/kernel/common" \
        -o "$BUILD_DIR/$BASE.k.s" \
        "$SRC"

    cat "$CMDS_DIR/lib/cstart_68k.s" "$BUILD_DIR/$BASE.k.s" > "$BUILD_DIR/full_$BASE.k.s"
    echo "    end start" >> "$BUILD_DIR/full_$BASE.k.s"

    echo "  [68K]  Assembling with asm68k..."
    "$ASM68K" -l "$BUILD_DIR/$BASE.k.list" -o "$BUILD_DIR/$BASE.k.srec" "$BUILD_DIR/full_$BASE.k.s"
    python3 "$SREC2DECB" "$BUILD_DIR/$BASE.k.srec" "$BUILD_DIR/$BASE.k.decb"
    cp -f "$BUILD_DIR/$BASE.k.decb" "$DIR/$BASE.k.decb"

    # --- Zilog Z80 Target (*.z.decb) ---
    echo "  [Z80]  Compiling with MiniGolf..."
    "$MINIGOLF" -m=z80 \
        $MANAGED_FLAG_Z $OPT_FLAGS_Z \
        -I "$CMDS_DIR/lib" \
        -I "$REPO_DIR/kernel/common" \
        -o "$BUILD_DIR/$BASE.z.asm" \
        "$SRC"

    cat "$CMDS_DIR/lib/cstart_z80.asm" "$BUILD_DIR/$BASE.z.asm" > "$BUILD_DIR/full_$BASE.z.asm"
    echo "    end start" >> "$BUILD_DIR/full_$BASE.z.asm"

    echo "  [Z80]  Assembling with asmz80..."
    "$ASMZ80" -l "$BUILD_DIR/$BASE.z.list" -o "$BUILD_DIR/$BASE.z.decb" "$BUILD_DIR/full_$BASE.z.asm"
    cp -f "$BUILD_DIR/$BASE.z.decb" "$DIR/$BASE.z.decb"

    SIZE_9=$(wc -c < "$DIR/$BASE.9.decb" | tr -d ' ')
    SIZE_K=$(wc -c < "$DIR/$BASE.k.decb" | tr -d ' ')
    SIZE_Z=$(wc -c < "$DIR/$BASE.z.decb" | tr -d ' ')
    echo "  -> Created $DIR/$BASE.9.decb ($SIZE_9 bytes), $DIR/$BASE.k.decb ($SIZE_K bytes), and $DIR/$BASE.z.decb ($SIZE_Z bytes)"
done

echo "Done."
