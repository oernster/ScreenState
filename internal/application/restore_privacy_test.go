package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// NFR-PRIV-001: nothing is stored beyond application identities, geometry, show
// states, display identities and profile names. A sign-in restore's report is
// written to the log, so a window title reaching the report is a title stored
// on disk; a title can say what the user is working on. Measured on 2026-09-21:
// the log named the windows put away by title.

// secretTitle is a title that must never be written anywhere.
const secretTitle = "Salary review - confidential"

// titled is a window of the application carrying the secret title.
func titled(id WindowID, application domain.ApplicationIdentity) Window {
	window := aWindow(id, application, at(int(id)))
	window.Description = secretTitle
	return window
}

// everythingSaid is every word a restore wrote down: its notes, each entry's
// reason and notes, then the steps it logged.
func everythingSaid(report *Report, log *fakeLog) []string {
	said := append([]string{report.Summary()}, report.SortedNotes()...)
	for _, entry := range report.Entries() {
		said = append(said, entry.Reason)
		said = append(said, entry.Notes...)
	}
	log.mutex.Lock()
	defer log.mutex.Unlock()
	return append(said, log.steps...)
}

// sayNoTitle fails the test on any word that carries the secret title, then
// on a report that does not name the stranger by its application instead.
func sayNoTitle(t *testing.T, report *Report, log *fakeLog, stranger domain.ApplicationIdentity) {
	t.Helper()
	said := everythingSaid(report, log)
	for _, words := range said {
		if strings.Contains(words, secretTitle) {
			t.Errorf("a window title was written down: %q", words)
		}
	}
	if !anyContaining(said, stranger.String()) {
		t.Errorf("the window put away is not named by its application: %v", said)
	}
}

// A window put away by minimising is named by its application (FR-063).
func TestAWindowPutAwayIsNamedByItsApplicationNotItsTitle(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0)), titled(2, nordvpn)},
	}
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, nordvpn),
		&fakeLauncher{}, newFakeStore(deskProfile(t).WithDefault(true)), newFakeClock(), log)

	report, _, err := service.RestoreDefault(context.Background())
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	sayNoTitle(t, report, log, nordvpn)
}

// Every sentence FR-064 writes about a window names it the same way: asked to
// close, refusing and so put away, failing to be asked, failing to be put away.
func TestNoWordAboutClosingCarriesATitle(t *testing.T) {
	t.Parallel()
	refuses, unaskable, stuck := titled(2, nordvpn), titled(3, stellody), titled(4, notepad)
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0)), refuses, unaskable, stuck},
		refuses:  map[WindowID]bool{refuses.ID: true, stuck.ID: true},
		closeErr: map[WindowID]error{unaskable.ID: errors.New("access is denied")},
		placeErr: map[WindowID]error{stuck.ID: errors.New("access is denied")},
	}
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, nordvpn, stellody, notepad),
		&fakeLauncher{}, newFakeStore(deskProfile(t).WithDefault(true)), newFakeClock(), log,
		&fakeStrangers{closing: true})

	report, _, err := service.RestoreDefault(context.Background())
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	for _, fragment := range []string{"could not be asked to close", "did not close", "could not be put away"} {
		if !anyContaining(report.SortedNotes(), fragment) {
			t.Errorf("the report has no note saying %q: %v", fragment, report.SortedNotes())
		}
	}
	sayNoTitle(t, report, log, nordvpn)
}
