// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A bilingual interface rots one string at a time: somebody adds a message in
// the language they speak, nothing fails, and the other language quietly
// develops holes. These are the mechanical guards against that.

// Every check below derives its language set from Langs and compares each
// language against DefaultLang as the reference. A third language added to
// Langs is therefore checked on the day it is added, with no edit here. The
// checks themselves live in helpers that return the defects as errors, so
// TestCatalogueChecksBiteOnABrokenLanguage can prove they detect what they
// claim to detect.

// sameOnPurpose lists the keys whose text is legitimately identical in every
// language. Each one is here because it is a proper noun or a token, not
// because nobody translated it.
var sameOnPurpose = map[Key]bool{
	MsgAppName: true, // the product's name
}

// sortedKeysOf returns every key of one catalogue, sorted, so a defect list is
// stable between runs.
func sortedKeysOf(m map[Key]string) []Key {
	out := make([]Key, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// catalogueParityDefects is the parity check, in both directions, between the
// reference language and every other language in langs.
//
// It returns one error per defect and never fails a test itself, so a negative
// test can run it over a deliberately broken catalogue.
func catalogueParityDefects(cat map[Lang]map[Key]string, langs []Lang, ref Lang) []error {
	var errs []error
	refMsgs := cat[ref]
	if len(refMsgs) == 0 {
		return []error{fmt.Errorf("the %s catalogue is empty, so nothing can be compared against it", ref)}
	}
	for _, lang := range langs {
		if lang == ref {
			continue
		}
		msgs := cat[lang]
		if len(msgs) == 0 {
			errs = append(errs, fmt.Errorf("the %s catalogue is empty", lang))
			continue
		}
		for _, k := range sortedKeysOf(msgs) {
			if _, ok := refMsgs[k]; !ok {
				errs = append(errs, fmt.Errorf("%q is in %s and missing from %s", k, lang, ref))
			}
		}
		for _, k := range sortedKeysOf(refMsgs) {
			if _, ok := msgs[k]; !ok {
				errs = append(errs, fmt.Errorf("%q is in %s and missing from %s", k, ref, lang))
			}
		}
		if len(msgs) != len(refMsgs) {
			errs = append(errs, fmt.Errorf("%s has %d messages and %s has %d", lang, len(msgs), ref, len(refMsgs)))
		}
	}
	return errs
}

// catalogueContentDefects catches the other way a catalogue rots: the key is
// present, so the parity check passes, and the value is a placeholder, a copy
// of the reference text, or a sentence with a different number of format verbs.
//
// Only keys present in both the reference and the language under test are
// compared; a missing key is a parity defect and is reported there.
func catalogueContentDefects(cat map[Lang]map[Key]string, langs []Lang, ref Lang, allowSame map[Key]bool) []error {
	var errs []error
	refMsgs := cat[ref]
	for _, k := range sortedKeysOf(refMsgs) {
		if strings.TrimSpace(refMsgs[k]) == "" {
			errs = append(errs, fmt.Errorf("%q has an empty %s message", k, ref))
		}
	}
	for _, lang := range langs {
		if lang == ref {
			continue
		}
		msgs := cat[lang]
		for _, k := range sortedKeysOf(msgs) {
			got := msgs[k]
			want, ok := refMsgs[k]
			if !ok {
				continue
			}
			if strings.TrimSpace(got) == "" {
				errs = append(errs, fmt.Errorf("%q has an empty %s message", k, lang))
			}
			if got == want && !allowSame[k] {
				errs = append(errs, fmt.Errorf("%q is identical in %s and %s (%q), which usually means it was never translated", k, lang, ref, got))
			}
			// A format verb in one language and not the other is a crash
			// waiting for whichever language nobody tested.
			if strings.Count(got, "%") != strings.Count(want, "%") {
				errs = append(errs, fmt.Errorf("%q has %d format verbs in %s and %d in %s", k,
					strings.Count(got, "%"), lang, strings.Count(want, "%"), ref))
			}
		}
	}
	return errs
}

// TestEveryMessageExistsInBothLanguages is the parity check, in both
// directions, for every language in Langs against the default language.
//
// It can genuinely fail, which is the reason the catalogues are written out as
// separate literals instead of being generated from one table of pairs. A
// generator would make them agree by construction and this test would report
// success on a catalogue that had lost half its Persian.
//
// The name predates the loop over Langs and is kept because i18n.go and
// i18n_messages.go refer to it by name.
func TestEveryMessageExistsInBothLanguages(t *testing.T) {
	if len(Langs) < 2 {
		t.Fatalf("Langs is %v, so there is no second language to compare and this test checked nothing", Langs)
	}
	for _, lang := range Langs {
		if len(messages[lang]) == 0 {
			t.Fatalf("the %s catalogue is empty, so this test checked nothing", lang)
		}
	}
	for _, err := range catalogueParityDefects(messages, Langs, DefaultLang) {
		t.Error(err)
	}
	t.Logf("%d messages in %s, compared against %d other languages", len(messages[DefaultLang]), DefaultLang, len(Langs)-1)
}

// TestNoMessageIsEmptyOrUntranslated catches the other way a catalogue rots:
// the key is present, so the parity test passes, and the value is a placeholder
// or a copy of the English.
func TestNoMessageIsEmptyOrUntranslated(t *testing.T) {
	errs := catalogueContentDefects(messages, Langs, DefaultLang, sameOnPurpose)
	identical := 0
	for _, err := range errs {
		t.Error(err)
		if strings.Contains(err.Error(), "identical") {
			identical++
		}
	}
	if identical > 0 {
		t.Logf("%d untranslated messages", identical)
	}
}

// TestCatalogueChecksBiteOnABrokenLanguage is the negative control for the two
// tests above. It builds a catalogue with a third, fake language that has one
// key missing, one key extra, one empty value, one untranslated value and one
// wrong format-verb count, and asserts that every defect is named.
//
// Without this, the checks could be refactored into something that loops over
// Langs and finds nothing, and the green run would look identical. The fake
// language is NOT registered in Langs; it exists only inside this test.
func TestCatalogueChecksBiteOnABrokenLanguage(t *testing.T) {
	const fake = Lang("xx")
	if fake.Valid() {
		t.Fatalf("%s is a real language in this build; pick another code for the fake", fake)
	}
	ref := DefaultLang
	cat := map[Lang]map[Key]string{
		ref: {
			"k.missing": "Missing from language xx",
			"k.empty":   "Present but empty in language xx",
			"k.same":    "Copied verbatim into language xx",
			"k.verbs":   "Needs %s and %d",
			"k.fine":    "Translated properly",
			MsgAppName:  "Caspian",
		},
		// A well-formed second language: nothing about it may be reported.
		LangFA: {
			"k.missing": "fa missing",
			"k.empty":   "fa empty",
			"k.same":    "fa same",
			"k.verbs":   "fa %s %d",
			"k.fine":    "fa fine",
			MsgAppName:  "Caspian",
		},
		fake: {
			"k.empty":  "   ",
			"k.same":   "Copied verbatim into language xx",
			"k.verbs":  "only %s",
			"k.fine":   "xx fine",
			"k.extra":  "Not in the reference at all",
			MsgAppName: "Caspian",
		},
	}
	langs := []Lang{ref, LangFA, fake}

	var all []string
	for _, err := range catalogueParityDefects(cat, langs, ref) {
		all = append(all, err.Error())
	}
	for _, err := range catalogueContentDefects(cat, langs, ref, sameOnPurpose) {
		all = append(all, err.Error())
	}
	joined := strings.Join(all, "\n")
	t.Logf("defects reported:\n%s", joined)

	want := []string{
		`"k.missing" is in en and missing from xx`,
		`"k.extra" is in xx and missing from en`,
		`"k.empty" has an empty xx message`,
		`"k.same" is identical in xx and en`,
		`"k.verbs" has 1 format verbs in xx and 2 in en`,
	}
	for _, w := range want {
		if !strings.Contains(joined, w) {
			t.Errorf("no defect says %q", w)
		}
	}
	// Five planted defects, no more: an extra report here would be a check that
	// fires on healthy input, which is how a guard gets commented out.
	if len(all) != len(want) {
		t.Errorf("%d defects reported, want exactly %d", len(all), len(want))
	}
	healthyLang := regexp.MustCompile(`\b` + string(LangFA) + `\b`)
	for _, line := range all {
		if healthyLang.MatchString(line) {
			t.Errorf("the healthy language was reported: %s", line)
		}
		if strings.Contains(line, string(MsgAppName)) {
			t.Errorf("an allow-listed key was reported: %s", line)
		}
	}

	// And the checks return nothing on the real catalogue with no fake
	// language, which is what the two positive tests rely on.
	healthy := append(catalogueParityDefects(messages, Langs, DefaultLang),
		catalogueContentDefects(messages, Langs, DefaultLang, sameOnPurpose)...)
	if len(healthy) != 0 {
		t.Errorf("the shipped catalogue reports %d defects; the positive tests should be failing", len(healthy))
	}
}

// TestEveryDeclaredKeyHasAMessage walks the Key constants declared in i18n.go
// and checks each one resolves.
//
// Parsed out of the source rather than listed here, so a constant added without
// a message fails without anybody remembering to add it to a list.
func TestEveryDeclaredKeyHasAMessage(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "i18n.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing i18n.go: %v", err)
	}
	found := 0
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		id, ok := vs.Type.(*ast.Ident)
		if !ok || id.Name != "Key" {
			return true
		}
		for _, v := range vs.Values {
			lit, ok := v.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			key := Key(strings.Trim(lit.Value, `"`))
			found++
			for _, lang := range Langs {
				if _, ok := messages[lang][key]; !ok {
					t.Errorf("%s: declared key %q has no message", lang, key)
				}
			}
		}
		return true
	})
	if found == 0 {
		t.Fatal("no Key constants were found, so this test checked nothing")
	}
	t.Logf("%d declared keys checked", found)
}

