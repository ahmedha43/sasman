#!/bin/bash

# ==============================================================================
# SASMAN Appliance Image Builder & Seeder Script (Ubuntu/Debian Container)
# ==============================================================================

set -e

# Colors for terminal output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}======================================================================${NC}"
echo -e "${GREEN}       SASMAN Appliance Image Seeder (Armbian TV Box Customizer)${NC}"
echo -e "${BLUE}======================================================================${NC}"

# Check for root privilege
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}[!] Error: This script must be run as root (privileged mode).${NC}"
  exit 1
fi

# Search for any .img files in the mounted workspace
IMG_FILE=$(ls /workspace/*.img 2>/dev/null | head -n 1)

if [ -z "$IMG_FILE" ]; then
  echo -e "${RED}[!] Error: No base Armbian .img file found in the workspace directory.${NC}"
  echo -e "${YELLOW}[i] Please download an Armbian image for your TV Box and place it in the project directory.${NC}"
  exit 1
fi

echo -e "${GREEN}[+] Found base Armbian image:${NC} $(basename "$IMG_FILE")"

# Copy base image to a temporary file in the workspace to avoid modifying the original
APPLIANCE_IMG="/workspace/sasman-appliance.img"
echo -e "${YELLOW}[*] Creating a copy of the base image as 'sasman-appliance.img'...${NC}"
cp "$IMG_FILE" "$APPLIANCE_IMG"
echo -e "${GREEN}[+] Copy created successfully.${NC}"

# Setup loop device
echo -e "${YELLOW}[*] Attaching the image to a loop device...${NC}"
# Probe partitions (-P)
LOOP_DEV=$(losetup -fP --show "$APPLIANCE_IMG")
echo -e "${GREEN}[+] Image attached to ${LOOP_DEV}${NC}"

# Wait a second for partition tables to settle
sleep 2

# Find partitions (Armbian typically has one or two partitions)
# If two: Partition 1 is Boot (FAT), Partition 2 is Rootfs (EXT4)
# If one: Partition 1 is Rootfs (EXT4)
PART_COUNT=$(ls ${LOOP_DEV}p* 2>/dev/null | wc -l)
ROOT_PART=""

if [ "$PART_COUNT" -eq 1 ]; then
  ROOT_PART="${LOOP_DEV}p1"
elif [ "$PART_COUNT" -eq 2 ]; then
  ROOT_PART="${LOOP_DEV}p2"
else
  echo -e "${RED}[!] Error: Unexpected number of partitions ($PART_COUNT).${NC}"
  losetup -d "$LOOP_DEV"
  exit 1
fi

echo -e "${GREEN}[+] Identified Root partition:${NC} $ROOT_PART"

# Mount Rootfs
MOUNT_DIR="/mnt/sasman_rootfs"
mkdir -p "$MOUNT_DIR"
echo -e "${YELLOW}[*] Mounting root filesystem to $MOUNT_DIR...${NC}"
mount "$ROOT_PART" "$MOUNT_DIR"
echo -e "${GREEN}[+] Root filesystem mounted successfully.${NC}"

# Create directories inside rootfs
SASMAN_DIR="$MOUNT_DIR/opt/sasman"
mkdir -p "$SASMAN_DIR"

# 1. Inject docker-compose.yml
echo -e "${YELLOW}[*] Injecting docker-compose.yml...${NC}"
cat << 'EOF' > "$SASMAN_DIR/docker-compose.yml"
services:
  sasman-app:
    image: ahmedkin99/sasman-manager:latest
    container_name: sasman-server
    restart: always
    network_mode: host
    volumes:
      - ./data:/app/data
    environment:
      - PORT=80
      - GODEBUG=x509negativeserial=1
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
EOF

# 2. Check if a pre-compiled tar package of SASMAN exists in the workspace
TAR_FILE=""
if [ -f "/workspace/sasman-manager-arm.tar" ]; then
  TAR_FILE="/workspace/sasman-manager-arm.tar"
elif [ -f "/workspace/sasman-manager.tar" ]; then
  TAR_FILE="/workspace/sasman-manager.tar"
fi

if [ -n "$TAR_FILE" ]; then
  echo -e "${GREEN}[+] Found pre-compiled Docker image:${NC} $(basename "$TAR_FILE")"
  echo -e "${YELLOW}[*] Copying Docker image tar to the appliance filesystem (for offline install)...${NC}"
  cp "$TAR_FILE" "$SASMAN_DIR/sasman-manager.tar"
  echo -e "${GREEN}[+] Docker image pre-loaded successfully.${NC}"
else
  echo -e "${YELLOW}[i] Note: No pre-compiled sasman-manager.tar found. The appliance will pull it on first boot.${NC}"
fi

# 3. Inject firstboot.sh script
echo -e "${YELLOW}[*] Creating firstboot configuration script...${NC}"
cat << 'EOF' > "$SASMAN_DIR/firstboot.sh"
#!/bin/bash
set -e
echo "[$(date)] --- SASMAN First Boot Appliance Initialization ---"

# 1. Install Docker & Docker Compose
echo "[$(date)] Installing Docker..."
curl -fsSL https://get.docker.com -o /tmp/get-docker.sh
sh /tmp/get-docker.sh

# 2. Check if we have a pre-loaded offline Docker image
if [ -f "/opt/sasman/sasman-manager.tar" ]; then
  echo "[$(date)] Loading pre-loaded offline Docker image..."
  docker load -i /opt/sasman/sasman-manager.tar
  echo "[$(date)] Cleaning up offline image tarball..."
  rm -f /opt/sasman/sasman-manager.tar
else
  echo "[$(date)] Pulling latest SASMAN image from Docker Hub..."
  docker pull ahmedkin99/sasman-manager:latest
fi

# 3. Boot SASMAN container
echo "[$(date)] Starting SASMAN Appliance service..."
cd /opt/sasman
docker compose up -d

echo "[$(date)] SASMAN system is running and fully initialized."
EOF
chmod +x "$SASMAN_DIR/firstboot.sh"

# 4. Inject systemd firstboot service
echo -e "${YELLOW}[*] Creating systemd service for firstboot initialization...${NC}"
cat << 'EOF' > "$MOUNT_DIR/etc/systemd/system/sasman-firstboot.service"
[Unit]
Description=SASMAN First Boot Configuration
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/opt/sasman/firstboot.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF

# Enable the firstboot service by symlinking
mkdir -p "$MOUNT_DIR/etc/systemd/system/multi-user.target.wants"
ln -sf /etc/systemd/system/sasman-firstboot.service "$MOUNT_DIR/etc/systemd/system/multi-user.target.wants/sasman-firstboot.service"

# 5. Inject Performance Tweaks directly into the filesystem
echo -e "${YELLOW}[*] Applying performance & stability tweaks...${NC}"

# Disable GUI (set default target to multi-user / CLI mode)
ln -sf /lib/systemd/system/multi-user.target "$MOUNT_DIR/etc/systemd/system/default.target"
echo -e "${GREEN}[+] GUI disabled. Default target set to CLI.${NC}"

# Set Governor to Performance
mkdir -p "$MOUNT_DIR/etc/default"
echo 'GOVERNOR="performance"' > "$MOUNT_DIR/etc/default/cpufrequtils"
echo -e "${GREEN}[+] CPU Governor configured to performance.${NC}"

# Setup tmpfs to protect SD/eMMC from burnout
if ! grep -q "tmpfs" "$MOUNT_DIR/etc/fstab"; then
  cat << 'EOF' >> "$MOUNT_DIR/etc/fstab"
tmpfs   /var/log    tmpfs   defaults,noatime,nosuid,mode=0755,size=50m    0   0
tmpfs   /tmp        tmpfs   defaults,noatime,nosuid,nodev,size=100m       0   0
EOF
  echo -e "${GREEN}[+] RAM Disk (tmpfs) added for /var/log and /tmp.${NC}"
fi

# Clean up and Unmount
echo -e "${YELLOW}[*] Unmounting root filesystem...${NC}"
sync
umount "$MOUNT_DIR"
rmdir "$MOUNT_DIR"
echo -e "${GREEN}[+] Root filesystem unmounted successfully.${NC}"

echo -e "${YELLOW}[*] Detaching loop device...${NC}"
losetup -d "$LOOP_DEV"
echo -e "${GREEN}[+] Loop device detached successfully.${NC}"

echo -e "${BLUE}======================================================================${NC}"
echo -e "${GREEN}[✔] SUCCESS: Custom SASMAN Appliance image generated successfully!${NC}"
echo -e "${GREEN}[✔] Output file: /workspace/sasman-appliance.img${NC}"
echo -e "${BLUE}======================================================================${NC}"
echo -e "${YELLOW}[i] You can now flash 'sasman-appliance.img' directly to your SD card using Rufus/Etcher.${NC}"
echo -e "${BLUE}======================================================================${NC}"
