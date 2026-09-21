package application

import (
	"context"
	"strings"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// Starting an application and asking a running one to show its window: what a
// restore launches, when it runs something a second time and what it reports
// when either fails (FR-025, FR-026, FR-036, FR-056).

// FR-026: an application that cannot be launched is named with its reason and
// the restore continues.
func TestAnApplicationThatCannotBeLaunchedIsNamed(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	launcher := &fakeLauncher{refuse: map[string]error{
		strings.ToLower(stellody.String()): errRefused,
	}}
	service := restoreUnder(desktop, newFakeProcesses(nordvpn), launcher,
		newFakeStore(), newFakeClock(), &fakeLog{})

	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{onPrimary}},
		domain.Entry{Application: nordvpn, Running: true})

	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	entry := reportOf(t, report, stellody)
	if entry.Satisfied || !containsText(entry.Reason, "could not be launched") {
		t.Fatalf("the launch failure was not reported: %+v", entry)
	}
	if !reportOf(t, report, nordvpn).Satisfied {
		t.Fatal("a launch failure stopped the rest of the restore")
	}
}

// FR-036 and FR-056: a running application holding its window hidden is asked
// to show it by being run again; the window that appears is placed.
func TestARunningApplicationIsAskedToShowItsHiddenWindow(t *testing.T) {
	t.Parallel()
	hidden := aWindow(1, nordvpn, at(0))
	hidden.Visible = false
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}, windows: []Window{hidden}}
	launcher := &fakeLauncher{}
	launcher.onLaunch = func(domain.ApplicationIdentity) { desktop.showWindow(1) }

	service := restoreUnder(desktop, newFakeProcesses(nordvpn), launcher,
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(nordvpn, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if launcher.launchCount(nordvpn) != 1 {
		t.Fatalf("it was run again %d times", launcher.launchCount(nordvpn))
	}
	entry := reportOf(t, report, nordvpn)
	if !entry.Satisfied || !noteSaying(entry, "asked to show one") {
		t.Fatalf("the hidden window was not asked for and placed: %+v", entry)
	}
}

// windowArrivesOnRead is the read of the desktop on which a freshly launched
// application's window first appears: its process is up well before that.
const windowArrivesOnRead = 3

// FR-025: an application this restore has just started is still starting rather
// than holding a window hidden, so it is not run a second time while its window
// is on the way. Measured on the reference machine on 2026-09-21: the log named
// two processes for each packaged application, started in the same second.
func TestAnApplicationJustLaunchedIsNotRunAgainWhileItsWindowIsComing(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	processes := newFakeProcesses()
	launcher := &fakeLauncher{}
	launcher.onLaunch = func(domain.ApplicationIdentity) { processes.start(stellody) }
	reads := 0
	desktop.onWindows = func() {
		reads++
		if reads == windowArrivesOnRead {
			desktop.addWindow(aWindow(1, stellody, at(0)))
		}
	}
	service := restoreUnder(desktop, processes, launcher,
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(stellody, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if launcher.launchCount(stellody) != 1 {
		t.Fatalf("it was started %d times rather than once", launcher.launchCount(stellody))
	}
	if !reportOf(t, report, stellody).Satisfied {
		t.Fatalf("the window that arrived was not placed: %+v", reportOf(t, report, stellody))
	}
}

// FR-036: an application this restore starts that goes straight to the
// notification area is still asked to show its window, once its launch has had
// the time any window is given to settle.
func TestAnApplicationThatStartsIntoTheTrayIsAskedOnceItHasSettled(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	processes := newFakeProcesses()
	launcher := &fakeLauncher{}
	launcher.onLaunch = func(domain.ApplicationIdentity) {
		if launcher.launchCount(nordvpn) == 1 {
			hidden := aWindow(1, nordvpn, at(0))
			hidden.Visible = false
			processes.start(nordvpn)
			desktop.addWindow(hidden)
			return
		}
		desktop.showWindow(1)
	}
	service := restoreUnder(desktop, processes, launcher,
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(nordvpn, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if launcher.launchCount(nordvpn) != 2 {
		t.Fatalf("it was run %d times rather than started then asked", launcher.launchCount(nordvpn))
	}
	entry := reportOf(t, report, nordvpn)
	if !entry.Satisfied || !noteSaying(entry, "asked to show one") {
		t.Fatalf("the window it hid was not asked for and placed: %+v", entry)
	}
}

// FR-069, its acceptance case: a profile recording two Windows Terminal windows
// with no Terminal running. Terminal is started, run once more after its first
// window appears and both windows are placed. Each run opens one window, which
// arrives a few reads after the run, as a real one does.
func TestEveryWindowAProfileRecordsIsOpened(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	processes := newFakeProcesses()
	launcher := &fakeLauncher{}
	var owed []WindowID
	launcher.onLaunch = func(domain.ApplicationIdentity) {
		processes.start(notepad)
		owed = append(owed, WindowID(launcher.launchCount(notepad)))
	}
	reads := 0
	desktop.onWindows = func() {
		reads++
		if len(owed) > 0 && reads%windowArrivesOnRead == 0 {
			desktop.addWindow(aWindow(owed[0], notepad, at(int(owed[0]))))
			owed = owed[1:]
		}
	}
	second := aPlacement(primaryID, domain.Rect{X: 100, Y: 100, Width: 400, Height: 300})
	profile, _ := domain.NewProfile("Desk", domain.Entry{
		Application: notepad, Running: true,
		Placements: []domain.Placement{onPrimary, second},
	})
	service := restoreUnder(desktop, processes, launcher,
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if launcher.launchCount(notepad) != 2 {
		t.Fatalf("it was run %d times for two windows", launcher.launchCount(notepad))
	}
	if entry := reportOf(t, report, notepad); !entry.Satisfied {
		t.Fatalf("both windows were not placed: %+v", entry)
	}
	if placed := desktop.placements(); len(placed) != 2 {
		t.Fatalf("%d window(s) were placed rather than two", len(placed))
	}
}

// FR-069: an application that allows one copy opens nothing when run again, so
// the asking ends after that one run and the report says how many opened,
// rather than the restore waiting on it until the ceiling.
func TestAnApplicationThatOpensNoFurtherWindowIsReported(t *testing.T) {
	t.Parallel()
	second := aPlacement(primaryID, domain.Rect{X: 100, Y: 100, Width: 400, Height: 300})
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	launcher := &fakeLauncher{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost), launcher,
		newFakeStore(), newFakeClock(), &fakeLog{})

	profile, _ := domain.NewProfile("Desk", domain.Entry{
		Application: pigeonpost, Running: true,
		Placements: []domain.Placement{onPrimary, second},
	})
	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if launcher.launchCount(pigeonpost) != 1 {
		t.Fatalf("it was run %d times rather than once", launcher.launchCount(pigeonpost))
	}
	entry := reportOf(t, report, pigeonpost)
	if entry.Satisfied || !containsText(entry.Reason, "opened 1 of the 2 windows") {
		t.Fatalf("the report does not say how many opened: %+v", entry)
	}
}

// FR-036: an application that shows no window after being asked is reported.
func TestAnApplicationThatShowsNoWindowIsReported(t *testing.T) {
	t.Parallel()
	hidden := aWindow(1, nordvpn, at(0))
	hidden.Visible = false
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}, windows: []Window{hidden}}
	launcher := &fakeLauncher{}
	service := restoreUnder(desktop, newFakeProcesses(nordvpn), launcher,
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(nordvpn, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if launcher.launchCount(nordvpn) != 1 {
		t.Fatalf("it was asked %d times rather than once", launcher.launchCount(nordvpn))
	}
	entry := reportOf(t, report, nordvpn)
	if entry.Satisfied || !containsText(entry.Reason, "showed no window") {
		t.Fatalf("the report does not say it never showed a window: %+v", entry)
	}
}

// An application that refuses to be run again is reported rather than retried.
func TestAnApplicationThatRefusesToBeAskedIsReported(t *testing.T) {
	t.Parallel()
	hidden := aWindow(1, nordvpn, at(0))
	hidden.Visible = false
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}, windows: []Window{hidden}}
	launcher := &fakeLauncher{refuse: map[string]error{
		strings.ToLower(nordvpn.String()): errRefused,
	}}
	service := restoreUnder(desktop, newFakeProcesses(nordvpn), launcher,
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), oneEntry(nordvpn, onPrimary))
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if entry := reportOf(t, report, nordvpn); entry.Satisfied ||
		!containsText(entry.Reason, "could not be asked") {
		t.Fatalf("the refusal was not reported: %+v", entry)
	}
}

// FR-070: a packaged application is launched and left where it opens, even
// where a profile recorded before that rule holds placements for it. It is
// satisfied once it is running and none of its windows is moved.
func TestAPackagedApplicationIsLaunchedAndNeverPlaced(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	processes := newFakeProcesses()
	launcher := &fakeLauncher{}
	launcher.onLaunch = func(domain.ApplicationIdentity) {
		processes.start(packaged)
		desktop.addWindow(aWindow(1, packaged, at(0)))
	}
	second := aPlacement(primaryID, domain.Rect{X: 100, Y: 100, Width: 400, Height: 300})
	profile, _ := domain.NewProfile("Desk", domain.Entry{
		Application: packaged, Running: true,
		Placements: []domain.Placement{onPrimary, second},
	})
	service := restoreUnder(desktop, processes, launcher,
		newFakeStore(), newFakeClock(), &fakeLog{})

	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if launcher.launchCount(packaged) != 1 {
		t.Fatalf("it was run %d times rather than once", launcher.launchCount(packaged))
	}
	if !reportOf(t, report, packaged).Satisfied {
		t.Fatalf("it was not satisfied by running: %+v", reportOf(t, report, packaged))
	}
	if placed := desktop.placements(); len(placed) != 0 {
		t.Fatalf("its windows were moved: %+v", placed)
	}
}
