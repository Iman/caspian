// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"context"
	"errors"
	"fmt"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
)

// ---------------------------------------------------------------------------
// The privilege boundary.
//
// The panel process holds no privilege (docs/2026-08-29-design.md section 5.5,
// docs/LAYOUT.md "Two processes, one binary"). It runs as the caspian user and
// asks a service running as root, over a unix socket at /run/caspian/priv.sock,
// to do the things that need root: routes, the firewall, the access point, the
// DHCP and DNS server, and the engine.
//
// The shape of that request is the entire security of the split, and the design
// is blunt about it: "The privileged side accepts a short list of named actions
// and never a command built from anything the user typed. A privileged helper
// that takes a path and an argument list from its client is not a boundary; it
// is a way to run anything as root."
//
// So the interface below has a fixed, named set of methods, and no method
// takes a command, a
// path, an argument list, an interface name chosen freely, or any other string
// that the privileged side would hand to a shell or an exec. Every argument is
// a typed field on a struct declared here, and the privileged side is expected
// to validate each one against what it detected for itself rather than trusting
// it. The panel cannot express "run this"; it can only express "start", and the
// privileged side decides what starting means.
//
// The one field that carries free-form bytes is StartRequest.ConfigJSON, and
// that is deliberate and bounded: design section 6 requires the pasted config to
// be parsed and re-serialised rather than interpolated, which is exactly what
// internal/link does. What crosses the socket is a JSON document that
// internal/link built from parsed structures, never the text the user pasted.
//
// This package does not implement the privileged side. It defines the vocabulary
// and provides a fake (see fake.go) so the panel can be tested without root.
// ---------------------------------------------------------------------------

// Action is the name of one privileged action. The set is closed: these are the
// only things the panel can ask for, and each one is exactly one method on
// Privileged.
//
// The strings exist for whoever writes the socket protocol on either side, so
// that the wire vocabulary and the Go vocabulary cannot drift apart.
// TestActionVocabularyMatchesTheInterface fails if a method is added without
// a name here.
type Action string

const (
	// ActionDetect reports what the machine has: which interface reaches the
	// internet, which can host an access point, and the radio's constraints.
	// It changes nothing.
	ActionDetect Action = "detect"

	// ActionStatus reports what is running now. It changes nothing.
	ActionStatus Action = "status"

	// ActionStart brings the tunnel and the hotspot up.
	ActionStart Action = "start"

	// ActionStop takes them down and returns the machine to how it was, by
	// replaying the teardown journal (design section 5.5).
	ActionStop Action = "stop"

	// ActionEngineLog returns the recent engine log, already redacted by
	// internal/engine.
	ActionEngineLog Action = "engine-log"

	// ActionCut drops forwarded client traffic while leaving the hotspot, the
	// DHCP and DNS server and this panel running, so a joined device stays
	// joined and can still reach the page that undoes it.
	//
	// It is not a smaller version of ActionStop. Stop takes the hotspot down
	// with everything else, which disconnects every device including the phone
	// the person is holding to press the button, and that is the wrong control
	// for somebody who wants traffic to stop right now.
	ActionCut Action = "cut"

	// ActionRecover is the way out of a stuck box without a reboot and without
	// a terminal.
	//
	// It takes everything down, replays the teardown journal so that every
	// interface, route and firewall rule this appliance changed is put back the
	// way it was found, and then brings it up again from the saved settings.
	//
	// It exists because of a measured day: 2026-08-30, when the appliance
	// repeatedly reached states that only a person with an SSH session could
	// clear. A created interface that outlived its start, an address flushed by
	// NetworkManager, a journal entry that survived a failed start. Every one
	// of those is recoverable by replaying what is already written down, and
	// none of them was reachable from the panel.
	//
	// It is deliberately NOT a reboot and it does not restart the two systemd
	// units. The point of this control is that a person holding only a phone
	// can press it: the panel and any SSH session stay up throughout, which a
	// reboot would take away, and taking the panel away is exactly what makes a
	// stuck box unrecoverable for somebody with no keyboard on it.
	ActionRecover Action = "recover"

	// ActionRestore puts forwarded client traffic back.
	ActionRestore Action = "restore"
)

// Actions is every action the panel can ask for, in the order they appear
// above. Nothing outside this list crosses the socket.
var Actions = []Action{
	ActionDetect, ActionStatus, ActionStart, ActionStop, ActionRecover, ActionEngineLog,
	ActionCut, ActionRestore,
}

