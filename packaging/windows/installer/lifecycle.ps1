param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('install', 'start', 'stop', 'restart', 'reset-password')]
    [string]$Action
)
$ErrorActionPreference = 'Stop'
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    try {
        $powershell = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
        $arguments = '-NoProfile -ExecutionPolicy Bypass -File "' + $PSCommandPath + '" -Action ' + $Action
        $child = Start-Process -FilePath $powershell -ArgumentList $arguments -Verb RunAs -Wait -PassThru
        exit $child.ExitCode
    } catch { exit 1 }
}

function Set-Automatic([string]$Name, [bool]$Enabled) {
    $mode = if ($Enabled) { 'auto' } else { 'disabled' }
    $sc = Join-Path $env:SystemRoot 'System32\sc.exe'
    & $sc config $Name start= $mode | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Service configuration failed.' }
}
function Stop-Caspian {
    Set-Automatic 'caspian-panel' $false
    Set-Automatic 'caspian' $false
    foreach ($name in @('caspian-panel', 'caspian')) {
        $service = Get-Service -Name $name
        if ($service.Status -ne 'Stopped') {
            $service.Stop()
            $service.WaitForStatus([ServiceProcess.ServiceControllerStatus]::Stopped, [TimeSpan]::FromSeconds(30))
        }
    }
}
function Start-Caspian {
    Set-Automatic 'caspian' $true
    Set-Automatic 'caspian-panel' $true
    foreach ($name in @('caspian', 'caspian-panel')) {
        $service = Get-Service -Name $name
        if ($service.Status -ne 'Running') {
            $service.Start()
            $service.WaitForStatus([ServiceProcess.ServiceControllerStatus]::Running, [TimeSpan]::FromSeconds(30))
        }
    }
}

try {
    switch ($Action) {
        'install' {
            foreach ($name in @('caspian-panel', 'caspian')) {
                $service = Get-Service -Name $name -ErrorAction SilentlyContinue
                if ($service -and $service.Status -ne 'Stopped') {
                    $service.Stop()
                    $service.WaitForStatus([ServiceProcess.ServiceControllerStatus]::Stopped, [TimeSpan]::FromSeconds(30))
                }
            }
            $password = & (Join-Path $PSScriptRoot 'service-install.ps1') -InstallDirectory $PSScriptRoot -GeneratePassword
            if ($password) {
                Add-Type -AssemblyName System.Windows.Forms
                [Windows.Forms.MessageBox]::Show(
                    "Save this Caspian password before you continue:`r`n`r`n$password`r`n`r`nUse it to sign in to Caspian. It differs from your Windows password.",
                    'Caspian setup', [Windows.Forms.MessageBoxButtons]::OK, [Windows.Forms.MessageBoxIcon]::Information) | Out-Null
                $password = $null
            }
        }
        'start' { Start-Caspian }
        'stop' { Stop-Caspian }
        'restart' { Stop-Caspian; Start-Caspian }
        'reset-password' {
            $backend = Join-Path $PSScriptRoot 'caspian.exe'
            # Keep the generated credential in this elevated process. Neither
            # a temporary file nor an unelevated IPC endpoint receives it.
            $result = & $backend reset-password 2>&1
            if ($LASTEXITCODE -ne 0) { throw 'Password recovery failed.' }
            $passwordLine = @($result | ForEach-Object { "$_" } | Where-Object { $_.StartsWith('New Caspian panel password: ') })
            if ($passwordLine.Count -ne 1) { throw 'Password recovery was not confirmed.' }
            Add-Type -AssemblyName System.Windows.Forms
            [Windows.Forms.MessageBox]::Show($passwordLine[0], 'Caspian', [Windows.Forms.MessageBoxButtons]::OK, [Windows.Forms.MessageBoxIcon]::Information) | Out-Null
            $result = $null
            $passwordLine = $null
        }
    }
    exit 0
} catch {
    # Do not return arbitrary service or backend output to the application.
    exit 1
}
