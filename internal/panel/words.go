// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"errors"
	"time"

	"caspianbyoc.org/caspian/internal/link"
)

// ---------------------------------------------------------------------------
// What the panel says, as keys rather than as sentences.
//
// Nothing in this file is a string a user reads. Every message is a Key
// resolved against the catalogue in i18n_messages.go at render time, in
// whichever language the browser has chosen.
//
// That is structural, not stylistic. A Problem carrying a Go string would let a
// handler write an English sentence inline, and a bilingual interface rots one
// inline string at a time: the new message is added in the language the author
// speaks, nothing fails, and the other language quietly develops holes.
// Carrying a Key means the only way to say something new is to add it to both
// catalogues. TestNoInlineUserFacingStringsInGoSource parses this package and
// fails on a string literal assigned to a Headline or an Advice.
//
// The audience has not changed: somebody who received a config from a person
// they trust and wants the devices in the room to work (design section 5.1).
// "No adapter on this machine can create a hotspot. Plug in a USB WiFi adapter"
// is right, and "no AP-capable phy" is not, in both languages.
// ---------------------------------------------------------------------------

// ConfigStage is which of the three failure states a config is in. Design
// section 8, step 11: "The three failure states are distinguished in plain
// words: link did not parse, engine rejected it, server did not answer."
//
// The distinction survives translation because it is carried by the Stage, and
// because the three headline keys are different keys in both catalogues.
// TestTheThreeFailureStatesAreDistinguished checks the wording in every
// language rather than in English only.
type ConfigStage int

const (
	// StageNone means the problem is not about the config at all.
	StageNone ConfigStage = iota
	// StageParse means the text was not a proxy link this box understands.
	StageParse
	// StageEngine means it parsed, and the connection software refused it.
	StageEngine
	// StageServer means it loaded, and the server did not answer.
	StageServer
)

func (s ConfigStage) String() string {
	switch s {
	case StageParse:
		return "parse"
	case StageEngine:
		return "engine"
	case StageServer:
		return "server"
	default:
		return "none"
	}
}

// Problem is one thing to tell the user: a headline naming which of the three
// states this is, and advice on what to do about it.
type Problem struct {
	Stage ConfigStage

	// Headline and Advice are message keys, never sentences.
	Headline Key
	Advice   Key

	// AdviceArgs are substituted into the advice, for the one message that
	// needs a number in it.
	AdviceArgs []any

	// Detail is the connection software's own words, already redacted by
	// internal/engine. It is NOT translated and NOT a key: it is a machine's
	// English, shown only in advanced mode, where somebody who wants it will
	// look. Putting it in front of this audience in basic mode would be the
	// jargon mistake in another costume.
	Detail string
}

// Empty reports whether there is nothing to show.
func (p Problem) Empty() bool { return p.Headline == "" && p.Advice == "" }

// TextIn is the whole message on one line, in a language.
func (p Problem) TextIn(l Lang) string {
	head, advice := T(l, p.Headline), T(l, p.Advice)
	switch {
	case p.Headline == "":
		return advice
	case p.Advice == "":
		return head
	default:
		return head + " " + advice
	}
}

// HeadlineIn is the headline in a language.
func (p Problem) HeadlineIn(l Lang) string { return T(l, p.Headline) }

// Text and HeadlineText are the English renderings, for terminal output and
// for tests.
//
// They exist for the same reason DetectedLine does: a caller outside the panel
// has no browser to have chosen a language, and a Persian sentence in the
// middle of an English diagnostic report helps nobody. Anything with a reader
// who HAS chosen a language must call TextIn or HeadlineIn.
func (p Problem) Text() string         { return p.TextIn(LangEN) }
func (p Problem) HeadlineText() string { return p.HeadlineIn(LangEN) }

