# Build Windows locally

From the repository folder, run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\build-windows.ps1
```

The command builds the Go app, the hotspot helper, the desktop app, and the installer.
The default version is `0.0.0`, and the default architecture is `x64`.
The build does not need administrator access.

The build needs these tools:

- Go 1.26 or later
- .NET SDK 9 or later
- Inno Setup 6, in a standard installation folder or on `PATH`

Go and .NET must be on `PATH`.
The first build needs internet access for dependencies and Wintun.
The build saves Wintun in `.cache` and compares its checksum on every run.
A checksum identifies the expected contents of a file.

For a specific version or ARM64, run:

```powershell
.\build-windows.ps1 -Version 1.2.3 -Architecture arm64
```

The build produces these files:

- Installer: `out\installer\CaspianSetup-0.0.0-windows-x64.exe`
- Installer checksum: the same path with `.sha256` at the end
- App and helpers: `packaging\windows\installer\payload\x64\`

The paths use your selected version and architecture.
The installer includes the .NET runtime and Wintun.
The build creates the installer but does not run it.
The installer needs administrator access when you run it.

The command uses the same builder as the release workflow.
It stops when a build, version comparison, architecture comparison, or checksum comparison fails.
It does not run the full test suite.
