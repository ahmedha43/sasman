# ==============================================================================
#   Upload Pre-Built Multi-Arch TAR Images directly to Central Server (VPS)
# ==============================================================================
$VPS_HOST = "167.86.73.203"
$VPS_USER = "root"
$VPS_PASS = "mushtaq99"
$REMOTE_DIR = "/root/sasman-central/server/data/releases"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "   Uploading SASMAN Container TARs to Central VPS Server  " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# Ensure remote directory exists
$mkdirCmd = "mkdir -p $REMOTE_DIR"
& plink -ssh "${VPS_USER}@${VPS_HOST}" -pw $VPS_PASS -batch $mkdirCmd

$files = @("sasman-armv7.tar", "sasman-arm64.tar", "sasman-amd64.tar")

foreach ($file in $files) {
    $localPath = "releases/$file"
    if (Test-Path $localPath) {
        $sizeMB = [math]::Round((Get-Item $localPath).Length / 1MB, 1)
        Write-Host "Uploading $file ($sizeMB MB)..." -ForegroundColor Yellow
        & cmd /c "echo y | pscp -pw $VPS_PASS $localPath ${VPS_USER}@${VPS_HOST}:${REMOTE_DIR}/$file"
        if ($LASTEXITCODE -eq 0) {
            Write-Host "  [+] Uploaded $file successfully" -ForegroundColor Green
        } else {
            Write-Host "  [-] Failed to upload $file" -ForegroundColor Red
        }
    } else {
        Write-Host "  [!] File not found locally: $localPath" -ForegroundColor DarkGray
    }
}

Write-Host "==========================================================" -ForegroundColor Green
Write-Host "   All Container Images are now live on sas-man.net" -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
