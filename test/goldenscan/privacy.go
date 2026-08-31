// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package goldenscan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The privacy scan: the WHOLE repository, not only its fixtures.
//
// # Why this is a second scan and not another shape class
//
// Scan above answers "did a CREDENTIAL reach a committed fixture". This one
// answers a different question: "did somebody's HOUSE reach a commit". The two
// need different scopes and different classes, and merging them would have made
// both worse.
//
// Scope, because a credential arrives through a golden and a location arrives
// through anything: a test assertion, a doc comment, a shell comment recording
// what a command printed, a filename. On 2026-08-31 the values removed from
// this repository were in test assertions, in package doc comments, in a
// hardware harness's shell comments and in a provenance file, and exactly four
// of the roughly forty sites were under a testdata directory. A testdata-only
// walk would have reported CLEAN over all of it.
//
// Classes, because the credential classes cannot run repository-wide. The
// base64-32byte-key shape fires on almost every line of go.sum. Running the two
// scans over one scope would have forced either a testdata scope on the privacy
// classes or an allowlist so wide that the credential classes stopped meaning
// anything.
//
// # What was removed, and what each class is for
//
// A Raspberry Pi was tested on the maintainer's home network and the readings
// were committed. The router BSSID is the worst of them: public WiFi
// geolocation services index router MAC addresses, so that value alone places a
// home within metres, and with the network name beside it there is no ambiguity
// left. The others are the Pi's own interface addresses, the network name, the
// hostname, the machine-id, the handset's serial and model, one proxy server
// address and the household's public IP as measured by the exit-IP harness.
//
//	non-local-mac     A MAC whose locally-administered bit is clear was
//	                  ASSIGNED BY A MANUFACTURER to one physical radio. It is
//	                  globally unique. Nothing invented needs one, so this
//	                  needs no curation and works in a file nobody has read.
//
//	unregistered-mac  A MAC outside the substitute block documented in
//	                  internal/netcfg/testdata/PROVENANCE.md. Strictly stronger
//	                  than the class above and the reason it is not enough on
//	                  its own: the router BSSID removed here IS locally
//	                  administered (many routers set the bit on a virtual AP),
//	                  so non-local-mac would have let it back in.
//
//	public-ipv4       A routable address outside the documentation ranges. A
//	                  server address, or a household's address as an echo
//	                  endpoint measured it.
//
//	privacy-sentinel  The specific strings removed, so that a revert is caught
//	                  rather than merely unlikely. See PrivacySentinels for why
//	                  the registry stores digests and which values are
//	                  deliberately NOT in it.
//
// # This scan prints no matched text either
//
// Same rule as Finding above, and it matters more here: the point of the scan
// is that these values are not in this repository, and a failure message
// quoting one would put it back, in the test output, in a CI log and in a
// terminal scrollback.

const (
	// ClassNonLocalMAC is a manufacturer-assigned MAC address.
	ClassNonLocalMAC = "non-local-mac"
	// ClassUnregisteredMAC is a MAC outside the documented substitute block.
	ClassUnregisteredMAC = "unregistered-mac"
	// ClassPublicIPv4 is a routable address outside the documentation ranges.
	ClassPublicIPv4 = "public-ipv4"
	// ClassPrivacySentinel is one of the values removed on 2026-08-31.
	ClassPrivacySentinel = "privacy-sentinel"
)

// privacyClassWhy explains each class in a failure message.
var privacyClassWhy = map[string]string{
	ClassNonLocalMAC: "a MAC address with the locally-administered bit clear was assigned by a " +
		"manufacturer to one physical radio and is globally unique. Public WiFi geolocation " +
		"services index router MACs, so one of them can place a home within metres",
	ClassUnregisteredMAC: "a MAC address outside the 02:00:5e:xx:xx:xx substitute block documented " +
		"in internal/netcfg/testdata/PROVENANCE.md. Locally-administered does not mean invented: " +
		"the home router removed from this repository had the bit set",
	ClassPublicIPv4: "a routable IPv4 address outside the documentation ranges. It is a server, " +
		"a household's public address, or something else that belongs to somebody",
	ClassPrivacySentinel: "one of the values removed from this repository on 2026-08-31 because it " +
		"identified the maintainer's home, box or handset. It can only be here because a " +
		"substitution was reverted or a new capture was taken on the same network",
}

// ---------------------------------------------------------------------------
// MAC addresses
// ---------------------------------------------------------------------------