// ParseProblem turns an error from internal/link into a message.
//
// It branches on the package's sentinel errors rather than on the error text,
// so a reworded sentinel does not silently fall through to the last resort. It
// never carries any part of the pasted text, including the scheme, because an
// error page is a document people screenshot and send to somebody for help.
func ParseProblem(err error) Problem {
	p := Problem{Stage: StageParse, Headline: MsgParseHeadline}
	switch {
	case err == nil:
		return Problem{}
	case errors.Is(err, link.ErrEmpty):
		p.Headline, p.Advice = MsgParseEmpty, MsgParseEmptyAdvice
	case errors.Is(err, link.ErrUnsupportedScheme):
		p.Advice = MsgParseScheme
	case errors.Is(err, link.ErrNoLink):
		p.Advice = MsgParseNoLink
	case errors.Is(err, link.ErrBadUUID):
		p.Advice = MsgParseUUID
	case errors.Is(err, link.ErrBadAddress):
		p.Advice = MsgParseAddress
	case errors.Is(err, link.ErrBadPort):
		p.Advice = MsgParsePort
	case errors.Is(err, link.ErrBadReality):
		p.Advice = MsgParseReality
	case errors.Is(err, link.ErrUnsupportedTransport):
		p.Advice = MsgParseTransport
	default:
		p.Advice = MsgParseOther
	}
	return p
}

// EngineProblem is the second state: the link parsed and the engine refused the
// config built from it.
func EngineProblem() Problem {
	return Problem{Stage: StageEngine, Headline: MsgEngineHeadline, Advice: MsgEngineAdvice}
}

// ServerProblem is the third state: the config loaded and nothing answered.
//
// The advice puts the machine's own internet connection first, because that is
// the one the user can check and the one most often at fault. Blaming the
// config first is what makes somebody throw away a config that was never
// broken.
func ServerProblem() Problem {
	return Problem{Stage: StageServer, Headline: MsgServerHeadline, Advice: MsgServerAdvice}
}

// StartProblem turns a fault from a failed start into a message.
//
// Two of the faults are config states. Everything else is a fault of the
// machine and must not be worded as though the user pasted something wrong.
// FaultClockImplausible is the sharpest example: design section 9 warns that a
// clock failure surfaces as an authentication failure "that the panel will
// blame on the config", so its message says so explicitly.
func StartProblem(f Fault) Problem {
	switch f {
	case FaultNone:
		return Problem{}
	case FaultEngineRejectedConfig:
		return EngineProblem()
	case FaultServerNoAnswer:
		return ServerProblem()
	default:
		return Problem{Stage: StageNone, Headline: f.Key()}
	}
}

// Key is the message for a fault.
func (f Fault) Key() Key {
	switch f {
	case FaultNone:
		return ""
	case FaultNoAPAdapter:
		return MsgFaultNoAPAdapter
	case FaultHotspotInterfaceBusy:
		return MsgFaultIfaceBusy
	case FaultNotRunning:
		return MsgFaultNotRunning
	case FaultRadioBlocked:
		return MsgFaultRadioBlocked
	case FaultNoInternetInterface:
		return MsgFaultNoInternet
	case FaultChannelRefused:
		return MsgFaultChannel
	case FaultHotspotFailed:
		return MsgFaultHotspot
	case FaultDHCPFailed:
		return MsgFaultDHCP
	case FaultEngineRejectedConfig:
		return MsgEngineHeadline
	case FaultServerNoAnswer:
		return MsgServerHeadline
	case FaultClockImplausible:
		return MsgFaultClock
	case FaultPermissionDenied:
		return MsgFaultPermission
	case FaultSoftwareMissing:
		return MsgFaultSoftware
	case FaultUnavailable:
		return MsgFaultUnavailable
	case FaultIPv6Unsupported:
		return MsgFaultIPv6Unsupported
	case FaultUnknown:
		return MsgFaultUnknown
	default:
		// A fault this build does not know about. Saying so is better than
		// picking the nearest sentence, which would be a confident wrong
		// answer, and better than showing the code, which means nothing to
		// this audience in either language.
		return MsgFaultUnrecognised
	}
}

// faults is every fault, so a test can check each has a message in both
// languages.
var faults = []Fault{
	FaultNoAPAdapter, FaultHotspotInterfaceBusy, FaultNotRunning, FaultRadioBlocked, FaultNoInternetInterface, FaultChannelRefused,
	FaultHotspotFailed, FaultDHCPFailed, FaultEngineRejectedConfig, FaultServerNoAnswer,
	FaultClockImplausible, FaultPermissionDenied, FaultSoftwareMissing, FaultUnavailable,
	FaultIPv6Unsupported, FaultUnknown,
}

// Key is what to call an interface kind on screen.
func (k InterfaceKind) Key() Key {
	switch k {
	case KindEthernet:
		return MsgKindEthernet
	case KindBuiltinWiFi:
		return MsgKindBuiltinWiFi
	case KindUSBWiFi:
		return MsgKindUSBWiFi
	case KindWiFi:
		return MsgKindWiFi
	default:
		return ""
	}
}

