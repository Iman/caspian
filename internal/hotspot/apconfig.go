// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Band is the radio band the access point runs on.
type Band string

const (
	// Band2GHz is 2.4 GHz, hostapd hw_mode=g.
	Band2GHz Band = "2.4GHz"
	// Band5GHz is 5 GHz, hostapd hw_mode=a.
	Band5GHz Band = "5GHz"
)

// WPA2 passphrase limits, from the WPA2-PSK definition that hostapd enforces:
// an ASCII passphrase is 8 to 63 printable characters. hostapd rejects anything
// outside that range at startup, which on this appliance would show the user a
// hotspot that never appears rather than a message they can act on, so the
// check happens here where the message can be written in plain words.
const (
	MinPassphraseLen = 8
	MaxPassphraseLen = 63
)

// MaxSSIDLen is the 802.11 SSID element limit: 32 octets, not 32 characters.
// A Persian or Arabic SSID reaches the limit in roughly 16 characters.
const MaxSSIDLen = 32

// bannedPassphrases are refused outright.
//
// "SecurePass123" is not a random example. The reference implementation this
// package replaces shipped it as the built-in hotspot password for every
// install (004-hotspot/install.sh:48, WIFI_PASSWORD="SecurePass123"), so every
// box built from that script had the same WPA2 key and anyone who had ever seen
// one could join any other. A fixed default is worse than no default, because
// it looks configured.
var bannedPassphrases = []string{
	"securepass123",
	"password",
	"password123",
	"12345678",
	"123456789",
	"changeme",
	"caspian",
	"hotspot",
	"raspberry",
	"admin",
}

// passphraseAlphabet is 32 symbols with the shapes a person confuses when
// copying from a screen to a phone removed: no l, no 1, no o, no 0. 32 symbols
// is exactly 5 bits, so a byte can be masked down without modulo bias.
const passphraseAlphabet = "abcdefghijkmnpqrstuvwxyz23456789"

// GeneratedPassphraseLen is 20 symbols from a 32 symbol alphabet, so 100 bits.
// Long because the panel shows a QR code and almost nobody types it; strong
// because a hotspot passphrase is offline-attackable from a captured handshake.
const GeneratedPassphraseLen = 20

// APConfig is everything needed to render a hostapd configuration.
//
// Interface is supplied by the caller. This package never detects it: which
// adapter is the hotspot and which is the uplink is internal/netcfg's decision.
type APConfig struct {
	// Interface is the wireless interface the access point runs on, for
	// example "wlan0". Supplied by the caller, never detected here.
	Interface string

	// Uplink is the interface the shared connection comes in on. hostapd has
	// no use for it and the renderer ignores it; the access point drivers
	// that share a connection themselves (Apple's Internet Sharing, Windows
	// Mobile Hotspot) need to be told which one. Optional.
	Uplink string

	// SSID is the network name, at most MaxSSIDLen octets.
	SSID string

	// Passphrase is the WPA2 pre-shared key. Empty means "generate one":
	// see EnsurePassphrase. It is never defaulted to a constant.
	Passphrase string

	// CountryCode is the ISO 3166-1 alpha-2 regulatory domain, for example
	// "GB". Required; see the comment on Validate.
	CountryCode string

	// Channel is the channel to run on. When the radio also holds a client
	// link this is not a free choice: see RadioConstraint.
	Channel int

	// Band selects hw_mode.
	Band Band

	// ControlDir is the directory for hostapd's control socket, used by this
	// program to ask hostapd whether the AP is actually up. Empty means
	// DefaultPaths().HostapdControlDir.
	ControlDir string
}

// Validate reports whether the configuration can be rendered.
//
// Every check here exists to turn a hostapd startup failure, which the user
// sees as "the hotspot never appeared", into a sentence they can act on.
func (c APConfig) Validate() error {
	return c.validate(false)
}

