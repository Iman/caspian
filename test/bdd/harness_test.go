// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package bdd

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode"
)

// ---------------------------------------------------------------------------
// A behaviour harness, in about a hundred lines and with no dependency.
//
// The whole of it is: a scenario is a name and an ordered list of steps, a step
// is a function on the World that returns an error, and the name of a step is
// taken from the name of the Go function so that the source reads as a
// sentence and cannot drift from what the report prints.
//
// One rule follows from taking the name off the function: every step is a
// distinct top-level function. There are no parameterised steps and no step
// closures, because two closures made by the same factory share one code
// pointer and would print the same phrase. A variant of a step is a second
// named function, which is also the version a non-programmer can read.
// ---------------------------------------------------------------------------

// stepFunc is one Given, When or Then. It returns nil when the step held.
type stepFunc func(w *World) error

type keyword string

const (
	kwGiven keyword = "Given"
	kwWhen  keyword = "When"
	kwThen  keyword = "Then"
	kwAnd   keyword = "And"
)

type step struct {
	kw     keyword
	phrase string
	fn     stepFunc
}

// scenario is one behaviour: a sentence a non-programmer could check, and the
// steps that check it.
type scenario struct {
	name  string
	steps []step

	// defect is the deliberate fault TestEveryScenarioCanFail injects to prove
	// this scenario is capable of going red. A scenario with no defect named is
	// a scenario nobody has seen fail, and the suite refuses to have one: see
	// TestEveryScenarioNamesADefect.
	defectName string
	defect     func(*defects)
}

// Scenario starts a scenario. The name is the behaviour, in a sentence.
func Scenario(name string) *scenario { return &scenario{name: name} }

func (s *scenario) add(kw keyword, fn stepFunc) *scenario {
	s.steps = append(s.steps, step{kw: kw, phrase: phraseOf(fn), fn: fn})
	return s
}

func (s *scenario) Given(fn stepFunc) *scenario { return s.add(kwGiven, fn) }
func (s *scenario) When(fn stepFunc) *scenario  { return s.add(kwWhen, fn) }
func (s *scenario) Then(fn stepFunc) *scenario  { return s.add(kwThen, fn) }
func (s *scenario) And(fn stepFunc) *scenario   { return s.add(kwAnd, fn) }

// BreaksWhen names the defect that must make this scenario fail, and the
// change that injects it. It is not documentation: TestEveryScenarioCanFail
// runs every scenario a second time with the defect applied and requires red.
func (s *scenario) BreaksWhen(name string, apply func(*defects)) *scenario {
	s.defectName, s.defect = name, apply
	return s
}

// result is the outcome of executing one scenario.
type result struct {
	transcript string
	failedAt   int
	err        error
}

func (r result) ok() bool { return r.failedAt < 0 }

// execute runs the steps against w and returns a transcript. It stops at the
// first failure: the steps are ordered by dependency, so everything after a
// failed step is reporting on a world that never reached the state under test.
//
// It takes no *testing.T so that the harness itself can be tested; see
// TestHarnessReportsTheFailingStep.
func (s *scenario) execute(w *World) result {
	var b strings.Builder
	fmt.Fprintf(&b, "Scenario: %s\n", s.name)

	res := result{failedAt: -1}
	for i, st := range s.steps {
		if !res.ok() {
			fmt.Fprintf(&b, "    %-5s %s\n", st.kw, pad(st.phrase, "not run"))
			continue
		}
		if err := st.fn(w); err != nil {
			res.failedAt, res.err = i, err
			fmt.Fprintf(&b, "    %-5s %s\n", st.kw, pad(st.phrase, "FAILED"))
			continue
		}
		fmt.Fprintf(&b, "    %-5s %s\n", st.kw, pad(st.phrase, "ok"))
	}
	res.transcript = b.String()
	return res
}

func pad(phrase, status string) string {
	const width = 62
	if len(phrase) < width {
		phrase += strings.Repeat(" ", width-len(phrase))
	}
	return phrase + " " + status
}

// run executes the scenario as a Go subtest.
func (s *scenario) run(t *testing.T, d defects) {
	t.Helper()
	t.Run(s.name, func(t *testing.T) {
		w := newWorld(t, d)
		defer w.close()
		res := s.execute(w)
		if !res.ok() {
			t.Errorf("\n%s\n  %s %s\n    %v\n",
				res.transcript, s.steps[res.failedAt].kw, s.steps[res.failedAt].phrase, res.err)
			return
		}
		t.Log("\n" + res.transcript)
	})
}

// ---------------------------------------------------------------------------
// Turning a Go function name into a sentence
// ---------------------------------------------------------------------------

// phraseOf renders a step function's name as words: aFreshBox becomes "a fresh
// box", theDNSRedirectIsInForce becomes "the DNS redirect is in force".
//
// A run of capitals is kept together so an acronym survives, and the last
// capital of a run starts the next word when a lower-case letter follows it,
// which is what keeps "DNSRedirect" from becoming "DNSR edirect".
func phraseOf(fn stepFunc) string {
	full := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	if i := strings.LastIndex(full, "."); i >= 0 {
		full = full[i+1:]
	}
	return strings.Join(wordsOf(full), " ")
}

