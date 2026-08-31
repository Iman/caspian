// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Bidirectional isolation
// ---------------------------------------------------------------------------

// TestEveryIsolatedValueIsIsolated is the guard the requirement asked for: a
// test that fails if a new value of that kind is rendered unisolated.
//
// It works by reflection over pageData rather than from a list, which is the
// whole point. A list would cover the fields somebody remembered to add to it.
// This walks the struct, fills every field of type LTR with a unique sentinel,
// renders the page, and checks each sentinel came out inside a bdi element or
// on an element carrying dir="ltr". A field of type LTR added tomorrow is
// covered tomorrow.
//
// Why it matters more than it looks: an address dropped bare into Persian text
// is laid out by the Unicode bidirectional algorithm along with the sentence
// around it, and the dots and colons in it are neutral characters that take
// their direction from their surroundings. "10.62.0.1:8080" can render as
// "8080:10.62.0.1". The user then types what they see, and an address that
// displays wrongly is worse than one not displayed at all.
func TestEveryIsolatedValueIsIsolated(t *testing.T) {
	p := &Panel{}
	data := p.newPageData(LangFA, MsgAppName, "test-token", true)
	data.HasProblem = true
	data.ProblemHeadline = "headline"
	data.ProblemAdvice = "advice"
	data.ProblemDetail = "detail"

	sentinels := map[string]string{}
	fillLTR(t, reflect.ValueOf(&data).Elem(), "pageData", sentinels)
	if len(sentinels) == 0 {
		t.Fatal("no LTR fields were found, so this test checked nothing")
	}

	var out strings.Builder
	if err := pages["index"].ExecuteTemplate(&out, "base.html", data); err != nil {
		t.Fatalf("rendering: %v", err)
	}
	body := out.String()

	// A second render with the hotspot not yet named, so the branch carrying
	// the suggested name and passphrase is covered too. Without it those two
	// fields are only ever reported as "not rendered", which is the shape a
	// missed field takes.
	unnamed := p.newPageData(LangFA, MsgAppName, "test-token", true)
	unnamedSentinels := map[string]string{}
	fillLTR(t, reflect.ValueOf(&unnamed).Elem(), "pageData(unnamed)", unnamedSentinels)
	unnamed.HotspotReady = false
	unnamed.HasConfig = false
	// Cleared so the form falls to its else branch and renders the suggested
	// pair, which is the only place those two fields appear.
	unnamed.SSID, unnamed.Passphrase = "", ""
	delete(unnamedSentinels, "pageData(unnamed).SSID")
	delete(unnamedSentinels, "pageData(unnamed).Passphrase")
	var out2 strings.Builder
	if err := pages["index"].ExecuteTemplate(&out2, "base.html", unnamed); err != nil {
		t.Fatalf("rendering the unnamed-hotspot page: %v", err)
	}
	body += out2.String()
	for path, sentinel := range unnamedSentinels {
		if _, seen := sentinels[path]; !seen {
			sentinels[path] = sentinel
		} else {
			sentinels[path+" (unnamed)"] = sentinel
		}
	}

	checked := 0
	for path, sentinel := range sentinels {
		if !strings.Contains(body, sentinel) {
			// A field that does not reach the page cannot be misrendered, but
			// it is worth naming so a genuinely missing one is visible.
			t.Logf("%s is not rendered on this page", path)
			continue
		}
		checked++
		if kind, ok := isolationOf(body, sentinel); !ok {
			t.Errorf("%s is rendered without bidirectional isolation (%s); "+
				"wrap it in <bdi> or put dir=\"ltr\" on the element", path, kind)
		}
	}
	if checked == 0 {
		t.Fatal("no LTR value reached the page, so this test checked nothing")
	}
	t.Logf("%d isolated values checked, %d LTR fields in total", checked, len(sentinels))
}