func (c APConfig) validate(mobileHotspot bool) error {
	if mobileHotspot {
		if c.Interface == "" || len(c.Interface) > 255 || !utf8.ValidString(c.Interface) {
			return errors.New("hotspot: invalid Windows adapter alias")
		}
		if err := noControlChars("interface", c.Interface); err != nil {
			return err
		}
	} else {
		if err := validConfigToken("interface", c.Interface); err != nil {
			return err
		}
	}

	if c.SSID == "" {
		return errors.New("hotspot: the network name is empty")
	}
	if len(c.SSID) > MaxSSIDLen {
		return fmt.Errorf("hotspot: the network name is %d bytes and the maximum is %d "+
			"(non-Latin names use two or more bytes per character)", len(c.SSID), MaxSSIDLen)
	}
	if !utf8.ValidString(c.SSID) {
		return errors.New("hotspot: the network name is not valid text")
	}
	if err := noControlChars("network name", c.SSID); err != nil {
		return err
	}

	if err := ValidatePassphrase(c.Passphrase); err != nil {
		return err
	}

	if err := validCountryCode(c.CountryCode); err != nil {
		return err
	}

	if err := validChannel(c.Band, c.Channel); err != nil {
		return err
	}

	if !mobileHotspot && c.ControlDir != "" {
		if err := validAbsPath("hostapd control directory", c.ControlDir); err != nil {
			return err
		}
	}
	return nil
}

// ValidatePassphrase applies the WPA2 rules and refuses known fixed defaults.
func ValidatePassphrase(p string) error {
	if p == "" {
		return errors.New("hotspot: no hotspot password was set")
	}
	if len(p) < MinPassphraseLen {
		return fmt.Errorf("hotspot: the hotspot password is %d characters and WPA2 needs at least %d",
			len(p), MinPassphraseLen)
	}
	if len(p) > MaxPassphraseLen {
		return fmt.Errorf("hotspot: the hotspot password is %d characters and the maximum is %d",
			len(p), MaxPassphraseLen)
	}
	// hostapd requires the ASCII passphrase to be printable ASCII. A byte
	// outside that range is silently mangled or rejected depending on version,
	// and a newline would end the wpa_passphrase line and let the rest of the
	// value be read as further hostapd directives.
	for i := 0; i < len(p); i++ {
		if p[i] < 0x20 || p[i] > 0x7e {
			return errors.New("hotspot: the hotspot password may only contain ordinary keyboard characters")
		}
	}
	if slicesContainsFold(bannedPassphrases, p) {
		return errors.New("hotspot: that hotspot password is a well-known default and cannot be used; " +
			"leave it blank and a strong one will be made for you")
	}
	return nil
}

// EnsurePassphrase returns cfg with a passphrase guaranteed to be present.
//
// When the caller supplied one it is validated and kept. When the caller
// supplied nothing a strong random one is generated and returned, so the panel
// can show it and put it in the join QR code. There is deliberately no constant
// fallback: see bannedPassphrases.
func EnsurePassphrase(cfg APConfig) (out APConfig, generated bool, err error) {
	if cfg.Passphrase != "" {
		if err := ValidatePassphrase(cfg.Passphrase); err != nil {
			return APConfig{}, false, err
		}
		return cfg, false, nil
	}
	p, err := GeneratePassphrase()
	if err != nil {
		return APConfig{}, false, err
	}
	cfg.Passphrase = p
	return cfg, true, nil
}

// GeneratePassphrase returns a fresh random WPA2 passphrase.
func GeneratePassphrase() (string, error) {
	buf := make([]byte, GeneratedPassphraseLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("hotspot: could not generate a hotspot password: %w", err)
	}
	var sb strings.Builder
	sb.Grow(GeneratedPassphraseLen)
	for _, b := range buf {
		// len(passphraseAlphabet) is 32 and 32 divides 256, so masking the low
		// 5 bits is uniform. Modulo on a non-power-of-two alphabet would not be.
		sb.WriteByte(passphraseAlphabet[b&0x1f])
	}
	return sb.String(), nil
}

// hwMode maps the band to hostapd's hw_mode.
func (c APConfig) hwMode() (string, error) {
	switch c.Band {
	case Band2GHz:
		return "g", nil
	case Band5GHz:
		return "a", nil
	default:
		return "", fmt.Errorf("hotspot: unknown radio band %q", string(c.Band))
	}
}

