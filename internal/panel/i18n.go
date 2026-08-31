// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"fmt"
	"net/http"
	"sort"
)

// The panel is in Persian and English, and Persian is what a fresh box shows.
//
// That ordering is the requirement, not a preference. The people this appliance
// is for read Persian, and an English default makes them do work before they
// can do anything at all. So Persian is not a translation laid over an English
// product; it is the product, and English is the alternative.
//
// Three consequences that are easy to get wrong and are handled here rather
// than left to whoever adds the next string:
//
//   - Persian is right-to-left, so the DOCUMENT direction changes. That is the
//     dir attribute on the root element plus a stylesheet written in logical
//     properties, not a second stylesheet with left and right swapped. See
//     assets/panel.css and TestStylesheetUsesLogicalProperties.
//   - Some values must stay left-to-right inside right-to-left text: addresses,
//     subnets, ports, interface names, the WiFi passphrase. Without isolation
//     their parts are reordered on screen, and an address that displays wrongly
//     is worse than one not displayed at all, because somebody will act on it.
//     See the LTR type in view.go.
//   - Nothing is fetched to make this work. No web font, no locale data, no
//     script. Design section 5.7 does not bend for a typeface.
//
// # Numerals: Latin digits in both languages
//
// Decided here rather than left implicit, because it is the kind of choice that
// otherwise gets made differently in three places.
//
// Persian has its own digits and Persian prose normally uses them. This panel
// uses Latin digits anyway, in both languages, with no exceptions. Almost every
// number on this screen is a thing the user has to COMPARE against something
// outside this panel: an address against their router's page, a channel against
// another tool, a device count against the phones in the room, a port against
// what somebody told them. All of those show Latin digits. A panel that shows
// ۱۹۲.۱۶۸.۱.۴۲ where the router shows 192.168.1.42 has made the comparison
// harder in exactly the situation where somebody is already confused.
//
// Applying it to prose numbers as well ("8 characters") and not only to
// addresses is deliberate: a rule with an exception list is a rule that gets
// applied differently by the next person. The cost is that the Persian reads
// slightly less naturally, and that is the smaller cost.
//
// The maintainer proposed this and the reasoning above is agreement, not
// deference: it was checked against what is actually on the screen.
//
// # A caveat that belongs in the open
//
// The Persian below is this author's own and has not been reviewed by a native
// speaker. For headings that is a small risk. For the fault sentences it is
// not: they are what a non-technical person reads when something is broken, and
// a wrong one sends them to fix the wrong thing. They should be reviewed before
// this ships to anybody.

// Lang is a language the panel is served in.
type Lang string

const (
	// LangFA is Persian, and the default.
	LangFA Lang = "fa"
	// LangEN is English.
	LangEN Lang = "en"
)

// DefaultLang is what a browser that has never chosen gets.
const DefaultLang = LangFA

// Langs is every language, in the order the switcher offers them.
var Langs = []Lang{LangFA, LangEN}

// langCookie remembers the choice.
//
// Per browser, not per box, and deliberately NOT in the state file. Two people
// using the same appliance must not fight over the language, and a language
// preference is not worth a privileged write to a file that also holds the
// user's proxy config.
const langCookie = "caspian_lang"

// Valid reports whether l is a language this build serves.
func (l Lang) Valid() bool {
	for _, k := range Langs {
		if k == l {
			return true
		}
	}
	return false
}

// Dir is the document direction for the html element.
func (l Lang) Dir() string {
	if l == LangFA {
		return "rtl"
	}
	return "ltr"
}

// RTL reports whether the language reads right to left.
func (l Lang) RTL() bool { return l.Dir() == "rtl" }

// Other is the language the switcher offers, which with two languages is
// simply the one you are not in.
func (l Lang) Other() Lang {
	if l == LangFA {
		return LangEN
	}
	return LangFA
}

// Key names one message. Every user-facing string in this package is one of
// these; see TestNoInlineUserFacingStringsInGoSource, which fails on a literal
// written into a Problem instead.
type Key string

// missingMarker is what T returns for a key with no message.
//
// It is deliberately loud and deliberately not the key on its own. A missing
// message that falls back to the English text, or to the key, is a missing
// message nobody notices; this one shows up in a rendered page and
// TestNoRenderedPageHasAMissingMessage fails on it.
const missingMarker = "!!"

