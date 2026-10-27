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

# 5. Ensure asm6502 assembler is built
ASM6502="$BUILD_DIR/asm6502"
if [ ! -x "$ASM6502" ]; then
    echo "==> Building asm6502..."
    if [ -n "$MINIGOLF_DIR" ] && [ -d "$MINIGOLF_DIR/cmd/asm6502" ]; then
        (cd "$MINIGOLF_DIR" && go build -o "$ASM6502" ./cmd/asm6502)
    else
        echo "Error: Cannot locate asm6502 source in $MINIGOLF_DIR/cmd/asm6502" >&2
        exit 1
    fi
fi

# 6. Ensure asm8086 assembler is built
ASM8086="$BUILD_DIR/asm8086"
if [ ! -x "$ASM8086" ]; then
    echo "==> Building asm8086..."
    if [ -n "$MINIGOLF_DIR" ] && [ -d "$MINIGOLF_DIR/cmd/asm8086" ]; then
        (cd "$MINIGOLF_DIR" && go build -o "$ASM8086" ./cmd/asm8086)
    else
        echo "Error: Cannot locate asm8086 source in $MINIGOLF_DIR/cmd/asm8086" >&2
        exit 1
    fi
fi

# 7. Ensure par4asm and par4bundle are built
PAR4ASM="$BUILD_DIR/par4asm"
if [ ! -x "$PAR4ASM" ]; then
    echo "==> Building par4asm..."
    (cd "$MINIGOLF_DIR" && go build -o "$PAR4ASM" ./cmd/par4asm)
fi
PAR4BUNDLE="$BUILD_DIR/par4bundle"
if [ ! -x "$PAR4BUNDLE" ]; then
    echo "==> Building par4bundle..."
    (cd "$REPO_DIR" && go build -o "$PAR4BUNDLE" ./cmd/par4bundle)
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
    MANAGED_FLAG_C=""
    MANAGED_FLAG_5=""
    MANAGED_FLAG_0=""
    OPT_FLAGS_9=""
    OPT_FLAGS_K=""
    OPT_FLAGS_Z=""
    GLOBAL_OFFSET=45056
    if grep -q 'import "mem"\|import "smap"' "$SRC" 2>/dev/null || [ "$BASE" = "gsh2" ]; then
        MANAGED_FLAG_9="-I $CMDS_DIR/managed -D prelude.HEAP_SIZE=8000"
        MANAGED_FLAG_K="-I $CMDS_DIR/managed -D prelude.HEAP_SIZE=64000"
        MANAGED_FLAG_Z="-I $CMDS_DIR/managed -D prelude.HEAP_SIZE=8000"
        MANAGED_FLAG_C="-I $CMDS_DIR/managed -D prelude.HEAP_SIZE=8000"
        MANAGED_FLAG_5="-I $CMDS_DIR/managed -D prelude.HEAP_SIZE=8000"
        MANAGED_FLAG_0="-I $CMDS_DIR/managed -D prelude.HEAP_SIZE=16000"
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

    # --- MOS 6502 Target (*.5.decb) ---
    echo "  [6502] Compiling with MiniGolf..."
    "$MINIGOLF" -m=6502 \
        -global_var_offset 512 \
        $MANAGED_FLAG_5 \
        -I "$CMDS_DIR/lib" \
        -I "$REPO_DIR/kernel/common" \
        -o "$BUILD_DIR/$BASE.5.asm" \
        "$SRC"

    cat "$BUILD_DIR/$BASE.5.asm" "$CMDS_DIR/lib/cstart_6502.asm" > "$BUILD_DIR/full_$BASE.5.asm"

    echo "  [6502] Assembling with asm6502..."
    "$ASM6502" -l "$BUILD_DIR/$BASE.5.list" -o "$BUILD_DIR/$BASE.5.decb" "$BUILD_DIR/full_$BASE.5.asm"
    cp -f "$BUILD_DIR/$BASE.5.decb" "$DIR/$BASE.5.decb"

    # --- Intel 8086 Target (*.0.decb) ---
    echo "  [8086] Compiling with MiniGolf..."
    "$MINIGOLF" -m=8086 \
        -global_var_offset 512 \
        $MANAGED_FLAG_0 \
        -I "$CMDS_DIR/lib" \
        -I "$REPO_DIR/kernel/common" \
        -o "$BUILD_DIR/$BASE.0.asm" \
        "$SRC"

    cat "$CMDS_DIR/lib/cstart_8086.asm" "$BUILD_DIR/$BASE.0.asm" > "$BUILD_DIR/full_$BASE.0.asm"
    echo "	END user_start" >> "$BUILD_DIR/full_$BASE.0.asm"

    echo "  [8086] Assembling with asm8086..."
    "$ASM8086" -l "$BUILD_DIR/$BASE.0.list" -o "$BUILD_DIR/$BASE.0.decb" "$BUILD_DIR/full_$BASE.0.asm"
    cp -f "$BUILD_DIR/$BASE.0.decb" "$DIR/$BASE.0.decb"

    # --- RCA CDP1802 Target (*.c.decb) ---
    echo "  [1802] Compiling with MiniGolf (par4)..."
    C_MSG=""
    if "$MINIGOLF" -m par4 \
        $MANAGED_FLAG_C \
        -I "$MINIGOLF_DIR/par4-lib" \
        -I "$CMDS_DIR/lib_c" \
        -I "$REPO_DIR/kernel/common" \
        -o "$BUILD_DIR/$BASE.c.p4s" \
        "$SRC" 2>/dev/null; then
        echo "  [1802] Assembling with par4asm..."
        "$PAR4ASM" -o "$BUILD_DIR/$BASE.c.p4p" "$BUILD_DIR/$BASE.c.p4s"

        echo "  [1802] Bundling with par4bundle..."
        "$PAR4BUNDLE" -vm "$BUILD_DIR/par4vmc.bin" \
            -p4p "$BUILD_DIR/$BASE.c.p4p" \
            -o "$BUILD_DIR/$BASE.c.decb" \
            -entry 0x0020 -base 0x2000
        cp -f "$BUILD_DIR/$BASE.c.decb" "$DIR/$BASE.c.decb"

        SIZE_C=$(wc -c < "$DIR/$BASE.c.decb" | tr -d ' ')
        C_MSG=", and $DIR/$BASE.c.decb ($SIZE_C bytes)"
    else
        echo "  [1802] Note: skipped for 1802 (unsupported syntax in par4)"
    fi

    SIZE_9=$(wc -c < "$DIR/$BASE.9.decb" | tr -d ' ')
    SIZE_K=$(wc -c < "$DIR/$BASE.k.decb" | tr -d ' ')
    SIZE_Z=$(wc -c < "$DIR/$BASE.z.decb" | tr -d ' ')
    echo "  -> Created $DIR/$BASE.9.decb ($SIZE_9 bytes), $DIR/$BASE.k.decb ($SIZE_K bytes), $DIR/$BASE.z.decb ($SIZE_Z bytes)$C_MSG"
done

echo "Done."
