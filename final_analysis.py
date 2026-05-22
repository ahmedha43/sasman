#!/usr/bin/env python3
"""
Comprehensive User Manager NPK Analysis Report
"""

import struct
import os
from pathlib import Path

print("=" * 70)
print("USER MANAGER NPK ANALYSIS REPORT")
print("Package: user-manager-7.21.4-arm64.npk")
print("=" * 70)

# Read the ELf binary
elf_path = r'D:\SASMAN\mikrotik_manager\extracted\squashfs_extracted\unknown_decompressed.bin'
with open(elf_path, 'rb') as f:
    data = f.read()

print(f"\n1. BINARY INFORMATION")
print("-" * 40)
print(f"File Size: {len(data):,} bytes ({len(data)/1024/1024:.2f} MB)")
print(f"Magic: {''.join(chr(b) for b in data[:4])} (ELF)")

# Find all interesting strings
strings_found = {
    'radius': [],
    'sqlite': [],
    'user': [],
    'paypal': [],
    'http': [],
    'thread': [],
    'pool': []
}

current = b''
for byte in data:
    if 32 <= byte < 127:
        current += bytes([byte])
    else:
        if len(current) >= 4:
            s = current.decode('ascii', errors='ignore')
            for keyword in strings_found:
                if keyword in s.lower():
                    strings_found[keyword].append(s)
        current = b''

print(f"\n2. KEY COMPONENTS FOUND")
print("-" * 40)
for keyword, items in strings_found.items():
    if items:
        unique = list(dict.fromkeys(items))[:20]
        print(f"\n{keyword.upper()} related:")
        for item in unique[:10]:
            print(f"  - {item[:80]}")

print(f"\n3. FREE RADIUS INTEGRATION")
print("-" * 40)
radius_indicators = [
    "liburadius.so",
    "_ZN6radius",
    "RadPack",
    "RadiusCode",
    "verifyFullUserFilePath"
]
for ind in radius_indicators:
    if ind.encode() in data:
        print(f"  [FOUND] {ind}")

print(f"\n4. SQLITE STORAGE ANALYSIS")
print("-" * 40)
sqlite_indicators = [
    "sqlite",
    "SELECT",
    "INSERT",
    "UPDATE",
    "DELETE",
    "BEGIN TRANSACTION"
]
for ind in sqlite_indicators:
    count = data.count(ind.encode())
    if count > 0:
        print(f"  [FOUND] '{ind}' appears {count} times")

print(f"\n5. THREADING/POOLING MECHANISMS")
print("-" * 40)
thread_indicators = [
    "thread",
    "pthread",
    "pool",
    "worker",
    "async",
    "concurrent"
]
for ind in thread_indicators:
    count = data.lower().count(ind.encode())
    if count > 0:
        print(f"  [FOUND] '{ind}' appears {count} times")

print(f"\n6. PERFORMANCE OPTIMIZATIONS")
print("-" * 40)
perf_indicators = [
    b"BEGIN IMMEDIATE",
    b"COMMIT",
    b"wal",  # Write-Ahead Logging
    b"shared cache",
    b"mmap",
    b"busy_timeout"
]
for ind in perf_indicators:
    if ind in data:
        print(f"  [FOUND] {ind.decode()}")

print(f"\n7. LIBRARIES LINKED")
print("-" * 40)
libs = ["liburadius.so", "libubox.so", "libumsg.so", "libucrypto.so", 
        "libz.so", "libeap.so", "libwww.so", "libuc++.so"]
for lib in libs:
    if lib.encode() in data[:10000]:  # Found in first 10KB (strings section)
        print(f"  [LINKED] {lib}")

print(f"\n" + "=" * 70)
print("ANALYSIS COMPLETE")
print("=" * 70)