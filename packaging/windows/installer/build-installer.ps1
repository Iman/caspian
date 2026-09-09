[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [ValidateSet("x64", "arm64")][string]$Architecture = "x64"
)
$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") { throw "Run this build on Windows." }
if ($Version -notmatch '^v?\d+\.\d+\.\d+(?:\.\d+)?$') {
    throw "Use a numeric version such as 1.2.3 or v1.2.3."
}
foreach ($tool in @("go.exe", "dotnet.exe")) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        throw "Missing $tool. See docs/WINDOWS-BUILD.md for the build prerequisites."
    }
}
$compiler = @(
    (Get-Command ISCC.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source),
    "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
    "$env:ProgramFiles\Inno Setup 6\ISCC.exe"
) | Where-Object { $_ -and (Test-Path -LiteralPath $_) } | Select-Object -First 1
if (-not $compiler) { throw "Install Inno Setup 6, then run this command again." }
$sdkVersions = & dotnet.exe --list-sdks
if ($LASTEXITCODE -ne 0 -or -not ($sdkVersions | Where-Object { $_ -match '^(9|[1-9]\d+)\.' })) {
    throw "Install .NET SDK 9 or later, then run this command again."
}
$numericVersion = $Version.TrimStart("v")
$releaseVersion = "v$numericVersion"
$runtime = "win-$Architecture"
$goArchitecture = if ($Architecture -eq "arm64") { "arm64" } else { "amd64" }
$wintunArchitecture = if ($Architecture -eq "arm64") { "arm64" } else { "amd64" }
$installerArchitecture = if ($Architecture -eq "arm64") { "arm64" } else { "x64os" }
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\..\.."))
$payload = Join-Path $PSScriptRoot "payload\$Architecture"
$output = Join-Path $repo "out\installer"
New-Item -ItemType Directory -Force -Path $payload, $output | Out-Null

Push-Location $repo
$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
$previousCGO = $env:CGO_ENABLED
$previousGOCACHE = $env:GOCACHE
try {
    $env:GOOS = "windows"
    $env:GOARCH = $goArchitecture
    # Release builds use Go's Windows implementation without a C compiler.
    $env:CGO_ENABLED = "0"
    if (-not $env:GOCACHE) { $env:GOCACHE = Join-Path $repo ".gocache" }
    Write-Host "Building Caspian $releaseVersion for $Architecture..."
    & go.exe build -trimpath -ldflags "-X main.version=$releaseVersion -X caspianbyoc.org/caspian/internal/panel.Version=$releaseVersion" -o (Join-Path $payload "caspian.exe") .\cmd\caspian
    if ($LASTEXITCODE -ne 0) { throw "The Go build failed." }
    & dotnet.exe publish .\tools\caspian-tethering\caspian-tethering.csproj -c Release -r $runtime --self-contained true -o $payload
    if ($LASTEXITCODE -ne 0) { throw "The hotspot helper build failed." }
    & dotnet.exe publish .\tools\caspian-control\caspian-control.csproj -c Release -r $runtime --self-contained true -p:Version=$numericVersion -p:InformationalVersion=$releaseVersion -p:IncludeSourceRevisionInInformationalVersion=false -o $payload
    if ($LASTEXITCODE -ne 0) { throw "The tray app build failed." }
    $controlVersion = [Diagnostics.FileVersionInfo]::GetVersionInfo((Join-Path $payload "CaspianControl.exe")).ProductVersion
    if ($controlVersion -ne $releaseVersion) { throw "The control window version '$controlVersion' does not match build version '$releaseVersion'." }
} finally {
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    $env:CGO_ENABLED = $previousCGO
    $env:GOCACHE = $previousGOCACHE
    Pop-Location
}

$wintunCache = Join-Path $repo ".cache\wintun-0.14.1"
$archive = Join-Path $wintunCache "wintun.zip"
New-Item -ItemType Directory -Force -Path $wintunCache | Out-Null
if (-not (Test-Path -LiteralPath $archive)) {
    Write-Host "Downloading Wintun 0.14.1..."
    Invoke-WebRequest -UseBasicParsing -Uri "https://www.wintun.net/builds/wintun-0.14.1.zip" -OutFile $archive
}
$actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actual -ne "07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51") {
    throw "The Wintun checksum does not match. Delete '$archive', then run the build again."
}
Expand-Archive -LiteralPath $archive -DestinationPath (Join-Path $wintunCache "expanded") -Force
Copy-Item -LiteralPath (Join-Path $wintunCache "expanded\wintun\bin\$wintunArchitecture\wintun.dll") -Destination (Join-Path $payload "wintun.dll") -Force