// DetectedLineIn is the single line the dashboard shows about what was
// detected, for example "Internet: Ethernet. Hotspot: built-in WiFi."
//
// Design section 5.4 requires this line to exist: an automatic choice that is
// never displayed is one nobody can tell is wrong.
//
// The kernel name is used when the kind is not known, because a name the user
// can compare against something is more use than the word "unknown" in either
// language.
func DetectedLineIn(l Lang, d Detection) string {
	name := func(iface string) string {
		if iface == "" {
			return T(l, MsgDetectedNone)
		}
		if k := d.KindOf(iface).Key(); k != "" {
			return T(l, k)
		}
		return iface
	}
	return T(l, MsgDetectedLine, name(d.InternetInterface), name(d.HotspotInterface))
}

// DetectedLine is DetectedLineIn in English, for terminal output.
//
// It exists because `caspian check` prints this line into a diagnostic report
// whose surrounding text is English and where there is no browser to have
// chosen a language. Rendering it in the product's default language there would
// put one Persian sentence in the middle of an English report.
//
// Anything with a reader who has chosen a language must call DetectedLineIn.
func DetectedLine(d Detection) string { return DetectedLineIn(LangEN, d) }

// DeviceCountLine is how many devices are on the hotspot, as a sentence.
func DeviceCountLine(l Lang, h HotspotStatus) string {
	var s string
	switch h.Devices {
	case 0:
		s = T(l, MsgDevicesNone)
	case 1:
		s = T(l, MsgDevicesOne)
	default:
		s = T(l, MsgDevicesMany, h.Devices)
	}
	if h.UnreadableLeaseLines > 0 {
		// Under-reporting silently is worse than saying the count may be
		// short: the user counts the phones in the room, gets a different
		// answer, and stops trusting the screen.
		s += T(l, MsgDevicesUnreadable)
	}
	return s
}

// UptimeWords is how long the tunnel has been up, in plain words.
//
// Coarse on purpose. The question a tile answers is "has this been steady or
// has it been flapping", and "3 hours" answers it while "3h 12m 40s" makes the
// reader do arithmetic to find out.
func UptimeWords(l Lang, d time.Duration) string {
	switch {
	case d < time.Minute:
		return T(l, MsgUptimeJustNow)
	case d < time.Hour:
		return T(l, MsgUptimeMinutes, int(d.Minutes()))
	case d < 24*time.Hour:
		return T(l, MsgUptimeHours, int(d.Hours()))
	default:
		return T(l, MsgUptimeDays, int(d.Hours()/24))
	}
}

// humanSeconds is the wait the rate limiter reports.
func humanSeconds(l Lang, n int) string {
	switch {
	case n < 60:
		return T(l, MsgSeconds, n)
	case n < 120:
		return T(l, MsgMinute)
	default:
		return T(l, MsgMinutes, (n+59)/60)
	}
}

// phaseKey is the engine phase in words, for advanced mode.
func phaseKey(phase string) Key {
	switch phase {
	case "stopped":
		return MsgPhaseStopped
	case "starting":
		return MsgPhaseStarting
	case "running":
		return MsgPhaseRunning
	case "failed":
		return MsgPhaseFailed
	default:
		return MsgPhaseStopped
	}
}

// isolateLTR wraps a strictly left-to-right technical value in the Unicode
// isolate characters, so that it keeps its own direction inside a Persian
// sentence.
//
// This exists for the one place a bdi element cannot go. An option element is
// only allowed to contain text, so a menu entry that reads "Let Caspian decide
// (now: eth0)" in Persian has no element to isolate the interface name with,
// and without isolation the bracket and the name reorder into nonsense. U+2066
// LEFT-TO-RIGHT ISOLATE and U+2069 POP DIRECTIONAL ISOLATE are what bdi is
// defined in terms of, so this is the same isolation by a different spelling.
//
// Anywhere an element is allowed, use bdi instead: it survives a copy and
// paste that strips control characters, and TestEveryIsolatedValueIsIsolated
// looks for it.
func isolateLTR(s string) string {
	if s == "" {
		return ""
	}
	return "\u2066" + s + "\u2069"
}

// defaultEngineLogLevel is what the engine uses when the setting is left
// empty. It repeats internal/xcfg's LogWarning rather than importing it,
// because the panel showing a different word from the one the engine uses
// should be a failing test and not a compile error somebody silences.
const defaultEngineLogLevel = "warning"

