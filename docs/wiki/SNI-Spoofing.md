[English](https://github.com/Iman/caspian/wiki/SNI-Spoofing) · [فارسی](https://github.com/Iman/caspian/wiki/SNI-Spoofing.fa) · [Русский](https://github.com/Iman/caspian/blob/feature/sni/README.ru.md) · [中文](https://github.com/Iman/caspian/blob/feature/sni/README.zh.md)

# Caspian SNI spoofing and TLS splitting for DPI circumvention

This guide covers optional deep packet inspection (DPI) circumvention in `feature/sni`, not the current released installer.

SNI is the server name in a TLS greeting.
This feature sends an extra greeting with a spoof name before the real proxy stream.
The real TLS or REALITY name in your imported config stays unchanged.
An imported `sni` value does not automatically enable spoofing.

## Set a spoof name

1. Save your proxy config in the panel.
2. Open **DPI circumvention (optional)** on the dashboard.
3. Enter a domain name, such as `cover.example.invalid` for a local test.
4. Save the setting.

Use a suitable domain for your actual network; the example domain cannot resolve.
If Caspian is on, saving reconnects it with the new setting.
To disable spoofing, clear the field and save it.
Replacing the config clears the spoof name and both split settings.
Selecting another entry or refreshing a subscription preserves it.

## Independent TCP split and TLS-record split

Open **DPI circumvention (optional)** beside your saved config:

- **Fake server name**: leave blank to turn fake SNI off.
- **TCP split**: send the initial TLS greeting in two writes near the middle of the SNI hostname.
- **TLS-record split**: divide a TLS greeting record at that position without changing the handshake contents.

Each option is independent; you can combine them or leave all three off.
Without an SNI extension, splitting uses a position just after the handshake type.
TCP splitting is best effort: separate writes do not guarantee separate packets on every OS and network.
TLS-record splitting changes record boundaries and can fail with some servers or CDNs.
Both split options currently require ordinary TLS with VLESS, VMess, or Trojan over the supported IPv4 TCP transports.
REALITY splitting is not enabled; REALITY can still use fake SNI alone.
Only the initial ClientHello is split; later application traffic is relayed normally.
Split-only mode needs no WinDivert, packet socket, or BPF access.
When fake SNI is also selected, its packet confirmation must finish before any real greeting is sent.
Malformed, incomplete, oversized, or stalled greetings close the connection; no retry silently removes an enabled option.
Saving reconnects an active tunnel; config replacement clears all three choices.

The state fields are `spoof_sni`, `tcp_split`, and `tls_record_split`.
Upgrading a version 4 file preserves its fake name and leaves both split settings off.

## State compatibility

This branch writes state schema version 5.
Older builds refuse this state file to prevent data loss.
Keep a pre-upgrade state backup if you need to return to an older build.

## Limits

This version supports VLESS, VMess, and Trojan over IPv4 TCP, including WebSocket, HTTPUpgrade, and gRPC.
It does not support Hysteria2, QUIC, SOCKS, Shadowsocks, XHTTP, or IPv6-only servers.
SOCKS can negotiate a separate UDP endpoint that this TCP forwarder cannot cover.
Native Shadowsocks UDP, including tunnel DNS traffic, cannot use this TCP-only forwarder.
The forwarder chooses the first available IPv4 address from the detected server addresses.
If confirmation fails, it closes the connection without forwarding the real stream.
It does not fall back to an ordinary connection.

Fake SNI on Windows x64 needs the WinDivert files included in the local installer build.
Windows ARM64 cannot use fake SNI; split-only mode does not use the packet backend.
Fake SNI on Linux needs packet-socket privileges; macOS needs access to a BPF device on an Ethernet-style interface.
The privileged Caspian service owns these resources and closes them when it stops.

The Windows loopback test proves that the server receives the real stream unchanged.
It does not prove that spoofing works against your provider's filtering.
Linux and macOS builds also need live packet tests on their target systems.

## Credits

The primary source and idea are [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing), licensed under GPL-3.0.
See [third-party credits](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md).

## DPI bypass, SNI spoofing, and security

### Is Caspian DPI safe?

There is no universal DPI-safe guarantee. Optional SNI spoofing attempts to influence how a filtering system reads the initial TCP traffic.
It does not hide the server IP, traffic volume, or timing, and a provider can still block the connection.
The `feature/sni` implementation keeps the real TLS identity and rejects failed spoof confirmation instead of sending the real stream directly.

### Does Caspian include GoodbyeDPI or zapret?

No. [GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) and [zapret](https://github.com/bol-van/zapret) are research references for possible future strategies.
The primary SNI code and idea come from [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing), with GPL attribution preserved.
Caspian does not bundle those other projects or claim their authors endorse it.

### Does a config with SNI enable DPI bypass automatically?

No. The imported SNI is the real server identity. Set the separate optional spoof name to enable this mode.
Read [SNI setup and limitations](https://github.com/Iman/caspian/wiki/SNI-Spoofing) before enabling it.


<!-- Caspian guide navigation -->

Caspian guides: [setup and supported protocols](https://github.com/Iman/caspian/blob/feature/sni/README.md) · [SNI spoofing for DPI circumvention: setup and limits](https://github.com/Iman/caspian/wiki/SNI-Spoofing).

[Validation results and remaining hardware tests](https://github.com/Iman/caspian/blob/feature/sni/docs/SNI-VALIDATION.md).
