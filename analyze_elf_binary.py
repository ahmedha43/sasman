#!/usr/bin/env python3
"""
Analyze the extracted ELF binary from User Manager
"""

import struct
import os
from pathlib import Path

elf_file = r'D:\SASMAN\mikrotik_manager\extracted\squashfs_extracted\unknown_decompressed.bin'
output_dir = Path(r'D:\SASMAN\mikrotik_manager\extracted\elf_analysis')
output_dir.mkdir(parents=True, exist_ok=True)

with open(elf_file, 'rb') as f:
    data = f.read()

print(f"ELF binary size: {len(data)} bytes")

# Parse ELF header
e_ident = data[:16]
e_type = struct.unpack('<H', data[16:18])[0]
e_machine = struct.unpack('<H', data[18:20])[0]
e_version = struct.unpack('<I', data[20:24])[0]
e_entry = struct.unpack('<I', data[24:28])[0]
e_phoff = struct.unpack('<I', data[32:36])[0]
e_shoff = struct.unpack('<I', data[40:44])[0]
e_flags = struct.unpack('<I', data[44:48])[0]
e_ehsize = struct.unpack('<H', data[48:50])[0]
e_phentsize = struct.unpack('<H', data[52:54])[0]
e_phnum = struct.unpack('<H', data[54:56])[0]
e_shentsize = struct.unpack('<H', data[56:58])[0]
e_shnum = struct.unpack('<H', data[58:60])[0]
e_shstrndx = struct.unpack('<H', data[60:62])[0]

ARCH_MAP = {0x28: 'ARM', 0x03: 'x86', 0x3E: 'x86-64', 0xB7: 'AArch64'}
TYPE_MAP = {1: 'Relocatable', 2: 'Executable', 3: 'Shared', 4: 'Core'}

print(f"\n=== ELF Header ===")
print(f"Magic: {e_ident[:4]}")
print(f"Class: {'32-bit' if e_ident[4] == 1 else '64-bit'}")
print(f"Endianness: {'little' if e_ident[5] == 1 else 'big'}")
print(f"Version: {e_version}")
print(f"Type: {TYPE_MAP.get(e_type, e_type)}")
print(f"Machine: {ARCH_MAP.get(e_machine, hex(e_machine))}")
print(f"Entry point: 0x{e_entry:x}")
print(f"Program headers: {e_phnum} entries at 0x{e_phoff:x}")
print(f"Section headers: {e_shnum} entries at 0x{e_shoff:x}")

# Parse program headers
print(f"\n=== Program Headers ===")
for i in range(e_phnum):
    offset = e_phoff + i * e_phentsize
    p_type = struct.unpack('<I', data[offset:offset+4])[0]
    p_offset = struct.unpack('<I', data[offset+4:offset+8])[0]
    p_vaddr = struct.unpack('<I', data[offset+8:offset+12])[0]
    p_filesz = struct.unpack('<I', data[offset+12:offset+16])[0]
    p_memsz = struct.unpack('<I', data[offset+16:offset+20])[0]
    p_flags = struct.unpack('<I', data[offset+20:offset+24])[0]
    
    TYPE_STR = {1: 'LOAD', 2: 'DYNAMIC', 3: 'INTERP', 4: 'NOTE', 5: 'SHLIB', 6: 'PHDR'}
    flags_str = ''.join([c for c, f in zip('R X W', [4, 1, 2]) if p_flags & f])
    
    print(f"  {i}: {TYPE_STR.get(p_type, p_type)} at 0x{p_offset:x}, vaddr=0x{p_vaddr:x}, size={p_filesz}")

# Look for strings
print(f"\n=== Interesting Strings ===")
strings = []
current = b''
for byte in data:
    if 32 <= byte < 127:
        current += bytes([byte])
    else:
        if len(current) >= 4:
            try:
                s = current.decode('ascii')
                if any(kw in s.lower() for kw in ['radius', 'sql', 'user', 'manager', 'freeradius', 'sqlite', '/db', '.so', 'http', 'ssl', 'thread', 'pool']):
                    strings.append(s)
            except:
                pass
        current = b''

# Remove duplicates and show unique
unique_strings = list(dict.fromkeys(strings))[:50]
for s in unique_strings:
    print(f"  {s}")

# Save hex dump of header
with open(output_dir / 'elf_hex.txt', 'w') as f:
    for i in range(0, min(4096, len(data)), 16):
        hex_str = ' '.join(f'{b:02x}' for b in data[i:i+16])
        ascii_str = ''.join(chr(b) if 32 <= b < 127 else '.' for b in data[i:i+16])
        f.write(f'{i:08x}: {hex_str:<48} {ascii_str}\n')

print(f"\nHex dump saved to: {output_dir}/elf_hex.txt")