// TestTheIsolationCheckerWorks is the positive control. Without it, a checker
// that returned true for everything would make the test above vacuous.
func TestTheIsolationCheckerWorks(t *testing.T) {
	cases := []struct {
		name string
		html string
		want bool
	}{
		{"bare", `<p>SENTINEL</p>`, false},
		{"bdi", `<p><bdi>SENTINEL</bdi></p>`, true},
		{"bdi with dir", `<p><bdi dir="ltr" class="mono">SENTINEL</bdi></p>`, true},
		{"input with dir", `<input dir="ltr" value="SENTINEL">`, true},
		{"input without dir", `<input value="SENTINEL">`, false},
		{"span with dir", `<span dir="ltr">SENTINEL</span>`, false},
		{"option without dir", `<option value="SENTINEL">SENTINEL</option>`, false},
		{"option with dir", `<option dir="ltr" value="SENTINEL">SENTINEL</option>`, true},
		{"input with dir auto", `<input dir="auto" value="SENTINEL">`, true},
		{"isolated once, bare later", `<p><bdi>SENTINEL</bdi></p><p>SENTINEL</p>`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, got := isolationOf(c.html, "SENTINEL"); got != c.want {
				t.Errorf("isolationOf(%q) = %v, want %v", c.html, got, c.want)
			}
		})
	}
}

// fillLTR walks a value, giving every LTR field a unique sentinel and making
// sure slices have an element to render.
func fillLTR(t *testing.T, v reflect.Value, path string, out map[string]string) {
	t.Helper()
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if !v.Field(i).CanSet() {
				continue
			}
			fillLTR(t, v.Field(i), path+"."+f.Name, out)
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Struct && v.Type().Elem() != reflect.TypeOf(LTR("")) {
			return
		}
		// One element, so the range in the template has something to draw.
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fillLTR(t, v.Index(0), path+"[0]", out)
	case reflect.String:
		if v.Type() == reflect.TypeOf(LTR("")) {
			s := fmt.Sprintf("ZZISOL%dZZ", len(out))
			v.SetString(s)
			out[path] = s
		}
	case reflect.Bool:
		// True everywhere, so every conditional section renders and every LTR
		// field inside one is reached.
		v.SetBool(true)
	}
}

// isolationOf reports whether EVERY occurrence of a value in rendered HTML is
// isolated, and how the first unisolated one fails.
//
// Every occurrence, not the first. A value wrapped in bdi in one place and
// dropped bare into a paragraph in another is exactly the bug this test is for,
// and checking only the first occurrence would pass it.
//
// Two forms count. Inside a bdi element, which is the normal case for text. Or
// on an element that cannot contain a bdi at all, which is an input, a textarea
// or an option, carrying dir="ltr" or dir="auto" itself. An option may only
// contain text, so a bdi inside one is not valid HTML and the dir attribute is
// the correct answer there.
//
// dir on an ordinary container such as a span or a div does NOT count. That
// sets the direction of the element rather than isolating this value from the
// text around it, which is a different thing.
func isolationOf(body, sentinel string) (string, bool) {
	if !strings.Contains(body, sentinel) {
		return "absent", false
	}
	isolable := []string{"<input", "<textarea", "<option", "<select"}
	hasDir := func(tag string) bool {
		return strings.Contains(tag, `dir="ltr"`) || strings.Contains(tag, `dir="auto"`)
	}
	isIsolable := func(tag string) bool {
		for _, e := range isolable {
			if strings.HasPrefix(tag, e) {
				return true
			}
		}
		return false
	}

	for off := 0; ; {
		i := strings.Index(body[off:], sentinel)
		if i < 0 {
			return "", true
		}
		i += off
		off = i + len(sentinel)

		before := body[:i]
		lt := strings.LastIndex(before, "<")
		gt := strings.LastIndex(before, ">")

		if lt > gt {
			// Inside a tag, so this is an attribute value. It counts only on an
			// element that cannot contain a bdi and that carries dir itself.
			tag := body[lt:]
			if end := strings.Index(tag, ">"); end >= 0 {
				tag = tag[:end]
			}
			if isIsolable(tag) && hasDir(tag) {
				continue
			}
			return "attribute on " + strings.Fields(tag)[0], false
		}

		// Text content. The thing that isolates it is the opening tag of the
		// element it sits directly inside, which is the tag ending at gt.
		if gt < 0 {
			return "bare text", false
		}
		tagStart := strings.LastIndex(before[:gt], "<")
		if tagStart < 0 {
			return "bare text", false
		}
		tag := before[tagStart : gt+1]
		if strings.HasPrefix(tag, "<bdi") {
			continue
		}
		if isIsolable(tag) && hasDir(tag) {
			continue
		}
		return "text inside " + strings.Fields(tag)[0], false
	}
}

