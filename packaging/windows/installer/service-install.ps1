param(
    [Parameter(Mandatory = $true)][string]$InstallDirectory,
    [string]$PasswordFile = "",
    [switch]$GeneratePassword
)
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
$password = $null
if (-not (Test-Path $stateFile)) {
    if ($GeneratePassword) {
        if (Test-Path -LiteralPath $seed) {
            # Retain the credential after an interrupted installation.
            $password = [IO.File]::ReadAllText($seed)
        } else {
            $bytes = New-Object byte[] 12
            $random = [Security.Cryptography.RandomNumberGenerator]::Create()
            try { $random.GetBytes($bytes) } finally { $random.Dispose() }
            $password = ([BitConverter]::ToString($bytes)).Replace('-', '').ToLowerInvariant()
        }
    } else {
        if (-not $PasswordFile -or -not (Test-Path -LiteralPath $PasswordFile)) {
            throw "A fresh installation needs a panel password from Setup."
        }
        try {
            $password = ([IO.File]::ReadAllText($PasswordFile, [Text.Encoding]::UTF8)) -replace "`r?`n$", ""
        } finally {
            Remove-Item -LiteralPath $PasswordFile -Force -ErrorAction SilentlyContinue
        }
    }
    if ($password.Length -lt 8) { throw "The panel password must contain at least 8 characters." }
    [IO.File]::WriteAllText($seed, $password)
    & icacls.exe $seed /inheritance:r /grant:r "${panelAccount}:F" "*S-1-5-18:F" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'The Caspian password permissions could not be set.' }
}

Start-Service caspian
Start-Service caspian-panel
# Only the elevated in-app helper requests this value. It displays the password
# in its administrator window and never passes it to the unelevated process.
if ($GeneratePassword -and $password) { Write-Output $password }
