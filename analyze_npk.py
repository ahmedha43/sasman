import npkpy
import os

npk_path = r'D:\SASMAN\mikrotik_manager\user-manager-7.21.4-arm64.npk'

# Load the NPK file
with open(npk_path, 'rb') as f:
    data = f.read()

# Parse the NPK
package = npkpy.Npk(data)

print('=== NPK Package Info ===')
print(f'Name: {package.name}')
print(f'Version: {package.version}')
print(f'Architecture: {package.arch}')
print(f'Date: {package.date}')
print(f'Files count: {len(package.files)}')
print()

print('=== Files in Package ===')
for i, file_info in enumerate(package.files):
    print(f'{i+1}. {file_info}')