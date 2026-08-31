# Provenance of these fixtures, file by file

Read this before treating any green test in this package as evidence about
hardware. Every file here is in exactly one of the classes below, and the class
decides what a passing test proves. The filename carries the class, so a test
that reads `capture-pi5-*` is making a claim about the target and a test that
reads `scenario-*` is not.

Both sets are kept deliberately. Each catches what the other cannot, and the
history below is the argument for keeping both rather than a preference.

| Class | Prefix | What it is |
|---|---|---|
| CAPTURED | `capture-pi5-` | Byte output of a real command on the target Pi |
| DERIVED  | `derived-pi5-` | Retired 2026-08-30. Briefly un-retired the same day for one file, which was then captured instead and the class re-retired. No file is in this class. The prefix stays reserved so a reappearance is visible |
| AUTHORED | `scenario-` | A machine nobody has measured, written to documented output shapes |
| GENERATED | `golden-` | Output of this package, rewritten by `go test -run Golden -update` |

## PRIVACY: these are real captures with identifying values substituted, 2026-08-31

Read this before the class table above is taken at its word. A `capture-pi5-`
file is the bytes a real kernel printed, EXCEPT that a fixed set of values that
identified a person, a home or a device has been replaced. Nothing else in them
was touched: no field was added, removed, reordered or reformatted, and the
parse shape a test reads is the shape the kernel produced.

**Why.** The reference Raspberry Pi was tested on the maintainer's home
network, and the readings were committed. A router BSSID is the serious one:
public WiFi geolocation services index router MAC addresses, so that value
alone places a home within metres, and with the network name beside it there is
no ambiguity left about which house. This repository is published.

**What this does and does not change.** It does not change what a green test
proves about the kernel: a MAC is an opaque token to every parser here, and an
SSID is a string. It DOES change the provenance, which is why it is recorded
rather than done quietly. The correct reading of these files is now "a real
command's output, with the values in the table below replaced", and NOT "a file
somebody wrote to look like a capture". If a future reader concludes the second
of those, this section has failed.

**The substitute table. Use these, do not invent a third set.** Several tests
turn on two interfaces having DIFFERENT addresses, so a substitution that
collapsed two values into one would pass a green suite while removing the thing
under test. The whole MAC block is `02:00:5e:`, which has the
locally-administered bit set and therefore cannot collide with any
manufacturer's assignment.

| What it was | Substitute | Where it appears |
|---|---|---|
| The home wifi network name | `HomeNet` | `iw dev`, `iw link`, `nmcli` captures, and the assertions that read them |
| The home router BSSID | `02:00:5e:00:00:01` | `capture-pi5-iw-link-connected.txt` and the link-state assertions |
| The Pi's ethernet MAC | `02:00:5e:00:00:10` | `capture-pi5-ip-d-link.txt` |
| The Pi's built-in wifi MAC, `wlan0` | `02:00:5e:00:00:11` | the `iw dev` and `ip -d link` captures |
| The Pi's USB dongle wifi MAC, `wlan1` | `02:00:5e:00:00:12` | the two-radio and dongle-only captures |
| The Pi's `P2P-device` address on `phy0` | `02:00:5e:00:00:13` | `capture-pi5-iw-dev.txt` |
| Any further real Pi address | `02:00:5e:00:00:14` upward, one per distinct value | |
| The Pi's hostname | `caspian-box` | this file, and the `parseFrequency` comment in `parse.go` |
| The Pi's `/etc/machine-id` | removed outright, not substituted | the host table below |
| A proxy server address | `198.51.100.10` | RFC 5737 TEST-NET-2 |
| The household's public address, as the exit-IP echo endpoints measured it | `203.0.113.20` | RFC 5737 TEST-NET-3, in `test/hardware/lib/exitip.sh` |
| The handset's adb serial | `PHONE-SERIAL` | `test/hardware/lib/phone.sh` |
| The handset's model and product code | `SM-X000F`, `SM_X000F`, `x000nnxx` | same shape as what they replace |

The AUTHORED `scenario-` fixtures and the hand-written lease fixtures in
`internal/hotspot/testdata` were never anybody's readings, but they DID carry
real manufacturer OUIs, which is indistinguishable from a real device to any
reader and to any scanner. They were moved into the same block: `02:00:5e:01:`
for the authored machines' interfaces and `02:00:5e:02:` for the invented DHCP
clients. Doing that rather than allowlisting them is what lets the repository
gate hold NO exception for a MAC address anywhere, so a real reading pasted
into a lease fixture still fails.

**What was deliberately NOT changed, so the decision is recorded rather than
silent.** RFC 1918 addresses stay as they were measured: `10.0.0.221`,
`10.0.0.1`, `192.168.1.57`, the hotspot's own `10.83.51.1` and
the `192.168.66.0/24` lease range. Once the names and the MACs are gone these
identify nobody, millions of households use the same numbers, and rewriting
them would churn a large number of assertions for no privacy gain. The device
hostnames in the lease fixtures (`iPhone`, `pixel-8`, `kitchen-printer` and the
rest) are hand-written and are recorded as such in
`internal/hotspot/testdata/PROVENANCE.md`.


One RFC 1918 address is treated differently, and the difference is worth
stating so nobody restores it as an oversight. The address of the resolver
this LAN hands out was removed on 2026-08-31. A host address says a machine
existed on a network millions of homes share; the resolver address, together
with the recorded fact that it sinkholes IP-echo services, describes a
particular household's DNS setup. In all three places it appeared, the
argument was about what the resolver DOES and never about its number, so the
number was load-bearing nowhere and cost something everywhere.

The privacy scan cannot catch this class. It treats all of 10.0.0.0/8 as
identifying nobody, which is correct for a server address and blind to the
resolver a particular home hands out.

**The gate.** `test/goldenscan` fails the build on any MAC outside the
`02:00:5e:` block, on any manufacturer-assigned MAC anywhere in the repository,
on any routable IPv4 outside the documentation ranges, and on the specific
values removed here, which it pins as digests so that the registry does not
itself hold them. Read `test/goldenscan/privacy.go` before adding an exception:
there is no allowlist entry for a MAC address in it, on purpose.

**The limit.** All of this is the working tree. A value that reached a commit
is permanent, and substituting removes it from every future clone's checkout
and from nothing in history.

## CAPTURED: byte captures from the target, 2026-08-30

    capture-pi5-ip-route-default.txt    ip route show default
    capture-pi5-ip-route6-default.txt   ip -6 route show default   (empty: the box has none)
    capture-pi5-ip-br-addr.txt          ip -br addr
    capture-pi5-ip-d-link.txt           ip -d link show
    capture-pi5-iw-dev.txt              iw dev
    capture-pi5-iw-list.txt             iw list
    capture-pi5-sysctl-n-flag.txt       sysctl -n -e -- <four knobs>
    capture-pi5-sysctl-base.txt         sysctl -e -- <the four base knobs>
    capture-pi5-sysctl-absent-interfaces.txt
                                        sysctl -e -- <eight knobs, three of
                                        them on interfaces that did not exist>

