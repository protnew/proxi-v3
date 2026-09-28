#Requires -Version 5.1
foreach ($k in @('HTTP_PROXY','HTTPS_PROXY','http_proxy','https_proxy','ALL_PROXY','all_proxy')) {
  [Environment]::SetEnvironmentVariable($k, $null, 'Process')
  Remove-Item "Env:$k" -ErrorAction SilentlyContinue
}
# Do not write User-level proxy. Ensure User empty:
if ([Environment]::GetEnvironmentVariable('HTTP_PROXY','User')) {
  [Environment]::SetEnvironmentVariable('HTTP_PROXY', $null, 'User')
}
if ([Environment]::GetEnvironmentVariable('HTTPS_PROXY','User')) {
  [Environment]::SetEnvironmentVariable('HTTPS_PROXY', $null, 'User')
}
Write-Host "PROCESS HTTP_PROXY=[$env:HTTP_PROXY] USER=[$([Environment]::GetEnvironmentVariable('HTTP_PROXY','User'))]"
