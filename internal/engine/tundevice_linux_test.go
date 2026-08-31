// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package engine

// The TUN device lifecycle, tested against a real kernel.
//
// # These tests SKIP unless they are run as root on Linux with /dev/net/tun
//
// A skipped run says NOTHING about the device. The skip messages say so in
// those words, because a green run on a developer machine that skipped all of
// this is not evidence of anything and must not read like a pass. The gate for
// this file is a run on the appliance.
//
// # What they are for
//
// xray-core creates the TUN device inside core.New and never closes it: the
// proxy object it belongs to has no Close method, and the inbound handler that
// owns that object closes only its workers and its mux
// (app/proxyman/inbound/always.go, AlwaysOnInboundHandler.Close). So the
// device, the descriptor and the whole gVisor stack outlive the instance
// unless this package removes them. Measured on the appliance on 2026-08-30:
// after Stop the device was still UP with persist off, one descriptor still
// open on /dev/net/tun, and the next Start failed with "device or resource
// busy".
//
// Every device these tests create is named csp-t*, never xray0, so a run on
// the appliance cannot touch the appliance's own device.

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// skipUnlessTunCapable skips loudly. The message names what was not proven.
func skipUnlessTunCapable(t *testing.T, dev string) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skipf("SKIPPED and PROVES NOTHING about the TUN device: this test needs root and /dev/net/tun. Run it on the appliance.")
	}
	if _, err := os.Stat("/dev/net/tun"); err != nil {
		t.Skipf("SKIPPED and PROVES NOTHING about the TUN device: no /dev/net/tun (%v). Run it on the appliance.", err)
	}
	if deviceExists(dev) {
		t.Fatalf("%s exists before the test started; a previous run left it behind", dev)
	}
	t.Cleanup(func() {
		if deviceExists(dev) {
			// Best effort, so that one failing test does not block the next
			// run. A test that needs this has already failed.
			_ = exec.Command("ip", "link", "delete", dev).Run()
		}
	})
}

// deviceExists reports whether the kernel currently has a device of this name.
func deviceExists(name string) bool {
	_, err := os.Stat(filepath.Join("/sys/class/net", name))
	return err == nil
}

// openTunDescriptors counts this process's open descriptors on /dev/net/tun.
// The engine's leak is one of these per start, so the count is the assertion.
func openTunDescriptors(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatalf("reading /proc/self/fd: %v", err)
	}
	n := 0
	for _, e := range ents {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name()))
		if err != nil {
			continue
		}
		if target == "/dev/net/tun" {
			n++
		}
	}
	return n
}

// tunOnlyConfig is a document whose only inbound is the TUN inbound.
func tunOnlyConfig(dev string) []byte {
	return []byte(fmt.Sprintf(`{
  "log": {"loglevel": "warning"},
  "inbounds": [{
    "tag": "tun-in",
    "protocol": "tun",
    "settings": {"name": %q, "MTU": 1500, "userLevel": 0}
  }],
  "outbounds": [{"tag": "direct", "protocol": "freedom", "settings": {}}]
}`, dev))
}

// tunPlusBusyPort makes inst.Start fail AFTER core.New has created the device,
// by pointing a socks inbound at a port somebody else already holds.
func tunPlusBusyPort(dev string, port int) []byte {
	return []byte(fmt.Sprintf(`{
  "log": {"loglevel": "warning"},
  "inbounds": [
    {"tag": "tun-in", "protocol": "tun", "settings": {"name": %q, "MTU": 1500, "userLevel": 0}},
    {"tag": "socks-in", "listen": "127.0.0.1", "port": %d, "protocol": "socks", "settings": {"auth": "noauth", "udp": false}}
  ],
  "outbounds": [{"tag": "direct", "protocol": "freedom", "settings": {}}]
}`, dev, port))
}

// tunPlusDuplicateTag makes core.New itself fail after the TUN inbound has
// been added, because app/proxyman/inbound Manager.AddHandler refuses a tag it
// already holds. There is no instance to close on this path, so it is the one
// leak the instance lifecycle cannot cover.
func tunPlusDuplicateTag(dev string, portA, portB int) []byte {
	return []byte(fmt.Sprintf(`{
  "log": {"loglevel": "warning"},
  "inbounds": [
    {"tag": "tun-in", "protocol": "tun", "settings": {"name": %q, "MTU": 1500, "userLevel": 0}},
    {"tag": "same", "listen": "127.0.0.1", "port": %d, "protocol": "socks", "settings": {"auth": "noauth", "udp": false}},
    {"tag": "same", "listen": "127.0.0.1", "port": %d, "protocol": "socks", "settings": {"auth": "noauth", "udp": false}}
  ],
  "outbounds": [{"tag": "direct", "protocol": "freedom", "settings": {}}]
}`, dev, portA, portB))
}

// waitGone polls, because the kernel removes the device asynchronously once
// the last reference goes. A single immediate check is racy in the direction
// that makes a broken release look fine.
func waitGone(t *testing.T, name string, d time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if !deviceExists(name) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestStopReleasesTheTunDeviceItCreated is the reproduction of the defect
// measured on the appliance on 2026-08-30.
func TestStopReleasesTheTunDeviceItCreated(t *testing.T) {
	const dev = "csp-t0"
	skipUnlessTunCapable(t, dev)

	before := openTunDescriptors(t)
	e := New()

	if err := e.Start(context.Background(), tunOnlyConfig(dev)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = e.Stop() })

	if !deviceExists(dev) {
		t.Fatalf("engine reports running but %s does not exist, so the rest of this test would prove nothing", dev)
	}
	if got := openTunDescriptors(t); got != before+1 {
		t.Fatalf("descriptors on /dev/net/tun while running = %d, want %d; this test cannot see what it is measuring", got, before+1)
	}

	if err := e.Stop(); err != nil {
		t.Errorf("Stop: %v", err)
	}

	if !waitGone(t, dev, 5*time.Second) {
		t.Errorf("after Stop the device %s is still present", dev)
	}
	if got := openTunDescriptors(t); got != before {
		t.Errorf("after Stop the process holds %d descriptors on /dev/net/tun, want %d", got, before)
	}
}

