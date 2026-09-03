# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Install Caspian-BYOC on Windows 11. Run from an elevated PowerShell in the
# directory that holds caspian.exe, caspian-tethering.exe and wintun.dll:
#
#   powershell -ExecutionPolicy Bypass -File packaging\windows\install.ps1
#
# What it creates, and nothing else:
#   %ProgramFiles%\Caspian\{caspian.exe, caspian-tethering.exe, wintun.dll}
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
$ErrorActionPreference = "Stop"

function Refuse($msg) { Write-Error "caspian: $msg"; exit 1 }

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { Refuse "run from an elevated PowerShell" }
if ($PSVersionTable.PSEdition -eq "Core" -and $PSVersionTable.PSVersion.Major -ge 6) {
    Write-Host "note: this script also runs under Windows PowerShell 5.1; either is fine for installing."
}

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$src = Get-Location
foreach ($f in @("caspian.exe", "caspian-tethering.exe", "wintun.dll")) {
    if (-not (Test-Path (Join-Path $src $f))) { Refuse "$f is not in the current directory; build or download it first" }
}

$programs = Join-Path $env:ProgramFiles "Caspian"
$state = Join-Path $env:ProgramData "Caspian"
$panelAccount = "NT SERVICE\caspian-panel"

New-Item -ItemType Directory -Force -Path $programs | Out-Null
New-Item -ItemType Directory -Force -Path $state | Out-Null

# Stop before replacing binaries; a running exe cannot be overwritten.
foreach ($svc in @("caspian-panel", "caspian")) {
    if (Get-Service -Name $svc -ErrorAction SilentlyContinue) { Stop-Service -Name $svc -Force -ErrorAction SilentlyContinue }
}
foreach ($f in @("caspian.exe", "caspian-tethering.exe", "wintun.dll")) {
    Copy-Item -Force (Join-Path $src $f) (Join-Path $programs $f)
}

# The two services. Creating the panel service first is what makes its virtual
# account exist, so the state directory can be handed to it below.
$exe = Join-Path $programs "caspian.exe"
if (-not (Get-Service -Name "caspian" -ErrorAction SilentlyContinue)) {
    & sc.exe create caspian binPath= "`"$exe`" serve --privileged" start= auto DisplayName= "Caspian-BYOC (privileged)" | Out-Null
}
& sc.exe description caspian "Caspian-BYOC: routes, firewall, Mobile Hotspot and the tunnel engine" | Out-Null
& sc.exe failure caspian reset= 300 actions= restart/2000/restart/2000/restart/2000 | Out-Null
if (-not (Get-Service -Name "caspian-panel" -ErrorAction SilentlyContinue)) {
    & sc.exe create caspian-panel binPath= "`"$exe`" serve --panel" start= auto obj= $panelAccount DisplayName= "Caspian-BYOC (panel)" | Out-Null
}
& sc.exe description caspian-panel "Caspian-BYOC: the web panel, unprivileged" | Out-Null
& sc.exe failure caspian-panel reset= 300 actions= restart/2000/restart/2000/restart/2000 | Out-Null

# State belongs to the panel account: it holds the pasted config. The
# privileged service (LocalSystem) keeps its own journal beside it, and
# LocalSystem is always able to write there.
& icacls $state /inheritance:r /grant:r "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F" "${panelAccount}:(OI)(CI)F" | Out-Null

$fresh = $false
$stateFile = Join-Path $state "state.json"
$seed = Join-Path $state "first-run-password"
if (-not (Test-Path $stateFile) -and -not (Test-Path $seed)) {
    $fresh = $true
    $bytes = New-Object byte[] 15
    [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $password = ([Convert]::ToBase64String($bytes)).ToLower() -replace '[^a-z0-9]', ''
    [IO.File]::WriteAllText($seed, $password)
    & icacls $seed /inheritance:r /grant:r "${panelAccount}:F" "*S-1-5-18:F" | Out-Null
}

Start-Service -Name "caspian"
Start-Service -Name "caspian-panel"

Write-Host "installed. Panel: http://127.0.0.1:8088 (and the hotspot address once it is up)"
if ($fresh) {
    Write-Host "first-run panel password: $password"
    Write-Host "It is consumed and deleted by the panel on its first start."
}
Write-Host "services: caspian, caspian-panel (sc query caspian)"
