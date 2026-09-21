package structural

import (
	"os"
	"path/filepath"
	"regexp"
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

// scriptSrc is what a page names when it loads a script; stylesheetHref is what
// it names when it loads a stylesheet.
var (
	scriptSrc      = regexp.MustCompile(`<script src="([^"]+)"`)
	stylesheetHref = regexp.MustCompile(`<link rel="stylesheet" href="([^"]+)"`)
)

// loadedBy answers the files a page's tags name, by the given pattern.
func loadedBy(page string, tag *regexp.Regexp) map[string]bool {
	loaded := map[string]bool{}
	for _, match := range tag.FindAllStringSubmatch(page, -1) {
		loaded[match[1]] = true
	}
	return loaded
}

// TestEveryManagerScriptIsLoadedByThePage holds the halves of a page that is
// spread over several files to each other.
//
// Nothing compiles a page here, so a script or stylesheet no tag names is dead
// weight that nothing reports, while a tag naming a file that is not there is a
// window that comes up half wired or half styled. The manager's script was one
// file until it was split into seven and its stylesheet one until it was cut
// into four, which is what makes this worth guarding: the next one is the one
// that gets written and never loaded.
func TestEveryManagerScriptIsLoadedByThePage(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "frontend", "dist")
	page := readSource(t, filepath.Join(dir, "index.html"))
	for _, kind := range []struct {
		extension string
		tag       *regexp.Regexp
	}{{".js", scriptSrc}, {".css", stylesheetHref}} {
		everyFileIsLoaded(t, dir, kind.extension, loadedBy(page, kind.tag))
	}
}

// everyFileIsLoaded fails any file of the kind the page does not load. It also
// fails any tag naming a file that is not there.
func everyFileIsLoaded(t *testing.T, dir, extension string, loaded map[string]bool) {
	t.Helper()
	if len(loaded) == 0 {
		t.Fatalf("index.html loads no %s file at all, so this check would pass over anything", extension)
	}
	for name := range loaded {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("index.html loads %s, which is not in frontend/dist", name)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the manager page: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, extension) {
			continue
		}
		if !loaded[name] {
			t.Errorf("frontend/dist/%s is loaded by nothing: index.html has no tag for it", name)
		}
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
	dir := filepath.Join(root, "frontend", "dist")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the manager page: %v", err)
	}
	// Every script of the page, not one named file: the page grew a second one
	// and a test that knew only the first would have stopped guarding half of
	// it without saying so.
	var seen int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		seen++
		text := readSource(t, filepath.Join(dir, entry.Name()))
		for _, match := range stateField.FindAllStringSubmatch(text, -1) {
			if !tags[match[1]] {
				t.Errorf("%s reads state.%s, which the agent never sends",
					entry.Name(), match[1])
			}
		}
		for _, match := range boundCall.FindAllStringSubmatch(text, -1) {
			if !bound[match[1]] {
				t.Errorf("%s calls %s, which the agent does not bind",
					entry.Name(), match[1])
			}
		}
	}
	if seen == 0 {
		t.Fatal("no manager scripts were found, the walk is wrong")
	}
}

// managerMethods is the agent's bound surface, which is spread over the root
// package's files by subject: the window plumbing, the profile work and About.
func managerMethods(t *testing.T, root string) map[string]bool {
	t.Helper()
	methods := map[string]bool{}
	// Every file of the root package, rather than a list of names. The facade is
	// split by subject and gains a file whenever a new one arrives: a named list
	// went stale the first time that happened and reported two bound methods as
	// unbound.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading %s: %v", root, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		for method := range exportedMethodsOf(t, filepath.Join(root, name), "App") {
			methods[method] = true
		}
	}
	if len(methods) == 0 {
		t.Fatal("no method of App was found, so this check would pass over anything")
	}
	return methods
}
