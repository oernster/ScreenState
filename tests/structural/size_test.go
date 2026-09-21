package structural

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The module-size rule (NFR-MAINT-003), over every file a person writes and
// reads: the Go source and the pages, whose scripts, stylesheets and markup no
// compiler ever looks at. The pages were outside it until 2026-09-21, which is
// how the manager's stylesheet reached 617 lines with nothing saying so.
//
// Build and packaging scripts are exempt from the rule. Here they are
// PowerShell and Python, which the walk does not reach, so the exemption needs
// no list of its own.

// lineLimit is the module-size cap. dangerBand is five per cent below it: a file
// that lands between them is refactored down to safeLanding rather than left one
// edit away from breaching, because shaving a line or two buys nothing.
const (
	lineLimit   = 400
	dangerBand  = lineLimit - lineLimit/20
	safeLanding = 350
)

// pageExtensions are the kinds of file a page is made of.
var pageExtensions = []string{".html", ".js", ".css"}

// notWritten names directories whose files nobody writes: the bindings Wails
// generates, the outputs of a build and anything a package manager fetched.
var notWritten = map[string]bool{
	".git": true, "venv": true, "node_modules": true, "wailsjs": true,
	"build": true, "dist-installer": true, ".claude": true,
}

// writtenPageFiles returns every page file in the repository: the manager's, the
// setup program's, the shared masters in assets/ and the site.
func writtenPageFiles(t *testing.T) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(repoRoot(t), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if notWritten[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		for _, extension := range pageExtensions {
			if strings.HasSuffix(path, extension) {
				found = append(found, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repository: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("no page files found, the walk is wrong")
	}
	return found
}

// sizedFiles is everything the size rule governs.
func sizedFiles(t *testing.T) []string {
	t.Helper()
	return append(goFiles(t), writtenPageFiles(t)...)
}

// lineCount counts the lines in a file.
func lineCount(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return strings.Count(string(raw), "\n") + 1
}

func TestNoFileExceedsLineLimit(t *testing.T) {
	for _, path := range sizedFiles(t) {
		if count := lineCount(t, path); count > lineLimit {
			t.Errorf("%s has %d lines, over the %d limit", path, count, lineLimit)
		}
	}
}

func TestNoFileInDangerBand(t *testing.T) {
	for _, path := range sizedFiles(t) {
		count := lineCount(t, path)
		if count > dangerBand && count <= lineLimit {
			t.Errorf(
				"%s has %d lines, inside the danger band %d to %d: reduce it to %d or fewer",
				path, count, dangerBand+1, lineLimit, safeLanding,
			)
		}
	}
}