// channels5GHz is the set this package will emit. It is deliberately the
// non-DFS subset plus the common DFS channels are excluded: on a DFS channel
// hostapd must perform a radar scan before beaconing, which delays the hotspot
// by a minute or more and takes it down again for 30 minutes if radar is seen.
// A user watching for their network to appear reads that as broken.
var channels5GHz = []int{36, 40, 44, 48, 149, 153, 157, 161, 165}

func validChannel(band Band, ch int) error {
	switch band {
	case Band2GHz:
		// 1 to 13 are usable somewhere in the world; 14 is Japan-only and
		// 802.11b-only, so it is refused rather than half supported.
		if ch < 1 || ch > 13 {
			return fmt.Errorf("hotspot: channel %d is not a 2.4 GHz channel (1 to 13)", ch)
		}
		return nil
	case Band5GHz:
		for _, c := range channels5GHz {
			if c == ch {
				return nil
			}
		}
		return fmt.Errorf("hotspot: channel %d is not one this program will use on 5 GHz %v "+
			"(radar-detection channels are excluded because they delay or interrupt the hotspot)",
			ch, channels5GHz)
	default:
		return fmt.Errorf("hotspot: unknown radio band %q", string(band))
	}
}

func validCountryCode(cc string) error {
	// country_code is not optional on this appliance. Omitting it is the single
	// most common reason hostapd starts and no network ever appears: with no
	// regulatory domain the driver falls back to the world domain, where most
	// channels are passive-scan only and beaconing is not permitted. The
	// reference implementation omitted it entirely
	// (004-hotspot/install.sh:415-430 has no country_code line).
	if cc == "" {
		return errors.New("hotspot: no country was set, and the hotspot cannot legally pick a channel without one")
	}
	if len(cc) != 2 {
		return fmt.Errorf("hotspot: country %q is not a two-letter country code", cc)
	}
	for i := 0; i < 2; i++ {
		if cc[i] < 'A' || cc[i] > 'Z' {
			return fmt.Errorf("hotspot: country %q must be two capital letters, for example GB", cc)
		}
	}
	return nil
}

// validConfigToken accepts an interface name or similar short identifier.
//
// The generated files are key=value lines read by a process running as root.
// A value carrying a newline would end its line and let the remainder be read
// as further directives, so every value that reaches a template is checked
// here rather than trusted. See the design note in docs/2026-08-29-design.md
// section 6: parse and re-serialise, never interpolate.
func validConfigToken(what, v string) error {
	if v == "" {
		return fmt.Errorf("hotspot: no %s was given", what)
	}
	if len(v) > 64 {
		return fmt.Errorf("hotspot: %s %q is too long", what, v)
	}
	for i := 0; i < len(v); i++ {
		b := v[i]
		ok := (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
			(b >= '0' && b <= '9') || b == '.' || b == '_' || b == '-'
		if !ok {
			return fmt.Errorf("hotspot: %s %q contains a character that is not allowed", what, v)
		}
	}
	return nil
}

// validServiceAccount accepts a POSIX user or group name.
//
// Stricter than validConfigToken in one way that matters: a leading hyphen is
// refused, because these names are also handed to commands and a value that
// begins with a hyphen is read as an option rather than a name.
func validServiceAccount(what, v string) error {
	if err := validConfigToken(what, v); err != nil {
		return err
	}
	if len(v) > 32 {
		return fmt.Errorf("hotspot: %s %q is longer than a user name can be", what, v)
	}
	if strings.HasPrefix(v, "-") {
		return fmt.Errorf("hotspot: %s %q cannot begin with a hyphen", what, v)
	}
	return nil
}

func validAbsPath(what, p string) error {
	if p == "" {
		return fmt.Errorf("hotspot: no %s was given", what)
	}
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("hotspot: %s %q must be an absolute path", what, p)
	}
	return noControlChars(what, p)
}

func noControlChars(what, v string) error {
	for i := 0; i < len(v); i++ {
		if v[i] < 0x20 || v[i] == 0x7f {
			return fmt.Errorf("hotspot: %s contains a control character and cannot be written to a configuration file", what)
		}
	}
	return nil
}

func slicesContainsFold(list []string, v string) bool {
	lower := strings.ToLower(v)
	for _, s := range list {
		if s == lower {
			return true
		}
	}
	return false
}
