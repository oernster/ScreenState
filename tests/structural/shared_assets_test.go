package structural

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sharedAssets are the files with one home in assets/ and a copy inside each
// window's page directory. Each window embeds its own page, so a copy is the
// only way to share them; build.ps1 refreshes the copies on every build.
var sharedAssets = []string{"theme.css", "shell.js"}

// pageDirs are the page directories a copy lands in.
var pageDirs = []string{
	filepath.Join("frontend", "dist"),
	filepath.Join("installer", "frontend", "dist"),
}

// TestTheSharedAssetsHaveNotDrifted holds the one thing a copy cannot hold by
// itself.
//
// The palette and the page furniture are shared by the manager and the setup
// program, which is what makes the two read as one product. Nothing stops
// somebody editing a copy in place: it would work, it would look right in that
// one window and the two would quietly stop matching. This is the only guard
// against that, so a failure here means the copy was edited rather than the
// file in assets/.
func TestTheSharedAssetsHaveNotDrifted(t *testing.T) {
	root := repoRoot(t)
	for _, name := range sharedAssets {
		source := readSource(t, filepath.Join(root, "assets", name))
		if strings.TrimSpace(source) == "" {
			t.Fatalf("assets/%s is empty, so this test is guarding nothing", name)
		}
		for _, dir := range pageDirs {
			path := filepath.Join(root, dir, name)
			if _, err := os.Stat(path); err != nil {
				t.Errorf("%s is missing: build.ps1 copies it from assets/%s",
					filepath.ToSlash(filepath.Join(dir, name)), name)
				continue
			}
			if readSource(t, path) != source {
				t.Errorf("%s has drifted from assets/%s: edit the one in assets/, never a copy",
					filepath.ToSlash(filepath.Join(dir, name)), name)
			}
		}
	}
}

// TestTheManagerPageNamesNothing holds for the manager what the setup page is
// already held to: a page has no compiler behind it, so a product name written
// into one survives a rename in silence.
func TestTheManagerPageNamesNothing(t *testing.T) {
	name := productNameValue(t)
	tagline := productTaglineValue(t)
	root := repoRoot(t)
	dir := filepath.Join(root, "frontend", "dist")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the manager page: %v", err)
	}
	var seen int
	for _, entry := range entries {
		if entry.IsDir() || !isPageSource(entry.Name()) {
			continue
		}
		seen++
		text := readSource(t, filepath.Join(dir, entry.Name()))
		for label, forbidden := range map[string]string{
			"the product's name": name,
			"the tagline":        tagline,
		} {
			if strings.Contains(text, forbidden) {
				t.Errorf("frontend/dist/%s writes %s down: it arrives on the state instead",
					entry.Name(), label)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no manager page files were found, the walk is wrong")
	}
}

// isPageSource reports whether a file is part of a page rather than artwork.
func isPageSource(name string) bool {
	return strings.HasSuffix(name, ".html") ||
		strings.HasSuffix(name, ".js") ||
		strings.HasSuffix(name, ".css")
}

// TestTheManagerWireIsStatedTwiceAndAgrees compares the two statements of the
// manager's boundary, exactly as the setup program's is compared: the program
// marshals a struct and the page reads a name off an object; nothing at all
// checks the page.
func TestTheManagerWireIsStatedTwiceAndAgrees(t *testing.T) {
	root := repoRoot(t)
	tags := jsonTagsOf(t, filepath.Join(root, "app.go"), "StateDTO")
	bound := managerMethods(t, root)
	path := filepath.Join(root, "frontend", "dist", "manager.js")
	text := readSource(t, path)

	for _, match := range stateField.FindAllStringSubmatch(text, -1) {
		if !tags[match[1]] {
			t.Errorf("manager.js reads state.%s, which the agent never sends", match[1])
		}
	}
	for _, match := range boundCall.FindAllStringSubmatch(text, -1) {
		if !bound[match[1]] {
			t.Errorf("manager.js calls %s, which the agent does not bind", match[1])
		}
	}
}

// managerMethods is the agent's bound surface, which is spread over two files
// because the window plumbing and the profile work are different concerns.
func managerMethods(t *testing.T, root string) map[string]bool {
	t.Helper()
	methods := map[string]bool{}
	for _, name := range []string{"app.go", "app_profiles.go"} {
		for method := range exportedMethodsOf(t, filepath.Join(root, name), "App") {
			methods[method] = true
		}
	}
	return methods
}