// macRun matches a run of colon-separated hex octets of ANY length, not exactly
// six.
//
// THE REASON IT IS NOT ANCHORED AT SIX. A DHCP client identifier is
// 01:<six octets> and a DHCPv6 DUID-LLT is 00:01:<four octets of time>:<six
// octets of hardware address>; both are in internal/hotspot/testdata. A
// six-octet pattern with word boundaries matches the FIRST six octets of a DUID
// and reports 00:01:00:01:2d:9f, which is a timestamp and not an address, while
// SAYING NOTHING about the real hardware address in the last six. That is the
// worst possible outcome: a loud false positive that trains a reader to
// allowlist the file, and the allowlist entry then also covers the trailing
// address the class exists to find.
//
// So the run is taken whole and macsIn decides what part of it is an address.
var macRun = regexp.MustCompile(`\b(?:[0-9a-fA-F]{2}:)+[0-9a-fA-F]{2}\b`)

// macsIn returns every hardware address a line contains.
//
// Six octets is the address itself. More than six is a client identifier or a
// DUID, and both formats END with the link-layer address, so the trailing six
// are read and the leading octets are ignored. Fewer than six is not an address
// at all: a clock time, an ISO duration, a port list.
func macsIn(line string) []string {
	var out []string
	for _, run := range macRun.FindAllString(line, -1) {
		oct := strings.Split(run, ":")
		if len(oct) < 6 {
			continue
		}
		out = append(out, strings.ToLower(strings.Join(oct[len(oct)-6:], ":")))
	}
	return out
}

// notAnAddress are the two six-octet values that name no interface.
//
// In code and not in the allowlist, because they are properties of the format
// rather than facts about a file: ff:ff:ff:ff:ff:ff is the broadcast
// destination and appears in the "brd" field of every single line of
// "ip -d link" output, and 00:00:00:00:00:00 is the unset address. An allowlist
// entry would have to name every file that holds link output, and would then
// permit a real address in those same files.
var notAnAddress = map[string]bool{
	"ff:ff:ff:ff:ff:ff": true,
	"00:00:00:00:00:00": true,
}

// locallyAdministered reports whether bit 1 of the first octet is set, which is
// the second hex digit being one of 2, 6, a or e.
func locallyAdministered(mac string) bool {
	b, err := strconv.ParseUint(mac[:2], 16, 8)
	if err != nil {
		return false
	}
	return b&0x02 != 0
}

// substituteMACPrefix is the block every MAC in this repository is drawn from.
//
// 02:00:5e is locally administered, so it cannot collide with any
// manufacturer's assignment. The per-address meanings are the table in
// internal/netcfg/testdata/PROVENANCE.md.
const substituteMACPrefix = "02:00:5e:"

// registeredMACs are the addresses outside the substitute block that are known
// not to belong to anybody.
//
// In code rather than in the path allowlist for the same reason as
// notAnAddress: the claim is about the VALUE, so a path-scoped entry would be
// both too narrow (it has to name every file) and too wide (it would permit any
// other address in those files).
var registeredMACs = map[string]string{
	"12:34:56:78:9a:bc": "the counting sequence, in internal/netcfg/simulated_kernel.go, where the " +
		"double needs an address that is visibly not a reading",
}

// ---------------------------------------------------------------------------
// IPv4 addresses
// ---------------------------------------------------------------------------

// knownPublicAddresses are routable addresses chosen by this repository rather
// than by a user, which therefore identify nobody.
//
// Every one of them is the same on every box. That is the test applied here,
// and it is why a proxy server address can never join this map however
// convenient it would be: a server address differs per user, which is the whole
// property that makes it identifying.
var knownPublicAddresses = map[string]string{
	// The resolvers internal/xcfg/resolvers.go selects, and the neighbours its
	// comments name in order to say which ones were NOT selected.
	"9.9.9.9":       "Quad9 filtering resolver, internal/xcfg/resolvers.go",
	"9.9.9.10":      "Quad9 unfiltered, named in resolvers.go as the one deliberately not chosen",
	"9.9.9.11":      "Quad9 filtering with ECS, internal/xcfg/resolvers.go",
	"1.1.1.1":       "Cloudflare unfiltered, a negative control and the pinned echo endpoint A",
	"1.1.1.2":       "Cloudflare malware-only, named in resolvers.go as deliberately not chosen",
	"1.1.1.3":       "Cloudflare family filtering, internal/xcfg/resolvers.go",
	"185.228.168.9": "CleanBrowsing family filtering, internal/xcfg/resolvers.go",
	"8.8.8.8":       "Google public resolver, used as the address the firewall must NOT permit",
	"8.8.4.4":       "Google public resolver, secondary, same use",
	// The /24 bases of the two Google prefixes, which appear as prefixes and
	// not as hosts.
	"8.8.8.0": "base of 8.8.8.0/24, a resolver prefix in internal/xcfg/resolvers.go",
	"8.8.4.0": "base of 8.8.4.0/24, a resolver prefix in internal/xcfg/resolvers.go",
	// IANA's example host. Reserved for documentation in the same sense as the
	// RFC 5737 ranges, and used here as "an address that is plainly public" in
	// the tests that refuse a public bind.
	"93.184.216.34": "IANA's example.com host, the public address in the bind-refusal tests",
	"93.184.216.0":  "base of example.com's /24, the public subnet in privsvc/validate_test.go",
}