// TestEveryFaultAndEventHasAMessage covers the two closed vocabularies whose
// values reach the screen through a Key method.
func TestEveryFaultAndEventHasAMessage(t *testing.T) {
	for _, lang := range Langs {
		for _, f := range faults {
			if got := T(lang, f.Key()); strings.Contains(got, missingMarker) {
				t.Errorf("%s: fault %q renders as %q", lang, f, got)
			}
		}
		for _, e := range eventKinds {
			if got := T(lang, e.Key()); strings.Contains(got, missingMarker) {
				t.Errorf("%s: event %q renders as %q", lang, e, got)
			}
		}
		for _, k := range []InterfaceKind{KindEthernet, KindBuiltinWiFi, KindUSBWiFi, KindWiFi} {
			if got := T(lang, k.Key()); strings.Contains(got, missingMarker) {
				t.Errorf("%s: interface kind %q renders as %q", lang, k, got)
			}
		}
	}
}

// TestMissingKeysAreLoud is the positive control for every test above that
// looks for the missing marker. If T quietly returned the key, or fell back to
// the other language, none of those tests could fail.
func TestMissingKeysAreLoud(t *testing.T) {
	got := T(LangFA, Key("this.key.does.not.exist"))
	if !strings.Contains(got, missingMarker) {
		t.Fatalf("a missing key rendered as %q, with no marker; the catalogue tests cannot fail", got)
	}
	// And it must not fall back to English, which would make a page half one
	// language and half the other: a bug to the user, a pass to the tests.
	if got == messages[LangEN][MsgAppName] {
		t.Error("a missing Persian message fell back to English")
	}
}