// Privileged is the panel's whole view of the privileged service.
//
// Every method takes a context so that a slow or wedged privileged side cannot
// hold an HTTP handler open; the panel always gives it a deadline.
//
// An implementation is expected to be safe for concurrent use: the panel polls
// Status from one request while another is running Start.
type Privileged interface {
	Detect(ctx context.Context) (Detection, error)
	Status(ctx context.Context) (SystemStatus, error)
	Start(ctx context.Context, req StartRequest) error
	Stop(ctx context.Context) error
	EngineLog(ctx context.Context) (EngineLog, error)

	// Cut drops forwarded client traffic. Restore puts it back. Both are
	// runtime state on the privileged side and neither is written down, so a
	// restart of the machine restores traffic on its own.
	Cut(ctx context.Context) error
	Restore(ctx context.Context) error

	// Recover takes everything down, puts the machine back the way it was
	// found by replaying the teardown journal, and starts again from the given
	// request. It is the panel's way out of a state that would otherwise need
	// a person with a terminal.
	Recover(ctx context.Context, req StartRequest) error
}

// ---------------------------------------------------------------------------
// Faults
// ---------------------------------------------------------------------------

// Fault is a machine-readable reason something is not working.
//
// The privileged side returns a Fault, never a sentence. The wording lives in
// this package, in words.go, for two reasons: the audience is here, so the
// person writing the sentence can see the screen it lands on, and a code can be
// tested for exactly while a sentence gets reworded and quietly breaks its test.
type Fault string

const (
	// FaultNone means nothing is wrong.
	FaultNone Fault = ""

	// FaultNoAPAdapter means no radio on the machine can host an access point.
	FaultNoAPAdapter Fault = "no-ap-adapter"

	// FaultHotspotInterfaceBusy means the adapter the hotspot needs is still
	// joined to another network and Caspian stopped rather than take it over.
	//
	// It is separate from FaultNoAPAdapter and from FaultHotspotFailed because
	// the advice attached to each of those is wrong here, and wrong in a way
	// that wastes the user's time. FaultNoAPAdapter says no adapter can create
	// a hotspot, which is false about an adapter that can and is merely busy.
	// FaultHotspotFailed says to restart the machine, and restarting is the one
	// thing that cannot help: the box boots, the network manager rejoins the
	// same network, and the refusal is identical.
	FaultHotspotInterfaceBusy Fault = "hotspot-interface-busy"

	// FaultNotRunning means the box was asked to cut or restore client traffic
	// while it was not switched on.
	//
	// The panel keeps the control out of reach when the box is off, so this is
	// the stale-tab case: somebody left the page open, switched the appliance
	// off elsewhere, and pressed the button. Without a word of its own the
	// refusal reaches them as "Caspian could not work out what", which is
	// untrue, since we know exactly what.
	FaultNotRunning Fault = "not-running"

	// FaultRadioBlocked means the radio is soft or hard blocked by rfkill.
	FaultRadioBlocked Fault = "radio-blocked"

	// FaultNoInternetInterface means nothing on the machine has a route out.
	FaultNoInternetInterface Fault = "no-internet-interface"

	// FaultChannelRefused means the radio would not take the channel it was
	// given, which on a single-radio machine usually means the access point
	// has to share the channel of the WiFi link carrying the internet.
	FaultChannelRefused Fault = "channel-refused"

	// FaultHotspotFailed means the access point software would not start.
	FaultHotspotFailed Fault = "hotspot-failed"

	// FaultDHCPFailed means the DHCP and DNS server would not start.
	FaultDHCPFailed Fault = "dhcp-failed"

	// FaultEngineRejectedConfig means the engine would not load the config.
	// This is the second of the three states design section 8 step 11 asks the
	// panel to tell apart.
	FaultEngineRejectedConfig Fault = "engine-rejected-config"

	// FaultServerNoAnswer means the config loaded and the server did not
	// answer. The third of the three states.
	FaultServerNoAnswer Fault = "server-no-answer"

	// FaultClockImplausible means the machine's clock is too far out to
	// attempt a handshake. Design section 9: REALITY writes the wall clock
	// into the handshake, and a Pi has no battery clock, so this must not be
	// reported as a bad config.
	FaultClockImplausible Fault = "clock-implausible"

	// FaultPermissionDenied means the privileged side was not allowed to do
	// something it needs to do.
	FaultPermissionDenied Fault = "permission-denied"

	// FaultSoftwareMissing means a program the box depends on is not
	// installed.
	FaultSoftwareMissing Fault = "software-missing"

	// FaultUnavailable means the privileged service could not be reached at
	// all. It is raised by this package, not by the privileged side, for the
	// obvious reason.
	FaultUnavailable Fault = "privileged-service-unavailable"

	// FaultIPv6Unsupported means the stored client IPv6 policy is one this
	// build cannot honour. Only state.ClientIPv6Block is supported.
	//
	// It is a fault of its own rather than FaultUnknown because the two send
	// the reader somewhere different. FaultUnknown says "we could not work out
	// what went wrong", which for a user who has just changed one setting in
	// advanced mode is both untrue and useless; this says which setting, and
	// that the answer is to put it back.
	FaultIPv6Unsupported Fault = "ipv6-unsupported"

	// FaultUnknown is for a failure the privileged side could not classify.
	// It exists so that an unclassified failure is reported as unclassified
	// rather than being forced into the nearest category, which would send the
	// user to fix the wrong thing.
	FaultUnknown Fault = "unknown"
)