// documentationIPv4 reports whether an IPv4 literal is one that cannot identify
// anybody: private, loopback, link-local, multicast, unspecified, broadcast,
// benchmarking, carrier-grade NAT, or one of the three ranges RFC 5737 reserves
// for documentation.
//
// It is deliberately a separate function from nonRoutable in scan.go even
// though the two currently agree. nonRoutable answers "would a hit here be
// noise for the CREDENTIAL scan"; this answers "can this address identify a
// person". Sharing one function would mean a later loosening for one scan
// silently loosening the other.
func documentationIPv4(s string) bool {
	var a, b, c, d int
	if n, err := fmt.Sscanf(s, "%d.%d.%d.%d", &a, &b, &c, &d); n != 4 || err != nil {
		return true
	}
	for _, v := range []int{a, b, c, d} {
		if v < 0 || v > 255 {
			return true // not an address at all
		}
	}
	switch {
	case a == 0: // "this network" and the unspecified address
		return true
	case a == 10, a == 127: // RFC 1918, loopback
		return true
	case a == 169 && b == 254: // link-local
		return true
	case a == 172 && b >= 16 && b <= 31: // RFC 1918
		return true
	case a == 192 && b == 168: // RFC 1918
		return true
	case a == 192 && b == 0 && c == 0: // IETF protocol assignments
		return true
	case a == 192 && b == 0 && c == 2: // TEST-NET-1
		return true
	case a == 198 && b == 51 && c == 100: // TEST-NET-2
		return true
	case a == 203 && b == 0 && c == 113: // TEST-NET-3
		return true
	case a == 198 && (b == 18 || b == 19): // benchmarking
		return true
	case a == 100 && b >= 64 && b <= 127: // carrier-grade NAT
		return true
	case a >= 224: // multicast, reserved, broadcast
		return true
	}
	return false
}

var ipv4Literal = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)

// ---------------------------------------------------------------------------
// The privacy sentinels
// ---------------------------------------------------------------------------

// PrivacySentinel is one value removed from this repository on 2026-08-31.
//
// # Why this registry holds a DIGEST and not the value
//
// The obvious registry is a list of strings. It cannot be used here, because
// this scan exists to keep exactly those strings out of a repository that is
// about to be published: a registry holding them would put the maintainer's
// home network name and hostname back into the published tree, in the file
// whose job is to keep them out. So each entry pins sha256 of the value
// lowercased, and the scan hashes candidate windows and compares.
//
// # The trade-off that buys, stated rather than left implicit
//
// A digest of a LOW-ENTROPY value is confirmable: anyone who guesses "the SSID
// was a common English word" can hash a word list and match. So these rows do
// not make the values unrecoverable, they make them not-plainly-readable and
// they make a revert fail the gate. That is the trade the tripwire costs, and
// it is worth naming because it decides which values are here.
//
// # Which values are deliberately NOT here
//
//   - EVERY MAC ADDRESS, including the home router BSSID, which was the worst
//     value in this incident. MAC addresses are covered by unregistered-mac,
//     which catches ANY address outside the substitute block rather than one
//     known address, so it is strictly better coverage. Storing a BSSID digest
//     as well would add nothing to the guard and would make the one value that
//     locates a home confirmable against a public WiFi-geolocation dump.
//   - The two removed IPv4 addresses. public-ipv4 already catches both, and an
//     IPv4 digest is exhaustively recoverable in seconds: 2^32 is nothing.
//
// In both cases a structural class covers the value, so a digest would be pure
// loss. Do not add one "for completeness".
type PrivacySentinel struct {
	// Name is what a failure prints. The value is never printed and is not
	// here to print.
	Name string

	// SHA256 is hex sha256 of the removed value, lowercased.
	SHA256 string

	// Len is the byte length of the lowercased value. The scan needs it to
	// know how wide a window to hash, and it is worth knowing that this leaks
	// the length; the alternative is hashing every window of every length,
	// which is the same information at a hundred times the cost.
	Len int

	// Why says what the value was, in words, so a reader can act on a failure
	// without being told the value.
	Why string
}

