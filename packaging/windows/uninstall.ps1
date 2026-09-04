# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Remove Caspian-BYOC from Windows. Stops both services first, which makes the
# privileged service replay its teardown journal (routes, filters, the tunnel
# adapter), then deletes the services, the program directory and the state.
$ErrorActionPreference = "SilentlyContinue"
foreach ($svc in @("caspian-panel", "caspian")) {
    if (Get-Service -Name $svc -ErrorAction SilentlyContinue) {
        Stop-Service -Name $svc -Force
        & sc.exe delete $svc | Out-Null
    }
}
Remove-Item -Recurse -Force (Join-Path $env:ProgramFiles "Caspian")
Remove-Item -Recurse -Force (Join-Path $env:ProgramData "Caspian")
Write-Host "removed"