// ---------------------------------------------------------------------------
// No user-facing text outside the catalogue
// ---------------------------------------------------------------------------

// TestNoInlineUserFacingStringsInGoSource parses this package and fails on a
// string literal assigned to a Problem's Headline or Advice.
//
// Those fields are of type Key, and Go converts an untyped string constant to a
// named string type without complaint, so `Problem{Headline: "text"}` compiles
// perfectly well. This is what stops it: an inline sentence there would be
// English on a Persian box, and nothing else in the build would notice.
func TestNoInlineUserFacingStringsInGoSource(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		checked++
		ast.Inspect(f, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			id, ok := kv.Key.(*ast.Ident)
			if !ok {
				return true
			}
			if id.Name != "Headline" && id.Name != "Advice" {
				return true
			}
			lit, ok := kv.Value.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if strings.Trim(lit.Value, `"`) == "" {
				return true // an explicit empty key is not a sentence
			}
			t.Errorf("%s: %s is set to the literal %s; user-facing text has to be a catalogue key",
				fset.Position(lit.Pos()), id.Name, lit.Value)
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no Go files were parsed, so this test checked nothing")
	}
}

// templateAction matches a Go template action, including the multi-line
// comment form this project uses at the top of each template.
var templateAction = regexp.MustCompile(`(?s)\{\{.*?\}\}`)

