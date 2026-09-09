# Caspian-BYOC

[**Download latest release**](https://github.com/Iman/caspian/releases/latest) | [**Open wiki**](https://github.com/Iman/caspian/wiki/Home)

To build the Windows app and installer locally, see [Windows build instructions](docs/WINDOWS-BUILD.md).

<div dir="ltr" align="left">

[English](README.md) | [فارسی](README.fa.md) | [Русский](README.ru.md) | [中文](README.zh.md) | [العربية](https://github.com/Iman/caspian/wiki/Home.ar) | [اردو](https://github.com/Iman/caspian/wiki/Home.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Home.tr)

</div>

<div dir="ltr" align="left">

[![ci](https://github.com/Iman/caspian/actions/workflows/ci.yml/badge.svg)](https://github.com/Iman/caspian/actions/workflows/ci.yml) [![release](https://img.shields.io/github/v/release/Iman/caspian?label=release)](https://github.com/Iman/caspian/releases/latest) [![licence AGPL-3.0-or-later](https://img.shields.io/badge/licence-AGPL--3.0--or--later-blue)](LICENSE) [![platform Windows, macOS, Raspberry Pi and Linux](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Raspberry%20Pi%20%7C%20Linux-blue)](https://github.com/Iman/caspian/releases/latest) [![container](https://img.shields.io/badge/ghcr.io-caspian-blue)](https://github.com/Iman/caspian/pkgs/container/caspian) [![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/Iman/caspian)

</div>

![Your devices join the box's Wi-Fi. The box connects with the config you pasted and tunnels everything to your own server abroad, so your home router and your internet provider see one encrypted connection to one address instead of what you open.](docs/images/flow-en.svg)

Caspian-BYOC turns a Windows PC, Mac running macOS, Raspberry Pi, or Linux
computer into a bring-your-own-config WiFi gateway. Paste a V2Ray or
Xray-compatible proxy configuration into the web panel and press one switch.
Caspian accepts VLESS,
VMess, Shadowsocks, SOCKS, Trojan, and Hysteria2 share links. It also accepts
Clash and Clash.Meta YAML, raw Xray JSON, link lists, and base64 subscription
data. Caspian connects through Xray-core and shares the tunnel as a WiFi
hotspot, so every device that joins is protected without installing an app.

![The Caspian panel, connected](docs/images/panel-en.png)

The panel above is a real screenshot from a running box, taken on a Raspberry Pi
5 on 2026-09-03 with the tunnel up, before any device had joined. The network
passphrase, the configuration name and the server address in it are substituted,
and the join code is blurred, because that code encodes the network name and its
password. Nothing else is altered.

The panel opens in English. Choose Persian from the language menu at the top. There is no account, no
telemetry, and the panel fetches nothing from the internet unless you press
Refresh on a subscription address you saved.

![Caspian Control on Windows](docs/images/caspian-control-windows.png)

![Caspian Control on macOS](docs/images/caspian-control-macos.png)

## Connections and supported formats

Start with Ethernet from your router to the computer running Caspian. Use that computer's built-in Wi-Fi for the hotspot, or a compatible USB Wi-Fi adapter on Linux. This gives the internet connection and hotspot separate adapters. It is the recommended starting arrangement, not a measured speed guarantee.

In these diagrams, [1] is your internet router, [2] is the computer running Caspian, and [3] is your phone or another device. ETH means an Ethernet cable. A USB Ethernet adapter brings internet in; a USB Wi-Fi adapter creates a wireless connection. They do different jobs.

```text
A  [1] --ETH--> [2] --built-in Wi-Fi--> [3]
B  [1] --ETH--> [2] --USB Wi-Fi-------> [3]
C  [1] --Wi-Fi A--> [2] --Wi-Fi B----> [3]
D  [1] --Wi-Fi--> [2: one radio] --Wi-Fi--> [3]
```

| Internet into Caspian | Hotspot to your devices | Linux / Raspberry Pi | macOS |
|---|---|---|---|
| A. Ethernet | Built-in Wi-Fi | Supported when the driver can create a hotspot | Supported arrangement |
| B. Ethernet | External USB Wi-Fi | Requires a Linux driver with access point (AP) support | Not supported as the hotspot by Caspian |
| C. Wi-Fi adapter A | Separate Wi-Fi adapter B | Requires AP support on adapter B | An external USB Wi-Fi hotspot is not supported |
| D. Wi-Fi | The same Wi-Fi radio | Conditional: the driver must allow a station and AP together; the channel may be shared | Not supported on the built-in radio |

These are the arrangements the current code can plan or refuse. They do not certify every adapter, OS update, or laptop. A Wi-Fi adapter that can join your home network may still be unable to create a hotspot. Linux USB arrangements have modelled tests; the existing hardware record does not establish that every USB adapter works. On macOS, use Ethernet and built-in Wi-Fi for the documented path. Plugging in USB Wi-Fi does not remove that restriction.

Caspian accepts VLESS, VMess, Shadowsocks, SOCKS, Trojan, and Hysteria2 links, including the hy2 alias. It also accepts supported Clash/Clash.Meta YAML, Xray JSON, lists of links, and base64 subscription content. It uses whichever entry of a list you choose. A subscription address can be saved beside the config and refreshed when you press the button, through the tunnel. Ask your provider for the actual supported configuration, not an account password or a web page link.

Supported transport names include raw/tcp, ws, grpc, httpupgrade, xhttp/splithttp, and kcp/mkcp. Protocol, transport, and security settings must be compatible; not every combination works. TUIC, WireGuard, SSR, AnyTLS, and Hysteria v1 links are not supported. Do not rename an unsupported protocol to make it pass validation. See the protocol guide for restrictions and test evidence.

[For connection diagrams, cable-first setup, service restarts, and common errors, read the home-user troubleshooting guide.](https://github.com/Iman/caspian/wiki/Troubleshooting)

## Install and read the guides

CPU and RAM: Caspian has no measured minimum RAM, CPU core count, or clock speed yet. Resource use depends on traffic volume, proxy protocol, and simultaneous connections. Idle and load benchmarks are needed before minimum requirements can be published.

Links labelled English in the Arabic, Urdu, and Turkish columns open the English guide where a translation is not yet available.

<div dir="ltr" align="left">

| Topic | English | فارسی | Русский | 中文 | العربية | اردو | Türkçe |
|---|---|---|---|---|---|---|---|
| Getting started | [English](https://github.com/Iman/caspian/wiki/Getting-Started) | [فارسی](https://github.com/Iman/caspian/wiki/Getting-Started.fa) | [Русский](https://github.com/Iman/caspian/wiki/Getting-Started.ru) | [中文](https://github.com/Iman/caspian/wiki/Getting-Started.zh) | [العربية](https://github.com/Iman/caspian/wiki/Getting-Started.ar) | [اردو](https://github.com/Iman/caspian/wiki/Getting-Started.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Getting-Started.tr) |
| Installation | [English](https://github.com/Iman/caspian/wiki/Installation) | [فارسی](https://github.com/Iman/caspian/wiki/Installation.fa) | [Русский](https://github.com/Iman/caspian/wiki/Installation.ru) | [中文](https://github.com/Iman/caspian/wiki/Installation.zh) | [العربية](https://github.com/Iman/caspian/wiki/Installation.ar) | [اردو](https://github.com/Iman/caspian/wiki/Installation.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Installation.tr) |
| Install on Linux and Raspberry Pi | [English](https://github.com/Iman/caspian/wiki/Install-Linux) | [فارسی](https://github.com/Iman/caspian/wiki/Install-Linux.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-Linux.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-Linux.zh) | [العربية](https://github.com/Iman/caspian/wiki/Install-Linux.ar) | [اردو](https://github.com/Iman/caspian/wiki/Install-Linux.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Install-Linux.tr) |
| Install on macOS | [English](https://github.com/Iman/caspian/wiki/Install-macOS) | [فارسی](https://github.com/Iman/caspian/wiki/Install-macOS.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-macOS.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-macOS.zh) | [العربية](https://github.com/Iman/caspian/wiki/Install-macOS.ar) | [اردو](https://github.com/Iman/caspian/wiki/Install-macOS.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Install-macOS.tr) |
| Install on Windows | [English](https://github.com/Iman/caspian/wiki/Install-Windows) | [فارسی](https://github.com/Iman/caspian/wiki/Install-Windows.fa) | [Русский](https://github.com/Iman/caspian/wiki/Install-Windows.ru) | [中文](https://github.com/Iman/caspian/wiki/Install-Windows.zh) | [العربية](https://github.com/Iman/caspian/wiki/Install-Windows.ar) | [اردو](https://github.com/Iman/caspian/wiki/Install-Windows.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Install-Windows.tr) |
| Protocols and transports | [English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh) | [English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports) | [English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports) | [English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports) |
| Architecture and data flow | [English](https://github.com/Iman/caspian/wiki/Architecture) | [فارسی](https://github.com/Iman/caspian/wiki/Architecture.fa) | [Русский](https://github.com/Iman/caspian/wiki/Architecture.ru) | [中文](https://github.com/Iman/caspian/wiki/Architecture.zh) | [English](https://github.com/Iman/caspian/wiki/Architecture) | [English](https://github.com/Iman/caspian/wiki/Architecture) | [English](https://github.com/Iman/caspian/wiki/Architecture) |
| Panel and configuration | [English](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) | [فارسی](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.fa) | [Русский](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.ru) | [中文](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.zh) | [English](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) | [English](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) | [English](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) |
| Security and privacy | [English](https://github.com/Iman/caspian/wiki/Security-and-Privacy) | [فارسی](https://github.com/Iman/caspian/wiki/Security-and-Privacy.fa) | [Русский](https://github.com/Iman/caspian/wiki/Security-and-Privacy.ru) | [中文](https://github.com/Iman/caspian/wiki/Security-and-Privacy.zh) | [English](https://github.com/Iman/caspian/wiki/Security-and-Privacy) | [English](https://github.com/Iman/caspian/wiki/Security-and-Privacy) | [English](https://github.com/Iman/caspian/wiki/Security-and-Privacy) |
| Development and testing | [English](https://github.com/Iman/caspian/wiki/Development-and-Testing) | [فارسی](https://github.com/Iman/caspian/wiki/Development-and-Testing.fa) | [Русский](https://github.com/Iman/caspian/wiki/Development-and-Testing.ru) | [中文](https://github.com/Iman/caspian/wiki/Development-and-Testing.zh) | [English](https://github.com/Iman/caspian/wiki/Development-and-Testing) | [English](https://github.com/Iman/caspian/wiki/Development-and-Testing) | [English](https://github.com/Iman/caspian/wiki/Development-and-Testing) |
| Troubleshooting and known defects | [English](https://github.com/Iman/caspian/wiki/Troubleshooting) | [فارسی](https://github.com/Iman/caspian/wiki/Troubleshooting.fa) | [Русский](https://github.com/Iman/caspian/wiki/Troubleshooting.ru) | [中文](https://github.com/Iman/caspian/wiki/Troubleshooting.zh) | [العربية](https://github.com/Iman/caspian/wiki/Troubleshooting.ar) | [اردو](https://github.com/Iman/caspian/wiki/Troubleshooting.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Troubleshooting.tr) |
| Releases and maintenance | [English](https://github.com/Iman/caspian/wiki/Releases-and-Maintenance) | [فارسی](https://github.com/Iman/caspian/wiki/Releases-and-Maintenance.fa) | [Русский](https://github.com/Iman/caspian/wiki/Releases-and-Maintenance.ru) | [中文](https://github.com/Iman/caspian/wiki/Releases-and-Maintenance.zh) | [English](https://github.com/Iman/caspian/wiki/Releases-and-Maintenance) | [English](https://github.com/Iman/caspian/wiki/Releases-and-Maintenance) | [English](https://github.com/Iman/caspian/wiki/Releases-and-Maintenance) |
| Licence and credits | [English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) | [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) | [中文](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh) | [English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) |
| Documentation map | [English](https://github.com/Iman/caspian/wiki/Documentation-Map) | [فارسی](https://github.com/Iman/caspian/wiki/Documentation-Map.fa) | [Русский](https://github.com/Iman/caspian/wiki/Documentation-Map.ru) | [中文](https://github.com/Iman/caspian/wiki/Documentation-Map.zh) | [English](https://github.com/Iman/caspian/wiki/Documentation-Map) | [English](https://github.com/Iman/caspian/wiki/Documentation-Map) | [English](https://github.com/Iman/caspian/wiki/Documentation-Map) |
| Translations | [English](https://github.com/Iman/caspian/wiki/Translations) | [فارسی](https://github.com/Iman/caspian/wiki/Translations.fa) | [Русский](https://github.com/Iman/caspian/wiki/Translations.ru) | [中文](https://github.com/Iman/caspian/wiki/Translations.zh) | [العربية](https://github.com/Iman/caspian/wiki/Translations.ar) | [اردو](https://github.com/Iman/caspian/wiki/Translations.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Translations.tr) |
| Page template | [English](https://github.com/Iman/caspian/wiki/Page-Template) | [فارسی](https://github.com/Iman/caspian/wiki/Page-Template.fa) | [Русский](https://github.com/Iman/caspian/wiki/Page-Template.ru) | [中文](https://github.com/Iman/caspian/wiki/Page-Template.zh) | [English](https://github.com/Iman/caspian/wiki/Page-Template) | [English](https://github.com/Iman/caspian/wiki/Page-Template) | [English](https://github.com/Iman/caspian/wiki/Page-Template) |

</div>

## Recorded experiments

> This guide comes from the existing README. Its measurements retain their original dates; this documentation move does not report a new test run.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

### What has carried bytes through a real server

Added by `test/tunnel`. Every scheme the parser accepts is driven end to end
against a real xray-core instance, built from this module's own dependency and
loaded through the same loader `internal/engine` uses. The client side is the
product path, unmodified: `link.Parse`, then `xcfg.Build`, then
`engine.Engine.Start`. No config is hand-written.

| protocol | transport | security | carries an HTTP request |
|---|---|---|---|
| VLESS | tcp (raw) | none | yes |
| VMess | tcp (raw) | none | yes |
| Shadowsocks, aes-256-gcm | tcp (raw) | none | yes |
| SOCKS | tcp (raw) | none | yes |
| Trojan | tcp (raw) | TLS, pinned by digest | yes |
| Hysteria2, and the `hy2` alias | QUIC | TLS, pinned by digest | yes |

Four controls stop a request that skipped the tunnel from passing, and all four
run rather than being asserted in prose. The client is never told where the
origin is, and is given a `.invalid` name and the port of a decoy. The name
cannot be resolved, and the suite says so out loud if a resolver on the machine
answers it anyway. The origin checks where the request was addressed, not only
that it arrived. The decoy counts its own hits, and a tunnelled request must add
none. `TestEveryCarriageProofCanFail` and
`TestTheProofRejectsARequestThatDidNotGoThroughTheTunnel` are what make those
controls evidence rather than intent.

Read each row narrowly. Every row but Hysteria2 runs over raw TCP. No row drives
REALITY, whose server side needs a real handshake target. Shadowsocks is
aes-256-gcm only, because the 2022 ciphers take a different code path. Every row
carries a TCP request, and UDP associate is off. Everything is on loopback, so
no exit IP is captured and none can be.

`TestEveryProtocolTheParserAcceptsIsDrivenEndToEnd` reads the accepted-scheme
list out of `internal/link`'s source, so an eighth scheme cannot be added
without a row here.


### What has actually been proven on hardware

The table below is what real traffic has traversed with an exit IP captured. It
is not what the parser accepts, and it is not what the loopback suite carries.

| protocol | transport | security | proven end to end |
|---|---|---|---|
| VLESS | tcp (raw) | REALITY | yes, on three separate servers |
| VLESS | ws (WebSocket) | none, plus VLESS Encryption | yes |
| VLESS | ws (WebSocket) | TLS | yes, through a CDN |
| VLESS | httpupgrade | TLS | yes, through a CDN |
| VLESS | xhttp | TLS | yes |
| VMess, Trojan, Shadowsocks, SOCKS, Hysteria2 | any | any | no |

Each of those was proven by driving a real browser on a real phone joined to the
hotspot. The exit address was captured from two independent sources and matched
to the server the configuration names. Three different servers were used and
each returned a different address, so a repeated or cached reading cannot be
mistaken for a working tunnel.

A row that is not proven is not a claim that it is broken. It is a claim that
nobody has watched a packet come out of the far end, which is a different thing
and the only thing this project treats as evidence. The engine document each
transport produces IS pinned as a golden file, so a change to how one is
composed shows up as a diff. That proves the document is stable and says nothing
about whether the transport connects.


## What has actually been verified

### The Go suite, this repository, measured 2026-08-31

At commit `5b0a8a7` with a clean working tree, on go1.27.0 darwin/arm64:

    go build ./...                 exit 0
    go test -count=1 -v ./...      exit 0

That run executed 1577 tests including subtests: 1572 passed, 5 skipped, 0
failed. Fifteen packages reported `ok`. Two have no test files: `bdd/harness`
and `local/devpanel`. The five skips announce what they are not proving: the TUN
device lifecycle, which is linux-only and needs root and `/dev/net/tun`, three
dnsmasq configuration checks that need dnsmasq installed, and an opt-in QR PNG
dump.

The previous recorded run, at commit `dd15ad6` on 2026-08-30, executed 1323
tests including subtests: 1319 passed, 4 skipped, 0 failed, across twelve
packages reporting `ok`.

`-count=1` is not optional. It defeats the result cache. Without it a second run
prints the first run's PASS lines and exits 0 having executed nothing.

The full gate is [`scripts/gate.sh`](https://github.com/Iman/caspian/blob/main/scripts/gate.sh): gofmt, `go vet`, the whole suite with the
race detector, and a per-package coverage floor. Read its header before you pipe
it anywhere. A shell pipeline returns the status of its last command, and that
trap has produced a false green in this project before.

[`packaging/test-install.sh`](https://github.com/Iman/caspian/blob/main/packaging/test-install.sh) covers the two shell scripts on any machine with
bash, including one that cannot be installed to.

### The behaviour suite

[`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) lists 24 scenarios. The 2026-08-31 run executed all 24, and
`TestEveryScenarioCanFail` executed 24 matching injected defects, so each
scenario has been watched going red for the specific thing it claims to detect.
`TestBehaviourDocumentListsEveryScenario` fails if the document and the suite
drift apart, in either direction. To run one:

    go test ./test/bdd/ -run 'TestBehaviour/the_firewall'

### The carriage suite

`test/tunnel` drives each of the seven schemes the parser accepts through a real
xray-core server on loopback. Each one has to deliver an HTTP request to an
origin only the far side of the tunnel can reach. See "What has carried bytes through a
real server" above for what each row covers and what it does not. Before this
package existed, the strongest statement held about six of those seven protocols
was that the engine would load the document they produce.

### On the target hardware

[`internal/netcfg/testdata/PROVENANCE.md`](https://github.com/Iman/caspian/blob/main/internal/netcfg/testdata/PROVENANCE.md) is the record, and it is careful about
the difference between what was captured and what was written. Filenames carry
the class: `capture-pi5-` is byte output of a real command on the Pi,
`scenario-` is a machine nobody has measured, and `golden-` is this project's
own output.

Measured on the Pi and recorded there: all five distinct generated rulesets
parsed by `nft -c -f`, with each file's sha256 read back on the Pi rather than
on the developer machine, and `nft list ruleset` empty before and after. The
interface release sequence and its inverses. The driver refusing to create a
second AP interface while accepting a type change on the existing one. The kill
switch's provocations with their negative controls. And the input-policy lockout
that caused that policy to be withdrawn.

That file also records what replacing authored bytes with measured ones broke,
which is the argument for keeping both kinds. Several defects had been green
through every prior run, including a firewall ruleset that no kernel would load
and a teardown that would have switched reverse-path filtering off on a machine
that had it on.

### End to end, with a real phone

The harness is [`test/hardware/caspian-hw`](https://github.com/Iman/caspian/blob/main/test/hardware/caspian-hw) and the runbook is
[`docs/HARDWARE-TEST.md`](https://github.com/Iman/caspian/blob/main/docs/HARDWARE-TEST.md). Its standard is the project's own. A connect is not a
result, and a transport is proven only when real traffic has traversed it and
the exit IP has been captured and matched to the server the config names. An
exit IP equal to the untunnelled baseline is a leak and outranks everything else
in the run. A phone that changed network state mid-capture makes the reading
void, not a pass and not a leak.

A run recorded on 2026-08-30, `run-20260830T144015Z`, graded, over IPv4:

- two configs proven, each `verdict PASS` with `sources agree` and an exit IP
  from both independent sources, matched to the box the config names.
- the exit fingerprint changing when the config was switched.
- the DNS check finding a per-run random `.invalid` label zero times in
  cleartext on the uplink during a 30 second window. Four plaintext DNS packets
  did cross that uplink in that window, and they are the box's own, which the
  design places outside the guarantee. That is exactly why the check looks for a
  label nothing else on the network could have produced, rather than counting
  port 53 packets, which cannot tell an escaped client query from the box's own.
- fail closed: with the engine stopped and the phone's cellular removed by
  airplane mode, neither source reached the internet, while the panel still
  answered over the hotspot. So this was the firewall refusing traffic rather
  than a dead link. Two earlier attempts at that step were graded VOID and
  retaken rather than reported.

Two things about that record. It reached a pass on the third attempt at the last
step, and the two void readings are in the ledger rather than deleted. And the
run artefacts live under `local/`, which is gitignored, so they are **not in
this repository**. If you clone this, you cannot check that run. You can only
re-run the harness yourself.

Two sources are used because one can be cached or stale, and both are pinned to
IP addresses rather than names. [`docs/HARDWARE-TEST.md`](https://github.com/Iman/caspian/blob/main/docs/HARDWARE-TEST.md) explains why in the
paragraph it calls the most important in the file. The resolver on that LAN
sinkholes IP-echo services. So a box that changed nothing but the DNS server,
and tunnelled no traffic at all, would have shown exactly the signature a
name-resolving harness looks for. It would have been graded a pass.

The harness redacts every config, server address, user id and key from
everything it writes. It re-reads each artefact to check the redaction held, and
it has a sweep that re-reads the whole run for anything that escaped the filter. No
packet capture ever leaves the Pi. The tcpdump output is reduced to two integers
on the box, because a capture on that uplink is a recording of the maintainer's
own browsing.

## Architecture and network diagrams

```mermaid
flowchart LR
    subgraph device["A device joined to the hotspot"]
        BR["Browser<br/>port 8088 on the hotspot address"]
    end

    subgraph panelproc["caspian serve --panel, runs as the caspian account"]
        PANEL["internal/panel<br/>routes, sessions, wording, rendering"]
        STATE["internal/state<br/>the only writer of state.json"]
        LINK1["internal/link<br/>parse the pasted share link"]
        ENG1["internal/engine<br/>Validate only, opens no socket"]
    end

    subgraph privproc["caspian serve --privileged, runs as root"]
        SVC["internal/privsvc<br/>Service.Start, Stop, Cut, Restore, Recover"]
        XCFG["internal/xcfg<br/>compose the engine document"]
        NETCFG["internal/netcfg<br/>routes, nftables, the teardown journal"]
        HOT["internal/hotspot<br/>hostapd and dnsmasq"]
        ENG2["internal/engine<br/>xray-core, in this process"]
    end

    BR --> PANEL
    PANEL --> STATE
    PANEL --> LINK1
    PANEL --> ENG1
    PANEL -->|"/run/caspian/priv.sock<br/>0660 root:caspian"| SVC
    SVC --> XCFG
    SVC --> NETCFG
    SVC --> HOT
    SVC --> ENG2
```

```mermaid
flowchart TB
    LINK["internal/link<br/>share link in, one outbound out.<br/>Carries no credential in an exported field"]
    XCFG["internal/xcfg<br/>everything around the outbound:<br/>TUN inbound, SOCKS, local DNS, routing"]
    ENGINE["internal/engine<br/>starts and stops xray-core.<br/>Redacts every line on the way in"]
    NETCFG["internal/netcfg<br/>plans the machine, generates the ruleset,<br/>journals the inverse of every change"]
    HOTSPOT["internal/hotspot<br/>renders and supervises hostapd and dnsmasq.<br/>Detects no interface, queries no radio"]
    STATE["internal/state<br/>state.json, atomically, 0600"]
    PANEL["internal/panel<br/>the web interface and the fault vocabulary"]
    PRIVSVC["internal/privsvc<br/>the order of the steps, and the readbacks"]

    PANEL --> LINK
    PANEL --> STATE
    PRIVSVC --> LINK
    PRIVSVC --> XCFG
    PRIVSVC --> NETCFG
    PRIVSVC --> HOTSPOT
    PRIVSVC --> ENGINE
    LINK --> XCFG
    XCFG --> ENGINE
```

```mermaid
flowchart TB
    subgraph panelowns["Written only by caspian serve --panel"]
        SJ["/var/lib/caspian/state.json<br/>0600 caspian. Holds the pasted config<br/>and the hotspot passphrase"]
    end

    subgraph privowns["Written only by caspian serve --privileged"]
        JN["/var/lib/caspian/netcfg.journal<br/>0600 root. The inverse of every change,<br/>written before the change"]
        HC["/run/caspian/hostapd.conf<br/>0600 root, tmpfs, rewritten every start"]
        DC["/run/caspian/dnsmasq.conf<br/>0600 root, tmpfs, rewritten every start"]
    end

    subgraph nofile["Held in memory and written to no file"]
        CUT["the cut"]
        EVT["the panel's event list"]
        RING["the engine log ring"]
    end
```

```mermaid
sequenceDiagram
    autonumber
    participant U as The person at the panel
    participant PA as internal/panel
    participant LK as internal/link
    participant EN as internal/engine
    participant PS as internal/privsvc, root
    participant NC as internal/netcfg
    participant HS as internal/hotspot

    U->>PA: POST /power, on=1
    PA->>LK: link.Parse of the stored text
    Note over LK: State 1. It did not parse.<br/>The user has to fix the text.
    LK-->>PA: a Link that holds no credential in any exported field
    PA->>LK: Link.XrayConfig
    LK-->>PA: one outbound, tagged proxy, nulls removed
    PA->>EN: engine.Validate
    Note over EN: State 2. Read, and unusable as written.<br/>No socket opens. Nothing is dialled.
    PA->>PS: StartRequest over priv.sock
    PS->>PS: clock floor, re-parse, validate against this machine
    PS->>NC: Detect, then PlanNetwork
    PS->>PS: xcfg.Build, then engine.Validate again
    PS->>NC: Apply PreEngineSteps. The firewall is first.
    PS->>NC: AssertHotspotInterfaceReleased
    PS->>EN: Engine.Start. The tunnel device appears here.
    PS->>NC: Apply PostEngineSteps. Each needs the tunnel or engine listener.
    PS->>HS: Supervisor.Start: hostapd, then dnsmasq
    PS->>NC: AssertHotspotIsAccessPoint
    PS->>PS: probe the server
    Note over PS: State 3. The link was fine and the<br/>server did not answer. No rollback:<br/>the box is fully configured and blocking.
    PS-->>PA: nil, or one panel.Fault
```

```mermaid
flowchart TB
    DEV["A joined device<br/>address from dnsmasq"] --> IF["The hotspot interface"]
    IF --> PRE["nft chain prerouting, type nat<br/>DNS on port 53 is redirected here"]
    PRE --> ROUTE{"Routing decision<br/>ip rule from the hotspot subnet<br/>lookup table 8410"}
    ROUTE -->|"tunnel route present"| TOTUN["oif is the tunnel device<br/>default route in table 8410"]
    ROUTE -->|"tunnel route withdrawn"| TOUP["oif is the uplink"]
    TOTUN --> FW1["nft chain forward, policy drop"]
    TOUP --> FW2["nft chain forward, policy drop"]
    FW1 -->|"iifname hotspot oifname tunnel<br/>ip saddr the hotspot subnet, accept"| POST["nft chain postrouting<br/>deliberately empty, no masquerade"]
    FW2 -->|"iifname hotspot oifname uplink, drop<br/>the leak block, first rule in the chain"| DROP["dropped"]
    POST --> TUN["The tunnel device<br/>a userspace netstack in the engine"]
    TUN --> OB["the outbound tagged proxy"]
    OB --> UP["The uplink<br/>a pinned host route to the server"]
    UP --> SRV["Your server"]
```

```mermaid
flowchart TB
    GONE["The tunnel stops carrying traffic"] --> Q{"Does the device still exist?"}
    Q -->|"device removed"| WD["The kernel withdraws every route through it"]
    WD --> FB["Client traffic falls back to the main table<br/>and heads for the uplink"]
    FB --> LB["The leak block matches: iifname hotspot oifname uplink, drop"]
    Q -->|"device persists with nothing servicing it"| ENTER["Traffic enters the tunnel device"]
    ENTER --> NOWHERE["Nothing reads it. It goes no further."]
    LB --> SAFE["No client traffic leaves"]
    NOWHERE --> SAFE
```

```mermaid
flowchart TB
    ASK["A joined device asks whatever resolver it was told to use,<br/>or one hardcoded into it, on port 53"]
    ASK --> RD["nft prerouting on the hotspot:<br/>udp dport 53 and tcp dport 53 redirect to :53<br/>The destination address is rewritten to this box"]
    RD --> DM["dnsmasq, bound to the hotspot interface<br/>/run/caspian/dnsmasq.conf"]
    DM -->|"its only permitted upstream is a loopback address"| LD["the engine's DNS listener<br/>127.0.0.1:5354, inbound tag local-dns-in"]
    LD --> R1["rule ruleTagLocalDNS<br/>inboundTag local-dns-in, outbound dns-out"]
    R1 --> APP["the engine's DNS app<br/>resolvers from internal/xcfg/resolvers.go"]
    APP --> R2["rule ruleTagResolvers<br/>inboundTag resolver-in, outbound proxy.<br/>Above the private-address rule"]
    R2 --> OB["the outbound tagged proxy"]
    OB --> EXIT["the resolver chain, reached from the far end of the tunnel"]
```

```mermaid
flowchart LR
    DOT["DNS over TLS<br/>tcp 853"] --> REJ["reject with tcp reset,<br/>so the device falls back to port 53"]
    DOQ["DNS over QUIC<br/>udp 853"] --> DRP["drop"]
    DOH["DNS over HTTPS<br/>port 443"] --> CAR["carried through the tunnel like any HTTPS.<br/>Not a leak. Not visible to anything here."]
```

```mermaid
flowchart LR
    subgraph modea["Mode A, the measured one"]
        A1["Ethernet<br/>brings the internet in"] --- A2["Built-in WiFi<br/>becomes the hotspot"]
    end
    subgraph modeb["Mode B, never run on real hardware"]
        B1["Built-in WiFi<br/>brings the internet in"] --- B2["USB adapter reporting AP support<br/>becomes the hotspot"]
    end
```

```mermaid
stateDiagram-v2
    [*] --> Off
    Off --> Running: POST /power on=1
    Running --> Off: POST /power on=0
    Running --> TrafficCut: POST /cut cut=1
    TrafficCut --> Running: POST /cut cut=0
    Running --> Running: POST /recover
    TrafficCut --> Off: the machine restarts
    Running --> Off: the machine restarts

    note right of Off
        The privileged service replays the
        journal at startup and starts nothing.
    end note
    note right of TrafficCut
        Hotspot up. Devices joined.
        Panel reachable. Nothing forwarded.
    end note
```


## Licence

AGPL-3.0-or-later. [LICENSE](LICENSE) | [NOTICE](NOTICE) | [English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) | [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) | [中文](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh)
