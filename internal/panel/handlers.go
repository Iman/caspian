// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/state"
)

// advancedCookie remembers whether the advanced section is revealed. It is a
// display preference and holds nothing else.
const advancedCookie = "caspian_advanced"

// formCookie carries the token for the two forms that exist before there is a
// session: login and setup. See checkFormToken.
const formCookie = "caspian_form"

// minPanelPassword is the shortest panel password this build accepts.
//
// internal/state refuses only an empty one, deliberately: it stores what it is
// given and does not hold policy. The policy belongs here, where the screen is.
const minPanelPassword = 8

// maxLabel bounds the user's own name for a config. It is arbitrary user text
// that gets rendered, so it is escaped by html/template and bounded here.
const maxLabel = 64

// ---------------------------------------------------------------------------
// The dashboard
// ---------------------------------------------------------------------------

// handleIndex draws the main screen.
//
// The order down the page is fixed by the design and by the maintainer's
// dashboard brief, and each position has a reason:
//
//	the tile row      the few answers a person wants without scrolling
//	the switch        inside the first tile, so the control is never separated
//	                  from the state it reports and stays the most prominent
//	                  thing on the page
//	messages          whatever is wrong, immediately under the control
//	the hotspot       the QR, name and password: what gets a device connected
//	the config        the one control that changes what the box does
//	the detected line one line naming what was decided (design section 5.4)
//	events            what the box has been doing, in sentences
//	advanced          the link into everything else (design section 5.3)
func (p *Panel) handleIndex(w http.ResponseWriter, r *http.Request) {
	// "GET /" in a ServeMux is the catch-all, so anything not matched by
	// another pattern arrives here.
	if r.URL.Path != "/" {
		p.renderProblem(w, r, http.StatusNotFound, Problem{
			Headline: MsgNotFound, Advice: MsgNotFoundAdvice,
		})
		return
	}

	lang := langFrom(r)
	advanced := p.advancedFlag(w, r)
	sess := sessionFrom(r)

	data := p.newPageData(lang, MsgAppName, csrfFrom(r), advanced)
	data.SignedIn = true
	if sess != nil {
		prob, notice := sess.flash()
		data.setProblem(prob)
		if notice != "" {
			data.Notice = T(lang, notice)
		}
	}

	st := p.store.Snapshot()
	status, fault := p.status(r)
	p.events.observe(status, fault == FaultNone)

	data.SetupIncomplete = p.store.NeedsSetup()
	data.fillConfig(st.Proxy)
	data.fillStatus(status, fault)
	data.fillTiles(status, fault, p.now)
	data.fillHotspot(st.Hotspot)
	data.fillEvents(p.events.entries())
	// After the others, because it reads what is saved rather than what is
	// running: a hotspot that is switched off has no live name, and asking the
	// live one told somebody to choose a WiFi name they had already chosen.
	data.fillNextStep(st.Hotspot.SSID != "", status.ClientTrafficCut,
		status.Engine.Phase == engine.PhaseStarting, status.Hotspot.Devices)

	if advanced {
		ctx, cancel := p.privCtx(r)
		log, err := p.priv.EngineLog(ctx)
		cancel()
		if err != nil {
			p.log.Warn("engine log unavailable", "fault", string(FaultOf(err)))
		}
		data.fillAdvanced(st.Advanced, status.Detection, log)
	}

	p.render(w, http.StatusOK, "index", data)
}

// advancedFlag reads the advanced toggle, honouring ?advanced=1 or 0 and
// remembering the answer.
//
// A query parameter and a cookie rather than JavaScript state, so the toggle
// works with scripting off and the choice survives the redirect after a form
// post.
func (p *Panel) advancedFlag(w http.ResponseWriter, r *http.Request) bool {
	if v := r.URL.Query().Get("advanced"); v != "" {
		on := v == "1"
		value := "0"
		if on {
			value = "1"
		}
		http.SetCookie(w, &http.Cookie{
			Name:     advancedCookie,
			Value:    value,
			Path:     "/",
			HttpOnly: true,
			Secure:   p.secureCookies,
			SameSite: http.SameSiteStrictMode,
		})
		return on
	}
	c, err := r.Cookie(advancedCookie)
	return err == nil && c.Value == "1"
}

