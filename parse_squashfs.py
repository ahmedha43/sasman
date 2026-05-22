import struct
import os

# Read the raw squashfs image
raw_file = r'D:\SASMAN\mikrotik_manager\extracted\npkPyExport_user-manager-7.21.4-arm64\008_cnt_CntSquashFsImage.raw'
with open(raw_file, 'rb') as f:
    data = f.read()

# SquashFS superblock is at offset 0
# Magic number check
magic = data[0:4]
print(f"Magic bytes: {magic.hex()}")
print(f"Expected SQUASHFS_MAGIC: 73717368 (hsqs in little endian)")

# Parse superblock (SquashFS 4.0 format)
# struct squashfs_super_block {
#   __le32 s_magic;
#   __le32 s_block_size;
#   __le32 s_layout;
#   __le32 s_n_uuids;
#   __le16 s_major;
#   __le16 s_minor;
#   __le64 s_root_inode;
#   __le64 s_bytesused;
#   __le64 s_ninodes;
#   __le64 s_nfolders;
#   __le64 s_nblocks;
#   __le64 s_nfragment;
#   __le64 s_nfragentries;
# };

major = struct.unpack('<H', data[16:18])[0]
minor = struct.unpack('<H', data[18:20])[0]
block_size = struct.unpack('<I', data[4:8])[0]
n_inodes = struct.unpack('<Q', data[32:40])[0]
n_folders = struct.unpack('<Q', data[40:48])[0]
bytes_used = struct.unpack('<Q', data[24:32])[0]

print(f"\n=== SquashFS Superblock ===")
print(f"Major version: {major}")
print(f"Minor version: {minor}")
print(f"Block size: {block_size}")
print(f"Total bytes used: {bytes_used}")
print(f"Total inodes: {n_inodes}")
print(f"Total folders: {n_folders}")

# Export raw data for analysis
export_dir = r'D:\SASMAN\mikrotik_manager\extracted\squashfs_analysis'
os.makedirs(export_dir, exist_ok=True)

# Save hex dump of first 1024 bytes
with open(os.path.join(export_dir, 'header_hex.txt'), 'w') as f:
    for i in range(0, min(1024, len(data)), 16):
        hex_str = ' '.join(f'{b:02x}' for b in data[i:i+16])
        ascii_str = ''.join(chr(b) if 32 <= b < 127 else '.' for b in data[i:i+16])
        f.write(f'{i:08x}: {hex_str:<48} {ascii_str}\n')

print(f"\nHeader hex dump saved to: {export_dir}/header_hex.txt")