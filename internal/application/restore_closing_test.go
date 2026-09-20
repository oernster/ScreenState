package application

import (
	"context"
	"errors"
	"testing"
)

// FR-064, the other arm. Most of the applications that start with Windows go to
// the notification area when their window is closed, which is where the user
// wanted them; minimising leaves them on the taskbar instead. The setting is
// what makes the choice, so a restore under it asks rather than minimises.
func TestAStrangerIsAskedToCloseWhenTheSettingSaysSo(t *testing.T) {
	t.Parallel()
	stranger := aWindow(2, nordvpn, at(1))
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, claude, at(0)), stranger},
	}
	service := restoreUnder(desktop, newFakeProcesses(claude, nordvpn),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{},
		&fakeStrangers{closing: true})

	report, err := service.Restore(context.Background(), deskProfile(t))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if asked := desktop.closedCount(stranger.ID); asked != 1 {
		t.Fatalf("the stranger was asked to close %d time(s)", asked)
	}
	if put := putAwayCalls(desktop, stranger.ID); len(put) != 0 {
		t.Errorf("the stranger was minimised as well as asked to close")
	}
	if !anyContaining(report.SortedNotes(), "to close") {
		t.Errorf("the report says nothing about it: %v", report.SortedNotes())
	}
}

// A window that is asked to close and is still there has refused: an
// application may prompt about unsaved work or ignore the request outright.
// Leaving it would undo the point of the setting, so it is put away instead and
// the report names it, since only the user can settle a prompt.
func TestAStrangerThatRefusesToCloseIsPutAway(t *testing.T) {
	t.Parallel()
	stranger := aWindow(2, nordvpn, at(1))
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, claude, at(0)), stranger},
		refuses:  map[WindowID]bool{stranger.ID: true},
	}
	service := restoreUnder(desktop, newFakeProcesses(claude, nordvpn),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{},
		&fakeStrangers{closing: true})

	report, err := service.Restore(context.Background(), deskProfile(t))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if put := putAwayCalls(desktop, stranger.ID); len(put) != 1 {
		t.Fatalf("a window that refused to close was put away %d time(s)", len(put))
	}
	if !anyContaining(report.SortedNotes(), "did not close") {
		t.Errorf("the report does not say it refused: %v", report.SortedNotes())
	}
}

// A window that cannot even be asked is named rather than passed over; the
// restore carries on with the rest.
func TestAStrangerThatCannotBeAskedToCloseIsReported(t *testing.T) {
	t.Parallel()
	stranger := aWindow(2, nordvpn, at(1))
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, claude, at(0)), stranger},
		closeErr: map[WindowID]error{stranger.ID: errors.New("the window is gone")},
	}
	service := restoreUnder(desktop, newFakeProcesses(claude, nordvpn),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{},
		&fakeStrangers{closing: true})

	report, err := service.Restore(context.Background(), deskProfile(t))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if !anyContaining(report.SortedNotes(), "could not be asked to close") {
		t.Errorf("the report says nothing about the refusal: %v", report.SortedNotes())
	}
}

// A setting that cannot be read takes the arm that asks nothing of anybody; it
// says why, because a window closed over an unreadable file is not a decision
// the user made.
func TestASettingThatCannotBeReadMinimises(t *testing.T) {
	t.Parallel()
	stranger := aWindow(2, nordvpn, at(1))
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, claude, at(0)), stranger},
	}
	service := restoreUnder(desktop, newFakeProcesses(claude, nordvpn),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{},
		&fakeStrangers{closing: true, readErr: errors.New("the settings file is unreadable")})

	report, err := service.Restore(context.Background(), deskProfile(t))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if asked := desktop.closedCount(stranger.ID); asked != 0 {
		t.Fatalf("a window was asked to close under a setting nobody could read")
	}
	if put := putAwayCalls(desktop, stranger.ID); len(put) != 1 {
		t.Fatalf("the stranger was put away %d time(s)", len(put))
	}
	if !anyContaining(report.SortedNotes(), "could not be") {
		t.Errorf("the report does not say why: %v", report.SortedNotes())
	}
}

// The setting is read and written through the restore service, which is what
// the manager's own settings panel calls.
func TestTheSettingIsReadAndWrittenThroughTheRestoreService(t *testing.T) {
	t.Parallel()
	choice := &fakeStrangers{}
	log := &fakeLog{}
	service := restoreUnder(&fakeDesktop{}, newFakeProcesses(),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), log, choice)

	if closing, err := service.CloseStrangers(); err != nil || closing {
		t.Fatalf("a user who has chosen nothing got closing: %v, %v", closing, err)
	}
	if err := service.SetCloseStrangers(true); err != nil {
		t.Fatalf("turning it on: %v", err)
	}
	if closing, err := service.CloseStrangers(); err != nil || !closing {
		t.Fatalf("it did not stay on: %v, %v", closing, err)
	}
	if !log.saying("asked to close") {
		t.Error("the log does not record the change")
	}
	if err := service.SetCloseStrangers(false); err != nil {
		t.Fatalf("turning it off: %v", err)
	}
	if !log.saying("will be minimised") {
		t.Error("the log does not record it going back")
	}
}

// A setting that cannot be written is an error the manager shows rather than a
// change it claims to have made.
func TestASettingThatCannotBeWrittenIsRefused(t *testing.T) {
	t.Parallel()
	refused := errors.New("the settings file is read only")
	service := restoreUnder(&fakeDesktop{}, newFakeProcesses(),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{},
		&fakeStrangers{writeErr: refused})

	if err := service.SetCloseStrangers(true); !errors.Is(err, refused) {
		t.Fatalf("a write that failed answered %v", err)
	}
}
