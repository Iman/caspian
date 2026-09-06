// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/panel/qr"
	"caspianbyoc.org/caspian/internal/state"
)

// LTR is a value that must not have its parts reordered by the bidirectional
// algorithm when it sits inside right-to-left text.
//
// This is the single most likely thing to go wrong in a Persian interface, and
// it goes wrong invisibly to whoever built it. An address, a subnet, a port, an
// interface name or a WiFi passphrase dropped into a Persian paragraph is laid
// out by the Unicode bidi algorithm along with everything around it, and the
// neutral characters in it, the dots, colons, slashes and hyphens, take their
// direction from their surroundings. "10.62.0.1/24" can come out as
// "24/10.62.0.1". The user then types what they see.
//
// So every one of them is a distinct Go type, and the template wraps every
// LTR-typed value in a bdi element. TestEveryIsolatedValueIsIsolated fills each
// LTR field in this struct with a sentinel by reflection, renders the page, and
// fails if any sentinel comes out unisolated. That is why the type exists
// rather than a naming convention: a new field of this type is covered by the
// test the moment it is added, and a new field of type string is not.
//
// bdi rather than a span with a dir attribute, and bdi with no dir on
// user-supplied names: bdi defaults to dir="auto", which takes the direction
// from the first strong character. For an address that is left-to-right, which
// is what is wanted; for a Persian network name that is right-to-left, which is
// also what is wanted. Forcing dir="ltr" on a name the user chose would reverse
// their own words. The technical values that can never be Persian are given an
// explicit dir="ltr" in the template anyway.
type LTR string

// pageData is what a template is given.
//
// Everything the template renders is a field on this struct. It does not reach
// into Detection, link.Link or state.Advanced, and that is deliberate: the
// isolation test walks THIS type, so a value rendered from somewhere else would
// not be covered by it. Flattening is the price of the test being complete.
//
// The pasted config is not in here in any form. There is no field holding the
// raw text and none holding the config document. What the page shows about the
// config comes from internal/link's redacted view, which carries no key
// material at all.
type pageData struct {
	// Lang and Dir drive the html element.
	Lang    Lang
	Dir     string
	Title   string
	Version string

	// LangSwitchHref and LangSwitchLabel offer the other language.
	LangSwitchHref  string
	LangSwitchLabel string

	// CSRF is the token every form on the page carries.
	CSRF string

	// SignedIn says whether to draw the navigation rail.
	//
	// It is not derived from CSRF, although an earlier draft did that and it
	// looked right: the sign-in and first-run forms carry a token too, so a
	// rail keyed on the token would appear on the page whose whole point is
	// that nobody has proved who they are yet, with jumps into sections the
	// visitor cannot reach.
	SignedIn bool

	// Advanced is whether the advanced section is revealed. Design section 5.3
	// says it reveals and never hides, so nothing above it changes.
	Advanced     bool
	AdvancedHref string

	// ---- the tile row ----
	Tiles []Tile

	// ---- the switch ----
	StatusWord  string
	StatusShape string
	Connected   bool

	// TrafficCut is the emergency control's state: forwarded client traffic is
	// being dropped while the hotspot and this page keep working.
	TrafficCut bool

	// HeroClass is the ground of the control bar, decided on the server for the
	// same reason PowerClass is: the page must not hold a second opinion about
	// which state the box is in.
	HeroClass string

	// PowerClass is the appearance of the switch, decided on the server so the
	// rendered page and the polled update cannot disagree about it.
	PowerClass string

	// NextStep and NextLabel are the instruction shown to somebody who has
	// never seen this page.
	NextStep  string
	NextLabel string

	// Running is whether the appliance is switched on, which is NOT the same
	// question as Connected.
	//
	// The switch has to key on this one. Connected went false the moment the
	// cut arrived, correctly, because client traffic is not flowing; a switch
	// keyed on it then offered to switch ON a box that was already running,
	// and pressing it would have started an appliance that had never stopped.
	Running bool

	// ---- messages ----
	ProblemHeadline string
	ProblemAdvice   string
	ProblemDetail   string
	HasProblem      bool
	Notice          string

	// ---- the hotspot ----
	HotspotReady        bool
	SSID                LTR
	Passphrase          LTR
	QR                  template.HTML
	QRProblem           string
	DeviceLine          string
	SuggestedSSID       LTR
	SuggestedPassphrase LTR

	// ---- the config ----
	SetupIncomplete bool
	HasConfig       bool
	ConfigName      string
	ConfigSummary   LTR

	// ---- what was detected ----
	DetectedLine string

	// ---- events ----
	Events []EventLine

	// ---- advanced ----
	Interfaces      []IfaceChoice
	Channels        []ChannelChoice
	Bands           []BandOption
	LogLevels       []LogLevelOption
	CurrentInternet LTR
	CurrentHotspot  LTR
	CurrentChannel  LTR
	CurrentBand     string
	CurrentCountry  LTR
	CurrentSubnet   LTR
	PlaceCountry    LTR
	PlaceSubnet     LTR

	// AutoInternet and the four below it are the "let Caspian decide" option
	// of each menu, already carrying what Caspian would choose right now.
	//
	// They are plain strings rather than LTR because the technical value is
	// embedded in a translated sentence, and an option element cannot hold a
	// bdi: HTML allows no elements inside one. The isolation is done with the
	// Unicode characters instead, by isolateLTR, which is the mechanism bdi
	// itself is defined in terms of.
	AutoInternet  string
	AutoHotspot   string
	AutoChannel   string
	AutoBand      string
	AutoLogLevel  string
	ChannelPinned bool
	PanelOnLAN    bool
	ConfigFacts   []Fact
	FixedFacts    []Fact
	EnginePhase   string
	EngineReason  LTR
	EngineLog     []LogLine
	EngineDropped int
}

