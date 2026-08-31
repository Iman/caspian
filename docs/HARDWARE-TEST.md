# Hardware test: proving the appliance with a real phone

> Persian edition: [`docs/HARDWARE-TEST.fa.md`](HARDWARE-TEST.fa.md). The
> English file is the one the tests read. If the two ever disagree, this one is
> correct.

Written 2026-08-30, before the session it is for. Everything marked MEASURED was
run against the attached hardware while this was written. Everything marked
UNVERIFIED was not, and says what would settle it.

The harness is `test/hardware/caspian-hw`. This file is how to use it and how to
read what it says.

## The standard

A connect is not a result. A green switch is not a result. An interface being up
is not a result.

A transport is proven when real traffic has traversed it AND the exit IP has
been captured and matched to the server the config names. An exit IP equal to
the untunnelled baseline is a leak, and it outranks every other result in the
run. If the tunnel, the app or the phone changed state during a capture, the
reading is VOID and must be retaken. It is not a leak and it is not a pass.

`docs/2026-08-29-design.md` section 6 puts it in one line: nothing is called
working without an exit IP captured from real traffic.

## What to connect

| Thing | How | Why it must be that way |
|---|---|---|
| The phone | USB cable to the machine running the harness | The fail-closed step takes the phone's network away. adb over that same network dies with it and the run reads as a device fault |
| The phone's WiFi | Joined to the Pi's hotspot BY HAND, once | MEASURED: this handset has no scriptable join. `cmd wifi` offers `set-wifi-enabled`, `list-networks`, `forget-network` and `add-suggestion`, and no `connect-network`; `svc wifi` offers only enable and disable |
| The Pi | Reachable over ssh, key-based, sudo without a password | The DNS check and the firewall check both run there. `ssh $PI_SSH 'sudo -n tcpdump --version'` must work |
| USB debugging | On, and the host authorised | Everything here is adb |

## What to put in `/local/` first

`/local/` is gitignored (`.gitignore:25`, the `/local/` rule). Nothing the
harness reads or writes can reach a commit, and `hw_new_run_dir` proves that
with `git check-ignore` rather than trusting it.

```
local/configs/<label>.conf    one share link, one file, one line
local/boxes.tsv               <label> TAB <addr>[,<addr>...]
local/box.env                 copy test/hardware/box.env.example and fill it in
```

The label is the filename with `.conf` removed, and it is the ONLY name that
ever reaches a log, a summary or a directory name. Keep it plain: the harness
refuses a label containing anything outside `A-Za-z0-9._-`, and refuses one
containing something shaped like an IPv4 address, because labels get written
into paths and addresses must not.

`boxes.tsv` is optional and worth having. It is the authority for the box NAME,
and it is the only way to express a server whose EGRESS address differs from the
address in its link, which is normal for a multi-homed box or a CDN ingress.
Without a row, a working tunnel whose egress differs from its ingress is scored
FAIL. The harness cross-checks the parsed host against the row and warns when
they disagree, so a stale row is visible rather than silent.

## Which configs are in scope

The engine is xray-core, so the scope is what the vendored parser handles. The
harness mirrors `internal/link/link.go`'s `supportedSchemes` exactly, and
`caspian-hw selftest` re-reads that Go source and fails if the two lists have
drifted apart.

**In scope:** `vless` (Reality included), `vmess`, `trojan`, `ss`, `socks`,
`hysteria2`, `hy2`.

**Out of scope, refused by name:** `tuic`, `ssr`, `wireguard`, `anytls` and
anything else. `docs/2026-08-29-design.md` section 4.4 records that the vendored
parser does not cover them. The harness says which scheme it refused and does
not quote the link.

Check before the session starts:

```
test/hardware/caspian-hw configs
```

## The order to run things

