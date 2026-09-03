// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Platform names the operating system a Plan is made for and applied on.
//
// It is a value and not a build tag on purpose. Every backend compiles and is
// tested on every development machine, exactly as the Linux half has been
// tested on macOS since the project began. Only the runner that executes
// commands is tied to the machine it runs on; see exec_linux.go and the files
// beside it.
type Platform string

const (
	// PlatformLinux is the Raspberry Pi appliance, and the meaning of the empty
	// string: every Plan and every Options value made before this seam existed
	// was a Linux one.
	PlatformLinux Platform = "linux"

	// PlatformDarwin is macOS. The access point is Apple's Internet Sharing on
	// the built-in radio, the firewall is pf, and routes come from route(8).
	PlatformDarwin Platform = "darwin"

	// PlatformWindows is Windows 11. The access point is Mobile Hotspot, the
	// firewall is the Windows Filtering Platform, and routes come from the IP
	// Helper API.
	PlatformWindows Platform = "windows"
)

// Backend is the part of this package that differs per operating system: how
// the machine is read, what commands a plan turns into, and how the result is
// read back from the kernel.
//
// The pure half is shared and does not know which backend produced the
// commands it applies: subnet choice, the decision ladder in PlanNetwork, the
// journal of inverses, the idempotence rules and the Applier all work on
// Command values whatever program those name.
type Backend interface {
	Platform() Platform

	// Detect reads the machine and changes nothing. knobs are the sysctl names
	// whose current values a plan needs, so that every change it makes has an
	// exact inverse.
	Detect(ctx context.Context, r Runner, knobs []string) (Facts, error)

	// BaseSysctlKnobs are the knobs detection reads before any plan exists.
	BaseSysctlKnobs() []string

	// The steps a plan turns into. Plan.PreEngineSteps and its siblings say
	// what each list means. The ORDER inside each list belongs to the backend
	// and is load bearing; internal/privsvc must not reorder them.
	PreEngineSteps(p *Plan, current map[string]string) []Step
	PostEngineSteps(p *Plan, current map[string]string) []Step
	CutStep(p *Plan) Step
	RestoreStep(p *Plan) Step
	SysctlKnobs(p *Plan) []string

	// Readbacks. Applying a step is not the same as the kernel having done it,
	// and internal/privsvc refuses to bind anything to the hotspot interface
	// until the first has passed, and refuses to report running until the
	// second has.
	AssertHotspotInterfaceReleased(ctx context.Context, r Runner, p *Plan) error
	AssertHotspotIsAccessPoint(ctx context.Context, r Runner, p *Plan, ssid string) error

	// RegulatoryDomain reports the two-letter country the radio is regulated
	// under, when the platform can say. false means "not known here", never
	// "no regulation".
	RegulatoryDomain(ctx context.Context, r Runner) (string, bool)

	// AllowedBinaries is the closed set of programs the runner for this
	// platform may execute. ValidateCommandOn enforces it.
	AllowedBinaries() map[string]bool
}

// BackendFor returns the backend for a platform. The empty string is Linux.
//
// A platform this build has no backend for gets one that refuses everything,
// for the reason exec_other.go gives: a backend that returned empty step lists
// would make an apply report success having changed nothing.
func BackendFor(pl Platform) Backend {
	if pl == "" {
		pl = PlatformLinux
	}
	if b, ok := backends[pl]; ok {
		return b
	}
	return unsupportedBackend{pl: pl}
}

// backends is the registry. Each platform file registers its backend from an
// init function, so adding a platform adds a file and edits nothing here.
var backends = map[Platform]Backend{PlatformLinux: linuxBackend{}}

func registerBackend(b Backend) { backends[b.Platform()] = b }

// ErrUnsupportedBackend is returned by every method of the backend for a
// platform this build does not implement.
var ErrUnsupportedBackend = errors.New("netcfg: this build has no network backend for that platform")

