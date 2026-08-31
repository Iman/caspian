# What Caspian-BYOC guarantees

> Persian edition: [`docs/BEHAVIOUR.fa.md`](BEHAVIOUR.fa.md). The English file
> is the one the tests read. If the two ever disagree, this one is correct.

This is the readable list of what the product promises, and of what is actually
checked on every run. It is not a description of the code. Each behaviour below
is a scenario in `test/bdd/`, and the heading is the scenario's name, so you can
run one on its own:

    go test ./test/bdd/ -run 'TestBehaviour/the_firewall'

Two things to read before the list, because they decide what a green run means.

**Nothing here proves a packet went anywhere.** The design's own rule is that
nothing is called working without an exit IP captured from real traffic
(`docs/2026-08-29-design.md`, section 6). Every scenario below is one layer
underneath that: it proves the box is CONFIGURED so that client traffic can only
leave through the tunnel, and that the block survives the tunnel going away. The
section "What this suite does not prove" at the end lists what is still owed.

**Every scenario has been watched failing.** A test nobody has seen fail is a
green light wired to nothing. `TestEveryScenarioCanFail` injects a named defect
into each scenario and requires it to go red, so "this test can detect the thing
it claims to detect" is itself a test result rather than a promise. The defect is
named beside each behaviour below, under "Goes red when".

---

## The thing the product is for

### a pasted config brings the hotspot up and carries client traffic

A person pastes a share link into the panel and presses one switch. The config is
saved to disk where only this program can read it. The box then works out which
interface has the internet and which radio can make a hotspot, and picks
addresses that do not clash with the network it is already on. It puts the
firewall in place, starts the tunnel, brings up the access point and the DHCP and
DNS server for it, and reports itself connected, naming what it detected in one
line a person can check. Every rule that lets a client's traffic out names the
tunnel, and the only default route for client traffic points at the tunnel.

Goes red when: the forward chain gains an accept rule that does not name the
tunnel.

### the panel is reachable from the hotspot and never from a public address

By default the panel is served on the box's own address on the hotspot and on
nothing else. It is never bound to a wildcard, which would make it reachable from
whatever network the box is plugged into. If the user turns on "also on my local
network", one more address is added, and only if it is a private one. A box whose
attached network address is globally routable is refused rather than quietly
bound.

Goes red when: the panel falls back to the local network because the hotspot has
no address yet.

---

## Order, which is not a preference

Two orderings cannot be seen in the finished state. Only the sequence shows them,
and getting either wrong opens a window or silently misses the tunnel.

### the firewall is in force before the box will forward a packet

The moment the box is told to forward, it is a router. If the block is not
already loaded at that moment, there is a window in which client traffic is
forwarded with nothing stopping it leaving by the uplink. The firewall is also
loaded before the tunnel exists, which it must be able to do: the moment the
ruleset is needed most is the moment the tunnel is gone.

Goes red when: the ruleset is loaded after forwarding has been enabled.

### everything that needs the tunnel device waits for the engine to make it

The tunnel device is created by the engine, and every command that names it fails
if it runs first. The value a new interface is created with is set beforehand, so
the tunnel device starts life with it.

Goes red when: the tunnel steps are applied before the engine has created the
device.

### the engine's own connection to the server never enters the tunnel it is building

The engine has to reach the user's server over the ordinary internet connection.
Once a default route through the tunnel exists, that connection matches it and
the engine tries to reach its own uplink through the tunnel it has not built yet.
A route pinned to the server's address through the real gateway keeps the one
connection that must stay outside the tunnel outside it, and it is pinned before
the engine opens anything.

Goes red when: the pinned host route to the server is left out of the plan.

---

## Fail closed

When the tunnel device disappears the kernel withdraws every route through it and
traffic falls back to the ordinary connection. So "the tunnel dropped" produces a
leak by default rather than a stop, and the block has to be something that does
not depend on the tunnel existing.

### with the tunnel gone, nothing lets client traffic out by the uplink

Every rule in the ruleset that permits client traffic names the tunnel. Delete
every one of them, which is what their stopping matching amounts to, and what is
left still blocks. The policy is drop, and an explicit rule naming only the
hotspot and the uplink drops client traffic heading for the uplink. The box
never rewrites client addresses on the way out to the uplink, which is the single
line that would quietly turn it into an ordinary router. Every interface is
matched by NAME and never by index. An index is resolved when the ruleset loads,
and a ruleset that cannot load while the tunnel is down is absent exactly when it
is needed.

