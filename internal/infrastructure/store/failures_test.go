package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
)

// A cancelled context stops every call before it touches the disk, so a user
// who closes the manager mid-save does not leave a half-finished store.
func TestACancelledContextStopsEveryCall(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := store.Names(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Names answered %v", err)
	}
	if _, err := store.Load(ctx, "Desk"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Load answered %v", err)
	}
	if err := store.Save(ctx, named(t, "Desk")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save answered %v", err)
	}
	if err := store.Delete(ctx, "Desk"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete answered %v", err)
	}
	if _, _, err := store.Default(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Default answered %v", err)
	}
	if _, err := store.Unreadable(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Unreadable answered %v", err)
	}
	if entries, _ := os.ReadDir(store.Directory()); len(entries) != 0 {
		t.Fatalf("a cancelled call wrote %d file(s)", len(entries))
	}
}

// A profile that is there but cannot be read is a different answer from a
// profile that is not there; the caller is told which.
func TestAProfileThatIsThereButBrokenIsNotMissing(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	if err := os.WriteFile(store.pathFor("Desk"), []byte(`{"format":1,`), 0o644); err != nil {
		t.Fatalf("planting: %v", err)
	}
	_, err := store.Load(ctx, "Desk")
	if errors.Is(err, application.ErrNoSuchProfile) {
		t.Fatal("a broken profile was reported as missing")
	}
	if !errors.Is(err, ErrUnreadable) {
		t.Fatalf("answered %v", err)
	}
}

// A store that has gone away under the agent says so on every path, rather than
// reporting a user with no profiles.
func TestAStoreThatHasGoneAwaySaysSoEverywhere(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()
	if err := os.RemoveAll(store.Directory()); err != nil {
		t.Fatalf("removing the store: %v", err)
	}

	if err := store.Save(ctx, named(t, "Desk")); err == nil {
		t.Fatal("saving into a store that is not there answered no error")
	}
	if _, _, err := store.Default(ctx); err == nil {
		t.Fatal("reading the default answered no error")
	}
}

// Deleting something that is not a profile file answers the reason rather than
// claiming the profile was never there.
func TestADeleteThatCannotHappenSaysWhy(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	occupied := store.pathFor("Desk")
	if err := os.Mkdir(occupied, 0o755); err != nil {
		t.Fatalf("planting: %v", err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "held"), []byte("x"), 0o644); err != nil {
		t.Fatalf("planting: %v", err)
	}
	err := store.Delete(ctx, "Desk")
	if err == nil {
		t.Fatal("deleting answered no error")
	}
	if errors.Is(err, application.ErrNoSuchProfile) {
		t.Fatal("a profile that could not be deleted was reported as missing")
	}
}

// FR-040 again, at the moment it goes wrong: the profile just marked as the
// default is written first, so a failure while unmarking the others is reported
// rather than swallowed; the new default is not lost.
func TestAFailureWhileClearingTheOldDefaultIsReported(t *testing.T) {
	store, _ := storeUnder(t)
	ctx := context.Background()

	if err := store.Save(ctx, named(t, "Desk").WithDefault(true)); err != nil {
		t.Fatalf("saving Desk: %v", err)
	}

	interrupted := errors.New("interrupted")
	previous := rename
	attempts := 0
	rename = func(from, to string) error {
		attempts++
		if attempts == 1 {
			return previous(from, to)
		}
		return interrupted
	}
	defer func() { rename = previous }()

	err := store.Save(ctx, named(t, "Sofa").WithDefault(true))
	if !errors.Is(err, interrupted) {
		t.Fatalf("the interrupted save answered %v", err)
	}
	rename = previous

	sofa, err := store.Load(ctx, "Sofa")
	if err != nil || !sofa.Default {
		t.Fatalf("the new default was lost: %+v (%v)", sofa, err)
	}
}

// A directory inside the store is not a profile and is stepped over.
func TestADirectoryInTheStoreIsNotAProfile(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	if err := os.Mkdir(filepath.Join(store.Directory(), "backup.json"), 0o755); err != nil {
		t.Fatalf("planting: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.Directory(), "notes.txt"), []byte("x"), 0o644); err != nil {
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
	excluded, err := store.Unreadable(ctx)
	if err != nil || len(excluded) != 0 {
		t.Fatalf("left out %+v (%v)", excluded, err)
	}
}

// The store is what the application layer asked for. This is checked by the
// compiler rather than by a reader comparing two files.
var _ application.ProfileStore = (*Store)(nil)

// A profile with no entries is valid: it says the user wants nothing arranged,
// which is different from having no profile.
func TestAnEmptyProfileIsAProfile(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	empty, err := domain.NewProfile("Blank")
	if err != nil {
		t.Fatalf("building it: %v", err)
	}
	if err := store.Save(ctx, empty); err != nil {
		t.Fatalf("saving: %v", err)
	}
	read, err := store.Load(ctx, "Blank")
	if err != nil || read.Name != "Blank" || len(read.Entries) != 0 {
		t.Fatalf("read back as %+v (%v)", read, err)
	}
}
