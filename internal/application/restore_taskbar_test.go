package application

import (
	"context"
	"errors"
	"testing"
)

// FR-067. After a sign-in on the reference machine the taskbar buttons of the
// applications the agent had started were drawn without their icons; they
// stayed that way until the user clicked anywhere on the taskbar: the icons were
// there, the drawing of them was stale. The restore asks for the same
// invalidation that click causes.
func TestASignInRestoreAsksTheTaskbarToRedrawItself(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, claude, at(0))},
	}
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(claude),
		&fakeLauncher{}, newFakeStore(deskProfile(t).WithDefault(true)), newFakeClock(), log)

	if _, marked, err := service.RestoreDefault(context.Background(), true); err != nil || !marked {
		t.Fatalf("restoring at sign-in: %v, marked %v", err, marked)
	}
	if asked := desktop.refreshCount(); asked != 1 {
		t.Fatalf("the taskbars were asked to redraw %d time(s)", asked)
	}
	if !log.saying("redraw themselves") {
		t.Error("the log does not record it, so a boot could not be read afterwards")
	}
}

// A restore the user asked for never showed the fault, so it is left alone:
// poking the shell after every Apply would be a habit rather than a repair.
func TestARestoreTheUserAskedForLeavesTheTaskbarAlone(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, claude, at(0))},
	}
	service := restoreUnder(desktop, newFakeProcesses(claude),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{})

	if _, err := service.Restore(context.Background(), deskProfile(t)); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if asked := desktop.refreshCount(); asked != 0 {
		t.Fatalf("Apply asked the taskbars to redraw %d time(s)", asked)
	}
}

// A shell that will not be asked changes nothing about the desktop, so it is
// noted in the log and nowhere else: the user has their windows back.
func TestATaskbarThatCannotBeAskedIsNotedAndNothingMore(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays:   []Display{primaryDisplay},
		windows:    []Window{aWindow(1, claude, at(0))},
		refreshErr: errors.New("the shell is not answering"),
	}
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(claude),
		&fakeLauncher{}, newFakeStore(deskProfile(t).WithDefault(true)), newFakeClock(), log)

	report, marked, err := service.RestoreDefault(context.Background(), true)
	if err != nil || !marked {
		t.Fatalf("restoring at sign-in: %v, marked %v", err, marked)
	}
	if !log.saying("not asked to redraw") {
		t.Error("the log does not say the taskbar was not asked")
	}
	if anyContaining(report.SortedNotes(), "taskbar") {
		t.Errorf("the report troubles the user with it: %v", report.SortedNotes())
	}
}
