# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
<#
.SYNOPSIS
Build the Windows app, hotspot helper, and installer locally.
.EXAMPLE
powershell -NoProfile -ExecutionPolicy Bypass -File .\build-windows.ps1
.EXAMPLE
.\build-windows.ps1 -Version 1.2.3 -Architecture arm64
#>
[CmdletBinding()]
param(
    [string]$Version = "0.0.0",
    [ValidateSet("x64", "arm64")][string]$Architecture = "x64"
)
$ErrorActionPreference = "Stop"
& (Join-Path $PSScriptRoot "packaging\windows\installer\build-installer.ps1") -Version $Version -Architecture $Architecture