Goes red when: the forward chain gains an accept rule that does not name the
tunnel.

### clients are never offered the IPv6 the tunnel cannot carry

There is no IPv6 tunnel. A device with a working IPv6 path prefers it over IPv4
and would bypass the tunnel entirely. Three separate things stop that: the box
does not forward IPv6 at all, the firewall drops forwarded IPv6 on the hotspot in
both directions, and the box never advertises an IPv6 prefix, so a device cannot
give itself an address in the first place.

Goes red when: the IPv6 drop rules are taken out of the forward chain.

### a client's DNS question is answered on this box and resolved through the tunnel

The scenario below is about the firewall: it stops a device addressing a resolver
other than this box. This one is about what happens to the question afterwards,
which is where a leak would actually have to occur.

The DHCP and DNS server forwards to a loopback address on this box and nowhere
else, and the engine has a listener on exactly that address and port. Inside the
engine, the queries that resolver makes are routed to the tunnel outbound, above
the rule that sends private addresses direct, so a resolver on a private address
is still reached through the tunnel rather than on the local network.

The port is checked across both artefacts rather than against a number written
in the test, because the two ends of this chain agreeing is the thing that
breaks quietly. If they drift, every joined device stops resolving while the
hotspot, the tunnel and the firewall all report healthy.

Goes red when: the resolver's own queries are routed direct instead of into the
tunnel.

### the box offers itself as the resolver and never names another

What a device is TOLD to use, as opposed to what it is permitted to reach. The
DHCP offer names this box, once, and no other resolver.

It gets a scenario of its own because getting it wrong is invisible. A device
that honours the offer would address a stranger. The redirect in the firewall
would rewrite those packets to this box anyway, so nothing on the wire would
ever look wrong, and the box would answer on that stranger's behalf
indefinitely.

Goes red when: the DHCP offer names a public resolver instead of this box.

### a client cannot reach a resolver of its own choosing

A device with a resolver hardcoded into it is answered by this box rather than
allowed out to reach the one it was told to use: DNS on the usual port is
redirected here, over both protocols. DNS over TLS is refused so the device falls
back to the port that is redirected, and DNS over QUIC is dropped. The resolver
this box forwards to is on this box, and it refuses to read the system's own
resolver list, so a lookup cannot leave beside the tunnel instead of through it.

DNS over HTTPS on port 443 is not distinguishable from other web traffic and is
carried through the tunnel like anything else. That is a limit of the design, not
an oversight.

Goes red when: the DNS redirect is taken out, leaving client DNS merely
permitted.

---

## Turning it off, and being killed

### turning the switch off returns every change the box made

Every change is written down, with how to undo it, before it is made. Turning the
switch off replays those in reverse, and afterwards nothing the box added is
still there. Every address, route and rule it added has been removed. Every
kernel setting it changed has been put back to the value it was READ as. The
firewall table it owns is gone, and the record itself is deleted. The two
processes it started are stopped, their pid files and the configuration files
generated for them removed, and any radio soft-block this box cleared on the way
in is put back. The configuration removal is not tidiness. The hotspot file
holds the WPA passphrase, and a credential should not outlive the thing that
needed it. The radio is put back only for devices this box recorded as blocked
before it unblocked them, and only if they are still in the state it left them
in, so a radio the user switched on themselves is left alone.

One change has no inverse on purpose and is checked to be the only one: bringing
the hotspot interface up. Taking a radio down on the way out is worse than
leaving it up, because the machine's own WiFi, and the panel the user is reading,
may be on it.

Goes red when: a routing change is made without first recording how to undo it.

### a teardown replayed from the journal of a killed process undoes the same changes

The record lives on disk, not in memory, so it survives the process being killed
or the power being cut. A restarted box reads it and undoes the same set of
changes, with the same result: nothing left applied, and the record deleted.

Goes red when: a routing change is made without first recording how to undo it.

### a change of uplink leaves the box blocked and waiting for a reconnect

The internet moving, because a cable was unplugged or an address lease renewed
differently, is a change nothing fails loudly for. The pinned route to the server
still exists and still points at an address that is no longer a way out.

