# Smoke guard: fail if public Nostr relays appear in serveable/active paths.
# Usage (from .04-Src or any cwd): powershell -File guard-no-public-relays.ps1 [-Root <path>]
param(
  [string]$Root = ""
)
$ErrorActionPreference = "Stop"
if (-not $Root) {
  # default: walk up to .04-Src or use known path
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
$excludeDirs = @('node_modules','archive','.git','android','coverage')
$hits = @()
foreach ($sr in $scanRoots) {
  if (-not (Test-Path $sr)) { continue }
  Get-ChildItem -Path $sr -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object {
      $p = $_.FullName
      $skip = $false
      foreach ($ex in $excludeDirs) {
        if ($p -match [regex]::Escape([IO.Path]::DirectorySeparatorChar + $ex + [IO.Path]::DirectorySeparatorChar) -or
            $p -match [regex]::Escape([IO.Path]::DirectorySeparatorChar + $ex + '$')) { $skip = $true }
      }
      # quarantine / Settings-v1 archive ok only if path contains archive
      if ($p -match '\\archive\\' -or $p -match '\\node_modules\\') { $skip = $true }
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
  $hits | Select-Object -First 30 | ForEach-Object { Write-Host ("{0}:{1}:{2}" -f $_.Path, $_.LineNumber, $_.Line.Trim().Substring(0, [Math]::Min(120, $_.Line.Trim().Length))) }
  exit 1
}
Write-Host "PASS: no public relays in serveable/active paths"
exit 0