// PrivacySentinels is the registry.
func PrivacySentinels() []PrivacySentinel {
	return []PrivacySentinel{
		{
			Name:   "home wifi network name",
			SHA256: "000c285457fc971f862a79b786476c78812c8897063c6fa9c045f579a3b2d63f",
			Len:    6,
			Why: "the SSID of the network the reference Raspberry Pi was tested on, which is " +
				"the maintainer's home. Beside a router BSSID it removes the last ambiguity " +
				"about which house. Substitute HomeNet",
		},
		{
			Name:   "reference box hostname",
			SHA256: "ee19ab44243bfa691ed09c79aa1a4dced6c2fd691c5f1e84f73ddd9f0dca116a",
			Len:    10,
			Why: "the hostname of the reference Raspberry Pi, which contains a person's given " +
				"name. Substitute caspian-box",
		},
		{
			Name:   "reference box machine-id",
			SHA256: "be9bc6dbce758dcf69016e72003cb568de49ff479008abc8d38c5b64d83f7260",
			Len:    32,
			Why: "/etc/machine-id of the reference Raspberry Pi. 128 bits of randomness " +
				"identifying one physical box for its lifetime. It was in the host-and-toolchain " +
				"table of internal/netcfg/testdata/PROVENANCE.md and adds nothing to that record",
		},
		{
			Name:   "handset adb serial",
			SHA256: "1a4ad2c51fc6cb0c75c10e476cc584d834daf006f98b38d71d566bdb1c2b7bac",
			Len:    11,
			Why: "the serial of the handset the hardware harness drives, which is a warranty " +
				"and retail-purchase identifier for one phone. Substitute PHONE-SERIAL",
		},
		{
			Name:   "handset model, hyphen form",
			SHA256: "ddd86b75181a78832b9cb1d15cd9e72ebc8a0d54626c35f247fbedb80b06b745",
			Len:    8,
			Why: "the handset model as ro.product.model prints it. Narrows one serial to one " +
				"product. Substitute SM-X000F, which has the same shape",
		},
		{
			Name:   "handset model, underscore form",
			SHA256: "cb5f9c47561dbf9f1a3a02a1011de467db8c88f24f67f2e75bdb8e1fc4971e0e",
			Len:    8,
			Why: "the same model as adb devices -l prints it, with an underscore. A separate " +
				"row because a substitution that fixed one spelling and not the other is exactly " +
				"the kind of half-done change this registry exists to catch",
		},
		{
			Name:   "handset product code",
			SHA256: "bf72fb381e9f539d1cbe617df8644561ef9313a01b7ebf0dac2d0cdb6587846f",
			Len:    8,
			Why: "the handset product code from the same adb devices -l line, which names the " +
				"model and the sales region. Substitute x000nnxx",
		},
	}
}

// sentinelHitsIn returns the names of every sentinel a line carries.
//
// A window is hashed only where it STARTS a token: at the beginning of the line
// or after a byte that is not a letter or a digit. That is what makes the scan
// affordable over a whole repository, and it still catches the realistic
// revert, because every removed value is a field on its own or follows a
// separator. Using the SUBSTITUTES to show the shape, those positions are
// "ssid HomeNet", "model:SM_X000F" and "netplan-wlan0-HomeNet".
//
// The examples are written with the substitutes on purpose. An earlier draft of
// this comment illustrated the same three positions with the values it was
// removing, which put two of them straight back into the repository, in the
// file whose job is to keep them out. It was caught by a grep and not by this
// scan, because test/goldenscan was excluded from the walk wholesale. That
// exclusion is now shape-only: see privacyScope.
func sentinelHitsIn(line string, sentinels []PrivacySentinel) []string {
	if len(sentinels) == 0 {
		return nil
	}
	low := strings.ToLower(line)
	// Group by width so each width is walked once.
	widths := map[int][]PrivacySentinel{}
	for _, s := range sentinels {
		widths[s.Len] = append(widths[s.Len], s)
	}
	var out []string
	for w, group := range widths {
		if w <= 0 || w > len(low) {
			continue
		}
		for i := 0; i+w <= len(low); i++ {
			if i > 0 && isTokenByte(low[i-1]) {
				continue
			}
			sum := hex.EncodeToString(sliceSum(low[i : i+w]))
			for _, s := range group {
				if s.SHA256 == sum {
					out = append(out, s.Name)
				}
			}
		}
	}
	return out
}