// Tile is one summary tile.
//
// A tile answers one question with one short value. If it needs a sentence to
// be understood it is the wrong tile, so there is no room for one here.
type Tile struct {
	// Label is the question, already resolved.
	Label string

	// Value is the answer, already resolved. It is not LTR: a device count and
	// a status word are ordinary text that should follow the page direction.
	Value string

	// ValueLTR is used instead of Value when the answer is a technical token
	// that must keep its own direction, such as a protocol name.
	ValueLTR LTR

	// Shape carries the state without colour: "on", "off", "working" or "".
	// The tiles that have no state leave it empty.
	Shape string
}

// EventLine is one entry in the events area, already a sentence.
type EventLine struct {
	// At is the time of day. LTR because it is a clock reading with a colon in
	// it, which is exactly the shape the bidi algorithm reorders.
	At LTR
	// Text is the sentence, in the reader's language.
	Text string
}

// LogLine is one engine log line for advanced mode.
type LogLine struct {
	At LTR
	// Text is engine output, already redacted by internal/engine. It is
	// machine English and is not translated; see the note on Problem.Detail.
	Text LTR
}

// Fact is a label and a value in the advanced tables.
type Fact struct {
	Label string
	// Value is a technical value: an address, a protocol, a transport name.
	Value LTR
	// Words is used instead of Value for a fact whose value is a sentence.
	Words string
}

// IfaceChoice is one option on an interface selector.
type IfaceChoice struct {
	Name     LTR
	Kind     string
	Note     string
	CanHost  bool
	Selected bool
}

// ChannelChoice is one option on the channel selector.
type ChannelChoice struct {
	Value    LTR
	Selected bool
}

// BandOption is one option on the band selector.
type BandOption struct {
	Value    string
	Words    string
	Selected bool
}

// LogLevelOption is one option on the log level selector.
type LogLevelOption struct {
	Value    LTR
	Selected bool
}

