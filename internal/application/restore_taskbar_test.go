package application

import (
	"context"
	"errors"
	"testing"
)

// taskbarsOnTheReferenceMachine is how many taskbars Explorer draws there: one
// per display, measured on 2026-09-21 by the probe that found the repair.
const taskbarsOnTheReferenceMachine = 4

// FR-072: a settled restore posts a click to every taskbar, since Explorer
// draws the buttons of the applications it started without their icons until
// any taskbar is clicked. The log records it, because only the log can say
// afterwards why the taskbar looks as it does.
func TestASettledRestoreSendsTheTaskbarsAClick(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), log)

	if _, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary)); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if sent := desktop.nudgeCount(); sent != 1 {
		t.Fatalf("the taskbars were sent a click %d time(s)", sent)
	}
	if !log.saying("taskbar(s) were sent a click") {
		t.Error("the log does not record it, so a boot could not be read afterwards")
	}
}

// The fault was measured after a sign-in and again mid-session, so the restore
// that runs at sign-in sends the click exactly as an Apply does.
func TestASignInRestoreSendsTheClickToo(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost),
		&fakeLauncher{}, newFakeStore(deskProfile(t).WithDefault(true)), newFakeClock(), &fakeLog{})

	if _, marked, err := service.RestoreDefault(context.Background(), true); err != nil || !marked {
		t.Fatalf("restoring at sign-in: %v, marked %v", err, marked)
	}
	if sent := desktop.nudgeCount(); sent != 1 {
		t.Fatalf("a sign-in restore sent the click %d time(s)", sent)
	}
}

// A taskbar that cannot be clicked changes nothing about the desktop, so it is
// noted in the log and nowhere else: the user has their windows back.
func TestTaskbarsThatCannotBeClickedAreNotedAndNothingMore(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
		nudgeErr: errors.New("the shell is not answering"),
	}
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), log)

	report, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if !log.saying("not sent a click") {
		t.Error("the log does not say the taskbars were not clicked")
	}
	if anyContaining(report.SortedNotes(), "taskbar") {
		t.Errorf("the report troubles the user with it: %v", report.SortedNotes())
	}
}