func (p *Panel) handleStatusJSON(w http.ResponseWriter, r *http.Request) {
	lang := langFrom(r)
	status, fault := p.status(r)
	p.events.observe(status, fault == FaultNone)

	body := p.newStatusJSON(lang, status, fault, p.store.Proxy().IsConfigured(),
		p.store.Snapshot().Hotspot.SSID != "", p.events.entries())

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		p.log.Warn("writing status json failed", "error", err.Error())
	}
}

// ---------------------------------------------------------------------------
// The switch
// ---------------------------------------------------------------------------

// handleCut stops and restarts forwarded client traffic without touching the
// hotspot.
//
// It is deliberately not part of handlePower. The switch answers "should this
// appliance be running"; this answers "should the devices on it reach the
// internet right now", and folding the second into the first would put an
// emergency control behind the same button that disconnects everybody.
//
// There is no confirmation step. This is the control somebody reaches for when
// they want traffic to stop this second, and a dialogue defeats it. What the
// page does instead is make the cut state unmistakable while it is in force.
// handleHelp serves the page that explains the appliance.
//
// It is a page rather than tooltips because the questions it answers are not
// about individual controls. Which adapter should carry the internet, what
// happens to a device when the tunnel drops, and what this box does not
// protect are all questions about the whole arrangement, and an answer split
// across six hover texts is one nobody reads.
//
// It is behind the session like everything else. Nothing on it is secret, but
// a page describing exactly what this machine is and how it is wired is not
// something to serve to whoever asks.
func (p *Panel) handleHelp(w http.ResponseWriter, r *http.Request) {
	data := p.newPageData(langFrom(r), MsgHelpTitle, csrfFrom(r), false)
	data.SignedIn = true
	if st, fault := p.status(r); fault == FaultNone {
		data.fillStatus(st, fault)
	}
	p.render(w, http.StatusOK, "help", data)
}

func (p *Panel) handleCut(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)
	cut := r.PostFormValue("cut") == "1"

	ctx, cancel := p.privCtx(r)
	defer cancel()

	var err error
	if cut {
		err = p.priv.Cut(ctx)
	} else {
		err = p.priv.Restore(ctx)
	}
	if err != nil {
		f := FaultOf(err)
		p.log.Warn("client traffic control failed", "cut", cut, "fault", string(f))
		sess.setFlash(StartProblem(f), "")
		p.home(w, r)
		return
	}

	if cut {
		p.log.Info("client traffic cut")
		p.events.add(EventTrafficCut, FaultNone)
		sess.setFlash(Problem{}, MsgNoticeCut)
	} else {
		p.log.Info("client traffic restored")
		p.events.add(EventTrafficRestored, FaultNone)
		sess.setFlash(Problem{}, MsgNoticeRestored)
	}
	p.home(w, r)
}

func (p *Panel) handlePower(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)
	on := r.PostFormValue("on") == "1"

	ctx, cancel := p.privCtx(r)
	defer cancel()

	if !on {
		if err := p.priv.Stop(ctx); err != nil {
			f := FaultOf(err)
			p.log.Warn("stop failed", "fault", string(f))
			sess.setFlash(StartProblem(f), "")
		} else {
			p.log.Info("switched off")
			p.events.add(EventSwitchedOff, FaultNone)
			sess.setFlash(Problem{}, MsgNoticeOff)
		}
		p.home(w, r)
		return
	}

	st := p.store.Snapshot()
	switch {
	case !st.Proxy.IsConfigured():
		sess.setFlash(Problem{Headline: MsgNoConfigYet, Advice: MsgNoConfigYetAdvice}, "")
	case st.Hotspot.SSID == "" || st.Hotspot.Passphrase.IsZero():
		sess.setFlash(Problem{Headline: MsgNoHotspotYet, Advice: MsgNoHotspotYetAdvice}, "")
	default:
		if prob := p.startNow(ctx, st); !prob.Empty() {
			sess.setFlash(prob, "")
		} else {
			p.log.Info("switched on", "config_fingerprint", st.Proxy.Fingerprint())
			p.events.add(EventSwitchedOn, FaultNone)
			sess.setFlash(Problem{}, MsgNoticeOn)
		}
	}
	p.home(w, r)
}

