#!/usr/bin/env python3
"""
Extract and analyze User Manager NPK
"""

import subprocess
import os
import struct

npk_file = r'D:\SASMAN\mikrotik_manager\user-manager-7.21.4-arm64.npk'
work_dir = r'D:\SASMAN\mikrotik_manager\extracted'

# Step 1: Show container info (already done, but let's capture it)
result = subprocess.run(['npkpy', '--files', npk_file, '--show-container'], 
                       capture_output=True, text=True)
print("=== NPK Container Info ===")
print(result.stdout)

# Step 2: Extract all containers
subprocess.run(['npkpy', '--files', npk_file, '--export-all', '--dst-folder', work_dir],
               capture_output=True)

# Step 3: List extracted files
print("\n=== Extracted Files ===")
for root, dirs, files in os.walk(work_dir):
    for f in files:
        full_path = os.path.join(root, f)
        rel_path = os.path.relpath(full_path, work_dir)
        size = os.path.getsize(full_path)
        print(f"{rel_path}: {size} bytes")