package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// named returns a small valid profile of that name.
func named(t *testing.T, name string) domain.Profile {
	t.Helper()
	profile, err := domain.NewProfile(name,
		domain.Entry{Application: domain.ApplicationIdentity{Value: `C:\a.exe`}, Running: true})
	if err != nil {
		t.Fatalf("building %q: %v", name, err)
	}
	return profile
}

// FR-040: marking a profile as the default clears the mark from every other.
func TestMarkingADefaultClearsTheOthers(t *testing.T) {
	t.Parallel()
	store, log := storeUnder(t)
	ctx := context.Background()

	if err := store.Save(ctx, named(t, "Desk").WithDefault(true)); err != nil {
		t.Fatalf("saving Desk: %v", err)
	}
	if err := store.Save(ctx, named(t, "Sofa").WithDefault(true)); err != nil {
		t.Fatalf("saving Sofa: %v", err)
	}

	marked, held, err := store.Default(ctx)
	if err != nil || !held {
		t.Fatalf("held=%v err=%v", held, err)
	}
	if marked.Name != "Sofa" {
		t.Fatalf("the default is %q", marked.Name)
	}
	desk, err := store.Load(ctx, "Desk")
	if err != nil {
		t.Fatalf("loading Desk: %v", err)
	}
	if desk.Default {
		t.Fatal("two profiles are marked as the default")
	}
	if !log.saying(`profile "Desk" is no longer the default`) {
		t.Fatal("the change of default was not recorded")
	}
}

// FR-039: no default is an answer rather than a fault.
func TestNoDefaultIsAnAnswer(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	if err := store.Save(ctx, named(t, "Desk")); err != nil {
		t.Fatalf("saving: %v", err)
	}
	profile, held, err := store.Default(ctx)
	if err != nil {
		t.Fatalf("reading the default: %v", err)
	}
	if held {
		t.Fatalf("a default was reported: %q", profile.Name)
	}
}

// A store edited by hand can hold two defaults although FR-040 prevents the
// product from writing them. One is chosen, the same one every time; the
// disagreement is recorded rather than passed over.
func TestTwoDefaultsAreSettledAndRecorded(t *testing.T) {
	t.Parallel()
	store, log := storeUnder(t)
	ctx := context.Background()

	for _, name := range []string{"Sofa", "Desk"} {
		raw, err := encode(named(t, name).WithDefault(true))
		if err != nil {
			t.Fatalf("encoding %q: %v", name, err)
		}
		if err := os.WriteFile(store.pathFor(name), raw, 0o644); err != nil {
			t.Fatalf("planting %q: %v", name, err)
		}
	}
	marked, held, err := store.Default(ctx)
	if err != nil || !held {
		t.Fatalf("held=%v err=%v", held, err)
	}
	if marked.Name != "Desk" {
		t.Fatalf("the default read as %q", marked.Name)
	}
	if !log.saying("2 profiles are marked as the default") {
		t.Fatal("the disagreement was not recorded")
	}
}

