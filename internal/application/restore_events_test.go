package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// A restore acts on events and never on a wait (FR-079). These cover what it
// does when there are no events to be had and what it keeps doing after it
// has ended.

var errNoHook = errors.New("the desktop could not be hooked")

// restoreWatching builds a restore over the given events, for the tests that
// set them.
func restoreWatching(desktop *fakeDesktop, clock *fakeClock, log *fakeLog, events *fakeEvents,
	running ...domain.ApplicationIdentity) *RestoreService {
	return NewRestoreService(desktop, newFakeProcesses(running...), &fakeLauncher{},
		newFakeStore(), clock, log, Policy{Ceiling: time.Minute}, &fakeCeilings{}, screenst,
		&fakeStrangers{}, &fakeSplash{}, events)
}

// A desktop that cannot be watched still gets every window already open placed;
// what has not appeared waits for the ceiling; both the log and the report
// say why.
func TestADesktopThatCannotBeWatchedStillPlacesWhatIsThere(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	clock := newFakeClock()
	log := &fakeLog{}
	service := restoreWatching(desktop, clock, log,
		&fakeEvents{clock: clock, watchErr: errNoHook}, pigeonpost, stellody)
	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true, Placements: []domain.Placement{onPrimary}},
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{onPrimary}})

	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if !reportOf(t, report, pigeonpost).Satisfied {
		t.Fatal("the window already open was not placed")
	}
	if entry := reportOf(t, report, stellody); entry.Satisfied || !containsText(entry.Reason, "ceiling") {
		t.Fatalf("the missing window was not waited for until the ceiling: %+v", entry)
	}
	if !anyContaining(report.Notes, "could not be watched") {
		t.Fatalf("the report does not say the desktop could not be watched: %v", report.Notes)
	}
}

// A watch that cannot be begun once the restore has ended is said in the log;
// the restore's own result stands.
func TestAWatchAfterTheRestoreThatCannotBeBegunIsSaid(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	clock := newFakeClock()
	log := &fakeLog{}
	service := restoreWatching(desktop, clock, log,
		&fakeEvents{clock: clock, tailWatchErr: errNoHook}, pigeonpost)

	report, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary))
	if err != nil || !reportOf(t, report, pigeonpost).Satisfied {
		t.Fatalf("the restore did not place the window: %v", err)
	}
	if !log.saying("could not be watched after the restore") {
		t.Fatal("the log does not say the placed windows could not be watched")
	}
}

// FR-033 after the restore: a window that moves itself once the restore has
// ended is still put back, once.
func TestAWindowThatMovesAfterTheRestoreIsPutBack(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	clock := newFakeClock()
	log := &fakeLog{}
	var waits int
	events := &fakeEvents{clock: clock, afterwards: func(context.Context) (Wake, error) {
		waits++
		if waits == 1 {
			desktop.moveWindow(1, domain.Rect{X: 999, Y: 999, Width: 800, Height: 600})
			return WakeChanged, nil
		}
		return WakeTouched, nil
	}}
	service := restoreWatching(desktop, clock, log, events, pigeonpost)

	if _, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary)); err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	log.waitFor(t, "placed again")
	if len(desktop.placements()) != 2 {
		t.Fatalf("the window was placed %d times, wanted 2", len(desktop.placements()))
	}
}

// After the restore the log carries notes and real failures, never the windows
// nothing happened to.
func TestAfterTheRestoreOnlyWhatHappenedIsLogged(t *testing.T) {
	t.Parallel()
	log := &fakeLog{}
	service := restoreWatching(&fakeDesktop{}, newFakeClock(), log, &fakeEvents{})
	scratch := NewReport("Desk", newFakeClock().Now())
	scratch.Track(pigeonpost)
	scratch.Track(stellody)
	scratch.Fail(stellody, "its window could not be moved")
	scratch.NoteEntry(pigeonpost, "was placed again")

	service.logAfterwards(scratch)

	if log.saying(notReached) {
		t.Errorf("a window nothing happened to was logged: %v", log.steps)
	}
	if !log.saying("could not be moved") || !log.saying("was placed again") {
		t.Errorf("a failure or a note was not logged: %v", log.steps)
	}
}

// A newer restore ends the watch the last one left running, so two restores
// never fight over the same windows.
func TestANewerRestoreEndsTheWatchTheLastLeftRunning(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	clock := newFakeClock()
	watching := make(chan struct{})
	ended := make(chan struct{})
	var began bool
	events := &fakeEvents{clock: clock, afterwards: func(ctx context.Context) (Wake, error) {
		if !began {
			began = true
			close(watching)
			<-ctx.Done()
			close(ended)
			return WakeDeadline, ctx.Err()
		}
		return WakeTouched, nil
	}}
	service := restoreWatching(desktop, clock, &fakeLog{}, events, pigeonpost)

	if _, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary)); err != nil {
		t.Fatalf("the first restore failed: %v", err)
	}
	<-watching
	if _, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary)); err != nil {
		t.Fatalf("the second restore failed: %v", err)
	}
	select {
	case <-ended:
	case <-time.After(testPatience):
		t.Fatal("the first restore's watch was left running")
	}
}

// Once the user takes over during a restore, nothing is watched afterwards:
// from then on a moved window is the user moving it (FR-033).
func TestNothingIsWatchedOnceTheUserHasTakenOver(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	clock := newFakeClock()
	// The window for the second entry never comes, so the restore waits; the
	// first wait is the user's key press.
	events := &fakeEvents{clock: clock, touchAt: 1}
	service := restoreWatching(desktop, clock, &fakeLog{}, events, pigeonpost, stellody)
	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true, Placements: []domain.Placement{onPrimary}},
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{onPrimary}})

	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if entry := reportOf(t, report, stellody); entry.Satisfied || !containsText(entry.Reason, "taken over") {
		t.Fatalf("the entry still waited for was not reported at the user's key press: %+v", entry)
	}
	events.mutex.Lock()
	watches := events.watches
	events.mutex.Unlock()
	if watches != 1 {
		t.Fatalf("%d watches were begun: the placed windows were watched after the user took over", watches)
	}
}