// handleRecover is the way out of a stuck box for somebody who has only the
// panel.
//
// It stops everything, replays the teardown journal so every change this
// appliance made to the machine is undone, and starts again from the saved
// settings. It is NOT a reboot and it does not restart the two services: the
// panel and any SSH session stay up throughout, because a control that takes
// away the page you pressed it on is no use to the person who needs it most.
func (p *Panel) handleRecover(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)

	st := p.store.Snapshot()

	ctx, cancel := p.privCtx(r)
	defer cancel()

	if prob := p.recoverNow(ctx, st); !prob.Empty() {
		p.log.Warn("recover failed")
		p.events.add(EventRecovered, FaultUnknown)
		sess.setFlash(prob, "")
		p.home(w, r)
		return
	}

	p.log.Info("recovered and started again")
	p.events.add(EventRecovered, FaultNone)
	sess.setFlash(Problem{}, MsgNoticeRecovered)
	p.home(w, r)
}

// startNow is the whole path from stored config to a running tunnel, and it is
// where two of the three config failure states are told apart.
//
// The order is fixed and each step answers a different question:
//
//  1. internal/link parses the stored text. A failure here is the first state,
//     "it did not parse", and the user has to fix the text.
//  2. internal/link re-serialises it into a config document. Nothing the user
//     typed is interpolated; design section 6.
//  3. internal/engine validates that document without starting anything. A
//     failure here is the second state, and doing it in the panel rather than
//     letting the privileged side discover it means the box does not begin
//     rewriting its routing table for a config that was never going to load.
//  4. the privileged service starts. FaultServerNoAnswer is the third state.
func (p *Panel) startNow(ctx context.Context, st state.State) Problem {
	return p.bringUp(ctx, st, p.priv.Start)
}

// recoverNow is startNow with the machine put back first.
//
// Same four steps, same request, and the privileged side stops everything and
// replays the teardown journal before it starts again. It exists because a
// start can leave the machine in a state only the journal knows how to undo,
// and until now the only way to apply that knowledge was an SSH session.
func (p *Panel) recoverNow(ctx context.Context, st state.State) Problem {
	return p.bringUp(ctx, st, p.priv.Recover)
}

// bringUp is everything both of them share: parse the stored config, compose
// the document, validate it without starting anything, then hand it over.
//
// The via function is the only difference between switching on and recovering,
// which is deliberate. A recovery that took a different path to the same state
// would be a second implementation of starting, and the two would drift.
func (p *Panel) bringUp(ctx context.Context, st state.State, via func(context.Context, StartRequest) error) Problem {
	l, err := link.Parse(st.Proxy.Raw.Reveal())
	if err != nil {
		p.log.Warn("stored config no longer parses", "config_fingerprint", st.Proxy.Fingerprint())
		return ParseProblem(err)
	}
	cfgJSON, err := l.XrayConfig()
	if err != nil {
		p.log.Warn("could not build a config document", "config_fingerprint", st.Proxy.Fingerprint())
		return EngineProblem()
	}
	if err := engine.Validate(cfgJSON); err != nil {
		// The fingerprint identifies which config failed without disclosing any
		// part of it (internal/state, ProxyConfig.Fingerprint). The engine's own
		// words are NOT logged: they are shown in advanced mode through
		// Problem.Detail, and this package's rule is that no log line carries
		// anything derived from the pasted config.
		p.log.Warn("engine refused the config", "config_fingerprint", st.Proxy.Fingerprint())
		prob := EngineProblem()
		prob.Detail = engineDetail(err)
		return prob
	}

	req := StartRequest{
		ConfigJSON: cfgJSON,
		Hotspot: HotspotSpec{
			SSID:       st.Hotspot.SSID,
			Passphrase: st.Hotspot.Passphrase.Reveal(),
			Interface:  st.Advanced.HotspotInterface,
			Channel:    st.Advanced.Channel,
			Band:       st.Advanced.Band,
			Country:    st.Advanced.Country,
			Subnet:     st.Advanced.Subnet,
		},
		Network: NetworkSpec{
			InternetInterface: st.Advanced.InternetInterface,
			DNSMode:           st.Advanced.DNSMode,
			OnTunnelDown:      st.Advanced.OnTunnelDown,
			// Copied, not defaulted. The privileged side refuses anything but
			// the blocking value and names the refusal, so substituting a safe
			// value here would hide the setting from the person who changed it
			// and turn that refusal into something they cannot act on.
			ClientIPv6: st.Advanced.ClientIPv6,
		},
		EngineLogLevel: st.Advanced.EngineLogLevel,
	}
	if err := via(ctx, req); err != nil {
		f := FaultOf(err)
		p.log.Warn("start failed", "fault", string(f), "config_fingerprint", st.Proxy.Fingerprint())
		p.events.add(EventStartFailed, f)
		return StartProblem(f)
	}
	return Problem{}
}

