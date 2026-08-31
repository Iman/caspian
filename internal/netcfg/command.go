// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// The binaries this package is allowed to run. The privileged side of the
// appliance accepts a short list of named actions and never a command built
// from anything the user typed (design 5.5). This allowlist is the enforcement
// point for that: every Command originates in this package, and the executor
// refuses any Path that is not below.
const (
	BinIP     = "ip"
	BinIw     = "iw"
	BinNft    = "nft"
	BinSysctl = "sysctl"
	// BinNmcli is on the list because taking an interface over means taking
	// it away from whatever manages it, and on a Raspberry Pi OS image that
	// is usually NetworkManager. Not asking produced the worst outcome this
	// package has had: the hotspot was planned on an interface still joined
	// to the user's house network, and the DHCP server bound to it and
	// started answering on a LAN we do not own.
	//
	// It is read-only during detection. The only change it makes is
	// "device set <ifname> managed no", whose inverse is journalled.
	BinNmcli = "nmcli"
)

var allowedBinaries = map[string]bool{
	BinIP:     true,
	BinIw:     true,
	BinNft:    true,
	BinSysctl: true,
	BinNmcli:  true,
}

// ErrUnsupportedPlatform is returned by the runner on any platform that is not
// Linux. The pure half of the package works everywhere; only execution is
// restricted.
var ErrUnsupportedPlatform = errors.New("netcfg: network configuration can only be applied on linux")

// ErrDisallowedBinary is returned when a Command names a binary outside the
// allowlist.
var ErrDisallowedBinary = errors.New("netcfg: command names a binary that is not on the allowlist")

// Command is one invocation. There is no shell anywhere in this package: Path
// is looked up as a file and Args are passed as a vector, so quoting and word
// splitting cannot happen and an argument containing a space or a semicolon is
// still one argument.
type Command struct {
	Path  string   `json:"path"`
	Args  []string `json:"args"`
	Stdin string   `json:"stdin,omitempty"`

	// Why records the failure this command exists to prevent. It is carried
	// into the journal so that somebody reading a half-applied journal after a
	// crash can tell what each entry was for.
	Why string `json:"why,omitempty"`
}

// Result is what a Runner reports back.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner executes commands. It is one method wide so that tests can substitute
// a recorder; see RecordingRunner.
type Runner interface {
	Run(ctx context.Context, c Command) (Result, error)
}

// IsZero reports whether the command is empty, which is how a Step says it has
// no inverse.
func (c Command) IsZero() bool { return c.Path == "" && len(c.Args) == 0 }

// String renders the command for logs. It is not a shell command and must
// never be fed to one; the quoting here is for a human reader only.
func (c Command) String() string {
	if c.IsZero() {
		return "(none)"
	}
	var b strings.Builder
	b.WriteString(c.Path)
	for _, a := range c.Args {
		b.WriteByte(' ')
		if a == "" || strings.ContainsAny(a, " \t\"'") {
			fmt.Fprintf(&b, "%q", a)
			continue
		}
		b.WriteString(a)
	}
	if c.Stdin != "" {
		fmt.Fprintf(&b, " <<(%d bytes on stdin)", len(c.Stdin))
	}
	return b.String()
}

// ValidateCommand is the check the executor makes before running anything. It
// lives outside the build-tagged files on purpose, so that it is tested on the
// development machine as well as on the target.
func ValidateCommand(c Command) error {
	if c.IsZero() {
		return errors.New("netcfg: empty command")
	}
	if !allowedBinaries[c.Path] {
		return fmt.Errorf("%w: %q", ErrDisallowedBinary, c.Path)
	}
	for i, a := range c.Args {
		if strings.ContainsRune(a, 0) {
			return fmt.Errorf("netcfg: argument %d of %q contains a NUL byte", i, c.Path)
		}
	}
	return nil
}

// ifaceNamePattern is what the kernel accepts for a network interface name:
// at most IFNAMSIZ-1 = 15 bytes, no slash, no whitespace, no NUL. Interface
// names reach this package by parsing the output of other programs, so they
// are validated before they are ever placed in an argument vector or in
// generated nftables text.
var ifaceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,15}$`)

// ValidInterfaceName reports whether name is safe to place in a command
// argument or in generated nftables text.
func ValidInterfaceName(name string) bool {
	return ifaceNamePattern.MatchString(name)
}