// powerLabel is the word on the switch for a state.
//
// One function, used by the page and by the status the script polls, so the
// two cannot disagree about what the button says.
func powerLabel(l Lang, running bool) string {
	if running {
		return T(l, MsgSwitchOff)
	}
	return T(l, MsgSwitchOn)
}

// cutLabel is the word on the client-traffic control for a state.
//
// One function, used by the page and by the status the script polls, so the
// two cannot disagree about what the button says. Same reason as powerLabel,
// and the same defect avoided: the words that were written into the script
// relabelled a Persian page in English five seconds after it loaded.
func cutLabel(l Lang, cut bool) string {
	if cut {
		return T(l, MsgCutRestoreButton)
	}
	return T(l, MsgCutButton)
}

// heroClass is the ground of the control bar for a state.
//
// Four states, and the three the user named are a traffic light: green when the
// box is connected and carrying, amber while it is starting up, red when it is
// off. The fourth, the cut, keeps its own coral pulse because it is the one
// state the box is deliberately not serving anybody.
//
// Amber takes the STARTING PHASE and not a "running but not connected" flag.
// The first version took that flag and the state could never occur, so the
// amber rule was dead code and a starting box drew red. Running is
// "engine running AND hotspot up" and Connected is the same thing AND not cut
// (SystemStatus.Connected), so running-and-not-connected is exactly the cut,
// which is already handled above it. Found by the browser suite, which measured
// all 32 combinations of phase, hotspot, cut and fault and never once saw
// this function return "wait".
//
// Amber is the state that had no colour of its own. A box that was starting, and
// a box that was off, both drew the plain page ground, so the two looked
// identical while one of them was seconds from working and the other needed a
// press. The sentence beside the bar already says which is which; now the ground
// agrees with it, and this time the state it names is one that happens.
//
// There is no state for "connected with the kill switch off", because there is
// no such state to have: internal/privsvc/validate.go refuses any tunnel-down
// policy except block, so forwarded client traffic is always stopped when the
// tunnel drops and the user cannot turn that off. If that ever becomes a choice,
// this function is where the fourth colour goes, and the test below will fail
// until it does.
//
// Colour is reinforcement and never the carrier: the state word, the lamp's
// shape and the next-step sentence each say it independently.
func heroClass(cut, connected, starting bool) string {
	switch {
	case cut:
		return "cut"
	case connected:
		return "ok"
	case starting:
		return "wait"
	default:
		return "off"
	}
}

// powerClass is the appearance of the switch for a state.
//
// Three states, not two. Off is the teal that every primary action on this page
// uses. Running AND carrying traffic is the soft green: a lit power button on an
// appliance means it is on and well, and until now this button was the dusty
// brown whenever the box was on, so a working appliance and a cut one looked
// the same. Running and NOT carrying, which is the cut and the fault, keeps the
// brown, because that is the state somebody needs to notice.
//
// Colour is reinforcement here and never the carrier: the word on the button
// says what pressing it does, the state word above says what the box is doing,
// and the lamp beside it differs in shape as well as in colour.
func powerClass(connected, running bool) string {
	switch {
	case connected:
		return "ok"
	case running:
		return "danger"
	default:
		return "go"
	}
}

// nextStep is the one thing the reader should do now.
//
// The panel is used by people who have never seen it and will not read a manual
// before pressing something. Every other line on the page reports state; this
// one turns the state into an instruction, and it is ordered so that the most
// blocking thing wins: a fault before an empty config, an empty config before
// an unnamed hotspot, and a working box before anything else.
//
// It is a Key rather than a sentence so the two catalogues stay the only place
// wording lives, and it is computed on the server so the page and the poll
// agree, which is the same rule the switch label and the switch appearance
// follow.
func nextStep(hasProblem, hasConfig, hasHotspot, cut, connected, starting bool, devices int) Key {
	switch {
	case cut:
		return MsgNextResume
	case hasProblem:
		return MsgNextProblem
	case !hasConfig:
		return MsgNextAddconfig
	case !hasHotspot:
		return MsgNextSetwifi
	case starting:
		return MsgNextStarting
	case connected && devices > 0:
		return MsgNextJoined
	case connected:
		return MsgNextJoin
	default:
		return MsgNextSwitchon
	}
}