// ---------------------------------------------------------------------------
// Direction-neutral stylesheet
// ---------------------------------------------------------------------------

// physicalProperty matches a CSS property that names a side.
//
// One stylesheet has to serve both directions, driven by the dir attribute on
// the root element. A layout mirrored by swapping left for right by hand is
// wrong somewhere within a week, and the tile row is exactly the shape that
// goes wrong that way.
var physicalProperty = regexp.MustCompile(
	`(?m)(^|[;{\s])(margin|padding|border)-(left|right)\s*:|` +
		`(?m)(^|[;{\s])(left|right)\s*:|` +
		`(?m)border-(left|right)-(width|color|style)\s*:|` +
		`(?m)text-align\s*:\s*(left|right)\s*[;}]`)

func TestStylesheetUsesLogicalProperties(t *testing.T) {
	css, err := assetFS.ReadFile("assets/panel.css")
	if err != nil {
		t.Fatal(err)
	}
	// Comments are stripped first: this file explains the rule in prose, and
	// the prose says "left" and "right" on purpose.
	stripped := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(css), " ")

	for i, line := range strings.Split(stripped, "\n") {
		if m := physicalProperty.FindString(line); m != "" {
			t.Errorf("panel.css line %d uses a physical direction (%q); use the logical property so one stylesheet serves both directions: %s",
				i+1, strings.TrimSpace(m), strings.TrimSpace(line))
		}
	}
}

// TestThePhysicalPropertyScannerWorks is the positive control.
func TestThePhysicalPropertyScannerWorks(t *testing.T) {
	for _, bad := range []string{
		"  margin-left: 1rem;",
		"  padding-right: 0;",
		"  left: -9999px;",
		"  border-left-width: 5px;",
		"  text-align: left;",
	} {
		if physicalProperty.FindString(bad) == "" {
			t.Errorf("the scanner did not flag %q, so it cannot flag anything", bad)
		}
	}
	for _, good := range []string{
		"  margin-inline-start: 1rem;",
		"  padding-block-end: 0;",
		"  inset-inline-start: -9999px;",
		"  border-inline-start-width: 5px;",
		"  text-align: center;",
		"  grid-column: 1 / -1;",
	} {
		if m := physicalProperty.FindString(good); m != "" {
			t.Errorf("the scanner flagged the logical property %q as %q", good, m)
		}
	}
}

// ---------------------------------------------------------------------------
// Colour
// ---------------------------------------------------------------------------

// Tropical jade sunrise is five colours. Measured 2026-08-30, as text on white
// only the teal reaches 4.5:1; the other four are between 1.40 and 1.96, so
// they are grounds and fills and nothing else.
//
// The four neutrals beside them: white is the reference card's own surface and
// is taken from it, and the other three are the teal lightened or darkened,
// because a page needs more than one surface and nothing supplied is dark
// enough for body text at a comfortable margin.
var tokens = map[string]string{
	// Supplied, unmodified.
	"teal":   "#097C87",
	"cyan":   "#23CED9",
	"sage":   "#A1CCA6",
	"coral":  "#FCA47C",
	"yellow": "#F9D779",
	// Neutrals. Only the first is not derived.
	"surface":   "#FFFFFF",
	"ground":    "#E6F2F3",
	"edge":      "#C2DEE1",
	"ink":       "#05444A",
	"ink-quiet": "#075D65",
}

