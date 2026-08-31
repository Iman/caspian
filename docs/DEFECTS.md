# Open defects and accepted gaps

[🇮🇷 فارسی](DEFECTS.fa.md) | 🇬🇧 **English** | [🇷🇺 Русский](../README.ru.md) | [🇨🇳 中文](../README.zh.md)

> Persian edition: [`docs/DEFECTS.fa.md`](DEFECTS.fa.md). The English file is
> the one the tests read. If the two ever disagree, this one is correct.

Things that are known, evidenced, and NOT fixed. A defect nobody wrote down is
indistinguishable from one nobody noticed, and the difference decides whether
the next person to read the code goes looking.

Each entry says what was measured, what it costs, and what would close it. None
of them is a leak of client traffic. One path was a leak, where the firewall's
inverse ran whatever had happened to the inverses before it. That path was
closed on 2026-08-30, and its guard is
`TestTheFirewallIsNotRemovedWhenAnEarlierInverseFailed`.

Recorded 2026-08-30 by the external-findings audit. Every measurement below was
taken that day on this working tree unless it names a different vantage.

---

## D1. Nothing re-asserts the firewall once it is loaded

**Status: OPEN. Not fixed, not mitigated.**

`nft` is invoked in exactly one form in the whole repository, `nft -f -`, from
`Plan.FirewallStep`, `Plan.CutStep`, `Plan.RestoreStep` and their shared
inverse. There is no read of the live ruleset anywhere in production code, and
no loop that re-checks it. `Applier.Apply` will also SKIP re-loading the ruleset
when the journal records the identical command as `PhaseDone`, which is a claim
about the journal and not about the kernel.

So if anything removes `table inet caspian` while the appliance is running, the
box keeps forwarding, the panel keeps reporting connected, and nothing notices.

What is NOT the trigger. The coordinator reported this from the running
appliance on 2026-08-30, and the audit did not measure it itself.
`systemctl is-enabled nftables` reported disabled, and `nft list tables` showed
`table inet caspian` present, so the Debian `nftables` package the installer
pulls in is not flushing it. Boot ordering is not the trigger either.
`packaging/caspian.service` is `After=network.target`, and Debian's
`nftables.service` runs before `network-pre.target`.

What remains: an administrator running `nft flush ruleset`, another package
taking over the firewall, or anything else that clears the table mid-session.

Cost: the kill switch stops blocking silently. The hotspot readback closed the
same class of gap on 2026-08-30, in `netcfg.AssertHotspotInterfaceReleased`,
added because a process being alive is not evidence it is working. The firewall
never got the equivalent.

Closing it: a readback after the load, and a periodic re-assert. Both need a
decision first, because a re-assert is a new background loop with its own
failure modes, and `nft list ruleset` output would have to be handled as a
string this package does not currently parse.

## D2. Two changes to the machine have no inverse and are not journalled

**Status: D2b CLOSED. D2a CLOSED in process, OPEN across a kill.**

Fixed 2026-08-30. `Supervisor.Stop` now removes both generated configuration
files, and re-blocks the radio devices it recorded as soft blocked before it
unblocked them. It re-reads their current state first, so a device somebody else
has already blocked, or that has become hard blocked, or that has been
unplugged, is left alone. A radio the user switched on themselves while the
appliance ran is indistinguishable from one we unblocked, so it is re-blocked.
That costs the user one rfkill command to reverse, against a machine quietly
left in a state we changed.

What is still open is narrow and worth stating precisely. The record of which
devices were unblocked lives in memory, so a service that is KILLED rather than
stopped does not re-block them, and `netcfg.Recover` cannot help because
`rfkill` is not on netcfg's allowed-binaries list and the journal therefore
cannot carry the command. Closing that half is a decision about the allowlist,
not a defect in the code that exists.

The remedy this entry originally proposed, "journal the rfkill change like any
other", is the thing that cannot be done without that decision.

**Everything from here to the end of D2 is the original entry, kept for the
history. It describes the state before the 2026-08-30 fix above.**

Everything `internal/netcfg` does is journalled with an inverse. Two effects
outside that package are not, so neither is undone by `Service.Stop` and neither
is replayed by `netcfg.Recover` after a kill.

**D2a, the radio soft-block.** `hotspot.Supervisor.ensureRadioUnblocked` runs
`rfkill unblock wifi` when it finds a soft block. `Supervisor.Stop` does not
re-block it. A box that was found with its radio switched off is left with it
switched on, including after a start that failed later in the sequence.

**D2b, the generated configuration files.** `Supervisor.Start` writes
`/run/caspian/hostapd.conf` and `/run/caspian/dnsmasq.conf` at 0600 root.
`Supervisor.Stop` removes the pid files and nothing else, so both survive a
stop. The hostapd file contains the hotspot passphrase.

Cost of D2b is bounded by `/run` being tmpfs, cleared at boot, and by the mode:
this is a file the owner of the box can already read as root. It is recorded
because "the appliance removes what it wrote" is easier to keep true than to
re-establish, and `docs/BEHAVIOUR.md` says of a stop that "the two processes it
started are stopped and their pid files removed", which is accurate today only
because it says pid files.