```
test/hardware/caspian-hw selftest        # offline. Proves the harness itself
test/hardware/caspian-hw preflight       # changes nothing. Reads the phone and the box
test/hardware/caspian-hw baseline        # phone on its ORDINARY network
#   >>> now join the phone to the hotspot by hand <<<
test/hardware/caspian-hw switch  <first> <second>
test/hardware/caspian-hw dns-leak
test/hardware/caspian-hw fail-closed --wifi-only <second>
test/hardware/caspian-hw leakscan        # prove nothing private reached the output
```

or the whole thing in one go, which runs the same steps in the same order and
pauses once for the hotspot join:

```
test/hardware/caspian-hw --wifi-only all <first> <second>
```

`baseline` opens a run directory and points `local/hardware-runs/current` at it.
Every later command lands in that run and is graded against that baseline.
**A run with no baseline is not a run**, and the later commands refuse to start
without one.

## Exit codes

The words and the numbers are the same thing, so an unattended wrapper and a
person reading the log agree.

| Code | Word | Means |
|---|---|---|
| 0 | PASS | Real traffic traversed the tunnel and the exit IP matched the config |
| 1 | UNPROVEN | No exit IP was captured. NOT a pass. Also used for a single-source result |
| 2 | FAIL | An exit IP was captured and it did not match |
| 3 | LEAK | The exit IP equals the untunnelled baseline, with state stable throughout |
| 4 | PRECONDITION | The run could not start, or the harness was misused, or it was a dry run |
| 5 | VOID | The phone changed network state mid-capture. Retake it |

A dry run exits 4 and never 0. It measured nothing, so it has no verdict.

## What a pass looks like

```
RESULT PASS for 'reality-de'.
  real traffic reached the internet through the tunnel and came back naming the box.
  box: reality-de
  two independent sources agreed (Chrome over HTTPS, and nc over HTTP).
```

The box is named, not addressed. A matching exit IP **is** the config's server
address, so printing it would put a server address in a log. When the exit IP
matches nothing configured it is printed in full, because then it is a
measurement and not a config's secret.

`RESULT SINGLE-SOURCE` is not a pass and exits 1. The second source exists to
catch a cached or stale page. Without it, that check did not happen.

## What a leak looks like

```
############################################################
RESULT LEAK. The engine was stopped and traffic still left the box.
############################################################
```

or, from `prove`:

```
RESULT LEAK for 'reality-de'. This outranks every other result in this run.
  the phone's exit IP through the appliance is the SAME address it has with no
  tunnel at all.
```

Stop. Nothing else in the run matters. `docs/2026-08-29-design.md` section 7
says why this is the default failure mode rather than an exotic one: when the
TUN device disappears the kernel withdraws every route through it and traffic
falls back to the main table and out of the uplink, so "the engine stopped"
produces a leak unless a firewall rule that does not depend on the interface
existing stops it.

The rule that must stop it is the first one in the forward chain, quoted from
`internal/netcfg/testdata/golden-ruleset-captured.nft`:

```
iifname "ap0" oifname "eth0" drop comment "fail-closed: client traffic never leaves by the uplink"
```

The fail-closed step checks over ssh that the table is loaded and that this rule
is in it before it grades anything. If the Pi is not reachable it says, in
those words, that the firewall state was NOT checked and that a pass therefore
means only "the phone reached nothing".

## What to do when a result is VOID

VOID means the phone's `ssid`, `airplane` or `defaultnet` differed between the
sample taken before the capture and the sample taken after. The reading says
nothing about the appliance.

1. Do not record it. Do not report it as a leak, and do not report it as a pass.
2. Find out why. The common causes, in order:
   - the phone roamed to a remembered network.
   - the phone decided the hotspot had no internet and moved the default route
     to mobile data.
   - the screen locked and WiFi was put to sleep.
3. Fix the cause, then retake the same step. `caspian-hw prove <label>` can be
   re-run any number of times inside one run.

The fail-closed step is where this bites hardest, and it has its own guard.

## The cellular trap, which will produce a false leak if ignored

