#!/usr/bin/env python3
"""
User Manager NPK Analysis Script
Analyzes user-manager-7.21.4-arm64.npk package structure
"""

import struct
import os
import json
from pathlib import Path

npk_path = r'D:\SASMAN\mikrotik_manager\user-manager-7.21.4-arm64.npk'
output_dir = Path(r'D:\SASMAN\mikrotik_manager\extracted\analysis')
output_dir.mkdir(parents=True, exist_ok=True)

with open(npk_path, 'rb') as f:
    data = f.read()

print("=" * 60)
print("User Manager NPK Analysis Report")
print("=" * 60)

# Parse NPK structure manually
pos = 0

# Pre-header
pre_header = {
    'signature': data[pos:pos+4],
    'version': struct.unpack('<H', data[pos+4:pos+6])[0],
    'container_count': struct.unpack('<H', data[pos+6:pos+8])[0]
}
print(f"\n1. NPK Signature: {pre_header['signature']}")
print(f"   Version: {pre_header['version']}")
print(f"   Container Count: {pre_header['container_count']}")

pos = 8

# Parse containers
containers = []
for i in range(pre_header['container_count']):
    # Each container header: 4 bytes id + 4 bytes offset + 4 bytes length
    cont_id = struct.unpack('<I', data[pos:pos+4])[0]
    cont_offset = struct.unpack('<I', data[pos+4:pos+8])[0]
    cont_len = struct.unpack('<I', data[pos+8:pos+12])[0]
    
    container = {
        'id': cont_id,
        'offset': cont_offset,
        'length': cont_len,
        'payload': data[cont_offset:cont_offset+cont_len]
    }
    
    # Identify container type by ID
    CONTAINER_TYPES = {
        25: 'PckPreHeader',
        1: 'PckHeader',
        24: 'PckReleaseTyp',
        16: 'CntArchitectureTag',
        2: 'PckDescription',
        23: 'PckEckcdsaHash',
        3: 'PckRequirementsHeader',
        22: 'CntNullBlock',
        21: 'CntSquashFsImage',
        9: 'CntSquashFsHashSignature'
    }
    
    container['type'] = CONTAINER_TYPES.get(cont_id, f'Unknown({cont_id})')
    containers.append(container)
    
    print(f"\n2.{i+1}. Container #{i}: {container['type']}")
    print(f"    Offset: 0x{cont_offset:x}, Length: {cont_len}")

# Extract key information
print("\n" + "=" * 60)
print("Package Details")
print("=" * 60)

# PckHeader (container 1)
pck_header = containers[1]['payload']
name_end = pck_header.find(b'\x00')
package_name = pck_header[:name_end].decode('utf-8', errors='replace')
os_version_start = name_end + 1
os_version_end = pck_header.find(b'\x00', os_version_start)
os_version = pck_header[os_end:os_version_end].decode('utf-8', errors='replace') if os_version_end > os_end else 'unknown'

# Actually parse properly
print(f"\nPackage Name: {package_name}")
print(f"OS Version: Read from header")

# Architecture
arch_container = [c for c in containers if c['type'] == 'CntArchitectureTag'][0]
architecture = arch_container['payload'].decode('utf-8', errors='replace')
print(f"Architecture: {architecture}")

# Description
desc_container = [c for c in containers if c['type'] == 'PckDescription'][0]
description = desc_container['payload'].decode('utf-8', errors='replace')
print(f"Description: {description[:50]}...")

# SquashFS info
squashfs_container = [c for c in containers if c['type'] == 'CntSquashFsImage'][0]
squashfs_data = squashfs_container['payload']
print(f"\nSquashFS Image: {len(squashfs_data)} bytes")

# Save SquashFS for further analysis
with open(output_dir / 'squashfs_raw.bin', 'wb') as f:
    f.write(squashfs_data)

# Analyze SquashFS superblock
print("\n" + "=" * 60)
print("SquashFS Analysis")
print("=" * 60)

# SquashFS magic at offset 0
magic = squashfs_data[0:4]
print(f"Magic: {magic.hex()} (hsqs = {magic == b'hsqs'})")

# SquashFS 4.0 superblock
s_major = struct.unpack('<H', squashfs_data[24:26])[0]
s_minor = struct.unpack('<H', squashfs_data[26:28])[0]
block_size = struct.unpack('<I', squashfs_data[8:12])[0]
print(f"Version: {s_major}.{s_minor}")
print(f"Block Size: {block_size}")

# Find XZ compression start
xz_pos = squashfs_data.find(b'7zXZ')
if xz_pos >= 0:
    xz_pos -= 1  # Back up to include FD
    print(f"XZ compression found at offset: {xz_pos}")
    
    # Decompress XZ
    import lzma
    try:
        decompressed = lzma.decompress(squashfs_data[xz_pos:])
        print(f"Decompressed size: {len(decompressed)} bytes")
        
        # Save decompressed
        with open(output_dir / 'squashfs_decompressed.bin', 'wb') as f:
            f.write(decompressed)
        
        # Check if it's an archive or filesystem
        if decompressed[:4] == b'hsqs':
            print("Valid SquashFS filesystem found!")
        else:
            # Try to identify
            print(f"Decompressed magic: {decompressed[:16].hex()}")
    except Exception as e:
        print(f"XZ decompression error: {e}")

# Save analysis report
report = {
    'package_name': package_name,
    'architecture': architecture,
    'description': description,
    'container_count': len(containers),
    'containers': [{'id': c['id'], 'type': c['type'], 'offset': c['offset'], 'length': c['length']} for c in containers],
    'squashfs_size': len(squashfs_data)
}

with open(output_dir / 'analysis_report.json', 'w') as f:
    json.dump(report, f, indent=2)

print(f"\nAnalysis saved to: {output_dir}")