package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// containsText reports whether a reason holds the fragment.
func containsText(reason string, fragment string) bool {
	return strings.Contains(reason, fragment)
}

var errRefused = errors.New("access denied")

// oneEntry returns a profile placing a single application on the primary
// display, which is the shape most of the edge cases need.
func oneEntry(application domain.ApplicationIdentity, placements ...domain.Placement) domain.Profile {
	entry := domain.Entry{Application: application, Running: true, Placements: placements}
	profile, _ := domain.NewProfile("Desk", entry)
	return profile
}

var onPrimary = aPlacement(primaryID, domain.Rect{X: 0, Y: 0, Width: 800, Height: 600})

// FR-033: an application that moves its own window after it was placed has it
// put back, once.
func TestAWindowThatMovesItselfIsPutBackOnce(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	desktop.afterPlace = func(desk *fakeDesktop, id WindowID) {
		desk.afterPlace = nil
		desk.moveWindow(id, domain.Rect{X: 999, Y: 999, Width: 800, Height: 600})
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if len(desktop.placements()) != 2 {
		t.Fatalf("the window was placed %d times, wanted 2", len(desktop.placements()))
	}
	entry := reportOf(t, report, pigeonpost)
	if !noteSaying(entry, "moved itself after being placed") {
		t.Fatalf("the report does not record the second attempt: %+v", entry)
	}
	if !entry.Satisfied {
		t.Fatalf("the entry was not satisfied once it stayed put: %+v", entry)
	}
}

// FR-034: a window that will not stay put is reported rather than fought over.
func TestAWindowThatKeepsMovingIsReportedAndLeft(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	desktop.afterPlace = func(desk *fakeDesktop, id WindowID) {
		desk.moveWindow(id, domain.Rect{X: 999, Y: 999, Width: 800, Height: 600})
	}
	clock := newFakeClock()
	log := &fakeLog{}
	// The restore ends once its one window is placed; the watch it keeps
	// afterwards (FR-033) sees the window move again. One change, then the user
	// takes over.
	var afterwardsWaits int
	events := &fakeEvents{clock: clock, afterwards: func(context.Context) (Wake, error) {
		afterwardsWaits++
		if afterwardsWaits == 1 {
			return WakeChanged, nil
		}
		return WakeTouched, nil
	}}
	service := NewRestoreService(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), clock, log, Policy{Ceiling: time.Minute}, &fakeCeilings{}, screenst,
		&fakeStrangers{}, &fakeSplash{}, events)

	if _, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary)); err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	log.waitFor(t, "did not stay where it was put")
	if len(desktop.placements()) != 2 {
		t.Fatalf("the window was placed %d times: the agent fought for it",
			len(desktop.placements()))
	}
}

// FR-035: a window Windows will not let this process move is named and the rest
// of the restore carries on.
func TestAWindowThatCannotBeMovedIsNamed(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0)), aWindow(2, stellody, at(1))},
		placeErr: map[WindowID]error{1: errRefused},
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, stellody), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})

	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true, Placements: []domain.Placement{onPrimary}},
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{onPrimary}})

	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if entry := reportOf(t, report, pigeonpost); entry.Satisfied ||
		!containsText(entry.Reason, "could not be moved") {
		t.Fatalf("the refusal was not reported: %+v", entry)
	}
	if !reportOf(t, report, stellody).Satisfied {
		t.Fatal("one window refusing to move stopped the rest of the restore")
	}
}

// FR-037: windows beyond the placements recorded are left alone and named.
func TestExtraWindowsAreLeftAloneAndNamed(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0)), aWindow(2, pigeonpost, at(1))},
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if placements := desktop.placements(); len(placements) != 1 || placements[0].id != 1 {
		t.Fatalf("the wrong windows were placed: %+v", placements)
	}
	if entry := reportOf(t, report, pigeonpost); !noteSaying(entry, "more window(s) open") {
		t.Fatalf("the extra window was not named: %+v", entry)
	}
}

// A window closed after it was placed is ordinary, not a failure.
func TestAWindowClosedAfterPlacingIsNotAFailure(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	desktop.afterPlace = func(desk *fakeDesktop, _ WindowID) {
		desk.mutex.Lock()
		defer desk.mutex.Unlock()
		desk.windows = nil
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	entry := reportOf(t, report, pigeonpost)
	if !entry.Satisfied || !noteSaying(entry, "closed after it was placed") {
		t.Fatalf("a closed window was treated as a failure: %+v", entry)
	}
}

// A window that cannot be read again is noted without failing the entry, which
// the restore did satisfy.
func TestAWindowThatCannotBeReadAgainIsNoted(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	desktop.afterPlace = func(desk *fakeDesktop, _ WindowID) {
		desk.mutex.Lock()
		defer desk.mutex.Unlock()
		desk.windowErr = errRefused
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if entry := reportOf(t, report, pigeonpost); !noteSaying(entry, "could not be read again") {
		t.Fatalf("the unreadable window was not noted: %+v", entry)
	}
}

// A window that has moved and cannot be moved back is reported.
func TestAWindowThatCannotBeMovedBackIsReported(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	desktop.afterPlace = func(desk *fakeDesktop, id WindowID) {
		desk.afterPlace = nil
		desk.moveWindow(id, domain.Rect{X: 999, Y: 999, Width: 800, Height: 600})
		desk.mutex.Lock()
		defer desk.mutex.Unlock()
		desk.placeErr = map[WindowID]error{id: errRefused}
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if entry := reportOf(t, report, pigeonpost); entry.Satisfied ||
		!containsText(entry.Reason, "could not be moved back") {
		t.Fatalf("the refusal was not reported: %+v", entry)
	}
}