**The box does not notice, and this is the honest statement of a limit rather
than a promise.** Nothing polls the uplink. The tunnel stops carrying traffic and
stays stopped until somebody presses connect again, which re-reads the machine and
re-pins the route through the gateway that now has the internet.

What that costs is availability, not privacy. Client traffic stays blocked
throughout: the forward chain's policy is drop and every accept in it names the
tunnel, so a packet from the hotspot heading for whichever interface now carries
the internet is dropped by the policy. The explicit leak block naming the old
interface stops matching and nothing depends on it doing so.

Until 2026-08-30 this section said the opposite, in both halves: that the box
notices and moves the route, and that a ruleset naming the old interface "stops
blocking the moment traffic starts leaving by the new one". `netcfg.WatchUplink`
and `Plan.RederiveForUplink` exist and work, and no shipped code calls either.
The only caller was the scenario step below, which performed the move itself and
then asserted it had happened. Both halves are now assertions.

Goes red when: the forward chain accepts client traffic that does not name the
tunnel, so a ruleset naming the old uplink stops blocking when the internet moves.

### a box killed halfway through cleans up before it does anything else

A box that died mid-change comes back with a record of what it was doing. It
replays that record BEFORE it looks at the machine or applies anything new,
because a plan built against a machine that still holds a leftover rule is a plan
built against the wrong machine.

Goes red when: the record left by the killed process is not replayed at start.

---

## When something is wrong with the config

Three failures look identical to a user and need three different actions from
them: fix the text, get a new config, or check the internet connection. One "it
did not work" makes all three look like the same problem, and it is usually the
third.

### text that is not a link at all is refused before anything is touched

The box says it could not read the link, and tells the user what to do. Nothing
on the machine has been changed, and no record of changes has been written: a box
that reconfigured its firewall and then said "I could not read that" has changed
the network for nothing.

Goes red when: the box looks at the machine before it reads the pasted text.

### a link the engine will not accept is told apart from one that would not parse

Some links read perfectly and still cannot be used. The commonest is a link for a
server with its own certificate. The setting it needs was removed from the
engine, and no amount of re-copying will help. The box says that the link was
read and cannot be used as written, which is a different sentence from "could
not read it", and again nothing on the machine has been touched.

Goes red when: every failure is worded as a failure to read the pasted text.

### a link whose server never answers is not blamed on the link

The text was fine and the software took it. The box says so, and points at the
machine's own internet connection first, because that is the thing the user can
check and the thing most often at fault. Blaming the config first is what makes
somebody throw away a config that was never broken. Meanwhile clients get
nothing, rather than a way out around the tunnel.

Goes red when: every failure is worded as a failure to read the pasted text.

---

## Secrets

### the pasted credential never reaches a screen, a log or a readable file

Everything the flow produces is searched for the pasted credential: every error,
every message shown to the user, the appliance's own log lines, the panel's
description of the config, the saved settings as they render for diagnostics, the
generated firewall, the generated DHCP and DNS configuration, the engine's own
log, the record of applied changes on disk, and the request that crosses from the
unprivileged panel to the privileged service. It is in none of them.

It IS in the two places it has to be, and that is checked too, so this cannot pass
by the config having gone missing: the configuration document handed to the
engine, and the settings file, which is readable only by this program.

Goes red when: one log line records the config as it was pasted.

### the hotspot password reaches the access point and nothing else

The WiFi password is in the access point's own configuration, because otherwise
no device could join. It is not in the DHCP and DNS configuration, which is
readable by others on the box, and printing the panel's hotspot request in any of
the usual ways does not print it.

Goes red when: one log line records the hotspot password.

---

## Pressing the switch twice

### pressing connect twice does not restart a working hotspot

The panel's switch, a reconnect after a drop, and a health check that decides to
repair all reach the same code. Restarting a working access point disconnects
every device on it. So a second connect starts no second tunnel, restarts
neither of the two hotspot processes, and signals nothing.

Goes red when: the hotspot configuration is regenerated differently on every
connect.

---

## Machines that cannot do the job

### a machine whose radio cannot make a hotspot is told what to go and buy

Not "no AP-capable phy". The message names the thing that is wrong and the action
that fixes it, in words the audience uses, and the refusal keeps its reason all
the way to the panel rather than arriving as a generic failure.

Goes red when: a refusal loses its typed reason on the way to the panel.

