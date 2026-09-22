package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// Reading the desktop can fail at either half; a restore says which rather
// than reporting that something went wrong.
func TestARestoreSaysWhichReadingOfTheDesktopFailed(t *testing.T) {
	t.Parallel()
	profile := oneEntry(pigeonpost, onPrimary)

	byWindows := restoreUnder(
		&fakeDesktop{displays: []Display{primaryDisplay}, windowsErr: errRefused},
		newFakeProcesses(pigeonpost), &fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{})
	if _, err := byWindows.Restore(context.Background(), profile); !containsText(err.Error(), "reading the windows") {
		t.Fatalf("the windows failure was reported as %v", err)
	}

	byDisplays := restoreUnder(
		&fakeDesktop{displaysErr: errRefused},
		newFakeProcesses(pigeonpost), &fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{})
	if _, err := byDisplays.Restore(context.Background(), profile); !containsText(err.Error(), "reading the displays") {
		t.Fatalf("the displays failure was reported as %v", err)
	}
}

// With no display connected there is nowhere to put anything, so a restore says
// so rather than guessing at coordinates.
func TestARestoreWithNoDisplayConnectedSaysSo(t *testing.T) {
	t.Parallel()
	service := restoreUnder(&fakeDesktop{}, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})
	if _, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary)); !errors.Is(err, ErrNoDisplays) {
		t.Fatalf("expected ErrNoDisplays, got %v", err)
	}
	if _, err := newDisplaySet(nil); !errors.Is(err, ErrNoDisplays) {
		t.Fatalf("expected ErrNoDisplays, got %v", err)
	}
}

// An application whose running state cannot be read is named, whether the
// question is asked at the launch step or while waiting for it.
func TestAnApplicationThatCannotBeReadIsNamed(t *testing.T) {
	t.Parallel()
	for name, entry := range map[string]domain.Entry{
		"with a placement": {Application: pigeonpost, Running: true,
			Placements: []domain.Placement{onPrimary}},
		"without one": {Application: nordvpn, Running: true},
	} {
		processes := newFakeProcesses()
		processes.err = errRefused
		desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
		service := restoreUnder(desktop, processes, &fakeLauncher{},
			newFakeStore(), newFakeClock(), &fakeLog{})

		profile, _ := domain.NewProfile("Desk", entry)
		report, err := service.Restore(context.Background(), profile)
		if err != nil {
			t.Fatalf("%s: the restore failed: %v", name, err)
		}
		reported := reportOf(t, report, entry.Application)
		if reported.Satisfied || !containsText(reported.Reason, "could not be read") {
			t.Fatalf("%s: the failure was not reported: %+v", name, reported)
		}
	}
}

// An application running with a hidden window whose state cannot be read while
// it is being asked to show one is named too.
func TestAHiddenApplicationThatCannotBeReadIsNamed(t *testing.T) {
	t.Parallel()
	hidden := aWindow(1, nordvpn, at(0))
	hidden.Visible = false
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}, windows: []Window{hidden}}
	processes := newFakeProcesses(nordvpn)
	clock := newFakeClock()
	clock.onSleep = func(_ context.Context, _ *fakeClock, count int) {
		if count == 1 {
			processes.mutex.Lock()
			processes.err = errRefused
			processes.mutex.Unlock()
		}
	}
	service := restoreUnder(desktop, processes, &fakeLauncher{},
		newFakeStore(), clock, &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(nordvpn, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if entry := reportOf(t, report, nordvpn); entry.Satisfied {
		t.Fatalf("the entry was satisfied although it could not be read: %+v", entry)
	}
}

// FR-052: a failure nobody expected is recorded and leaves the agent able to
// restore again, rather than taking the process with it.
func TestAnUnexpectedFailureIsRecordedAndTheAgentSurvives(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	desktop.onWindows = func() { panic("the desktop fell over") }
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), newFakeClock(), log)

	report, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary))
	if err == nil {
		t.Fatal("the restore reported success after falling over")
	}
	if !anyContaining(report.Notes, "failed unexpectedly") {
		t.Fatalf("the report does not record the failure: %v", report.Notes)
	}
	if !log.saying("failed unexpectedly") {
		t.Fatal("the log does not record the failure")
	}

	// The agent is still able to restore, which is the half of FR-052 that
	// matters: an agent that vanishes leaves a half arranged desktop.
	desktop.onWindows = nil
	desktop.addWindow(aWindow(1, pigeonpost, at(0)))
	if _, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary)); err != nil {
		t.Fatalf("the agent could not restore again: %v", err)
	}
}