// T resolves a key in the page's language. Templates call it as {{.T "key"}}.
func (d pageData) T(k string) string { return T(d.Lang, Key(k)) }

// The status shape.
//
// The palette makes this rule matter more than it did. The soft green and the
// dusty brown are both desaturated and close in lightness, so as a pair
// carrying "on" against "trouble" they are weak for anyone with reduced colour
// vision and weak again in bright sunlight, which is where this box is used. So
// three things carry the state at once and colour is the least of them: the
// word, which is always present; the shape of the marker, a filled disc against
// a hollow ring against a dashed ring, which survives any colour treatment; and
// the colour, reinforcing rather than carrying.
const (
	shapeOn      = "on"
	shapeOff     = "off"
	shapeWorking = "working"
	// shapeTrouble is the lamp for a box that is running and not carrying
	// traffic. It is a broken ring, so it reads differently from all three
	// above without depending on its colour.
	shapeTrouble = "trouble"
)

func (p *Panel) newPageData(l Lang, title Key, csrf string, advanced bool) pageData {
	other := l.Other()
	return pageData{
		Lang:            l,
		Dir:             l.Dir(),
		Title:           T(l, title),
		Version:         Version,
		LangSwitchHref:  "/?lang=" + string(other),
		LangSwitchLabel: T(l, MsgOtherLanguage),
		CSRF:            csrf,
		Advanced:        advanced,
		AdvancedHref:    advancedHref(advanced),
		StatusWord:      T(l, MsgStatusOff),
		StatusShape:     shapeOff,
	}
}

func advancedHref(current bool) string {
	if current {
		return "/?advanced=0"
	}
	return "/?advanced=1"
}

// setProblem resolves a Problem into the page.
func (d *pageData) setProblem(p Problem) {
	if p.Empty() {
		return
	}
	d.HasProblem = true
	if p.Headline != "" {
		d.ProblemHeadline = T(d.Lang, p.Headline)
	}
	if p.Advice != "" {
		d.ProblemAdvice = T(d.Lang, p.Advice, p.AdviceArgs...)
	}
	d.ProblemDetail = p.Detail
}

// fillStatus writes the status word, the switch state and the device count.
func (d *pageData) fillStatus(st SystemStatus, fault Fault) {
	l := d.Lang
	d.DetectedLine = DetectedLineIn(l, st.Detection)
	d.DeviceLine = DeviceCountLine(l, st.Hotspot)
	d.EnginePhase = T(l, phaseKey(st.Engine.Phase.String()))
	d.EngineReason = LTR(st.Engine.Reason)
	d.TrafficCut = st.ClientTrafficCut
	d.Running = st.Engine.Phase == engine.PhaseRunning && st.Hotspot.Running

	switch {
	case fault != FaultNone:
		d.StatusWord, d.StatusShape = T(l, MsgStatusNotWorking), shapeOff
		if !d.HasProblem {
			d.setProblem(Problem{Headline: fault.Key()})
		}
	case st.ClientTrafficCut:
		// Running, and deliberately carrying nothing. "Off" would be untrue
		// and "connected" would be the false green the cut exists to avoid,
		// so this state gets a word of its own and Connected stays false:
		// that field answers "can client traffic leave", and it cannot.
		d.StatusWord, d.StatusShape = T(l, MsgStatusCut), shapeTrouble
	case st.Connected():
		d.Connected = true
		d.StatusWord, d.StatusShape = T(l, MsgStatusConnected), shapeOn
	case st.Engine.Phase == engine.PhaseStarting:
		d.StatusWord, d.StatusShape = T(l, MsgStatusStarting), shapeWorking
	case st.Engine.Phase == engine.PhaseFailed:
		d.StatusWord, d.StatusShape = T(l, MsgStatusNotConn), shapeOff
	default:
		d.StatusWord, d.StatusShape = T(l, MsgStatusOff), shapeOff
	}

	// Decided once, after the state is known, and read by both the rendered
	// page and the polled update from the one function.
	d.PowerClass = powerClass(d.Connected, d.Running)
	d.HeroClass = heroClass(d.TrafficCut, d.Connected, st.Engine.Phase == engine.PhaseStarting)
	d.NextLabel = T(l, MsgNextLabel)

	// A running engine with no hotspot is worth saying out loud rather than
	// reporting as "not connected": the tunnel is up and no device can use it,
	// which is a different problem with a different fix.
	if st.Engine.Phase == engine.PhaseRunning && !st.Hotspot.Running && !d.HasProblem {
		if f := st.Hotspot.Fault; f != FaultNone {
			d.setProblem(Problem{Headline: f.Key()})
		} else {
			d.setProblem(Problem{Headline: MsgTunnelNoHotspot})
		}
	}
	if !d.Connected && !d.HasProblem && st.Detection.Fault != FaultNone {
		d.setProblem(Problem{Headline: st.Detection.Fault.Key()})
	}
}

