// SPDX-License-Identifier: AGPL-3.0-or-later
package privsvc

import (
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/snispoof"
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

type testSNIForwarder struct{ closed int }

func (f *testSNIForwarder) Addr() netip.AddrPort { return netip.MustParseAddrPort("127.0.0.1:12345") }
func (f *testSNIForwarder) Close() error         { f.closed++; return nil }
func TestSNIServicePreservesIdentityAndOwnsLifecycle(t *testing.T) {
	w := newWorld(t)
	req := startRequest(t)
	req.SpoofSNI = "cover.example.invalid"
	var forwards []*testSNIForwarder
	w.svc.cfg.StartSNI = func(remote netip.AddrPort, iface string, options snispoof.Options) (SNIForwarder, error) {
		if !remote.Addr().Is4() || remote.Addr().IsLoopback() || remote.Port() != 443 || iface == "" || options.FakeSNI != req.SpoofSNI {
			t.Fatalf("wrong SNI start arguments %v %s %s", remote, iface, options.FakeSNI)
		}
		f := &testSNIForwarder{}
		forwards = append(forwards, f)
		return f, nil
	}
	ctx := context.Background()
	if err := w.svc.Start(ctx, req); err != nil {
		t.Fatal(err)
	}
	docs := w.eng.documents()
	doc := string(docs[len(docs)-1])
	if !strings.Contains(doc, fakeSNI) || !strings.Contains(doc, fakeUUID) || !strings.Contains(doc, `"address": "127.0.0.1"`) {
		t.Fatal("engine document lost identity or endpoint")
	}
	if err := w.svc.Start(ctx, req); err != nil {
		t.Fatal(err)
	}
	if len(forwards) != 1 {
		t.Fatal("idempotent start recreated forwarder")
	}
	req.SpoofSNI = "other.example.invalid"
	if err := w.svc.Start(ctx, req); err != nil {
		t.Fatal(err)
	}
	if len(forwards) != 2 || forwards[0].closed != 1 {
		t.Fatal("replacement leaked old forwarder")
	}
	if err := w.svc.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if forwards[1].closed != 1 {
		t.Fatal("stop leaked forwarder")
	}
}
func TestSNIServiceRollsBackFailures(t *testing.T) {
	for _, kind := range []string{"open", "engine"} {
		t.Run(kind, func(t *testing.T) {
			w := newWorld(t)
			req := startRequest(t)
			req.SpoofSNI = "cover.example.invalid"
			f := &testSNIForwarder{}
			w.svc.cfg.StartSNI = func(netip.AddrPort, string, snispoof.Options) (SNIForwarder, error) {
				if kind == "open" {
					return nil, snispoof.ErrUnavailable
				}
				return f, nil
			}
			if kind == "engine" {
				w.eng.startErr = errors.New("injected failure")
			}
			if w.svc.Start(context.Background(), req) == nil {
				t.Fatal("failure hidden")
			}
			if kind == "engine" && f.closed != 1 {
				t.Fatal("rollback leaked forwarder")
			}
			if w.svc.sniForwarder != nil {
				t.Fatal("rollback retained forwarder")
			}
		})
	}
}
func TestSNIServiceRejectsInvalidNameBeforeMutation(t *testing.T) {
	w := newWorld(t)
	req := startRequest(t)
	req.SpoofSNI = "bad/name"
	if w.svc.Start(context.Background(), req) == nil {
		t.Fatal("accepted invalid name")
	}
	if len(w.mutatingCommands()) != 0 {
		t.Fatal("mutated before validation")
	}
}

func TestSNIPreferenceCrossesTheServiceTransport(t *testing.T) {
	w := newWorld(t)
	req := startRequest(t)
	req.SpoofSNI = "cover.example.invalid"
	l, err := link.Parse("vless://" + fakeUUID + "@" + fakeHost + ":443?security=tls&type=tcp&sni=" + fakeSNI)
	if err != nil {
		t.Fatal(err)
	}
	req.ConfigJSON, err = l.XrayConfig()
	if err != nil {
		t.Fatal(err)
	}
	req.TCPSplit = true
	req.TLSRecordSplit = true
	f := &testSNIForwarder{}
	called := false
	w.svc.cfg.StartSNI = func(_ netip.AddrPort, _ string, options snispoof.Options) (SNIForwarder, error) {
		called = true
		if options.FakeSNI != req.SpoofSNI || !options.TCPSplit || !options.TLSRecordSplit {
			t.Error("wire lost spoof name")
		}
		return f, nil
	}
	path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t)})
	client := NewClient(path)
	if err := client.Start(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("wire did not start forwarder")
	}
	if err := client.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.closed != 1 {
		t.Fatal("wire stop leaked forwarder")
	}
}

func TestSNIUnsupportedBackendIsReportedAsUnsupported(t *testing.T) {
	w := newWorld(t)
	req := startRequest(t)
	req.SpoofSNI = "cover.example.invalid"
	w.svc.cfg.StartSNI = func(netip.AddrPort, string, snispoof.Options) (SNIForwarder, error) {
		return nil, snispoof.ErrUnsupported
	}
	err := w.svc.Start(context.Background(), req)
	if faultOf(err) != panel.FaultSNISpoofUnsupported {
		t.Fatalf("wrong fault: %v", err)
	}
}

func TestSplitServiceRestartsForEachIndependentChoice(t *testing.T) {
	w := newWorld(t)
	req := startRequest(t)
	l, err := link.Parse("vless://" + fakeUUID + "@" + fakeHost + ":443?security=tls&type=tcp&sni=" + fakeSNI)
	if err != nil {
		t.Fatal(err)
	}
	req.ConfigJSON, err = l.XrayConfig()
	if err != nil {
		t.Fatal(err)
	}
	var forwards []*testSNIForwarder
	var got snispoof.Options
	w.svc.cfg.StartSNI = func(_ netip.AddrPort, _ string, o snispoof.Options) (SNIForwarder, error) {
		got = o
		f := &testSNIForwarder{}
		forwards = append(forwards, f)
		return f, nil
	}
	for _, o := range []snispoof.Options{{TCPSplit: true}, {TLSRecordSplit: true}, {TCPSplit: true, TLSRecordSplit: true}, {FakeSNI: "cover.example.invalid", TCPSplit: true, TLSRecordSplit: true}} {
		req.SpoofSNI = o.FakeSNI
		req.TCPSplit = o.TCPSplit
		req.TLSRecordSplit = o.TLSRecordSplit
		if err = w.svc.Start(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		if got != o {
			t.Fatal("service lost settings")
		}
		if err = w.svc.Start(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	if len(forwards) != 4 {
		t.Fatal("split settings missing from request fingerprint")
	}
	if err = w.svc.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, f := range forwards {
		if f.closed != 1 {
			t.Fatal("forwarder not closed exactly once")
		}
	}
}
func TestSplitServiceRejectsRealityBeforeMutation(t *testing.T) {
	w := newWorld(t)
	req := startRequest(t)
	req.TCPSplit = true
	if faultOf(w.svc.Start(context.Background(), req)) != panel.FaultSNISpoofUnsupported {
		t.Fatal("accepted unvalidated REALITY splitting")
	}
	if len(w.mutatingCommands()) != 0 {
		t.Fatal("mutated before validation")
	}
}
