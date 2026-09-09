# SNI spoofing validation record

[English](SNI-VALIDATION.md) · [فارسی](SNI-VALIDATION.fa.md) · [Русский](../README.ru.md) · [中文](../README.zh.md)

Validated on 2026-09-09 on Windows 11 x64, build 22621, with Go 1.26.2.
This record covers the optional DPI circumvention work on `feature/sni`.
It does not certify Windows 10 hardware, Linux/macOS packet behavior, or bypass against an internet provider.

## Results

| Check | Result |
| --- | --- |
| Go package tests with race detection | Passed across the initial suite and affected-package reruns; configuration matrix passed separately in 1985 seconds |
| Configuration matrix | 96.7% statement coverage |
| Configuration endpoint rewrite | 98.7% statement coverage; real TLS/REALITY identity and transport authority preserved |
| SNI package | 71.1% statement coverage on Windows; native driver paths require the separate opt-in test |
| Regression | 99 golden tests, fixture leak scan, provenance and registry checks passed |
| Documentation | Local links, English/Persian parity, language navigation and source scans passed |
| Smoke | 87 test executions passed; 297 seconds under concurrent load, above the script's 30-second budget |
| API BDD | Full run: 31 scenarios and 274 steps; final SNI rerun: 2 scenarios and 11 steps passed |
| Browser BDD | Full run in real headless Chrome: 35 scenarios and 311 steps; final SNI rerun: 2 scenarios and 13 steps passed |
| Packet parser fuzzing | 58,219 executions in a 15-second campaign; no crash |
| Linux installer fixtures | 148 checks passed, including complete license payloads and uninstall preservation of unrelated documentation |
| Windows local build | Go app, .NET helpers and x64 installer built successfully; installer checksum verified; WinDivert driver signature valid |

The first complete gate invocation failed on missing documentation translations and its 30-minute configuration-test limit.
Those documentation failures were fixed and checked again; the unchanged configuration package passed with a 45-minute timeout.
The results above combine those runs; they are not a claim that one final invocation of the gate script passed.
The standalone scripts also passed after the final source and documentation changes.

## Native Windows packet and TLS proof

The opt-in WinDivert test uses only a loopback server.
It verifies that the decoy bytes do not enter the real stream, including a half-close and echo.
A real TLS connection then checks the trusted server name and receives an unchanged payload.
The negative case requires a certificate-name mismatch error; a generic connection failure is not accepted as proof.

One TLS run hit its 8-second client timeout while the installer was compressing.
After compression ended, three consecutive runs of both native tests passed.
Run native packet tests separately from heavy builds; this is local functional evidence, not a load benchmark.

Enable the native tests with `CASPIAN_SNI_NATIVE_TEST=1` and place the signed WinDivert files beside the compiled test executable.
The test binary must run with administrator access.
Go's race runtime on this host required the test-only linker flags `-linkmode=external` and `-extldflags=-Wl,--disable-dynamicbase`.
These flags were not used for the production installer.

## Remaining validation

- Test the supported Windows 10 floor on an actual Windows 10 system, including Mobile Hotspot and joined-device DNS.
- Run native Linux and macOS packet tests on their target systems; cross-compilation is not packet-path validation.
- Measure baseline and spoofed connections on the intended network, including real proxy delivery, DNS, reconnects and sustained traffic.
- Recheck NuGet vulnerability data: restore/build succeeded from cached dependencies, but the audit endpoint was unavailable (NU1900). The installed .NET SDK was a preview build.

See [SNI setup and limitations](SNI.md), [source research](THIRD-PARTY.md), and [third-party licenses](THIRD-PARTY.md).