// engineDetail returns the engine's own message for advanced mode.
//
// internal/engine has already redacted it, and its Error type has no Unwrap so
// the unredacted cause cannot be recovered downstream. Redact is run again
// anyway: it is documented idempotent, it costs nothing on a clean string, and
// this is the one place where engine text reaches a rendered page.
func engineDetail(err error) string {
	if err == nil {
		return ""
	}
	return engine.Redact(err.Error())
}

// ---------------------------------------------------------------------------
// Adding a config
// ---------------------------------------------------------------------------

// handleConfig stores a pasted config.
//
// The rules this handler exists to keep, from design section 6 and section 9:
//
//   - the pasted text is never echoed into a response, on any path,
//   - it is never written to a log line, and neither is anything derived from
//     it except the fingerprint, which is a hash prefix by construction,
//   - it never appears in a URL: this is a POST, and the redirect afterwards
//     carries a message key through the session rather than a query parameter,
//   - it is parsed and re-serialised before it goes near the privileged process.
func (p *Panel) handleConfig(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)
	raw := strings.TrimSpace(r.PostFormValue("config"))
	label := strings.TrimSpace(r.PostFormValue("label"))
	if len(label) > maxLabel {
		label = label[:maxLabel]
	}
	replacing := p.store.Proxy().IsConfigured()

	l, err := link.Parse(raw)
	if err != nil {
		// Nothing about raw is logged, including its length: this is the path a
		// mistyped credential takes.
		p.log.Info("a pasted config could not be read")
		sess.setFlash(ParseProblem(err), "")
		p.home(w, r)
		return
	}
	cfgJSON, err := l.XrayConfig()
	if err != nil {
		sess.setFlash(EngineProblem(), "")
		p.home(w, r)
		return
	}
	if err := engine.Validate(cfgJSON); err != nil {
		prob := EngineProblem()
		prob.Detail = engineDetail(err)
		p.log.Info("the engine refused a pasted config")
		sess.setFlash(prob, "")
		p.home(w, r)
		return
	}

	if err := p.store.SetProxyConfig(raw, l.Protocol, label); err != nil {
		p.log.Error("saving the config failed", "error", err.Error())
		sess.setFlash(Problem{Headline: MsgSaveConfigFailed, Advice: MsgSaveFailedAdvice}, "")
		p.home(w, r)
		return
	}
	p.log.Info("config saved", "scheme", l.Protocol, "config_fingerprint", p.store.Proxy().Fingerprint())
	if replacing {
		p.events.add(EventConfigChanged, FaultNone)
	} else {
		p.events.add(EventConfigAdded, FaultNone)
	}

	// If the tunnel is up it is still using the old config, so it is replaced
	// rather than left running. Doing nothing here would leave the panel saying
	// one thing and the box doing another.
	status, fault := p.status(r)
	if fault == FaultNone && status.Engine.Phase == engine.PhaseRunning {
		ctx, cancel := p.privCtx(r)
		defer cancel()
		if err := p.priv.Stop(ctx); err != nil {
			sess.setFlash(StartProblem(FaultOf(err)), "")
			p.home(w, r)
			return
		}
		if prob := p.startNow(ctx, p.store.Snapshot()); !prob.Empty() {
			sess.setFlash(prob, "")
			p.home(w, r)
			return
		}
		sess.setFlash(Problem{}, MsgNoticeConfigReconn)
		p.home(w, r)
		return
	}
	sess.setFlash(Problem{}, MsgNoticeConfigSaved)
	p.home(w, r)
}

