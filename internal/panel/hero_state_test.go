// SPDX-License-Identifier: AGPL-3.0-or-later

package panel

import (
	"strings"
	"testing"
)

// The control bar is a traffic light. Green is the only state that means the
// box is doing its job; red is the only one that means it is doing nothing and
// somebody has to press something; amber is everything in between.
//
// This exists because amber had no state of its own. "Switched on, tunnel not
// up yet" and "switched off" both drew the page ground, so the bar said the
// same thing about a box that was seconds from working and a box that needed a
// press.
func TestHeroClassIsATrafficLight(t *testing.T) {
	for _, c := range []struct {
		name                     string
		cut, connected, starting bool
		want                     string
	}{
		{"carrying traffic is green", false, true, false, "ok"},
		{"starting up is amber", false, false, true, "wait"},
		{"switched off is red", false, false, false, "off"},
		{"a deliberate cut keeps its own state", true, true, false, "cut"},
		{"the cut outranks connected", true, true, false, "cut"},
		{"the cut outranks starting", true, false, true, "cut"},
		{"connected outranks starting", false, true, true, "ok"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := heroClass(c.cut, c.connected, c.starting)
			if got != c.want {
				t.Errorf("heroClass(cut=%v, connected=%v, starting=%v) = %q, want %q",
					c.cut, c.connected, c.starting, got, c.want)
			}
		})
	}

	// Every class this function can return must be one a client can actually
	// be served, or the rule that paints it is decoration.
	//
	// The first version of this took a "running" flag, and "wait" was
	// unreachable: Running is engine-running AND hotspot-up, Connected is that
	// AND not cut, so running-and-not-connected IS the cut, which is handled
	// first. The amber rule existed, was measured for contrast, was documented
	// to the user, and could never appear. The browser suite caught it by
	// driving all 32 combinations of phase, hotspot, cut and fault and never
	// seeing it. This is the unit-level guard for the same thing.
	seen := map[string]bool{}
	for _, cut := range []bool{true, false} {
		for _, connected := range []bool{true, false} {
			for _, starting := range []bool{true, false} {
				seen[heroClass(cut, connected, starting)] = true
			}
		}
	}
	for _, want := range []string{"ok", "wait", "off", "cut"} {
		if !seen[want] {
			t.Errorf("no combination of inputs makes heroClass return %q, "+
				"so the .hero-%s rule can never be drawn and the state it names "+
				"is one this appliance cannot report", want, want)
		}
	}
}

// Every class heroClass can return has a rule that paints it. A state that
// returns a class the stylesheet has never heard of falls through to the page
// ground, which is exactly the fault this change exists to remove, and it does
// it silently: the page still renders, it just says nothing.
func TestEveryHeroStateHasAGround(t *testing.T) {
	b, err := assetFS.ReadFile("assets/panel.css")
	if err != nil {
		t.Fatalf("read panel.css: %v", err)
	}
	css := string(b)
	for _, class := range []string{"ok", "wait", "off", "cut"} {
		if !strings.Contains(css, ".hero-"+class+" {") {
			t.Errorf("heroClass can return %q and the stylesheet has no .hero-%s rule, "+
				"so that state would draw the page ground and say nothing", class, class)
		}
	}
}

// The bar's state class and the cut form's wrapper class were the same string,
// .hero-cut. Two rules, two different elements, both applying to both: the form
// carried the cut state's coral ground and its infinite pulse in EVERY state
// including green, and the bar took the form's display and align-items whenever
// it went cut, which is not the layout the bar is written for.
//
// The guard is that the form's class is not one heroClass can produce. A future
// edit that renames it back collides again, and this fails.
func TestTheCutFormDoesNotShareAClassWithABarState(t *testing.T) {
	b, err := templateFS.ReadFile("templates/index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	html := string(b)
	for _, class := range []string{"ok", "wait", "off", "cut"} {
		if strings.Contains(html, `<form class="hero-`+class+`"`) {
			t.Errorf("a form is classed hero-%s, which is also what heroClass returns "+
				"for one of the bar's states, so the two elements share every rule "+
				"written for either of them", class)
		}
	}
}

// The bar's ground comes from its state class and from nothing else.
//
// This is the guard for the defect that made the whole traffic light invisible.
// The four .hero-<state> rules were written first, then `.hero { background:
// var(--ground) }` was written after them. Equal specificity, later wins, so
// every state drew the page ground: a connected box, a starting box, a switched
// off box and a cut box were the same colour, and the comment above the state
// rules said in prose that they were not. Nothing failed, because nothing
// checked.
//
// A rule that sets a background on .hero itself can only ever be dead code or
// an override of a state, so the check is that it does not exist.
func TestTheBarsGroundComesFromItsStateAndNothingElse(t *testing.T) {
	b, err := assetFS.ReadFile("assets/panel.css")
	if err != nil {
		t.Fatalf("read panel.css: %v", err)
	}

	// The bare .hero selector only. `.hero-ok` and `.hero .tile-label` are
	// different selectors and are allowed whatever they need.
	blocks := strings.Split(string(b), "\n.hero {")
	if len(blocks) < 2 {
		t.Fatal("no bare .hero rule in panel.css; this guard has stopped guarding anything")
	}
	for _, after := range blocks[1:] {
		body, _, ok := strings.Cut(after, "}")
		if !ok {
			t.Fatal("unterminated .hero rule in panel.css")
		}
		// Comments are stripped first. The prose in this stylesheet explains
		// what it does NOT do as often as what it does, and the first version
		// of this guard failed on its own explanation of the fault.
		for _, line := range strings.Split(stripCSSComments(body), "\n") {
			decl := strings.TrimSpace(line)
			name, _, ok := strings.Cut(decl, ":")
			if !ok {
				continue
			}
			name = strings.TrimSpace(name)
			if name == "background" || strings.HasPrefix(name, "background-") {
				t.Errorf(".hero sets %q. The four .hero-<state> rules set the ground and "+
					"this has the same specificity, so whichever is written later wins and "+
					"the traffic light stops working. Put the colour on the state, not here.",
					decl)
			}
		}
	}
}

// stripCSSComments removes every /* ... */ span. Unterminated comments take the
// rest of the input, which is what a browser does with them too.
func stripCSSComments(s string) string {
	var b strings.Builder
	for {
		before, after, ok := strings.Cut(s, "/*")
		b.WriteString(before)
		if !ok {
			return b.String()
		}
		_, rest, closed := strings.Cut(after, "*/")
		if !closed {
			return b.String()
		}
		s = rest
	}
}
