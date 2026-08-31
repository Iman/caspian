// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"reflect"
	"testing"

	xtun "github.com/xtls/xray-core/proxy/tun"
)

// TestTheTunInboundStillHasNoCloseMethod is the guard under the whole of
// tundevice.go.
//
// This package removes the TUN device itself only because xray-core does not.
// The proxy object that owns the device, proxy/tun.Handler, has no Close
// method at all, so the inbound handler manager closing its handlers
// (app/proxyman/inbound Manager.Close, then AlwaysOnInboundHandler.Close,
// which closes workers and mux) reaches nothing that would release it. The TUN
// inbound has no workers either: infra/conf/xray.go skips the port branch for
// protocol "tun", so ReceiverConfig.PortList is nil and no worker is built.
//
// If this test fails, xray-core has gained a Close on that type and the
// workaround in tundevice.go may now be unnecessary or, worse, duplicated. Do
// not delete the release code on the strength of the method existing: check
// that something CALLS it during core.Instance.Close, then delete
// Engine.releaseTunDevices and this test together.
func TestTheTunInboundStillHasNoCloseMethod(t *testing.T) {
	typ := reflect.TypeOf(&xtun.Handler{})
	if m, ok := typ.MethodByName("Close"); ok {
		t.Fatalf("proxy/tun.Handler now has a %v method: re-read the comment above this test before changing anything", m.Type)
	}

	// Naming the methods that ARE there makes the failure above readable: it
	// says what the type looks like now, not just what is missing.
	var got []string
	for i := 0; i < typ.NumMethod(); i++ {
		got = append(got, typ.Method(i).Name)
	}
	t.Logf("proxy/tun.Handler exported methods: %v", got)
}

// TestTheLoaderDefaultsTheTunDeviceName pins the behaviour tunInboundNames
// relies on: by the time a config reaches this package the device name is
// never empty, because infra/conf/tun.go substitutes "xray0" while building
// the protobuf. tunInboundNames skips an empty name rather than guessing at
// it, so if this default ever went away the engine would stop releasing the
// device instead of deleting one by the wrong name.
func TestTheLoaderDefaultsTheTunDeviceName(t *testing.T) {
	cfg, err := loadConfig([]byte(`{
  "inbounds": [{"tag": "tun-in", "protocol": "tun", "settings": {}}],
  "outbounds": [{"protocol": "freedom", "settings": {}}]
}`))
	if err != nil {
		t.Fatalf("loading a tun config with no name: %v", err)
	}
	got := tunInboundNames(cfg)
	want := []string{"xray0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tunInboundNames = %v, want %v", got, want)
	}
}

// TestTunInboundNamesFindsEveryTunInbound. The engine releases what it
// created, so missing one device would leave exactly the defect this package
// exists to remove.
func TestTunInboundNamesFindsEveryTunInbound(t *testing.T) {
	cfg, err := loadConfig([]byte(`{
  "inbounds": [
    {"tag": "a", "protocol": "tun", "settings": {"name": "csp-a", "MTU": 1500}},
    {"tag": "s", "listen": "127.0.0.1", "port": 1080, "protocol": "socks", "settings": {"auth": "noauth"}},
    {"tag": "b", "protocol": "tun", "settings": {"name": "csp-b", "MTU": 1500}}
  ],
  "outbounds": [{"protocol": "freedom", "settings": {}}]
}`))
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	got := tunInboundNames(cfg)
	want := []string{"csp-a", "csp-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tunInboundNames = %v, want %v", got, want)
	}
}

// TestTunInboundNamesOnAConfigWithNoTunnel. A config with the TUN inbound
// switched off must produce no device to release, on every platform.
func TestTunInboundNamesOnAConfigWithNoTunnel(t *testing.T) {
	cfg, err := loadConfig([]byte(`{
  "inbounds": [{"tag": "s", "listen": "127.0.0.1", "port": 1080, "protocol": "socks", "settings": {"auth": "noauth"}}],
  "outbounds": [{"protocol": "freedom", "settings": {}}]
}`))
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if got := tunInboundNames(cfg); got != nil {
		t.Fatalf("tunInboundNames = %v, want none", got)
	}
	if got := tunInboundNames(nil); got != nil {
		t.Fatalf("tunInboundNames(nil) = %v, want none", got)
	}
}

// TestReleasingNothingIsNotAnError covers the two shapes Stop can hold when no
// tunnel was ever created: a nil hold, from a start that failed before
// core.New, and an empty one, from a config with no TUN inbound.
func TestReleasingNothingIsNotAnError(t *testing.T) {
	var nilHold *tunHold
	if err := nilHold.release(); err != nil {
		t.Errorf("releasing a nil hold: %v", err)
	}
	empty := &tunHold{}
	if err := empty.release(); err != nil {
		t.Errorf("releasing an empty hold: %v", err)
	}
}

// TestStopReleasesTheHoldEvenWithNoInstance pins the invariant Stop's early
// return depends on: whatever else it does, Stop gives the device back and
// leaves no hold behind. A hold that outlived a Stop would be a device nobody
// will ever delete.
func TestStopReleasesTheHoldEvenWithNoInstance(t *testing.T) {
	e := New()
	e.tun = &tunHold{}
	if err := e.Stop(); err != nil {
		t.Fatalf("Stop with a hold and no instance: %v", err)
	}
	if e.tun != nil {
		t.Error("Stop left the tunnel hold in place")
	}
	if got := e.State().Phase; got != PhaseStopped {
		t.Errorf("phase after Stop = %v, want stopped", got)
	}
}
