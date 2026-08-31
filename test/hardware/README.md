# test/hardware

The hardware proof harness. **The runbook is `docs/HARDWARE-TEST.md`** and this
file is only the map.

```
caspian-hw            the only thing you run. Subcommands, --dry-run, --help
box.env.example       copy to local/box.env and fill in
lib/common.sh         verdicts, step ledger, dry run, run directory, redaction
lib/config.sh         reads local/configs, scheme gate, host extraction, verdicts
lib/control.sh        driving the box. A seam, deliberately: see its header
lib/phone.sh          everything adb. Every command carries what it measured
lib/pi.sh             the Pi: firewall check, SOCKS cross-check, uplink capture
lib/exitip.sh         capturing an exit IP twice, from two independent sources
bin/cdp-eval.py       reads one fact out of a Chrome tab over DevTools
steps/baseline.sh     step 1. A run with no baseline is not a run
steps/prove.sh        step 2. One config, proven or not
steps/switch.sh       step 3. Two configs; the exit IP must change
steps/failclosed.sh   step 4. Engine stopped: the phone must reach nothing
steps/dnsleak.sh      step 5. No plaintext client DNS on the uplink
selftest/run.sh       offline checks. No phone, no Pi, no network
```

## Before touching it

```
./caspian-hw selftest     # 69 assertions, offline
./caspian-hw --help
./caspian-hw --dry-run --unattended all <first> <second>
```

`--dry-run` prints every action and performs none. It exits 4, never 0: it
measured nothing, so it has no verdict, and a harness that produced a green line
from a dry run would be inventing a result.

## What the selftest covers, and what it cannot

It covers the parts that decide what gets printed and what gets redacted,
because those are where a silent bug is worst: the scheme gate and its drift
check against `internal/link/link.go`, host extraction for all six shapes, the
redaction filter, the verdict ordering, the two-source logic, the partial-run
detector and the fingerprints.

It cannot cover anything that needs the phone, the Pi or the network. Those are
listed, one per row with the command that settles each, in the last section of
`docs/HARDWARE-TEST.md`.

## House rules for edits here

- Every command that talks to a device carries a comment saying what it returned
  when it was measured, and the date. Where a behaviour was not observed, the
  comment says so in those words.
- Progress goes to stderr, results go to stdout. Several helpers return their
  answer on stdout inside `$( )`, and a progress line on stdout is swallowed
  into the answer.
- Nothing writes an artefact except `hw_write`, which redacts and then re-reads
  the file to check the redaction held.
- `bash -n`, `shellcheck -x`, and `./selftest/run.sh` all pass before a change
  is finished.

## What this vantage cannot grade: IPv6

Measured 2026-08-30, on the network the box and the phone are both on.

The phone carries only a link-local address on its WiFi interface, `ip -6 route
show default` is empty, and a connection to an IPv6 literal answers `Network is
unreachable`. The Pi is the same: `capture-pi5-ip-route6-default.txt` in
`internal/netcfg/testdata` is empty because the box has no IPv6 default route
either.

So there is no IPv6 on this LAN at all, and an IPv6 leak check run here would
pass without the appliance doing anything. That is a test that passes for the
wrong reason, and the reference project's leak suite has one
(`004-hotspot/test-for-leaks.sh`, section 3, which greps for "Network is
unreachable" and calls it a pass). It is not repeated here.

What this means for every result this harness has produced: they are IPv4
results. They say nothing about whether IPv6 traffic from a joined device would
be carried, dropped or leaked, because no IPv6 traffic was possible. Anyone
running this on a network with working IPv6 should treat that as a new question
rather than a covered one, and should expect to write the step, not just enable
it.

The same applies to QUIC over IPv6 specifically, which is the shape most likely
to surprise: a browser with a working IPv6 path prefers it, and prefers UDP on
it.
