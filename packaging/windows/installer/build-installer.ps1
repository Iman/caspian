[CmdletBinding()]
param([string]$Version = "0.2.1")
$ErrorActionPreference = "Stop"
$numericVersion = $Version.TrimStart("v")
$releaseVersion = "v$numericVersion"
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\..\.."))
$payload = Join-Path $PSScriptRoot "payload"
$output = Join-Path $repo "out\installer"
New-Item -ItemType Directory -Force -Path $payload, $output | Out-Null

Push-Location $repo
try {
    & go.exe build -trimpath -ldflags "-X main.version=$releaseVersion -X caspianbyoc.org/caspian/internal/panel.Version=$releaseVersion" -o (Join-Path $payload "caspian.exe") .\cmd\caspian
    if ($LASTEXITCODE -ne 0) { throw "The Go build failed." }
    & dotnet.exe publish .\tools\caspian-tethering\caspian-tethering.csproj -c Release -r win-x64 --self-contained true -o $payload
    if ($LASTEXITCODE -ne 0) { throw "The hotspot helper build failed." }
    & dotnet.exe publish .\tools\caspian-control\caspian-control.csproj -c Release -r win-x64 --self-contained true -p:Version=$numericVersion -o $payload
    if ($LASTEXITCODE -ne 0) { throw "The tray app build failed." }
} finally { Pop-Location }

$wintun = Join-Path $repo "wintun.dll"
if (-not (Test-Path -LiteralPath $wintun)) {
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ("caspian-wintun-" + [Guid]::NewGuid().ToString("N"))
    $archive = Join-Path $temporary "wintun.zip"
    New-Item -ItemType Directory -Path $temporary | Out-Null
    try {
        Invoke-WebRequest -UseBasicParsing -Uri "https://www.wintun.net/builds/wintun-0.14.1.zip" -OutFile $archive
        $actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -ne "07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51") { throw "The Wintun checksum does not match." }
        Expand-Archive -LiteralPath $archive -DestinationPath (Join-Path $temporary "expanded")
        Copy-Item -LiteralPath (Join-Path $temporary "expanded\wintun\bin\amd64\wintun.dll") -Destination $wintun
    } finally { Remove-Item -LiteralPath $temporary -Recurse -Force -ErrorAction SilentlyContinue }
}
Copy-Item -LiteralPath $wintun -Destination (Join-Path $payload "wintun.dll") -Force
$compiler = @(
    "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
    "$env:ProgramFiles\Inno Setup 6\ISCC.exe"
) | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
if (-not $compiler) { throw "Install Inno Setup 6, then run this command again." }
& $compiler "/DAppVersion=$numericVersion" (Join-Path $PSScriptRoot "Caspian.iss")
if ($LASTEXITCODE -ne 0) { throw "Inno Setup failed." }
Get-ChildItem $output -Filter "CaspianSetup-*.exe"