// ---------------------------------------------------------------------------
// Naming the hotspot
// ---------------------------------------------------------------------------

func (p *Panel) handleHotspot(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)
	ssid := strings.TrimSpace(r.PostFormValue("ssid"))
	// Not trimmed: a space is a legal character in a WPA passphrase.
	pass := r.PostFormValue("passphrase")

	if prob := ValidateSSID(ssid); !prob.Empty() {
		sess.setFlash(prob, "")
		p.home(w, r)
		return
	}
	if prob := ValidatePassphrase(pass); !prob.Empty() {
		sess.setFlash(prob, "")
		p.home(w, r)
		return
	}
	if err := p.store.SetHotspot(ssid, pass); err != nil {
		p.log.Error("saving the hotspot settings failed", "error", err.Error())
		sess.setFlash(Problem{Headline: MsgSaveHotspotFailed, Advice: MsgSaveFailedAdvice}, "")
		p.home(w, r)
		return
	}
	// The SSID is broadcast to anyone in range, so it is not a secret and is
	// logged. The passphrase is never logged, here or anywhere.
	p.log.Info("hotspot name and password set", "ssid", ssid)
	p.events.add(EventHotspotNamed, FaultNone)

	status, fault := p.status(r)
	if fault == FaultNone && status.Hotspot.Running {
		ctx, cancel := p.privCtx(r)
		defer cancel()
		if err := p.priv.Stop(ctx); err != nil {
			sess.setFlash(StartProblem(FaultOf(err)), "")
			p.home(w, r)
			return
		}
		if prob := p.startNow(ctx, p.store.Snapshot()); !prob.Empty() {
			sess.setFlash(prob, "")
			p.home(w, r)
			return
		}
		sess.setFlash(Problem{}, MsgNoticeHotspotRenamed)
		p.home(w, r)
		return
	}
	sess.setFlash(Problem{}, MsgNoticeHotspotSaved)
	p.home(w, r)
}

// ---------------------------------------------------------------------------
// Advanced mode overrides
// ---------------------------------------------------------------------------

// handleAdvanced saves the overrides design section 5.3 exposes.
//
// Every value is checked against what the privileged side reported, not merely
// for shape. An interface name is accepted only if detection listed it, and a
// hotspot interface only if detection said that radio can host an access point.
// That is not politeness towards the user: it is what keeps this form from
// being a way to hand an arbitrary string to the privileged process.
//
// The empty value means "stop overriding this and detect it again", which is
// the convention internal/state.Advanced already uses.
func (p *Panel) handleAdvanced(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)

	ctx, cancel := p.privCtx(r)
	det, err := p.priv.Detect(ctx)
	cancel()
	if err != nil {
		sess.setFlash(Problem{Headline: FaultOf(err).Key()}, "")
		p.home(w, r)
		return
	}

	internet := strings.TrimSpace(r.PostFormValue("internet_interface"))
	hotspotIf := strings.TrimSpace(r.PostFormValue("hotspot_interface"))
	band := strings.TrimSpace(r.PostFormValue("band"))
	country := strings.ToUpper(strings.TrimSpace(r.PostFormValue("country")))
	subnet := strings.TrimSpace(r.PostFormValue("subnet"))
	logLevel := strings.TrimSpace(r.PostFormValue("engine_log_level"))
	onLAN := r.PostFormValue("panel_on_lan") == "1"

	channel := 0
	if v := strings.TrimSpace(r.PostFormValue("channel")); v != "" {
		n, convErr := strconv.Atoi(v)
		if convErr != nil {
			sess.setFlash(Problem{Headline: MsgAdvChannelNaN}, "")
			p.home(w, r)
			return
		}
		channel = n
	}

	if prob := validateOverrides(det, internet, hotspotIf, channel, band, country, subnet, logLevel); !prob.Empty() {
		sess.setFlash(prob, "")
		p.home(w, r)
		return
	}

	err = p.store.Update(func(st *state.State) error {
		st.Advanced.InternetInterface = internet
		st.Advanced.HotspotInterface = hotspotIf
		st.Advanced.Channel = channel
		st.Advanced.Band = band
		st.Advanced.Country = country
		st.Advanced.Subnet = subnet
		st.Advanced.EngineLogLevel = logLevel
		st.Advanced.PanelOnLAN = onLAN
		return nil
	})
	if err != nil {
		p.log.Error("saving the advanced settings failed", "error", err.Error())
		sess.setFlash(Problem{Headline: MsgSaveAdvancedFailed, Advice: MsgSaveFailedAdvice}, "")
		p.home(w, r)
		return
	}
	p.log.Info("advanced settings saved")
	p.events.add(EventAdvancedSaved, FaultNone)
	sess.setFlash(Problem{}, MsgNoticeAdvancedSaved)
	p.home(w, r)
}

