# Caspian-BYOC FAQ

> ### [فارسی: پرسش‌های پرتکرار](FAQ.fa.md)
>
> **[Read this in Persian](FAQ.fa.md)**

The questions people actually hit. Every answer names the file, the test, or the
fault code it comes from, so you can check it rather than take it.

The panel carries a shorter version of this on its own help page, in Persian and
English, served from the box with no internet. `README.md` is the long form.
`docs/DEFECTS.md` is what is known and not fixed.

- [Before you start](#before-you-start)
- [It will not connect](#it-will-not-connect)
- [The hotspot](#the-hotspot)
- [The controls](#the-controls)
- [The panel and its password](#the-panel-and-its-password)
- [What it protects, and what it does not](#what-it-protects-and-what-it-does-not)
- [Checking any of this for yourself](#checking-any-of-this-for-yourself)

---

## Before you start

### What is this for?

You have a proxy config that works. You want the phones and laptops in the room
to use it, without installing anything on them. Caspian connects with that
config and shares the connection as a WiFi hotspot.

### Can I install it with the one-line command in the docs?

Yes. The installer works out your architecture, downloads the matching binary
from the latest release, and verifies it against the published `SHA256SUMS`
before installing anything. It refuses on a mismatch rather than installing an
unverified binary.

Read the script before piping it into a shell. For software of this kind that
is not a formality, and the script is written to be read:

    curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh | less

Building it yourself is still fully supported and needs no download at all. See
"Installing" in `README.md`.

### What kind of link can I paste?

Seven schemes, from `supportedSchemes` in `internal/link/link.go`: `vless`,
including REALITY, plus `vmess`, `trojan`, `ss`, `socks`, `hysteria2` and `hy2`.

Anything else is refused before the parser runs, including `tuic`, `ssr`,
`wireguard` and `anytls`. `checkScheme` names the scheme it found, and the panel
says which kinds of link this box can use rather than failing later.

Accepted is not the same as proven. `README.md` has three tables that keep the
three claims apart: what the parser accepts, what has carried bytes through a
real server on loopback, and what has been proven on hardware with an exit
address captured.

### Can I paste a QR code image?

No. The panel takes pasted text only. `internal/panel/qr` is an encoder, and no
handler in `internal/panel` reads an uploaded file. The QR code the panel shows
is the one your phone scans to join the hotspot.

### What hardware do I need?

A Raspberry Pi, and two network interfaces in one of two arrangements. Either
the internet arrives on Ethernet and the built-in WiFi becomes the hotspot, or
the internet arrives on the built-in WiFi and a USB adapter that reports AP
support becomes the hotspot.

The first arrangement is the one that has been measured. The second has never
been run on real hardware: the target has one radio and no USB adapter, so every
fixture for it is authored rather than captured.

---

## It will not connect

### The panel says it could not read the link

That is the first of three states, and it means the text never parsed. The panel
then says which part failed, and each one has a different action. `ParseProblem`
in `internal/panel/words.go` maps the parser's error onto the message, and the
wording lives in `internal/panel/i18n_messages.go`.

| Parser error | Message key | What to do |
|---|---|---|
| `ErrEmpty` | `problem.parse.empty.*` | Paste the whole config, then press Add |
| `ErrUnsupportedScheme` | `problem.parse.scheme.advice` | It is a kind of link this box does not carry |
| `ErrNoLink` | `problem.parse.nolink.advice` | Copy the whole thing again, first character to last |
| `ErrBadUUID` | `problem.parse.uuid.advice` | The link was copied in two pieces. Copy it in one go |
| `ErrBadAddress` | `problem.parse.address.advice` | It names no server. Ask for a fresh copy |
| `ErrBadPort` | `problem.parse.port.advice` | It has no usable port. Ask for a fresh copy |
| `ErrBadReality` | `problem.parse.reality.advice` | REALITY material was lost in copying. Ask for a fresh copy |
| `ErrUnsupportedTransport` | `problem.parse.transport.advice` | The transport was removed from the engine. Ask for a link without it |

The keys are named rather than the sentences quoted, because the sentences get
reworded and a quotation of one rots silently.

Nothing on the machine has been touched at this point. A box that reconfigured
its firewall and then said "I could not read that" would have changed the
network for nothing.

### The panel says it read the link and it cannot be used as written

That is the second state: `FaultEngineRejectedConfig`. The text parsed, and the
engine refused the document built from it. Re-copying will not help. Advanced
mode shows the engine's own message, already redacted.

One common cause is a setting the engine has removed. `allowInsecure` is the
example to know: in xray-core v1.260327.0 a config carrying it does not load at
all once the wall clock is past 2026-06-01. Note that it is the clock and not
the version that decides, which matters on a box whose clock can be wrong on
boot.

### The panel says the server did not answer

That is the third state, and it is the commonest. The link is fine.

1. Make sure that the box still has its own internet connection.
2. Try again.
3. If the box has internet, the server can be switched off, or the person who
   gave you the config can have moved it.

Blaming the config first is what makes somebody throw away a config that was
never broken. That is why these three states have three different sentences.

### Why does my transport work through a CDN and fail when I point it at the server directly?

Almost always a certificate name mismatch, and the fix is on your side. The
engine reports it like this:

    transport/internet/httpupgrade: failed to dial request ...
      tls: failed to verify certificate: x509: certificate is valid for
      <the apex>, not <the cdn subdomain>

A share link carries two names that people assume have to match and do not:

    sni   the name TLS validates the certificate against
    host  the name the server routes the request on, an HTTP header

Failing links usually carry the CDN's name in both. Through the CDN that works,
because the CDN holds a certificate for that name. Pointed straight at the
origin it cannot, because the origin holds a certificate for the apex only.

Set `sni` to the name the certificate actually carries, and leave `host` as the
name the server routes on:

    sni=example.com          host=cdn.example.com

MEASURED on 2026-08-30: two links that had failed with that error both connected
after this one change, with exit addresses captured from two independent sources
and matched to their own servers.

To see what a server presents before you guess:

    openssl s_client -connect <address>:443 -servername <name>

Refusing the mismatched certificate is the behaviour you want. Accepting it
would mean the tunnel could be terminated by anything holding any certificate.
No server change is needed.

### The panel says the clock is wrong

`FaultClockImplausible`. This is not a problem with your config, and the panel
says so, because a Pi has no battery clock.

Leave the machine connected to the internet for a minute so it can set its
clock, then try again.

The check runs before validation, not only before the handshake. Which configs
the engine accepts depends on the date, so a box with a wrong clock accepts a
config the same binary rejects once the clock is corrected. See
`internal/privsvc/clock.go` and `TestClockFailureIsNotBlamedOnTheConfig`.

---

## The hotspot

### The hotspot is on, but my phone cannot see the network

Three causes, in the order worth checking.

**The country code.** hostapd with no `country_code` falls back to the world
regulatory domain, where most channels are passive-scan only and beaconing is
not permitted. The access point starts and no network ever appears. Caspian
refuses to start rather than reach that state, and takes the two-letter code
from your advanced settings or from what the radio reported. A wrong code can
still silence the radio, so check it in advanced settings.

**hostapd was alive and not beaconing.** MEASURED on the target on 2026-08-30.
The service logged itself running with a hotspot on `wlan0` while `wlan0` was
still a station on the house network. hostapd was a live process whose control
socket did not answer. A phone in the room listed eleven networks with ours not
among them. Every command had returned success.

That is now caught rather than reported as working.
`netcfg.AssertHotspotInterfaceReleased` proves the interface is free before
anything binds to it, and `AssertHotspotIsAccessPoint` reads back an access
point broadcasting the expected name before the service says it is running. So
a box in that state now says it failed, and says which half failed.

**The band and the channel.** If one adapter has to do both jobs, the hotspot
must share the channel of the network the box uses for its own internet. The
panel says so, and the channel cannot be chosen. A USB WiFi adapter removes the
limit.

### The panel says the hotspot would not start

`FaultHotspotFailed`. Restarting the machine is the first thing to try. If it
comes back the same way, look at the channel, the band and the country code in
advanced settings, in that order.

### The panel says the WiFi adapter is busy

`FaultHotspotInterfaceBusy`. The adapter Caspian needs for the hotspot is still
joined to another network, and Caspian stopped rather than take it over.

Disconnect that network on the machine, or plug in a USB WiFi adapter, then
switch Caspian on again. Restarting the box does not help: it boots, the network
manager rejoins the same network, and the refusal is identical. That is why this
fault is separate from "the hotspot would not start", whose advice is to
restart.

### Devices see the network and cannot join it

`FaultDHCPFailed`. The access point started and the part that hands out
addresses did not. Restarting the machine is the first thing to try.

### Why did the Pi lose its own WiFi when I switched Caspian on?

Because on the measured hardware one radio cannot do both jobs, and the driver
refuses to pretend otherwise. `brcmfmac` refuses
`iw phy phy0 interface add ap0 type __ap` with `Input/output error (-5)`, even
though `iw list` advertises the combination.

So Caspian falls back to taking `wlan0` over: it releases the interface from
NetworkManager, strips the address it holds on the house network, and retypes
it. The panel and the log say what that costs before it happens. Test:
`TestTheTakeoverSaysWhatItCost`.

You get it back. Every change is written to `/var/lib/caspian/netcfg.journal`
with its inverse before it is made, and switching off replays them in reverse.
Measured on the target: the four commands went out, and the inverses put the box
back on its own network with its own address eight seconds later.

### I am joined to the hotspot and there is no internet

That is what you are meant to see when the tunnel is down. It is the leak being
blocked, not a bug.

1. Look at the top of the panel. If it says client traffic is cut, somebody
   pressed the cut control, and the button beside it puts it back.
2. If it says the server did not answer, read that answer above.
3. If it says connected and nothing loads, the tunnel is up and the server is
   the thing to suspect. Try another config if you have one.

The reason the failure looks like this is in `internal/netcfg/nftables.go`. The
forward chain's policy is drop, and every rule that lets client traffic out
names the tunnel device. When the tunnel goes, those rules stop matching and the
policy drops everything.

### It is slow

All traffic goes through one connection and one server, so the speed depends on
that server and how far you are from it. If you have more than one config, try
another. Changing config takes a few seconds and reconnects on its own.

---

## The controls

### What do the two controls do, and which one can I press from a phone?

|  | The switch, `POST /power` | The cut, `POST /cut` |
|---|---|---|
| Tunnel | stopped | left running |
| Hotspot | stopped, the network stops existing | left running |
| DHCP and DNS on the hotspot | stopped | left running |
| Joined devices | disconnected, including your phone | stay joined, keep their addresses |
| The panel from the hotspot | unreachable | reachable |
| Network changes to the box | all undone from the journal | none |
| Survives a restart of the box | the box comes back off | no, the cut is cleared |

**Press the cut from a phone.** It is the emergency stop that does not strand
the person using it. It is immediate, it asks for no confirmation, and undoing
it costs no reconnection, because nothing your device was attached to went away.

**Do not press the switch from a phone on the hotspot.** Switching off removes
the WiFi network you are reaching the panel over.

### Why does the switch strand me and the cut does not?

The panel binds to the hotspot address by default and to nothing else. Serving
it on the network the box itself sits on is a setting you have to turn on, and
it is off in the shipped default. See `internal/panel/listen.go`, `BindAddrs`,
and `internal/state/state.go`, `PanelOnLAN`.

The cut changes the forward chain and nothing else.
`TestForwardCut_DiffersFromNormalOnlyInTheForwardChain` compares the input,
output, prerouting and postrouting chains line for line. The panel is traffic to
this box, not traffic through it, so it survives.

### I switched it off from my phone and now I cannot reach the panel

You need another way to the box. One of these:

1. A device on the network the box itself sits on, if you turned on "also on my
   local network" in advanced settings beforehand. It is off by default.
2. A keyboard and screen on the box, or an SSH session.

There is no third way, and that is the reason two controls exist rather than
one.

### The cut says it is refused because Caspian is not switched on

`FaultNotRunning`, and it is telling you exactly what happened. The panel keeps
the control out of reach when the box is off, so this is the stale-tab case:
the page was left open, the appliance was switched off elsewhere, and the button
was pressed.

There is no forwarding to stop. And a ruleset that names a hotspot interface
which does not exist is a change made to a machine whose whole invariant while
off is that it was left as it was found.

### Does restarting the box put my traffic back?

It clears the cut and leaves the box switched off. The cut is held in memory and
written to no file, so a restart loses it. That is deliberate: somebody who
cannot work out why their internet stopped gets it back by pulling the plug.

What a restart does not do is switch the appliance on. The privileged service
replays the journal at startup and starts nothing. Traffic flows again once you
press the switch, not before. Tests: `TestACutIsNeverWrittenDown`,
`TestACutDoesNotSurviveARestart`.

### What does the recovery button do?

`POST /recover`. It stops everything, replays the teardown journal so every
interface, route and firewall rule this appliance changed is put back, and then
starts again from your saved settings. It is the way out of a stuck box without
a reboot and without a terminal.

It does not restart the machine, and it does not restart either systemd unit, so
the panel process and any SSH session stay up. It does stop the access point and
start it again, so a device joined to the hotspot leaves the network and rejoins
when the hotspot returns.

It exists because of a measured day. On 2026-08-30 the appliance repeatedly
reached states that only somebody with an SSH session could clear: an interface
created by a failed start and never removed, an address flushed out from under
it, a journal entry that survived a failed start. Every one was recoverable from
what was already written down, and none of it was reachable from the panel.

### I changed a setting and nothing happened

Advanced settings are read when the appliance starts, and the panel says so
after it saves them. See `notice.advancedsaved` in
`internal/panel/i18n_messages.go`.

Pasting a new config or renaming the hotspot is different. Both stop and restart
the appliance on their own when it is running, because leaving it on the old one
would make the panel say one thing while the box does another.

---

## The panel and its password

### I cannot reach the panel

Open it from a device joined to the hotspot. While Caspian is on, the panel is
always reachable that way, even while client traffic is cut.

Reaching it from the network the box itself sits on is off by default. Turn it
on in advanced settings, before you need it. Even then it is refused unless the
box's address on that network is a private one: a box whose attached network
address is globally routable is refused rather than quietly bound.

### I have forgotten the panel password

There is no way back from inside the panel, deliberately. Any recovery path is
also a way past the password. You need root access to the machine.

Re-running the installer on a box that already has a state file keeps the
existing password, and says so: "Existing state found; keeping the current panel
password." See `seed_first_run_password` in `install.sh`. Setting a fresh
password means removing `/var/lib/caspian/state.json` first, or running
`bash uninstall.sh --purge` and installing again.

Either way you lose the saved proxy config and the hotspot name and password,
because all three live in that one file.

### Does the panel ever show my config back to me?

No. Not on any path, including the failure paths. The pasted text is never
echoed into a response, never written to a log line, and never put in a URL.
What advanced mode shows is a description built by `internal/link`: the
protocol, the server, the transport, the security layer, and whether each
REALITY value is present. Never a value.

The tests that hold it are named in `README.md` under "The pasted config never
reaches a screen, a log or a readable file". The one that matters most is that
the same tests check the config IS in the two places it has to be, so they
cannot pass by the config having gone missing.

### Does the panel load anything from the internet?

No. Every stylesheet, script and icon is compiled into the binary, and there is
no web font: the font stack is entirely system faces, Persian-capable ones
first.

The privacy reason is that a remote asset tells a third party the address of
everyone who opens the panel. The stronger reason is availability. The panel has
to load when the tunnel is down, which is exactly when somebody needs it.

---

## What it protects, and what it does not

### If the tunnel drops, can my devices leak?

Not by the uplink, and the block does not depend on the tunnel existing. The
first rule in the forward chain names only the hotspot and the internet
connection:

    iifname "wlan0" oifname "eth0" drop comment "fail-closed: client traffic never leaves by the uplink"

Every rule that permits client traffic names the tunnel, so the tunnel going
away removes the permits and leaves the drop. Every interface is matched by name
and never by index, so the ruleset loads with no tunnel present.

### Where do my devices' DNS lookups go?

To this box, and then through the tunnel. Port 53 from the hotspot is
redirected, not merely permitted. So a device with a resolver hardcoded into it
is answered here, rather than allowed out to reach the one it was told to use.

dnsmasq on the box forwards to one loopback address and nowhere else, and the
engine answers there. Inside the engine those queries are routed to the tunnel
outbound, above the rule that sends private addresses direct, so a resolver on a
private address is still reached through the tunnel.

DNS over TLS on port 853 is rejected with a TCP reset, so the device falls back
to the redirected port. DNS over QUIC on 853 is dropped.

### What about DNS over HTTPS?

It is carried, not blocked, and nothing here can see it. On port 443 it is not
distinguishable from any other HTTPS. A client using it is inside the tunnel and
is not leaking. It is also invisible to this project and to the hardware
harness. That is a limit of the design, and it is stated in the generated
ruleset itself as well as here.

### Does it carry IPv6?

No. There is no IPv6 tunnel, so IPv6 is blocked for joined devices. A device
with a working IPv6 path prefers it over IPv4 and would bypass the tunnel
entirely. The box does not forward IPv6, the firewall drops forwarded IPv6 on
the hotspot in both directions, and router advertisements towards the hotspot
are dropped so a device cannot give itself an address.

Read the limit with it. Every hardware result this project has is an IPv4
result, because there was no IPv6 on the network the box and the phone were on.
An IPv6 leak check run there would have passed without the appliance doing
anything. On a network with working IPv6, treat this as a new question rather
than a covered one.

### Is the box's own traffic protected?

No, and that is by design. The engine has to reach your server over the ordinary
internet connection or there is no tunnel at all. The promise is about forwarded
client traffic.

The output chain narrows it and does not close it. While the appliance is on,
the box can reach the network only for the things the chain names: renewing its
own address, setting its clock, resolving names, and reaching your server, which
is permitted by address rather than by port so a UDP-on-443 transport is not
broken silently.

Two residuals are stated in the ruleset's own header. Anything on the box can
still reach the network on port 53, and the server's hostname is resolved in the
clear on the local network before any tunnel exists.

### Why does `apt update` fail while Caspian is on?

Because of the chain above. The cost is known and was accepted, and it is stated
in the generated ruleset's header rather than left to be discovered. Switch the
appliance off to update the machine.

### Can devices on the hotspot see each other?

The ruleset says no: `iifname "wlan0" oifname "wlan0" drop`. That the rule is
present is checked. That it works is not measured. Read it as a rule, not as a
result.

### What does it not protect me from at all?

- Your own devices. If an app on your phone gives you away, the tunnel carries
  that just as faithfully.
- Anybody with the WiFi password. The hotspot is WPA2, so treat the password as
  a key.
- Anything the server operator can see. All traffic goes to one server you were
  given by somebody.
- Traffic analysis. What your traffic looks like on the wire is decided by the
  transport in the link you were given, not by this box. Caspian composes the
  document around that outbound and re-serialises the outbound unchanged.

---

## Checking any of this for yourself

### How do I run the tests?

    bash scripts/gate.sh > gate.log 2>&1; echo "exit: $?"

That is gofmt, `go vet`, the whole suite with the race detector, and a
per-package coverage floor. Do not pipe it into `tail`, `tee`, `head` or `grep`:
a shell pipeline returns the status of its last command, and that trap has
produced a false green in this project before.

To run one behaviour on its own:

    go test ./test/bdd/ -run 'TestBehaviour/the_firewall'

`-count=1` is not optional on a repeat run. Without it a second run prints the
first run's PASS lines and exits 0 having executed nothing.

### How do I know a test can detect what it claims to?

Because it has been watched failing. `TestEveryScenarioCanFail` injects a named
defect into each behaviour and requires it to go red. `TestEveryCarriageProofCanFail`
does the same for the protocol carriage suite, and the leak scanner in
`test/goldenscan` has been watched catching a planted secret of every class it
knows.

### How do I prove a transport actually carries traffic?

Run the hardware harness. `test/hardware/caspian-hw` with the runbook in
`docs/HARDWARE-TEST.md`. Its standard is the project's own: a connect is not a
result. The exit codes say it plainly. PASS is 0, UNPROVEN is 1 and is not a
pass, FAIL is 2, LEAK is 3, PRECONDITION is 4, and VOID is 5.

A dry run exits 4 and never 0. It measured nothing, so it has no verdict.

### What is known to be broken or missing?

`docs/DEFECTS.md`, with what was measured, what it costs, and what would close
each one. None of them is a leak of client traffic. The summary is in
`README.md` under "Open defects". The shortest version:

- nothing re-asserts the firewall once it is loaded.
- a service that is killed rather than stopped does not re-block a radio it
  unblocked.
- a hotspot interface this program created is not released from NetworkManager.
- a stop that could not undo anything still reports success, while the box stays
  fail-closed.
- the uninstaller replays the journal by its own rules, without the rule that
  holds the firewall's inverse.