// shippedPair is one foreground actually drawn on one background, with the
// threshold its role has to meet: 4.5:1 for text, 3:1 for a control boundary or
// a state marker.
type shippedPair struct {
	role   string
	fg, bg string
	min    float64
}

var shippedPairs = []shippedPair{
	{"body text", "ink", "surface", 4.5},
	{"body text", "ink", "ground", 4.5},
	{"body text on a chip", "ink", "sage", 4.5},
	{"secondary text", "ink-quiet", "surface", 4.5},
	{"secondary text", "ink-quiet", "ground", 4.5},
	{"link", "ink-quiet", "surface", 4.5},
	{"link", "ink-quiet", "ground", 4.5},
	{"heading", "teal", "surface", 4.5},

	// The rail. Its ground is the teal, so every foreground on it is one of
	// the near-whites, and the focus ring inverts there for the same reason.
	{"rail link", "surface", "teal", 4.5},
	{"rail icon", "ground", "teal", 3.0},
	// Hover fills the item with the ink rather than lightening the teal,
	// because lightening is what alpha is for and there is none here.
	{"rail link, hovered", "surface", "ink", 4.5},
	{"focus ring on the rail", "surface", "teal", 3.0},

	// The buttons. The primary action is the deepest colour on the page and
	// carries the near-white; the switch-off is the coral, which is a ground
	// and takes the ink.
	{"primary button label", "surface", "teal", 4.5},
	{"switch-off label", "ink", "coral", 4.5},
	{"focus ring on the primary button", "surface", "teal", 3.0},

	// The control bar's four grounds. It is a traffic light: green carrying,
	// amber switched on but not up yet, red switched off, and the coral pulse
	// for a deliberate cut. Every one of them holds the state word, the
	// next-step sentence and the fail-closed line, so every one is measured at
	// body-text strength rather than at the 3:1 a decoration would take.
	{"control bar, connected", "ink", "sage", 4.5},
	{"control bar, switched on and not up yet", "ink", "yellow", 4.5},
	{"control bar, switched off", "ink", "coral", 4.5},
	{"control bar, traffic cut", "ink", "coral", 4.5},

	// Messages.
	{"problem text", "ink", "coral", 4.5},
	{"notice text", "ink", "yellow", 4.5},
	// The badge on the instruction. The cyan is decoration everywhere else and
	// carries text here, so it is measured like anything that does: 5.64:1.
	// Note that the quieter ink is 3.95:1 on it and would NOT qualify, which
	// is why the badge uses the darker one.
	{"instruction badge", "ink", "cyan", 4.5},

	// Boundaries and state.
	{"control border", "teal", "surface", 3.0},
	{"control border on the page", "teal", "ground", 3.0},
	{"state marker ring", "ink", "surface", 3.0},
	{"state marker ring on the page", "ink", "ground", 3.0},
	{"state marker on a chip", "ink", "sage", 3.0},

	// The focus ring on every ground it is actually drawn on.
	{"focus ring", "ink", "surface", 3.0},
	{"focus ring on the page", "ink", "ground", 3.0},
	{"focus ring on a chip", "ink", "sage", 3.0},
	{"focus ring on the coral", "ink", "coral", 3.0},
	{"focus ring on the yellow", "ink", "yellow", 3.0},
}

