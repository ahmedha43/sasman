@echo off
echo Building SASMAN MikroTik Manager with optimizations...

:: Enable Go Modules
set GO111MODULE=on

:: Build the unified executable with debug symbols stripped
:: -s removes symbol table and debugging information
:: -w removes DWARF debugging information
go build -ldflags="-s -w" -o sasman_optimized.exe main.go

echo Build complete!
echo Previous sizes:
dir sasman_unified.exe | findstr "sasman"
echo New optimized size:
dir sasman_optimized.exe | findstr "sasman"
