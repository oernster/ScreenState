package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// FR-073: a settled restore puts the taskbar button of every window it placed
// back in its ordinary state; the log says how many were drawing attention.
func TestASettledRestoreSettlesTheButtonsOfTheWindowsItPlaced(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays:  []Display{primaryDisplay},
		windows:   []Window{aWindow(1, pigeonpost, at(0))},
		attention: map[WindowID]bool{1: true},
	}
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), log)

	if _, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary)); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	settled := desktop.settledButtons()
	if len(settled) != 1 || settled[0] != WindowID(1) {
		t.Fatalf("the buttons settled were %v", settled)
	}
	if !log.saying("1 of which were drawing attention") {
		t.Error("the log does not say how many were drawing attention")
	}
}

// A window this restore never placed is left alone: its button may be lit
// because something really does want the user.
func TestAWindowTheRestoreNeverPlacedIsLeftAlone(t *testing.T) {
	t.Parallel()
	stranger := aWindow(2, nordvpn, at(1))
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0)), stranger},
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, nordvpn),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{})

	if _, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary)); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	for _, settled := range desktop.settledButtons() {
		if settled == stranger.ID {
			t.Fatal("a window the profile does not name had its button settled")
		}
	}
}

// An entry that says only that the application should be running places no
// window, so there is no button to settle and nothing to say about it.
func TestAnEntryThatPlacesNothingSettlesNoButton(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	profile, _ := domain.NewProfile("Desk", domain.Entry{Application: pigeonpost, Running: true})
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), log)

	if _, err := service.Restore(context.Background(), profile); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if settled := desktop.settledButtons(); len(settled) != 0 {
		t.Fatalf("buttons were settled with nothing placed: %v", settled)
	}
	if log.saying("taskbar button(s) were settled") {
		t.Error("the log talks about buttons that do not exist")
	}
}

// A button that cannot be settled changes nothing about the desktop, so it is
// noted in the log and nowhere else.
func TestAButtonThatCannotBeSettledIsNotedAndNothingMore(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
		stopErr:  errors.New("the window is not answering"),
	}
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), log)

	report, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if !log.saying("was not settled") {
		t.Error("the log does not say the button was not settled")
	}
	if anyContaining(report.SortedNotes(), "button") {
		t.Errorf("the report troubles the user with it: %v", report.SortedNotes())
	}
}
