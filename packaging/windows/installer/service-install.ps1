param([Parameter(Mandatory = $true)][string]$InstallDirectory)
$ErrorActionPreference = "Stop"

function Invoke-SC([string[]]$ServiceArgs) {
    $result = & sc.exe @ServiceArgs 2>&1
    if ($LASTEXITCODE -ne 0) { throw "sc.exe failed: $($ServiceArgs -join ' '): $($result -join ' ')" }
}

$executable = Join-Path $InstallDirectory "caspian.exe"
$state = Join-Path $env:ProgramData "Caspian"
$panelAccount = "NT SERVICE\caspian-panel"
New-Item -ItemType Directory -Force -Path $state | Out-Null

if (-not (Get-Service caspian-panel -ErrorAction SilentlyContinue)) {
    New-Service -Name caspian-panel -BinaryPathName "`"$executable`" serve --panel" -DisplayName "Caspian-BYOC (panel)" -StartupType Automatic | Out-Null
}
Invoke-SC @("config", "caspian-panel", "start=", "auto")
Invoke-SC @("config", "caspian-panel", "obj=", $panelAccount)
Invoke-SC @("sidtype", "caspian-panel", "unrestricted")
Invoke-SC @("failure", "caspian-panel", "reset=", "300", "actions=", "restart/2000/restart/2000/restart/2000")

if (-not (Get-Service caspian -ErrorAction SilentlyContinue)) {
    New-Service -Name caspian -BinaryPathName "`"$executable`" serve --privileged" -DisplayName "Caspian-BYOC (privileged)" -StartupType Automatic | Out-Null
}
Invoke-SC @("config", "caspian", "start=", "auto")
Invoke-SC @("failure", "caspian", "reset=", "300", "actions=", "restart/2000/restart/2000/restart/2000")

& icacls.exe $state /inheritance:r /grant:r "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F" "${panelAccount}:(OI)(CI)F" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "icacls.exe failed for $state" }

$stateFile = Join-Path $state "state.json"
$seed = Join-Path $state "first-run-password"
if (-not (Test-Path $stateFile) -and -not (Test-Path $seed)) {
    $bytes = New-Object byte[] 15
    [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $password = ([Convert]::ToBase64String($bytes)).ToLower() -replace '[^a-z0-9]', ''
    [IO.File]::WriteAllText($seed, $password)
    & icacls.exe $seed /inheritance:r /grant:r "${panelAccount}:F" "*S-1-5-18:F" | Out-Null
}

Start-Service caspian
Start-Service caspian-panel
