package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

func captureUnder(desktop *fakeDesktop, processes *fakeProcesses, store *fakeStore) *CaptureService {
	return NewCaptureService(desktop, processes, store, &fakeLog{}, screenst)
}

// FR-003, FR-004, FR-005 and FR-013 together: one entry per application, a
// placement per visible window naming the display holding it, no placement for
// a hidden window and nothing at all for ScreenState itself.
func TestACaptureReadsOneEntryPerApplication(t *testing.T) {
	t.Parallel()
	hidden := aWindow(3, nordvpn, at(2))
	hidden.Visible = false
	onTheLeft := aWindow(2, stellody, at(1))
	onTheLeft.Rect = domain.Rect{X: -3000, Y: 200, Width: 1200, Height: 800}
	onTheLeft.State = domain.ShowMaximised

	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay, leftDisplay},
		windows: []Window{
			aWindow(4, screenst, at(3)),
			hidden,
			onTheLeft,
			aWindow(1, pigeonpost, at(0)),
		},
	}
	service := captureUnder(desktop, newFakeProcesses(), newFakeStore())

	review, err := service.Review(context.Background(), "")
	if err != nil {
		t.Fatalf("the capture failed: %v", err)
	}
	if len(review.Entries) != 3 {
		t.Fatalf("captured %d entries: %+v", len(review.Entries), review.Entries)
	}
	// Oldest window first, so Claude leads and NordVPN follows Stellody.
	if !review.Entries[0].Application.Equal(pigeonpost) || !review.Entries[1].Application.Equal(stellody) {
		t.Fatalf("entries out of order: %+v", review.Entries)
	}
	if display := review.Entries[1].Placements[0].Display; !display.Equal(leftID) {
		t.Fatalf("Stellody was recorded on %s", display)
	}
	if state := review.Entries[1].Placements[0].State; state != domain.ShowMaximised {
		t.Fatalf("Stellody was recorded showing %s", state)
	}
	nord := review.Entries[2]
	if !nord.Application.Equal(nordvpn) || len(nord.Placements) != 0 || !nord.Running {
		t.Fatalf("the hidden window was not recorded as running with no placement: %+v", nord)
	}
	for _, entry := range review.Entries {
		if entry.Application.Equal(screenst) {
			t.Fatal("the capture recorded ScreenState itself")
		}
	}
}

// FR-014: a window whose state could not be read is omitted and named.
func TestAnUnreadableWindowIsNamedRatherThanDropped(t *testing.T) {
	t.Parallel()
	unreadable := Window{ID: 7, Description: "Some other window", Unreadable: "access denied"}
	nameless := Window{ID: 8, Application: pigeonpost, Unreadable: "access denied"}
	unknown := Window{ID: 9, Unreadable: "access denied"}
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{unreadable, nameless, unknown, aWindow(1, pigeonpost, at(0))},
	}
	service := captureUnder(desktop, newFakeProcesses(), newFakeStore())

	review, err := service.Review(context.Background(), "")
	if err != nil {
		t.Fatalf("the capture failed: %v", err)
	}
	if len(review.Entries) != 1 {
		t.Fatalf("captured %d entries", len(review.Entries))
	}
	if len(review.Unreadable) != 3 {
		t.Fatalf("named %d unreadable windows: %v", len(review.Unreadable), review.Unreadable)
	}
	if !anyContaining(review.Unreadable, "Some other window") ||
		!anyContaining(review.Unreadable, pigeonpost.String()) ||
		!anyContaining(review.Unreadable, "window 9") {
		t.Fatalf("an unreadable window was not named usefully: %v", review.Unreadable)
	}
}

// FR-012: recapturing a profile keeps the applications it named, even those
// with no window now, so the tray applications are not quietly dropped.
func TestRecapturingKeepsTheProfilesOwnApplications(t *testing.T) {
	t.Parallel()
	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: nordvpn, Running: true},
		domain.Entry{Application: stellody, Running: true},
		domain.Entry{Application: screenst, Running: true},
	)
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	service := captureUnder(desktop, newFakeProcesses(nordvpn), newFakeStore(profile))

	review, err := service.Review(context.Background(), "Desk")
	if err != nil {
		t.Fatalf("the capture failed: %v", err)
	}
	if len(review.Entries) != 3 {
		t.Fatalf("captured %d entries: %+v", len(review.Entries), review.Entries)
	}
	byName := make(map[string]domain.Entry)
	for _, entry := range review.Entries {
		byName[entry.Application.String()] = entry
	}
	if !byName[nordvpn.String()].Running {
		t.Fatal("NordVPN is running and was not recorded as running")
	}
	if byName[stellody.String()].Running {
		t.Fatal("Stellody is not running and was recorded as running")
	}
	if _, present := byName[screenst.String()]; present {
		t.Fatal("ScreenState came back through the profile it was excluded from")
	}
}

// A capture based on a profile that is no longer stored is a capture of the
// desktop, not a failure.
func TestRecapturingAProfileThatHasGoneCapturesTheDesktop(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	service := captureUnder(desktop, newFakeProcesses(), newFakeStore())

	review, err := service.Review(context.Background(), "Gone")
	if err != nil {
		t.Fatalf("the capture failed: %v", err)
	}
	if len(review.Entries) != 1 {
		t.Fatalf("captured %d entries", len(review.Entries))
	}
}

