# Layout and conventions

[🇮🇷 فارسی](LAYOUT.fa.md) | 🇬🇧 **English** | [🇷🇺 Русский](../README.ru.md) | [🇨🇳 中文](../README.zh.md)

> Persian edition: [`docs/LAYOUT.fa.md`](LAYOUT.fa.md). The English file is the
> one the tests read. If the two ever disagree, this one is correct.

Fixed on 2026-08-30. Everything in the project agrees with this file. Change it
here first, not in one package.

## Names

| Thing | Value |
|---|---|
| Product | Caspian-BYOC |
| Binary | `caspian` |
| Go module | `caspianbyoc.org/caspian` |
| Service user and group | `caspian`, a system account with no login shell |
| Privileged unit | `caspian.service` |
| Panel unit | `caspian-panel.service` |

## Paths

| Path | Mode | Owner | What |
|---|---|---|---|
| `/usr/local/bin/caspian` | 0755 | root | The single binary. Subcommands select the role |
| `/var/lib/caspian` | 0700 | caspian | Persistent state. Holds a credential |
| `/var/lib/caspian/state.json` | 0600 | caspian | Written atomically by package state |
| `/run/caspian` | 0750 | root:caspian | Runtime sockets |
| `/run/caspian/priv.sock` | 0660 | root:caspian | Panel to privileged service |
| `/run/caspian/hostapd.conf` | 0600 | root | Generated, rewritten every start |
| `/run/caspian/dnsmasq.conf` | 0600 | root | Generated, rewritten every start |
| `/run/caspian/dnsmasq/` | 0700 | caspian | Owned by dnsmasq after it drops privileges |
| `/run/caspian/dnsmasq/dnsmasq.pid` | 0644 | caspian | Written by dnsmasq itself |
| `/var/lib/caspian/netcfg.journal` | 0600 | root | Inverse of every applied network change |

Two corrections made on 2026-08-30, both because the code was right and this
file was not. First, the generated hostapd and dnsmasq files live under `/run`,
not `/etc`. They are rewritten on every start, they contain the WPA2 passphrase,
and `/run` is a tmpfs, so a credential does not persist into a file nobody knows
is there. Second, the network journal is named `netcfg.journal`, which is what
the code writes and tests.

### Why dnsmasq gets its own directory

`/run/caspian` is 0750 root:caspian, so the group can list it and cannot write
in it. dnsmasq drops to the `caspian` account, and whether it writes its pid file
before or after dropping is a property of dnsmasq we have not measured. Rather
than measure it and depend on the answer, dnsmasq gets a directory it owns, and
the question stops mattering.

Do not "fix" this by making `/run/caspian` group-writable. Permission to create
and delete inside a directory comes from the directory, not the file, so a
group-writable `/run/caspian` would let the unprivileged panel account delete
`hostapd.conf` and write its own, which the privileged side then hands to hostapd
running as root. That turns a pid-file inconvenience into local privilege
escalation.

## Ports

Fixed here so no package has to learn them from another package's test fixture.
`cmd/caspian` reads these and passes them in. No package hardcodes a value it
does not own.

| Port | Bind | What |
|---|---|---|
| 53 | hotspot interface | dnsmasq, DHCP and DNS for joined devices |
| 5354 | 127.0.0.1 | The engine's local DNS listener. dnsmasq's only permitted upstream |
| 8088 | panel address | The web panel |
| 10808 | 127.0.0.1 | SOCKS, for diagnostics, the exit-IP proof, and the interim macOS system proxy |

The 5354 pairing is the one that breaks quietly. dnsmasq refuses any upstream
that is not a loopback address, and the engine's listener is what answers there.
If the two drift, DNS stops resolving for every joined device while the hotspot
and the tunnel both look healthy. A cross-check test exists in `internal/xcfg`.
The values above are the reason it can check anything.

## Other platforms

The tables above are the Linux appliance's and are the ones the tests read.
The macOS and Windows ports (branch `port/platforms`, see `docs/PORTS.md`)
keep the same names and ports and change only what the operating system
forces; each value below is fixed in `cmd/caspian/paths_darwin.go` and
`cmd/caspian/paths_windows.go`.

