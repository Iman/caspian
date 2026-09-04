// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/privsvc"
	"caspianbyoc.org/caspian/internal/state"
)

// runCheck reports what this box looks like and changes nothing.
//
// It exists for the moment somebody is standing in front of a box that is not
// working. Everything it prints is either a measurement taken in this run or a
// value from docs/LAYOUT.md, and every section says which vantage it came from,
// because "the privileged service says the radio can host an access point" and
// "this command asked the radio directly" are different claims and only one of
// them survives the service being down.
//
// It runs read-only commands only: "ip -br addr", "ip route show default",
// "ip -d link show", "iw dev", "iw list", "iw reg get" and "sysctl -e --".
// Every one of them is on internal/netcfg's binary allowlist and none of them
// takes an argument that changes anything.
func runCheck(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "caspian: %q is not an option of \"caspian check\".\n", args[0])
		return exitUsage
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	w := stdout
	section(w, "This binary")
	writeVersion(w)
	_, who := runningPrivileged()
	fmt.Fprintf(w, "running as: %s\n", who)

	checkPrograms(w)
	checkPaths(w)
	checkPorts(w)
	checkPrivilegedService(ctx, w)
	checkLocalDetection(ctx, w)
	checkState(w)

	return exitOK
}

func section(w io.Writer, title string) {
	fmt.Fprintf(w, "\n%s\n%s\n", title, strings.Repeat("-", len(title)))
}

// checkPrograms reports which of the programs this appliance runs are present.
//
// Presence is decided by finding the file, the same way docs/INSTALL.md decides
// it: "Presence is decided by the command each one provides, not by asking a
// package database, because 'is nft on this box' has the same answer
// everywhere."
// checkedProgram is one row of the programs table: a binary this platform's
// backend runs, and what it is for.
type checkedProgram struct {
	name string
	what string
}

func checkPrograms(w io.Writer) {
	section(w, "Programs this box needs")
	progs := platformPrograms()
	if len(progs) == 0 {
		fmt.Fprintf(w, "  none: this build has no network backend for %s\n", runtime.GOOS)
		return
	}
	for _, p := range progs {
		if path, ok := findProgram(p.name); ok {
			fmt.Fprintf(w, "  found    %-14s %s (%s)\n", p.name, path, p.what)
			continue
		}
		fmt.Fprintf(w, "  MISSING  %-14s needed for %s\n", p.name, p.what)
	}
}

var programSearchPath = platformProgramSearchPath()

func findProgram(name string) (string, bool) {
	for _, dir := range programSearchPath {
		p := filepath.Join(dir, name)
		fi, err := os.Stat(p)
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		// Windows has no execute bit: os.Stat reports every regular file as
		// rw-rw-rw- regardless of extension, so the permission check below
		// would never pass there and every program would read MISSING. The
		// unix build still asks for it, because a non-executable file beside
		// the binary there really is one that will fail to run.
		if runtime.GOOS != "windows" && fi.Mode().Perm()&0o111 == 0 {
			continue
		}
		return p, true
	}
	return "", false
}

// checkPaths reports every path docs/LAYOUT.md fixes, with the mode and owner
// it fixes beside what is actually there.
func checkPaths(w io.Writer) {
	section(w, "Paths, against what docs/LAYOUT.md fixes")
	type want struct {
		path string
		mode fs.FileMode
		who  string
		note string
	}
	acct := layout.ServiceAccount
	paths := []want{
		{layout.BinaryPath, 0o755, "root", "the single binary"},
		{stateDir, 0o700, acct, "persistent state; holds a credential"},
		{filepath.Join(stateDir, state.FileName), 0o600, acct, "written by the panel"},
		{journalPath, 0o600, "root", "written by the privileged service"},
		{firstRunPasswordPath, 0o600, acct, "consumed and deleted by the panel on first start"},
	}
	if layout.RuntimeDir != "" {
		paths = append(paths,
			want{layout.RuntimeDir, 0o750, "root:" + acct, "runtime sockets"},
			want{socketPath, 0o660, "root:" + acct, "panel to privileged service"})
	}
	if runtime.GOOS == "linux" {
		paths = append(paths, want{filepath.Join(layout.RuntimeDir, "dnsmasq"), 0o700, acct, "dnsmasq's own directory for its pid file"})
	}
	for _, p := range paths {
		fi, err := os.Stat(p.path)
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(w, "  absent   %-38s want %04o %-13s  %s\n", p.path, p.mode.Perm(), p.who, p.note)
			continue
		}
		if err != nil {
			fmt.Fprintf(w, "  ?        %-38s could not be examined: %v\n", p.path, err)
			continue
		}
		mark := "ok      "
		if fi.Mode().Perm() != p.mode.Perm() {
			mark = "MODE    "
		}
		fmt.Fprintf(w, "  %s %-38s is %04o %-13s  want %04o %s\n",
			mark, p.path, fi.Mode().Perm(), ownerOf(fi), p.mode.Perm(), p.who)
	}
	fmt.Fprintf(w, "\n  A file that is absent is not always wrong: /run is a tmpfs and is rebuilt at\n")
	fmt.Fprintf(w, "  every boot, the journal exists only while changes are applied, and the\n")
	fmt.Fprintf(w, "  first-run password is deleted the moment the panel has used it.\n")
}

