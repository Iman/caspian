# caspian-tethering

The Windows tethering helper: a C# console program that drives Mobile Hotspot
through the WinRT `NetworkOperatorTetheringManager` API, because that API has
no Go projection. `internal/hotspot/mobilehotspot.go` spawns it once per action
with one JSON request on standard input and reads one JSON line back.

    echo {"op":"status","uplink":"Ethernet"} | caspian-tethering.exe status

Operations: `start` (uplink, adapter, ssid, passphrase, band), `stop`, `status`.
Replies carry `ok`, `state` (on, off, transition, unknown), `ssid`, `band`,
`clients` (MAC and host names, never addresses) and, on failure, `code` (the
Windows enum name, for example `NetworkLimitedConnectivity`) and `error`.

Build on Windows with the .NET 9 SDK:

    dotnet publish -c Release -r win-x64 -o out

and place `caspian-tethering.exe` beside `caspian.exe`. The project also
compiles on macOS and Linux (`EnableWindowsTargeting`) as a check that the code
is well formed; the result is not run there.

What it does on `start`, in order: find the uplink's connection profile, check
the tethering capability (and report the Windows reason when it is not
`Enabled`), create the manager on the named adapter when one is given, apply
SSID, passphrase and band, disable the five-minute no-clients timeout, start,
and report the state read back from Windows.