// ValidateCommandOn is ValidateCommand against the allowlist of one platform.
// The Linux runner keeps calling ValidateCommand, whose list has not moved.
func ValidateCommandOn(pl Platform, c Command) error {
	if c.IsZero() {
		return errors.New("netcfg: empty command")
	}
	if !BackendFor(pl).AllowedBinaries()[c.Path] {
		return fmt.Errorf("%w: %q", ErrDisallowedBinary, c.Path)
	}
	for i, a := range c.Args {
		if strings.ContainsRune(a, 0) {
			return fmt.Errorf("netcfg: argument %d of %q contains a NUL byte", i, c.Path)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Linux. Every method is the function this package had before the seam
// existed, under its old name with "linux" in front, so the measured
// behaviour and its tests are untouched.
// ---------------------------------------------------------------------------

type linuxBackend struct{}

func (linuxBackend) Platform() Platform { return PlatformLinux }

func (linuxBackend) Detect(ctx context.Context, r Runner, knobs []string) (Facts, error) {
	return Detect(ctx, r, knobs)
}

func (linuxBackend) BaseSysctlKnobs() []string { return BaseSysctlKnobs() }

func (linuxBackend) PreEngineSteps(p *Plan, current map[string]string) []Step {
	return p.linuxPreEngineSteps(current)
}

func (linuxBackend) PostEngineSteps(p *Plan, current map[string]string) []Step {
	return p.linuxPostEngineSteps(current)
}

func (linuxBackend) CutStep(p *Plan) Step     { return p.linuxCutStep() }
func (linuxBackend) RestoreStep(p *Plan) Step { return p.linuxRestoreStep() }
func (linuxBackend) SysctlKnobs(*Plan) []string {
	return BaseSysctlKnobs()
}

func (linuxBackend) AssertHotspotInterfaceReleased(ctx context.Context, r Runner, p *Plan) error {
	return linuxAssertHotspotInterfaceReleased(ctx, r, p)
}

func (linuxBackend) AssertHotspotIsAccessPoint(ctx context.Context, r Runner, p *Plan, ssid string) error {
	return linuxAssertHotspotIsAccessPoint(ctx, r, p, ssid)
}

// RegulatoryDomain reads the country out of "iw reg get".
func (linuxBackend) RegulatoryDomain(ctx context.Context, r Runner) (string, bool) {
	res, err := r.Run(ctx, Command{
		Path: BinIw, Args: []string{"reg", "get"},
		Why: "the regulatory domain, which hostapd needs before it will beacon on any channel",
	})
	if err != nil {
		return "", false
	}
	return ParseRegDomain(res.Stdout)
}

func (linuxBackend) AllowedBinaries() map[string]bool { return allowedBinaries }

// ParseRegDomain reads the country out of "iw reg get".
//
// The output looks like:
//
//	global
//	country GB: DFS-ETSI
//		(2402 - 2482 @ 40), (N/A, 20), (N/A)
//
// and on a box where nothing has set a domain the country line reads
// "country 00: DFS-UNSET". A machine with several phys prints a block per phy;
// the first country line that is not the world domain is taken, because a
// hotspot that is legal under one domain and not another is a situation this
// program cannot resolve and should not pretend to.
func ParseRegDomain(out string) (string, bool) {
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		rest, ok := strings.CutPrefix(line, "country ")
		if !ok {
			continue
		}
		cc, _, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}
		cc = strings.TrimSpace(cc)
		if len(cc) != 2 || cc == "00" || !IsUpperAlpha2(cc) {
			continue
		}
		return cc, true
	}
	return "", false
}

// IsUpperAlpha2 reports whether s is two upper-case ASCII letters, the shape
// of an ISO 3166 country code.
func IsUpperAlpha2(s string) bool {
	if len(s) != 2 {
		return false
	}
	for i := 0; i < 2; i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// A platform this build cannot serve.
// ---------------------------------------------------------------------------

type unsupportedBackend struct{ pl Platform }

func (u unsupportedBackend) Platform() Platform { return u.pl }

func (u unsupportedBackend) Detect(context.Context, Runner, []string) (Facts, error) {
	return Facts{}, fmt.Errorf("%w: %s", ErrUnsupportedBackend, u.pl)
}

func (unsupportedBackend) BaseSysctlKnobs() []string                               { return nil }
func (unsupportedBackend) PreEngineSteps(*Plan, map[string]string) []Step          { return nil }
func (unsupportedBackend) PostEngineSteps(*Plan, map[string]string) []Step         { return nil }
func (unsupportedBackend) CutStep(*Plan) Step                                      { return Step{} }
func (unsupportedBackend) RestoreStep(*Plan) Step                                  { return Step{} }
func (unsupportedBackend) SysctlKnobs(*Plan) []string                              { return nil }
func (unsupportedBackend) RegulatoryDomain(context.Context, Runner) (string, bool) { return "", false }
func (unsupportedBackend) AllowedBinaries() map[string]bool                        { return map[string]bool{} }

func (u unsupportedBackend) AssertHotspotInterfaceReleased(context.Context, Runner, *Plan) error {
	return fmt.Errorf("%w: %s", ErrUnsupportedBackend, u.pl)
}

func (u unsupportedBackend) AssertHotspotIsAccessPoint(context.Context, Runner, *Plan, string) error {
	return fmt.Errorf("%w: %s", ErrUnsupportedBackend, u.pl)
}
