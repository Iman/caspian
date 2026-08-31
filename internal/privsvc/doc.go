// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

// Package privsvc is the privileged half of the appliance: the thing that runs
// as root, owns routes, the firewall, the access point and the engine, and
// answers the named actions the panel can ask for, and nothing else.
//
// It implements panel.Privileged for real. internal/panel defines that
// interface and provides a fake; this package is the other end.
//
// # What it is allowed to trust
//
// Nothing the caller sends. internal/panel/priv.go states the rule plainly:
// "the privileged side is expected to validate each one against what it
// detected for itself rather than trusting it." Every field of every request
// is therefore checked against this machine before it is used:
//
//   - StartRequest.ConfigJSON is re-parsed by internal/link, which is the same
//     validation the panel ran (UUID shape, address, port, TLS for trojan) and
//     is run again here because the panel is not trusted to have run it. See
//     configFromRequest for why re-parsing is also the only way this package
//     can reach internal/xcfg.
//   - Network.InternetInterface must be an interface this machine has and one
//     that carries a default route.
//   - Hotspot.Interface must be a radio or interface that reported AP support
//     in this machine's own "iw list" output.
//   - Hotspot.Channel must be one the radio reported as usable, and when the
//     radio pins the channel it must be the pinned one.
//   - Hotspot.Subnet must be a private IPv4 network of a size that has a
//     usable range in it.
//   - Network.DNSMode and Network.OnTunnelDown must be the values internal/state
//     guarantees, and empty is refused rather than read as a default.
//   - EngineLogLevel must be one of panel.EngineLogLevels.
//
// A refusal is a panel.Fault, never a sentence, and never a value the caller
// sent. See faults.go.
//
// # Ordering, which is the part that cannot be got wrong by accident
//
// internal/netcfg splits its steps into two lists precisely so that a caller
// cannot get this wrong without meaning to, and internal/netcfg/doc.go,
// "Ordering, which is load bearing", gives the two failure modes:
//
//   - the fail-closed ruleset must be in force before forwarding is enabled,
//     and the pinned host route before the engine opens its first connection,
//     or there is a window in which client traffic can be forwarded and the
//     block is not yet loaded;
//   - every post-engine step names the tunnel device in an "ip" command, so
//     every one of them fails if the device does not exist yet.
//
// That note used to give a third reason, that net.ipv4.conf.default.rp_filter
// is inherited when an interface is created and so had to be set before the
// engine created the tunnel. internal/netcfg retracted it on 2026-08-30:
// net.ipv4.conf.all.rp_filter decides the outcome for every interface on its
// own, so nothing about rp_filter depends on this ordering. This package cited
// the retracted reason until the same day; it is written out here rather than
// quietly deleted, because the ordering it argued for is still correct and
// somebody checking why will otherwise find two sources that disagree.
//
// So the sequence in Start is: recover, validate, detect, plan, PreEngineSteps,
// READ THE HOTSPOT INTERFACE BACK, engine, PostEngineSteps, hotspot, READ THE
// ACCESS POINT BACK. The firewall is first inside PreEngineSteps, which is
// internal/netcfg's doing and not this package's; this package's job is only to
// apply the two lists on the two sides of the engine and never to merge them.
//
// # Why two of those steps are readbacks
//
// Applying a step is not the same as the kernel having done it, and a live
// process is not an access point. Both halves of that were measured on the
// target on 2026-08-30 in one event: the service reported running on a box
// whose hotspot interface was still a station on the house network, while a
// DHCP server this appliance started answered a real device on that network.
// Every command had returned success.
//
// So the interface is read back from the kernel before anything is allowed to
// bind to it, and the access point is read back before this service says it is
// running. readback.go holds both, says what each one can and cannot prove, and
// records which of them the shell implementation this project replaces already
// had.
//
// # What happens when a start fails half way
//
// Everything applied is undone. The journal already holds the inverse of every
// change, written to disk before the change reached the kernel, so rollback is
// Applier.Teardown plus stopping whatever was started. A start that fails
// leaves the machine as it was found, not half configured.
//
// # Recovery
//
// Recover replays a journal left by a process that was killed. It runs before
// anything else, at service startup, because the plan a start is about to apply
// assumes the machine is in the state detection found it in, and a leftover
// policy rule or half default route makes that false.
//
// # It reads no state file
//
// docs/LAYOUT.md, "Who writes what": the panel process owns state.json and the
// privileged process owns netcfg.journal, and neither writes the other's file.
// Everything this package needs arrives in the request.
//
// internal/state IS imported, for exactly two exported string constants,
// DNSModeTunnel and OnTunnelDownBlock, so that the values this package refuses
// to accept anything but cannot drift from the values that package refuses to
// persist anything but. It never calls Load and never names state.json.
// TestPrivsvcReadsNoStateFile scans this package's own source and fails if it
// ever does, which is a guard the import does not weaken and a comment would
// not have provided.
package privsvc
