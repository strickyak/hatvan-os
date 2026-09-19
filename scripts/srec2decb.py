#!/usr/bin/env python3
"""Convert Motorola S-Record binary to Extended DECB binary format.

Extended DECB format consists of:
- Chunk preamble: 0x00, 2-byte count, 2-byte load address, payload bytes
- Termination postamble: 0xFF, 0x00 0x00, 2-byte exec address
"""
import sys

def srec2decb(srec_path, decb_path):
    with open(srec_path, 'r') as f:
        lines = f.readlines()

    data = bytearray()
    entry = 0x200
    base_addr = None

    for line in lines:
        line = line.strip()
        if not line:
            continue
        if line.startswith('S3'):
            addr = int(line[4:12], 16)
            if base_addr is None:
                base_addr = addr
            payload = bytes.fromhex(line[12:-2])
            data.extend(payload)
        elif line.startswith('S1'):
            addr = int(line[4:8], 16)
            if base_addr is None:
                base_addr = addr
            payload = bytes.fromhex(line[8:-2])
            data.extend(payload)
        elif line.startswith('S7'):
            entry = int(line[4:12], 16)
        elif line.startswith('S9'):
            entry = int(line[4:8], 16)

    if base_addr is None:
        base_addr = 0x200

    decb = bytearray()
    decb.append(0x00)
    decb.append((len(data) >> 8) & 0xFF)
    decb.append(len(data) & 0xFF)
    decb.append((base_addr >> 8) & 0xFF)
    decb.append(base_addr & 0xFF)
    decb.extend(data)

    decb.append(0xFF)
    decb.append(0x00)
    decb.append(0x00)
    decb.append((entry >> 8) & 0xFF)
    decb.append(entry & 0xFF)

    with open(decb_path, 'wb') as f:
        f.write(decb)

if __name__ == '__main__':
    if len(sys.argv) < 3:
        print("Usage: srec2decb.py input.srec output.decb", file=sys.stderr)
        sys.exit(1)
    srec2decb(sys.argv[1], sys.argv[2])