// TestPaletteContrastMeetsWCAG recomputes every ratio rather than trusting the
// table in the stylesheet comment.
//
// A comment full of numbers is a comment that rots the first time somebody
// nudges a colour. This is the same arithmetic, run on every build, so a nudged
// colour is a failing test rather than an unreadable panel.
func TestPaletteContrastMeetsWCAG(t *testing.T) {
	for _, p := range shippedPairs {
		fg, ok := tokens[p.fg]
		if !ok {
			t.Fatalf("unknown token %q", p.fg)
		}
		bg, ok := tokens[p.bg]
		if !ok {
			t.Fatalf("unknown token %q", p.bg)
		}
		got := contrastRatio(fg, bg)
		if got < p.min {
			t.Errorf("%s: %s on %s is %.2f:1, below the %.1f:1 this role needs",
				p.role, p.fg, p.bg, got, p.min)
			continue
		}
		t.Logf("%-28s %-11s on %-11s %5.2f:1 (needs %.1f)", p.role, p.fg, p.bg, got, p.min)
	}
}

// TestNoSuppliedColourIsUsedAsText records the measurement that shaped the
// whole scheme, so that somebody who later reaches for the soft green as a text
// colour is stopped by a test rather than by a screenshot.
func TestNoSuppliedColourIsUsedAsText(t *testing.T) {
	for _, name := range []string{"cyan", "sage", "coral", "yellow", "edge", "ground"} {
		if got := contrastRatio(tokens[name], tokens["surface"]); got >= 4.5 {
			t.Errorf("%s reaches %.2f:1 on the surface; the premise that these are surface colours has changed", name, got)
		}
	}
	// And the one that is nearly a boundary colour and is not: the cyan. At
	// 1.93:1 on the card it cannot be a control boundary or a state marker,
	// which is why it draws the cap on a card and nothing else.
	if got := contrastRatio(tokens["cyan"], tokens["surface"]); got >= 3.0 {
		t.Errorf("cyan is %.2f:1 on surface, so the premise that it is decoration only has changed", got)
	}
}

// TestStylesheetUsesOnlyApprovedColours scans the stylesheet for hex colours
// and fails on one that is not a measured token.
//
// This is what stops the palette growing a shade nobody measured, which is how
// a scheme with a contrast table ends up with an unreadable corner.
func TestStylesheetUsesOnlyApprovedColours(t *testing.T) {
	css, err := assetFS.ReadFile("assets/panel.css")
	if err != nil {
		t.Fatal(err)
	}
	// The QR code's own black and white are not palette colours and must not
	// be: a QR symbol is read by a camera, and tinting it makes some readers
	// refuse it.
	allowed := map[string]bool{"#FFFFFF": true, "#000000": true}
	for _, v := range tokens {
		allowed[strings.ToUpper(v)] = true
	}

	stripped := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(css), " ")
	found := 0
	for _, m := range regexp.MustCompile(`#[0-9A-Fa-f]{3,8}\b`).FindAllString(stripped, -1) {
		found++
		if !allowed[strings.ToUpper(m)] {
			t.Errorf("panel.css uses %s, which is not a measured token", m)
		}
	}
	if found == 0 {
		t.Fatal("no colours were found in the stylesheet, so this test checked nothing")
	}
}

// TestStylesheetHasNoDarkMode pins the decision.
//
// A muted light scheme has no automatic dark counterpart. Either one is derived
// deliberately and its contrast measured, or the panel ships light only. This
// build ships light only and says so to the browser with color-scheme, so that
// a user-agent dark treatment is not applied to form controls and does not
// produce something nobody designed.
func TestStylesheetHasNoDarkMode(t *testing.T) {
	css, err := assetFS.ReadFile("assets/panel.css")
	if err != nil {
		t.Fatal(err)
	}
	stripped := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(css), " ")
	if strings.Contains(stripped, "prefers-color-scheme") {
		t.Error("the stylesheet has a colour-scheme rule; a dark mode needs its own measured palette, " +
			"and this build deliberately ships light only")
	}
	if !strings.Contains(stripped, "color-scheme: light") {
		t.Error("the stylesheet does not declare color-scheme: light, so a browser may apply its own dark treatment")
	}
}

// ---------------------------------------------------------------------------
// WCAG relative luminance and contrast, from the definition.
// ---------------------------------------------------------------------------