func sliceSum(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

func isTokenByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= '0' && b <= '9'
}

// ---------------------------------------------------------------------------
// Walking the repository
// ---------------------------------------------------------------------------

// privacySkipDirs are the directories the privacy scan does not walk at all,
// each with the reason. Every one of them is either not published or not
// readable as source, so a value in it does not reach a clone. A reader who
// thinks one should be in scope is reading a decision, not an oversight.
//
// third_party is DELIBERATELY NOT HERE although the credential scan excludes
// vendored trees. That exclusion is about npm deciding the contents of
// node_modules; third_party/ is committed, published, and small enough that
// scanning it costs nothing. Measured 2026-08-31: it holds no MAC address and
// no address outside loopback and RFC 1918.
var privacySkipDirs = map[string]string{
	".git":         "the object store. History is out of this scan's reach by construction: removing a value from the working tree removes nothing from a commit that already carries it",
	".codegraph":   "gitignored. A generated index of this repository, rebuilt from the files the scan already reads",
	".idea":        "gitignored. Editor state, not source",
	"node_modules": "gitignored. Installed by npm, decided by nobody here",
	"local":        "gitignored and holding real configuration by design. Never committed, and reading it would pull live values into this process for no benefit",
}

// shapeExemptPaths are walked, but only the privacy-sentinel class is applied
// to them.
//
// WHY THE EXEMPTION IS SHAPE-ONLY AND NOT WHOLESALE. The scanner's own source
// and tests necessarily contain a planted MAC of each class, a planted routable
// address and the literals every allowlist entry names, so running the MAC and
// IPv4 classes over them would report the guard as the leak. None of that is
// true of the sentinel class: the registry holds DIGESTS precisely so it does
// not hold the values, so a sentinel hit in this directory is a real one.
//
// This was a wholesale exclusion for about an hour on 2026-08-31, and in that
// hour a comment in privacy.go illustrating three token positions did it with
// two of the values being removed. A grep caught it; this scan did not, because
// of the exclusion. That is the entire argument for the narrower form.
var shapeExemptPaths = map[string]string{
	"test/goldenscan": "the scanner's own source and its own tests. They declare every class and " +
		"plant one of each, and every allowlist entry names the literal it permits, so the MAC " +
		"and IPv4 classes would report the guard as the leak. The sentinel class still applies " +
		"here, and has to: the registry holds digests, not values, so a hit is real",
}

// privacyScope decides what applies to a path relative to the module root.
//
// walk is false for a path that is not read at all. shapeExempt is true for a
// path where only the privacy-sentinel class is applied. Exported through
// PrivacyRoots and PrivacyShapeExempt so the decisions can be asserted by a
// test rather than trusted.
func privacyScope(rel string) (walk bool, shapeExempt bool, why string) {
	rel = filepath.ToSlash(rel)
	for _, seg := range strings.Split(rel, "/") {
		if w, ok := privacySkipDirs[seg]; ok {
			return false, false, w
		}
	}
	for dir, w := range shapeExemptPaths {
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			return true, true, w
		}
	}
	return true, false, ""
}

// PrivacyRoots reports whether a path is walked at all.
func PrivacyRoots(rel string) (bool, string) {
	walk, _, why := privacyScope(rel)
	return walk, why
}

// PrivacyShapeExempt reports whether only the sentinel class applies to a path.
func PrivacyShapeExempt(rel string) (bool, string) {
	_, exempt, why := privacyScope(rel)
	if !exempt {
		return false, ""
	}
	return true, why
}

