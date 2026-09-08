# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Install Caspian-BYOC on Windows 10 (version 2004 or later) or Windows 11.
# Run from an elevated PowerShell in the
# repository with Go, Flutter and the .NET SDK installed:
#
#   powershell -ExecutionPolicy Bypass -File packaging\windows\install.ps1
#
# What it creates, and nothing else:
#   %ProgramFiles%\Caspian containing the Flutter app and background services
#   %ProgramData%\Caspian                    owned by the panel's service account
#   %ProgramData%\Caspian\first-run-password  (fresh install only)
#   service "caspian"        LocalSystem, automatic: the privileged half
#   service "caspian-panel"  NT SERVICE\caspian-panel, automatic: the web panel
#
# wintun.dll is shipped exactly as downloaded from wintun.net, under its own
# prebuilt-binaries licence, and must sit beside caspian.exe: that is where
# the engine loads it from and nowhere else.
#
# Idempotent: running it twice is an upgrade.
[CmdletBinding()]
param([switch]$NoOpen)

$ErrorActionPreference = "Stop"
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))
$panelUrl = "http://127.0.0.1:8088/"

function Refuse($msg) { Write-Error "caspian: $msg"; exit 1 }

function Invoke-SC([string[]]$ServiceArgs) {
    $output = & sc.exe @ServiceArgs 2>&1
    if ($LASTEXITCODE -ne 0) {
        Refuse "sc.exe failed with arguments [$($ServiceArgs -join ' | ')]: $($output -join ' ')"
    }
}

