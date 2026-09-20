package application

import (
	"context"
	"errors"
	"testing"
)

// FR-067. After a sign-in on the reference machine the taskbar buttons of the
// applications the agent had started were drawn grey; they stayed that way
// until the user clicked anywhere on the taskbar. Asking the taskbars to
// repaint was tried and measured not to fix it, so the restore tells the shell
// its icons may have changed instead.
func TestASignInRestoreTellsTheShellItsIconsMayHaveChanged(t *testing.T) {
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
	if told := desktop.refreshCount(); told != 1 {
		t.Fatalf("the shell was told %d time(s)", told)
	}
	if !log.saying("icons may have changed") {
		t.Error("the log does not record it, so a boot could not be read afterwards")
	}
}

// A restore the user asked for never showed the fault, so the shell is left
// alone: telling it after every Apply would be a habit rather than a repair.
func TestARestoreTheUserAskedForLeavesTheShellAlone(t *testing.T) {
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
	if told := desktop.refreshCount(); told != 0 {
		t.Fatalf("Apply told the shell %d time(s)", told)
	}
}

// A shell that cannot be told changes nothing about the desktop, so it is
// noted in the log and nowhere else: the user has their windows back.
func TestAShellThatCannotBeToldIsNotedAndNothingMore(t *testing.T) {
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
	if !log.saying("was not told") {
		t.Error("the log does not say the shell was not told")
	}
	if anyContaining(report.SortedNotes(), "icons") {
		t.Errorf("the report troubles the user with it: %v", report.SortedNotes())
	}
}
