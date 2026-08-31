package goldenscan

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestEveryLocalLinkResolves opens every markdown link and image that points at
// a file in this repository and checks the file is there.
//
// Documentation links rot in one direction: the person who writes the link can
// see the target, and the person who follows it a month later cannot. Nothing
// about reading a document reveals that a link is dead, so this is checked
// rather than reviewed. Two of the faults this catches have already happened
// here: a document renamed without its referrers being updated, and a set of
// links written against files that had not been created yet.
//
// Remote links are NOT fetched. A test that reaches the network is a test that
// fails when a train goes into a tunnel, and this suite is meant to be runnable
// on a box with no internet at all.
func TestEveryLocalLinkResolves(t *testing.T) {
	root := moduleRoot(t)
	link := regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

	var checked, broken int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "node_modules" || base == "local" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("reading %s: %v", rel, readErr)
			return nil
		}

		var missing []string
		for _, m := range link.FindAllStringSubmatch(string(body), -1) {
			target := strings.TrimSpace(m[1])
			// Remote, mail and pure in-page anchors are out of scope.
			if target == "" || strings.HasPrefix(target, "#") {
				continue
			}
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			file := target
			if i := strings.IndexByte(file, '#'); i >= 0 {
				file = file[:i]
			}
			if file == "" {
				continue
			}
			checked++
			if _, statErr := os.Stat(filepath.Join(filepath.Dir(path), file)); statErr != nil {
				missing = append(missing, target)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			broken += len(missing)
			t.Errorf("%s links to %d file(s) that do not exist: %s",
				rel, len(missing), strings.Join(missing, ", "))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
	if checked < 40 {
		t.Errorf("only %d local links were checked, which is fewer than this repository "+
			"has. The walk is not finding them.", checked)
	}
	t.Logf("checked %d local links, %d broken", checked, broken)
}