MEASURED on the attached handset: it has a SIM (`getprop gsm.sim.state` returned
`LOADED,ABSENT`, so slot 1 is populated) and mobile data was on
(`settings get global mobile_data` returned `1`).

When the hotspot stops reaching the internet, which is exactly what the
fail-closed step arranges, Android marks that network unvalidated and moves the
default route to mobile data. The phone then reaches everything, over LTE, and a
careless harness calls that a leak when the packets never went near the Pi.

`--wifi-only` removes the possibility rather than hoping. MEASURED, from the
shell uid with no root, and restored afterwards:

```
adb shell cmd connectivity airplane-mode enable    # airplane on, wifi goes down
adb shell cmd wifi set-wifi-enabled enabled        # wifi back up, still no cellular
adb shell cmd connectivity airplane-mode disable   # restored
```

Without `--wifi-only` the step refuses to grade unless airplane mode is already
on. That refusal is deliberate: a fail-closed result taken beside a live
cellular path cannot be told apart from a false leak.

## The two sources, and what each cannot see

### Both endpoints are pinned to IP addresses. Do not change them back to names

The first draft used `icanhazip.com` and `ifconfig.me`. MEASURED on 2026-08-30,
from the Pi and then confirmed from the phone: the resolver this LAN hands out,
**sinkholes IP-echo services**. From the phone, `ping` for
`icanhazip.com`, `ifconfig.me`, `api.ipify.org` and `checkip.amazonaws.com` all
answered `127.0.0.1`; the same names answered `::` from the Pi. Only
`ipinfo.io` survived, at `34.117.59.81`, from both vantages.


The resolver's address is deliberately not written down. It is a private
address, so the privacy scan treats it as identifying nobody, which is right
for a server and wrong for the resolver a particular household hands out. The
argument here is about the resolver's BEHAVIOUR and not its number, so the
number adds nothing and costs something. Removed 2026-08-31, the same day and
for the same reason as the residential exit address further down this file.

Two dead endpoints would merely be annoying. This is a **confounder that would
have produced a false pass**:

> Untunnelled, the phone uses that sinkholing resolver and every echo lookup
> fails. Tunnelled, DNS goes through the appliance's own resolver chain inside
> the tunnel and the same names resolve. So a box that changed nothing but the
> DNS server, and tunnelled no traffic whatsoever, would show exactly the
> signature this harness looks for: nothing before, an answer after. It would
> have been graded a pass.

Pinning the addresses removes DNS from the exit-IP question completely.

**A consequence worth having on purpose.** Because neither source resolves
anything, a DNS leak and a traffic leak are now independently observable. The
exit IP answers only "where did the packets go". The separate port-53 capture
answers only "where did the lookups go". Neither can mask the other any more.

The addresses are anycast and may move. That is handled by validating the
response SHAPE at runtime rather than by trusting the pin: an unrecognised shape
is reported UNPROVEN, never guessed at and never passed.

| | Address | Why |
|---|---|---|
| Source A | `https://1.1.1.1/cdn-cgi/trace` | TLS to a bare IP; the certificate carries the address in a SAN. MEASURED through Chrome on the handset: `ip=`, `loc=GB`, `warp=off`, `tls=TLSv1.3`, over http/2. The cache-busting query string is accepted |
| Source B | `http://34.117.59.81/ip`, `Host: ipinfo.io` | Plain port 80, which is all the phone's `nc` can do. Port 80 on `1.1.1.1` answers 301 and is useless here |

All three readings of the same address on 2026-08-30 agreed: source A through
Chrome, source B through nc, and an independent probe from the Pi.

### Source A: Chrome, over HTTPS. The one that matters

It is what a user actually runs, and it drags the whole path behind it: DNS,
TLS, HTTP, and Chrome's own opinions about all three.

Chrome is driven by intent, MEASURED working:

```
adb shell am start -a android.intent.action.VIEW -d '<url>' -p com.android.chrome
```

