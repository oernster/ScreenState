package application

import (
	"strings"
	"testing"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// anyContaining reports whether any line holds the fragment.
func anyContaining(lines []string, fragment string) bool {
	for _, line := range lines {
		if strings.Contains(line, fragment) {
			return true
		}
	}
	return false
}

// The reference machine's arrangement, from section 5 of the specification,
// reduced to the two displays a test needs: a primary above and a second one
// to its left, whose coordinates are negative exactly as the real one's are.
var (
	primaryID = domain.DisplayIdentity{MonitorID: "DISPLAY#HSJ1340#5&14514d51&0&UID4354"}
	leftID    = domain.DisplayIdentity{MonitorID: "DISPLAY#HSJ1340#5&14514d51&0&UID4356"}

	primaryDisplay = Display{
		Identity: primaryID,
		Bounds:   domain.Rect{X: 0, Y: 0, Width: 3440, Height: 1440},
		WorkArea: domain.Rect{X: 0, Y: 0, Width: 3440, Height: 1392},
		Primary:  true,
	}
	leftDisplay = Display{
		Identity: leftID,
		Bounds:   domain.Rect{X: -3840, Y: 0, Width: 3840, Height: 2400},
		WorkArea: domain.Rect{X: -3840, Y: 0, Width: 3840, Height: 2352},
	}
)

var (
	pigeonpost = domain.ApplicationIdentity{Value: `C:\Programs\PigeonPost\PigeonPost.exe`}
	stellody   = domain.ApplicationIdentity{Value: `C:\Programs\Stellody\Stellody.exe`}
	nordvpn    = domain.ApplicationIdentity{Value: `C:\Programs\NordVPN\NordVPN.exe`}
	// notepad opens another window each time it is run (FR-069).
	notepad  = domain.ApplicationIdentity{Value: `C:\Windows\notepad.exe`}
	screenst = domain.ApplicationIdentity{Value: `C:\Programs\ScreenState\ScreenState.exe`}
)

// at is a moment, for ordering windows by the age this product cares about.
func at(minute int) time.Time {
	return time.Date(2026, time.September, 20, 8, minute, 0, 0, time.UTC)
}

// aWindow returns a visible window of an application, on the primary display.
func aWindow(id WindowID, application domain.ApplicationIdentity, created time.Time) Window {
	return Window{
		ID:          id,
		Application: application,
		Rect:        domain.Rect{X: 100, Y: 100, Width: 1200, Height: 800},
		State:       domain.ShowNormal,
		Visible:     true,
		Created:     created,
	}
}

// aPlacement returns a placement on the given display, maximised, which is how
// the worked example records every placed application.
func aPlacement(display domain.DisplayIdentity, rect domain.Rect) domain.Placement {
	return domain.Placement{Display: display, Rect: rect, State: domain.ShowMaximised}
}

// restoreUnder returns a restore service over the given fakes, with timings a
// test can reason about: a poll of one second, a settle check of ten and a
// ceiling of one minute, so a ceiling is reached in sixty polls rather than in
// sixty seconds of a test run.
//
// The setting for the windows a profile does not name is optional, since almost
// no test is about it: one that does not state a setting gets minimising, which
// is what a user who has chosen nothing gets (FR-064).
func restoreUnder(
	desktop *fakeDesktop,
	processes *fakeProcesses,
	launcher *fakeLauncher,
	store *fakeStore,
	clock *fakeClock,
	log *fakeLog,
	strangers ...*fakeStrangers,
) *RestoreService {
	choice := &fakeStrangers{}
	if len(strangers) > 0 {
		choice = strangers[0]
	}
	return restoreShowing(&fakeSplash{}, desktop, processes, launcher, store, clock, log, choice)
}

// restoreShowing is restoreUnder with the splash stated, for the tests that
// read what the splash was told (FR-078).
func restoreShowing(
	splash *fakeSplash,
	desktop *fakeDesktop,
	processes *fakeProcesses,
	launcher *fakeLauncher,
	store *fakeStore,
	clock *fakeClock,
	log *fakeLog,
	strangers *fakeStrangers,
) *RestoreService {
	return NewRestoreService(desktop, processes, launcher, store, clock, log,
		Policy{Ceiling: time.Minute}, &fakeCeilings{}, screenst, strangers, splash, &fakeEvents{clock: clock})
}

// reportOf returns the entry report for one application, failing the test where
// the report does not mention it at all.
func reportOf(t *testing.T, report *Report, application domain.ApplicationIdentity) EntryReport {
	t.Helper()
	for _, entry := range report.Entries() {
		if entry.Application.Equal(application) {
			return entry
		}
	}
	t.Fatalf("the report says nothing about %s", application)
	return EntryReport{}
}

// noteSaying reports whether an entry carries a note containing the fragment.
func noteSaying(entry EntryReport, fragment string) bool {
	return anyContaining(entry.Notes, fragment)
}