Host and toolchain, read on the same box in the same session:

    host           caspian-box (Raspberry Pi 5 Model B Rev 1.0)
    machine-id     <removed: a globally unique identifier for one physical box>
    captured       2026-08-30 00:48 BST (2026-08-29T23:48Z)
    uname -a       Linux caspian-box 6.18.34+rpt-rpi-2712 #1 SMP PREEMPT
                   Debian 1:6.18.34-1+rpt1 (2026-06-09) aarch64 GNU/Linux
    os-release     Debian GNU/Linux 13 (trixie), DEBIAN_VERSION_FULL 13.5
    ip -V          ip utility, iproute2-6.15.0, libbpf 1.5.0
    iw --version   iw version 6.9
    nft --version  nftables v1.1.3 (Commodore Bullmoose #4)
    wifi driver    brcmfmac on phy0, SDIO bus (mmc1)

Each command was run through `env -i` with the absolute path under `/sbin`,
which is what the package's own runner does: `searchPath` in `exec_linux.go`
puts `/sbin` first and `systemRunner.Run` sets `cmd.Env = []string{}`. Output
captured in the C locale for the same reason.

`capture-pi5-sysctl-n-flag.txt` is kept although the command that produced it
is no longer used. It is the evidence for a fixed defect: `-n` prints values
with no names, so the four bare lines it holds parse to an empty map. The test
`TestParseSysctl_BareValuesYieldNothing` reads it, so the defect stays
reproducible instead of becoming a story about a defect.

## The reference box has no global IPv6, so the v6 server path is UNPROVABLE here

Measured on the target on 2026-08-30, after the empty
`capture-pi5-ip-route6-default.txt` above raised the question:

    ip -6 route show default    (no output)
    ip -6 -br addr show eth0    fe80::2ecf:67ff:fe72:51f6/64

Link-local and nothing else. The network this box sits on hands it no global
IPv6 address and no v6 default route.

WHAT THAT MAKES UNPROVABLE, stated as a standing UNPROVEN rather than as an
outstanding task, because no amount of work in this repository closes it. The
IPv6 branch of `Plan.hostRouteArgs`, which emits
`ip -6 route add <server>/128 via <gw> dev <if>` for a server reachable only
over IPv6, cannot be exercised on this hardware. `Plan.canPin` correctly
declines to pin when the family has no default route, and
`PlanNetwork` refuses outright when every server address is v6 and no v6 route
exists, so the REFUSAL paths are covered by
`TestPlanNetwork_RefusesAnIPv6OnlyServerWithNoIPv6Route`. The SUCCESS path is
covered only by the authored fixture `scenario-modea-ip-route6-default.txt`,
which is a machine nobody has measured.

TO CLOSE IT: a v6-capable network, not more code. Until then, anything written
about that path is a claim about a fixture.

## The measured IPv6 sysctls, 2026-08-30, and the general claim they contradict

    net.ipv6.conf.all.forwarding = 0
    net.ipv6.conf.eth0.accept_ra = 0

Recorded because the second value is not what the kernel documents as the
default, and a comment asserting the documented behaviour would be wrong about
this machine.

`Documentation/networking/ip-sysctl.txt` says of `accept_ra`: "Functional
default: enabled if local forwarding is disabled. disabled if local forwarding
is enabled." Here forwarding is 0 and `accept_ra` is 0 anyway, so something on
this box sets it explicitly and the functional default is not in force.

THE CONSEQUENCE, and it is the whole reason this is written down. The general
claim "turning on IPv6 forwarding costs the box its own SLAAC address and v6
default route" is true in general and FALSE HERE, because there is nothing to
lose: see the section above, this box has no global v6 address to begin with.
So the cost of `netcfg.IPv6Forward` cannot be observed on this hardware either.
Anyone pricing that trade-off needs a box with working IPv6, and a comment
stating the general rule as though it described this machine would be another
confident sentence that does not match the thing it is written about.

## The sysctl promotion, 2026-08-30

`capture-pi5-sysctl-base.txt` and `capture-pi5-sysctl-absent-interfaces.txt`
were `derived-pi5-sysctl-base.txt` and `derived-pi5-sysctl-full.txt` until this
date. They were DERIVED because the VALUES were measured and the FORMAT was
not, and because the per-interface values were assumed equal to `conf.all`,
which the file said in plain words was a guess. Both files now hold the bytes
the command printed.

The commands, run on the target with the `env -i` and absolute-path discipline
the other captures used, because that is what `systemRunner.Run` gives a
command (`cmd.Env = []string{}`, `searchPath` in `exec_linux.go` putting
`/sbin` first):

    env -i /sbin/sysctl -e -- net.ipv4.ip_forward net.ipv4.conf.all.rp_filter \
      net.ipv4.conf.default.rp_filter net.ipv6.conf.all.forwarding \
      > capture-pi5-sysctl-base.txt

    env -i /sbin/sysctl -e -- net.ipv4.ip_forward net.ipv4.conf.all.rp_filter \
      net.ipv4.conf.default.rp_filter net.ipv6.conf.all.forwarding \
      net.ipv4.conf.eth0.rp_filter net.ipv4.conf.ap0.rp_filter \
      net.ipv4.conf.xray0.rp_filter net.ipv6.conf.ap0.disable_ipv6 \
      > capture-pi5-sysctl-absent-interfaces.txt

Host and toolchain, read on the same box in the same session:

    host           caspian-box (Raspberry Pi 5 Model B Rev 1.0), 10.0.0.221
    captured       2026-08-30 01:14 BST (2026-08-30T00:14Z)
    uname -a       Linux caspian-box 6.18.34+rpt-rpi-2712 #1 SMP PREEMPT
                   Debian 1:6.18.34-1+rpt1 (2026-06-09) aarch64 GNU/Linux
    sysctl         sysctl from procps-ng 4.0.4 (Debian procps 2:4.0.4-9, arm64)
    nft --version  nftables v1.1.3 (Commodore Bullmoose #4)
    systemd        257 (257.13-1~deb13u1)
    sha256         base 5f4d968266f4108fd13df61534f42992a134d46ff151d933b66939769b71f1b9
                   full c65fe01b6b886e63eb46b89bb7326e1dd15a3d297c0daa46358d47154a3338a2

Identical bytes as root and as the unprivileged account, checked by sha256 on
the box, so nothing here depends on who ran it.

What the real bytes changed. Both are the guess named above coming apart, and
neither was visible while the fixtures were derived:

- `net.ipv4.conf.eth0.rp_filter` is **2**, not the 0 the guess carried. What
  was measured is the three values: `conf.all` 0, `conf.default` 2, `conf.eth0`
  2. So eth0 tracks `conf.default` and not `conf.all`, and the guess named the
  wrong one of the two. The mechanism is not measured here; `SysctlSteps` in
  `route.go` already states it, that `conf.default` is inherited by an
  interface at the moment it is created. The recorded inverse in
  `golden-commands-captured.txt` was
  `sysctl -w net.ipv4.conf.eth0.rp_filter=0`: an uninstall would have DISABLED
  reverse-path filtering on the uplink of a box that had it on loose, which is
  a state the box was never in and a weaker posture than it was found in. The
  golden now reads `=2`.
- Three knobs come back with NO LINE AT ALL: `net.ipv4.conf.ap0.rp_filter`,
  `net.ipv4.conf.xray0.rp_filter` and `net.ipv6.conf.ap0.disable_ipv6`. Eight
  knobs asked, five lines returned, exit 0, empty stderr. `Detect` passes `-e`
  precisely so a knob this kernel does not have is skipped rather than failing
  the whole read, and the per-interface read happens BEFORE the hotspot and
  tunnel devices are created, so `/proc/sys/net/ipv4/conf/ap0` and
  `.../xray0` do not exist yet. Measured on the box: `/proc/sys/net/ipv4/conf`
  holds `all default eth0 lo wlan0` and nothing else.

The second one is why the test double in `helpers_test.go` changed. It required
the fixture's knobs to EQUAL the knobs asked, which is the same assumption in a
different place, and it made every test using the captured machine fail at the
double rather than at the assertion. It now requires an ordered SUBSET, which
is the relation `-e` actually produces, and still fails loudly if a fixture
answers with a knob nobody asked for or answers a read it does not model.

Three assertions are left RED on purpose and are not this file's to settle:
`TestCaptured_TeardownRestoresTheMeasuredSysctlValues`,
`TestDetectAndPlan_EveryChangedKnobHasAMeasuredValue` and
`TestPlan_InvariantsHoldOnEveryModelledMachine` each require every changed knob
to carry a measured inverse, which the captured machine cannot give for an
interface that does not exist when the read happens. Weakening them without a
decision would put the assumption back.

## AUTHORED: machines nobody has measured

Nothing in any session verified these bytes. They must not be read as evidence
about any box. Each exists because the captured machine cannot produce the
shape, and the note says which shape.

    scenario-modea-ip-route-default.txt   a 192.168.1.0/24 uplink, so a hotspot
                                          subnet collision can be tested
                                          against the range domestic routers
                                          actually hand out
    scenario-modea-ip-route6-default.txt  an IPv6 default route. The target has
                                          none, so this is the only coverage of
                                          pinning a host route for an IPv6
                                          server
    scenario-modea-ip-br-addr.txt         addresses matching the above
    scenario-modea-ip-d-link.txt          same-line parentbus, platform and sdio
    scenario-modea-iw-dev.txt             the Unnamed/non-netdev stanza placed
                                          AFTER the interface. The capture has
                                          it before, which is the ordering that
                                          happens not to trigger the bug
    scenario-modea-iw-list.txt            frequencies in the older INTEGER form
                                          ("2412 MHz"). iw 6.9 prints decimals;
                                          this is the only coverage of the form
                                          earlier iw prints
    scenario-modeb-ip-route-default.txt   a wireless uplink
    scenario-modeb-ip-br-addr.txt         a second radio with no address
    scenario-modeb-ip-d-link.txt          a USB adapter: "parentbus usb". The
                                          target has no USB adapter, so this is
                                          the only USB bus sample
    scenario-modeb-iw-dev.txt             two radios
    scenario-modeb-iw-list.txt            a second AP-capable radio
    scenario-iw-list-usb-noap.txt         a second radio without AP support
    scenario-iw-list-noap-only.txt        no AP support anywhere
    scenario-iw-list-no-concurrency.txt   AP supported, but never beside a
                                          station
    scenario-ip-route-default-linkdown.txt  an unplugged cable
    scenario-ip-route-default-none.txt      no internet connection
    scenario-ip-route-default-onlink.txt    a point-to-point uplink, no gateway
    scenario-modea-sysctl-base.txt        knob values for the authored machine
    scenario-modeb-sysctl-base.txt        knob values for the authored machine

Measured on the target on 2026-08-30: exactly one radio, `phy0`, listed in
`/sys/class/ieee80211`; `lsusb` shows four root hubs and no attached device. So
mode B, which needs a second adapter, is proven against authored bytes only.

## GENERATED

    golden-ruleset-captured.nft     the ruleset for the measured machine
    golden-commands-captured.txt    the command sequence for the measured machine
    golden-ruleset-mode-a.nft       the ruleset for the authored mode A machine
    golden-ruleset-mode-b.nft       the ruleset for the authored mode B machine
    golden-commands-mode-a.txt      the command sequence for authored mode A

Rewrite with `go test ./internal/netcfg -run Golden -update`. They exist so a
change to generated output is a reviewable diff rather than a green test.

These have been checked against a real nft. Do not re-run them as an open
action; re-run them when the generator changes.

Checked on the target on 2026-08-30, nftables v1.1.3, with the file copied to
`/tmp` and `sudo nft -c -f` run against it. All three parsed: no output, exit
0, for `golden-ruleset-captured.nft`, `golden-ruleset-mode-a.nft` and
`golden-ruleset-mode-b.nft`. `sudo nft list ruleset` was empty before and after,
so nothing was loaded; `-c` checks and does not commit. The copies were removed.

One thing that check does NOT buy, and it is worth writing down before somebody
counts it twice: `golden-ruleset-captured.nft` and `golden-ruleset-mode-a.nft`
are BYTE-IDENTICAL, sha256
b1fc7570efe591169ee5025498bb009bb53f3392cddd6af9e9fb05306c8e89a2, verified with
`shasum -a 256` on 2026-08-30. Mode B is a genuinely different sample, sha256
106cf8e5e9e171a200adf34c6014e736fa18ed6ff3068278c3694f1ac5216763.

So three exit-zeros are TWO independent confirmations, not three. Checking
mode A added no information over checking the captured file.

Why they are identical, stated precisely because a slightly wrong mechanism is
how the last round's worst defect got in. `Plan.Ruleset` reads exactly ten
things: `Uplink`, `Hotspot`, `Tun` and `HotspotSubnet` from the plan, and
`DNSPort`, `PanelPort`, `UplinkInputTCP`, `IPv6`, `ClientIsolation` and
`MasqueradeToTunnel` from the options. The two machines agree on all four plan
fields (`eth0`, `ap0`, `xray0`, `10.83.51.0/24`) and both goldens are generated
with `DefaultOptions`, so all ten inputs match. Everything the two machines
differ in, the gateway and the knob values, reaches the command sequence and
never the ruleset: no gateway address appears in any ruleset golden.

`TestGolden_CapturedAndModeARulesetsAreOneSample` asserts this, so if a fixture
change ever makes them diverge the test says which of the ten inputs moved, and
this paragraph has to be rewritten rather than quietly becoming false.

## What the real captures changed, and what that argues

Replacing authored bytes with measured ones turned the suite red and exposed
defects that had been green through every prior run:

- `parseFrequency` could not read `2412.0 MHz`. iw 6.9 prints one decimal
  place; the authored fixture used the integer form. Every frequency was
  dropped, so `Phy.UsableChannels` returned nothing on a radio with seventeen
  usable channels. FIXED.
- `Detect` read knobs with `sysctl -n`, which prints values without names.
  `ParseSysctl` needs the names, so it returned an empty map, and every sysctl
  step recorded no inverse: uninstall would have left `ip_forward` and
  `rp_filter` changed. FIXED. The test double had been formatting
  `name = value` from a Go map, so it agreed with the parser by construction
  and no test could ever have seen it. The double now returns fixture bytes.
- The two DHCP client rules chained two header fields without repeating the
  protocol keyword. `nft -c -f` rejected them with "No symbol type
  information", which fails the WHOLE ruleset, so the box would have had no
  firewall at all. FIXED.
- `ParseIwDev` attributed an `Unnamed/non-netdev interface` stanza's fields to
  the previously parsed interface. It read correctly on the capture only
  because that stanza comes first. FIXED, with an authored fixture that puts it
  second.
- `ip -d link show` prints `parentbus` at the end of the `link/ether` line, not
  on a line of its own, and the built-in radio is on the SDIO bus rather than
  platform.
- The target has no IPv6 default route. A server address that can get no host
  route is now recorded on the plan as `UnpinnableServers`, and a plan whose
  server addresses are ALL unreachable is refused in plain words.

## The per-interface knobs, and why no fixture models them any more

`capture-pi5-sysctl-absent-interfaces.txt` is the output of a read this code no
longer makes. It is kept as the evidence for why it stopped making it: eight
knobs were asked for and five came back, because `ap0` and `xray0` did not
exist when the read ran and `sysctl -e` skips what it cannot read.

Three separate defects followed from changing a knob that names an interface,
and all three were visible in this one capture:

  - No measured value, so no inverse, so teardown could not put it back.
  - The write for `conf.ap0.rp_filter` was ordered BEFORE the step that creates
    `ap0`. `sysctl -w` on a missing knob fails, the step carries no `-e`, and
    `Applier.Apply` stops at the first failure, so the appliance would not have
    started on its first run.
  - `conf.eth0.rp_filter` reads 2 on the box while `conf.all.rp_filter` reads 0.
    The derived fixture had guessed the interface value from the global one and
    got 0, so the generated teardown would have written 0 to the uplink and
    turned reverse-path filtering OFF on a machine that had it on.

The plan now changes four global knobs and nothing else. That costs no
guarantee: the kernel uses the maximum of `conf/{all,interface}/rp_filter` and
loose (2) is numerically larger than strict (1), so `conf.all = 2` pins every
interface to loose by itself. The citation is in the comment above
`SysctlSteps` in `route.go`.

The `eth0` line is kept in the fixture deliberately. It is the evidence that a
per-interface value can differ from the global one, and therefore the reason to
distrust the next guess of that shape.

`scenario-modea-sysctl-full.txt` and `scenario-modeb-sysctl-full.txt` were
deleted on 2026-08-30. They modelled the second read, which no longer happens.

## The two interface-state captures, and why both exist

Taken on the target on 2026-08-30, same vantage as every other capture on this
page: Raspberry Pi 5, kernel 6.18.34, brcmfmac on phy0, nftables 1.1.3, run
through `env -i` with an absolute path in the C locale.

`capture-pi5-iw-info-ap-serving.txt` is `iw dev wlan0 info` with a real hostapd
beaconing: an access point named `Caspian-Probe` on channel 6.

    Interface wlan0
            ssid Caspian-Probe
            type AP
            channel 6 (2437 MHz)
            txpower 31.00 dBm

`capture-pi5-iw-info-freed-ap.txt` is the same command on an interface this
package's own sequence had just produced: released from NetworkManager, its
station address stripped, brought down, and typed with
`iw dev wlan0 set type __ap`, with nothing serving it.

    Interface wlan0
            type AP
            channel 10 (2457 MHz), width: 20 MHz, center1: 2457 MHz

The difference between those two files is the whole reason they exist, and it
is a defect this package shipped. Note what the second one reports: type AP, NO
ssid, and channel 10. Ten is the channel the STATION link was using before the
release. The driver keeps reporting it and it is stale, because no access point
is on channel 10 or on any channel: nothing is serving.

A predicate here read "an ssid is set OR a channel is reported" as evidence
that an interface was in use. Against the second file it answered TRUE, so the
readback that proves a release worked would have refused a correctly released
interface on this hardware every time and the appliance would never have
started. Association is a property of a station; asking it of an access point
is a category error. See `WirelessIface.IsAccessPoint`, `StationLink` and
`InUse` in facts.go, which now say which question each answers.

A third state is NOT captured and is named in the code as uncaptured: a station
reporting a channel with no ssid yet, an association in progress. Nothing here
establishes whether that occurs on this driver. The channel clause is kept on
the station side because refusing to disturb an interface that might be
mid-association is the safe direction and costs nothing.

## The third fixture was captured, and the derivation it replaced was wrong

`capture-pi5-iw-dev-freed-ap.txt` is `iw dev`, the whole command's output, on
wlan0 released from NetworkManager, address stripped, typed `__ap`, link up,
nothing serving it. Same vantage as the rest: Raspberry Pi 5 caspian-box, kernel
6.18.34+rpt-rpi-2712 aarch64, iw 6.9, brcmfmac on phy0, 2026-08-30. The
interface was restored afterwards and verified back on the house network as
managed with its own address.

It exists because the code reads `iw dev`, not `iw dev wlan0 info`.
`readWireless` in verify.go runs `iw dev` deliberately: it is the command this
package has real captures of, and using it means one parser rather than two.
The two `info` captures above record the same states in the other format and
are what the state fields were checked against.

### The correction, because a derivation stood here for a few minutes

This file replaced `derived-pi5-iw-dev-freed-ap.txt`, which spliced the state
lines from the `info` capture into framing taken from the managed-state
`iw dev` capture. The note beside it said "nothing in the file is invented: it
is two captures spliced". That was wrong, and the real bytes show how.

MEASURED: `iw dev` in the managed state lists wlan0 AND a second device,
`p2p-dev-wlan0`, as an `Unnamed/non-netdev interface` stanza. In the released
state that device is GONE from `iw dev` entirely. Releasing the interface from
NetworkManager takes its P2P device with it, and `nmcli` reported the sibling
as `unavailable` at the same moment, which
`capture-pi5-nmcli-after-release.txt` records.

So the derivation carried a device that does not exist in the state it claimed
to model. Anything walking that device list would have walked one device too
many. The splice was invented in exactly the place its own note promised
nothing was, which is the hazard of building a fixture out of two states rather
than reading one.

`TestCaptured_ReleasingTheInterfaceRemovesItsP2PDevice` pins the finding, and
fails if either capture stops showing it.

## The stale channel, and the third place a channel was read as a connection

MEASURED on the target 2026-08-30, on the Raspberry Pi 5 caspian-box. wlan0 had
hosted the hotspot, had been put back to managed, and was joined to nothing.
Three independent sources agreed that it was free:

    sudo iw dev wlan0 link       Not connected.
    sudo iw dev wlan0 info       type managed
                                 channel 36 (5180 MHz), width: 20 MHz
                                 (no ssid line)
    nmcli -t -f DEVICE,STATE     wlan0:disconnected
                                 p2p-dev-wlan0:disconnected

and it still reported channel 36, left over from when it last served.

`WirelessIface.StationLink` read that channel as a live connection. The
planner therefore pinned the access point to channel 36 and emitted this note,
which is quoted verbatim from the box's own log:

    WARN "network plan note" note="phy1 reports #channels <= 1, so the hotspot
      is pinned to channel 36, the channel an existing WiFi connection is using
      on wlan0; if that connection roams to another channel the hotspot follows
      it and every joined device is dropped while it does"

There was no existing WiFi connection on wlan0. Two consequences, and the
second is worse than the first:

  - The user had set band 2.4GHz in advanced settings, with channel on auto.
    Channel 36 is 5GHz. hostapd was given a band and a channel that contradict
    each other and the start failed, reported as `fault=hotspot-failed`, which
    says nothing about a channel.
  - When the same pin SUCCEEDED earlier, the hotspot came up on 5GHz channel
    36. The test handset cannot see 5GHz at all: its scan returns 2412 to 2462
    MHz only. The panel showed the hotspot up and broadcasting, which was true,
    and the phone could not find the network, which was also true, and nothing
    in the interface connected the two.

The fix is the same one the release readback got: ASK. `Detect` now runs
`iw dev <if> link` for every wireless interface that is not an access point and
records the answer in `WirelessIface.Associated`, and `StationLink` returns
that measurement. Where the probe did not run or was not understood the SSID is
the fallback. A channel is evidence of nothing.

The note is now emitted only when a station link was measured, and it names the
network it is talking about.

Guards: `TestChannel_AMeasuredNonAssociationBeatsAStaleChannelAndAStaleName`
drives the captured `iw dev` listing together with the captured
`Not connected.`, and fails if the leftover channel or the leftover name is
read as a connection again. `TestChannel_NoNoteClaimsAConnectionThatWasNotMeasured`
guards the sentence itself.

## The three `iw dev <if> link` captures

MEASURED on the target 2026-08-30, on the Raspberry Pi 5 caspian-box, kernel
6.18.34+rpt-rpi-2712 aarch64, iw 6.9. They exist because the readback in
`AssertHotspotInterfaceReleased` now ASKS whether an interface is associated
instead of INFERRING it from a channel, and the parser had to be written
against the real bytes rather than a guess at them.

Four states were run. Three are files; the fourth is a file plus an exit code.

`capture-pi5-iw-link-not-connected.txt` is `Not connected.` with a trailing
newline, and it is the output of BOTH of these:

  - `iw dev wlan0 link` with wlan0 up, managed, and not joined to anything.
  - `iw dev captest link` on a vif freshly created with
    `iw phy phy0 interface add captest type __ap`, whose `iw dev captest info`
    says `type AP` at the same moment.

The two are BYTE-IDENTICAL, sha256 `8355bd37192ada17...` (first 16 hex of the
digest; the file is 15 bytes). That identity is the whole point of the fixture:
a just-created access point interface and an idle station produce the same
answer to "are you associated", which is "no", and that is the answer the
readback needs. `rc` is 0 in both cases. Not connected is not an error.

`capture-pi5-iw-link-connected.txt` is `iw dev wlan1 link` with wlan1 joined to
the house network. Two shapes in it that an authored fixture would have got
wrong, and both are pinned by tests:

  - The first line carries the BSSID and the interface, not the SSID:
    `Connected to 02:00:5e:00:00:01 (on wlan1)`. The SSID is on its own line
    below. `parseIwLink` takes the SSID from the `SSID:` line for that reason.
  - Every line after the first is indented with a literal TAB, not spaces.

`capture-pi5-iw-link-nosuchdev-stderr.txt` is what `iw dev nosuchdev link`
writes to STDERR: `command failed: No such device (-19)`. `rc` is non-zero.
This is the fourth state and it gets its own verdict in the code: a hotspot
interface that does not exist is `ErrHotspotInterfaceMissing`, a named refusal,
NOT "free". Reporting a missing device as released would have let the caller
proceed to configure an interface that is not there.

## The band, and a channel that carried the wrong label

MEASURED on the target 2026-08-30. The user set band 2.4GHz with the channel on
auto, in advanced settings, and the saved state records it:

    {"internet_interface": "eth0", "hotspot_interface": "wlan0",
     "band": "2.4GHz", ...}

The plan pinned channel 36, which is 5GHz, from the stale-channel defect
recorded above. `internal/privsvc` then took the CHANNEL from the plan and the
BAND from the request without checking that they agree, so hostapd was given
hw_mode=g with channel 36. That cannot work, and it surfaced as
`fault=hotspot-failed`, which points at nothing.

The mirror of it, also measured, is worse: when the pin had succeeded earlier,
the hotspot came up on 5GHz channel 36 and the user's handset could not see it
at all, its scan covering 2412 to 2462 MHz only. The panel reported the hotspot
up and broadcasting, which was true.

`Options.HotspotBand` now carries the request into the decision. An explicit
band is honoured or REFUSED, never replaced, and there are two ways to refuse:
the radio has no usable channel in that band, or a station link pins the access
point to a channel outside it. Both name the band; the second also names the
network responsible.

With no band asked for, 2.4GHz wins. That is a REACH decision and it is now
explicit: it used to be a side effect of sorting channel numbers ascending,
which happens to put 2.4GHz first, and a side effect is not a decision.

One claim that did NOT survive checking. The report that prompted this work
said the box "currently prefers 5GHz when the radio is free". It does not, and
did not: `UsableChannels` sorts ascending, so channel 1 is picked on any radio
that has it. The 5GHz hotspot came from the stale-channel pin, not from a band
preference. Recorded because acting on the reported premise would have
"fixed" a preference that was never there.

## Addresses

The captured files contain this network's real RFC 1918 addresses
(10.0.0.0/24) and the radio's real BSSID and SSID. The authored files use
RFC 1918 and RFC 5737 example space. No address in either set belongs to
Google or to any resolver this project uses.

## The radio refuses a combination it advertises

MEASURED on the target 2026-08-30, and it breaks the plan the capability table
implies:

    sudo iw phy phy0 interface add ap0 type __ap
    command failed: Input/output error (-5)      exit 251, no interface appears

with `wlan0` ASSOCIATED, which is the state the planner plans for.

CORRECTION, measured later the same day on the same box: the refusal is
CONDITIONAL on that association, not unconditional. With `wlan0` NOT joined to
a network, the SAME command returned rc=0 and the interface appeared. Any
sentence in this package that says brcmfmac always refuses `type __ap` is
wrong; what it refuses is a second interface while the first one holds a
station link. The `-5` capture above records one arm of that, and
`capture-pi5-iw-link-not-connected.txt` records the state in which the other
arm succeeds.

`iw list`
on the same radio reports `#{ managed } <= 1, #{ AP } <= 1, #{ P2P-client }
<= 1, #{ P2P-device } <= 1, total <= 4, #channels <= 1`, and the parser and
planner read that correctly. The `brcmfmac` driver refuses anyway.

So `capture-pi5-iw-list.txt` states what the hardware could do IN PRINCIPLE.
It is not evidence that creating the interface succeeds, and no test built on
that fixture can be. Only trying it settles the difference, which is why the
answer is a runtime fallback (`Plan.HotspotTakeover`) rather than a planner
rule.

A second wording was observed for the same command with a name that already
exists as a netdev: `command failed: Invalid exchange (-52)`. Neither string is
in `alreadyExistsMarkers`, and the `-5` one must never be: it means nothing was
created, which is the opposite of "already exists".

## golden-ruleset-takeover.nft, and where its nft status is kept

It is the ruleset the target will actually install, because the driver refuses
the plan the capability table implies and the box falls back to taking over
`wlan0`. Every interface match in it names `wlan0` where the mode A and mode B
goldens name `ap0`, so it is a different file and needs a check of its own.

THIS SECTION DOES NOT SAY WHETHER IT HAS HAD ONE, and that omission is the
point. It used to say the file had never been through `nft -c -f`. That was
true when it was written and stopped being true on 2026-08-30, when the check
was run against every ruleset golden. Nothing failed, because prose sitting
beside a record is not checked against the record. The same stale sentence
existed a second time, above `TestGolden_RulesetTakeover`.

The status lives in ONE place: `nftCheckedDigests` in
`internal/netcfg/golden_test.go`, keyed by the sha256 of the bytes. Read it
there rather than trusting a summary of it here, and note that a digest is
matched against the CURRENT file, so an entry stops covering a golden the
moment the generator changes.

`TestProvenance_NoDocumentClaimsAVerifiedRulesetIsUnchecked` is the guard, and
it fails if this file or `golden_test.go` claims a golden is unparsed while the
record says otherwise. `TestProvenance_TheUncheckedClaimGuardCanActuallyFail`
is its positive control, so a typo in the phrases it looks for cannot make it
silently pass forever.

## The input policy was measured locking the owner out, and was removed

MEASURED on the target 2026-08-30. With the previous ruleset loaded (input
policy drop, nothing accepted on the uplink) every NEW inbound connection to
the box was refused and SSH stopped answering on both addresses. Established
connections kept working, so the session already open stayed responsive and
the panel answered normally while the box was unreachable to everything new.
Switching the appliance off restored it immediately, which places the cause in
the input chain rather than in a crash or the interface takeover.

The input policy is now accept, and the only restriction in that chain is on
the hotspot side. See doc.go, "What this appliance does not firewall".

CONSEQUENCE FOR THE VERIFICATION RECORD: every ruleset golden changed, so the
`nft -c -f` results recorded above cover bytes that are no longer in the tree.
`TestGolden_CheckedRulesetDigestsAreStillCurrent` FAILS deliberately until the
check is re-run. The digests it reports are:

    61a6306c570b0c537eedbaaf5c7ba24835ce4cecc03fa72f0a7781141f2a9937  captured, mode A
    558933430ad45b92f143ca50b1be53b6b355aa6eecf9ea39aa38ccf25e719d59  mode B
    1168bb33a0801367516edc7ae706acdf45c8496d1aea136c499f40e7528a8c27  takeover

Captured and mode A are still byte-identical, so this is three distinct
rulesets, not four.

## Ruleset goldens parsed by nft on the target, 2026-08-30

The three distinct `golden-ruleset-*.nft` files in the tree were copied to the
Pi and checked with `sudo nft -c -f`. All returned exit 0.

    61a6306c570b0c537eedbaaf5c7ba24835ce4cecc03fa72f0a7781141f2a9937
                    golden-ruleset-captured.nft, golden-ruleset-mode-a.nft
    558933430ad45b92f143ca50b1be53b6b355aa6eecf9ea39aa38ccf25e719d59
                    golden-ruleset-mode-b.nft
    1168bb33a0801367516edc7ae706acdf45c8496d1aea136c499f40e7528a8c27
                    golden-ruleset-takeover.nft

Captured and mode A are byte-identical, so three rulesets were checked, not
four. The digests above were read back with `sha256sum` ON THE PI rather than
taken from this machine, because what matters is that the bytes nft parsed are
the bytes in this tree, and a local hash of a file that was copied does not
establish that.

    nft --version   nftables v1.1.3 (Commodore Bullmoose #4)
    uname -r        6.18.34+rpt-rpi-2712

`sudo nft list ruleset | wc -l` returned 0 immediately before and immediately
after the run, so nothing in the result came from a rule that was already
loaded, and the check left the box as it found it. `nft -c` parses and does not
commit, which is why this is safe to run on a box being used for other things.

These digests are pinned in `TestGolden_CheckedRulesetDigestsAreStillCurrent`.
The two earlier digests in that map are the input-policy-drop rulesets checked
the same morning; that policy was withdrawn after it was measured closing every
new inbound connection to the box, and they are kept labelled WITHDRAWN so that
a golden reverting to the old shape reports as withdrawn rather than as never
checked.

## The nmcli fixtures are CAPTURED, and so is the whole release sequence

Captured from the target on 2026-08-30, nmcli 1.52.1, same vantage as the
other captures on this page: run through `env -i` with the absolute path, C
locale, which is what the package's own runner does.

    capture-pi5-iw-info-ap-serving.txt   iw dev wlan0 info, hostapd beaconing
    capture-pi5-iw-info-freed-ap.txt     iw dev wlan0 info, released and typed,
                                         nothing serving
    capture-pi5-iw-dev-freed-ap.txt      iw dev, the whole output, in that same
                                         released and typed state
    capture-pi5-nmcli-device-status.txt   nmcli -t -f DEVICE,STATE device status
    capture-pi5-nmcli-after-release.txt   the same command after the release,
                                          FILTERED to the two lines that were
                                          reported; it is not a whole listing

Two shapes in those bytes that the authored guess did not have, and both are
now tested:

  - A state can be `connected (externally)`, with a space and a parenthetical.
    `lo` and `xray0` carry it; `eth0` and `wlan0` do not. Any classifier
    comparing the whole field against `connected` answers differently for the
    two groups.
  - The radio presents a SECOND device, `p2p-dev-wlan0`, whose name contains
    the real one. After the release the two have DIFFERENT managers, wlan0
    `unmanaged` and the sibling `unavailable`, so a substring lookup returns
    whichever the map happens to yield. Devices are keyed and looked up by
    exact name; an audit of the package found no prefix or substring match on
    an interface name anywhere in production code.

Also read on the box: `nmcli -t -f GENERAL.STATE,GENERAL.CONNECTION device show
wlan0` gives `100 (connected)` and `netplan-wlan0-HomeNet`, so this is a
netplan-rendered NetworkManager.

`scenario-leftover-hotspot-addr.txt` is CAPTURED in substance and AUTHORED in
form, which is why it carries the scenario prefix. Its shape is `ip -br addr`
output for a machine whose hotspot interface still holds an address from a
previous run, alongside its own: on 2026-08-30 the real box was in exactly that
state, wlan0 carrying both 10.0.0.222/24 and a leftover 10.83.51.1/24 from a
hand-run probe, and a start refused. The refusal was not caused by the extra
address itself; it was caused by `ip address del` meeting an address that had
already gone when NetworkManager released the interface, which `Apply` treated
as fatal. The fixture exists so the release sequence is exercised against an
interface carrying more than it should.

`scenario-nmcli-wlan0-unmanaged.txt` stays AUTHORED. It models a state this box
was not in, and it drives the refusal test for an interface that is associated
while no manager claims it.

## The release sequence has been run on the target

MEASURED 2026-08-30, over the eth0 session, with a trap that restored
regardless of outcome. Every command below is one this package generates, in
the order it generates them:

    nmcli device set wlan0 managed no        exit 0   -> wlan0:unmanaged
    ip address del 10.0.0.222/24 dev wlan0   exit 0
    ip link set dev wlan0 down               exit 0
    iw dev wlan0 set type __ap               exit 0   -> iw info: type AP

and the inverses put the box back: type managed, link up, managed yes, and
eight seconds later wlan0 was `connected` on HomeNet with 10.0.0.222/24
returned. `p2p-dev-wlan0` returned to `disconnected` on its own.

So the takeover's forward sequence and its journalled inverses are no longer
reasoned from a simulator. What has NOT been run is the sequence driven by this
package rather than by hand, and everything after it: hostapd, dnsmasq, and the
readback that is supposed to stop the box reporting itself up without them.

### The prediction that was wrong

The previous report flagged `iw dev wlan0 set type __ap` as the next likely
failure, because the same driver refuses `iw phy phy0 interface add` with
`Input/output error (-5)`. It does not fail. Creating an interface and changing
the type of one are different operations and brcmfmac treats them differently.
That distinction is the whole justification for the takeover path, and there is
no USB adapter question to answer on this hardware.

## Observed, not ours, not explained: xray0 outlives the engine

`xray0` appears in the NetworkManager device list with the service switched
OFF, as `connected (externally)`. Nothing here established why, and the engine
is not this package's code.

It does not change the fail-closed ruleset, and that is deliberate rather than
lucky. If the device is gone, the kernel withdraws the routes through it and
the leak block catches the fallback to the uplink. If it persists with nothing
servicing it, traffic entering it is dropped there. Neither branch leaks and
neither depends on knowing which one happens, which is why the block was
written to name only the hotspot and the uplink.

Worth someone answering anyway: a TUN opened without IFF_PERSIST goes away when
the last file descriptor closes, so a device that survives the engine suggests
either a descriptor still held or a persistent device, and the second would
change what a restart is doing.

## golden-commands-takeover.txt

The command sequence for the fallback path, which is the one the target
actually takes. Generated, not captured. It is the file to read when asking
what this package does to `wlan0`, and every line in it has its inverse beside
it.

## The kill switch was extended to the box's own traffic

The OUTPUT chain is now `policy drop` with a named permit list. The premise was
measured on the target on 2026-08-30 by enumerating what runs rather than
sampling traffic, because the things that matter are periodic and a short
capture misses them:

    NetworkManager      two DHCP client sockets, 10.0.0.221:68 and
                        10.0.0.222:68, both established to 10.0.0.1:67
    systemd-timesyncd   active, NTP=yes, NTPSynchronized=yes
    avahi-daemon        running; the box sends IGMP reports for 224.0.0.251
    wpa_supplicant, bluetooth, cron, udisks2   running, no network peer
    45s capture on eth0, appliance off and idle: six packets, five ARP
                        replies and one IGMP report

Each permit in the chain names the reading that justifies it. One permit was
NOT in that enumeration and was added from reasoning rather than measurement:
`udp sport 67 udp dport 68`, the box answering DHCP as a SERVER on the hotspot.
The socket table showed the client half only. Established cannot cover the
server half, because a DHCP reply goes to a broadcast address or to a client
that has no address yet, so request and reply share no tuple. Without it the
hotspot beacons, devices associate, and none of them gets an address.

### Two new goldens

    golden-ruleset-takeover-cut.nft           forwarding cut by the user
    golden-ruleset-takeover-egress-open.nft   the EgressOpen way back

### Every current ruleset has been parsed by nft on the target

Extending the kill switch changed the OUTPUT chain in every ruleset: six files
holding five distinct rulesets, since captured and mode A are byte-identical.
All five were checked on 2026-08-30, and this section replaces an earlier one
that said none of them had been, which was true when it was written and stopped
being true the same afternoon.

Each file was copied to the Pi, parsed with `sudo nft -c -f`, every one exit 0.
The sha256 of each was read back ON the Pi rather than taken from the developer
machine, so the bytes nft parsed are provably the bytes in this tree, and
`nft list ruleset | wc -l` returned 0 immediately before and immediately after,
so nothing in the result came from a rule already loaded and the check left the
box as it found it. nftables 1.1.3, kernel 6.18.34.

The five digests and their vantage line are pinned in
`TestGolden_CheckedRulesetDigestsAreStillCurrent`, which passes. If it fails,
a ruleset has changed and needs checking again; it does not mean this section
is wrong.

### The three provocations, run with the policy loaded

Measured on the target 2026-08-30, with the appliance running and
`table inet caspian` loaded, output policy drop.

    DHCP   nmcli connection down/up netplan-eth0, a real DISCOVER/REQUEST
           address 10.0.0.221 back after 3s, default route intact, table
           still loaded throughout
    NTP    systemctl restart systemd-timesyncd
           NTPSynchronized=yes after 3s, against 0.debian.pool.ntp.org at
           143.20.69.40, which also exercised resolution
    DNS    getent hosts deb.debian.org answered, and a direct lookup of
           example.org returned 172.66.157.237

And the negative controls, which matter more than the permits, because a
policy that lets everything through would pass all three above.

The proxy server's address is written as <the proxy server> rather than as a
number. .gitignore names server addresses as live credentials, alongside user
ids and Reality keys, and ignores /local/ for exactly that reason; this file is
committed. The address adds nothing to the evidence here, because what the test
shows is that the ONE address the policy names connects while three that it
does not name time out, and that holds whatever the number is. The other
addresses below are a public DNS resolver, an IANA example host and an NTP pool
member, which are not secrets.

Recorded 2026-08-30. The address is already permanent in git history, so this
removes it from the working tree and from every future clone's checkout, and
not from the past. Rewriting history to remove it is a separate decision.

    <the proxy server>:443  permitted by address                   CONNECTED 0.02s
    1.1.1.1:443           not permitted                            timed out
    93.184.216.34:80      not permitted                            timed out
    1.1.1.1:53            DNS, permitted                           CONNECTED

A note on how not to run these, because the first attempt cost the box its
wired link and half an hour. A script that resolves the NetworkManager
connection name AFTER taking the interface down reads an empty device column
and runs `nmcli connection up ""`, which leaves the machine with no route and
looks exactly like the permit being wrong. Capture the name once, while it is
still active. The recovery, if it happens anyway: the hotspot is still up, so
join a device to it and use the panel on the hotspot address to switch the
appliance off, which hands the radio back and restores ssh.

Two probes that prove nothing on this box: `/dev/tcp/host/port` is a bash
feature and the shell here is dash, so both a permitted and a blocked address
return the same "Directory nonexistent" error. Use a real socket.


## The two-radio machine, CAPTURED 2026-08-30

A TP-Link TL-WN823N v2/v3 (RTL8192EU, usb 2357:0109) plugged into the target.
Same vantage as every other capture here: Raspberry Pi 5 caspian-box, kernel
6.18.34+rpt-rpi-2712 aarch64, iw 6.9, nftables 1.1.3.

    capture-pi5-2radio-ip-br-addr.txt    ip -br addr
    capture-pi5-2radio-iw-dev.txt        iw dev
    capture-pi5-2radio-iw-list.txt       iw list
    capture-pi5-2radio-nmcli.txt         nmcli -t -f DEVICE,STATE device status

    phy0  brcmfmac, built in, wlan0
    phy1  rtl8xxxu, TP-Link RTL8192EU, wlan1

At capture time the appliance was running: wlan0 was the access point
broadcasting Caspian-Wifi on channel 1, unmanaged by NetworkManager, and wlan1
was a station joined to the house network on channel 10 under NetworkManager.

### An absent declaration is not a prohibition

`phy1` declares AP, AP/VLAN, managed and monitor among its interface modes and
declares NO "valid interface combinations" section at all. `phy0` declares two.
`Phy.APWithStation` therefore answered false for the dongle, and the planner
turned that into a refusal.

The refusal was wrong, and wrong in the mirror of a direction this package has
already been caught in. The brcmfmac finding was that a DECLARED capability is
aspirational: phy0 advertises AP alongside managed and then refuses
`interface add` with `Input/output error (-5)` while wlan0 is associated. This
one is the inverse: an absent declaration was read as a prohibition.

MEASURED on the target the same day, station released first:

    nmcli device set wlan1 managed no
    ip addr flush dev wlan1
    ip link set dev wlan1 down
    iw dev wlan1 set type __ap          exit 0
    ip link set dev wlan1 up
    hostapd <config>                    wlan1: AP-ENABLED

A combination table is about COEXISTING. It has nothing to say about a radio
whose station is going to be ended, which is what the takeover does. The two
questions are now asked separately: a link that must be KEPT, because it is the
uplink, still needs the declaration; a link that may be ENDED needs only AP
support.

### Second occurrence: releasing an interface takes its P2P device

The nmcli capture shows `wlan0:unmanaged` and `p2p-dev-wlan0:unavailable`, and
the `iw dev` capture shows phy0 with `Interface wlan0` and NO
`Unnamed/non-netdev interface` stanza, while the managed-state capture
`capture-pi5-iw-dev.txt` does show one. This is the same finding as
`TestCaptured_ReleasingTheInterfaceRemovesItsP2PDevice`, observed
independently on a second occasion and on a machine with two radios.

### Authored fixtures built alongside these

    scenario-2radio-ip-route-wlan0.txt      a default route on the built-in
                                            radio, so it is the uplink and the
                                            dongle is free to host
    scenario-dongle-only-ip-route.txt       a machine whose only radio is the
    scenario-dongle-only-ip-br-addr.txt     dongle, using it for the internet
    scenario-dongle-only-iw-dev.txt
    scenario-dongle-only-nmcli.txt
    scenario-dongle-only-iw-list.txt        the phy1 block of the capture above,
                                            verbatim; the MACHINE it describes
                                            is authored

The dongle-only set exists for the case that must still refuse: the only radio
that could host is the one carrying the internet connection, so the access
point would have to coexist with it, so the declaration is required and its
absence is decisive.

## CLOSED AT PLAN LEVEL: the second-interface attempt is no longer made on a radio that cannot hold one

STATUS, 2026-08-30, third and final revision of this section. The plan no
longer attempts a second interface on a radio that declares no combination of
managed and AP. It ends that radio's station link first and runs the access
point on the interface already there, which is the shape the radio itself
declares it can do.

The three states this section has been through, because the sequence is the
lesson:

  1. ATTEMPTED. A note said "choosing it for the hotspot will fail to start"
     and the plan returned success. The start created the interface, failed
     two steps later at `ip link set dev ap0 up`, and left a part-applied
     start to unwind.
  2. REFUSED. The plan stopped before touching the network. Honest, and it
     delivered nothing: the dongle is the hardware the user bought for this.
  3. TAKEN OVER. The radio's own declaration was measured (below), it says AP
     among its modes and no combinations, and those two facts together name
     the sequence exactly.

CLOSED means the plan is right about this radio. It does NOT mean the dongle
has been seen to work: whether it BEACONS under hostapd is unmeasured, and the
readback is what has to catch a radio that accepts AP mode and serves nothing.

New measurement, target, 2026-08-30, which is what settled it:

    TP-Link TL-WN823N v2/v3, Realtek RTL8192EU, driver rtl8xxxu
    firmware rtlwifi/rtl8192eu_nic.bin rev 35.7, loaded, no errors

    iw phy <dongle> info, supported interface modes:
        managed, AP, AP/VLAN, monitor
    iw phy <dongle> info, valid interface combinations:
        (none declared)

The measurements below are unchanged and are why the two-interface attempt was
the wrong shape.


MEASURED on the target 2026-08-30. Two radios, two different refusals, and the
package can only detect one of them.

    phy0, brcmfmac    iw phy phy0 interface add ap0 type __ap
                      WITH WLAN0 ASSOCIATED:
                      -> command failed: Input/output error (-5), exit 251
                      -> no interface appears; the step fails with op=iface
                      WITH WLAN0 NOT JOINED TO ANYTHING, same day, same box:
                      -> rc=0, the interface is created. The refusal is
                         conditional on the station link, not a property of
                         the driver.

    phy1, rtl8xxxu    iw phy phy1 interface add captest type __ap   -> rc=0
                      THE INTERFACE IS CREATED, carrying wlan1's MAC verbatim
                      (02:00:5e:00:00:12 on both)
                      ip link set dev captest up
                      -> RTNETLINK answers: Name not unique on network
                      with a distinct MAC:
                      -> RTNETLINK answers: Device or resource busy
                      the step fails with op=link

The second wording is the kernel's for a duplicate ADDRESS, not a duplicate
name, which sends a reader to the wrong place.

So "declares no interface combinations" was ACCURATE about phy1: the radio
cannot hold an access point beside an associated station. The earlier entry on
this page argued an absent declaration is not a prohibition. That is still true
as a statement about what can be INFERRED, and it was turned into "so attempt
it anyway", which is a guess in the other direction and is measured wrong here.

`TestVifEvidence_*` pins both walls. `SimulatedKernel.InheritsParentMAC` and
`.RefuseLinkUp` model them.

### Why this is not fixed in this change

The fix that removes the problem is to plan the takeover directly for a radio
whose link may be ended, rather than attempting a second interface and falling
back. It was written and measured: it produces the exact command sequence
proved by hand on the box, in ONE pass, and every ruleset it generates is
byte-identical to one nft has already parsed on the target, so it needs no
re-verification.

It was reverted rather than shipped because it changes behaviour for the
built-in radio as well as the dongle, which is beyond what was asked, and
because it turns 30 existing tests red. Rewriting 30 test expectations around
an unapproved design change, with no hardware to verify against, is the exact
pattern that produced several of the defects on this page.

### NOT reproduced: the partial-undo leak

The live log reported `undone=6 left=1` after a failed start. The model here
rolls back completely: `TestVifEvidence_TheFailureLandsOnAnOpNoFallbackWatches`
asserts the machine returns to its exact prior state. So the leak is real on the
box and its cause is not established. The journal file names the entry that was
left; that entry is what would settle it.