// fillTiles builds the summary row.
//
// Four tiles, and the reasoning for the set is worth stating because a tile
// that does not earn its place is worse than no tile: it is one more thing to
// read past.
//
//   - Is it on. The single most important fact, and the switch lives in the
//     same tile so the control is not separated from the state it reports.
//   - How many devices. The answer to "is it actually working for the room",
//     which is the question this product exists to answer.
//   - Which config. Design section 5.2 asks for it on the status line, and it
//     is the one thing a user changes; after replacing a config, "which one am
//     I on" is a real question.
//   - How long it has been up. This one earns its place for a reason that is
//     not obvious: it is the only thing on the screen that distinguishes a
//     steady connection from one that is dropping and reconnecting. A box that
//     says "2 minutes" when the user has had it on all afternoon has told them
//     something no other tile can.
func (d *pageData) fillTiles(st SystemStatus, fault Fault, now func() time.Time) {
	l := d.Lang

	status := Tile{Label: T(l, MsgTileStatus), Value: d.StatusWord, Shape: d.StatusShape}

	devices := Tile{Label: T(l, MsgTileDevices), Value: strconv.Itoa(st.Hotspot.Devices)}

	config := Tile{Label: T(l, MsgTileConfig), Value: T(l, MsgTileConfigNone)}
	if d.HasConfig {
		if d.ConfigName != "" {
			config.Value = d.ConfigName
		} else {
			config.Value = ""
			config.ValueLTR = d.ConfigSummary
		}
	}

	uptime := Tile{Label: T(l, MsgTileUptime), Value: T(l, MsgTileUptimeOff)}
	if fault == FaultNone && st.Connected() && !st.Engine.Since.IsZero() {
		uptime.Value = UptimeWords(l, now().Sub(st.Engine.Since))
	}

	d.Tiles = []Tile{status, devices, config, uptime}
}

// fillConfig writes what the page says about the stored config.
//
// The raw text is read from state here and used for exactly one thing: parsing
// it. What leaves this function is the parsed, redacted view.
func (d *pageData) fillConfig(proxy state.ProxyConfig) {
	d.HasConfig = proxy.IsConfigured()
	if !d.HasConfig {
		return
	}
	d.ConfigName = proxy.Label

	l, err := link.Parse(proxy.Raw.Reveal())
	if err != nil {
		// A stored config that no longer parses is a real state: it can happen
		// after an engine upgrade drops a transport. Saying so is better than
		// showing an empty box.
		if !d.HasProblem {
			d.setProblem(ParseProblem(err))
		}
		d.ConfigSummary = ""
		return
	}
	d.ConfigSummary = LTR(fmt.Sprintf("%s %s", l.Protocol, l.Address))
	d.fillConfigFacts(l)
}

