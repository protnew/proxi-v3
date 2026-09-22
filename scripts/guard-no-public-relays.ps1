# Smoke guard: fail if public Nostr relays appear in serveable/active paths.
# Usage: powershell -File scripts/guard-no-public-relays.ps1 [-Root <path>]
param([string]$Root = "")
$ErrorActionPreference = "Stop"
if (-not $Root) {
  $cand = "C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src"
  if (Test-Path $cand) { $Root = $cand } else { $Root = (Get-Location).Path }
}
$pattern = 'nos\.lol|nostr\.band|damus\.io'
$scanRoots = @(
  (Join-Path $Root "src\src-vpn\cmd\webserver\dist"),
  (Join-Path $Root "cmd\webserver\dist"),
  (Join-Path $Root "prototypes\pwa-vpn\dist"),
  (Join-Path $Root "prototypes\pwa-vpn\src")
)
$excludeDirs = @('node_modules','archive','.git','android','coverage','_quarantine_p9')
$hits = @()
foreach ($sr in $scanRoots) {
  if (-not (Test-Path $sr)) { continue }
  Get-ChildItem -Path $sr -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object {
      $p = $_.FullName
      $skip = $false
      foreach ($ex in $excludeDirs) {
        if ($p -like "*\$ex\*" -or $p -like "*/$ex/*") { $skip = $true }
      }
      if ($p -match 'dist\.bak-p9') { $skip = $true }
      -not $skip
    } |
    ForEach-Object {
      $m = Select-String -Path $_.FullName -Pattern $pattern -AllMatches -ErrorAction SilentlyContinue
      if ($m) { $hits += $m }
    }
}
Write-Host "ROOT=$Root"
Write-Host "HITS=$($hits.Count)"
if ($hits.Count -gt 0) {
  $hits | Select-Object -First 30 | ForEach-Object {
    $line = $_.Line.Trim()
    if ($line.Length -gt 120) { $line = $line.Substring(0,120) }
    Write-Host ("{0}:{1}:{2}" -f $_.Path, $_.LineNumber, $line)
  }
  exit 1
}
Write-Host "PASS: no public relays in serveable/active paths"
exit 0
