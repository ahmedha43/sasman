# =============================================================
#  SASMAN Central Server - Deploy Script
#  Builds central server image locally and updates VPS
# =============================================================

$ErrorActionPreference = "Stop"

$VPS_HOST   = "51.241.184.4"
$VPS_PORT   = 2026
$VPS_USER   = "maram"
$VPS_PASS   = '[2adu!k;Opf.lMr]IdVG`ASgk'
$IMAGE_NAME = "server-central"
$IMAGE_TAG  = "new"
$TAR_FILE   = "server-central.tar"
$CONTAINER  = "sasman-central"
$REMOTE_TAR = "/tmp/server-central-new.tar"

function Write-Step($msg) {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "  $msg" -ForegroundColor Yellow
    Write-Host "==========================================" -ForegroundColor Cyan
}
function Write-OK($msg)   { Write-Host "  OK  $msg" -ForegroundColor Green }
function Write-Fail($msg) { Write-Host "  ERR $msg" -ForegroundColor Red; exit 1 }
function Write-Info($msg) { Write-Host "  ... $msg" -ForegroundColor Gray }

function Run-Remote([string]$cmd) {
    $oldEAP = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = & plink -ssh "${VPS_USER}@${VPS_HOST}" -P $VPS_PORT -pw $VPS_PASS -hostkey "SHA256:o6mNNX9NEPuHiYt0X34IEhg3LBb1R44TT64dkF8ZsfY" -batch "echo '$VPS_PASS' | sudo -S bash -c '$cmd'" 2>&1 | Out-String
    $ErrorActionPreference = $oldEAP
    return $out
}

# 1. Test Connection
Write-Step "Checking prerequisites & connection..."
$pingResult = Run-Remote "echo CONNECTION_OK"
if ($pingResult -notmatch "CONNECTION_OK") {
    Write-Fail "Cannot connect to VPS ${VPS_HOST}!"
}
Write-OK "Connection to VPS is OK"

$REPO_ROOT = if (Test-Path "$PSScriptRoot\..\server") { (Resolve-Path "$PSScriptRoot\..").Path } else { (Get-Location).Path }
$TAR_FILE   = Join-Path $REPO_ROOT "server-central.tar"

# 2. Build Image Locally
Write-Step "STEP 1/4 | Building Docker image..."
$buildStart = Get-Date
& docker build -f "$REPO_ROOT/server/Dockerfile" -t "${IMAGE_NAME}:${IMAGE_TAG}" "$REPO_ROOT"
if ($LASTEXITCODE -ne 0) { Write-Fail "Docker build failed!" }
$secs = [math]::Round(((Get-Date) - $buildStart).TotalSeconds)
Write-OK "Build finished in $secs seconds"

# 3. Save Image as TAR
Write-Step "STEP 2/4 | Saving Docker image to TAR..."
if (Test-Path $TAR_FILE) { Remove-Item $TAR_FILE -Force }
& docker save "${IMAGE_NAME}:${IMAGE_TAG}" -o $TAR_FILE
if ($LASTEXITCODE -ne 0) { Write-Fail "Docker save failed!" }
$sizeMB = [math]::Round((Get-Item $TAR_FILE).Length / 1MB, 1)
Write-OK "Saved: $TAR_FILE ($sizeMB MB)"

# 4. Upload to VPS
Write-Step "STEP 3/4 | Uploading to VPS..."
Run-Remote "rm -f $REMOTE_TAR" | Out-Null
& cmd /c "echo y | pscp -pw $VPS_PASS $TAR_FILE ${VPS_USER}@${VPS_HOST}:${REMOTE_TAR}"
if ($LASTEXITCODE -ne 0) { Write-Fail "Upload to VPS failed!" }
Write-OK "Upload completed successfully"

# 5. Load and Restart Container on VPS
Write-Step "STEP 4/4 | Updating container on VPS and configuring Nginx upload limits..."
$remoteCmd = "sed -i '/client_max_body_size/d' /etc/nginx/nginx.conf && sed -i '/http {/a \    client_max_body_size 256M;' /etc/nginx/nginx.conf && (nginx -t && systemctl reload nginx || true) && docker load -i $REMOTE_TAR && docker stop $CONTAINER || true && docker rm $CONTAINER || true && docker run -d --name $CONTAINER --restart unless-stopped --network host -e ADDR=:8080 -e SASMAN_CENTRAL_DOMAIN=sas-man.net -e SASMAN_DB_PATH=/app/data/sasman-central.db -e SASMAN_ADMIN_USER=admin -e SASMAN_ADMIN_PASSWORD='Mushtaq@Sasman#9977!' -v /var/run/docker.sock:/var/run/docker.sock -v /root/sasman-central/server/data:/app/data ${IMAGE_NAME}:${IMAGE_TAG} && sleep 4 && docker ps --filter name=$CONTAINER && echo '--- LOGS ---' && docker logs $CONTAINER --tail 25 && rm -f $REMOTE_TAR && echo 'DEPLOY_SUCCESS'"

$output = Run-Remote $remoteCmd
Write-Host $output

if ($output -notmatch "DEPLOY_SUCCESS") {
    Write-Fail "Deployment failed on VPS!"
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "  Server deployed successfully on VPS!"   -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
Write-Host "  Domain: https://sas-man.net/login"      -ForegroundColor Cyan
Write-Host "  Health: https://sas-man.net/health"     -ForegroundColor Cyan
Write-Host ""
