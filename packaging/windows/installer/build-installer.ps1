[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [ValidateSet("x64", "arm64")][string]$Architecture = "x64",
    [switch]$PayloadOnly
)
$ErrorActionPreference = "Stop"
$env:NO_COLOR = "1"
if ($Version -ne 'dev' -and $Version -notmatch '^v?[0-9]+\.[0-9]+\.[0-9]+$') { throw "Use a release version such as v1.2.3." }
$numericVersion = if ($Version -eq "dev") { "0.0.0" } else { $Version.TrimStart("v") }
$releaseVersion = if ($Version -eq "dev") { "dev" } else { "v$numericVersion" }
$runtime = "win-$Architecture"
$goArchitecture = if ($Architecture -eq "arm64") { "arm64" } else { "amd64" }
$wintunArchitecture = if ($Architecture -eq "arm64") { "arm64" } else { "amd64" }
$installerArchitecture = if ($Architecture -eq "arm64") { "arm64" } else { "x64os" }
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\..\.."))
$payload = Join-Path $PSScriptRoot "payload\$Architecture"
$output = Join-Path $repo "out\installer"
if (Test-Path -LiteralPath $payload) { Remove-Item -LiteralPath $payload -Recurse -Force }
New-Item -ItemType Directory -Force -Path $payload, $output | Out-Null

Push-Location $repo
$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
try {
    Push-Location (Join-Path $repo "ui")
    try {
        & flutter --suppress-analytics pub get
        if ($LASTEXITCODE -ne 0) { throw "Flutter dependency resolution failed." }
        & flutter --suppress-analytics build web --release --no-web-resources-cdn --pwa-strategy=none --build-name $numericVersion "--dart-define=CASPIAN_VERSION=$releaseVersion"
        if ($LASTEXITCODE -ne 0) { throw "The Flutter web build failed." }
        $webAssets = Join-Path $repo "internal\panel\flutter"
        New-Item -ItemType Directory -Force -Path $webAssets | Out-Null
        Get-ChildItem -LiteralPath $webAssets | Where-Object { $_.Name -ne ".keep" } | Remove-Item -Recurse -Force
        Copy-Item -Path "build\web\*" -Destination $webAssets -Recurse -Force
        & flutter --suppress-analytics build windows --release --build-name $numericVersion "--dart-define=CASPIAN_VERSION=$releaseVersion"
        if ($LASTEXITCODE -ne 0) { throw "The Flutter Windows build failed." }
        $flutterBundle = "build\windows\$Architecture\runner\Release"
        if (-not (Test-Path "$flutterBundle\caspian_ui.exe")) { throw "Build on a Windows $Architecture host with a matching Flutter SDK." }
        Copy-Item -Path "$flutterBundle\*" -Destination $payload -Recurse -Force
    } finally { Pop-Location }
    $env:GOOS = "windows"
    $env:GOARCH = $goArchitecture
    & go.exe build -tags flutterui -trimpath -buildvcs=false -ldflags "-X main.version=$releaseVersion -X caspianbyoc.org/caspian/internal/panel.Version=$releaseVersion" -o (Join-Path $payload "caspian.exe") .\cmd\caspian
    if ($LASTEXITCODE -ne 0) { throw "The Go build failed." }
    & dotnet.exe publish .\tools\caspian-tethering\caspian-tethering.csproj -c Release -r $runtime --self-contained true -o $payload
    if ($LASTEXITCODE -ne 0) { throw "The hotspot helper build failed." }

} finally {
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    Pop-Location
}

$temporary = Join-Path ([IO.Path]::GetTempPath()) ("caspian-wintun-" + [Guid]::NewGuid().ToString("N"))
$archive = Join-Path $temporary "wintun.zip"
New-Item -ItemType Directory -Path $temporary | Out-Null
try {
    Invoke-WebRequest -UseBasicParsing -Uri "https://www.wintun.net/builds/wintun-0.14.1.zip" -OutFile $archive
    $actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne "07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51") { throw "The Wintun checksum does not match." }
    Expand-Archive -LiteralPath $archive -DestinationPath (Join-Path $temporary "expanded")
    Copy-Item -LiteralPath (Join-Path $temporary "expanded\wintun\bin\$wintunArchitecture\wintun.dll") -Destination (Join-Path $payload "wintun.dll") -Force
} finally { Remove-Item -LiteralPath $temporary -Recurse -Force -ErrorAction SilentlyContinue }

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
foreach ($binary in @("caspian.exe", "caspian-tethering.exe", "caspian_ui.exe", "flutter_windows.dll", "msvcp140.dll", "vcruntime140.dll", "vcruntime140_1.dll", "wintun.dll")) {
    Assert-PEArchitecture (Join-Path $payload $binary) $expectedMachine
}
Copy-Item -LiteralPath (Join-Path $PSScriptRoot "lifecycle.ps1") -Destination $payload -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot "service-install.ps1") -Destination $payload -Force
if ($PayloadOnly) { return }
$compiler = @(
    "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
    "$env:ProgramFiles\Inno Setup 6\ISCC.exe"
) | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
if (-not $compiler) { throw "Install Inno Setup 6, then run this command again." }
& $compiler "/DAppVersion=$numericVersion" "/DBuildArchitecture=$Architecture" "/DAllowedArchitecture=$installerArchitecture" (Join-Path $PSScriptRoot "Caspian.iss")
if ($LASTEXITCODE -ne 0) { throw "Inno Setup failed." }
Get-ChildItem $output -Filter "CaspianSetup-*.exe"
