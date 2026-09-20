package application

import (
	"context"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// FR-065. The window holds the user on a panel that offers nothing while a
// restore runs, which can be minutes: the bar is what says the wait is going
// somewhere. It counts entries rather than seconds, so the reading has to
// follow what the restore has actually settled.
func TestTheProgressReadingFollowsTheRestore(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, claude, at(0))},
	}
	clock := newFakeClock()
	var duringFirst, duringLater RestoreProgress
	service := restoreUnder(desktop, newFakeProcesses(claude, nordvpn),
		&fakeLauncher{}, newFakeStore(), clock, &fakeLog{})
	clock.onSleep = func(_ context.Context, _ *fakeClock, count int) {
		if count == 1 {
			duringFirst = service.Progress()
			desktop.addWindow(aWindow(2, nordvpn, at(1)))
			return
		}
		if count == 2 {
			duringLater = service.Progress()
		}
	}

	profile, err := domain.NewProfile("Desk",
		domain.Entry{Application: claude, Running: true,
			Placements: []domain.Placement{onPrimary}},
		domain.Entry{Application: nordvpn, Running: true,
			Placements: []domain.Placement{aPlacement(primaryID,
				domain.Rect{X: 10, Y: 10, Width: 400, Height: 300})}},
	)
	if err != nil {
		t.Fatalf("the profile is not valid: %v", err)
	}
	if _, err := service.Restore(context.Background(), profile); err != nil {
		t.Fatalf("the restore failed: %v", err)
	}

	if !duringFirst.Running || duringFirst.Profile != "Desk" || duringFirst.Total != 2 {
		t.Fatalf("the first reading does not describe the restore: %+v", duringFirst)
	}
	if duringFirst.Satisfied != 1 {
		t.Errorf("the entry whose window was already there was not counted: %+v", duringFirst)
	}
	if duringLater.Satisfied != 2 {
		t.Errorf("the entry satisfied on the second pass was not counted: %+v", duringLater)
	}
}

// A restore that has finished leaves a reading saying so, which is what the
// window reads the moment the work ends: a bar left at nine tenths under a
// panel nobody is on is worse than no bar.
func TestTheProgressReadingClearsWhenTheRestoreEnds(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, claude, at(0))},
	}
	service := restoreUnder(desktop, newFakeProcesses(claude),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{})

	if reading := service.Progress(); reading.Running {
		t.Fatalf("a service that has restored nothing says one is running: %+v", reading)
	}
	if _, err := service.Restore(context.Background(), deskProfile(t)); err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	reading := service.Progress()
	if reading.Running || reading.Total != 0 {
		t.Errorf("the reading outlived the restore: %+v", reading)
	}
}
