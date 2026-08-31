// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/state"
)

// Design section 5.6: the hotspot interface always, the local network only if
// the user turns it on, never the uplink. A wildcard bind ignores all three
// clauses at once and looks perfectly healthy in testing, which is why it is
// refused rather than warned about.

func TestWildcardBindIsRefused(t *testing.T) {
	wildcards := []string{
		"0.0.0.0:8080",
		"[::]:8080",
		":8080",
		"[::0]:8080",
		"0.0.0.0:80",
	}
	for _, addr := range wildcards {
		t.Run(addr, func(t *testing.T) {
			err := ValidateBindAddr(addr)
			if err == nil {
				t.Fatalf("ValidateBindAddr(%q) allowed a wildcard bind", addr)
			}
			if !errors.Is(err, ErrWildcardBind) {
				t.Errorf("error is %v, want it to wrap ErrWildcardBind", err)
			}
		})
	}
}

func TestPublicAndNonsenseBindsAreRefused(t *testing.T) {
	cases := []struct {
		addr string
		want error
	}{
		// Globally routable, which is the "never the uplink" clause with a
		// definition attached.
		{"93.184.216.34:8080", ErrPublicBind},
		{"[2606:2800:220:1:248:1893:25c8:1946]:8080", ErrPublicBind},
		// A name is refused because what it resolves to is decided elsewhere
		// and can change under the process.
		{"localhost:8080", nil},
		{"caspian.local:8080", nil},
		// Nonsense.
		{"10.0.0.1:0", nil},
		{"10.0.0.1:70000", nil},
		{"10.0.0.1", nil},
		{"", nil},
		{"[ff02::1]:8080", nil},
	}
	for _, c := range cases {
		t.Run(c.addr, func(t *testing.T) {
			err := ValidateBindAddr(c.addr)
			if err == nil {
				t.Fatalf("ValidateBindAddr(%q) allowed it", c.addr)
			}
			if c.want != nil && !errors.Is(err, c.want) {
				t.Errorf("error is %v, want it to wrap %v", err, c.want)
			}
		})
	}
}

// TestNarrowBindsAreAllowed is the control. A validator that refused everything
// would pass every test above and leave the panel unable to listen at all.
func TestNarrowBindsAreAllowed(t *testing.T) {
	for _, addr := range []string{
		"10.62.0.1:8080",    // the hotspot address
		"192.168.1.42:8080", // the local network
		"172.16.5.1:8080",
		"127.0.0.1:8080",      // loopback, for a developer machine
		"[fd00::1]:8080",      // unique local
		"[fe80::1]:8080",      // link local
		"100.64.0.1:8080",     // carrier-grade NAT, not reachable from outside
		"10.62.0.1:" + "1024", //
		"10.62.0.1:65535",     //
	} {
		if err := ValidateBindAddr(addr); err != nil {
			t.Errorf("ValidateBindAddr(%q) refused a legitimate address: %v", addr, err)
		}
	}
}

func TestBindAddrsDefaultsToTheHotspotOnly(t *testing.T) {
	d := Detection{
		HotspotAddress:      "10.62.0.1",
		LocalNetworkAddress: "192.168.1.42",
	}

	got, err := BindAddrs(d, DefaultPort, false)
	if err != nil {
		t.Fatalf("BindAddrs: %v", err)
	}
	wantHotspot := fmt.Sprintf("10.62.0.1:%d", DefaultPort)
	if len(got) != 1 || got[0] != wantHotspot {
		t.Fatalf("default bind is %v, want only the hotspot address", got)
	}

	got, err = BindAddrs(d, DefaultPort, true)
	if err != nil {
		t.Fatalf("BindAddrs with the local network on: %v", err)
	}
	wantLocal := fmt.Sprintf("192.168.1.42:%d", DefaultPort)
	if len(got) != 2 || got[1] != wantLocal {
		t.Fatalf("with the local network on, bind is %v, want the hotspot and the local address", got)
	}
}