func validateOverrides(det Detection, internet, hotspotIf string, channel int, band, country, subnet, logLevel string) Problem {
	known := func(name string) bool {
		for _, i := range det.Interfaces {
			if i.Name == name {
				return true
			}
		}
		return false
	}
	apCapable := func(name string) bool {
		for _, i := range det.Interfaces {
			if i.Name == name {
				return i.CanHostAP
			}
		}
		return false
	}

	if internet != "" && !known(internet) {
		return Problem{Headline: MsgAdvBadInternet, Advice: MsgAdvBadInternetAdvice}
	}
	if hotspotIf != "" {
		if !known(hotspotIf) {
			return Problem{Headline: MsgAdvBadHotspot, Advice: MsgAdvBadHotspotAdvice}
		}
		if !apCapable(hotspotIf) {
			return Problem{Headline: MsgAdvNoAP, Advice: MsgAdvNoAPAdvice}
		}
	}
	if channel != 0 {
		ok := false
		for _, c := range det.UsableChannels {
			if c == channel {
				ok = true
				break
			}
		}
		if !ok {
			return Problem{Headline: MsgAdvBadChannel, Advice: MsgAdvBadChannelAdvice}
		}
	}
	if !ValidBand(band) {
		return Problem{Headline: MsgAdvBadBand, Advice: MsgAdvBadBandAdvice}
	}
	if country != "" && !isCountryCode(country) {
		return Problem{Headline: MsgAdvBadCountry, Advice: MsgAdvBadCountryAdvice}
	}
	if subnet != "" {
		prefix, err := netip.ParsePrefix(subnet)
		if err != nil {
			return Problem{Headline: MsgAdvBadSubnet, Advice: MsgAdvBadSubnetAdvice}
		}
		if !prefix.Addr().Is4() {
			return Problem{Headline: MsgAdvSubnetV4, Advice: MsgAdvSubnetV4Advice}
		}
		if prefix.Bits() > 30 {
			return Problem{Headline: MsgAdvSubnetSmall, Advice: MsgAdvSubnetSmallAdvice}
		}
	}
	if !ValidEngineLogLevel(logLevel) {
		return Problem{Headline: MsgAdvBadLogLevel, Advice: MsgAdvBadLogLevelAdvice}
	}
	return Problem{}
}

