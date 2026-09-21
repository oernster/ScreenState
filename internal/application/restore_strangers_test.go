package application

import (
	"context"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// A profile says what the desktop should look like, so a window it does not
// name is in the way (FR-063). The applications that start with Windows and are
// not part of a session were landing on top of the arrangement and had to be
// put away by hand.
func TestAWindowTheProfileDoesNotNameIsPutAway(t *testing.T) {
	t.Parallel()
	stranger := aWindow(2, nordvpn, at(1))
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0)), stranger},
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, nordvpn),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), deskProfile(t))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	put := putAwayCalls(desktop, stranger.ID)
	if len(put) != 1 {
		t.Fatalf("the stranger was put away %d time(s)", len(put))
	}
	if put[0].rect != stranger.Rect {
		t.Errorf("the stranger was moved to %v as well as put away", put[0].rect)
	}
	if !anyContaining(report.SortedNotes(), "put away") {
		t.Errorf("the report says nothing about it: %v", report.SortedNotes())
	}
}

// Three windows are never put away: one the profile names, this product's own,
// and one already minimised, which has nothing left to do.
func TestTheWindowsARestoreLeavesAlone(t *testing.T) {
	t.Parallel()
	named := aWindow(1, pigeonpost, at(0))
	minimised := aWindow(3, nordvpn, at(2))
	minimised.State = domain.ShowMinimised
	ours := aWindow(4, screenst, at(3))
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{named, minimised, ours},
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, nordvpn),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), deskProfile(t))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	for _, window := range []Window{named, minimised, ours} {
		if calls := putAwayCalls(desktop, window.ID); len(calls) != 0 {
			t.Errorf("window %d was put away", uint64(window.ID))
		}
	}
	if anyContaining(report.SortedNotes(), "put away") {
		t.Errorf("the report claims something was put away: %v", report.SortedNotes())
	}
}

// deskProfile is a profile naming one application, placed on the primary
// display, which is enough for a restore to settle.
func deskProfile(t *testing.T) domain.Profile {
	t.Helper()
	profile, err := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true, Placements: []domain.Placement{
			aPlacement(primaryID, domain.Rect{X: 0, Y: 0, Width: 3440, Height: 1392})}},
	)
	if err != nil {
		t.Fatalf("the profile is not valid: %v", err)
	}
	return profile
}

// putAwayCalls returns the minimising placements made against one window.
func putAwayCalls(desktop *fakeDesktop, id WindowID) []placeCall {
	var put []placeCall
	for _, call := range desktop.placements() {
		if call.id == id && call.state == domain.ShowMinimised {
			put = append(put, call)
		}
	}
	return put
}
