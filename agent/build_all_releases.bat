@echo off
setlocal enabledelayedexpansion
title SASMAN MikroTik Container Images Builder
cls

echo ===============================================================================
echo           SASMAN MikroTik RouterOS Container Images Builder
echo ===============================================================================
echo.

if not exist "releases" mkdir releases

echo [1/3] Building Container Image for linux/arm/v7 (MikroTik RB4011, hAP ax2/ax3)...
docker buildx build --platform linux/arm/v7 -t sasman:armv7 --load .
if %ERRORLEVEL% EQU 0 (
    docker save sasman:armv7 -o releases\sasman-armv7.tar
    echo   [OK] Generated releases\sasman-armv7.tar
) else (
    echo   [ERR] Failed armv7 build
)

echo.
echo [2/3] Building Container Image for linux/arm64 (MikroTik CCR2004, RB5009, L009)...
docker buildx build --platform linux/arm64 -t sasman:arm64 --load .
if %ERRORLEVEL% EQU 0 (
    docker save sasman:arm64 -o releases\sasman-arm64.tar
    echo   [OK] Generated releases\sasman-arm64.tar
) else (
    echo   [ERR] Failed arm64 build
)

echo.
echo [3/3] Building Container Image for linux/amd64 (MikroTik CHR, x86_64 PC)...
docker buildx build --platform linux/amd64 -t sasman:amd64 --load .
if %ERRORLEVEL% EQU 0 (
    docker save sasman:amd64 -o releases\sasman-amd64.tar
    echo   [OK] Generated releases\sasman-amd64.tar
) else (
    echo   [ERR] Failed amd64 build
)

echo.
echo ===============================================================================
echo                      CONTAINER IMAGES BUILD COMPLETE!
echo ===============================================================================
echo.
dir releases\*.tar /b
echo.
pause
