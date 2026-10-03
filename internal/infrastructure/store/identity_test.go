package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/oernster/ScreenState/internal/application"
)

// moveFile renames a file inside the store, the way Explorer does.
func moveFile(t *testing.T, store *Store, from, to string) {
	t.Helper()
	if err := os.Rename(filepath.Join(store.Directory(), from), filepath.Join(store.Directory(), to)); err != nil {
		t.Fatalf("renaming %s: %v", from, err)
	}
}

// S-1: a profile file renamed in Explorer holds a name its file name no longer
// says. It is named under the list as a file the store cannot offer, never kept
// out of sight while still applied at sign-in; marking another profile works.
func TestARenamedFileIsNamedRatherThanHidden(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()
	if err := store.Save(ctx, named(t, "Work").WithDefault(true)); err != nil {
		t.Fatalf("saving Work: %v", err)
	}
	if err := store.Save(ctx, named(t, "Home")); err != nil {
		t.Fatalf("saving Home: %v", err)
	}
	moveFile(t, store, "work.json", "desk.json")

	names, err := store.Names(ctx)
	if err != nil || len(names) != 1 || names[0] != "Home" {
		t.Fatalf("listed %v (%v)", names, err)
	}
	excluded, err := store.Unreadable(ctx)
	if err != nil || len(excluded) != 1 || excluded[0].File != "desk.json" {
		t.Fatalf("left out %+v (%v)", excluded, err)
	}
	if !strings.Contains(excluded[0].Reason, `"Work"`) {
		t.Fatalf("the reason does not name what the file holds: %q", excluded[0].Reason)
	}
	if marked, held, err := store.Default(ctx); err != nil || held {
		t.Fatalf("a file the manager cannot show is still the default: %q held=%v (%v)", marked.Name, held, err)
	}
	if err := store.Save(ctx, named(t, "Home").WithDefault(true)); err != nil {
		t.Fatalf("marking Home: %v", err)
	}
	if marked, held, err := store.Default(ctx); err != nil || !held || marked.Name != "Home" {
		t.Fatalf("the default is %q held=%v (%v)", marked.Name, held, err)
	}
	if _, err := store.Load(ctx, "Work"); err == nil {
		t.Fatal("a profile the list does not show was loaded")
	}
}