// wordsOf splits an identifier and lower-cases everything that is not an
// acronym.
func wordsOf(ident string) []string {
	words := splitCamel(ident)
	for i, w := range words {
		if !keepsCase(w) {
			words[i] = strings.ToLower(w)
		}
	}
	return words
}

// preserved are the words that camel case cannot be read out of, so they are
// listed. Two shapes need it: a mixed-case token, where "IPv6" would otherwise
// split into "I" and "Pv6" by the same rule that correctly splits "AResolver"
// into "a" and "resolver"; and a proper noun, which would otherwise be
// lower-cased along with every ordinary word.
//
// The list is short on purpose. It is not a dictionary; anything that needs an
// entry here is usually a step name that would read better rewritten.
var preserved = []string{"IPv4", "IPv6", "Google"}

func matchPreserved(rs []rune, i int) string {
	for _, tok := range preserved {
		t := []rune(tok)
		if i+len(t) > len(rs) {
			continue
		}
		if string(rs[i:i+len(t)]) == tok {
			return tok
		}
	}
	return ""
}

// splitCamel breaks an identifier into words.
//
// A word ends at a capital that follows a lower-case letter, at a capital that
// ends a run of capitals and is followed by a lower-case letter, and at the
// start of a preserved token.
func splitCamel(s string) []string {
	var words []string
	rs := []rune(s)
	for i := 0; i < len(rs); {
		if tok := matchPreserved(rs, i); tok != "" {
			words = append(words, tok)
			i += len([]rune(tok))
			continue
		}
		start := i
		i++
		for i < len(rs) {
			if matchPreserved(rs, i) != "" {
				break
			}
			if unicode.IsUpper(rs[i]) {
				if !unicode.IsUpper(rs[i-1]) {
					break
				}
				if i+1 < len(rs) && unicode.IsLower(rs[i+1]) {
					break
				}
			}
			i++
		}
		words = append(words, string(rs[start:i]))
	}
	return words
}

// keepsCase reports whether a word must not be lower-cased: a preserved token,
// or an acronym, which is a word opening with two capitals. That covers DNS,
// AP and MTU and excludes ordinary words such as Box, Is and A.
func keepsCase(w string) bool {
	for _, tok := range preserved {
		if w == tok {
			return true
		}
	}
	rs := []rune(w)
	return len(rs) >= 2 && unicode.IsUpper(rs[0]) && unicode.IsUpper(rs[1])
}

// ---------------------------------------------------------------------------
// Tests of the harness itself.
//
// The suite's whole claim is that a scenario can fail. That claim starts with
// the harness: a harness that swallows an error would make every scenario in
// the file below a green light wired to nothing.
// ---------------------------------------------------------------------------

func alwaysHolds(w *World) error { return nil }

func neverHolds(w *World) error { return fmt.Errorf("this step always fails") }

func alsoAlwaysHolds(w *World) error { return nil }

func TestHarnessReportsTheFailingStep(t *testing.T) {
	s := Scenario("a scenario with a step that cannot hold").
		Given(alwaysHolds).
		When(neverHolds).
		Then(alsoAlwaysHolds)

	res := s.execute(nil)
	if res.ok() {
		t.Fatal("the harness reported a pass for a scenario whose second step returned an error")
	}
	if res.failedAt != 1 {
		t.Errorf("failedAt = %d, want 1", res.failedAt)
	}
	if !strings.Contains(res.transcript, "When  never holds") {
		t.Errorf("the transcript does not name the failing step:\n%s", res.transcript)
	}
	if !strings.Contains(res.transcript, "FAILED") {
		t.Errorf("the transcript does not mark the failure:\n%s", res.transcript)
	}
	if !strings.Contains(res.transcript, "not run") {
		t.Errorf("the transcript does not say the later step was skipped:\n%s", res.transcript)
	}
}

func TestHarnessReportsAPassWhenEveryStepHolds(t *testing.T) {
	s := Scenario("a scenario whose steps all hold").Given(alwaysHolds).Then(alsoAlwaysHolds)
	if res := s.execute(nil); !res.ok() {
		t.Fatalf("a scenario of two holding steps was reported failed: %v", res.err)
	}
}

func TestStepPhrasesReadAsSentences(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"aFreshBox", "a fresh box"},
		{"theEngineIsRunning", "the engine is running"},
		{"clientTrafficLeavesOnlyThroughTheTunnel", "client traffic leaves only through the tunnel"},
		{"theDNSRedirectIsInForce", "the DNS redirect is in force"},
		{"noClientIPv6IsOffered", "no client IPv6 is offered"},
		{"theAPIsBeaconing", "the AP is beaconing"},
		{"noDownloadedGeoDataFileIsNeeded", "no downloaded geo data file is needed"},
		{"clientDNSCannotEscapeToAResolverOfItsOwnChoosing", "client DNS cannot escape to a resolver of its own choosing"},
		{"theUserIsToldNoAdapterCanCreateAHotspot", "the user is told no adapter can create a hotspot"},
		{"noResolverIsAGoogleOne", "no resolver is a Google one"},
	} {
		if got := strings.Join(wordsOf(c.in), " "); got != c.want {
			t.Errorf("phrase of %q = %q, want %q", c.in, got, c.want)
		}
	}
}