// TestAFailedStartReleasesTheTunDevice covers the path where core.New has
// already created the device and inst.Start then fails.
func TestAFailedStartReleasesTheTunDevice(t *testing.T) {
	const dev = "csp-t1"
	skipUnlessTunCapable(t, dev)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	before := openTunDescriptors(t)
	e := New()

	err = e.Start(context.Background(), tunPlusBusyPort(dev, port))
	if err == nil {
		t.Fatalf("the start was expected to fail on the busy port; this test proves nothing if it succeeded")
	}
	t.Logf("Start failed as intended: %v", err)

	if !waitGone(t, dev, 5*time.Second) {
		t.Errorf("a start that failed left the device %s behind", dev)
	}
	if got := openTunDescriptors(t); got != before {
		t.Errorf("a start that failed left %d descriptors on /dev/net/tun, want %d", got, before)
	}
}

// TestAFailedConstructionReleasesTheTunDevice covers the path where core.New
// itself fails after the TUN inbound was built. There is no instance to close.
func TestAFailedConstructionReleasesTheTunDevice(t *testing.T) {
	const dev = "csp-t2"
	skipUnlessTunCapable(t, dev)

	before := openTunDescriptors(t)
	e := New()

	err := e.Start(context.Background(), tunPlusDuplicateTag(dev, freeLoopbackPort(t), freeLoopbackPort(t)))
	if err == nil {
		t.Fatalf("the start was expected to fail on the duplicate tag; this test proves nothing if it succeeded")
	}
	t.Logf("Start failed as intended: %v", err)

	if !waitGone(t, dev, 5*time.Second) {
		t.Errorf("a construction that failed left the device %s behind", dev)
	}
	if got := openTunDescriptors(t); got != before {
		t.Errorf("a construction that failed left %d descriptors on /dev/net/tun, want %d", got, before)
	}
}

// TestTheEngineRestartsAfterAStop is the behaviour the appliance actually
// needs: the panel switched off and then on again. Before the release code
// existed this failed with "device or resource busy", three times on the
// appliance on 2026-08-30 between 13:02:48 and 13:02:54.
func TestTheEngineRestartsAfterAStop(t *testing.T) {
	const dev = "csp-t3"
	skipUnlessTunCapable(t, dev)

	ctx := context.Background()
	before := openTunDescriptors(t)

	first := New()
	if err := first.Start(ctx, tunOnlyConfig(dev)); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := first.Stop(); err != nil {
		t.Errorf("first Stop: %v", err)
	}

	second := New()
	if err := second.Start(ctx, tunOnlyConfig(dev)); err != nil {
		t.Fatalf("second Start after a Stop: %v", err)
	}
	t.Cleanup(func() { _ = second.Stop() })
	if !deviceExists(dev) {
		t.Fatalf("the second start reported success but %s does not exist", dev)
	}

	if err := second.Stop(); err != nil {
		t.Errorf("second Stop: %v", err)
	}
	if !waitGone(t, dev, 5*time.Second) {
		t.Errorf("after the second Stop the device %s is still present", dev)
	}
	if got := openTunDescriptors(t); got != before {
		t.Errorf("after two full cycles the process holds %d descriptors on /dev/net/tun, want %d", got, before)
	}
}

// TestReleaseLeavesAnotherInterfaceOfTheSameNameAlone is the guard on the
// deletion.
//
// The device name comes from a configuration field the user can set. Deleting
// an interface because it carries the name you expected is how you delete
// somebody else's interface, so the engine records the interface index when it
// creates the device and refuses to delete anything else. This test builds
// exactly that situation: a device with the right name and the wrong index.
func TestReleaseLeavesAnotherInterfaceOfTheSameNameAlone(t *testing.T) {
	const dev = "csp-t4"
	skipUnlessTunCapable(t, dev)

	// A device this engine did NOT create, standing where its own device was.
	if out, err := exec.Command("ip", "tuntap", "add", "dev", dev, "mode", "tun").CombinedOutput(); err != nil {
		t.Fatalf("creating the stand-in device: %v: %s", err, out)
	}
	index, err := lookupLinkIndex(dev)
	if err != nil {
		t.Fatalf("reading the index of the stand-in device: %v", err)
	}

	// A hold that believes it created a device of that name, with an index
	// that is not this one.
	hold := &tunHold{devices: []tunDevice{{name: dev, index: index + 1000}}}

	err = hold.release()
	if err == nil {
		t.Errorf("releasing a hold whose device was replaced reported success; it should say what it refused to do")
	} else {
		t.Logf("release reported, as it should: %v", err)
	}
	if !deviceExists(dev) {
		t.Fatalf("release deleted %s, an interface this engine did not create", dev)
	}
	if now, err := lookupLinkIndex(dev); err != nil || now != index {
		t.Fatalf("the stand-in device changed: index %d err %v, want %d", now, err, index)
	}
}

// TestReleaseIsSafeWhenTheDeviceIsAlreadyGone. Something else removing the
// device is the state release wants, so it is not an error.
func TestReleaseIsSafeWhenTheDeviceIsAlreadyGone(t *testing.T) {
	const dev = "csp-t5"
	skipUnlessTunCapable(t, dev)

	hold := &tunHold{devices: []tunDevice{{name: dev, index: 999999}}}
	if err := hold.release(); err != nil {
		t.Errorf("releasing a device that is not there: %v", err)
	}
}
