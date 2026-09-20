package application

import (
	"context"
	"errors"
	"testing"
)

// TestSetDefaultLeavesExactlyOneMarked is FR-040. The count is asserted over
// the whole store rather than over the two profiles in question, because the
// failure this guards against is a third one left marked from before.
func TestSetDefaultLeavesExactlyOneMarked(t *testing.T) {
	t.Parallel()
	store := newFakeStore(
		profileOf(t, "Work", claude).WithDefault(true),
		profileOf(t, "Gaming", nordvpn),
		profileOf(t, "Admin", stellody).WithDefault(true),
	)
	manager, _, log := managerOver(store)

	if err := manager.SetDefault(context.Background(), "Gaming"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	marked := markedProfiles(t, store)
	if len(marked) != 1 {
		t.Fatalf("%d profiles are marked as the default, want 1: %v", len(marked), marked)
	}
	if marked[0] != "Gaming" {
		t.Errorf("the default is %q, want %q", marked[0], "Gaming")
	}
	if !log.saying("is now the default") {
		t.Error("the change of default was not recorded")
	}
}

func TestSetDefaultOnTheProfileAlreadyMarkedChangesNothing(t *testing.T) {
	t.Parallel()
	store := newFakeStore(
		profileOf(t, "Work", claude).WithDefault(true),
		profileOf(t, "Gaming", nordvpn),
	)
	manager, _, _ := managerOver(store)

	if err := manager.SetDefault(context.Background(), "work"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	marked := markedProfiles(t, store)
	if len(marked) != 1 || marked[0] != "Work" {
		t.Errorf("the marked profiles are %v, want just Work", marked)
	}
}

func TestClearDefaultLeavesNoneMarked(t *testing.T) {
	t.Parallel()
	store := newFakeStore(
		profileOf(t, "Work", claude).WithDefault(true),
		profileOf(t, "Gaming", nordvpn),
	)
	manager, _, log := managerOver(store)

	if err := manager.ClearDefault(context.Background()); err != nil {
		t.Fatalf("ClearDefault: %v", err)
	}
	if marked := markedProfiles(t, store); len(marked) != 0 {
		t.Errorf("%v is still marked", marked)
	}
	if !log.saying("no profile is marked") {
		t.Error("the clearing was not recorded")
	}
}

func TestSetDefaultReportsEveryFailureItMeets(t *testing.T) {
	t.Parallel()
	cases := map[string]func(store *fakeStore){
		"the wanted profile cannot be read": func(store *fakeStore) {
			store.loadErr = errors.New("unreadable")
		},
		"the profiles cannot be listed": func(store *fakeStore) {
			store.namesErr = errors.New("no directory")
		},
		"a profile cannot be written": func(store *fakeStore) {
			store.saveErr = errors.New("the disk is full")
		},
	}
	for name, breaking := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := newFakeStore(
				profileOf(t, "Work", claude).WithDefault(true),
				profileOf(t, "Gaming", nordvpn),
			)
			breaking(store)
			manager, _, _ := managerOver(store)
			if err := manager.SetDefault(context.Background(), "Gaming"); err == nil {
				t.Fatal("SetDefault reported success over a store that refused")
			}
		})
	}
}

func TestSetDefaultReportsAProfileItCannotReadWhileClearing(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Work", claude).WithDefault(true))
	manager, _, _ := managerOver(store)

	// A name in the listing with nothing behind it is what a profile removed
	// between the two calls looks like. Leaving a second profile marked is the
	// outcome this must not produce quietly, so it is reported.
	store.order = append(store.order, "Vanished")
	if err := manager.SetDefault(context.Background(), "Work"); err == nil {
		t.Fatal("SetDefault passed over a profile it could not read")
	}
}

