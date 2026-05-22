import struct
import os

decompressed_file = r'D:\SASMAN\mikrotik_manager\extracted\squashfs_decompressed\squashfs_decompressed.raw'

with open(decompressed_file, 'rb') as f:
    data = f.read()

print(f"Total size: {len(data)} bytes")
print(f"\n=== ELF Header ===")
# ELF magic
print(f"Magic: {data[:4].hex()} (should be 7f454c46)")
# EI_CLASS (1 = 32-bit, 2 = 64-bit)
ei_class = data[4]
print(f"Class: {'32-bit' if ei_class == 1 else '64-bit' if ei_class == 2 else ei_class}")
# EI_DATA (1 = little endian, 2 = big endian)
ei_data = data[5]
print(f"Endianness: {'little' if ei_data == 1 else 'big'}")
# EI_VERSION
print(f"ELF version: {data[6]}")
# EI_OSABI
osabi = data[7]
print(f"OS/ABI: {osabi} ({'Linux' if osabi == 3 else 'System V' if osabi == 0 else 'Other'})")

# Type
e_type = struct.unpack('<H', data[16:18])[0]
print(f"Type: {e_type} ({'Executable' if e_type == 2 else 'Shared object' if e_type == 3 else 'Relocatable'})")

# Machine
e_machine = struct.unpack('<H', data[18:20])[0]
arch = 'ARM64/AArch64' if e_machine == 183 else 'x86-64' if e_machine == 62 else 'i386' if e_machine == 3 else 'Unknown'
print(f"Machine: {e_machine} ({arch})")

# Entry point
e_entry = struct.unpack('<Q', data[24:32])[0]
print(f"Entry point: 0x{e_entry:x}")

# Section header offset
e_shoff = struct.unpack('<Q', data[32:40])[0]
print(f"Section header offset: 0x{e_shoff:x}")

# Program header offset
e_phoff = struct.unpack('<Q', data[40:48])[0]
print(f"Program header offset: 0x{e_phoff:x}")

# Write hex dump of header area
output_dir = r'D:\SASMAN\mikrotik_manager\extracted\elf_analysis'
os.makedirs(output_dir, exist_ok=True)

with open(os.path.join(output_dir, 'elf_header_hex.txt'), 'w') as f:
    for i in range(0, min(512, len(data)), 16):
        hex_str = ' '.join(f'{b:02x}' for b in data[i:i+16])
        ascii_str = ''.join(chr(b) if 32 <= b < 127 else '.' for b in data[i:i+16])
        f.write(f'{i:08x}: {hex_str:<48} {ascii_str}\n')

print(f"\nSaved header hex to: {output_dir}/elf_header_hex.txt")