func isCountryCode(s string) bool {
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
// Setup, the first screen on a box with no password
// ---------------------------------------------------------------------------

func (p *Panel) handleSetupForm(w http.ResponseWriter, r *http.Request) {
	if p.store.Snapshot().Panel.IsSet() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	token, err := p.issueFormToken(w)
	if err != nil {
		p.internalError(w, r, "issuing a form token", err)
		return
	}
	data := p.newPageData(langFrom(r), MsgSetupTitle, token, false)
	data.Notice = T(data.Lang, MsgSetupNotice)
	p.render(w, http.StatusOK, "setup", data)
}

func (p *Panel) handleSetup(w http.ResponseWriter, r *http.Request) {
	// The gate. Without it, anyone who can reach the panel could set the
	// password on a box that already has one, which is not a setup screen; it
	// is a takeover.
	if p.store.Snapshot().Panel.IsSet() {
		p.renderProblem(w, r, http.StatusForbidden, Problem{
			Headline: MsgSetupDone, Advice: MsgSetupDoneAdvice,
		})
		return
	}
	if !p.checkFormToken(r) {
		p.renderProblem(w, r, http.StatusForbidden, Problem{
			Headline: MsgBadForm, Advice: MsgBadFormAdvice,
		})
		return
	}

	pw := r.PostFormValue("password")
	confirm := r.PostFormValue("confirm")
	fail := func(prob Problem) {
		data := p.newPageData(langFrom(r), MsgSetupTitle, "", false)
		if token, err := p.issueFormToken(w); err == nil {
			data.CSRF = token
		}
		data.setProblem(prob)
		p.render(w, http.StatusBadRequest, "setup", data)
	}
	if len([]rune(pw)) < minPanelPassword {
		fail(Problem{Headline: MsgSetupShort, Advice: MsgSetupShortAdvice})
		return
	}
	// Compared in constant time even though both sides came from the same form
	// and neither is stored. It costs nothing, and a plain != on secret
	// material is a habit worth not forming in a file that handles passwords.
	if subtle.ConstantTimeCompare([]byte(pw), []byte(confirm)) != 1 {
		fail(Problem{Headline: MsgSetupMismatch, Advice: MsgSetupMismatchAdvice})
		return
	}

	if err := p.store.SetPanelPassword(pw); err != nil {
		p.internalError(w, r, "setting the panel password", err)
		return
	}
	p.log.Info("panel password set for the first time")
	p.startSession(w, r)
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func (p *Panel) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if !p.store.Snapshot().Panel.IsSet() {
		// A box with no password shows setup, never a login form nobody can
		// get past.
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if _, ok := p.currentSession(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	token, err := p.issueFormToken(w)
	if err != nil {
		p.internalError(w, r, "issuing a form token", err)
		return
	}
	p.render(w, http.StatusOK, "login", p.newPageData(langFrom(r), MsgLoginTitle, token, false))
}

func (p *Panel) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !p.store.Snapshot().Panel.IsSet() {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if !p.checkFormToken(r) {
		p.renderProblem(w, r, http.StatusForbidden, Problem{
			Headline: MsgBadForm, Advice: MsgBadFormAdvice,
		})
		return
	}

	lang := langFrom(r)
	key := clientKey(r)
	failPage := func(status int, prob Problem) {
		data := p.newPageData(lang, MsgLoginTitle, "", false)
		if token, err := p.issueFormToken(w); err == nil {
			data.CSRF = token
		}
		data.setProblem(prob)
		p.render(w, status, "login", data)
	}

	if ok, retry := p.limiter.allow(key); !ok {
		// Retry-After is in seconds and is what a browser and a person both
		// understand. The message says how long rather than "try later",
		// because "later" gets retried immediately and makes it worse.
		seconds := int(retry.Seconds()) + 1
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		p.log.Warn("password attempts rate limited", "client", key)
		failPage(http.StatusTooManyRequests, Problem{
			Headline:   MsgLoginTooMany,
			Advice:     MsgLoginTooManyWait,
			AdviceArgs: []any{humanSeconds(lang, seconds)},
		})
		return
	}

	ok, err := p.store.VerifyPanelPassword(r.PostFormValue("password"))
	switch {
	case errors.Is(err, state.ErrNoPanelPassword):
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	case err != nil:
		// The stored verifier could not be read. That is a corrupt-state
		// condition, not a wrong password, and saying so stops the user
		// retyping a password that was never going to be checked.
		p.log.Error("the stored panel password could not be read", "error", err.Error())
		p.renderProblem(w, r, http.StatusInternalServerError, Problem{
			Headline: MsgLoginCorrupt, Advice: MsgLoginCorruptAdvice,
		})
		return
	case !ok:
		// Both, and they are not the same audience. The log line carries the
		// client for somebody reading the journal; the event is what the owner
		// of the box sees on the page, and until now a wrong password was
		// invisible there. An appliance that can be reached by anyone within
		// radio range should say when somebody has tried.
		//
		// The event carries no client and no attempted password. It is a
		// closed vocabulary value and a timestamp, like every other event, so
		// there is nowhere for either to be written even by accident.
		p.log.Warn("wrong panel password", "client", key)
		p.events.add(EventWrongPassword, FaultNone)
		failPage(http.StatusUnauthorized, Problem{
			Headline: MsgLoginWrong, Advice: MsgLoginWrongAdvice,
		})
		return
	}

	p.limiter.succeed(key)
	p.log.Info("signed in", "client", key)
	p.events.add(EventSignedIn, FaultNone)
	p.startSession(w, r)
}

func (p *Panel) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s := sessionFrom(r); s != nil {
		// The message a sticky flash is holding may describe the user's config,
		// so it does not outlive the session that was allowed to see it.
		s.clearFlash()
	}
	if c, err := r.Cookie(p.cookieName); err == nil {
		p.sessions.destroy(c.Value)
	}
	http.SetCookie(w, clearSessionCookie(p.cookieName, p.secureCookies))
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// startSession issues a session cookie and sends the browser to the dashboard.
func (p *Panel) startSession(w http.ResponseWriter, r *http.Request) {
	token, _, err := p.sessions.create()
	if err != nil {
		p.internalError(w, r, "creating a session", err)
		return
	}
	http.SetCookie(w, newSessionCookie(p.cookieName, token, p.secureCookies))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// The token on the two forms that exist before there is a session
// ---------------------------------------------------------------------------

// issueFormToken sets a token cookie and returns the value to embed in the form.
//
// The login and setup forms cannot use the per-session token, because their
// whole purpose is that there is no session yet. Without something in its place,
// a page on another site could post to /setup on a box that has not been set up
// and choose the panel password. That is not theoretical on this product: the
// box is on a network the user is browsing from, its address is predictable, and
// a form post is not subject to a preflight.
//
// So: a random token in a SameSite=Strict cookie, and the same value in a hidden
// field. A cross-site post carries neither. The Origin and Sec-Fetch-Site check
// in ServeHTTP is the second lock on the same door.
func (p *Panel) issueFormToken(w http.ResponseWriter) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     formCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   p.secureCookies,
		SameSite: http.SameSiteStrictMode,
	})
	return token, nil
}

func (p *Panel) checkFormToken(r *http.Request) bool {
	c, err := r.Cookie(formCookie)
	if err != nil || c.Value == "" {
		return false
	}
	got := r.PostFormValue("csrf")
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(got)) == 1
}