func TestRemoveEntryDropsOnlyThatApplication(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Work", claude, stellody, nordvpn))
	manager, _, log := managerOver(store)

	if err := manager.RemoveEntry(context.Background(), "Work", stellody.Value); err != nil {
		t.Fatalf("RemoveEntry: %v", err)
	}
	profile, err := store.Load(context.Background(), "Work")
	if err != nil {
		t.Fatalf("loading the profile: %v", err)
	}
	if len(profile.Entries) != 2 {
		t.Fatalf("the profile holds %d entries, want 2", len(profile.Entries))
	}
	if _, held := profile.Find(stellody); held {
		t.Error("the entry that was removed is still there")
	}
	if _, held := profile.Find(claude); !held {
		t.Error("an entry that was not asked for was removed")
	}
	if !log.saying("was removed from profile") {
		t.Error("the removal was not recorded")
	}
}

// TestRemoveEntryLeavesAProfileWithNothingInIt is deliberate: a profile that
// arranges nothing is a profile; deleting it is a separate act the user asks
// for separately.
func TestRemoveEntryLeavesAProfileWithNothingInIt(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Work", claude))
	manager, _, _ := managerOver(store)

	if err := manager.RemoveEntry(context.Background(), "Work", claude.Value); err != nil {
		t.Fatalf("RemoveEntry: %v", err)
	}
	profile, err := store.Load(context.Background(), "Work")
	if err != nil {
		t.Fatalf("the profile was deleted rather than emptied: %v", err)
	}
	if len(profile.Entries) != 0 {
		t.Errorf("the profile holds %d entries, want none", len(profile.Entries))
	}
}

func TestRemoveEntryReportsAnApplicationTheProfileDoesNotHold(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Work", claude))
	manager, _, _ := managerOver(store)

	err := manager.RemoveEntry(context.Background(), "Work", nordvpn.Value)
	if !errors.Is(err, ErrNoSuchEntry) {
		t.Fatalf("RemoveEntry answered %v, want %v", err, ErrNoSuchEntry)
	}
}

func TestRemoveEntryReportsEveryFailureItMeets(t *testing.T) {
	t.Parallel()
	cases := map[string]func(store *fakeStore){
		"the profile cannot be read": func(store *fakeStore) {
			store.loadErr = errors.New("unreadable")
		},
		"the profile cannot be written": func(store *fakeStore) {
			store.saveErr = errors.New("the disk is full")
		},
	}
	for name, breaking := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := newFakeStore(profileOf(t, "Work", claude))
			breaking(store)
			manager, _, _ := managerOver(store)
			if err := manager.RemoveEntry(context.Background(), "Work", claude.Value); err == nil {
				t.Fatal("RemoveEntry reported success over a store that refused")
			}
		})
	}
}

func TestStartsWithWindowsReadsTheSignInEntry(t *testing.T) {
	t.Parallel()
	manager, startup, _ := managerOver(newFakeStore())
	startup.enabled = true

	enabled, err := manager.StartsWithWindows()
	if err != nil {
		t.Fatalf("StartsWithWindows: %v", err)
	}
	if !enabled {
		t.Error("StartsWithWindows reported off while the entry is on")
	}
}

func TestStartsWithWindowsReportsARegistryItCannotRead(t *testing.T) {
	t.Parallel()
	manager, startup, _ := managerOver(newFakeStore())
	startup.readErr = errors.New("the key is gone")

	if _, err := manager.StartsWithWindows(); err == nil {
		t.Fatal("StartsWithWindows answered over a registry it cannot read")
	}
}

func TestSetStartsWithWindowsWritesTheEntryAndSaysSo(t *testing.T) {
	t.Parallel()
	manager, startup, log := managerOver(newFakeStore())

	if err := manager.SetStartsWithWindows(true); err != nil {
		t.Fatalf("SetStartsWithWindows: %v", err)
	}
	if startup.writes != 1 || !startup.lastWrite {
		t.Errorf("the entry was written %d times, last %t", startup.writes, startup.lastWrite)
	}
	if !log.saying("start with Windows is now true") {
		t.Error("the change was not recorded")
	}
}

func TestSetStartsWithWindowsReportsARegistryItCannotWrite(t *testing.T) {
	t.Parallel()
	manager, startup, _ := managerOver(newFakeStore())
	startup.writeErr = errors.New("access denied")

	if err := manager.SetStartsWithWindows(true); err == nil {
		t.Fatal("SetStartsWithWindows reported success over a registry that refused")
	}
}
