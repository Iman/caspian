# macOS and Windows ports: what is built, what is measured, what is not

[🇮🇷 فارسی](PORTS.fa.md) | 🇬🇧 **English** | [🇷🇺 Русский](../README.ru.md) | [🇨🇳 中文](../README.zh.md)

> Persian edition: [`docs/PORTS.fa.md`](PORTS.fa.md). The English file is the
> one the tests read. If the two ever disagree, this one is correct.

Branch `port/platforms`, started 2026-09-03. This file is the honest state of
the two desktop ports. Everything marked MEASURED was run on a machine;
everything marked VERIFIED was read in a primary source (Apple or Microsoft
documentation, the operating system's own binaries, or the source of the
libraries named); UNVERIFIED means neither, and nothing in this project is
allowed to rest on an UNVERIFIED claim without saying so.

The Linux appliance is unchanged. Every command it runs, every rule it
generates and every test it had still runs byte for byte; the ports were made
by putting a seam under it, not by editing it.

## The seams

| Seam | Where | What differs per platform |
|---|---|---|
| Network backend | `internal/netcfg/platform.go`, `Backend` | how the machine is read, what a plan turns into, how it is read back. Chosen by `Options.Platform`, a value, so all three compile and are tested everywhere |
| Runner | `exec_linux.go`, `exec_darwin.go`, `exec_windows.go` | the only build-tagged code: what actually executes. Linux and macOS exec fixed-path binaries from a closed allowlist; Windows calls the IP Helper API, WFP and Wintun in process, as pseudo-binaries so the journal and idempotence rules are shared |
| Access point | `internal/hotspot/accesspoint.go`, `AccessPoint` | hostapd and dnsmasq under the Supervisor (Linux), Apple's Internet Sharing (`internetsharing.go`), Mobile Hotspot through a helper (`mobilehotspot.go`) |
| Transport | `internal/privsvc/transport_unix.go`, `transport_windows.go` | unix socket with kernel peer credentials, or a named pipe with a security descriptor and impersonation |
| Layout | `cmd/caspian/paths_*.go` | paths, the service account, the service manager's verbs |
| Privilege and lifecycle | `cmd/caspian/privilege_*.go`, `lifecycle_*.go` | euid 0 versus an elevated token; signals versus the Service Control Manager |

## macOS

Decided with the owner on 2026-09-03: the access point is the Mac's own
radio through Apple's Internet Sharing, and the internet comes in on a wired
interface (Ethernet, USB Ethernet, iPhone USB). A USB Wi-Fi adapter cannot be
the access point on Apple Silicon: no vendor ships a macOS 11 or later driver
for Realtek or MediaTek USB radios, DriverKit has no Wi-Fi family, macOS has no
API that puts a radio into access point mode, and hostapd has no Darwin
backend (all VERIFIED; sources in the port research). The same radio cannot be
station and access point at once, so a Mac whose only internet is its Wi-Fi
cannot host; the planner says so in words.

MEASURED on the development Mac (macOS 26.6, Apple Silicon), read-only:

- `caspian check` runs the macOS detection end to end and the planner refuses
  correctly when the Wi-Fi radio is the uplink.
- Internet Sharing is a configd plugin (`InternetSharingPreference.bundle`)
  watching `com.apple.nat.plist` through SCPreferences, plus the XPC daemon
  `com.apple.NetworkSharing`; there is no `com.apple.InternetSharing` launchd
  job any more. bootpd does DHCP; mDNSResponder's proxy answers client DNS.

Built and unit-tested (with recorders, no root):

- `darwinnet_detect.go`: `ifconfig -a`, `route -n get default`,
  `networksetup -listallhardwareports` and `-getairportnetwork`,
  `sysctl -e`, `pfctl -s info`, into the shared `Facts`.
- `darwinnet_steps.go`: one pf anchor `com.apple/250.CaspianBYOC` (evaluated
  by `/etc/pf.conf`'s `anchor "com.apple/*"` without editing the main
  ruleset), loaded as one transaction. It redirects client DNS to the engine,
  steers client traffic into the utun with `route-to`, blocks everything else
  from clients inbound on `bridge100`, and passes the panel. Pinned server
  route with `route -n add -host`. `net.inet.ip.forwarding` on. No tunnel
  address step: xray-core's darwin TUN assigns 169.254.10.2/30 itself and
  insists the device is `utunN`, so the device is `utun100`.
- `internetsharing.go`: writes the preferences file with the keys real dumps
  and the plugin's strings show (NetworkName, NetworkPassword as UTF-16LE
  data, Channel, PrimaryService as the uplink's service UUID, SharingDevices,
  SharingNetworkNumberStart/End/Mask from the plan's subnet), re-saves it
  through `scutil --prefs` so configd gets the commit and apply notifications,
  kickstarts the daemon once if the bridge does not appear, and reads the
  bridge back. Device count from `/var/db/dhcpd_leases`.

