// SPDX-License-Identifier: AGPL-3.0-or-later

package privsvc

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"caspianbyoc.org/caspian/internal/netcfg"
)

type windowsReadbackRunner func(context.Context, netcfg.Command) (netcfg.Result, error)

func (f windowsReadbackRunner) Run(ctx context.Context, c netcfg.Command) (netcfg.Result, error) {
	return f(ctx, c)
}

func TestWindowsHotspotReadbackWaitsForAdapterAndGateway(t *testing.T) {
	const down = `{"adapters":[{"alias":"Local Area Connection* 13","wifiDirect":true,"up":false}]}`
	const noAddress = `{"adapters":[{"alias":"Local Area Connection* 14","wifiDirect":true,"up":true}]}`
	const ready = `{"adapters":[{"alias":"Local Area Connection* 13","wifiDirect":true,"up":false},{"alias":"Local Area Connection* 14","wifiDirect":true,"up":true,"prefixes":["192.168.137.1/24"]}]}`
	readFailure := errors.New("adapter inventory unavailable")
	for _, tc := range []struct {
		name    string
		states  []string
		readErr error
		wantErr error
		waits   int
	}{
		{"already ready", []string{ready}, nil, nil, 0},
		{"adapter then gateway", []string{down, noAddress, ready}, nil, nil, 2},
		{"never ready", []string{down}, nil, netcfg.ErrNotAccessPoint, 40},
		{"inventory failure", nil, readFailure, readFailure, 0},
		{"cancelled read", nil, context.Canceled, context.Canceled, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			reads := 0
			w.svc.cfg.Runner = windowsReadbackRunner(func(_ context.Context, _ netcfg.Command) (netcfg.Result, error) {
				reads++
				if tc.readErr != nil {
					return netcfg.Result{}, tc.readErr
				}
				i := min(reads-1, len(tc.states)-1)
				return netcfg.Result{Stdout: tc.states[i]}, nil
			})
			plan := &netcfg.Plan{Hotspot: "Wi-Fi 3", HotspotGateway: netip.MustParseAddr("192.168.137.1"), Platform: netcfg.PlatformWindows}
			err := w.svc.assertHotspotIsAccessPoint(context.Background(), plan, "Caspian")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if len(w.sys.Sleeps) != tc.waits {
				t.Fatalf("waits = %d, want %d", len(w.sys.Sleeps), tc.waits)
			}
			if reads != tc.waits+1 {
				t.Fatalf("reads = %d, want %d", reads, tc.waits+1)
			}
		})
	}
}
