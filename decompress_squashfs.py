import lzma
import os

raw_file = r'D:\SASMAN\mikrotik_manager\extracted\npkPyExport_user-manager-7.21.4-arm64\008_cnt_CntSquashFsImage.raw'
output_dir = r'D:\SASMAN\mikrotik_manager\extracted\squashfs_decompressed'
os.makedirs(output_dir, exist_ok=True)

with open(raw_file, 'rb') as f:
    data = f.read()

# Try to find XZ stream and decompress
# XZ magic: FD 37 7A 58 5A 00
xz_offset = data.find(b'\xfd7zXZ\x00')
print(f"XZ signature found at offset: {xz_offset}")

if xz_offset >= 0:
    xz_data = data[xz_offset:]
    try:
        decompressed = lzma.decompress(xz_data)
        output_file = os.path.join(output_dir, 'squashfs_decompressed.raw')
        with open(output_file, 'wb') as f:
            f.write(decompressed)
        print(f"Decompressed {len(decompressed)} bytes")
        print(f"Saved to: {output_file}")
        
        # Show first bytes
        print(f"\nFirst 64 bytes of decompressed:")
        print(decompressed[:64].hex())
    except Exception as e:
        print(f"LZMA decompression failed: {e}")
        # Try raw deflate
        import zlib
        try:
            # Find deflate stream
            decompressed = zlib.decompress(data[xz_offset:], -15)
            print(f"Deflate decompressed: {len(decompressed)} bytes")
        except Exception as e2:
            print(f"Deflate also failed: {e2}")
else:
    print("XZ signature not found, trying raw decompression")
    # Maybe it's already raw squashfs
    with open(os.path.join(output_dir, 'raw_copy.raw'), 'wb') as f:
        f.write(data)
    print("Saved raw copy")