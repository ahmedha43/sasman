import npkpy.npk as npk
import os

npk_path = r'D:\SASMAN\mikrotik_manager\user-manager-7.21.4-arm64.npk'

# Read raw data
with open(npk_path, 'rb') as f:
    raw_data = f.read()

# Parse using npkpy internal API
from npkpy import npk as npk_module

# Use the internal parsing
result = npk_module.NpkParseFile(raw_data)

print("=== NPK Parsing Result ===")
for i, container in enumerate(result):
    print(f"\nContainer {i}: {container.containerType}")
    
# Get the SquashFS image
for container in result:
    if 'SquashFs' in str(container.containerType):
        print(f"\n=== Found SquashFS at index {result.index(container)} ===")
        # The raw data is in container.payload
        if hasattr(container, 'payload'):
            squashfs_data = container.payload
            print(f"SquashFS size: {len(squashfs_data)} bytes")
            
            # Save raw
            with open(r'D:\SASMAN\mikrotik_manager\extracted\raw_squashfs.img', 'wb') as f:
                f.write(squashfs_data)
            print("Saved raw SquashFS to extracted/raw_squashfs.img")