// S-2: Explorer's copy of a profile file holds the same name in a second file.
// Marking another profile leaves sign-in applying that one, not the copy.
func TestACopiedFileDoesNotKeepTheDefault(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()
	if err := store.Save(ctx, named(t, "Work").WithDefault(true)); err != nil {
		t.Fatalf("saving Work: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(store.Directory(), "work.json"))
	if err != nil {
		t.Fatalf("reading Work: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.Directory(), "work - Copy.json"), raw, 0o644); err != nil {
		t.Fatalf("copying Work: %v", err)
	}
	if err := store.Save(ctx, named(t, "Zebra").WithDefault(true)); err != nil {
		t.Fatalf("marking Zebra: %v", err)
	}
	marked, held, err := store.Default(ctx)
	if err != nil || !held || marked.Name != "Zebra" {
		t.Fatalf("sign-in would apply %q held=%v (%v)", marked.Name, held, err)
	}
	if names, _ := store.Names(ctx); len(names) != 2 {
		t.Fatalf("listed %v", names)
	}
}

// Two files whose names both say one profile (the current file name and the
// one an earlier build gave it) are both named rather than one picked in
// silence.
func TestTwoFilesHoldingOneNameAreBothNamed(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()
	const name = "Σx"
	if escape(name) == legacyEscape(name) {
		t.Fatalf("%q is named the same both ways, so it cannot make two files", name)
	}
	if err := store.Save(ctx, named(t, name)); err != nil {
		t.Fatalf("saving: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(store.Directory(), escape(name)+extension))
	if err != nil {
		t.Fatalf("reading it: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.Directory(), legacyEscape(name)+extension), raw, 0o644); err != nil {
		t.Fatalf("copying it: %v", err)
	}
	if _, err := store.Load(ctx, name); err == nil {
		t.Fatal("one of two files holding one name was loaded")
	}
	if names, _ := store.Names(ctx); len(names) != 0 {
		t.Fatalf("listed %v", names)
	}
	if excluded, _ := store.Unreadable(ctx); len(excluded) != 2 {
		t.Fatalf("left out %+v", excluded)
	}
}

// S-3: two names the manager treats as different never share a file, so saving
// one never writes over the other (FR-002).
func TestTwoDifferentNamesNeverShareAFile(t *testing.T) {
	t.Parallel()
	for _, pair := range [][2]string{{"Iş", "İş"}, {"ὠ0", "😀"}} {
		if strings.EqualFold(pair[0], pair[1]) {
			t.Fatalf("%q and %q are one name to the manager", pair[0], pair[1])
		}
		store, _ := storeUnder(t)
		ctx := context.Background()
		for _, name := range pair {
			if err := store.Save(ctx, named(t, name)); err != nil {
				t.Fatalf("saving %q: %v", name, err)
			}
		}
		names, err := store.Names(ctx)
		if err != nil || len(names) != 2 {
			t.Fatalf("after saving %q and %q the store lists %v (%v)", pair[0], pair[1], names, err)
		}
	}
}

// The file name is one to one with the name as the manager compares it: two
// names share a file exactly when strings.EqualFold says they are one name.
func TestTheFileNameFollowsTheNameComparison(t *testing.T) {
	t.Parallel()
	samples := []rune{'a', 'A', 'k', 'K', 'K', 's', 'ſ', 'S', 'σ', 'ς', 'Σ', 'İ', 'I', 'i', 'ı',
		'ὠ', 'Ὠ', '0', '😀', 'ß', 'ẞ', 'ǅ', 'Ǆ', 'ǆ', '%', '+', ' ', unicode.ReplacementChar}
	for _, one := range samples {
		for _, two := range samples {
			left, right := string(one)+"x", string(two)+"x"
			same := escape(left) == escape(right)
			if same != strings.EqualFold(left, right) {
				t.Errorf("%q and %q: one file %v, one name %v", left, right, same, !same)
			}
		}
	}
	if escape("ὠ0") == escape("😀") {
		t.Fatal("a letter followed by a digit reads as one wider letter")
	}
}

// A file saved by an earlier build under the earlier file name is still the
// profile it holds; replacing it writes the same file rather than a second.
func TestAFileNamedTheEarlierWayIsStillItsProfile(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()
	if err := store.Save(ctx, named(t, "Ωmega")); err != nil {
		t.Fatalf("saving: %v", err)
	}
	moveFile(t, store, escape("Ωmega")+extension, legacyEscape("Ωmega")+extension)
	if _, err := store.Load(ctx, "ωMEGA"); err != nil {
		t.Fatalf("loading the earlier file: %v", err)
	}
	if err := store.Save(ctx, named(t, "Ωmega").WithDefault(true)); err != nil {
		t.Fatalf("replacing it: %v", err)
	}
	entries, _ := os.ReadDir(store.Directory())
	if len(entries) != 1 || entries[0].Name() != legacyEscape("Ωmega")+extension {
		t.Fatalf("the store holds %v", entries)
	}
	if err := store.Delete(ctx, "Ωmega"); err != nil {
		t.Fatalf("deleting it: %v", err)
	}
}

// A new profile is never written over a file that already sits at its file
// name holding something else, whether another profile or a file the store
// cannot read (DATA-003: such a file is left exactly as it is).
func TestANewProfileNeverWritesOverAnotherFile(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()
	future := []byte(`{"format":99,"name":"Work","entries":[]}`)
	path := filepath.Join(store.Directory(), "work.json")
	if err := os.WriteFile(path, future, 0o644); err != nil {
		t.Fatalf("planting: %v", err)
	}
	if err := store.Save(ctx, named(t, "Work")); err == nil {
		t.Fatal("a file from a newer format was written over")
	}
	if after, _ := os.ReadFile(path); string(after) != string(future) {
		t.Fatalf("the file now reads %q", after)
	}

	other, _ := storeUnder(t)
	if err := other.Save(ctx, named(t, "Iş")); err != nil {
		t.Fatalf("saving: %v", err)
	}
	moveFile(t, other, escape("Iş")+extension, escape("Iz")+extension)
	err := other.Save(ctx, named(t, "Iz"))
	if !errors.Is(err, application.ErrProfileNameInUse) {
		t.Fatalf("saving over a file holding another name answered %v", err)
	}
}