// htmlTag matches a tag.
var htmlTag = regexp.MustCompile(`(?s)<[^>]*>`)

// letters matches anything that would be a word on screen, in either script.
var letters = regexp.MustCompile(`[A-Za-z\x{0600}-\x{06FF}]`)

// TestNoTranslatableTextInTemplates strips the actions and the tags out of each
// template and fails on any letters left over.
//
// What is left after both passes is text a browser would render, and any of it
// is a sentence somebody wrote in one language.
func TestNoTranslatableTextInTemplates(t *testing.T) {
	names := templateNames()
	if len(names) == 0 {
		t.Fatal("no templates were found")
	}
	for _, name := range names {
		body, err := templateFS.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		// Actions first: they can contain angle brackets and live inside tags.
		stripped := templateAction.ReplaceAllString(string(body), " ")
		stripped = htmlTag.ReplaceAllString(stripped, " ")
		stripped = strings.ReplaceAll(stripped, "<!doctype html>", " ")

		for i, line := range strings.Split(stripped, "\n") {
			if m := letters.FindString(line); m != "" {
				t.Errorf("%s line %d has text outside the catalogue: %q",
					name, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// TestTheTemplateTextScannerWorks is the positive control for the test above.
func TestTheTemplateTextScannerWorks(t *testing.T) {
	planted := `<p class="hint">Paste your config here</p>`
	stripped := htmlTag.ReplaceAllString(templateAction.ReplaceAllString(planted, " "), " ")
	if letters.FindString(stripped) == "" {
		t.Fatal("the scanner did not see inline English, so it cannot fail on any")
	}
	// And a template made only of actions and tags is clean.
	clean := `<p class="hint">{{.T "config.paste.hint"}}</p>`
	stripped = htmlTag.ReplaceAllString(templateAction.ReplaceAllString(clean, " "), " ")
	if letters.FindString(stripped) != "" {
		t.Errorf("the scanner flagged a clean template: %q", stripped)
	}
}

var templateKeyRE = regexp.MustCompile(`\.T\s+"([^"]+)"`)

// TestEveryTemplateKeyExists checks the keys the templates name.
func TestEveryTemplateKeyExists(t *testing.T) {
	found := 0
	for _, name := range templateNames() {
		body, err := templateFS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range templateKeyRE.FindAllStringSubmatch(string(body), -1) {
			found++
			for _, lang := range Langs {
				if _, ok := messages[lang][Key(m[1])]; !ok {
					t.Errorf("%s: template %s names key %q, which has no %s message", lang, name, m[1], lang)
				}
			}
		}
	}
	if found == 0 {
		t.Fatal("no template keys were found, so this test checked nothing")
	}
	t.Logf("%d key references in templates", found)
}

// ---------------------------------------------------------------------------
// Language selection
// ---------------------------------------------------------------------------

// TestEnglishIsTheDefault checks a browser with no saved preference.
func TestEnglishIsTheDefault(t *testing.T) {
	h := newHarness(t)
	res, body := h.get("/setup")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /setup: %d", res.StatusCode)
	}
	if !strings.Contains(body, `<html lang="en" dir="ltr">`) {
		t.Error("a fresh browser must receive English LTR")
	}
	if !strings.Contains(body, T(LangEN, MsgSetupHeading)) {
		t.Error("English setup heading is missing")
	}
}

func TestLanguageDropdownOnEveryPage(t *testing.T) {
	for _, lang := range Langs {
		h := newHarness(t)
		h.get("/?lang=" + string(lang))
		for _, path := range []string{"/setup"} {
			_, body := h.get(path)
			checkLanguageDropdown(t, body, lang)
		}
		h.lang = lang
		h.ready()
		for _, path := range []string{"/", "/?advanced=1", "/help"} {
			_, body := h.get(path)
			checkLanguageDropdown(t, body, lang)
		}
		h.signedOut()
		_, body := h.get("/login?lang=" + string(lang))
		checkLanguageDropdown(t, body, lang)
	}
}

func checkLanguageDropdown(t *testing.T, body string, lang Lang) {
	t.Helper()
	header := regexp.MustCompile(`(?s)<header class="topbar[^>]*>(.*?)</header>`).FindString(body)
	if !strings.Contains(header, `<label for="panel-language">`) || !strings.Contains(header, `<select id="panel-language" name="lang"`) {
		t.Fatal("topbar lacks a labelled language dropdown")
	}
	for _, available := range Langs {
		option := `value="` + string(available) + `"`
		if available == lang {
			option += ` selected`
		}
		if !strings.Contains(header, option) {
			t.Errorf("missing option or selection: %s", option)
		}
	}
	if strings.Count(header, ` selected`) != 1 {
		t.Error("exactly one language must be selected")
	}
	if !strings.Contains(header, `method="get"`) || !strings.Contains(header, `type="submit"`) {
		t.Error("language choice must work without JavaScript")
	}
}

// TestLanguageIsPerBrowserAndSticks checks the switch and the cookie.
//
// Per browser, not per box, and not in the state file: two people using the
// same appliance must not fight over the language, and it is not worth a
// privileged write to a file that also holds the user's proxy config.
func TestLanguageIsPerBrowserAndSticks(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)

	res, body := h.get("/?lang=en")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /?lang=en: %d", res.StatusCode)
	}
	if !strings.Contains(body, `lang="en"`) || !strings.Contains(body, `dir="ltr"`) {
		t.Fatal("asking for English did not produce an English left-to-right page")
	}

	// It sticks without the parameter, which means the cookie was set and the
	// jar sent it back.
	_, body = h.get("/")
	if !strings.Contains(body, `lang="en"`) {
		t.Error("the language choice was not remembered")
	}

	// And back again.
	_, body = h.get("/?lang=fa")
	if !strings.Contains(body, `lang="fa"`) || !strings.Contains(body, `dir="rtl"`) {
		t.Error("switching back to Persian did not work")
	}

	// A value that is not a language is ignored rather than being an error: the
	// worst outcome of a bad parameter should be the default language, not a
	// page nobody can read.
	_, body = h.get("/?lang=klingon")
	if !strings.Contains(body, `lang="fa"`) {
		t.Error("an unknown language was not ignored")
	}

	// Nothing about the language reaches the state file.
	if data, err := os.ReadFile(h.store.Path()); err == nil {
		if strings.Contains(string(data), "lang") {
			t.Errorf("the language preference reached the state file: %s", data)
		}
	}
}

// TestNoRenderedPageHasAMissingMessage walks every page in every language.
//
// This is the test that catches a key used in a template or a handler that
// nobody added to a catalogue, which is the failure the marker exists for.
func TestNoRenderedPageHasAMissingMessage(t *testing.T) {
	for _, lang := range Langs {
		t.Run(string(lang), func(t *testing.T) {
			h := newHarness(t)
			h.get("/?lang=" + string(lang))
			h.lang = lang

			// Signed out: setup and login.
			for _, path := range []string{"/setup", "/login", "/", "/nope"} {
				_, body := h.get(path)
				if strings.Contains(body, missingMarker) {
					t.Errorf("%s %s has a missing message: %s", lang, path, findMarker(body))
				}
			}

			h.ready()
			h.priv.SetHotspot(HotspotStatus{Running: true, SSID: "Caspian-test", Devices: 2})

			// Every state that can put a message on the dashboard.
			h.postForm("/power", urlValues("csrf", h.tokenOn("/"), "on", "1"))
			h.postForm("/config", urlValues("csrf", h.tokenOn("/"), "config", unparseableConfig()))
			h.postForm("/advanced", urlValues("csrf", h.tokenOn("/"), "channel", "140"))
			h.postForm("/hotspot", urlValues("csrf", h.tokenOn("/"), "ssid", "", "passphrase", "x"))

			for _, path := range []string{"/", "/?advanced=1", "/status.json"} {
				_, body := h.get(path)
				if strings.Contains(body, missingMarker) {
					t.Errorf("%s %s has a missing message: %s", lang, path, findMarker(body))
				}
			}
		})
	}
}

// findMarker returns a little context around the first missing message, so the
// failure names the key instead of dumping the page.
func findMarker(body string) string {
	i := strings.Index(body, missingMarker)
	if i < 0 {
		return ""
	}
	end := i + 80
	if end > len(body) {
		end = len(body)
	}
	return body[i:end]
}

// TestNoTemplateCallsAFormatMessageWithNoArgument is the guard left behind by
// a defect that shipped and was visible on the page.
//
// Five menus in the advanced section offered an option reading "Let Caspian
// decide (now: %s)". The catalogue sentence takes an argument, the template
// called it with none, and Go's formatter has nothing to substitute, so the
// verb reached the browser. Nothing failed: the key existed, both languages
// had it, the page rendered, and the only symptom was two characters of
// nonsense inside a menu nobody opens until something has gone wrong.
//
// The check is mechanical because the mistake is: a call site with no argument
// cannot use a sentence that needs one. It reads the templates rather than the
// Go, since that is where a resolved key is easy to write and easy to get
// wrong.
func TestNoTemplateCallsAFormatMessageWithNoArgument(t *testing.T) {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		t.Fatal(err)
	}
	call := regexp.MustCompile(`\{\{-?\s*\.T\s+"([^"]+)"\s*-?\}\}`)
	verb := regexp.MustCompile(`%[sdvqt]`)

	seen := 0
	for _, e := range entries {
		body, err := templateFS.ReadFile("templates/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range call.FindAllStringSubmatch(string(body), -1) {
			key := Key(m[1])
			seen++
			for _, l := range Langs {
				raw, ok := messages[l][key]
				if !ok {
					t.Errorf("%s: %s has no %s message", e.Name(), key, l)
					continue
				}
				if verb.MatchString(raw) {
					t.Errorf("%s calls %s with no argument, but the %s sentence has a format verb in it: %q",
						e.Name(), key, l, raw)
				}
			}
		}
	}
	if seen == 0 {
		t.Fatal("no argument-free .T calls were found in any template, so this test checked nothing")
	}
	t.Logf("%d argument-free catalogue calls checked", seen)
}

// TestThePersianNameIsSpelledCorrectly guards a spelling that was wrong in
// seventy-eight places at once.
//
// The product is written کاسپین in Persian, with the alef. It was written
// کسپین everywhere, which is a different word, and it reached every sentence
// in the catalogue including the sign-in page and the help. A misspelled
// product name is not a typo in a product for people who are deciding whether
// to trust it: it is the first thing a reader sees, and it reads as carelessness
// about everything behind it.
//
// It is checked as an absence rather than a count, so a new sentence that
// misspells it fails on the day it is written rather than at the next review.
func TestThePersianNameIsSpelledCorrectly(t *testing.T) {
	const right = "کاسپین"
	const wrong = "کسپین"

	// The wrong spelling is not a substring of the right one: the correct form
	// has an alef the wrong one lacks, so a plain search for each is enough and
	// no counting is needed. An earlier version of this test assumed otherwise
	// and reported that no message mentioned the product at all.
	seen := 0
	for _, k := range keys(LangFA) {
		msg := messages[LangFA][k]
		if strings.Contains(msg, wrong) {
			t.Errorf("%s spells the name %q; in Persian it is %q: %s", k, wrong, right, msg)
		}
		if strings.Contains(msg, right) {
			seen++
		}
	}
	if seen == 0 {
		t.Fatal("no Persian message names the product, so this test checked nothing")
	}
	t.Logf("%d Persian messages name the product, all spelled %s", seen, right)
}

// The four engine phases must be four DIFFERENT words in every language.
//
// English rendered "failed" as "not running", which is also what a box that was
// never switched on looks like, so a reader could not tell a tunnel that tried
// and failed from one nobody had started. The Persian catalogue kept the
// distinction the whole time, which is how it was found: one language carried
// information the other had dropped.
//
// This is not a translation check. It is a check that no language quietly
// collapses two states of the product into one word.
func TestEveryEnginePhaseIsADistinctWordInEveryLanguage(t *testing.T) {
	phases := []Key{"phase.stopped", "phase.starting", "phase.running", "phase.failed"}
	for _, lang := range Langs {
		seen := map[string]Key{}
		for _, k := range phases {
			w := T(lang, k)
			if w == "" {
				t.Errorf("%s: %s renders empty", lang, k)
				continue
			}
			if prev, dup := seen[w]; dup {
				t.Errorf("%s: %s and %s both render %q, so the panel cannot tell those "+
					"two states apart in this language", lang, prev, k, w)
			}
			seen[w] = k
		}
	}
}

// TestLanguagePreferencePrecedenceAndCookie derives its valid cases from Langs:
// every language can be saved, survives an invalid query, and wins over every
// other saved language when asked for explicitly. The invalid cases use a code
// that is checked NOT to be in Langs, so a future language cannot turn them
// into accidental positives.
func TestLanguagePreferencePrecedenceAndCookie(t *testing.T) {
	type tc struct {
		name, query, cookie string
		want                Lang
		sets                bool
	}
	const invalid = "xx"
	if Lang(invalid).Valid() {
		t.Fatalf("%q is a real language in this build; the invalid cases need another code", invalid)
	}
	if len(Langs) < 2 {
		t.Fatalf("Langs is %v, so no case can show one language winning over another", Langs)
	}
	cases := []tc{
		{"fresh", "", "", DefaultLang, false},
		{"invalid query", invalid, "", DefaultLang, false},
		{"invalid cookie", "", invalid, DefaultLang, false},
	}
	for _, lang := range Langs {
		cases = append(cases,
			tc{"saved " + string(lang), "", string(lang), lang, false},
			tc{"invalid query preserves saved " + string(lang), invalid, string(lang), lang, false},
		)
		for _, other := range Langs {
			if other == lang {
				continue
			}
			cases = append(cases, tc{"explicit " + string(lang) + " wins over saved " + string(other), string(lang), string(other), lang, true})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, secure := range []bool{false, true} {
				p := &Panel{secureCookies: secure}
				r := httptest.NewRequest(http.MethodGet, "/help?lang="+tc.query, nil)
				r.Header.Set("Accept-Language", "fa")
				if tc.cookie != "" {
					r.AddCookie(&http.Cookie{Name: langCookie, Value: tc.cookie})
				}
				w := httptest.NewRecorder()
				if got := p.langFor(w, r); got != tc.want {
					t.Fatalf("got %s, want %s", got, tc.want)
				}
				cookies := w.Result().Cookies()
				if !tc.sets {
					if len(cookies) != 0 {
						t.Fatal("invalid or absent choice wrote cookie")
					}
					continue
				}
				if len(cookies) != 1 {
					t.Fatal("choice must write one cookie")
				}
				c := cookies[0]
				if c.Value != string(tc.want) || c.Path != "/" || !c.HttpOnly || c.Secure != secure || c.SameSite != http.SameSiteStrictMode || c.MaxAge <= 0 {
					t.Errorf("incorrect preference cookie: %+v", c)
				}
			}
		})
	}
}
