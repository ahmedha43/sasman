#!/usr/bin/env python3
"""
Extract SquashFS from User Manager NPK and analyze contents
"""

import lzma
import struct
import os
from pathlib import Path

# Read the raw SquashFS image from npkpy extraction
squashfs_raw = r'D:\SASMAN\mikrotik_manager\extracted\npkPyExport_user-manager-7.21.4-arm64\008_cnt_CntSquashFsImage.raw'
output_dir = Path(r'D:\SASMAN\mikrotik_manager\extracted\squashfs_extracted')
output_dir.mkdir(parents=True, exist_ok=True)

with open(squashfs_raw, 'rb') as f:
    data = f.read()

print(f"SquashFS raw size: {len(data)} bytes")
print(f"Magic: {data[:4]}")

# The SquashFS contains XZ-compressed data
# Find XZ signature (FD 37 7A 58 5A)
xz_sig = b'\xfd7zXZ\x00'
xz_pos = data.find(xz_sig)
print(f"XZ signature at offset: {xz_pos}")

# The SquashFS superblock is at the beginning
# Let's parse it properly first
print("\n=== SquashFS Superblock (raw) ===")
# Magic already checked
# Bytes 4-7: number of bytes in the archive (inflation size if compressed)
arch_bytes = struct.unpack('<I', data[4:8])[0]
print(f"Archive bytes (block size?): {arch_bytes}")

# Bytes 8-11: number of fragments
n_fragments = struct.unpack('<I', data[8:12])[0]
print(f"Fragments?: {n_fragments}")

# Let's try a different approach - the file might be stored as-is
# with XZ-compressed data embedded

# Extract and decompress XZ portion
if xz_pos >= 0:
    xz_data = data[xz_pos:]
    print(f"\nDecompressing XZ data ({len(xz_data)} bytes)...")
    
    try:
        decompressed = lzma.decompress(xz_data, format=lzma.FORMAT_AUTO)
        print(f"Decompressed: {len(decompressed)} bytes")
        
        # Check if it's a filesystem
        if decompressed[:4] == b'hsqs':
            print("Valid SquashFS filesystem!")
            # This is the uncompressed SquashFS - need to parse it
            
            with open(output_dir / 'uncompressed.squashfs', 'wb') as f:
                f.write(decompressed)
            
            # Parse SquashFS superblock
            print("\n=== Parsed SquashFS Superblock ===")
            major = struct.unpack('<H', decompressed[24:26])[0]
            minor = struct.unpack('<H', decompressed[26:28])[0]
            block_size = struct.unpack('<I', decompressed[8:12])[0]
            n_inodes = struct.unpack('<I', decompressed[32:36])[0] if len(decompressed) > 36 else 0
            n_folders = struct.unpack('<I', decompressed[36:40])[0] if len(decompressed) > 40 else 0
            
            print(f"Version: {major}.{minor}")
            print(f"Block size: {block_size}")
            print(f"Inodes: {n_inodes}")
            print(f"Folders: {n_folders}")
            
            # Write hex dump
            with open(output_dir / 'squashfs_hex.txt', 'w') as f:
                for i in range(0, min(2048, len(decompressed)), 16):
                    hex_str = ' '.join(f'{b:02x}' for b in decompressed[i:i+16])
                    ascii_str = ''.join(chr(b) if 32 <= b < 127 else '.' for b in decompressed[i:i+16])
                    f.write(f'{i:08x}: {hex_str:<48} {ascii_str}\n')
            
        else:
            print(f"Unknown format after decompression: {decompressed[:16].hex()}")
            # Save anyway
            with open(output_dir / 'unknown_decompressed.bin', 'wb') as f:
                f.write(decompressed)
                
    except Exception as e:
        print(f"Decompression failed: {e}")
        
        # Try raw data as SquashFS
        print("\nTrying raw SquashFS parsing...")
        # Save raw for analysis
        with open(output_dir / 'raw_squashfs.bin', 'wb') as f:
            f.write(data)

print(f"\nOutput saved to: {output_dir}")