func channel(c float64) float64 {
	c /= 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func luminance(hex string) float64 {
	h := strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseInt(h[0:2], 16, 32)
	g, _ := strconv.ParseInt(h[2:4], 16, 32)
	b, _ := strconv.ParseInt(h[4:6], 16, 32)
	return 0.2126*channel(float64(r)) + 0.7152*channel(float64(g)) + 0.0722*channel(float64(b))
}

func contrastRatio(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	hi, lo := math.Max(la, lb), math.Min(la, lb)
	return (hi + 0.05) / (lo + 0.05)
}

// TestContrastArithmeticIsRight checks the implementation above against the two
// ratios the standard fixes: black on white is exactly 21:1, and any colour on
// itself is exactly 1:1.
func TestContrastArithmeticIsRight(t *testing.T) {
	if got := contrastRatio("#000000", "#FFFFFF"); math.Abs(got-21) > 0.01 {
		t.Errorf("black on white is %.4f:1, want 21:1; the contrast arithmetic is wrong and every number above is meaningless", got)
	}
	if got := contrastRatio("#B7C396", "#B7C396"); math.Abs(got-1) > 0.0001 {
		t.Errorf("a colour on itself is %.4f:1, want 1:1", got)
	}
}

// TestStylesheetHasNoAlpha pins an instruction, not a preference.
//
// The scheme is flat: depth is carried by a line and by a change of ground,
// both of which are colours somebody measured and put in the table above. A
// shadow is a black overlay at low opacity, a faded placeholder is ink nobody
// measured, and an eight-digit hex is a colour that is not the colour it names.
// None of the three can be checked by the contrast test, because none of them
// is a pair: they are a colour blended with whatever happens to be underneath.
//
// So they are banned outright and checked here. If a design later needs one, it
// needs a measured token instead, which is the point.
func TestStylesheetHasNoAlpha(t *testing.T) {
	css, err := assetFS.ReadFile("assets/panel.css")
	if err != nil {
		t.Fatal(err)
	}
	// Comments first: this file explains the rule in prose, and the prose has
	// to be able to name the things it is banning.
	stripped := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(css), " ")

	for _, ban := range []struct {
		what string
		re   *regexp.Regexp
		why  string
	}{
		{"rgba", regexp.MustCompile(`rgba\s*\(`), "a colour blended with whatever is behind it, which no contrast table can check"},
		{"hsla", regexp.MustCompile(`hsla\s*\(`), "same as rgba"},
		{"an eight-digit hex", regexp.MustCompile(`#[0-9A-Fa-f]{8}\b`), "a colour that is not the colour it names"},
		{"a colour with an alpha channel", regexp.MustCompile(`(?i)\b(?:color|background|border-color|fill|stroke)\s*:\s*[^;]*\/\s*0?\.\d`), "the slash form of alpha"},
		{"partial opacity", regexp.MustCompile(`opacity\s*:\s*(?:0|0?\.\d+)\s*[;}]`), "ink or a fill nobody measured"},
	} {
		if m := ban.re.FindString(stripped); m != "" {
			t.Errorf("panel.css uses %s (%q): %s", ban.what, strings.TrimSpace(m), ban.why)
		}
	}

	// box-shadow is allowed to appear only as the two tokens, and both are
	// none. A literal shadow would slip past the rgba check if it were written
	// with a named colour or a bare hex.
	for _, m := range regexp.MustCompile(`box-shadow\s*:\s*([^;]+);`).FindAllStringSubmatch(stripped, -1) {
		v := strings.TrimSpace(m[1])
		if v != "none" && !strings.HasPrefix(v, "var(--shadow-") {
			t.Errorf("panel.css sets box-shadow to %q; the scheme is flat and the shadow tokens are none", v)
		}
	}
	for _, tok := range []string{"--shadow-1", "--shadow-2"} {
		re := regexp.MustCompile(regexp.QuoteMeta(tok) + `\s*:\s*([^;]+);`)
		m := re.FindStringSubmatch(stripped)
		if m == nil {
			t.Errorf("%s is not defined, so a rule asking for it gets nothing and this test checks nothing", tok)
			continue
		}
		if strings.TrimSpace(m[1]) != "none" {
			t.Errorf("%s is %q, not none", tok, strings.TrimSpace(m[1]))
		}
	}
}

