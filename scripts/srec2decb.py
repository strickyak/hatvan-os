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
    # Tag 253 (0xFD): Hatvan Executable Magic header ('x', 'k')
    decb.extend([0xFD, 0x00, 0x00, ord('x'), ord('k')])
    cur_addr = base_addr
    offset = 0
    current_high16 = 0

    while offset < len(data):
        high16 = (cur_addr >> 16) & 0xFFFF
        if high16 != current_high16:
            # Tag 254: SET_HIGH16_ADDR32
            decb.append(0xFE)
            decb.append(0x00)
            decb.append(0x00)
            decb.append((high16 >> 8) & 0xFF)
            decb.append(high16 & 0xFF)
            current_high16 = high16

        # Chunk cannot cross 64KB boundary and max chunk length is 32768
        space_in_bank = 0x10000 - (cur_addr & 0xFFFF)
        chunk_len = min(len(data) - offset, space_in_bank, 32768)

        addr_low16 = cur_addr & 0xFFFF
        decb.append(0x00)
        decb.append((chunk_len >> 8) & 0xFF)
        decb.append(chunk_len & 0xFF)
        decb.append((addr_low16 >> 8) & 0xFF)
        decb.append(addr_low16 & 0xFF)
        decb.extend(data[offset : offset + chunk_len])

        cur_addr += chunk_len
        offset += chunk_len

    entry_high16 = (entry >> 16) & 0xFFFF
    if entry_high16 != current_high16:
        decb.append(0xFE)
        decb.append(0x00)
        decb.append(0x00)
        decb.append((entry_high16 >> 8) & 0xFF)
        decb.append(entry_high16 & 0xFF)
        current_high16 = entry_high16

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
