package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// fakeStartup is the sign-in entry in memory, with error injection on each half
// so a manager can be asked what it does when the registry refuses.
type fakeStartup struct {
	enabled   bool
	readErr   error
	writeErr  error
	writes    int
	lastWrite bool
}

func (startup *fakeStartup) Enabled() (bool, error) {
	if startup.readErr != nil {
		return false, startup.readErr
	}
	return startup.enabled, nil
}

func (startup *fakeStartup) SetEnabled(enabled bool) error {
	if startup.writeErr != nil {
		return startup.writeErr
	}
	startup.writes++
	startup.lastWrite = enabled
	startup.enabled = enabled
	return nil
}

// profileOf builds a stored profile of the given name over one entry each for
// the named applications, so a test says what a profile holds in one line.
func profileOf(t *testing.T, name string, applications ...domain.ApplicationIdentity) domain.Profile {
	t.Helper()
	entries := make([]domain.Entry, 0, len(applications))
	for _, application := range applications {
		entries = append(entries, domain.Entry{
			Application: application,
			Running:     true,
			Placements: []domain.Placement{
				aPlacement(primaryID, domain.Rect{X: 0, Y: 0, Width: 3440, Height: 1392}),
			},
		})
	}
	profile, err := domain.NewProfile(name, entries...)
	if err != nil {
		t.Fatalf("building profile %q: %v", name, err)
	}
	return profile
}

// managerOver returns a manager service and the fakes behind it.
func managerOver(store *fakeStore) (*ManagerService, *fakeStartup, *fakeLog) {
	startup := &fakeStartup{}
	log := &fakeLog{}
	return NewManagerService(store, startup, log), startup, log
}

// markedProfiles returns the names of every profile marked as the default,
// which is how a test asserts that exactly one is.
func markedProfiles(t *testing.T, store *fakeStore) []string {
	t.Helper()
	names, err := store.Names(context.Background())
	if err != nil {
		t.Fatalf("listing the profiles: %v", err)
	}
	var marked []string
	for _, name := range names {
		profile, err := store.Load(context.Background(), name)
		if err != nil {
			t.Fatalf("loading %q: %v", name, err)
		}
		if profile.Default {
			marked = append(marked, profile.Name)
		}
	}
	return marked
}

func TestProfilesListsEveryProfileInOrderWithItsCounts(t *testing.T) {
	t.Parallel()
	store := newFakeStore(
		profileOf(t, "Work", pigeonpost, stellody),
		profileOf(t, "admin", nordvpn),
		profileOf(t, "Gaming").WithDefault(true),
	)
	manager, _, _ := managerOver(store)

	listed, err := manager.Profiles(context.Background())
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("Profiles returned %d summaries, want 3", len(listed))
	}
	// Ordered without regard to case, which is how a reader looks for a name.
	for at, want := range []string{"admin", "Gaming", "Work"} {
		if listed[at].Name != want {
			t.Errorf("summary %d is %q, want %q", at, listed[at].Name, want)
		}
	}
	if listed[2].Entries != 2 {
		t.Errorf("Work holds %d entries, want 2", listed[2].Entries)
	}
	if !listed[1].Default {
		t.Error("Gaming is not marked as the default")
	}
}

// TestProfilesLeavesOutOneItCannotReadRatherThanFailing holds the store's own
// rule: an unreadable profile costs the user that profile, never the list.
func TestProfilesLeavesOutOneItCannotReadRatherThanFailing(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Work", pigeonpost))
	store.loadErr = errors.New("the file is unreadable")
	manager, _, log := managerOver(store)

	listed, err := manager.Profiles(context.Background())
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("Profiles returned %d summaries, want none", len(listed))
	}
	if !log.saying("is not being listed") {
		t.Error("nothing was said about the profile that was left out")
	}
}

func TestProfilesReportsAStoreItCannotList(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.namesErr = errors.New("the directory is gone")
	manager, _, _ := managerOver(store)

	if _, err := manager.Profiles(context.Background()); err == nil {
		t.Fatal("Profiles answered a list over a store that cannot be listed")
	}
}

func TestEntriesDescribesEveryEntryOfTheProfile(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Work", pigeonpost, stellody))
	manager, _, _ := managerOver(store)

	views, err := manager.Entries(context.Background(), "Work")
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("Entries returned %d views, want 2", len(views))
	}
	if views[0].Application != pigeonpost.Value {
		t.Errorf("first entry names %q, want %q", views[0].Application, pigeonpost.Value)
	}
	if views[0].Name != "PigeonPost" || views[0].Program != "PigeonPost.exe" {
		t.Errorf("first entry reads %q over %q, want PigeonPost over PigeonPost.exe",
			views[0].Name, views[0].Program)
	}
	if views[0].Kind != pigeonpost.Kind.String() {
		t.Errorf("first entry's kind is %q, want %q", views[0].Kind, pigeonpost.Kind.String())
	}
	if !views[0].Running {
		t.Error("the first entry does not say the application should be running")
	}
	if len(views[0].Placements) != 1 {
		t.Fatalf("the first entry shows %d placements, want 1", len(views[0].Placements))
	}
	placement := views[0].Placements[0]
	if placement.Display != primaryID.String() {
		t.Errorf("the placement names display %q, want %q", placement.Display, primaryID.String())
	}
	if placement.State != domain.ShowMaximised.String() {
		t.Errorf("the placement reads %q, want %q", placement.State, domain.ShowMaximised.String())
	}
	if placement.Rect == "" {
		t.Error("the placement shows no rectangle")
	}
	if placement.Size == "" {
		t.Error("the placement shows no size")
	}
}

func TestEntriesReportsAProfileThatIsNotThere(t *testing.T) {
	t.Parallel()
	manager, _, _ := managerOver(newFakeStore())

	_, err := manager.Entries(context.Background(), "Missing")
	if !errors.Is(err, ErrNoSuchProfile) {
		t.Fatalf("Entries answered %v, want %v", err, ErrNoSuchProfile)
	}
}