// T returns the message for a key in a language.
func T(l Lang, k Key, args ...any) string {
	if !l.Valid() {
		l = DefaultLang
	}
	s, ok := messages[l][k]
	if !ok {
		// Not falling back to the other language on purpose: a page that is
		// half Persian and half English looks like a bug to the user and looks
		// like success to the tests.
		return missingMarker + string(k) + missingMarker
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

// keys returns every key in the catalogue, sorted. Used by the parity tests.
func keys(l Lang) []Key {
	out := make([]Key, 0, len(messages[l]))
	for k := range messages[l] {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ---------------------------------------------------------------------------
// Choosing the language for a request
// ---------------------------------------------------------------------------

// langFor decides which language to serve and records a change.
//
// A ?lang= parameter switches and is remembered; otherwise the cookie decides;
// otherwise Persian. An unknown value is ignored rather than being an error:
// the worst outcome of a bad language parameter should be the default language,
// not a page the user cannot read.
//
// Accept-Language is deliberately not consulted. The default is Persian
// because of who this is for, and a browser that happens to be configured for
// English would otherwise override that for exactly the user the default
// exists to serve.
func (p *Panel) langFor(w http.ResponseWriter, r *http.Request) Lang {
	if v := Lang(r.URL.Query().Get("lang")); v.Valid() {
		http.SetCookie(w, &http.Cookie{
			Name:     langCookie,
			Value:    string(v),
			Path:     "/",
			HttpOnly: true,
			Secure:   p.secureCookies,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   int((365 * 24 * 60 * 60)),
		})
		return v
	}
	if c, err := r.Cookie(langCookie); err == nil {
		if v := Lang(c.Value); v.Valid() {
			return v
		}
	}
	return DefaultLang
}

// ---------------------------------------------------------------------------
// The keys
//
// Grouped by where they appear. Every one has to exist in both maps below;
// TestEveryMessageExistsInBothLanguages fails on a key present in one and
// missing from the other, in either direction.
// ---------------------------------------------------------------------------

const (
	// Chrome: the bits around every page.
	MsgAppName       Key = "app.name"
	MsgSkipToMain    Key = "nav.skip"
	MsgSignOut       Key = "nav.signout"
	MsgFooterNote    Key = "footer.note"
	MsgOtherLanguage Key = "lang.other"
	MsgBackToMain    Key = "nav.back"

	// The status line and the switch.
	MsgStatusHeading    Key = "status.heading"
	MsgStatusConnected  Key = "status.connected"
	MsgStatusNotConn    Key = "status.notconnected"
	MsgStatusOff        Key = "status.off"
	MsgStatusStarting   Key = "status.starting"
	MsgStatusNotWorking Key = "status.notworking"
	MsgStatusNoConfig   Key = "status.noconfig"
	MsgStatusUsing      Key = "status.using"
	MsgStatusUnreadable Key = "status.unreadable"
	MsgSwitchOn         Key = "power.on"
	MsgSwitchOff        Key = "power.off"

	// The hotspot.
	MsgWifiHeading     Key = "wifi.heading"
	MsgWifiQRCaption   Key = "wifi.qr.caption"
	MsgWifiQRFailed    Key = "wifi.qr.failed"
	MsgWifiNetworkName Key = "wifi.name"
	MsgWifiPassword    Key = "wifi.password"
	MsgWifiIntro       Key = "wifi.intro"
	MsgWifiChange      Key = "wifi.change"
	MsgWifiFormName    Key = "wifi.form.name"
	MsgWifiFormPass    Key = "wifi.form.password"
	MsgWifiFormHint    Key = "wifi.form.hint"
	MsgWifiFormSave    Key = "wifi.form.save"

	// Devices.
	MsgDevicesNone       Key = "devices.none"
	MsgDevicesOne        Key = "devices.one"
	MsgDevicesMany       Key = "devices.many"
	MsgDevicesUnreadable Key = "devices.unreadable"

	// The config control.
	MsgConfigHeading     Key = "config.heading"
	MsgConfigNone        Key = "config.none"
	MsgConfigAdd         Key = "config.add"
	MsgConfigReplace     Key = "config.replace"
	MsgConfigPasteLabel  Key = "config.paste.label"
	MsgConfigPasteHint   Key = "config.paste.hint"
	MsgConfigPlaceholder Key = "config.placeholder"
	MsgConfigNameLabel   Key = "config.name.label"
	MsgConfigSubmit      Key = "config.submit"

	// The detected line.
	MsgDetectedLine    Key = "detected.line"
	MsgDetectedNone    Key = "detected.notfound"
	MsgKindEthernet    Key = "kind.ethernet"
	MsgKindBuiltinWiFi Key = "kind.builtinwifi"
	MsgKindUSBWiFi     Key = "kind.usbwifi"
	MsgKindWiFi        Key = "kind.wifi"

	// Advanced mode.
	MsgAdvancedShow       Key = "advanced.show"
	MsgAdvancedHide       Key = "advanced.hide"
	MsgAdvancedHeading    Key = "advanced.heading"
	MsgAdvancedHint       Key = "advanced.hint"
	MsgAdvInternet        Key = "advanced.internet"
	MsgAdvHotspotIface    Key = "advanced.hotspotiface"
	MsgAdvChannel         Key = "advanced.channel"
	MsgAdvBand            Key = "advanced.band"
	MsgAdvCountry         Key = "advanced.country"
	MsgAdvCountryHint     Key = "advanced.country.hint"
	MsgAdvSubnet          Key = "advanced.subnet"
	MsgAdvSubnetHint      Key = "advanced.subnet.hint"
	MsgAdvLogLevel        Key = "advanced.loglevel"
	MsgAdvLogLevelHint    Key = "advanced.loglevel.hint"
	MsgAdvPanelOnLAN      Key = "advanced.panelonlan"
	MsgAdvPanelOnLANHint  Key = "advanced.panelonlan.hint"
	MsgAdvSave            Key = "advanced.save"
	MsgAdvAuto            Key = "advanced.auto"
	MsgAdvAutoPlain       Key = "advanced.auto.plain"
	MsgAdvChannelPinned   Key = "advanced.channelpinned"
	MsgAdvCannotHostAP    Key = "advanced.cannothost"
	MsgAdvInUse           Key = "advanced.inuse"
	MsgAdvFixedHeading    Key = "advanced.fixed.heading"
	MsgAdvDNSLabel        Key = "advanced.dns.label"
	MsgAdvDNSValue        Key = "advanced.dns.value"
	MsgAdvDropLabel       Key = "advanced.drop.label"
	MsgAdvDropValue       Key = "advanced.drop.value"
	MsgAdvConfigHeading   Key = "advanced.config.heading"
	MsgAdvConfigKind      Key = "advanced.config.kind"
	MsgAdvConfigServer    Key = "advanced.config.server"
	MsgAdvConfigTransport Key = "advanced.config.transport"
	MsgAdvConfigSecurity  Key = "advanced.config.security"
	MsgAdvConfigSNI       Key = "advanced.config.sni"
	MsgAdvConfigFP        Key = "advanced.config.fingerprint"
	MsgAdvConfigFlow      Key = "advanced.config.flow"
	MsgAdvRealityKey      Key = "advanced.reality.publickey"
	MsgAdvRealityShortID  Key = "advanced.reality.shortid"
	MsgAdvRealityPQV      Key = "advanced.reality.pqv"
	MsgAdvConfigCount     Key = "advanced.config.count"
	MsgAdvPresent         Key = "advanced.present"
	MsgAdvAbsent          Key = "advanced.absent"
	MsgAdvNotSet          Key = "advanced.notset"
	MsgAdvNoSecrets       Key = "advanced.nosecrets"
	MsgAdvLogHeading      Key = "advanced.log.heading"
	MsgAdvLogPhase        Key = "advanced.log.phase"
	MsgAdvLogReason       Key = "advanced.log.reason"
	MsgAdvLogDropped      Key = "advanced.log.dropped"
	MsgAdvLogEmpty        Key = "advanced.log.empty"
	MsgAdvBand24          Key = "advanced.band.24"
	MsgAdvBand5           Key = "advanced.band.5"
	MsgPhaseStopped       Key = "phase.stopped"
	MsgPhaseStarting      Key = "phase.starting"
	MsgPhaseRunning       Key = "phase.running"
	MsgPhaseFailed        Key = "phase.failed"

	// Sign in.
	MsgLoginTitle         Key = "login.title"
	MsgLoginHeading       Key = "login.heading"
	MsgLoginPassword      Key = "login.password"
	MsgLoginSubmit        Key = "login.submit"
	MsgLoginHint          Key = "login.hint"
	MsgLoginWrong         Key = "login.wrong.headline"
	MsgLoginWrongAdvice   Key = "login.wrong.advice"
	MsgLoginTooMany       Key = "login.toomany.headline"
	MsgLoginTooManyWait   Key = "login.toomany.advice"
	MsgLoginCorrupt       Key = "login.corrupt.headline"
	MsgLoginCorruptAdvice Key = "login.corrupt.advice"

	// Setup.
	MsgSetupTitle          Key = "setup.title"
	MsgSetupHeading        Key = "setup.heading"
	MsgSetupNotice         Key = "setup.notice"
	MsgSetupIntro          Key = "setup.intro"
	MsgSetupPassword       Key = "setup.password"
	MsgSetupConfirm        Key = "setup.confirm"
	MsgSetupHint           Key = "setup.hint"
	MsgSetupSubmit         Key = "setup.submit"
	MsgSetupWriteItDown    Key = "setup.writeitdown"
	MsgSetupShort          Key = "setup.short.headline"
	MsgSetupShortAdvice    Key = "setup.short.advice"
	MsgSetupMismatch       Key = "setup.mismatch.headline"
	MsgSetupMismatchAdvice Key = "setup.mismatch.advice"
	MsgSetupDone           Key = "setup.done.headline"
	MsgSetupDoneAdvice     Key = "setup.done.advice"

	// Time, for the rate limiter's message.
	MsgSeconds Key = "time.seconds"
	MsgMinute  Key = "time.minute"
	MsgMinutes Key = "time.minutes"

	// Whole-page problems.
	MsgNotFound        Key = "problem.notfound.headline"
	MsgNotFoundAdvice  Key = "problem.notfound.advice"
	MsgBadForm         Key = "problem.badform.headline"
	MsgBadFormAdvice   Key = "problem.badform.advice"
	MsgCrossSite       Key = "problem.crosssite.headline"
	MsgCrossSiteAdvice Key = "problem.crosssite.advice"
	MsgInternalError   Key = "problem.internal.headline"
	MsgInternalAdvice  Key = "problem.internal.advice"
	MsgProblemDetail   Key = "problem.detail.label"

	// The three config failure states. These three headlines are the ones that
	// have to stay distinguishable in both languages.
	MsgParseHeadline  Key = "problem.parse.headline"
	MsgEngineHeadline Key = "problem.engine.headline"
	MsgEngineAdvice   Key = "problem.engine.advice"
	MsgServerHeadline Key = "problem.server.headline"
	MsgServerAdvice   Key = "problem.server.advice"

	// Why a link would not parse.
	MsgParseEmpty       Key = "problem.parse.empty.headline"
	MsgParseEmptyAdvice Key = "problem.parse.empty.advice"
	MsgParseScheme      Key = "problem.parse.scheme.advice"
	MsgParseNoLink      Key = "problem.parse.nolink.advice"
	MsgParseUUID        Key = "problem.parse.uuid.advice"
	MsgParseAddress     Key = "problem.parse.address.advice"
	MsgParsePort        Key = "problem.parse.port.advice"
	MsgParseReality     Key = "problem.parse.reality.advice"
	MsgParseTransport   Key = "problem.parse.transport.advice"
	MsgParseOther       Key = "problem.parse.other.advice"

	// Faults from the privileged side.
	MsgFaultNoAPAdapter           Key = "fault.noapadapter"
	MsgFaultIfaceBusy             Key = "fault.interfacebusy"
	MsgFaultNotRunning            Key = "fault.notrunning"
	MsgEventTrafficCut            Key = "event.trafficcut"
	MsgEventTrafficRestored       Key = "event.trafficrestored"
	MsgEventWrongPassword         Key = "event.wrongpassword"
	MsgNoticeCut                  Key = "notice.cut"
	MsgNoticeRestored             Key = "notice.restored"
	MsgNoticeRecovered            Key = "notice.recovered"
	MsgNavOutside                 Key = "nav.outside"
	MsgAdvRecoverHeading          Key = "advanced.recover.heading"
	MsgAdvRecoverHint             Key = "advanced.recover.hint"
	MsgAdvRecoverButton           Key = "advanced.recover.button"
	MsgCutButton                  Key = "cut.button"
	MsgCutRestoreButton           Key = "cut.restore"
	MsgCutBanner                  Key = "cut.banner"
	MsgCutSwitchLabel             Key = "cut.switchlabel"
	MsgPowerCaption               Key = "power.caption"
	MsgCutCaption                 Key = "cut.caption"
	MsgStatusCut                  Key = "status.cut"
	MsgNextAddconfig              Key = "next.addconfig"
	MsgNextSetwifi                Key = "next.setwifi"
	MsgNextSwitchon               Key = "next.switchon"
	MsgNextJoin                   Key = "next.join"
	MsgNextJoined                 Key = "next.joined"
	MsgNextResume                 Key = "next.resume"
	MsgNextProblem                Key = "next.problem"
	MsgNextStarting               Key = "next.starting"
	MsgNextLabel                  Key = "next.label"
	MsgHelpHeading                Key = "help.heading"
	MsgHelpTitle                  Key = "help.title"
	MsgHelpWhatHeading            Key = "help.what.heading"
	MsgHelpWhatBody               Key = "help.what.body"
	MsgHelpFlowCaption            Key = "help.flow.caption"
	MsgHelpFlowDevice             Key = "help.flow.device"
	MsgHelpFlowBox                Key = "help.flow.box"
	MsgHelpFlowTunnel             Key = "help.flow.tunnel"
	MsgHelpFlowServer             Key = "help.flow.server"
	MsgHelpFlowNote               Key = "help.flow.note"
	MsgHelpArrangeHeading         Key = "help.arrange.heading"
	MsgHelpArrangeBody            Key = "help.arrange.body"
	MsgHelpArrangeAHeading        Key = "help.arrange.a.heading"
	MsgHelpArrangeATop            Key = "help.arrange.a.top"
	MsgHelpArrangeABottom         Key = "help.arrange.a.bottom"
	MsgHelpArrangeAWhen           Key = "help.arrange.a.when"
	MsgHelpArrangeBHeading        Key = "help.arrange.b.heading"
	MsgHelpArrangeBTop            Key = "help.arrange.b.top"
	MsgHelpArrangeBBottom         Key = "help.arrange.b.bottom"
	MsgHelpArrangeBWhen           Key = "help.arrange.b.when"
	MsgHelpArrangeNounplug        Key = "help.arrange.nounplug"
	MsgHelpTwoHeading             Key = "help.two.heading"
	MsgHelpTwoIntro               Key = "help.two.intro"
	MsgHelpTwoPowerHeading        Key = "help.two.power.heading"
	MsgHelpTwoPowerBody           Key = "help.two.power.body"
	MsgHelpTwoCutHeading          Key = "help.two.cut.heading"
	MsgHelpTwoCutBody             Key = "help.two.cut.body"
	MsgHelpTwoLockout             Key = "help.two.lockout"
	MsgHelpTwoWhich               Key = "help.two.which"
	MsgHelpTwoRestart             Key = "help.two.restart"
	MsgHelpControlsHeading        Key = "help.controls.heading"
	MsgHelpControlsSwitch         Key = "help.controls.switch"
	MsgHelpControlsCut            Key = "help.controls.cut"
	MsgHelpControlsConfig         Key = "help.controls.config"
	MsgHelpControlsWifi           Key = "help.controls.wifi"
	MsgHelpControlsUplink         Key = "help.controls.uplink"
	MsgHelpControlsHotspotiface   Key = "help.controls.hotspotiface"
	MsgHelpControlsChannel        Key = "help.controls.channel"
	MsgHelpControlsCountry        Key = "help.controls.country"
	MsgHelpControlsSubnet         Key = "help.controls.subnet"
	MsgHelpControlsPanelonlan     Key = "help.controls.panelonlan"
	MsgHelpSecurityHeading        Key = "help.security.heading"
	MsgHelpSecurityDoesHeading    Key = "help.security.does.heading"
	MsgHelpSecurityDoesFailclosed Key = "help.security.does.failclosed"
	MsgHelpSecurityDoesDns        Key = "help.security.does.dns"
	MsgHelpSecurityDoesEgress     Key = "help.security.does.egress"
	MsgHelpSecurityDoesIsolation  Key = "help.security.does.isolation"
	MsgHelpSecurityDoesSecrets    Key = "help.security.does.secrets"
	MsgHelpSecurityDoesGiveback   Key = "help.security.does.giveback"
	MsgHelpSecurityNotHeading     Key = "help.security.not.heading"
	MsgHelpSecurityNotIntro       Key = "help.security.not.intro"
	MsgHelpSecurityNotDoh         Key = "help.security.not.doh"
	MsgHelpSecurityNotIpv6        Key = "help.security.not.ipv6"
	MsgHelpSecurityNotOwnbox      Key = "help.security.not.ownbox"
	MsgHelpSecurityNotEndpoints   Key = "help.security.not.endpoints"
	MsgHelpSecurityNotWifi        Key = "help.security.not.wifi"
	MsgHelpTroubleHeading         Key = "help.trouble.heading"
	MsgHelpTroubleNossidQ         Key = "help.trouble.nossid.q"
	MsgHelpTroubleNossidA         Key = "help.trouble.nossid.a"
	MsgHelpTroubleNointernetQ     Key = "help.trouble.nointernet.q"
	MsgHelpTroubleNointernetA     Key = "help.trouble.nointernet.a"
	MsgHelpTroubleSlowQ           Key = "help.trouble.slow.q"
	MsgHelpTroubleSlowA           Key = "help.trouble.slow.a"
	MsgHelpTroubleLostpanelQ      Key = "help.trouble.lostpanel.q"
	MsgHelpTroubleLostpanelA      Key = "help.trouble.lostpanel.a"
	MsgHelpTroubleForgotQ         Key = "help.trouble.forgot.q"
	MsgHelpTroubleForgotA         Key = "help.trouble.forgot.a"
	MsgFaultRadioBlocked          Key = "fault.radioblocked"
	MsgFaultNoInternet            Key = "fault.nointernet"
	MsgFaultChannel               Key = "fault.channelrefused"
	MsgFaultHotspot               Key = "fault.hotspotfailed"
	MsgFaultDHCP                  Key = "fault.dhcpfailed"
	MsgFaultClock                 Key = "fault.clock"
	MsgFaultPermission            Key = "fault.permission"
	MsgFaultSoftware              Key = "fault.softwaremissing"
	MsgFaultUnavailable           Key = "fault.unavailable"
	MsgFaultIPv6Unsupported       Key = "fault.ipv6unsupported"
	MsgFaultUnknown               Key = "fault.unknown"
	MsgFaultUnrecognised          Key = "fault.unrecognised"
	MsgTunnelNoHotspot            Key = "fault.tunnelnohotspot"

	// Confirmations.
	MsgNoticeOff            Key = "notice.off"
	MsgNoticeOn             Key = "notice.on"
	MsgNoticeConfigSaved    Key = "notice.configsaved"
	MsgNoticeConfigReconn   Key = "notice.configreconnected"
	MsgNoticeHotspotSaved   Key = "notice.hotspotsaved"
	MsgNoticeHotspotRenamed Key = "notice.hotspotrenamed"
	MsgNoticeAdvancedSaved  Key = "notice.advancedsaved"

	// Preconditions for the switch.
	MsgNoConfigYet        Key = "power.noconfig.headline"
	MsgNoConfigYetAdvice  Key = "power.noconfig.advice"
	MsgNoHotspotYet       Key = "power.nohotspot.headline"
	MsgNoHotspotYetAdvice Key = "power.nohotspot.advice"

	// Naming the hotspot.
	MsgSSIDMissing        Key = "hotspot.noname.headline"
	MsgSSIDMissingAdvice  Key = "hotspot.noname.advice"
	MsgSSIDTooLong        Key = "hotspot.toolong.headline"
	MsgSSIDTooLongAdvice  Key = "hotspot.toolong.advice"
	MsgSSIDBadChars       Key = "hotspot.badchars.headline"
	MsgSSIDBadCharsAdvice Key = "hotspot.badchars.advice"
	MsgSSIDSpaces         Key = "hotspot.spaces.headline"
	MsgSSIDSpacesAdvice   Key = "hotspot.spaces.advice"
	MsgPassTooShort       Key = "hotspot.short.headline"
	MsgPassTooShortAdvice Key = "hotspot.short.advice"
	MsgPassTooLong        Key = "hotspot.long.headline"
	MsgPassTooLongAdvice  Key = "hotspot.long.advice"
	MsgPassBadChars       Key = "hotspot.passchars.headline"
	MsgPassBadCharsAdvice Key = "hotspot.passchars.advice"
	MsgPassRefused        Key = "hotspot.refused.headline"
	MsgPassRefusedAdvice  Key = "hotspot.refused.advice"

	// Advanced-mode overrides.
	MsgAdvBadInternet       Key = "adv.badinternet.headline"
	MsgAdvBadInternetAdvice Key = "adv.badinternet.advice"
	MsgAdvBadHotspot        Key = "adv.badhotspot.headline"
	MsgAdvBadHotspotAdvice  Key = "adv.badhotspot.advice"
	MsgAdvNoAP              Key = "adv.noap.headline"
	MsgAdvNoAPAdvice        Key = "adv.noap.advice"
	MsgAdvBadChannel        Key = "adv.badchannel.headline"
	MsgAdvBadChannelAdvice  Key = "adv.badchannel.advice"
	MsgAdvChannelNaN        Key = "adv.channelnan.headline"
	MsgAdvBadBand           Key = "adv.badband.headline"
	MsgAdvBadBandAdvice     Key = "adv.badband.advice"
	MsgAdvBadCountry        Key = "adv.badcountry.headline"
	MsgAdvBadCountryAdvice  Key = "adv.badcountry.advice"
	MsgAdvBadSubnet         Key = "adv.badsubnet.headline"
	MsgAdvBadSubnetAdvice   Key = "adv.badsubnet.advice"
	MsgAdvSubnetV4          Key = "adv.subnetv4.headline"
	MsgAdvSubnetV4Advice    Key = "adv.subnetv4.advice"
	MsgAdvSubnetSmall       Key = "adv.subnetsmall.headline"
	MsgAdvSubnetSmallAdvice Key = "adv.subnetsmall.advice"
	MsgAdvBadLogLevel       Key = "adv.badloglevel.headline"
	MsgAdvBadLogLevelAdvice Key = "adv.badloglevel.advice"

	// Storage.
	MsgSaveConfigFailed   Key = "error.saveconfig.headline"
	MsgSaveHotspotFailed  Key = "error.savehotspot.headline"
	MsgSaveAdvancedFailed Key = "error.saveadvanced.headline"
	MsgSaveFailedAdvice   Key = "error.save.advice"
)

// Keys added with the dashboard layout: the tile row, how long the box has
// been up, and the events area.
const (
	// The summary tiles.
	MsgTileStatus     Key = "tile.status"
	MsgTileDevices    Key = "tile.devices"
	MsgTileConfig     Key = "tile.config"
	MsgTileUptime     Key = "tile.uptime"
	MsgTileConfigNone Key = "tile.config.none"
	MsgTileUptimeOff  Key = "tile.uptime.off"

	// How long it has been up. Latin digits, per the numeral decision above.
	MsgUptimeJustNow Key = "uptime.justnow"
	MsgUptimeMinutes Key = "uptime.minutes"
	MsgUptimeHours   Key = "uptime.hours"
	MsgUptimeDays    Key = "uptime.days"

	// The events area.
	MsgEventsHeading Key = "events.heading"
	MsgEventsEmpty   Key = "events.empty"
	MsgEventsNote    Key = "events.note"

	MsgEventSignedIn      Key = "event.signedin"
	MsgEventSwitchedOn    Key = "event.switchedon"
	MsgEventSwitchedOff   Key = "event.switchedoff"
	MsgEventConnected     Key = "event.connected"
	MsgEventDisconnected  Key = "event.disconnected"
	MsgEventStartFailed   Key = "event.startfailed"
	MsgEventConfigAdded   Key = "event.configadded"
	MsgEventConfigChanged Key = "event.configchanged"
	MsgEventHotspotNamed  Key = "event.hotspotnamed"
	MsgEventAdvancedSaved Key = "event.advancedsaved"
)