// DATA-003: a profile written in a format this build does not understand is
// left exactly as it is, kept out of the list, with the reason stated.
func TestAProfileFromANewerFormatIsLeftAlone(t *testing.T) {
	t.Parallel()
	store, log := storeUnder(t)
	ctx := context.Background()

	future := []byte(`{"format":99,"name":"Future","entries":[]}` + "\n")
	path := filepath.Join(store.Directory(), "future.json")
	if err := os.WriteFile(path, future, 0o644); err != nil {
		t.Fatalf("planting: %v", err)
	}
	if err := store.Save(ctx, named(t, "Desk")); err != nil {
		t.Fatalf("saving: %v", err)
	}

	names, err := store.Names(ctx)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(names) != 1 || names[0] != "Desk" {
		t.Fatalf("the listing holds %v", names)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	if string(after) != string(future) {
		t.Fatalf("the file was changed to %q", after)
	}
	excluded, err := store.Excluded(ctx)
	if err != nil {
		t.Fatalf("listing what was left out: %v", err)
	}
	if len(excluded) != 1 || excluded[0].File != "future.json" {
		t.Fatalf("left out %+v", excluded)
	}
	if !strings.Contains(excluded[0].Reason, "newer format") {
		t.Fatalf("the reason reads %q", excluded[0].Reason)
	}
	// The store records the exclusion rather than narrating it. Listing runs
	// several times a run, so the log line belongs to whoever asked, once.
	if log.saying("future.json") {
		t.Fatal("the store narrated an exclusion it only needed to record")
	}
}

// A file that is not a profile costs the user that file and nothing else.
func TestABrokenFileCostsOnlyItself(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	for name, contents := range map[string]string{
		"broken.json":    `{"format":1,"name":"Broken",`,
		"nameless.json":  `{"format":1,"name":"   ","entries":[]}`,
		"badkind.json":   `{"format":1,"name":"Bad","entries":[{"application":{"kind":"registry","value":"x"}}]}`,
		"badstate.json":  `{"format":1,"name":"Bad","entries":[{"application":{"kind":"path","value":"C:\\a.exe"},"placements":[{"display":"D","rect":{"x":0,"y":0,"width":1,"height":1},"state":"folded"}]}]}`,
		"emptyrect.json": `{"format":1,"name":"Bad","entries":[{"application":{"kind":"path","value":"C:\\a.exe"},"placements":[{"display":"D","rect":{"x":0,"y":0,"width":0,"height":0},"state":"normal"}]}]}`,
		"nodisplay.json": `{"format":1,"name":"Bad","entries":[{"application":{"kind":"path","value":"C:\\a.exe"},"placements":[{"display":"","rect":{"x":0,"y":0,"width":1,"height":1},"state":"normal"}]}]}`,
		"novalue.json":   `{"format":1,"name":"Bad","entries":[{"application":{"kind":"path","value":""}}]}`,
	} {
		if err := os.WriteFile(filepath.Join(store.Directory(), name), []byte(contents), 0o644); err != nil {
			t.Fatalf("planting %s: %v", name, err)
		}
	}
	if err := store.Save(ctx, named(t, "Desk")); err != nil {
		t.Fatalf("saving: %v", err)
	}

	names, err := store.Names(ctx)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(names) != 1 || names[0] != "Desk" {
		t.Fatalf("the listing holds %v", names)
	}
	excluded, err := store.Excluded(ctx)
	if err != nil || len(excluded) != 7 {
		t.Fatalf("left out %d files (%v)", len(excluded), err)
	}
}

// FR-006: an interruption leaves the previous profile or the new one, never a
// mixture. The last step of the write is made to fail, which is the only moment
// at which the target file is touched at all.
func TestAnInterruptedWriteLeavesThePreviousProfile(t *testing.T) {
	store, _ := storeUnder(t)
	ctx := context.Background()

	original := deskProfile(t)
	if err := store.Save(ctx, original); err != nil {
		t.Fatalf("saving: %v", err)
	}
	before, err := os.ReadFile(store.pathFor("Desk"))
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}

	interrupted := errors.New("interrupted")
	previous := rename
	rename = func(string, string) error { return interrupted }
	defer func() { rename = previous }()

	replacement := named(t, "Desk")
	if err := store.Save(ctx, replacement); !errors.Is(err, interrupted) {
		t.Fatalf("the interrupted save answered %v", err)
	}

	after, err := os.ReadFile(store.pathFor("Desk"))
	if err != nil {
		t.Fatalf("the previous profile is gone: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("the previous profile was changed by a write that failed")
	}
	read, err := store.Load(ctx, "Desk")
	if err != nil || len(read.Entries) != len(original.Entries) {
		t.Fatalf("the previous profile no longer loads: %+v (%v)", read, err)
	}
	// Nothing half written is left behind for the next listing to find.
	entries, err := os.ReadDir(store.Directory())
	if err != nil {
		t.Fatalf("reading the store: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("the store holds %d files after a failed write", len(entries))
	}
}

// A profile the domain would refuse never reaches the disk.
func TestAnInvalidProfileIsNotWritten(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	if err := store.Save(ctx, domain.Profile{Name: "  "}); !errors.Is(err, domain.ErrEmptyProfileName) {
		t.Fatalf("saving answered %v", err)
	}
	if names, _ := store.Names(ctx); len(names) != 0 {
		t.Fatalf("the store holds %v", names)
	}
}

// DATA-001 and C-2: the store sits under the signed-in user's own local
// application data directory.
func TestTheStoreSitsUnderThisUsersOwnDirectory(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)

	directory, err := DefaultDirectory()
	if err != nil {
		t.Fatalf("finding the store: %v", err)
	}
	if !strings.HasPrefix(directory, local) {
		t.Fatalf("the store would sit at %s, outside %s", directory, local)
	}
	if filepath.Base(directory) != "profiles" || !strings.Contains(directory, "ScreenState") {
		t.Fatalf("the store would sit at %s", directory)
	}

	// With nothing naming a location, the answer is a place under this
	// product's own name or a reason there is none. It is never a bare relative
	// path, which would put a user's profiles wherever the agent happened to be
	// started from. On Windows the fallback reads the same variable that was
	// just cleared, so a refusal is the right answer there.
	t.Setenv("LOCALAPPDATA", "")
	fallback, err := DefaultDirectory()
	if err != nil {
		if fallback != "" {
			t.Fatalf("a refusal still answered the path %q", fallback)
		}
		return
	}
	if !filepath.IsAbs(fallback) || !strings.Contains(fallback, "ScreenState") {
		t.Fatalf("the fallback would sit at %s", fallback)
	}
}

// A store whose directory cannot be made or read says so rather than answering
// an empty list, which would read as a user with no profiles.
func TestAStoreThatCannotBeReadSaysSo(t *testing.T) {
	t.Parallel()
	log := &recordingLog{}
	occupied := filepath.Join(t.TempDir(), "occupied")
	if err := os.WriteFile(occupied, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("planting: %v", err)
	}
	if _, err := New(occupied, log); err == nil {
		t.Fatal("a store was opened over a file")
	}

	store, _ := storeUnder(t)
	if err := os.RemoveAll(store.Directory()); err != nil {
		t.Fatalf("removing the store: %v", err)
	}
	if _, err := store.Names(context.Background()); err == nil {
		t.Fatal("listing a store that is not there answered no error")
	}
}
