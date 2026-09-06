# Licence and credits

[English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) | [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) | [中文](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh)

[Caspian wiki](https://github.com/Iman/caspian/wiki/Home)

> This guide comes from the existing README. Its measurements retain their original dates; this documentation move does not report a new test run.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## Licence

AGPL-3.0-or-later, with three additional terms under section 7. All three are of
a kind section 7 permits and none restricts what you may do with the software:
preserve the copyright notice, this attribution and a visible reference to the
Caspian project in any user interface; mark your version as changed if you
modify it; and do not use the authors' or the project's names for publicity,
which includes soliciting donations, sponsorship or grants in those names. The
full text is in [`LICENSE`](https://github.com/Iman/caspian/blob/main/LICENSE) and the terms are in [`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE).

That third term restricts the use of NAMES and nothing else. You remain free to
run, study, modify and redistribute the software under the AGPL, for any
purpose including a commercial one. What you may not do is raise money in the
authors' name.

The AGPL rather than the GPL, because this program is normally operated as a
service other people connect to, and section 13 closes the gap the plain GPL
leaves. Not a permissive licence, because the binary statically links
GPL-3.0-or-later code: `github.com/sagernet/sing` and
`github.com/sagernet/sing-shadowsocks`, both reached through xray-core. So the
combined work must be on GPL-family terms, and MIT or Apache-2.0 are not
available for it.

## Built on

Caspian is a small amount of code around other people's work. The engine is
xray-core, and the share-link parser is XTLS's. Neither project endorses this
one; they are credited because the work is theirs.

| Project | Licence | What it does here |
|---|---|---|
| [xray-core](https://github.com/xtls/xray-core) | MPL-2.0 | The proxy engine, linked in-process rather than run as a separate program |
| [libXray](https://github.com/XTLS/libXray) | MIT | The share-link parser, vendored under `third_party/libxray-share/` |
| [REALITY](https://github.com/xtls/reality) | MPL-2.0 | The TLS camouflage transport |
| [uTLS](https://github.com/refraction-networking/utls) | BSD-3-Clause | TLS fingerprint mimicry |
| [quic-go](https://github.com/apernet/quic-go) | MIT | The QUIC stack Hysteria2 runs on |
| [gVisor](https://github.com/google/gvisor) | Apache-2.0 | The userspace network stack the TUN inbound uses |
| [sing](https://github.com/sagernet/sing) and [sing-shadowsocks](https://github.com/sagernet/sing-shadowsocks) | GPL-3.0-or-later | Shadowsocks 2022, and the reason this project is copyleft |
| [netlink](https://github.com/vishvananda/netlink) | Apache-2.0 | Interfaces, addresses and routes |
| [miekg/dns](https://github.com/miekg/dns) | BSD-3-Clause | DNS message handling |
| [gorilla/websocket](https://github.com/gorilla/websocket) | BSD-2-Clause | The WebSocket transport |
| [CIRCL](https://github.com/cloudflare/circl) | BSD-3-Clause | Post-quantum key exchange |
| [Wintun](https://www.wintun.net/) | Wintun Prebuilt Binaries License | The signed `wintun.dll` tunnel driver on Windows |
| [.NET runtime and Windows Forms](https://github.com/dotnet/runtime) | MIT | The self-contained Windows helper and tray app runtime |
| `System.ServiceProcess.ServiceController` | MIT | Windows service control from `CaspianControl.exe` |

The Windows installation has one separate third-party DLL: `wintun.dll`.
Caspian distributes the official signed Wintun 0.14.1 binary without changes.
Its license is in
[`third_party/wintun/PREBUILT-BINARIES-LICENSE.txt`](https://github.com/Iman/caspian/blob/main/third_party/wintun/PREBUILT-BINARIES-LICENSE.txt) and is copied to
`C:\Program Files\Caspian\WINTUN-LICENSE.txt` during installation.

`caspian-tethering.exe` and `CaspianControl.exe` are self-contained .NET
programs. Their .NET components are inside the executable files, not beside
them as extra DLLs. The .NET license and notices are in `third_party/dotnet/`.
The Windows SDK reference package is a build input and is not installed with
Caspian.

It also needs `hostapd`, `dnsmasq`, `nftables`, `iw` and `iproute2` on the
machine. Those run as separate programs rather than being linked, so their
licences do not affect this one, but the appliance is nothing without them.

[`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE) carries the full record: every module in the binary, the licence read
from its own licence file, and the compatibility reasoning.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)
