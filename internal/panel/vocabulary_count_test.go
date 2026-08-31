// SPDX-License-Identifier: AGPL-3.0-or-later

package panel

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// No comment may state how many privileged actions there are.
//
// Five separate comments said "five" while there were eight: priv.go, doc.go,
// privsvc/doc.go and wire.go twice. Three actions had been added over time and
// nobody went back to the sentences, because nothing made them.
//
// TestActionVocabularyMatchesTheInterface already checks the action list
// against the interface, so both of those move together. Neither is compared to
// the prose, and prose cannot be. So the rule is not "keep the number right",
// which is the thing that already failed five times. The rule is that a number
// nothing can check does not get written down.
//
// A reader who wants the count reads panel.Actions, which is the list itself.
func TestNoCommentCountsTheActions(t *testing.T) {
	// Two phrasings, both taken from the comments that were actually wrong:
	// a number sitting directly on the noun ("five named actions", "five
	// methods"), and the set referred to by its size ("one of the five").
	//
	// The first version of this allowed 40 characters between the number and
	// the noun, and caught two innocent sentences: "three different actions
	// from the user", which is about what the READER does, and "one of the
	// three shapes ... decided by the action", which counts response shapes.
	// Both are counting something that does not grow. Requiring the number to
	// sit on the noun separates them, and it is the shape the real defect had
	// in all five places.
	const num = `(three|four|five|six|seven|eight|nine|ten)`
	// In the second form the number must STAND ALONE as the size of the set,
	// so it is followed by punctuation rather than by another noun. "one of
	// the five." is the defect; "one of the three shapes" counts response
	// shapes, which do not grow, and is fine.
	counting := regexp.MustCompile(`(?i)\b` + num + `\s+(named\s+)?(actions?|methods?|verbs?)\b` +
		`|(?i)\bone of the\s+` + num + `\s*[.,;)]`)

	for _, path := range []string{
		"priv.go", "doc.go",
		"../privsvc/doc.go", "../privsvc/wire.go", "../privsvc/server.go",
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if !strings.Contains(line, "//") {
				continue
			}
			if m := counting.FindString(line); m != "" {
				t.Errorf("%s:%d says %q. The set of actions changes; a comment counting it "+
					"does not, and five comments already said \"five\" while there were eight. "+
					"Name panel.Actions instead of counting it.", path, i+1, strings.TrimSpace(m))
			}
		}
	}
}
