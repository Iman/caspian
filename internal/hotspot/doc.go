// SPDX-License-Identifier: AGPL-3.0-or-later

// Package hotspot runs the access point, its DHCP server and its DNS server.
//
// The package is split in two halves on purpose.
//
// The pure half turns a struct into configuration text: [RenderHostapd] and
// [RenderDnsmasq] take a validated struct and return a string. They touch no
// files, no processes and no network, so their output is checked byte for byte
// against golden files and a change to the shipped configuration shows up as a
// diff in review.
//
// The side-effecting half is [Supervisor], which starts, stops and health
// checks hostapd and dnsmasq. Every effect it has on the machine goes through
// the [System] interface, so tests substitute [Recorder] and assert on the
// exact command lines.
//
// What this package does NOT do:
//
//   - It does not detect interfaces. The caller says which interface to use.
//     Detection belongs to internal/netcfg.
//   - It does not write firewall or routing rules. The fail-closed ruleset, the
//     DNS redirect and the client isolation rules belong to internal/netcfg.
//     ap_isolate in the generated hostapd configuration stops one associated
//     station talking to another THROUGH hostapd; it is not the whole of client
//     isolation and is not claimed to be.
//   - It does not query the radio. The caller passes a [RadioConstraint]
//     describing what the radio reported.
//
// Platform: the appliance is Linux. The pure half and the tests build and run
// everywhere, so development on darwin works; [NewSystemRunner] is the only
// thing gated by a build tag.
package hotspot
