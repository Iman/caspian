# Security and privacy

[English](https://github.com/Iman/caspian/wiki/Security-and-Privacy) | [فارسی](https://github.com/Iman/caspian/wiki/Security-and-Privacy.fa) | [Русский](https://github.com/Iman/caspian/wiki/Security-and-Privacy.ru) | [中文](https://github.com/Iman/caspian/wiki/Security-and-Privacy.zh)

[Caspian wiki](https://github.com/Iman/caspian/wiki/Home)

> This guide comes from the existing README. Its measurements retain their original dates; this documentation move does not report a new test run.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## What it guarantees

Each heading here is backed by generated firewall output in
`internal/netcfg/testdata/`, by a named test, or by a measurement recorded in
the repository. [`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) is the readable list of promises. Every
heading in it is the name of a scenario in `test/bdd/`, and every scenario has a
matching injected defect. So "this test can detect the thing it claims to
detect" is itself a test result.

### Forwarded client traffic fails closed, and the block does not need the tunnel

The forward chain's policy is `drop`. The first rule in it is the leak block,
and it names only the hotspot and the uplink:

    iifname "wlan0" oifname "eth0" drop comment "fail-closed: client traffic never leaves by the uplink"

Every rule that permits client traffic names the tunnel device, so when the
tunnel disappears those rules stop matching and the policy drops everything. The
block itself cannot stop working when the tunnel goes, because it does not
mention it. Every interface is matched by name and never by index, so the
ruleset loads with no tunnel present, which is exactly when it is needed. The
postrouting chain is empty on purpose.

Scenario: "with the tunnel gone, nothing lets client traffic out by the uplink".
Supporting analyser test: `TestWithoutInterfaceRemovesOnlyTheRulesNamingIt`.

### The kill switch covers the box's own traffic too

The output chain is `policy drop` with a named permit list. The permits were
derived by enumerating what actually runs on the target rather than by sampling
traffic, and each permit in the generated ruleset carries the reading that
justifies it: NetworkManager's DHCP client sockets, systemd-timesyncd, DNS, the
loopback, the tunnel device, IPv6 neighbour discovery, and the proxy server
permitted **by address** rather than by port, so a UDP-on-443 transport is not
broken silently. One permit was added from reasoning rather than measurement and
says so: the box answering DHCP as a server on the hotspot, which conntrack
cannot cover because a DHCP reply and its request share no tuple.

The provocations and, more importantly, the negative controls are recorded in
`PROVENANCE.md`, "The three provocations, run with the policy loaded". Tests:
`TestRestrictedEgress_PermitList`,
`TestRestrictedEgress_AcceptsEstablishedBeforeItDropsAnything`,
`TestRestrictedEgress_ServerIsPermittedByAddressNotPort`.

The cost is stated in the ruleset's own header rather than discovered: `apt
update` from a shell on the box fails while the appliance is on.

### There is an emergency cut that does not take the hotspot with it

Switching the appliance off takes the hotspot down, which disconnects the phone
holding the button. So there is a separate control that drops forwarded client
traffic while leaving the hotspot, DHCP, DNS and the panel up. See
[`internal/privsvc/cut.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/cut.go) and panel action `cut` in [`internal/panel/priv.go`](https://github.com/Iman/caspian/blob/main/internal/panel/priv.go).

The cut is runtime state and is never written to disk, so pulling the plug
undoes it. Tests: `TestCuttingClientTrafficLeavesTheWayBack`,
`TestACutIsNeverWrittenDown`, `TestACutDoesNotSurviveARestart`,
`TestForwardCut_StopsClientsAndKeepsThePanelReachable`.

See [Panel and configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) for the controls and what each one stops.

### It reads the interface back from the kernel instead of trusting a process

A started process is not evidence that it worked. This was a real failure. The
header of [`internal/privsvc/readback.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/readback.go) records that on 2026-08-30 the service
logged itself running with a hotspot on `wlan0` while `wlan0` was still a
station on the house network. hostapd was a live process whose control socket
did not answer. A phone in the room listed eleven networks with ours not among
them. And dnsmasq was answering a stranger's device on somebody else's LAN with
a DHCPNAK.

Nothing is allowed to bind to the hotspot interface until
`netcfg.AssertHotspotInterfaceReleased` proves it is free, and nothing reports
itself running until `AssertHotspotIsAccessPoint` reads back an access point
broadcasting the expected name. Tests:
`TestNothingBindsToTheHotspotInterfaceUntilItIsProvedFree`,
`TestTheServiceDoesNotReportRunningUntilTheAccessPointReadsBackAsOne`,
`TestAnAccessPointBroadcastingAnotherNameIsNotOurs`,
`TestTheReleaseIsReadBackBeforeAnythingBindsAndTheAccessPointAfter`.

### You get your WiFi back

Every network change is written to `/var/lib/caspian/netcfg.journal` with its
inverse **before** it is made, and switching off replays those in reverse. The
record is on disk rather than in memory, so a killed process or a power cut does
not lose it. A box that died mid-change replays the record before it looks at
the machine or applies anything new.

The takeover of a WiFi interface is journalled step by step. The forward
sequence and its inverses have been run on the target and are recorded in
`PROVENANCE.md`, "The release sequence has been run on the target". The four
commands went out, and the inverses put the box back on its own network with its
own address eight seconds later.

One change has no inverse on purpose, and
`TestPlan_InvariantsHoldOnEveryModelledMachine` asserts it is the only one:
bringing the hotspot interface up. Taking a radio down on the way out is worse
than leaving it up, because the machine's own WiFi, and the panel the user is
reading, can be on it.

Scenarios: "turning the switch off returns every change the box made", "a
teardown replayed from the journal of a killed process undoes the same changes",
"a box killed halfway through cleans up before it does anything else". Tests:
`TestJournal_RecordsInverseBeforeTheChange`,
`TestTeardown_ReplaysInExactReverseOrder`,
`TestRecover_UndoesAJournalLeftByAKilledProcess`,
`TestTheTakeoverReleasesTheInterfaceItSaysItWillRelease`.

If an inverse fails, the firewall's own inverse is **held** rather than run, so
a box that could not undo its routes keeps its block. Test:
`TestTheFirewallIsNotRemovedWhenAnEarlierInverseFailed`.

### The pasted config never reaches a screen, a log or a readable file

Everything the flow produces is searched for the pasted credential:

- every error, and every message shown to the user
- the appliance's log lines
- the panel's description of the config
- the saved settings as they render for diagnostics
- the generated firewall
- the generated DHCP and DNS configuration
- the engine's own log
- the journal on disk
- the request that crosses from the unprivileged panel to the privileged service

It is in none of them. It is checked to be in the two places it has to be, so
the test cannot pass by the config having gone missing.

Scenarios: "the pasted credential never reaches a screen, a log or a readable
file", "the hotspot password reaches the access point and nothing else". Tests:
`TestPastedConfigNeverAppearsInAResponseOrALog`,
`TestFailedConfigPathsDoNotEchoTheInput`, `TestStartRequestRedactsItself`,
`TestNoCredentialReachesTheAdvancedView`,
`TestTheServerAddressNeverAppearsInADiagnosticLine`.

### The panel asks the internet for nothing

Every stylesheet, script and icon the browser loads is compiled into the binary
with `go:embed`. See [`internal/panel/assets.go`](https://github.com/Iman/caspian/blob/main/internal/panel/assets.go). There is no web font at all:
the stylesheet's font stack is entirely system faces, Persian-capable ones
first.

The privacy reason is that a remote asset tells a third party the address of
everyone who opens the panel. The stronger reason is availability. The panel has
to load when the tunnel is down, which is exactly when somebody needs it.

Two mechanisms, not one. `TestNoAssetReferencesAnExternalURL` and
`TestNoRenderedPageReferencesAnExternalURL` scan the assets and every rendered
page for an absolute URL. `setSecurityHeaders` sends `default-src 'none'` with
every listed source set to `'self'`, so a browser refuses one that got past the
tests. No outbound HTTP client exists anywhere in `internal/panel` outside its
own tests.

The generated configuration also names no Google resolver anywhere, and uses no
`geoip:` or `geosite:` rule, because either would reintroduce a download to a
product whose whole install story is one verified binary. Tests:
`TestNoGoogleAnywhereInGeneratedConfigs`,
`TestGoogleResolverIsRejectedAtTheSource`. Scenario: "the box needs no download
and asks no Google server anything".

### Privilege is split

`caspian serve --privileged` runs as root and owns routes, the firewall, the
access point and the engine. It accepts a short list of named actions over a
unix socket and never a command built from user input. `caspian serve --panel`
runs as the unprivileged `caspian` account and owns the web interface and
nothing else. See [Architecture](https://github.com/Iman/caspian/wiki/Architecture) for the vocabulary and frame format.

The panel password is hashed with argon2id. See [`internal/state/password.go`](https://github.com/Iman/caspian/blob/main/internal/state/password.go). It
is a local password on the box. There is no account anywhere else.

### The clock is checked before anything handshakes

A Pi has no battery clock, and two separate mechanisms depend on the wall clock.
REALITY writes it into the handshake, and which configs xray-core **accepts**
depends on the date. So a box whose clock comes up wrong does not merely fail to
connect. It accepts a config the same binary rejects once the clock is
corrected.

The check runs before validation and before anything is attempted. See
[`internal/privsvc/clock.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/clock.go), called from `Service.Start` as step 1 of
`applyLocked`. It raises a distinct fault so the panel does not blame the user's
config. Test: `TestClockFailureIsNotBlamedOnTheConfig`.

### Three config failures are told apart

"Could not read that link", "read it, and it cannot be used as written", and
"the link was fine and the server did not answer" need three different actions
from the user, and the third is the commonest. Blaming the config first is what
makes somebody throw away a config that was never broken. Nothing on the machine
is touched before the pasted text is read. Scenarios: "text that is not a link
at all is refused before anything is touched", "a link the engine will not
accept is told apart from one that would not parse", "a link whose server never
answers is not blamed on the link".

## What it does not guarantee

This list is the one to read closely.

### DNS over HTTPS on port 443 is carried, not blocked, and nothing here can see it

Client DNS on port 53 is redirected to this box over both protocols rather than
merely permitted. So a device with a hardcoded resolver is answered here, rather
than allowed out to reach the one it was told to use. DNS over TLS on 853 is
rejected with a TCP reset, so the device falls back to the redirected port. DNS
over QUIC on 853 is dropped.

DNS over HTTPS on port 443 is not distinguishable from any other HTTPS and is
carried through the tunnel like anything else. A client using it is inside the
tunnel and is not leaking. It is also invisible. Nothing in this project, and
nothing in the hardware harness, can observe it. That is a limit of the design.
It is stated in the generated ruleset itself, in [`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md), and in the
printed output of the DNS leak check rather than only here.

### IPv6 is blocked, and the IPv6 path is not finished

There is no IPv6 tunnel. A device with a working IPv6 path prefers it over IPv4
and would bypass the tunnel entirely, so the default policy is to block. Four
things hold that. `IPv6Block` is the default in `netcfg.DefaultOptions`. The box
does not forward IPv6. The firewall drops forwarded IPv6 on the hotspot in both
directions. And router advertisements towards the hotspot are dropped, so a
device cannot give itself an address. Scenario: "clients are never offered the
IPv6 the tunnel cannot carry".

`IPv6Forward` exists as an option and its own comment says not to set it. The
engine's TUN inbound has not been shown to carry IPv6 on the target. It also
adds no permit rule to the forward chain, deliberately: the IPv4 permits name
the hotspot subnet on both directions, there is no v6 prefix anywhere in the
plan to name, and a rule matching only the two interface names would accept any
source address a client wrote. `TestRuleset_NoUnconstrainedIPv6AcceptInForward`
holds that line.

**"Blocked" is about routing, not about DNS, and the difference matters.** An
AAAA query from a joined device is not suppressed and is not answered empty. It
goes to the engine, through the tunnel, and comes back with real AAAA records,
because the engine document asks for `UseIP` and dnsmasq sets no `filter-AAAA`.
A device therefore learns IPv6 addresses it has no way to reach, and falls back
to IPv4.

That is harmless while nothing can give a client a v6 address, and it is the
first thing that stops being harmless if anything ever does, because a client
with a working v6 path prefers the AAAA answer and would leave by a route this
box does not carry. It is written down here rather than left as a surprise, and
`TestAAAAQueriesAreAnsweredAndNotSuppressed` pins both halves so that changing
it has to be a decision.

**The hardware rig cannot grade IPv6 at all, so no IPv6 result from it means
anything.** [`test/hardware/README.md`](https://github.com/Iman/caspian/blob/main/test/hardware/README.md) records, under "What this vantage cannot
grade: IPv6", that the phone carries only a link-local address, that
`ip -6 route show default` is empty on the phone and on the Pi, and that a
connection to an IPv6 literal answers "Network is unreachable". There is no IPv6
on that LAN at all, so an IPv6 leak check run there would pass without the
appliance doing anything. Every hardware result this project has is an IPv4
result. Anyone running this on a network with working IPv6 must treat that as a
new question, not a covered one, and must expect to write the test rather than
enable one.

### The box's own traffic is outside the fail-closed promise, by design

The promise is about **forwarded client traffic**. The box's own connection to
your server has to reach the uplink directly or there is no tunnel at all, and
[`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md) section 7 puts the box's own traffic outside the
guarantee for that reason.

The output-chain kill switch narrows that, and does not close it. The generated
ruleset states the residual in its own header. DNS is a hole: anything on the
box can still reach the network on port 53, and the server's hostname is
resolved in the clear on the local network before any tunnel exists. Neither is
a leak of client traffic, and neither is made worse by the kill switch.

The input chain's policy is `accept`, also by design and also stated in the
ruleset. An earlier version had it as `drop`, and `PROVENANCE.md` records what
happened when it was measured on the target. Every new inbound connection was
refused and SSH stopped answering, while the already-open session kept working,
which on a headless machine is indistinguishable from a crash. The only place
the input chain restricts anything is the hotspot side, where a joined device
reaches DHCP, DNS, the panel and ICMP echo and nothing else on the box.

### Client isolation is a rule, not a measurement

The ruleset contains `iifname "wlan0" oifname "wlan0" drop`. That the rule is
present is checked. That it works is not.

### Nothing in this repository captures an exit IP

`test/tunnel` moves real bytes through a real xray-core server, and everything
in it is on loopback, so it captures no exit IP and cannot. `test/bdd` has no
network, no radio, no root and no tunnel device. It runs the real engine in
process through the real config loader, so "the engine accepted this config and
started" means what it says, but the tunnel inbound is switched off.

So nothing in this repository satisfies the project's own standard for calling
something working. [`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) ends with a section, "What this suite
does not prove", listing what is still owed. Read it as part of the suite.

### Nothing re-checks the firewall once it is loaded

See [defect D1](https://github.com/Iman/caspian/wiki/Troubleshooting). If something flushes the table while the appliance is
running, the box keeps forwarding, the panel keeps reporting connected, and
nothing notices.

### Nothing watches the uplink

The internet moving, because a cable was unplugged or a lease renewed
differently, is a change nothing fails loudly for. The pinned route to the
server still exists and still points at an address that is no longer a way out.
The box does not notice, and the tunnel stays stopped until somebody presses the
switch again.

What that costs is availability, not privacy. Client traffic stays blocked
throughout, because the forward policy is drop and every accept in it names the
tunnel. `netcfg.WatchUplink` and `Plan.RederiveForUplink` exist and work, and no
shipped code calls either. `TestNothingInTheApplianceWatchesTheUplink` is what
stops the opposite sentence drifting back into the documents, where it stood
until 2026-08-30.

### Mode B has never been run on real hardware

Every mode B fixture is authored. `PROVENANCE.md` records that the target has
one radio and no USB adapter, so the arrangement this product tells people to
buy an adapter for is proven against bytes nobody measured.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

[Architecture](https://github.com/Iman/caspian/wiki/Architecture) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting)