// A context that ended for a reason of its own is passed back as it is, rather
// than being read as a cancellation or as a replacement.
func TestARestoreReportsAContextThatRanOut(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	service := restoreUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses(pigeonpost), &fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{})

	if _, err := service.Restore(ctx, oneEntry(pigeonpost, onPrimary)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected the deadline, got %v", err)
	}
}

// FR-057, the other direction: a display arriving during a restore is recorded
// as well as one going away.
func TestADisplayArrivingDuringARestoreIsRecorded(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	clock := newFakeClock()
	clock.onSleep = func(_ context.Context, _ *fakeClock, count int) {
		if count == 1 {
			desktop.mutex.Lock()
			desktop.displays = []Display{primaryDisplay, leftDisplay}
			desktop.mutex.Unlock()
			desktop.addWindow(aWindow(1, pigeonpost, at(0)))
		}
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), clock, &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if !anyContaining(report.Notes, "the left display was connected during the restore") {
		t.Fatalf("the new display was not recorded: %v", report.Notes)
	}
}

// A window that cannot be read is no candidate for a placement either, so an
// entry whose only window is unreadable waits rather than being placed wrongly.
func TestAnUnreadableWindowIsNotPlaced(t *testing.T) {
	t.Parallel()
	unreadable := aWindow(1, pigeonpost, at(0))
	unreadable.Unreadable = "access denied"
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}, windows: []Window{unreadable}}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if len(desktop.placements()) != 0 {
		t.Fatal("an unreadable window was placed")
	}
	if reportOf(t, report, pigeonpost).Satisfied {
		t.Fatal("an entry was satisfied by a window that could not be read")
	}
}

// The default profile cannot be read: the caller is told which reading failed.
func TestAFailureToReadTheDefaultProfileIsReported(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.defaultErr = errRefused
	service := restoreUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses(), &fakeLauncher{}, store, newFakeClock(), &fakeLog{})

	if _, _, err := service.RestoreDefault(context.Background()); !containsText(err.Error(), "the default profile") {
		t.Fatalf("reported as %v", err)
	}
}

// FR-032: a window larger than the display it lands on is held within it;
// one starting outside it is slid back in rather than left unreachable.
func TestAWindowIsHeldWithinTheDisplayItLandsOn(t *testing.T) {
	t.Parallel()
	tall := domain.Rect{X: -500, Y: -500, Width: 9000, Height: 9000}
	held := fit(tall, primaryDisplay)
	if held.Width != primaryDisplay.WorkArea.Width || held.Height != primaryDisplay.WorkArea.Height {
		t.Fatalf("an oversized window was not clamped: %s", held)
	}
	if held.X != primaryDisplay.WorkArea.X || held.Y != primaryDisplay.WorkArea.Y {
		t.Fatalf("an oversized window was not placed at the corner: %s", held)
	}

	// A display whose work area was not reported falls back to its bounds.
	noWorkArea := Display{Identity: leftID, Bounds: leftDisplay.Bounds}
	inside := fit(domain.Rect{X: 30000, Y: 30000, Width: 400, Height: 300}, noWorkArea)
	if inside.Right() > noWorkArea.Bounds.Right() || inside.Bottom() > noWorkArea.Bounds.Bottom() {
		t.Fatalf("the window was not slid back into the display: %s", inside)
	}
}
