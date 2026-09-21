package application

import (
	"context"
	"errors"
	"testing"
)

func TestRenameKeepsTheEntriesAndTheDefaultMarking(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Work", pigeonpost, stellody).WithDefault(true))
	manager, _, log := managerOver(store)

	if err := manager.Rename(context.Background(), "Work", "Office"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	renamed, err := store.Load(context.Background(), "Office")
	if err != nil {
		t.Fatalf("loading the renamed profile: %v", err)
	}
	if len(renamed.Entries) != 2 {
		t.Errorf("the renamed profile holds %d entries, want 2", len(renamed.Entries))
	}
	if !renamed.Default {
		t.Error("the renamed profile lost its default marking")
	}
	if _, err := store.Load(context.Background(), "Work"); !errors.Is(err, ErrNoSuchProfile) {
		t.Error("the old profile is still there")
	}
	if !log.saying("was renamed to") {
		t.Error("the rename was not recorded")
	}
}

// TestRenameThatOnlyChangesCaseKeepsTheProfile is the one that would destroy a
// profile if it were got wrong. The store addresses a profile by its lowercased
// name, so the old file IS the new file: deleting the old name afterwards would
// delete what was just written.
func TestRenameThatOnlyChangesCaseKeepsTheProfile(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Work", pigeonpost))
	manager, _, _ := managerOver(store)

	if err := manager.Rename(context.Background(), "Work", "WORK"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	renamed, err := store.Load(context.Background(), "WORK")
	if err != nil {
		t.Fatalf("the profile did not survive a change of case: %v", err)
	}
	if renamed.Name != "WORK" {
		t.Errorf("the profile is named %q, want %q", renamed.Name, "WORK")
	}
	if len(renamed.Entries) != 1 {
		t.Errorf("the profile holds %d entries, want 1", len(renamed.Entries))
	}
}

func TestRenameRefusesANameAlreadyInUse(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Work", pigeonpost), profileOf(t, "Gaming", nordvpn))
	manager, _, _ := managerOver(store)

	err := manager.Rename(context.Background(), "Work", "gaming")
	if !errors.Is(err, ErrProfileNameInUse) {
		t.Fatalf("Rename answered %v, want %v", err, ErrProfileNameInUse)
	}
	if _, err := store.Load(context.Background(), "Work"); err != nil {
		t.Error("the profile being renamed was lost to a refused rename")
	}
}

func TestRenameReportsEveryFailureItMeets(t *testing.T) {
	t.Parallel()
	cases := map[string]func(store *fakeStore){
		"the profile cannot be read": func(store *fakeStore) {
			store.loadErr = errors.New("unreadable")
		},
		"the profiles cannot be listed": func(store *fakeStore) {
			store.namesErr = errors.New("no directory")
		},
		"the new profile cannot be written": func(store *fakeStore) {
			store.saveErr = errors.New("the disk is full")
		},
		"the old profile cannot be removed": func(store *fakeStore) {
			store.deleteErr = errors.New("the file is locked")
		},
	}
	for name, breaking := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := newFakeStore(profileOf(t, "Work", pigeonpost))
			breaking(store)
			manager, _, _ := managerOver(store)
			if err := manager.Rename(context.Background(), "Work", "Office"); err == nil {
				t.Fatal("Rename reported success over a store that refused")
			}
		})
	}
}

func TestRenameRefusesANameTheDomainWillNotAccept(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Work", pigeonpost))
	manager, _, _ := managerOver(store)

	if err := manager.Rename(context.Background(), "Work", "   "); err == nil {
		t.Fatal("Rename accepted a name with nothing in it")
	}
}

func TestDeleteRemovesTheProfileAndSaysSo(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Work", pigeonpost))
	manager, _, log := managerOver(store)

	if err := manager.Delete(context.Background(), "Work"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Load(context.Background(), "Work"); !errors.Is(err, ErrNoSuchProfile) {
		t.Error("the profile is still there")
	}
	if !log.saying("was deleted") {
		t.Error("the deletion was not recorded")
	}
}

func TestDeleteReportsAProfileThatIsNotThere(t *testing.T) {
	t.Parallel()
	manager, _, _ := managerOver(newFakeStore())

	if err := manager.Delete(context.Background(), "Missing"); !errors.Is(err, ErrNoSuchProfile) {
		t.Fatalf("Delete answered %v, want %v", err, ErrNoSuchProfile)
	}
}