// TestBindAddrsRefusesAPublicLocalAddress is the case the "never the uplink"
// clause exists for: a box whose own address is globally routable, where a user
// ticking a box marked "local network" would otherwise publish the panel to the
// internet.
func TestBindAddrsRefusesAPublicLocalAddress(t *testing.T) {
	d := Detection{
		HotspotAddress:      "10.62.0.1",
		LocalNetworkAddress: "93.184.216.34",
	}
	if _, err := BindAddrs(d, DefaultPort, false); err != nil {
		t.Fatalf("with the local network off it should still bind the hotspot: %v", err)
	}
	_, err := BindAddrs(d, DefaultPort, true)
	if err == nil {
		t.Fatal("BindAddrs published the panel on a globally routable address")
	}
	if !errors.Is(err, ErrPublicBind) {
		t.Errorf("error is %v, want it to wrap ErrPublicBind", err)
	}
}

// TestBindAddrsRefusesWhenThereIsNoHotspotYet is the hazard design section 5.6
// names and does not solve: the hotspot interface does not exist until the
// access point starts. The point of the test is that the failure is an error
// rather than a silent fall back to a wildcard, which is the shape this bug
// always takes.
func TestBindAddrsRefusesWhenThereIsNoHotspotYet(t *testing.T) {
	_, err := BindAddrs(Detection{}, DefaultPort, false)
	if err == nil {
		t.Fatal("BindAddrs returned something to listen on with no hotspot address")
	}
	if !errors.Is(err, ErrNoBindAddress) {
		t.Errorf("error is %v, want it to wrap ErrNoBindAddress", err)
	}
}

// TestNewRefusesAWildcardListenAddress puts the check at construction, so a
// misconfigured panel fails to start rather than starting wrong.
func TestNewRefusesAWildcardListenAddress(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = New(Config{
		Store:       store,
		Priv:        NewFakePrivileged(),
		Logger:      slog.New(slog.DiscardHandler),
		ListenAddrs: []string{"10.62.0.1:8080", "0.0.0.0:8080"},
	})
	if err == nil {
		t.Fatal("New accepted a wildcard listen address")
	}
	if !errors.Is(err, ErrWildcardBind) {
		t.Errorf("error is %v, want it to wrap ErrWildcardBind", err)
	}

	// The control: the same call without the wildcard succeeds.
	p, err := New(Config{
		Store:       store,
		Priv:        NewFakePrivileged(),
		Logger:      slog.New(slog.DiscardHandler),
		ListenAddrs: []string{"10.62.0.1:8080"},
	})
	if err != nil {
		t.Fatalf("New refused a narrow listen address: %v", err)
	}
	if got := p.ListenAddrs(); len(got) != 1 || got[0] != "10.62.0.1:8080" {
		t.Errorf("ListenAddrs is %v", got)
	}
}

// TestListenRefusesAWildcardBeforeOpeningASocket checks the last point before a
// socket exists, rather than trusting the caller validated it.
func TestListenRefusesAWildcardBeforeOpeningASocket(t *testing.T) {
	ln, err := Listen("0.0.0.0:8080")
	if err == nil {
		ln.Close()
		t.Fatal("Listen opened a wildcard socket")
	}
	if !errors.Is(err, ErrWildcardBind) {
		t.Errorf("error is %v, want it to wrap ErrWildcardBind", err)
	}

	// The control: a narrow bind really does open, so the refusal above is
	// about the address and not about Listen being broken.
	//
	// A free port has to be found first, because ValidateBindAddr refuses port
	// 0 on purpose: the installer prints an address for the user to type, and a
	// port the kernel picks afresh at every boot is one nobody can find. There
	// is a small race between closing the probe listener and reopening the
	// port, which is acceptable in a test and is the reason the port is not
	// hardcoded, since a fixed port collides with whatever else is running.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}
	freeAddr := probe.Addr().String()
	probe.Close()

	ln, err = Listen(freeAddr)
	if err != nil {
		t.Fatalf("Listen refused a loopback address %s: %v", freeAddr, err)
	}
	defer ln.Close()
	if !strings.HasPrefix(ln.Addr().String(), "127.0.0.1:") {
		t.Errorf("listening on %s", ln.Addr())
	}
}

// TestPortZeroIsRefused pins the decision above, so that somebody who finds it
// inconvenient in a test changes it deliberately rather than by accident.
func TestPortZeroIsRefused(t *testing.T) {
	if err := ValidateBindAddr("10.62.0.1:0"); err == nil {
		t.Fatal("port 0 was accepted; the panel would move port at every boot")
	}
}