// FaultError carries a Fault across the interface as an error.
//
// It deliberately holds nothing else. An implementation that wants to attach
// detail must not do it here: the detail on this path comes from the engine and
// the engine's error strings embed the user's keys (internal/engine/redact.go).
// A Fault cannot leak a credential because the set of values is closed and
// visible above.
type FaultError struct{ Fault Fault }

func (e *FaultError) Error() string { return "caspian: " + string(e.Fault) }

// Errorf-free helper for implementations and tests.
func faultErr(f Fault) error { return &FaultError{Fault: f} }

// FaultOf extracts the Fault from an error, reporting FaultUnknown for an error
// that carries none and FaultNone for a nil error.
func FaultOf(err error) Fault {
	if err == nil {
		return FaultNone
	}
	var fe *FaultError
	if errors.As(err, &fe) {
		return fe.Fault
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return FaultUnavailable
	}
	return FaultUnknown
}

// ---------------------------------------------------------------------------
// What the privileged side reports
// ---------------------------------------------------------------------------

// InterfaceKind is what an interface is, in terms a person recognises. It is
// how basic mode gets to say "Ethernet" and "built-in WiFi" instead of "eth0"
// and "wlan0" (design section 5.2).
type InterfaceKind string

const (
	KindUnknown     InterfaceKind = ""
	KindEthernet    InterfaceKind = "ethernet"
	KindBuiltinWiFi InterfaceKind = "builtin-wifi"
	KindUSBWiFi     InterfaceKind = "usb-wifi"
	KindWiFi        InterfaceKind = "wifi"
)

// InterfaceInfo is one network interface as the privileged side sees it.
type InterfaceInfo struct {
	// Name is the kernel name, for example eth0. It is shown in advanced mode
	// and used as the value of an override, never built into a command here.
	Name string

	// Kind is what to call it on screen.
	Kind InterfaceKind

	// CanHostAP reports whether this radio can run an access point. An
	// interface with this false must never be offered as a hotspot choice.
	CanHostAP bool

	// HasDefaultRoute reports whether the machine's route out uses it.
	HasDefaultRoute bool
}

// Detection is everything the panel needs to show what was decided and to offer
// the alternatives. Design section 5.4: every automatic choice is shown in
// basic mode in one line, and every automatic choice can be changed in advanced
// mode.
type Detection struct {
	// InternetInterface and HotspotInterface are the choices in force, whether
	// they were detected or set as an override.
	InternetInterface string
	HotspotInterface  string

	// Interfaces is every candidate, so advanced mode can offer a list rather
	// than a free text box. A free text box here would be a way to feed an
	// arbitrary string to the privileged side.
	Interfaces []InterfaceInfo

	// Channel, Band and Country are the radio settings in force.
	Channel int
	Band    string
	Country string

	// UsableChannels is what this radio will actually accept, excluding
	// channels needing radar detection.
	UsableChannels []int

	// ChannelPinned reports that the access point cannot choose its own
	// channel because it shares a radio with the WiFi link carrying the
	// internet. On the measured Pi 5 built-in radio this is true.
	ChannelPinned bool

	// Subnet is the hotspot subnet in CIDR form, chosen not to clash with the
	// network the box is already on.
	Subnet string

	// HotspotAddress is the address the box itself holds on the hotspot
	// interface. It is where the panel listens by default, and it is the
	// address the installer prints for the user (docs/LAYOUT.md). Empty until
	// the access point has been brought up, which is the hazard design section
	// 5.6 names and does not solve: the hotspot interface does not exist until
	// the access point starts.
	HotspotAddress string

	// LocalNetworkAddress is the address the box holds on the network it is
	// attached to. The panel binds to it only when the user has turned that on
	// (state.Advanced.PanelOnLAN), and only when it is a private address; see
	// BindAddrs.
	LocalNetworkAddress string

	// Fault is set when detection could not find a workable arrangement, for
	// example when no radio can host an access point.
	Fault Fault

	// At is when this was measured.
	At time.Time
}