Getting text back out is the awkward part, and the harness has three ways,
tried in this order.

**1. DevTools, `Runtime.evaluate` on `document.body.innerText`.** MEASURED. The
`@chrome_devtools_remote` abstract socket is absent before Chrome runs and
present after; `adb forward tcp:19222 localabstract:chrome_devtools_remote` then
answers `/json/version` with `Chrome/148.0.7778.178`, and the evaluate returned
the page's text. Needs `python3` with either the `websockets` or the
`websocket-client` package. This traffic rides the USB cable, so it reads the
result of a fetch that went over WiFi and never becomes the fetch itself.

**2. `uiautomator dump`.** MEASURED, and the measurement is the reason method 1
exists. The FIRST dump after launching Chrome contained only a promotional modal
dialog (`com.android.chrome:id/modal_dialog_view`, "Chrome notifications make
things easier") and NEITHER the page body NOR the page title. After tapping its
`negative_button` the next dump held the body text, the title and the URL-bar
text. A harness that dumped once and found no IP would have said UNPROVEN about
a page that was fine. The harness detects the modal and dismisses it, and if it
finds a modal with no button it recognises it stops rather than reading the
dialog.

**3. A screenshot.** Not machine-read, kept so a person can settle an argument.
MEASURED: `adb exec-out screencap -p` returned 1,549,897 bytes on this handset,
so the screencap-returns-zero-bytes trap seen on emulators elsewhere does not
apply here.

**How source A's body is parsed, and the trap in it.** The response is
key-value lines, and the measured body begins:

```
fl=985f9
h=1.1.1.1
ip=<baseline>
```

`h` is the endpoint's own address and it comes FIRST. Reading "the first IP
literal in the body" therefore returns `1.1.1.1`, which is a **constant**: it is
the same whatever the tunnel is doing. Every config would report the same exit
address and the switch test would announce "the exit IP did not change" about a
box that had switched correctly. The harness reads the `ip=` field by name, and
`selftest/run.sh` pins both the correct read and the wrong one so the trap
cannot come back.

**What source A cannot see.** Whether Chrome resolved anything through its own
DNS-over-HTTPS ("Secure DNS"). With a pinned IP literal there is nothing for it
to resolve, so this no longer affects the exit-IP reading at all. It still
matters for the DNS check below.

Chrome cannot be given a clean tab remotely. MEASURED: `PUT /json/new` answered
`Could not create new page`. The cache defeat is therefore a unique query
parameter on every request, which needs no cooperation from anyone.

### Source B: `toybox nc`, over plain HTTP. Not Chrome

Its job is narrow: fetch the answer again, over the same WiFi, with a different
program, so a cached or stale page in Chrome cannot produce a false agreement.

MEASURED: this handset has NO `curl` and NO `wget`. `which curl wget` found
neither and toybox 0.8.6-android lists neither. `nc` is not a preference, it is
the whole of what is available. This form works:

```
adb shell '(printf "GET /ip HTTP/1.0\r\nHost: ipinfo.io\r\n\r\n"; sleep 5) | toybox nc -w 8 34.117.59.81 80'
```

and returned `<baseline>`. Two things about it are load-bearing.

The obvious form, a plain pipe with no `sleep`, returned NOTHING and still
exited 0, because nc sees EOF on stdin and leaves before the response arrives.
The `-q` flag did not fix it either. The sleep is the measured difference
between an answer and a silent empty string.

**The `Host` header is required.** MEASURED: the identical request without it
answers `HTTP/1.0 404 Not Found` with the body `fault filter abort`. ipinfo is
a name-based virtual host, so pinning the address is only half of reaching it.

**What source B cannot see.** TLS. toybox nc speaks TCP and nothing else, so
source B is plain HTTP on port 80 and a transparent proxy in the path could
rewrite it. That is exactly why it is the second source and not the first.

### Source C: the Pi's own SOCKS probe, when the Pi is reachable

`docs/LAYOUT.md`'s port table puts 10808 on 127.0.0.1 and says what it is for in
those words: "SOCKS, for diagnostics and the exit-IP proof". The harness uses
it as a third, phone-independent reading.

**What it cannot see.** Whether the PHONE's traffic used the tunnel. It proves
what the tunnel egresses as, right now, and nothing more. The harness never
lets it stand in for the phone's own reading. It reports a disagreement as a
warning that one of the two is not going where you think.

## The DNS check, and its one honest limit

While the phone browses, the harness captures on the Pi's uplink and asks one
question that has no attribution problem in it.

Counting port 53 packets does not work. The box resolves names itself, for the
server address and for time, and design section 7 puts the box's own traffic
outside the guarantee deliberately. Worse, a client query that DID escape would
be masqueraded on the way out and would look exactly like the box's own.

So the phone resolves a name that has never existed: a per-run random label
under `.invalid`, which RFC 2606 reserves. It is resolved twice, through Chrome
and through `ping`, because `ping` uses the OS resolver and cannot be diverted
by Chrome's Secure DNS. Then:

- **the label appears in cleartext on the uplink** -> a client query escaped.
  Nothing else on the network can have produced that label.
- **it appears zero times** -> no client query escaped in cleartext during the
  window.

**WHAT THIS CANNOT SEE, and it is printed in the result rather than hidden
here.** DNS over HTTPS on port 443 is indistinguishable from any other HTTPS and
is carried through the tunnel like anything else. A client using it is INSIDE
the tunnel and invisible to this check. That is correct behaviour and not an
oversight. The generated ruleset says so in its own words above its 853 rules.
Also unseen: any interface other than the uplink, and anything outside the
capture window.

**No packet capture leaves the Pi.** The tcpdump output is consumed by `awk` on
the box and two integers come back. A capture on that uplink is a recording of
the maintainer's own browsing, and no version of this harness is worth keeping
one.

## Privacy: what the harness writes, and how that is checked

Nothing the harness writes carries a config, a server address, a user id or a
key. Three mechanisms, not one:

1. **Registration.** Every share link in `local/configs/`, its userinfo, its
   host, its query values and every address in `boxes.tsv` are registered as
   redaction rules before any step runs, for EVERY config, not just the one
   under test.
2. **Substitution.** Every artefact is written through a filter that replaces
   them with `<box:label>`, `<credential:label>` and so on, and the harness
   re-reads the file afterwards. If a secret survived, it deletes the file and
   stops the run.
3. **The sweep.** `caspian-hw leakscan` re-reads every file and every path under
   a run and reports anything that got out by a route that skipped the filter.

The redaction table and the raw captured exit addresses live in a 0700 temporary
directory that is removed when the process exits, never in the run directory.
The run directory keeps salted fingerprints instead, which compare for equality
and cannot be read backwards.

This document used to print that address in two places, which is the same
disclosure the paragraph below argues against, made by the document that argues
it. Both are now `<baseline>`, the placeholder the harness already substitutes
into every artefact it writes. Removed 2026-08-31. It stays in git history, so
this stops it reaching new checkouts rather than unpublishing it. The value is
the maintainer's residential connection: it is not a key or a server, and it
identifies a person's home, which is a different kind of harm and not a smaller
one.

The one file written raw is `baseline.ip`, the phone's own untunnelled address.
It has to be, because every later verdict is a comparison against it. It is not
a config, a server address, a user id or a key, and it lives inside the
gitignored `/local/` tree. From the moment it is captured it appears in every
other artefact as `<baseline>`.

## Driving the box

The harness contains no `caspian` command line for applying a config or starting
and stopping the tunnel, because no such command line exists.

That was checked twice on 2026-08-30 and it changed in between, so the second
reading is the one that counts. Early in the session `cmd/caspian/` was empty.
Later the same session another agent had populated it, and the usage text in
`cmd/caspian/main.go` now offers exactly four things: `serve --privileged`,
`serve --panel`, `check` (read-only) and `version`. It ends with the sentence
"After the installer has run, everything a person does happens in the panel."

So there is still no subcommand that applies a config or drives the switch, and
the CLI says in its own words that there is not meant to be. The seam below
matches the product's decision rather than working around a gap. `cmd/caspian`
is under active edit, so re-run `caspian help` rather than trusting this
paragraph.

The privileged side has a closed action vocabulary in `internal/panel/priv.go`:
detect, status, start, stop and engine-log. That is a Go interface over a unix
socket whose wire format this session did not read, and the panel's own surface
is session-gated HTML forms with per-form tokens.

So control is a seam with two modes:

- **`manual`, the default.** The harness prints exactly what to do in the panel
  and waits. This works today.
- **`hook`.** Set `CASPIAN_HW_CONTROL=hook` and write `local/control.local.sh`
  defining four functions: `ctl_hook_apply <label>`, `ctl_hook_start`,
  `ctl_hook_stop`, `ctl_hook_status`. Write it once the command line exists.
  This is what makes `--unattended` possible.

`--unattended` with `manual` is refused rather than silently skipped.

## Reading a partial run

Every step writes a row to `steps.tsv` when it starts and rewrites it when it
finishes. A run killed halfway leaves its last row marked `RUNNING`, and the
summary says:

```
steps: 3 of 6 finished, 1 still marked RUNNING
RESULT PARTIAL. This run did not complete. Do not read it as a clean result.
```

A short log that ends quietly is the failure mode this exists to prevent.

## Verify on the day, before trusting any result

These could not be settled while the harness was written. Each names the command
that settles it.

| What | Command | Why it matters |
|---|---|---|
| Both echo endpoints still answer | `adb shell '(printf "GET /ip HTTP/1.0\r\nHost: ipinfo.io\r\n\r\n"; sleep 5) \| toybox nc -w 8 34.117.59.81 80'` and open `https://1.1.1.1/cdn-cgi/trace` in Chrome | MEASURED working from the phone on 2026-08-30. Both addresses are anycast and may move. A moved address shows up as a shape failure and UNPROVEN, not as a wrong answer, but it still wastes the session |
| The sinkhole is still in place, or is not | `adb shell ping -c1 icanhazip.com` | If it answers a real address, the LAN resolver changed. The pinned endpoints keep working either way; this only tells you whether the confounder that forced the pinning is still there |
| The Pi can capture | `ssh $PI_SSH 'sudo -n tcpdump --version'` | The DNS check is the only step that needs sudo on the Pi |
| The uplink name | `ssh $PI_SSH 'ip route show default'` | The capture is scoped to one interface |
| The hotspot address | `ssh $PI_SSH 'ip -br addr show ap0'` | `HOTSPOT_ADDR` is the fail-closed positive control |
| The panel answers on 8088 over the hotspot | join the phone, open `http://<HOTSPOT_ADDR>:8088/login` | If it does not, the fail-closed step reports VOID for a reason that is not the firewall |
| Chrome's Secure DNS setting | on the phone, `chrome://settings/security` | With it on, the Chrome arm of the DNS check never reaches the Pi's resolver. The `ping` arm still does |
| The modal that Chrome shows on the day | run `caspian-hw preflight`, then look at the phone | MEASURED once, on 2026-08-30, as the notifications promo. A Chrome upgrade can change the resource-id. The harness stops loudly on an unknown modal rather than reading it |
| That the phone stays on the hotspot for a whole capture | the harness checks it, but watch the first run yourself | A roam mid-capture is VOID, and knowing whether it happens often changes how long the windows should be |

## A note about this machine

`cat` on the developer Mac is a shell alias for `highlight -O ansi --force`, so
reading any of these files at an interactive prompt injects ANSI escapes. It
does not affect the harness: aliases are not expanded in non-interactive shells.
Use `sed -n '1,200p' <file>` if you are parsing something by hand.