| Thing | macOS | Windows |
|---|---|---|
| Binary | `/usr/local/bin/caspian` | `%ProgramFiles%\Caspian\caspian.exe`, with `caspian-tethering.exe` and `wintun.dll` beside it |
| Persistent state | `/Library/Application Support/Caspian` 0700 `_caspian` | `%ProgramData%\Caspian`, ACL: SYSTEM, Administrators, the panel account |
| Runtime directory | `/var/run/caspian` 0750 root:`_caspian` | none |
| Panel to privileged service | `/var/run/caspian/priv.sock` 0660 | named pipe `\\.\pipe\caspian-priv`, descriptor admitting SYSTEM, Administrators and the panel account |
| Service account | `_caspian`, a role account (UID 450 to 499, no shell) | `NT SERVICE\caspian-panel`, a virtual service account |
| Service manager | launchd: `org.caspianbyoc.caspian`, `org.caspianbyoc.caspian-panel` | Service Control Manager: `caspian`, `caspian-panel` |
| Tunnel device | `utun100` (xray-core's darwin TUN insists on `utunN`) | `xray0`, a Wintun adapter |
| Access point | Apple Internet Sharing on the built-in radio | Mobile Hotspot, through the tethering helper |
| Client subnet | chosen by the planner, given to Internet Sharing | 192.168.137.0/24, fixed by Internet Connection Sharing |

## Two processes, one binary

`caspian serve --privileged` runs as root. It owns routes, the firewall, the
access point and the engine. It accepts a short list of named actions over the
socket and never a command built from user input.

`caspian serve --panel` runs as the `caspian` user. It owns the web interface
and nothing privileged. It reaches the other over the socket.

The split exists so that a fault in the part that parses user input and serves
HTTP is not a fault in the part that holds root.

## Who writes what

Decided 2026-08-30, reversing an earlier call made before the panel's privileged
interface existed.

The panel process owns `state.json`. It runs as the `caspian` account, which owns
that file and its directory, and it is the only thing a person interacts with, so
it is the natural owner of what the person configures.

The privileged process owns `netcfg.journal`. It runs as root, that file is root
owned, and the journal records changes only root can make or undo.

Neither writes the other's file, so there is no shared writer and no lost update
to protect against. An earlier draft gave the privileged side both files. That
needed a lock, and it made the panel ask permission to save a setting the panel
alone is responsible for.

The privileged side receives everything it needs in the start request. It reads
no state file.

## Install

One command, in the style of a package-manager installer:

```
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"
```

The script is the only thing a user runs in a terminal. After it finishes, every
further action happens in the panel.

Rules for it:

- Refuse anything that is not Linux on x86_64, aarch64, armv7 or armv6, with a
  message naming what was found. armv6 was missing from this line while the
  installer accepted it and the table below listed it, which is the same
  omission that broke the older Pi models in a previous project.
- Detect the distribution and install only what is missing: `hostapd`,
  `dnsmasq`, `nftables`, `iw`, `iproute2`.
- Download the release artefact for the detected architecture and verify its
  SHA-256 against a published checksums file. Refuse on mismatch. Never install
  an unverified binary.
- Be idempotent. Running it twice is an upgrade, not a mess.
- Create the service user, the directories and the units with the modes in the
  table above.
- Ship an uninstall path that removes the units, the binary and the directories,
  and replays the network journal so the box's network is left as it was found.
- Print, at the end, the panel address and the first-run password, and nothing
  else that matters.
- Never print or log the user's proxy config.

## Architecture naming

The release artefacts follow the Go convention, not the kernel's:
`caspian-linux-amd64`, `caspian-linux-arm64`, `caspian-linux-arm`. The installer
maps `uname -m` onto those: `x86_64` to amd64, `aarch64` to arm64, `armv7l` and
`armv6l` to arm. A previous project in this workspace mapped armv6 onto an armv7
artefact and broke the older Pi models. Do not repeat that.