// FR-032 at capture time: a window sitting on no connected display is recorded
// against the primary one, so restoring the profile brings it back reachable.
func TestAWindowOnNoDisplayIsRecordedAgainstThePrimaryOne(t *testing.T) {
	t.Parallel()
	stray := aWindow(1, pigeonpost, at(0))
	stray.Rect = domain.Rect{X: 30000, Y: 30000, Width: 400, Height: 300}
	desktop := &fakeDesktop{displays: []Display{leftDisplay}, windows: []Window{stray}}
	service := captureUnder(desktop, newFakeProcesses(), newFakeStore())

	review, err := service.Review(context.Background(), "")
	if err != nil {
		t.Fatalf("the capture failed: %v", err)
	}
	// leftDisplay is not marked primary, so the first display stands in as one.
	if display := review.Entries[0].Placements[0].Display; !display.Equal(leftID) {
		t.Fatalf("the stray window was recorded on %s", display)
	}
}

// FR-002: a name already in use is refused, with nothing overwritten, unless
// the caller has asked to replace it.
func TestSavingUnderANameInUseIsRefused(t *testing.T) {
	t.Parallel()
	existing, _ := domain.NewProfile("Desk", domain.Entry{Application: nordvpn})
	store := newFakeStore(existing)
	service := captureUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses(), store)

	entries := []domain.Entry{{Application: pigeonpost, Running: true}}
	if _, err := service.Save(context.Background(), "desk", entries, false); !errors.Is(err, ErrProfileNameInUse) {
		t.Fatalf("expected ErrProfileNameInUse, got %v", err)
	}
	held, err := store.Load(context.Background(), "Desk")
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if !held.Entries[0].Application.Equal(nordvpn) {
		t.Fatal("the refused save overwrote the stored profile")
	}

	saved, err := service.Save(context.Background(), "desk", entries, true)
	if err != nil {
		t.Fatalf("replacing: %v", err)
	}
	if !saved.Entries[0].Application.Equal(pigeonpost) {
		t.Fatal("the replacing save did not take")
	}
}

// FR-011: the entries the user leaves in the review are the profile.
func TestSavingWritesTheConfirmedEntries(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	service := captureUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses(), store)

	entries := []domain.Entry{
		{Application: pigeonpost, Running: true},
		{Application: stellody, Running: true},
	}
	profile, err := service.Save(context.Background(), "  Desk  ", entries, false)
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	if profile.Name != "Desk" {
		t.Fatalf("the name was stored as %q", profile.Name)
	}
	if len(profile.Entries) != 2 {
		t.Fatalf("stored %d entries", len(profile.Entries))
	}
	if _, err := store.Load(context.Background(), "Desk"); err != nil {
		t.Fatalf("the profile was not stored: %v", err)
	}
}

// The first profile a user makes is the one applied at sign-in. Leaving it
// unmarked is the product appearing not to work: the desktop is captured, the
// machine is restarted and nothing is arranged, with no clue that a marking was
// owed. A second capture leaves the marking where the user put it.
func TestTheFirstProfileBecomesTheDefault(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	service := captureUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses(), store)
	entries := []domain.Entry{{Application: pigeonpost, Running: true}}

	first, err := service.Save(context.Background(), "Desk", entries, false)
	if err != nil {
		t.Fatalf("saving the first: %v", err)
	}
	if !first.Default {
		t.Fatal("the only profile there is was not marked as the default")
	}
	stored, _, err := store.Default(context.Background())
	if err != nil || stored.Name != "Desk" {
		t.Fatalf("the store answers %q as the default, %v", stored.Name, err)
	}

	second, err := service.Save(context.Background(), "Away", entries, false)
	if err != nil {
		t.Fatalf("saving the second: %v", err)
	}
	if second.Default {
		t.Fatal("a second profile took the marking from the first")
	}
	stored, _, err = store.Default(context.Background())
	if err != nil || stored.Name != "Desk" {
		t.Fatalf("the marking moved to %q, %v", stored.Name, err)
	}
}

// A store that cannot say whether anything is marked still writes the profile:
// the marking is a convenience and the capture is the work.
func TestAProfileIsStillWrittenWhenTheMarkingCannotBeRead(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.defaultErr = errors.New("the store would not answer")
	service := captureUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses(), store)

	profile, err := service.Save(context.Background(), "Desk",
		[]domain.Entry{{Application: pigeonpost, Running: true}}, false)
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	if profile.Default {
		t.Fatal("a profile was marked although the store could not be asked")
	}
	if _, err := store.Load(context.Background(), "Desk"); err != nil {
		t.Fatalf("the profile was not stored: %v", err)
	}
}

// An invalid review is refused by the domain before anything is written, which
// is what keeps the store free of profiles that cannot be restored.
func TestAnInvalidReviewIsNotWritten(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	service := captureUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses(), store)

	twice := []domain.Entry{{Application: pigeonpost, Running: true}, {Application: pigeonpost}}
	if _, err := service.Save(context.Background(), "Desk", twice, false); !errors.Is(err, domain.ErrDuplicateEntry) {
		t.Fatalf("expected ErrDuplicateEntry, got %v", err)
	}
	if _, err := service.Save(context.Background(), "   ", nil, false); !errors.Is(err, domain.ErrEmptyProfileName) {
		t.Fatalf("expected ErrEmptyProfileName, got %v", err)
	}
	if names, _ := store.Names(context.Background()); len(names) != 0 {
		t.Fatalf("the store holds %v", names)
	}
}
