// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Copyright (C) 2026 Iman Samizadeh
//
// This file is part of Caspian-BYOC.

// Package netcfg owns everything about interfaces, addresses, routes and the
// firewall on the appliance.
//
// # The split, and why it is the shape it is
//
// The package is deliberately in two halves.
//
// The pure half takes facts and returns decisions. Parsers turn the output of
// "ip", "iw" and "sysctl" into a [Facts] value; [PlanNetwork] turns a [Facts]
// plus the user's server addresses into a [Plan]; a [Plan] returns an ordered
// list of [Step] values and an nftables ruleset as text. None of that touches
// the machine, so all of it is tested on a developer Mac.
//
// The impure half is [Runner], which is one method wide, plus an [Applier]
// that runs steps and writes the inverse of each one to a journal on disk
// before it runs it. The only real implementation is Linux, behind a build
// tag; on every other platform [NewSystemRunner] returns a runner that refuses
// with [ErrUnsupportedPlatform], so the package still compiles for development.
//
// # What the engine does not do
//
// The xray engine creates the tunnel device, sets its MTU and brings the link
// up. It assigns no address, adds no route and installs no rule. Everything
// below the device is this package's job:
//
//   - an address on the tunnel device
//   - a pinned host route to the user's server via the real gateway, without
//     which the engine's own connection to the server matches the default
//     route through the tunnel and loops through itself
//   - a default route for client traffic, or a policy rule plus a second table
//   - rp_filter handling, because two tables make return packets arrive on a
//     different interface from the one the reverse path names, and the strict
//     filter drops them as martians; the symptom is a tunnel that connects and
//     carries nothing
//   - INPUT and OUTPUT firewall permits for the tunnel device, because a
//     router's own traffic is INPUT and OUTPUT and never FORWARD
//
// # Ordering, which is load bearing
//
// [Plan.PreEngineSteps] must be applied before the engine starts and
// [Plan.PostEngineSteps] after. Two reasons, both failure modes rather than
// preferences:
//
//   - The fail-closed ruleset must be in force before forwarding is enabled,
//     and the pinned host route before the engine opens its first connection.
//     Applying either afterwards leaves a window in which client traffic can
//     be forwarded and the block is not yet loaded.
//   - Every post-engine step names the tunnel device in an "ip" command, so
//     every one of them fails if the device does not exist yet.
//
// The firewall is in the pre-engine list precisely because it never needs the
// device to exist: see [Plan.Ruleset].
//
// An earlier version of this note gave a third reason, that
// net.ipv4.conf.default.rp_filter is inherited when an interface is created
// and so had to be set before the engine created the tunnel. The two sentences
// are true and the conclusion did not follow: net.ipv4.conf.all.rp_filter
// decides the outcome for every interface on its own, so nothing about
// rp_filter depends on this ordering. See the citation above
// [Plan.SysctlKnobs].
//
// # Applying twice
//
// A plan may be applied again while it is already in force, because a person
// pressing a button twice is not an error and a panel reporting failure on a
// working box is worse than one that quietly does nothing. [Applier.Apply]
// therefore converges rather than failing.
//
// It does not converge by ignoring errors. A step journals an inverse if and
// only if it changed something, because an inverse recorded for a change that
// was never made is an instruction to delete, on teardown, an address, route
// or rule that existed before this program ran. Anything already present and
// not recorded as ours is left exactly as it is and reported, never replaced.
// The three mechanisms and the reasoning are at the top of idempotence.go.
//
// # Test doubles live in non-test files
//
// [RecordingRunner] and [SimulatedKernel] are compiled into the package
// proper, not into _test.go files, so that other packages' tests and the
// behaviour suite can use them. The cost is that a kernel simulator ships in
// the production binary. If that is ever paid down, both doubles should move
// together into a sibling package, which is the Go convention and is a change
// worth making once rather than twice.
//
// # When the radio refuses what it advertises
//
// The access point's first choice is an interface of its own, added beside the
// station link so that link is not disturbed. MEASURED on the target: the Pi 5
// radio advertises that combination in "iw list" and the brcmfmac driver
// refuses to create it, with "Input/output error (-5)" and no interface,
// WHILE THE STATION LINK IS UP.
//
// That last clause was missing until 2026-08-30 and its absence made the
// sentence wrong. The refusal is CONDITIONAL, not a property of the driver:
// with the station not joined to a network, "iw phy phy0 interface add ap0
// type __ap" returns 0 and the interface appears. Anything in this package
// claiming brcmfmac always refuses is wrong; what it refuses is a second
// interface while the first one holds an association.
//
// So a combination line states what the hardware could do in principle. It is
// not proof that creating the interface succeeds, and nothing short of trying
// settles the difference. The answer is therefore a runtime fallback and not a
// planner rule: [Plan.HotspotTakeover] returns a plan that takes over an
// existing wireless interface instead. A caller applies the first plan, and if
// the step with op [OpCreateIface] fails, tears down and applies the fallback.
//
// The fallback is never free and never silent. Taking over an interface ends
// whatever WiFi connection it holds, which is recorded in the plan's notes and
// in the sentence [Plan.Explain] gives the panel. It is never the uplink: on a
// box whose only route to the internet is that radio, taking it would cut off
// the connection the box exists to share, so that case is refused in words a
// person can act on.
//
// # The kill switch covers this box too
//
// [EgressRestricted] is the default: the OUTPUT chain drops by policy and names
// what may leave. Without it the guarantee had a hole the size of everything
// else on the Pi reaching the internet directly, outside the tunnel: apt, a
// stray daemon, a shell the owner left open.
//
// The permit list is short and every line names the reading that justifies it,
// because the failure modes here are DELAYED rather than immediate. A missing
// DHCP permit costs the box its address at the next renewal, hours later. A
// missing NTP permit lets the clock drift over days and then fails REALITY and
// TLS in a way that reads as a rejected configuration. Neither points at the
// firewall on its own.
//
// Two things about that chain are worth knowing before editing it:
//
//   - Established is the FIRST rule. Every outbound reply to an inbound
//     connection is there, the administrator's SSH session included.
//   - DHCP is permitted in BOTH directions and DNS in neither. That asymmetry
//     is deliberate: a DNS answer matches the conntrack entry the query made,
//     and a DHCP reply goes to a broadcast address or to a client with no
//     address yet, so there is no tuple to match. Delete the server-side DHCP
//     line and the hotspot beacons, devices associate, and none of them gets
//     an address.
//
// The cost is stated in the generated header and in [Plan.Explain]: "apt
// update" from a shell on the box fails while the appliance is on. The way
// back is [EgressOpen], a config change rather than a rebuild, for a user on a
// network nobody thought about.
//
// # Cutting client traffic without switching off
//
// [ForwardCut] drops forwarded client traffic while the hotspot, DHCP, DNS and
// the panel keep working, so a joined device stays joined and can still reach
// the panel to turn it back on. Switching the appliance off would take the
// hotspot down and disconnect the device the user is holding.
//
// It is a flag on the one ruleset rather than a second table: one atomic
// replace, the same names and ports and subnet, so the two states cannot drift
// apart. [Plan.CutStep] and [Plan.RestoreStep] carry the same inverse as the
// firewall step that installed the table, so teardown does not grow a second
// path.
//
// It does not survive a restart, and that falls out of the design rather than
// needing a mechanism: nothing here persists the ruleset, so the service
// regenerates and reloads it on every start. netcfg does not store the cut
// state and a caller should not either.
//
// Residual: a cut client can still make this box work on its behalf by asking
// its resolver. That is not a leak, because the query still goes through the
// tunnel rather than the uplink; it is an incomplete cut.
//
// # What this appliance does not firewall
//
// The input policy is accept. This program controls FORWARDED client traffic
// and makes sure it cannot leave except through the tunnel. It does not decide
// what the owner may run on their own machine, and it does not close their
// administrative access to it.
//
// The only place the input chain restricts anything is the hotspot, where the
// untrusted devices are: a joined device reaches DHCP, DNS, the panel and
// ICMP echo, and nothing else on the box.
//
// Accepting does not weaken a host firewall the owner installs. Every base
// chain registered at a hook is traversed, so a drop in their own table still
// drops. This policy only stops THIS program from installing a host firewall
// nobody asked it for.
//
// # If the box seems to have died after switching on
//
// Recorded here because the shape of the failure is misleading, and a reader
// diagnosing it from outside will reach the wrong conclusion first.
//
// A firewall that drops new inbound connections does not look like a firewall
// from the outside. Established connections keep working, because conntrack
// matches them before any policy applies. So a session that was already open
// stays responsive, and the panel answers normally, while every new connection
// is refused. The box appears simultaneously alive and dead, which reads as a
// crash or a hung service rather than as a rule.
//
// The test is whether an ALREADY OPEN connection still works while a NEW one
// to the same address does not. If so it is a filtering rule, not a crash, and
// switching the appliance off restores it immediately.
//
// # Taking an interface over means taking it away from something
//
// The fallback plans the access point on an interface that already exists. On
// a Raspberry Pi OS image that interface is usually held by NetworkManager,
// and an interface nobody released stays joined to the network it was on.
//
// MEASURED on the target on 2026-08-30: the fallback ran, the service reported
// running with hotspot=wlan0, the panel showed connected, and wlan0 was still
// type managed, still associated to the house network, still on the station's
// channel, broadcasting nothing. The DHCP server bound to it anyway and
// answered a real device on that LAN with DHCPNAK. A user who installs this
// and presses the switch could knock devices off their own network.
//
// So [Plan.HotspotTakeover] asks what owns the interface rather than assuming,
// and refuses when it cannot tell. [Plan.HotspotReleaseSteps] releases it,
// strips the addresses it carries from the network it was joined to, and sets
// the type, in that order and with every inverse journalled: a user whose Pi
// permanently stopped joining their WiFi has lost more than a hotspot.
//
// None of that is proof. [AssertHotspotInterfaceReleased] and
// [AssertHotspotIsAccessPoint] read the state back from the kernel, and
// nothing should report itself up without them. A started process is the same
// class of evidence as a connect code.
//
// # The hotspot on a USB dongle, and why it is a takeover rather than a second interface
//
// A radio that carries a station link can host the access point two ways, and
// which one is right is a property of the RADIO, read from the radio.
//
//   - It DECLARES a combination of managed and AP. The access point gets an
//     interface of its own beside the station, and the connection on that
//     station is kept. This is the Pi 5's built-in brcmfmac radio and it is
//     the default arrangement's path.
//   - It declares NO combination. Then it cannot be both at once, so the
//     station ENDS and the access point runs on the interface already there.
//     This is the TP-Link RTL8192EU dongle.
//
// MEASURED on the target on 2026-08-30, on the dongle:
//
//	TP-Link TL-WN823N v2/v3, RTL8192EU, rtl8xxxu,
//	firmware rtlwifi/rtl8192eu_nic.bin rev 35.7, loaded, no errors
//	supported interface modes:     managed, AP, AP/VLAN, monitor
//	valid interface combinations:  none declared
//	iw phy <dongle> interface add captest type __ap   rc=0, CREATED
//	ip link set dev captest up                        refused
//
// The declaration and the refusal say the same thing: it can be an access
// point, and not while it is also a station. The two-interface attempt is
// therefore the wrong shape for this radio, and it fails in the worst possible
// place: the create SUCCEEDS, so the runtime fallback, which watches for the
// create failing, never runs, and the start dies two steps later at the link
// up with the machine half configured.
//
// Deciding this at PLAN time rather than by fallback is what makes it safe.
// The plan either produces the release sequence proved by hand on the box, or
// it refuses; it never hands the applier a path it has already concluded
// cannot work.
//
// WHAT IS STILL UNPROVEN, and it is the one thing this change reveals rather
// than settles: whether the dongle BEACONS once it is an access point. No
// scan from another device has been taken. [AssertHotspotIsAccessPoint] is
// what stands between a radio that accepts AP mode and serves nothing and a
// panel claiming a working hotspot, and its own doc comment states exactly
// what it can and cannot prove.
//
// # Defects that only real bytes and a real box found
//
// Recorded here because each one was green in a full test run beforehand, and
// because the shape repeats. testdata/PROVENANCE.md carries the detail.
//
//   - A sysctl write ordered before the interface it names. The generated
//     sequence set net.ipv4.conf.ap0.rp_filter at step 6 and created ap0 at
//     step 9. "sysctl -w" on a missing knob fails, the step carries no "-e",
//     and [Applier.Apply] stops at the first failure. This was not a teardown
//     problem: it was a first-run startup failure on every box where the
//     hotspot interface has to be created, which is the common case, and it is
//     the most consequential defect found in this package. No sysctl step
//     names an interface any more, and a test asserts it.
//   - A comment asserting the inverse of the mechanism it described. It said
//     relaxing conf.all alone "changes nothing", and three per-interface
//     writes were built on it. Strict is 1 and loose is 2 and the kernel takes
//     the maximum, so conf.all pins every interface by itself.
//   - A guessed inverse that would have weakened the machine. A fixture
//     assumed conf.eth0.rp_filter matched conf.all and put 0; the box reported
//     2, so teardown would have switched reverse-path filtering off on the
//     uplink of a box that had it on.
//   - Two firewall rules that no kernel would load. "udp sport 67 dport 68"
//     needs the protocol keyword repeated for the second field; nft rejects
//     the whole ruleset otherwise, leaving no firewall rather than a partial
//     one.
//   - A parser that could not read its own target's output. iw 6.9 prints
//     "2412.0 MHz"; the authored fixture used the integer form, so every
//     frequency was dropped and a radio with seventeen usable channels
//     reported none.
//   - A test double that formatted what the parser would read, so the two
//     agreed by construction and neither matched the command being run.
//   - Applying a plan twice was untestable, not merely untested. Against a
//     recorder that succeeds at everything, a second apply looks perfect; on a
//     kernel it stops at the first EEXIST. Found by reasoning about what the
//     double could not see rather than by running anything, which is the same
//     lesson as the sysctl double one layer up: a double that always says yes
//     agrees with the code by construction. [SimulatedKernel] is the answer,
//     and it models refusal.
//   - An appliance that silently closed its owner's access to their own
//     machine. The input policy was drop with nothing accepted on the uplink,
//     so from the moment the ruleset loaded every new inbound connection was
//     refused and SSH stopped answering. Measured on the target. The comment
//     in the generator claimed an operator "loses that session", which was
//     false in the direction that mattered: the conntrack rule kept existing
//     connections alive, so the failure was silent rather than loud, and a
//     loud one would have been noticed during installation instead of from
//     another room. On a headless box the remaining recovery is a power cycle
//     and a card reader. The policy is now accept; see above.
//   - A guess about a driver, corrected by measurement in the safe direction
//     for once. The takeover was expected to fail at "iw dev wlan0 set type
//     __ap", because the same driver refuses "iw phy phy0 interface add" with
//     Input/output error (-5). It does not: creating an interface and changing
//     the type of one are different operations, and brcmfmac treats them
//     differently. Recorded because the prediction was wrong and the reasoning
//     behind it (one refusal implies the other) is the same shape as inferring
//     an errno from a neighbouring case.
//   - A takeover that took nothing, and said it had. The note read "this ends
//     the WiFi connection wlan0 currently holds", which described an effect no
//     code produced: nothing in the tree mentioned NetworkManager, nmcli or
//     wpa_supplicant, and the design's own question about which of them owns
//     the box was never answered. The sentence then suppressed the work that
//     would have caught it, exactly as the rule describes: it was read in a
//     journal an hour later and taken as confirmation the takeover had worked.
//     The DHCP server meanwhile answered a device on the user's home network.
//   - The same capability table trusted too LITTLE, which is the mirror. A USB
//     dongle (rtl8xxxu) declares AP among its interface modes and declares no
//     "valid interface combinations" section at all. That absence was read as
//     a prohibition and the planner refused to use the radio. A combination
//     table is about COEXISTING, and it has nothing to say about a radio whose
//     station is going to be ended, which is what the takeover does. Measured:
//     with the station released, the dongle hosts an access point. The two
//     questions are now asked separately: a link that must be KEPT, because it
//     is the uplink, still needs the declaration; a link that may be ENDED
//     needs only AP support.
//   - The hotspot could be planned onto the interface carrying the uplink.
//     Found while reproducing the above: an interface reporting type AP while
//     holding the default route has no station link to preserve, so it read as
//     a free radio and was chosen, giving hotspot == uplink. The uplink now
//     counts as a link that must not be disturbed whatever type it reports.
//   - A capability table read correctly and trusted too far. The planner
//     concluded from "iw list" that it could add an access point beside the
//     station, which is what the radio advertises and what the driver refuses.
//     The parser was right, the planner was right, and the plan did not work:
//     capability is not permission. Fixed with a runtime fallback rather than
//     by teaching the planner a per-driver exception it could only ever guess.
//   - An inverse for a step that never took effect was retried for ever. The
//     failed start above left one journal entry at "begin"; its undo, "iw dev
//     ap0 del", failed because ap0 was never created, so the entry was
//     retained and every later start repeated it. Teardown was correct to
//     continue past it and wrong to keep it: an object that is already gone
//     means the inverse has achieved what it wanted. See IsNotFound.
//   - A created interface was called a station because it reported a channel,
//     and that stopped the appliance starting at all. The readback refused
//     with "ap0 is still a station on channel 36 and was never put into
//     access point mode" for an interface whose own "iw dev ap0 info" said
//     "type AP" at that moment. A vif created on a radio inherits the parent's
//     channel, so the channel is ALWAYS there, so the check always failed.
//     Third occurrence of one shape: a channel with no network name is not an
//     association. The question is now asked of "iw dev <if> link", which
//     answers it, rather than inferred. MEASURED: a freshly created AP vif and
//     an idle station print BYTE-IDENTICAL output there, "Not connected.",
//     with rc 0, and both mean free. See AssertHotspotInterfaceReleased.
//   - NetworkManager takes the interface this package CREATES, and the address
//     with it. This was carried as an open question ("whether NetworkManager
//     takes an interface that appears from iw phy ... interface add") and the
//     answer is yes. MEASURED on the target on 2026-08-30, polling every 0.3s:
//     ap0 DOWN with 10.83.51.1/24, then UP with no address, then gone, inside
//     1.4 seconds, while NetworkManager logged the device going from unmanaged
//     to external to "managed-type: 'full'" and avahi-daemon independently
//     logged "Withdrawing address record for 10.83.51.1 on ap0". dnsmasq then
//     exited 2 with "failed to create listening socket for 10.83.51.1: Cannot
//     assign requested address". The created path now runs the same nmcli
//     release the takeover path always ran, gated on NetworkManager being
//     present on the machine rather than on a per-interface manager that
//     cannot be measured for a device that does not exist yet. The fix was
//     proven on the hardware BEFORE it was written: with the release in place
//     the address survived and a UDP socket bound 10.83.51.1:67.
//   - The failure above surfaced as somebody else's exit code. Nothing in this
//     package checked that the address it had just added was still there, so
//     the first thing to notice was dnsmasq, and the user was shown a message
//     about another program holding the address. EADDRNOTAVAIL means the
//     opposite: the address is on NO interface. AssertHotspotAddressPresent
//     now reads it back. It deliberately does NOT require the interface to be
//     UP: an access point interface has no carrier until hostapd starts and
//     holds a bindable address while reading DOWN, measured on the target.
//   - Wiphy indices are not stable across a reboot. MEASURED on the target on
//     2026-08-30: the built-in radio and the USB dongle came back with their
//     numbers exchanged. Nothing may be decided from a remembered index; the
//     radio is resolved from the interface within the same detection run, and
//     TestPhyNumbersMaySwapAndTheDecisionFollowsTheInterface renames the phys
//     in the facts and proves the decision follows the interface.
//   - A CHANNEL READ AS AN ASSOCIATION, for the third time, in the third
//     place. The readback learned it, the plan note learned it, and the
//     CHANNEL SELECTOR had not. MEASURED on the target on 2026-08-30: wlan0,
//     left over from a previous hotspot and joined to nothing by all three of
//     "iw dev wlan0 link" ("Not connected."), "iw dev wlan0 info" (no ssid
//     line) and nmcli ("wlan0:disconnected"), still reported channel 36. The
//     planner read that as a live connection it had to match, pinned the
//     access point to 36, and said so in a note. The user had asked for
//     2.4GHz; 36 is 5GHz; hostapd was handed the contradiction and the start
//     failed as "the hotspot failed". Earlier, when the same pin succeeded,
//     the hotspot came up on 5GHz and the test handset could not see it at
//     all: its scan returns 2412 to 2462 MHz. The panel said the hotspot was
//     up and the phone could not find it, and both were true.
//     Detect now asks "iw dev <if> link" for every station interface and
//     WirelessIface.StationLink answers from that measurement. A channel is
//     evidence of nothing anywhere in this package.
//   - The note that went with that pin asserted a connection unconditionally:
//     "the channel an existing WiFi connection is using on wlan0", printed
//     about an interface joined to nothing. A confident wrong sentence in the
//     operator's own log, and it did what the rule above says such sentences
//     do: it explained the wrong channel away and stopped anyone looking for
//     the cause. The sentence is now emitted only when a station link was
//     measured, and it names the network.
//   - A plan that had CONCLUDED a path could not work handed it to the
//     applier anyway. The dongle branch added a note saying "choosing it for
//     the hotspot will fail to start" and then returned success. On
//     2026-08-30 it did exactly that: created the interface, failed at "ip
//     link set dev ap0 up" two steps later, and left a part-applied start to
//     unwind. It became a refusal, and then, once the radio's own declaration
//     was measured, a takeover: the shape the radio says it can do. What
//     never returned is a plan that predicts its own failure and proceeds.
//   - An explicit band was silently replaced. The plan chose a channel with
//     no idea what band the user had asked for, internal/privsvc took the
//     band from the request and the channel from the plan, and hostapd was
//     handed hw_mode=g with channel 36. Options.HotspotBand now carries the
//     request into the decision, and a band that cannot be honoured is a
//     refusal naming the band, the radio, and the connection responsible when
//     a pin is what blocks it.
//   - The address strip was gated on the takeover. An interface joined to
//     nothing can still be HOLDING an address from another network, and
//     before the channel fix such an interface was never chosen directly, so
//     the gap was unreachable and invisible. Choosing one directly is now
//     normal, so every path that names an interface which already exists
//     strips the addresses that are not in the hotspot subnet, each with its
//     inverse. Bringing the interface down and retyping it stays the
//     takeover's business alone.
//   - The first fix for that carried a wrong claim of its own. It said the
//     kernel accepts a duplicate "ip rule add" silently, and a per-add query
//     was built to prevent one. Measured on the target: an identical selector
//     at the same explicit priority is REFUSED. Duplicates arise only where no
//     priority is given, so the query became an invariant instead, and
//     [SimulatedKernel] was corrected because a double that is wrong in the
//     safe direction still teaches a false model to everything downstream.
package netcfg