func (d *pageData) fillConfigFacts(l *link.Link) {
	lang := d.Lang
	add := func(k Key, v string) {
		if v != "" {
			d.ConfigFacts = append(d.ConfigFacts, Fact{Label: T(lang, k), Value: LTR(v)})
		}
	}
	add(MsgAdvConfigKind, l.Protocol)
	add(MsgAdvConfigServer, fmt.Sprintf("%s:%d", l.Address, l.Port))
	add(MsgAdvConfigTransport, l.Network)
	add(MsgAdvConfigSecurity, string(l.Security))
	add(MsgAdvConfigSNI, l.ServerName)
	add(MsgAdvConfigFP, l.Fingerprint)
	add(MsgAdvConfigFlow, l.Flow)

	if l.Security == link.SecurityReality {
		present := func(ok bool) string {
			if ok {
				return T(lang, MsgAdvPresent)
			}
			return T(lang, MsgAdvAbsent)
		}
		notset := func(ok bool) string {
			if ok {
				return T(lang, MsgAdvPresent)
			}
			return T(lang, MsgAdvNotSet)
		}
		d.ConfigFacts = append(d.ConfigFacts,
			Fact{Label: T(lang, MsgAdvRealityKey), Words: present(l.Reality.HasPublicKey)},
			Fact{Label: T(lang, MsgAdvRealityShortID), Words: notset(l.Reality.HasShortID)},
			Fact{Label: T(lang, MsgAdvRealityPQV), Words: notset(l.Reality.HasMldsa65Verify)},
		)
	}
	if l.Count > 1 {
		d.ConfigFacts = append(d.ConfigFacts, Fact{
			Label: T(lang, MsgAdvConfigCount),
			Words: T(lang, MsgAdvConfigCount, l.Count),
		})
	}
}

// fillHotspot writes the hotspot name, passphrase and join code.
func (d *pageData) fillHotspot(h state.HotspotConfig) {
	d.SSID = LTR(h.SSID)
	d.Passphrase = LTR(h.Passphrase.Reveal())
	d.HotspotReady = h.SSID != "" && !h.Passphrase.IsZero()
	if !d.HotspotReady {
		d.SuggestedSSID = LTR(SuggestSSID())
		d.SuggestedPassphrase = LTR(SuggestPassphrase())
		return
	}
	// The join string carries the passphrase, so this is the one place in the
	// page build where a credential becomes markup. qr.Matrix.SVG puts no byte
	// of its input into its output; it emits integers only, which is what makes
	// marking it template.HTML safe. TestSVGCarriesNoInputText holds that.
	m, err := qr.Encode([]byte(qr.WiFiJoin(string(d.SSID), string(d.Passphrase), false)))
	if err != nil {
		d.QRProblem = T(d.Lang, MsgWifiQRFailed)
		return
	}
	d.QR = template.HTML(m.SVG())
}

// fillNextStep turns the state into the one thing to do now.
//
// It runs AFTER the other fills and reads what is SAVED rather than what is
// running, which is the distinction that made the first version wrong: it asked
// the live hotspot for its name, and a hotspot that is not running has no name,
// so the page went on telling somebody to choose a WiFi name for as long as the
// box was switched off, however many times they had already chosen one.
func (d *pageData) fillNextStep(hasHotspot, cut, starting bool, devices int) {
	d.NextLabel = T(d.Lang, MsgNextLabel)
	d.NextStep = T(d.Lang, nextStep(
		d.HasProblem, d.HasConfig, hasHotspot, cut, d.Connected, starting, devices))
}

// fillEvents writes the events area.
func (d *pageData) fillEvents(events []Event) {
	for _, e := range events {
		d.Events = append(d.Events, EventLine{
			At:   LTR(e.At.Format("15:04")),
			Text: e.Sentence(d.Lang),
		})
	}
}

