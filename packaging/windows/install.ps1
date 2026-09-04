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
[CmdletBinding()]
param([switch]$NoOpen)

$ErrorActionPreference = "Stop"
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))
$build = Join-Path $repo "out\windows"
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
    if ($NoOpen) { $arguments += "-NoOpen" }
    try {
        $process = Start-Process powershell.exe -Verb RunAs -Wait -PassThru -ArgumentList $arguments
    } catch {
        Refuse "administrator access was not granted: $($_.Exception.Message)"
    }
    exit $process.ExitCode
}
if ($PSVersionTable.PSEdition -eq "Core" -and $PSVersionTable.PSVersion.Major -ge 6) {
    Write-Host "note: this script also runs under Windows PowerShell 5.1; either is fine for installing."
}

New-Item -ItemType Directory -Force -Path $build | Out-Null
if (-not (Get-Command go.exe -ErrorAction SilentlyContinue)) { Refuse "Go is not installed" }
if (-not (Get-Command dotnet.exe -ErrorAction SilentlyContinue)) { Refuse "the .NET SDK is not installed" }
$nativeArchitecture = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$runtime = switch ($nativeArchitecture) {
    "AMD64" { "win-x64" }
    "ARM64" { "win-arm64" }
    default { Refuse "unsupported Windows architecture: $nativeArchitecture" }
}

Write-Host "building Caspian..."
Push-Location $repo
& go.exe build -trimpath -o (Join-Path $build "caspian.exe") .\cmd\caspian
if ($LASTEXITCODE -ne 0) { Refuse "the Go build failed" }
& dotnet.exe publish (Join-Path $repo "tools\caspian-tethering\caspian-tethering.csproj") -c Release -r $runtime --self-contained true -o $build
if ($LASTEXITCODE -ne 0) { Refuse "the Mobile Hotspot helper build failed" }
if ($runtime -eq "win-x64") {
    & dotnet.exe publish (Join-Path $repo "tools\caspian-control\caspian-control.csproj") -c Release -r $runtime --self-contained true -o $build
    if ($LASTEXITCODE -ne 0) { Refuse "the tray launcher build failed" }
}
Pop-Location

$wintun = Join-Path $repo "wintun.dll"
if (-not (Test-Path -LiteralPath $wintun)) {
    $version = "0.14.1"
    $expectedHash = "07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51"
    $architecture = if ($runtime -eq "win-arm64") { "arm64" } else { "amd64" }
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ("caspian-wintun-" + [Guid]::NewGuid().ToString("N"))
    $archive = Join-Path $temporary "wintun.zip"
    New-Item -ItemType Directory -Path $temporary | Out-Null
    try {
        Write-Host "downloading Wintun $version..."
        Invoke-WebRequest -UseBasicParsing -Uri "https://www.wintun.net/builds/wintun-$version.zip" -OutFile $archive
        $actualHash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actualHash -ne $expectedHash) { Refuse "the Wintun archive checksum does not match" }
        Expand-Archive -LiteralPath $archive -DestinationPath (Join-Path $temporary "expanded")
        Copy-Item -LiteralPath (Join-Path $temporary "expanded\wintun\bin\$architecture\wintun.dll") -Destination $wintun
    } finally {
        if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Recurse -Force }
    }
}
Copy-Item -LiteralPath $wintun -Destination (Join-Path $build "wintun.dll") -Force

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
foreach ($f in @("caspian.exe", "caspian-tethering.exe", "wintun.dll")) {
    $source = Join-Path $src $f
    $destination = Join-Path $programs $f
    $copied = $false
    for ($attempt = 1; $attempt -le 10; $attempt++) {
        try {
            Copy-Item -Force -LiteralPath $source -Destination $destination
            $copied = $true
            break
        } catch [IO.IOException] {
            if ($attempt -eq 10) { throw }
            Start-Sleep -Milliseconds 500
        }
    }
    if (-not $copied) { Refuse "could not install $f" }
}
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

$controlSource = Join-Path $src "CaspianControl.exe"
$controlTarget = Join-Path $programs "CaspianControl.exe"
if (Test-Path -LiteralPath $controlSource) {
    Get-CimInstance Win32_Process -Filter "Name = 'CaspianControl.exe'" |
        Where-Object { $_.ExecutablePath -eq $controlTarget } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force }
    Copy-Item -LiteralPath $controlSource -Destination $controlTarget -Force
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut((Join-Path ([Environment]::GetFolderPath("CommonDesktopDirectory")) "Caspian Control.lnk"))
    $shortcut.TargetPath = $controlTarget
    $shortcut.WorkingDirectory = $programs
    $shortcut.Description = "Start, stop, restart, and open Caspian"
    $shortcut.Save()
}

Write-Host "installed. Panel: http://127.0.0.1:8088 (and the hotspot address once it is up)"
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