### a machine with no internet connection of its own is told to plug something in

A box with nothing to share says so, and says what to do about it.

Goes red when: a refusal loses its typed reason on the way to the panel.

### the hotspot takes an address range that does not clash with the network the box is on

An address range that collides with the network the box is plugged into, or with
the tunnel's own addresses, produces devices that reach nothing while every
indicator says healthy. The range is chosen to avoid both.

Goes red when: the hotspot subnet is overridden to one the box is already on.

---

## What the box asks the internet for

### the box needs no download and asks no Google server anything

The generated configuration names no resolver belonging to Google, in the engine
or in the DHCP and DNS server. It also uses no rule that would need a geography
data file, because that would reintroduce a download to a product whose whole
install story is one verified binary.

Goes red when: a Google resolver is put into the generated configuration.

---

## What this suite does not prove

This list matters as much as the one above. Each item is a claim the product
makes that no scenario here can check, and why.

**No traffic goes anywhere.** There is no network, no radio, no root and no
tunnel device. No exit IP is captured, so nothing here satisfies the design's own
standard for calling something working (section 6). The strongest statement any
scenario makes is about configuration and ordering.

**The tunnel device is never created.** The engine runs for real, in process,
loading the real configuration through the real loader, so "the engine accepted
this config and started" means what it says. But its tunnel inbound is switched
off, because a developer machine has no `/dev/net/tun` and no root. The ordering
rule "everything that needs the tunnel device waits for the engine" is therefore
checked against a marker emitted at the moment the engine starts, not against a
device appearing. Whether the tunnel inbound behaves at all on Raspberry Pi OS is
recorded as UNKNOWN in the design (section 4.6) and is still unknown.

**This suite restates the order rather than driving it, and that is now a
choice rather than a necessity.** The paragraph that used to be here said
`cmd/caspian` was an empty directory and `internal/privsvc` did not compile and
had no `Start` method. That was true when it was written and was false by the
time it was last edited: `019fba6` added both on 2026-08-30, and the sentence
survived a commit five hours later whose subject was correcting stale documents.

What is true: the order the packages are used in is stated twice, once by
`internal/privsvc.Service.Start` and once by `test/bdd/appliance_test.go`, and
the second does not drive the first. Two statements of one sequence can
disagree, and nothing here would notice. `internal/privsvc` has its own tests
for its own ordering. What no test covers is the two agreeing.

**The clock check does not exist yet.** The design makes it step 5, before
anything that handshakes, and warns that a wrong clock produces an authentication
failure the panel will blame on the config (sections 8 and 9). There is a fault
code and a sentence for it in the panel, and nothing that raises them. One
scenario here is affected by the same trap from the other side: which configs the
engine ACCEPTS depends on the date, so the scenario about a link that cannot be
used as written fails loudly, with an explanation, on a box whose clock reads
before June 2026.

**The panel's screens are not driven.** What is checked is the wording, the three
failure states, the line naming what was detected, and where the panel is allowed
to listen. Nothing here opens a browser, logs in, or presses anything. The
first-run password, the join QR code and the advanced-mode overrides are not
covered.

**Nothing survives a reboot here.** Settings are written to disk and read back
within one run. No scenario stops the program and starts it again, so "survives a
reboot with no terminal" (design step 8) is not shown.

**Pressing connect twice re-applies every network change.** The scenario above
covers what was asked of it, which is that no second tunnel starts and the
hotspot is not restarted. It does not cover the network half. The second connect
runs every address, route and rule command again, and against a recorder every
command succeeds. On a real box some of those would fail because the thing
already exists. Whether that is intended is not documented anywhere, so no
scenario asserts either answer.

**Client isolation is a rule, not a measurement.** The ruleset stops one device
on the hotspot reaching another. The suite checks that the rule is present. It
does not check that the rule works.

**The installer and uninstaller are not covered.** `install.sh` and
`uninstall.sh` are shell, and a Go suite is the wrong tool. The uninstall path
replays the same record of changes that the teardown scenarios above do exercise.

**The machine is described by authored bytes.** The output of `ip`, `iw` and
`sysctl` that these scenarios read is written in the shape of a real capture, not
captured. Real captures from the target hardware live in
`internal/netcfg/testdata/`, with their provenance, and that package's own tests
are what prove the parsers read them. If the real output changes shape, those
tests catch it and these do not.