// fillAdvanced writes the fields design section 5.3 lists.
func (d *pageData) fillAdvanced(adv state.Advanced, det Detection, log EngineLog) {
	l := d.Lang

	d.CurrentInternet = LTR(adv.InternetInterface)
	d.CurrentHotspot = LTR(adv.HotspotInterface)
	d.CurrentBand = adv.Band
	d.CurrentCountry = LTR(adv.Country)
	d.CurrentSubnet = LTR(adv.Subnet)
	d.PlaceCountry = LTR(det.Country)
	d.PlaceSubnet = LTR(det.Subnet)
	d.ChannelPinned = det.ChannelPinned
	d.PanelOnLAN = adv.PanelOnLAN
	if adv.Channel != 0 {
		d.CurrentChannel = LTR(strconv.Itoa(adv.Channel))
	}

	// What each menu would pick if it is left alone. Every one of these was
	// the literal text "%s" on the page until 2026-08-30: the catalogue
	// sentence takes an argument and the template called it with none, so
	// five menus offered "Let Caspian decide (now: %s)". The fix is the
	// argument, not a sentence without one, because which interface Caspian
	// would choose is the only thing that makes the option decidable.
	d.AutoInternet = T(l, MsgAdvAuto, isolateLTR(det.InternetInterface))
	d.AutoHotspot = T(l, MsgAdvAuto, isolateLTR(det.HotspotInterface))
	if det.Channel != 0 {
		d.AutoChannel = T(l, MsgAdvAuto, isolateLTR(strconv.Itoa(det.Channel)))
	} else {
		d.AutoChannel = T(l, MsgAdvAutoPlain)
	}
	d.AutoBand = T(l, MsgAdvAutoPlain)
	for _, b := range BandChoices {
		if b.Value == det.Band {
			d.AutoBand = T(l, MsgAdvAuto, T(l, b.WordsKey))
			break
		}
	}
	d.AutoLogLevel = T(l, MsgAdvAuto, isolateLTR(defaultEngineLogLevel))

	for _, i := range det.Interfaces {
		c := IfaceChoice{Name: LTR(i.Name), CanHost: i.CanHostAP}
		if k := i.Kind.Key(); k != "" {
			c.Kind = T(l, k)
		}
		switch {
		case !i.CanHostAP:
			c.Note = T(l, MsgAdvCannotHostAP)
		case i.HasDefaultRoute:
			c.Note = T(l, MsgAdvInUse)
		}
		d.Interfaces = append(d.Interfaces, c)
	}
	for _, ch := range det.UsableChannels {
		d.Channels = append(d.Channels, ChannelChoice{
			Value:    LTR(strconv.Itoa(ch)),
			Selected: adv.Channel == ch,
		})
	}
	for _, b := range BandChoices {
		d.Bands = append(d.Bands, BandOption{
			Value: b.Value, Words: T(l, b.WordsKey), Selected: adv.Band == b.Value,
		})
	}
	for _, lv := range EngineLogLevels {
		d.LogLevels = append(d.LogLevels, LogLevelOption{
			Value: LTR(lv), Selected: adv.EngineLogLevel == lv,
		})
	}

	d.FixedFacts = []Fact{
		{Label: T(l, MsgAdvDNSLabel), Words: T(l, MsgAdvDNSValue)},
		{Label: T(l, MsgAdvDropLabel), Words: T(l, MsgAdvDropValue)},
	}

	for _, e := range log.Entries {
		d.EngineLog = append(d.EngineLog, LogLine{
			At:   LTR(e.At.Format("15:04:05")),
			Text: LTR(e.Text),
		})
	}
	d.EngineDropped = int(log.Dropped)
}

// ---------------------------------------------------------------------------
// The JSON the page polls
// ---------------------------------------------------------------------------