func checkPorts(w io.Writer) {
	section(w, "Ports, from docs/LAYOUT.md")
	fmt.Fprintf(w, "  %-6d %-16s %s\n", dnsPort, "hotspot", "dnsmasq: DHCP and DNS for joined devices")
	fmt.Fprintf(w, "  %-6d %-16s %s\n", localDNSPort, loopbackHost, "the engine's DNS listener; dnsmasq's only upstream")
	fmt.Fprintf(w, "  %-6d %-16s %s\n", panelPort, "panel address", "the web panel")
	fmt.Fprintf(w, "  %-6d %-16s %s\n", socksPort, loopbackHost, "SOCKS, for diagnostics and the exit-IP proof")
	fmt.Fprintf(w, "\n  The pairing that breaks quietly is %d: dnsmasq forwards only there and the\n", localDNSPort)
	fmt.Fprintf(w, "  engine listens there. If they drift, DNS stops resolving for every joined\n")
	fmt.Fprintf(w, "  device while the hotspot and the tunnel both still look healthy.\n")
}

// checkPrivilegedService asks the service what it sees, over the socket, as the
// panel would.
func checkPrivilegedService(ctx context.Context, w io.Writer) {
	section(w, "The privileged service, asked over the socket")
	if !privsvc.EndpointPresent(socketPath) {
		fmt.Fprintf(w, "  Nothing answers at %s, so the privileged service is not running.\n", socketPath)
		fmt.Fprintf(w, "  Start it with: %s\n", layout.StartPrivilegedAdvice)
		return
	}

	c := privsvc.NewClient(socketPath)
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	st, err := c.Status(cctx)
	if err != nil {
		f := panel.FaultOf(err)
		if f == panel.FaultUnavailable {
			fmt.Fprintf(w, "  The socket is there and the service did not answer. Either it is starting,\n")
			fmt.Fprintf(w, "  or this account is not allowed to talk to it: only root and the %q\n", serviceAccount)
			fmt.Fprintf(w, "  account may, whatever the mode on the socket says.\n")
			return
		}
		fmt.Fprintf(w, "  The service answered with a fault: %s\n", string(f))
		return
	}

	fmt.Fprintf(w, "  engine:  %s\n", st.Engine.Phase)
	if st.Engine.Reason != "" {
		fmt.Fprintf(w, "  reason:  %s\n", st.Engine.Reason)
	}
	fmt.Fprintf(w, "  hotspot: running=%t devices=%d", st.Hotspot.Running, st.Hotspot.Devices)
	if st.Hotspot.SSID != "" {
		fmt.Fprintf(w, " ssid=%q", st.Hotspot.SSID)
	}
	if st.Hotspot.Fault != panel.FaultNone {
		fmt.Fprintf(w, " fault=%s", string(st.Hotspot.Fault))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  carrying client traffic: %t\n", st.Connected())
	fmt.Fprintf(w, "  %s\n", panel.DetectedLine(st.Detection))
	writeDetection(w, st.Detection)
}

// checkLocalDetection asks this machine directly, without the service.
//
// This is the vantage that still works when the service will not start, which
// is when somebody reaches for this command.
func checkLocalDetection(ctx context.Context, w io.Writer) {
	section(w, "This machine, measured directly by this command")

	runner := netcfg.NewSystemRunner()
	backend := netcfg.SystemBackend()
	facts, err := backend.Detect(ctx, runner, backend.BaseSysctlKnobs())
	if err != nil {
		fmt.Fprintf(w, "  Could not read this machine: %v\n", err)
		if errors.Is(err, netcfg.ErrUnsupportedPlatform) || errors.Is(err, netcfg.ErrUnsupportedBackend) {
			fmt.Fprintf(w, "  This is %s, and this build has no network backend for it, so only the parts of\n", runtime.GOOS)
			fmt.Fprintf(w, "  this report that do not run a command are meaningful here.\n")
		}
		return
	}

	fmt.Fprintf(w, "  interfaces:\n")
	for _, l := range facts.Links {
		kind := "wired"
		if w2, ok := facts.WirelessByName(l.Name); ok {
			kind = "wireless on " + w2.Phy
			if w2.Channel > 0 {
				kind += fmt.Sprintf(", channel %d", w2.Channel)
			}
		}
		if l.IsLoopback() {
			kind = "loopback"
		}
		fmt.Fprintf(w, "    %-10s %-8s %-28s %s\n", l.Name, l.State, prefixList(l.Prefixes), kind)
	}

	fmt.Fprintf(w, "  default routes:\n")
	if len(facts.Routes) == 0 {
		fmt.Fprintf(w, "    none. Nothing on this machine has a route out, so there is no internet to share.\n")
	}
	for _, r := range facts.Routes {
		gw := "on-link"
		if r.Gateway.IsValid() {
			gw = "via " + r.Gateway.String()
		}
		fmt.Fprintf(w, "    IPv%d %-10s %-22s metric %d\n", r.Family, r.Dev, gw, r.Metric)
	}

	fmt.Fprintf(w, "  radios:\n")
	if len(facts.Phys) == 0 {
		fmt.Fprintf(w, "    none reported. No adapter on this machine can create a hotspot.\n")
	}
	for _, phy := range facts.Phys {
		fmt.Fprintf(w, "    %-6s access point: %t, usable channels: %v\n", phy.Name, phy.SupportsAP(), phy.UsableChannels())
		if ok, combo := phy.APWithStation(); ok {
			pinned := ""
			if combo.Channels == 1 {
				pinned = " (so the hotspot is pinned to the channel any existing WiFi connection is on)"
			}
			fmt.Fprintf(w, "           can run an access point beside a WiFi connection, #channels <= %d%s\n",
				combo.Channels, pinned)
		}
	}

	fmt.Fprintf(w, "  kernel settings that decide whether traffic flows:\n")
	for _, k := range backend.BaseSysctlKnobs() {
		v, ok := facts.Sysctl[k]
		if !ok {
			v = "could not be read"
		}
		fmt.Fprintf(w, "    %-34s %s\n", k, v)
	}

	// The plan, with the loudest possible label on it.
	fmt.Fprintf(w, "\n  What would be chosen, using a PLACEHOLDER server address:\n")
	fmt.Fprintf(w, "    The planner needs the proxy server's address, and this command has no config.\n")
	fmt.Fprintf(w, "    %s is used instead, so the interfaces, channel and subnet below are real and\n", checkPlaceholderServer)
	fmt.Fprintf(w, "    the route to the server is not. Nothing here has been applied.\n")
	plan, err := netcfg.PlanNetwork(facts, []netip.Addr{checkPlaceholderServer}, planOptionsForCheck())
	if err != nil {
		var pe *netcfg.PlanError
		if errors.As(err, &pe) {
			fmt.Fprintf(w, "    No workable arrangement: %s\n", pe.UserMessage())
			return
		}
		fmt.Fprintf(w, "    No workable arrangement: %v\n", err)
		return
	}
	fmt.Fprintf(w, "    %s\n", plan.Explain())
	fmt.Fprintf(w, "    hotspot subnet %s, box at %s, tunnel device %s\n",
		plan.HotspotSubnet, plan.HotspotGateway, plan.Tun)
	for _, n := range plan.Notes {
		fmt.Fprintf(w, "    note: %s\n", n)
	}
}

// checkPlaceholderServer is RFC 5737 TEST-NET-1, used so the planner can run
// with no configuration present. It is never applied.
var checkPlaceholderServer = netip.MustParseAddr("192.0.2.1")

// planOptionsForCheck are the options the privileged service would use, built
// from the same values in paths.go, so that what this command reports is what
// that service would do.
func planOptionsForCheck() netcfg.Options {
	o := netcfg.DefaultOptions()
	o.DNSPort = dnsPort
	o.PanelPort = panelPort
	return o
}

// checkState reports what the panel has stored, without disclosing any of it.
func checkState(w io.Writer) {
	section(w, "Stored settings")
	store, err := state.Load(stateDir)
	if err != nil {
		fmt.Fprintf(w, "  %v\n", err)
		return
	}
	// state.State renders itself redacted through String, and Redacted is that
	// rendering by name. The pasted config appears as a fingerprint and never
	// as any part of itself.
	fmt.Fprintf(w, "  %s\n", store.Snapshot().Redacted())
	if store.NeedsSetup() {
		fmt.Fprintf(w, "\n  Setup is not finished: the panel needs a password and a config before it\n")
		fmt.Fprintf(w, "  can show its normal screen.\n")
	}
}

func writeDetection(w io.Writer, d panel.Detection) {
	if d.Fault != panel.FaultNone {
		fmt.Fprintf(w, "  detection fault: %s\n", string(d.Fault))
	}
	fmt.Fprintf(w, "  internet on %q, hotspot on %q\n", d.InternetInterface, d.HotspotInterface)
	if d.Channel > 0 {
		pinned := ""
		if d.ChannelPinned {
			pinned = " (fixed by the radio)"
		}
		fmt.Fprintf(w, "  channel %d%s, band %s, country %q, usable channels %v\n",
			d.Channel, pinned, d.Band, d.Country, d.UsableChannels)
	}
	if d.Subnet != "" {
		fmt.Fprintf(w, "  hotspot subnet %s\n", d.Subnet)
	}
	if d.HotspotAddress == "" {
		fmt.Fprintf(w, "  the hotspot has no address yet, so the panel cannot serve on it\n")
	} else {
		fmt.Fprintf(w, "  panel on the hotspot: http://%s/\n", d.HotspotAddress+":"+strconv.Itoa(panelPort))
	}
}

func prefixList(ps []netip.Prefix) string {
	if len(ps) == 0 {
		return "-"
	}
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.String())
	}
	return strings.Join(out, " ")
}
