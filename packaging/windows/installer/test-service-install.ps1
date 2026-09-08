# SPDX-License-Identifier: AGPL-3.0-or-later
# Run with PowerShell. Windows services and ACL commands are replaced by mocks.
$ErrorActionPreference = 'Stop'
if ($PSVersionTable.PSVersion.Major -ge 7) { $PSStyle.OutputRendering = 'PlainText' }
$installer = Join-Path $PSScriptRoot 'service-install.ps1'
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('caspian-setup-test-' + [Guid]::NewGuid().ToString('N'))
$previousProgramData = $env:ProgramData
$env:ProgramData = $temporary
$global:CaspianSetupTest = @{}
$passed = 0
$failed = 0

function global:Get-Service { param($Name) return $null }
function global:New-Service { param($Name, $BinaryPathName, $DisplayName, $StartupType) }
function global:sc.exe { $global:LASTEXITCODE = $global:CaspianSetupTest.SCExit }
function global:icacls.exe {
    $global:LASTEXITCODE = 0
    if ($global:CaspianSetupTest.ACLFailure -and "$($args[0])".EndsWith('first-run-password')) {
        $global:LASTEXITCODE = 5
    }
}
function global:Start-Service {
    param($Name)
    if ($global:CaspianSetupTest.StartFailure) { throw 'Synthetic service refusal.' }
    $global:CaspianSetupTest.Starts.Add($Name)
}
function Assert-True($Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}
function Expect-Failure([scriptblock]$Action) {
    $didFail = $false
    try { & $Action | Out-Null } catch { $didFail = $true }
    Assert-True $didFail 'Expected installation to fail.'
}
function Case([string]$Name, [scriptblock]$Test) {
    if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Recurse -Force }
    New-Item -ItemType Directory -Path (Join-Path $temporary 'Caspian') -Force | Out-Null
    $global:CaspianSetupTest = @{
        SCExit = 0; ACLFailure = $false; StartFailure = $false
        Starts = [Collections.Generic.List[string]]::new()
    }
    try { & $Test; $script:passed++; Write-Output "PASS $Name" }
    catch { $script:failed++; Write-Output "FAIL $Name : $($_.Exception.Message)" }
}

try {
    Case 'fresh in-app installation supplies one password after services start' {
        $result = @(& $installer -InstallDirectory $temporary -GeneratePassword)
        $seed = [IO.File]::ReadAllText((Join-Path $temporary 'Caspian/first-run-password'))
        Assert-True ($result.Count -eq 1 -and $result[0] -eq $seed -and $seed -match '^[a-f0-9]{24}$') 'Generated password was not retained and returned exactly once.'
        Assert-True ($global:CaspianSetupTest.Starts.Count -eq 2) 'Both services must start before success.'
    }
    Case 'existing installation preserves state and does not issue a password' {
        $password = 'synthetic-caller-password'
        $stateFile = Join-Path $temporary 'Caspian/state.json'
        [IO.File]::WriteAllText($stateFile, '{"synthetic":"existing"}')
        $result = @(& $installer -InstallDirectory $temporary -GeneratePassword)
        Assert-True ($result.Count -eq 0) 'An upgrade must not issue a replacement password.'
        Assert-True ([IO.File]::ReadAllText($stateFile) -eq '{"synthetic":"existing"}') 'Existing state changed.'
        Assert-True (-not (Test-Path (Join-Path $temporary 'Caspian/first-run-password'))) 'An upgrade created a seed.'
    }
    Case 'interrupted installation keeps its existing password' {
        [IO.File]::WriteAllText((Join-Path $temporary 'Caspian/first-run-password'), 'synthetic-retry-password')
        $result = @(& $installer -InstallDirectory $temporary -GeneratePassword)
        Assert-True ($result.Count -eq 1 -and $result[0] -eq 'synthetic-retry-password') 'Retry replaced or hid the password.'
    }
    Case 'invalid interrupted password is refused' {
        [IO.File]::WriteAllText((Join-Path $temporary 'Caspian/first-run-password'), 'short')
        Expect-Failure { & $installer -InstallDirectory $temporary -GeneratePassword }
        Assert-True ($global:CaspianSetupTest.Starts.Count -eq 0) 'Service started with an invalid password.'
    }
    Case 'fresh installation without either password source is refused' {
        Expect-Failure { & $installer -InstallDirectory $temporary }
        Assert-True ($global:CaspianSetupTest.Starts.Count -eq 0) 'Service started without a password.'
    }
    Case 'Setup password file is consumed without returning its secret' {
        $inputFile = Join-Path $temporary 'setup-password.txt'
        [IO.File]::WriteAllText($inputFile, "synthetic-setup-password`n")
        $result = @(& $installer -InstallDirectory $temporary -PasswordFile $inputFile)
        Assert-True ($result.Count -eq 0 -and -not (Test-Path $inputFile)) 'Setup password was exposed or not consumed.'
        Assert-True ([IO.File]::ReadAllText((Join-Path $temporary 'Caspian/first-run-password')) -eq 'synthetic-setup-password') 'Setup password did not reach the seed.'
    }
    Case 'password ACL failure prevents startup' {
        $global:CaspianSetupTest.ACLFailure = $true
        $inputFile = Join-Path $temporary 'setup-password.txt'
        [IO.File]::WriteAllText($inputFile, 'synthetic-setup-password')
        Expect-Failure { & $installer -InstallDirectory $temporary -PasswordFile $inputFile }
        Assert-True ($global:CaspianSetupTest.Starts.Count -eq 0) 'Service started after password ACL refusal.'
    }
    Case 'failed startup retains the password for retry' {
        $global:CaspianSetupTest.StartFailure = $true
        Expect-Failure { & $installer -InstallDirectory $temporary -GeneratePassword }
        $seedFile = Join-Path $temporary 'Caspian/first-run-password'
        $before = [IO.File]::ReadAllText($seedFile)
        $global:CaspianSetupTest.StartFailure = $false
        $result = @(& $installer -InstallDirectory $temporary -GeneratePassword)
        Assert-True ($result.Count -eq 1 -and $result[0] -eq $before) 'Retry did not return the retained password.'
    }
    Case 'service configuration refusal prevents seed creation and startup' {
        $global:CaspianSetupTest.SCExit = 5
        Expect-Failure { & $installer -InstallDirectory $temporary -GeneratePassword }
        Assert-True ($global:CaspianSetupTest.Starts.Count -eq 0) 'Service started after configuration refusal.'
        Assert-True (-not (Test-Path (Join-Path $temporary 'Caspian/first-run-password'))) 'Password generated after configuration refusal.'
    }
} finally {
    $env:ProgramData = $previousProgramData
    if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Recurse -Force }
    foreach ($name in @('Get-Service', 'New-Service', 'sc.exe', 'icacls.exe', 'Start-Service')) {
        Remove-Item -LiteralPath "Function:\$name" -ErrorAction SilentlyContinue
    }
    Remove-Variable CaspianSetupTest -Scope Global -ErrorAction SilentlyContinue
}
Write-Output "Service installation: $passed passed, $failed failed"
if ($failed -ne 0) { exit 1 }
