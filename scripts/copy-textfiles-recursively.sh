#!/bin/sh
#
# Copy recursively unix directory $1 to OS9 disk $2

DISK="$(echo "$2" | cut -d, -f1)"
DEST_DIR="$(echo "$2" | cut -s -d, -f2 | sed 's:^/*::' | sed 's:/*$::')"

# Find all directories in source and create them in order
for x in $(find "$1" -type d -print | sort)
do
    rel="$(echo "$x" | sed 's:^/*::' | sed 's:/*$::')"
    if [ -n "$rel" ]; then
        if [ -n "$DEST_DIR" ]; then
            target="/$DEST_DIR/$rel"
        else
            target="/$rel"
        fi
        target="$(echo "$target" | sed 's://*:/:g' | sed 's:/*$::')"
        os9 makdir "$DISK,$target" 2>/dev/null || true
    fi
done

for x in $(find "$1" -type f -print | sort)
do
    rel="$(echo "$x" | sed 's:^/*::')"
    if [ -n "$DEST_DIR" ]; then
        target="/$DEST_DIR/$rel"
    else
        target="/$rel"
    fi
    target="$(echo "$target" | sed 's://*:/:g')"
    os9 copy -r -l "$x" "$DISK,$target"
done