// KindOf reports what to call the named interface.
func (d Detection) KindOf(name string) InterfaceKind {
	for _, i := range d.Interfaces {
		if i.Name == name {
			return i.Kind
		}
	}
	return KindUnknown
}

// HotspotStatus is the state of the access point and its DHCP server.
type HotspotStatus struct {
	// Running is true only when the access point is actually beaconing and the
	// DHCP server is up. A half-started hotspot is not running.
	Running bool

	// SSID is the network name currently being published.
	SSID string

	// Devices is how many devices hold a live DHCP lease.
	Devices int

	// UnreadableLeaseLines lets the panel say the device count may be short
	// rather than quietly under-reporting it.
	UnreadableLeaseLines int

	// Fault is why it is not running, when it is not.
	Fault Fault
}

// SystemStatus is one poll of the whole appliance.
type SystemStatus struct {
	// Engine is the tunnel's lifecycle state. Its Reason field has already
	// been through internal/engine.Redact, but it is engine vocabulary, so the
	// panel shows it only in advanced mode.
	Engine engine.State

	// Hotspot is the access point.
	Hotspot HotspotStatus

	// Detection is what is in force now, so a status poll and a detection do
	// not have to be two round trips.
	Detection Detection

	// ClientTrafficCut reports that forwarded client traffic is being dropped
	// at the user's request, while the hotspot, the DHCP and DNS server and
	// this panel keep working.
	//
	// It is runtime state on the privileged side and is written to no file, so
	// restarting the machine restores traffic without anybody having to
	// remember how it was turned off. The panel says so in the banner, because
	// a control with no visible way back is one people pull the plug over.
	ClientTrafficCut bool

	// At is when this was measured.
	At time.Time
}

// Connected reports whether client traffic can actually leave through the
// tunnel: the engine is running and the hotspot is up.
//
// Both halves are required on purpose. A running engine with no hotspot serves
// nobody, and a running hotspot with no engine is the fail-closed case where
// clients are attached and going nowhere. Design section 7.
// The cut clause is not decoration. While client traffic is cut the engine is
// running and the hotspot is running, so without it this returns true and the
// panel reports a working connection to somebody whose devices reach nothing.
// That is the same false green this package spent a week removing from the
// hotspot, and it would be reintroduced by the control that exists to stop
// traffic.
func (s SystemStatus) Connected() bool {
	return s.Engine.Phase == engine.PhaseRunning && s.Hotspot.Running && !s.ClientTrafficCut
}

// EngineLog is the recent engine output for advanced mode.
type EngineLog struct {
	// Entries are already redacted by internal/engine. They are engine
	// vocabulary and are never shown in basic mode.
	Entries []engine.LogEntry

	// Dropped is how many lines the ring has evicted, so the panel can say the
	// view is truncated instead of implying it is complete.
	Dropped uint64
}

// ---------------------------------------------------------------------------
// What the panel asks for
// ---------------------------------------------------------------------------

// StartRequest is the argument to ActionStart. Every field is typed, and the
// privileged side is expected to validate each against its own detection rather
// than trusting the panel.
//
// It carries two credentials, so it redacts itself; see String below.
type StartRequest struct {
	// ConfigJSON is the engine config document. It is produced by
	// internal/link from parsed structures and never by interpolating the text
	// the user pasted (design section 6). It is a credential: it carries the
	// user id, the REALITY key material and the server address.
	ConfigJSON []byte

	// Hotspot is the access point to publish.
	Hotspot HotspotSpec

	// Network is how the machine should be wired up.
	Network NetworkSpec

	// EngineLogLevel is one of EngineLogLevels, or empty for the engine's own
	// default. It is carried here rather than read from the state file by the
	// privileged side, so that one process owns reading state and the other
	// receives what it needs as an argument.
	EngineLogLevel string
}