function Invoke-ICACLS([string[]]$AclArgs) {
    $output = & icacls.exe @AclArgs 2>&1
    if ($LASTEXITCODE -ne 0) {
        Refuse "icacls failed: $($output -join ' ')"
    }
}

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    $arguments = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "`"$PSCommandPath`"")
    $arguments += "-NoOpen"
    try {
        $process = Start-Process powershell.exe -Verb RunAs -Wait -PassThru -ArgumentList $arguments
    } catch {
        Refuse "administrator access was not granted: $($_.Exception.Message)"
    }
    if ($process.ExitCode -eq 0 -and -not $NoOpen) {
        Start-Process -FilePath (Join-Path $env:ProgramFiles "Caspian\caspian_ui.exe")
    }
    exit $process.ExitCode
}
if ($PSVersionTable.PSEdition -eq "Core" -and $PSVersionTable.PSVersion.Major -ge 6) {
    Write-Host "note: this script also runs under Windows PowerShell 5.1; either is fine for installing."
}

$nativeArchitecture = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$architecture = switch ($nativeArchitecture) {
    "AMD64" { "x64" }
    "ARM64" { "arm64" }
    default { Refuse "unsupported Windows architecture: $nativeArchitecture" }
}
& (Join-Path $PSScriptRoot "installer\build-installer.ps1") -Version dev -Architecture $architecture -PayloadOnly
$build = Join-Path $PSScriptRoot "installer\payload\$architecture"

$src = $build
foreach ($f in @("caspian.exe", "caspian-tethering.exe", "wintun.dll")) {
    if (-not (Test-Path (Join-Path $src $f))) { Refuse "$f is not in the current directory; build or download it first" }
}
$wintunLicense = Join-Path $repo "third_party\wintun\PREBUILT-BINARIES-LICENSE.txt"

$programs = Join-Path $env:ProgramFiles "Caspian"
$state = Join-Path $env:ProgramData "Caspian"
$panelAccount = "NT SERVICE\caspian-panel"

New-Item -ItemType Directory -Force -Path $programs | Out-Null
New-Item -ItemType Directory -Force -Path $state | Out-Null

# Stop before replacing binaries; Windows keeps an executable locked until its
# service process has completely exited. Stop-Service only sends the control,
# so explicitly wait for SCM to report Stopped before copying an upgrade.
foreach ($svc in @("caspian-panel", "caspian")) {
    $service = Get-Service -Name $svc -ErrorAction SilentlyContinue
    if ($service -and $service.Status -ne "Stopped") {
        Write-Host "stopping $svc..."
        Stop-Service -InputObject $service -Force -ErrorAction Stop
        $service.WaitForStatus("Stopped", [TimeSpan]::FromSeconds(30))
    }
}
foreach ($name in @("CaspianControl.exe", "caspian_ui.exe")) {
    $installedExecutable = Join-Path $programs $name
    Get-CimInstance Win32_Process -Filter "Name = '$name'" |
        Where-Object { $_.ExecutablePath -eq $installedExecutable } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force }
}
Copy-Item -Path (Join-Path $src "*") -Destination $programs -Recurse -Force
Remove-Item -LiteralPath (Join-Path $programs "CaspianControl.exe") -Force -ErrorAction SilentlyContinue
if (Test-Path -LiteralPath $wintunLicense) {
    Copy-Item -LiteralPath $wintunLicense -Destination (Join-Path $programs "WINTUN-LICENSE.txt") -Force
}

# The two services. A virtual service account is tied to an existing service
# name. Create the panel service first under the default LocalSystem account,
# then configure that existing service to use NT SERVICE\caspian-panel. That is
# also what makes the account resolvable when the state ACL is applied below.
$exe = Join-Path $programs "caspian.exe"
if (-not (Get-Service -Name "caspian-panel" -ErrorAction SilentlyContinue)) {
    New-Service -Name "caspian-panel" `
        -BinaryPathName "`"$exe`" serve --panel" `
        -DisplayName "Caspian-BYOC (panel)" `
        -StartupType Automatic | Out-Null
}
Invoke-SC @("config", "caspian-panel", "obj=", $panelAccount)
# Add the per-service SID to the panel process token. The privileged named
# pipe grants access to this SID, so leaving the default NONE value makes the
# panel service run but prevents it from reaching the privileged service.
Invoke-SC @("sidtype", "caspian-panel", "unrestricted")
Invoke-SC @("description", "caspian-panel", "Caspian-BYOC: the web panel, unprivileged")
Invoke-SC @("failure", "caspian-panel", "reset=", "300", "actions=", "restart/2000/restart/2000/restart/2000")
if (-not (Get-Service -Name "caspian" -ErrorAction SilentlyContinue)) {
    New-Service -Name "caspian" `
        -BinaryPathName "`"$exe`" serve --privileged" `
        -DisplayName "Caspian-BYOC (privileged)" `
        -StartupType Automatic | Out-Null
}
Invoke-SC @("description", "caspian", "Caspian-BYOC: routes, firewall, Mobile Hotspot and the tunnel engine")
Invoke-SC @("failure", "caspian", "reset=", "300", "actions=", "restart/2000/restart/2000/restart/2000")

# State belongs to the panel account: it holds the pasted config. The
# privileged service (LocalSystem) keeps its own journal beside it, and
# LocalSystem is always able to write there.
Invoke-ICACLS @($state, "/inheritance:r", "/grant:r", "*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F", "${panelAccount}:(OI)(CI)F")

$fresh = $false
$stateFile = Join-Path $state "state.json"
$seed = Join-Path $state "first-run-password"
if (-not (Test-Path $stateFile) -and -not (Test-Path $seed)) {
    $fresh = $true
    $bytes = New-Object byte[] 15
    [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $password = ([Convert]::ToBase64String($bytes)).ToLower() -replace '[^a-z0-9]', ''
    [IO.File]::WriteAllText($seed, $password)
    Invoke-ICACLS @($seed, "/inheritance:r", "/grant:r", "${panelAccount}:F", "*S-1-5-18:F")
}

Start-Service -Name "caspian"
Start-Service -Name "caspian-panel"

$controlSource = Join-Path $src "caspian_ui.exe"
$controlTarget = Join-Path $programs "caspian_ui.exe"
if (Test-Path -LiteralPath $controlSource) {
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut((Join-Path ([Environment]::GetFolderPath("CommonDesktopDirectory")) "Caspian.lnk"))
    $shortcut.TargetPath = $controlTarget
    $shortcut.WorkingDirectory = $programs
    $shortcut.Description = "Start, stop, restart, and open Caspian"
    $shortcut.Save()
}

Write-Host "installed. Open Caspian from the desktop shortcut."
if ($fresh) {
    Write-Host "first-run panel password: $password"
    Write-Host "It is consumed and deleted by the panel on its first start."
}
Write-Host "services: caspian, caspian-panel (sc query caspian)"

$deadline = [DateTime]::UtcNow.AddSeconds(45)
do {
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $panelUrl -TimeoutSec 3
        if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 500) { break }
    } catch {}
    Start-Sleep -Seconds 1
} while ([DateTime]::UtcNow -lt $deadline)
if ([DateTime]::UtcNow -ge $deadline) { Refuse "the panel did not answer within 45 seconds" }
if (-not $NoOpen -and (Test-Path -LiteralPath $controlTarget)) { Start-Process -FilePath $controlTarget }