// TestEveryPairTheStylesheetActuallyDrawsIsMeasured closes the gap that let a
// two-point-seven-to-one pair ship while the contrast test was green.
//
// shippedPairs is a declaration. It says what the author believes is drawn on
// what, and TestPaletteContrastMeetsWCAG checks the arithmetic of that belief.
// Neither of them reads the stylesheet, so when the rail was given the soft
// green as its text colour on the teal ground, 2.76:1 against the 4.5:1 body
// text needs, every colour test stayed green and the page was wrong.
//
// This reads the rules instead. For every block that sets BOTH a colour and a
// background, the pair it draws has to be one somebody measured.
//
// What it does not cover, said plainly rather than left to be assumed: a block
// that sets only a colour and inherits its ground from an ancestor. Resolving
// that needs the cascade, which needs a browser. So this is a guard against the
// mistake that has actually been made here, not a proof about the whole page.
func TestEveryPairTheStylesheetActuallyDrawsIsMeasured(t *testing.T) {
	css, err := assetFS.ReadFile("assets/panel.css")
	if err != nil {
		t.Fatal(err)
	}
	stripped := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(css), " ")

	// The role names in the stylesheet resolve to the measured token names.
	// Anything not in here is either a literal or a token this test cannot
	// follow, and is skipped rather than guessed at.
	role := map[string]string{
		"teal": "teal", "cyan": "cyan", "sage": "sage", "coral": "coral", "yellow": "yellow",
		"surface": "surface", "ground": "ground", "edge": "edge",
		"ink": "ink", "ink-quiet": "ink-quiet",
		"rail": "teal", "edge-strong": "teal", "focus": "ink",
		"tile": "sage", "accent-on": "sage", "accent-warn": "coral",
		"accent-note": "yellow", "accent-bright": "cyan",
	}
	measured := map[string]bool{}
	for _, p := range shippedPairs {
		measured[p.fg+" on "+p.bg] = true
	}

	varRe := regexp.MustCompile(`var\(--([a-z-]+)\)`)
	tokenOf := func(decl string) string {
		m := varRe.FindStringSubmatch(decl)
		if m == nil {
			return ""
		}
		return role[m[1]]
	}

	blockRe := regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`)
	colorRe := regexp.MustCompile(`(?m)^\s*color\s*:\s*([^;]+);`)
	bgRe := regexp.MustCompile(`(?m)^\s*background(?:-color)?\s*:\s*([^;]+);`)

	checked := 0
	for _, b := range blockRe.FindAllStringSubmatch(stripped, -1) {
		sel, body := strings.TrimSpace(b[1]), b[2]
		cm, bm := colorRe.FindStringSubmatch(body), bgRe.FindStringSubmatch(body)
		if cm == nil || bm == nil {
			continue
		}
		fg, bg := tokenOf(cm[1]), tokenOf(bm[1])
		if fg == "" || bg == "" {
			continue
		}
		checked++
		if !measured[fg+" on "+bg] {
			t.Errorf("%s draws %s on %s, which is not in shippedPairs.\n"+
				"Measured, that pair is %.2f:1. Either it is wrong, or it is right and the table is missing it.",
				sel, fg, bg, contrastRatio(tokens[fg], tokens[bg]))
		}
	}
	if checked == 0 {
		t.Fatal("no block sets both a colour and a background through a token, so this test checked nothing")
	}
	t.Logf("%d drawn pairs checked against the measured table", checked)
}
