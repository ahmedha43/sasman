@echo off
title SASMAN Appliance Custom Image Builder
cls
echo =====================================================================
echo       SASMAN TV Box OS Image Builder (Windows Privileged Docker)
echo =====================================================================
echo.
echo  [!] IMPORTANT REQUIREMENTS:
echo  1. Make sure Docker Desktop is RUNNING on your Windows machine.
echo  2. Download any base Armbian .img file for your TV Box model
echo     (e.g., from ophub repository) and PLACE it inside this folder:
echo     "%cd%"
echo.
echo  This script will automatically inject SASMAN, pre-load the Docker image,
echo  configure performance tweaks, disable GUI, and output a ready-to-flash 
echo  "sasman-appliance.img" that you can write directly via Rufus!
echo.
echo =====================================================================
echo Press any key to start the image creation...
pause > nul
echo.

echo [*] Starting builder container via Docker (WSL 2 backend)...
docker run --privileged --rm -it -v "%cd%:/workspace" ubuntu:22.04 bash -c "apt-get update && apt-get install -y util-linux udev && chmod +x /workspace/tools/appliance_seeder/seed.sh && /workspace/tools/appliance_seeder/seed.sh"

echo.
echo =====================================================================
echo  [✔] Done! If successful, flash the generated "sasman-appliance.img"
echo      located in this folder directly onto your TV Box SD Card/eMMC.
echo =====================================================================
echo.
pause
