// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	xtun "github.com/xtls/xray-core/proxy/tun"
)

// TestTheTunInboundReleasesItselfOnClose is the guard under tundevice.go.
//
// Until xray-core v26.4.15 the proxy object that owns the device,
// proxy/tun.Handler, had no Close method, so core.Instance.Close reached
// nothing that released the device and this package removed it itself. That
// commit (c5edc122b70e) gave Handler a Close that closes the gVisor stack and
// the device, and app/proxyman/inbound's AlwaysOnInboundHandler.Close calls
// common.Close on its proxy, which is how the instance's Close reaches it.
//
// Both halves are checked: the method through reflection, the call through
// the engine's own source in the module cache, because a method nobody calls
// releases nothing. tundevice.go's release path is kept as the measured
// safety net (TestReleaseIsSafeWhenTheDeviceIsAlreadyGone): on the appliance
// the device is now gone before it runs, and it finds nothing to remove.
func TestTheTunInboundReleasesItselfOnClose(t *testing.T) {
	typ := reflect.TypeOf(&xtun.Handler{})
	m, ok := typ.MethodByName("Close")
	if !ok {
		t.Fatalf("proxy/tun.Handler has no Close method; the engine in go.mod is older than v26.4.15 and tundevice.go is the only thing releasing the device")
	}
	if m.Type.NumIn() != 1 || m.Type.NumOut() != 1 || m.Type.Out(0).String() != "error" {
		t.Fatalf("proxy/tun.Handler.Close has the shape %v, not func() error", m.Type)
	}

	dir := xrayModuleDir(t)
	always, err := os.ReadFile(filepath.Join(dir, "app", "proxyman", "inbound", "always.go"))
	if err != nil {
		t.Fatalf("reading the inbound manager's source: %v", err)
	}
	if !strings.Contains(string(always), "common.Close(h.proxy)") {
		t.Fatal("app/proxyman/inbound/always.go no longer closes the proxy from AlwaysOnInboundHandler.Close, " +
			"so Handler.Close is not reached from core.Instance.Close and tundevice.go is load bearing again")
	}
}

// xrayModuleDir is where the engine's source is, asked of the toolchain
// rather than assumed, so a replace directive or a different cache location
// still points here.
func xrayModuleDir(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/xtls/xray-core").Output()
	if err != nil {
		t.Skipf("go list is not available here: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		t.Skip("go list reported no directory for the engine module")
	}
	return dir
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
