# SASMAN Container Images & Release Builder
param (
    [string]$Version = "5.1.0"
)

$ErrorActionPreference = "Stop"
Write-Host "===============================================================================" -ForegroundColor Cyan
Write-Host "           SASMAN MikroTik RouterOS Container Images Builder                   " -ForegroundColor Green
Write-Host "===============================================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "[*] Target Version: $Version" -ForegroundColor Yellow

$releasesDir = Join-Path $PSScriptRoot "releases"
if (!(Test-Path $releasesDir)) {
    New-Item -ItemType Directory -Path $releasesDir | Out-Null
    Write-Host "[+] Created output folder: $releasesDir" -ForegroundColor Green
}

$targets = @(
    @{ Name = "armv7"; Platform = "linux/arm/v7"; Desc = "MikroTik RB4011, hAP ax2/ax3, 32-bit ARM"; OutputTar = "sasman-armv7.tar" },
    @{ Name = "arm64"; Platform = "linux/arm64";  Desc = "MikroTik CCR2004, RB5009, L009, 64-bit ARM"; OutputTar = "sasman-arm64.tar" },
    @{ Name = "amd64"; Platform = "linux/amd64";  Desc = "MikroTik CHR, x86_64 PC, Servers"; OutputTar = "sasman-amd64.tar" }
)

$step = 1
foreach ($t in $targets) {
    Write-Host ""
    Write-Host "[$step/3] Building Container Image for $($t.Desc)..." -ForegroundColor Cyan
    
    $tag = "sasman:$($t.Name)"
    $tarPath = Join-Path $releasesDir $t.OutputTar
    
    Write-Host "  -> Running docker buildx build for $($t.Platform)..." -ForegroundColor Gray
    & docker buildx build --platform $($t.Platform) -f agent/Dockerfile -t $tag --load .
    
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  [ERR] Build failed for $($t.Name)" -ForegroundColor Red
        continue
    }
    
    Write-Host "  -> Saving image to $tarPath..." -ForegroundColor Gray
    if (Test-Path $tarPath) { Remove-Item $tarPath -Force }
    & docker save $tag -o $tarPath
    
    if ($LASTEXITCODE -eq 0) {
        $sizeMB = [math]::Round((Get-Item $tarPath).Length / 1MB, 1)
        Write-Host "  [OK] Generated $($t.OutputTar) ($sizeMB MB)" -ForegroundColor Green
    } else {
        Write-Host "  [ERR] Failed to save $($t.OutputTar)" -ForegroundColor Red
    }
    $step++
}

Write-Host ""
Write-Host "===============================================================================" -ForegroundColor Cyan
Write-Host "                      CONTAINER IMAGES READY!                                  " -ForegroundColor Green
Write-Host "===============================================================================" -ForegroundColor Cyan
Write-Host ""
Get-ChildItem -Path $releasesDir -Filter "*.tar" | Select-Object Name, @{Name="Size(MB)";Expression={[math]::Round($_.Length / 1MB, 2)}}, LastWriteTime | Format-Table -AutoSize
Write-Host "All RouterOS container images are ready in: $releasesDir" -ForegroundColor Yellow