UNVERIFIED until run with root on a Mac, in this order (script:
`local/measure-internet-sharing.sh`, gitignored, redacts before saving):

1. That the Sharing pane on macOS 26 writes the keys above in that form, and
   where it keeps the WPA passphrase (the plist, or a System keychain item
   through CoreWLAN's private HostAP API). If the keychain, the driver needs
   a `security add-generic-password` step with the attributes measured.
2. That the `scutil --prefs` commit and apply start sharing on 26.6, or that
   `launchctl kickstart -k system/com.apple.NetworkSharing` does.
3. That pf `route-to` onto a utun works on Apple's pf (xnu's `pf_route` says
   it does; nobody has published a run), and that an `rdr` in a `com.apple/*`
   child is honoured for `bridge100` traffic.
4. That Internet Sharing does not tear itself down when it sees the utun.

Exercising it on this Mac needs an Ethernet uplink (a cable in one of the USB
Ethernet adapters), `sudo`, and `bash packaging/darwin/install-darwin.sh`.

## Windows

Decided with the owner on 2026-09-03: Mobile Hotspot is the access point
(driven by the C# helper `tools/caspian-tethering`), the whole host is
tunnelled because Windows has no per-source routing, and fail-closed is
enforced by Windows Filtering Platform filters at the IP forwarding layer,
because ordinary firewall rules never see forwarded traffic.

Built here and compile-checked for `windows/amd64` and `windows/arm64`; the
C# helper compiles against the real WinRT projection on this Mac:

- `winnet.go`: the backend. Pseudo-binaries `iphlpapi`, `wfp`, `wintun`.
  Pre-engine: create the tunnel adapter (so the filters can name it), load the
  filters, forwarding on, pinned host route to the server. Post-engine:
  address, interface metric 0, default route through the tunnel. The hotspot
  subnet is pinned to 192.168.137.0/24, which is what Internet Connection
  Sharing serves and cannot be told otherwise.
- `exec_windows.go` and `winsys_windows.go`: the runner. Structures copied
  field for field from WireGuard's MIT `winipcfg` and `firewall` packages;
  WFP layer and condition GUIDs from Microsoft's headers by way of
  tailscale/wf. Filters are persistent (they outlive the process, so a crash
  leaves the box closed) under a provider and sublayer with fixed keys.
- `mobilehotspot.go` and the helper: one process per action, JSON in, one
  JSON line out. The helper checks the tethering capability and reports the
  Windows reason by name, applies SSID, passphrase and band, disables the
  five-minute no-clients timeout, starts, and reads the state back.
- `transport_windows.go`: named pipe `\\.\pipe\caspian-priv`, security
  descriptor admitting SYSTEM, Administrators and the panel's virtual
  service account; the client's SID read through impersonation.
- `packaging/windows/install.ps1`: two services, ACLs, the first-run
  password.

To finish on the Windows machine, in this order:

1. Build: `go build -o caspian.exe ./cmd/caspian` and
   `dotnet publish -c Release -r win-x64 -o out tools/caspian-tethering`;
   download `wintun.dll` from wintun.net (do not rename it).
2. `caspian.exe check` from an elevated prompt: the inventory, the plan.
3. The helper by hand: `echo {"op":"status","uplink":"Ethernet"} |
   caspian-tethering.exe status`, then a start with the dongle named as the
   adapter. This settles whether the `(profile, NetworkAdapter)` overload
   pins the dongle and whether a no-internet Wintun profile is accepted.
4. Whether ICS-translated client packets follow the host routing table into
   the tunnel (they should, with the default route on it) or are forced out
   the shared adapter. If forced, the alternative is to share FROM the Wintun
   adapter's profile, which the helper can be told to do.
5. Whether the WFP forwarding filters see the client packets (before or
   after ICS translation) and drop them when the tunnel is gone.
6. Client DNS: clients are told 192.168.137.1 and the ICS proxy answers with
   the host's resolvers, which now go through the tunnel; confirm with a
   capture on the uplink that no port 53 leaves in the clear.

## Not done on any platform yet

- xray-core is v26.4.15 since 2026-09-03. Its `Handler.Close` makes the engine
  release its TUN device on every platform; the Linux-only release path in
  `internal/engine/tundevice_linux.go` is kept as a measured safety net until it
  is re-measured on the appliance as redundant.
- `docs/LAYOUT.md` documents the Linux table only. `cmd/caspian/paths_*.go`
  hold the macOS and Windows tables; the document should gain both.
- `install.sh` still refuses anything that is not Linux, by design; the
  platform installers are under `packaging/darwin` and `packaging/windows`.
- On Windows the 0600 protection `internal/state` promises is not in force
  (`perm_other.go` says so); the installer's ACLs stand in for it.
