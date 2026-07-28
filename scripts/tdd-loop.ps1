# tdd-loop.ps1 — native Windows (NO Docker)
# Runs: go unit subset + vitest + optional playwright
param(
  [switch]$SkipE2E,
  [string]$GoPackages = "./auth/;./store/;./chat/"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Vpn = Join-Path $Root "src\src-vpn"
$Pwa = Join-Path $Root "prototypes\pwa-vpn"
$Go = if (Test-Path "C:\Program Files\Go\bin\go.exe") { "C:\Program Files\Go\bin\go.exe" } else { "go" }

$env:GOMAXPROCS = "1"
$env:GOTMPDIR = "C:\Hermes\hermes-data-native\gotmp"
New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null

Write-Host "=== 1) go test (native) ===" -ForegroundColor Cyan
Push-Location $Vpn
try {
  foreach ($pkg in $GoPackages.Split(";")) {
    if (-not $pkg) { continue }
    Write-Host "go test $pkg"
    & $Go test $pkg -count=1 -timeout 90s
    if ($LASTEXITCODE -ne 0) { throw "go test failed: $pkg" }
  }
} finally { Pop-Location }

Write-Host "=== 2) vitest ===" -ForegroundColor Cyan
Push-Location $Pwa
try {
  if (-not (Test-Path "node_modules")) { npm ci --no-audit --no-fund }
  npx vitest run
  if ($LASTEXITCODE -ne 0) { throw "vitest failed" }
} finally { Pop-Location }

if (-not $SkipE2E) {
  Write-Host "=== 3) playwright (expects :5173 + :8080) ===" -ForegroundColor Cyan
  # health check
  try {
    $h = Invoke-WebRequest -Uri "http://127.0.0.1:8080/api/health" -UseBasicParsing -TimeoutSec 3
    if ($h.StatusCode -ne 200) { throw "API not healthy" }
  } catch {
    Write-Host "SKIP playwright: Go API :8080 not up (start messenger-server-dev.exe)" -ForegroundColor Yellow
    exit 0
  }
  Push-Location $Pwa
  try {
    npx playwright test --config=playwright.config.ts
    if ($LASTEXITCODE -ne 0) { throw "playwright failed" }
  } finally { Pop-Location }
}

Write-Host "TDD LOOP OK" -ForegroundColor Green