# SNI spoofing is optional and supported by the official x64 driver only.
if ($Architecture -eq "x64") {
    $divertCache = Join-Path $repo ".cache\windivert-2.2.2"
    $divertArchive = Join-Path $divertCache "WinDivert.zip"
    New-Item -ItemType Directory -Force -Path $divertCache | Out-Null
    if (-not (Test-Path -LiteralPath $divertArchive)) {
        Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/basil00/WinDivert/releases/download/v2.2.2/WinDivert-2.2.2-A.zip" -OutFile $divertArchive
    }
    if ((Get-FileHash -LiteralPath $divertArchive -Algorithm SHA256).Hash.ToLowerInvariant() -ne "63cb41763bb4b20f600b6de04e991a9c2be73279e317d4d82f237b150c5f3f15") {
        throw "The WinDivert checksum does not match. Delete '$divertArchive', then run the build again."
    }
    $divertSource = Join-Path $divertCache "WinDivert-source.zip"
    if (-not (Test-Path -LiteralPath $divertSource)) {
        Invoke-WebRequest -UseBasicParsing -Uri "https://codeload.github.com/basil00/WinDivert/zip/refs/tags/v2.2.2" -OutFile $divertSource
    }
    if ((Get-FileHash -LiteralPath $divertSource -Algorithm SHA256).Hash.ToLowerInvariant() -ne "65ec79c9e6afa99f648a3f4d1f6db794640b40d0b65bd438770ea503ee14ecb7") {
        throw "The WinDivert source checksum does not match. Delete '$divertSource', then run the build again."
    }
    Copy-Item -LiteralPath $divertSource -Destination (Join-Path $payload "WinDivert-source.zip") -Force
    Expand-Archive -LiteralPath $divertArchive -DestinationPath (Join-Path $divertCache "expanded") -Force
    foreach ($divertFile in @("WinDivert.dll", "WinDivert64.sys")) {
        Copy-Item -LiteralPath (Join-Path $divertCache "expanded\WinDivert-2.2.2-A\x64\$divertFile") -Destination (Join-Path $payload $divertFile) -Force
    }
}

function Assert-PEArchitecture([string]$Path, [uint16]$ExpectedMachine) {
    $bytes = [IO.File]::ReadAllBytes($Path)
    if ($bytes.Length -lt 64 -or $bytes[0] -ne 0x4d -or $bytes[1] -ne 0x5a) {
        throw "$Path is not a Windows PE file."
    }
    $peOffset = [BitConverter]::ToInt32($bytes, 0x3c)
    if ($peOffset -lt 0 -or $peOffset + 6 -gt $bytes.Length -or
        $bytes[$peOffset] -ne 0x50 -or $bytes[$peOffset + 1] -ne 0x45) {
        throw "$Path has an invalid Windows PE header."
    }
    $machine = [BitConverter]::ToUInt16($bytes, $peOffset + 4)
    if ($machine -ne $ExpectedMachine) {
        throw ("{0} has PE machine 0x{1:x4}; expected 0x{2:x4} for {3}." -f
            $Path, $machine, $ExpectedMachine, $Architecture)
    }
}

$expectedMachine = if ($Architecture -eq "arm64") { [uint16]0xaa64 } else { [uint16]0x8664 }
foreach ($binary in @("caspian.exe", "caspian-tethering.exe", "CaspianControl.exe", "wintun.dll")) {
    Assert-PEArchitecture (Join-Path $payload $binary) $expectedMachine
}
if ($Architecture -eq "x64") {
    foreach ($binary in @("WinDivert.dll", "WinDivert64.sys")) {
        Assert-PEArchitecture (Join-Path $payload $binary) $expectedMachine
    }
}
Write-Host "Building the installer..."
& $compiler "/DAppVersion=$numericVersion" "/DBuildArchitecture=$Architecture" "/DAllowedArchitecture=$installerArchitecture" (Join-Path $PSScriptRoot "Caspian.iss")
if ($LASTEXITCODE -ne 0) { throw "Inno Setup failed." }
$installer = Join-Path $output "CaspianSetup-$numericVersion-windows-$Architecture.exe"
$checksumFile = "$installer.sha256"
$checksum = (Get-FileHash -LiteralPath $installer -Algorithm SHA256).Hash.ToLowerInvariant()
"$checksum  $([IO.Path]::GetFileName($installer))" | Set-Content -LiteralPath $checksumFile -Encoding ASCII
Write-Host "Build complete."
Write-Host "App and helpers: $payload"
Write-Host "Installer:       $installer"
Write-Host "SHA256:          $checksumFile"