// statusJSON carries no credential and no engine text: the SSID is broadcast
// anyway, and everything else is a word or a number. The passphrase is not
// here, and nor is anything about the config beyond whether one exists, because
// this document is fetched every few seconds and is the easiest thing in the
// panel to end up in a browser cache or a developer console.
type statusJSON struct {
	Connected bool `json:"connected"`

	// Running is whether the appliance is switched on. Connected is whether
	// client traffic can actually leave, which the cut makes false while the
	// box is still running, and the switch has to key on this one or it offers
	// to start a box that never stopped.
	Running bool `json:"running"`

	// HeroClass is how the control bar's ground should look. Same reason as
	// PowerClass: four states cannot be derived from one boolean.
	HeroClass string `json:"heroClass"`

	// PowerClass is how the switch should look, decided here rather than in
	// the script, because three states cannot be derived from one boolean and
	// a second opinion about which is which is how a page and a box start
	// disagreeing.
	PowerClass string `json:"powerClass"`

	// NextStep lets the instruction change as the box changes, without a
	// reload. Somebody watching the page while it connects should see the
	// instruction become the next one rather than stay on the last.
	NextStep   string `json:"nextStep"`
	Word       string `json:"word"`
	Shape      string `json:"shape"`
	Devices    int    `json:"devices"`
	DeviceLine string `json:"deviceLine"`
	Detected   string `json:"detected"`
	Problem    string `json:"problem"`
	HasConfig  bool   `json:"hasConfig"`
	Uptime     string `json:"uptime"`

	// PowerLabel is the word on the switch, already in the reader's language.
	//
	// It is here because the script had the two English words written into it,
	// and wrote them over the Persian ones five seconds after a Persian page
	// loaded. The file's own comment said it "has no idea which language the
	// page is in and must not learn", which was true of every other field and
	// false of this one. Every string the script displays now comes from the
	// server already translated.
	PowerLabel string `json:"powerLabel"`

	// TrafficCut lets the poll show the cut without a reload, and lets the
	// script keep the control's label in step with the box rather than with
	// whatever it was when the page was rendered.
	TrafficCut bool     `json:"trafficCut"`
	CutLabel   string   `json:"cutLabel"`
	CutBanner  string   `json:"cutBanner"`
	Events     []string `json:"events"`
}

func (p *Panel) newStatusJSON(l Lang, st SystemStatus, fault Fault, hasConfig, hasHotspot bool, events []Event) statusJSON {
	d := pageData{Lang: l}
	d.HasConfig = hasConfig
	d.fillStatus(st, fault)
	d.fillTiles(st, fault, p.now)
	d.fillNextStep(hasHotspot, st.ClientTrafficCut,
		st.Engine.Phase == engine.PhaseStarting, st.Hotspot.Devices)

	out := statusJSON{
		Connected:  d.Connected,
		Running:    d.Running,
		Word:       d.StatusWord,
		Shape:      d.StatusShape,
		Devices:    st.Hotspot.Devices,
		DeviceLine: d.DeviceLine,
		Detected:   d.DetectedLine,
		Problem:    strings.TrimSpace(strings.TrimSpace(d.ProblemHeadline + " " + d.ProblemAdvice)),
		HasConfig:  hasConfig,
		Uptime:     d.Tiles[3].Value,
		PowerLabel: powerLabel(l, d.Running),
		HeroClass:  heroClass(d.TrafficCut, d.Connected, st.Engine.Phase == engine.PhaseStarting),
		PowerClass: powerClass(d.Connected, d.Running),
		NextStep:   d.NextStep,
		TrafficCut: st.ClientTrafficCut,
		CutLabel:   cutLabel(l, st.ClientTrafficCut),
		CutBanner:  T(l, MsgCutBanner),
	}
	for _, e := range events {
		out.Events = append(out.Events, e.Sentence(l))
	}
	return out
}

// Recovery commands are executable text, identical in both languages.
func (pageData) RecoveryUnixCommand() string { return "sudo /usr/local/bin/caspian reset-password" }
func (pageData) RecoveryWindowsCommand() string {
	return `& "$env:ProgramFiles\Caspian\caspian.exe" reset-password`
}