That quotation is now stale. `docs/BEHAVIOUR.md` says "their pid files and the
configuration files generated for them removed", which is the D2b fix landing.
The argument above turned on the word "pid", so it no longer applies to the
sentence it quotes. Left in place and marked rather than rewritten, because the
reasoning is the record of why D2b was opened.

Closing them: journal the rfkill change like any other, and remove the two
configuration files in `Supervisor.Stop`.

## D3. A created hotspot interface is not released from NetworkManager

**Status: OPEN by decision. The measurable half was fixed on 2026-08-30.**

The plan emits `nmcli device set <iface> managed no` whenever its measured
`HotspotManager` is NetworkManager. That covers the takeover and the free
interface a second radio offers, which is mode B, the USB adapter this product
tells people to buy.

It does NOT cover the two paths where this package CREATES the access point's
interface, because detection ran before that interface existed and no manager
was measured for it. `Plan.HotspotManager` is deliberately left unknown there
rather than guessed from the parent radio.

What is unknown, and it is a live-machine question: whether NetworkManager takes
an interface that appears from `iw phy ... interface add`. If it does, the
2026-08-30 incident recorded above `HotspotReleaseSteps` can arrive by that
door.

The check, on a box where the mode A path succeeds: bring the appliance up, then
`nmcli device status | grep ap0`. `unmanaged` means there is nothing to fix.
Anything else means the release has to be extended to created interfaces, which
needs a way to tolerate `nmcli` being asked about a device it has not enumerated
yet.

Guard: `TestACreatedHotspotInterfaceHasNoMeasuredManagerAndIsNotReleased` pins
the gap so it stays a decision. It fails if a future change starts measuring a
created interface, and the right response then is to extend
`TestTheHotspotInterfaceIsReleasedFromNetworkManagerOnEveryPathThatNamesOne` and
delete it.

## D4. Stop reports success when it undid nothing

**Status: OPEN. Reporting only. The box stays fail-closed.**

`Applier.Teardown` returns a non-nil error only when the journal FILE cannot be
closed or rewritten. A replay in which every inverse failed returns no error,
and the count lives in the report. `Service.stopLocked` reads `rep.Failed` only
to log a warning, and joins the engine's and the hotspot's errors for its return
value.

So a stop in which no route, rule or address could be removed returns nil, and
the panel tells the user the box was returned to how it was found while it is
still fully configured. MEASURED 2026-08-30:
`TestAStopThatCouldNotUndoEverythingKeepsTheBlockAndTheJournal` logs
`Stop returned <nil> with 7 inverses failed and 8 journal entries left`.

This is not a leak. Since 2026-08-30 the firewall's inverse is HELD whenever an
earlier one failed, so a box in this state keeps its block, and the next start
replays what is outstanding.

`internal/privsvc.applyPreEngine` had the same defect on the fallback path. The
2026-08-30 fix there reads `rep.Failed` alongside the error, and the same change
in `stopLocked` would close this. That change was left out of the batch
deliberately, because changing what `Stop` returns changes what the panel shows,
and that wants its own decision.

## D5. The uninstaller replays the journal by its own rules

**Status: OPEN. Divergence, not yet a decision.**

`uninstall.sh` carries an independent Python reimplementation of the journal
replay. It replays newest first and carries on past failures, matching
`Applier.Teardown` as it was. It has NO equivalent of the rule added on
2026-08-30 that holds the firewall's inverse when an earlier one failed.

An uninstall in which the routing inverses fail therefore still deletes the
table, which is the end state that was a leak while the appliance was running.
It is weaker here, because an uninstall also stops the service and takes hostapd
and dnsmasq with it, so there is normally no hotspot left to leak from.

The decision needed is which of the two an uninstall should be. "Remove
everything the product installed" argues for the current behaviour. "Never take
the block away from a machine still carrying what it blocks" argues for
mirroring the Go rule and leaving the table with a message saying why. It is
recorded rather than changed because `uninstall.sh` is outside the packages the
2026-08-30 batch was scoped to.

---

## What is NOT in this file

Findings that were closed rather than recorded, so that this list is not read as
the whole picture:

- The firewall's inverse running while the machine still carried routes,
  forwarding and a live access point. Closed 2026-08-30 in `netcfg.replay`.
- The hotspot fallback being applied on top of a first plan that could not be
  undone. Closed 2026-08-30 in `privsvc.applyPreEngine`.
- Mode B starting hostapd on an interface NetworkManager still held. Closed
  2026-08-30 in `Plan.acceptHotspot` and `Plan.HotspotReleaseSteps`.
- `docs/BEHAVIOUR.md` and `netcfg.RederiveForUplink` asserting that the box
  watches its uplink and reloads the firewall when it moves. Both rewritten
  2026-08-30, with `TestNothingInTheApplianceWatchesTheUplink` left behind so
  the sentence cannot drift back.