// ---------------------------------------------------------------------------
// Small shared pieces
// ---------------------------------------------------------------------------

// home sends the browser back to the dashboard after a form post, so a reload
// does not repeat the post.
func (p *Panel) home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (p *Panel) renderProblem(w http.ResponseWriter, r *http.Request, status int, prob Problem) {
	data := p.newPageData(langFrom(r), MsgAppName, csrfFrom(r), false)
	data.setProblem(prob)
	p.render(w, status, "problem", data)
}

// internalError logs the cause and shows the user a sentence with no internals
// in it.
//
// The error is deliberately not rendered. An error page is a document people
// screenshot and send to somebody for help, so nothing that has been anywhere
// near a credential goes on one.
func (p *Panel) internalError(w http.ResponseWriter, r *http.Request, doing string, err error) {
	p.log.Error("panel error", "doing", doing, "error", err.Error())
	p.renderProblem(w, r, http.StatusInternalServerError, Problem{
		Headline: MsgInternalError, Advice: MsgInternalAdvice,
	})
}

// sameOriginPost reports whether an unsafe request came from this panel's own
// pages.
//
// Two independent signals, either of which is enough to refuse:
//
//   - Sec-Fetch-Site, which every current browser sends and which says
//     cross-site for a form posted from another origin. It is absent on older
//     browsers, so its absence cannot be treated as a refusal.
//   - Origin, which is sent on every form post. When present it must match the
//     host the request came to.
//
// This is defence in depth behind the token checks, not a replacement for them.
func sameOriginPost(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" || origin == "null" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}
