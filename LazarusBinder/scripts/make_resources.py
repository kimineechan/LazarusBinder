#!/usr/bin/env python3
"""Create an AMD64 COFF resource object for Go's Windows linker; no windres needed."""
import argparse
import struct
from pathlib import Path


def resource_section(ico: bytes, manifest: bytes):
    count = struct.unpack_from('<H', ico, 4)[0]
    group = bytearray(struct.pack('<HHH', 0, 1, count))
    resources = {3: {}, 14: {}, 24: {1: manifest}}
    for index in range(count):
        entry = ico[6 + index * 16:22 + index * 16]
        size, offset = struct.unpack_from('<II', entry, 8)
        resources[3][index + 1] = ico[offset:offset + size]
        group += entry[:12] + struct.pack('<H', index + 1)
    resources[14][1] = bytes(group)
    section = bytearray()
    relocations = []

    def reserve(size):
        offset = len(section)
        section.extend(b'\0' * size)
        return offset

    def directory(keys):
        keys = sorted(keys)
        offset = reserve(16 + 8 * len(keys))
        struct.pack_into('<HH', section, offset + 12, 0, len(keys))
        return offset, keys

    root, types = directory(resources)
    leaves = []
    for ti, kind in enumerate(types):
        typed, names = directory(resources[kind])
        struct.pack_into('<II', section, root + 16 + ti * 8, kind, typed | 0x80000000)
        for ni, name in enumerate(names):
            langdir, _ = directory([0])
            struct.pack_into('<II', section, typed + 16 + ni * 8, name, langdir | 0x80000000)
            leaf = reserve(16)
            struct.pack_into('<II', section, langdir + 16, 0, leaf)
            leaves.append((leaf, resources[kind][name]))
    for leaf, blob in leaves:
        while len(section) % 4:
            section.append(0)
        offset = len(section)
        section.extend(blob)
        struct.pack_into('<IIII', section, leaf, offset, len(blob), 0, 0)
        relocations.append(leaf)
    return bytes(section), relocations


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('ico', type=Path)
    parser.add_argument('manifest', type=Path)
    parser.add_argument('output', type=Path)
    args = parser.parse_args()
    data, relocs = resource_section(args.ico.read_bytes(), args.manifest.read_bytes())
    raw_offset = 60
    reloc_offset = raw_offset + len(data)
    symbol_offset = reloc_offset + 10 * len(relocs)
    header = struct.pack('<HHIIIHH', 0x8664, 1, 0, symbol_offset, 1, 0, 0)
    section = struct.pack('<8sIIIIIIHHI', b'.rsrc\0\0\0', 0, 0, len(data), raw_offset,
                          reloc_offset, 0, len(relocs), 0, 0x40000040)
    reloc_bytes = b''.join(struct.pack('<IIH', offset, 0, 3) for offset in relocs)
    symbol = struct.pack('<8sIhHBB', b'.rsrc\0\0\0', 0, 1, 0, 3, 0)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(header + section + data + reloc_bytes + symbol + struct.pack('<I', 4))
    print('Resource object:', args.output)


if __name__ == '__main__':
    main()
