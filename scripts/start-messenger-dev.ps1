# start-messenger-dev.ps1 — native Windows ONLY (no Docker)
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Vpn = Join-Path $Root "src\src-vpn"
$Pwa = Join-Path $Root "prototypes\pwa-vpn"
$Go = if (Test-Path "C:\Program Files\Go\bin\go.exe") { "C:\Program Files\Go\bin\go.exe" } else { "go" }
$Npm = if (Test-Path "C:\Program Files\nodejs\npm.cmd") { "C:\Program Files\nodejs\npm.cmd" } else { "npm.cmd" }

function Test-Port([int]$Port) {
  try {
    $c = New-Object System.Net.Sockets.TcpClient
    $c.Connect("127.0.0.1", $Port)
    $c.Close()
    return $true
  } catch { return $false }
}

Write-Host "=== Proxi Messenger DEV (native) ===" -ForegroundColor Cyan
Write-Host "Code: $Root"

if (-not (Test-Port 8080)) {
  Write-Host "Building Go server..." -ForegroundColor Yellow
  Push-Location $Vpn
  try {
    & $Go build -o messenger-server-dev.exe ./cmd/webserver/
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }
    $data = Join-Path $Vpn "data-dev"
    New-Item -ItemType Directory -Force -Path $data | Out-Null
    $env:PORT = "8080"
    $env:DATA_DIR = $data
    Start-Process -FilePath (Join-Path $Vpn "messenger-server-dev.exe") -WorkingDirectory $Vpn -WindowStyle Minimized
  } finally { Pop-Location }
  Start-Sleep -Seconds 1
} else { Write-Host "API :8080 already up" -ForegroundColor Green }

if (-not (Test-Port 5173)) {
  Write-Host "Starting Vite :5173..." -ForegroundColor Yellow
  Push-Location $Pwa
  try {
    if (-not (Test-Path "node_modules")) { & $Npm ci --no-audit --no-fund }
    Start-Process -FilePath $Npm -ArgumentList @("run","dev","--","--host","127.0.0.1","--port","5173") -WorkingDirectory $Pwa -WindowStyle Minimized
  } finally { Pop-Location }
  Start-Sleep -Seconds 2
} else { Write-Host "UI :5173 already up" -ForegroundColor Green }

Write-Host "UI:  http://127.0.0.1:5173/" -ForegroundColor Green
Write-Host "API: http://127.0.0.1:8080/api/health" -ForegroundColor Green
Write-Host "2 users: Chrome + Incognito, copy User ID in New Chat"