// ScanRepo walks the whole module and returns every privacy finding, allowlisted
// entries removed.
//
// sentinels is a parameter rather than a call to PrivacySentinels so that the
// proof test can plant a value of its own. A caller that wants the gate passes
// PrivacySentinels().
func ScanRepo(moduleRoot string, sentinels []PrivacySentinel) ([]Finding, error) {
	var found []Finding
	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(moduleRoot, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		walk, shapeExempt, _ := privacyScope(rel)
		if !walk {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		opt := scanOpts{applyLiteralAllow: true, sentinelsOnly: shapeExempt}
		found = append(found, scanPrivacyName(rel, sentinels, opt)...)
		if d.IsDir() {
			return nil
		}
		if skipExt[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		found = append(found, scanPrivacyBody(rel, string(body), sentinels, opt)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return filterAllowed(found), nil
}

// scanPrivacyName checks file and directory names. A capture named after the
// network it was taken on carries the network name into a path, where no body
// scan will ever look at it.
// scanOpts carries the two decisions the walk makes per path.
type scanOpts struct {
	// applyLiteralAllow is false only for the guard that has to see raw hits in
	// order to report an allowlist entry that matches nothing.
	applyLiteralAllow bool
	// sentinelsOnly suppresses the MAC and IPv4 classes. See shapeExemptPaths.
	sentinelsOnly bool
}

func scanPrivacyName(rel string, sentinels []PrivacySentinel, opt scanOpts) []Finding {
	return privacyFindings(rel, filepath.Base(rel), 0, sentinels, opt)
}

func scanPrivacyBody(rel, body string, sentinels []PrivacySentinel, opt scanOpts) []Finding {
	var out []Finding
	for i, line := range strings.Split(body, "\n") {
		out = append(out, privacyFindings(rel, line, i+1, sentinels, opt)...)
	}
	return out
}

// privacyFindings applies every class to one piece of text.
//
// At most ONE finding per class per line. A line of "ip -d link" output holds
// two addresses and a fixture line holds an address and a client identifier;
// reporting each separately would triple the failure output and say nothing the
// first one did not.
func privacyFindings(rel, text string, line int, sentinels []PrivacySentinel, opt scanOpts) []Finding {
	var out []Finding

	if opt.sentinelsOnly {
		for _, name := range sentinelHitsIn(text, sentinels) {
			out = append(out, Finding{Path: rel, Line: line, Class: ClassPrivacySentinel, Name: name})
		}
		return out
	}

	nonLocal, unregistered := false, false
	for _, mac := range macsIn(text) {
		if notAnAddress[mac] {
			continue
		}
		if !locallyAdministered(mac) {
			nonLocal = true
		}
		if _, ok := registeredMACs[mac]; ok {
			continue
		}
		if !strings.HasPrefix(mac, substituteMACPrefix) {
			unregistered = true
		}
	}
	if nonLocal {
		out = append(out, Finding{Path: rel, Line: line, Class: ClassNonLocalMAC})
	}
	if unregistered {
		out = append(out, Finding{Path: rel, Line: line, Class: ClassUnregisteredMAC})
	}

	for _, m := range ipv4Literal.FindAllString(text, -1) {
		if documentationIPv4(m) {
			continue
		}
		if _, ok := knownPublicAddresses[m]; ok {
			continue
		}
		// The allowlist is applied HERE, with the value in hand, and not in
		// filterAllowed. An entry for this class names the literals it permits,
		// so a file that legitimately holds one pinned service address does not
		// thereby permit every other address in the same file. See the Literals
		// field in registry.go for the measurement that forced this.
		if opt.applyLiteralAllow && allowedLiteral(rel, ClassPublicIPv4, m) {
			continue
		}
		out = append(out, Finding{Path: rel, Line: line, Class: ClassPublicIPv4})
		break
	}

	for _, name := range sentinelHitsIn(text, sentinels) {
		out = append(out, Finding{Path: rel, Line: line, Class: ClassPrivacySentinel, Name: name})
	}
	return out
}

// PrivacyClassNames returns every privacy class, for the test that requires
// each one to have been watched firing.
func PrivacyClassNames() []string {
	return []string{ClassNonLocalMAC, ClassUnregisteredMAC, ClassPublicIPv4, ClassPrivacySentinel}
}

// PrivacyClassWhy returns the explanation for a privacy class.
func PrivacyClassWhy(class string) string {
	if why, ok := privacyClassWhy[class]; ok {
		return why
	}
	return "unknown class"
}

// KnownPublicAddresses and RegisteredMACs expose the two in-code exception maps
// so their reasons can be checked by a test the same way allowlist reasons are.
func KnownPublicAddresses() map[string]string { return knownPublicAddresses }

// RegisteredMACs exposes the MAC exception map.
func RegisteredMACs() map[string]string { return registeredMACs }

// PrivacySkipReasons exposes the exclusions so a test can require each to say
// why.
func PrivacySkipReasons() map[string]string {
	out := map[string]string{}
	for k, v := range privacySkipDirs {
		out[k] = v
	}
	for k, v := range shapeExemptPaths {
		out[k] = v
	}
	return out
}