// EngineLogLevels are the log levels the engine accepts, read from
// github.com/xtls/xray-core/infra/conf/log.go:49-61 at v1.260327.0. Anything
// the engine does not recognise falls through to its default, which is warning,
// so an unknown value is not an error there; it is refused here anyway, because
// a setting that silently does nothing is worse than one that is rejected.
//
// The engine also accepts "none", and this list deliberately leaves it out.
// "none" sets ErrorLogType to None, which empties the advanced-mode log view
// entirely and leaves somebody diagnosing a failure with nothing at all. The
// disclosure worry that would motivate it is already handled: every line goes
// through internal/engine.Redact before it is retained.
var EngineLogLevels = []string{"error", "warning", "info", "debug"}

// ValidEngineLogLevel reports whether level is one this panel will send.
// The empty string is valid and means "leave the engine's default alone".
func ValidEngineLogLevel(level string) bool {
	if level == "" {
		return true
	}
	for _, l := range EngineLogLevels {
		if l == level {
			return true
		}
	}
	return false
}

// HotspotSpec is the access point to publish.
type HotspotSpec struct {
	// SSID is the network name. Not secret: it is broadcast.
	SSID string

	// Passphrase is the WPA passphrase. It is a credential and must not reach
	// a log line.
	Passphrase string

	// Interface must be one of the names Detect reported with CanHostAP set.
	Interface string

	// Channel, Band and Country are the radio settings. Zero and empty mean
	// "use what detection chose", which keeps an override that cannot be
	// honoured from becoming a refusal to start.
	Channel int
	Band    string
	Country string

	// Subnet is the hotspot subnet in CIDR form.
	Subnet string
}

// String redacts the passphrase.
//
// The same reasoning as internal/state's Secret and internal/link's Link: a
// single %v on this struct in some later handler would otherwise write the
// user's WiFi passphrase into a log file. Defining String means every fmt verb
// that would have walked the struct calls this instead.
func (h HotspotSpec) String() string {
	return fmt.Sprintf("hotspot ssid=%q interface=%q channel=%d band=%q country=%q subnet=%q passphrase=[redacted]",
		h.SSID, h.Interface, h.Channel, h.Band, h.Country, h.Subnet)
}

// GoString covers %#v, which would otherwise print every exported field.
func (h HotspotSpec) GoString() string { return h.String() }

// NetworkSpec is how the machine should be wired up while the tunnel is on.
type NetworkSpec struct {
	// InternetInterface must be one of the names Detect reported.
	InternetInterface string

	// DNSMode is state.DNSModeTunnel or another value internal/state accepts.
	// It is never empty; internal/state refuses to persist an empty one,
	// because empty must never be readable as "let client traffic out".
	DNSMode string

	// OnTunnelDown is what happens to forwarded client traffic when the tunnel
	// drops, state.OnTunnelDownBlock by default. Never empty, same reason.
	OnTunnelDown string

	// ClientIPv6 is whether hotspot clients get IPv6 at all,
	// state.ClientIPv6Block by default. Never empty, and the reason is sharper
	// than for the two above: a client with a working IPv6 path prefers it over
	// IPv4, so an empty value read as "no policy configured" would not degrade
	// the tunnel, it would bypass it.
	//
	// The privileged side supports state.ClientIPv6Block alone and refuses
	// anything else with FaultIPv6Unsupported. This field carries what the user
	// stored, not what this build can honour; correcting it here would hide the
	// setting from the person who changed it.
	ClientIPv6 string
}

// String redacts the config document.
//
// ConfigJSON is the pasted credential in another shape, and this is the struct
// most likely to be handed to a log line at the moment something goes wrong.
// Its size is reported because "did we send anything at all" is a real
// question, and a byte count discloses nothing useful.
func (r StartRequest) String() string {
	return fmt.Sprintf("start config=[redacted %d bytes] %v network=%+v engine_log_level=%q",
		len(r.ConfigJSON), r.Hotspot, r.Network, r.EngineLogLevel)
}

// GoString covers %#v.
func (r StartRequest) GoString() string { return r.String() }
