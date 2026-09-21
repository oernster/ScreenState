package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// FR-069 at sign-in with the application already running: it shows one of the
// two windows the profile records, so it is run again for the second; both are
// placed, oldest first (FR-037). The desktop is being rebuilt from nothing at
// sign-in, so the windows that are open are not evidence of what the user
// wants.
func TestASignInRunsARunningApplicationAgainForAWindowItIsMissing(t *testing.T) {
	t.Parallel()
	second := aPlacement(primaryID, domain.Rect{X: 100, Y: 100, Width: 400, Height: 300})
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	launcher := &fakeLauncher{}
	launcher.onLaunch = func(domain.ApplicationIdentity) { desktop.addWindow(aWindow(2, pigeonpost, at(1))) }
	profile, _ := domain.NewProfile("Desk", domain.Entry{
		Application: pigeonpost, Running: true,
		Placements: []domain.Placement{onPrimary, second},
	})
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost), launcher,
		newFakeStore(profile.WithDefault(true)), newFakeClock(), &fakeLog{})
	report, _, err := service.RestoreDefault(context.Background(), true)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if !reportOf(t, report, pigeonpost).Satisfied {
		t.Fatalf("the entry never settled: %+v", reportOf(t, report, pigeonpost))
	}
	placements := desktop.placements()
	if len(placements) != 2 || placements[0].id != 1 || placements[1].id != 2 {
		t.Fatalf("the windows were not placed oldest first: %+v", placements)
	}
	if launcher.launchCount(pigeonpost) != 1 {
		t.Fatalf("it was run %d times for one missing window", launcher.launchCount(pigeonpost))
	}
}

// An application that stops answering after it was launched is named, rather
// than the restore waiting on a question it can no longer ask.
func TestAnApplicationThatStopsAnsweringWhileWaitedForIsNamed(t *testing.T) {
	t.Parallel()
	processes := newFakeProcesses()
	clock := newFakeClock()
	clock.onSleep = func(_ context.Context, _ *fakeClock, count int) {
		if count == 2 {
			processes.mutex.Lock()
			processes.err = errRefused
			processes.mutex.Unlock()
		}
	}
	service := restoreUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		processes, &fakeLauncher{}, newFakeStore(), clock, &fakeLog{})

	profile, _ := domain.NewProfile("Tray", domain.Entry{Application: nordvpn, Running: true})
	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	entry := reportOf(t, report, nordvpn)
	if entry.Satisfied || !containsText(entry.Reason, "could not be read") {
		t.Fatalf("the failure was not reported: %+v", entry)
	}
}

// A capture says which reading of the desktop failed, exactly as a restore does.
func TestACaptureSaysWhichReadingFailed(t *testing.T) {
	t.Parallel()
	byWindows := captureUnder(&fakeDesktop{windowsErr: errRefused}, newFakeProcesses(), newFakeStore())
	if _, err := byWindows.Review(context.Background(), ""); !containsText(err.Error(), "reading the windows") {
		t.Fatalf("reported as %v", err)
	}
	byDisplays := captureUnder(&fakeDesktop{displaysErr: errRefused}, newFakeProcesses(), newFakeStore())
	if _, err := byDisplays.Review(context.Background(), ""); !containsText(err.Error(), "reading the displays") {
		t.Fatalf("reported as %v", err)
	}
	none := captureUnder(&fakeDesktop{}, newFakeProcesses(), newFakeStore())
	if _, err := none.Review(context.Background(), ""); !errors.Is(err, ErrNoDisplays) {
		t.Fatalf("expected ErrNoDisplays, got %v", err)
	}
}

// A capture based on a profile that cannot be read stops rather than quietly
// capturing less than it was asked for.
func TestACaptureStopsWhenTheProfileCannotBeRead(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.loadErr = errRefused
	service := captureUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses(), store)
	if _, err := service.Review(context.Background(), "Desk"); !containsText(err.Error(), "reading profile") {
		t.Fatalf("reported as %v", err)
	}

	unreadable := newFakeProcesses()
	unreadable.err = errRefused
	profile, _ := domain.NewProfile("Desk", domain.Entry{Application: nordvpn, Running: true})
	held := captureUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		unreadable, newFakeStore(profile))
	if _, err := held.Review(context.Background(), "Desk"); !containsText(err.Error(), "is running") {
		t.Fatalf("reported as %v", err)
	}
}

// An application open now and named by the profile appears once, not twice.
func TestAnApplicationInBothTheDesktopAndTheProfileAppearsOnce(t *testing.T) {
	t.Parallel()
	profile, _ := domain.NewProfile("Desk", domain.Entry{Application: pigeonpost, Running: true})
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	service := captureUnder(desktop, newFakeProcesses(pigeonpost), newFakeStore(profile))

	review, err := service.Review(context.Background(), "Desk")
	if err != nil {
		t.Fatalf("the capture failed: %v", err)
	}
	if len(review.Entries) != 1 {
		t.Fatalf("captured %d entries: %+v", len(review.Entries), review.Entries)
	}
	if len(review.Entries[0].Placements) != 1 {
		t.Fatal("the entry read from the desktop was replaced by the profile's")
	}
}

// A store that will not answer or will not write is reported as it is.
func TestAStoreThatWillNotAnswerIsReported(t *testing.T) {
	t.Parallel()
	entries := []domain.Entry{{Application: pigeonpost, Running: true}}

	cannotList := newFakeStore()
	cannotList.namesErr = errRefused
	listing := captureUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses(), cannotList)
	if _, err := listing.Save(context.Background(), "Desk", entries, false); !containsText(err.Error(), "listing the profiles") {
		t.Fatalf("reported as %v", err)
	}

	cannotSave := newFakeStore()
	cannotSave.saveErr = errRefused
	saving := captureUnder(&fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses(), cannotSave)
	if _, err := saving.Save(context.Background(), "Desk", entries, false); !containsText(err.Error(), "writing profile") {
		t.Fatalf("reported as %v", err)
	}
}

// The report answers the two questions the tray and the report window ask of
// it: what is outstanding; what happened to the restore as a whole.
func TestAReportListsWhatIsOutstandingAndWhatHappened(t *testing.T) {
	t.Parallel()
	report := NewReport("Desk", time.Now())
	report.Track(pigeonpost)
	report.Track(stellody)
	report.Satisfy(pigeonpost)
	report.Fail(stellody, "did not start")
	report.NoteEntry(pigeonpost, "placed at once")
	report.Note("a display went away")
	report.Note("a display was connected")

	outstanding := report.Outstanding()
	if len(outstanding) != 1 || !outstanding[0].Application.Equal(stellody) {
		t.Fatalf("outstanding reads %+v", outstanding)
	}
	if outstanding[0].Reason != "did not start" {
		t.Fatalf("the reason reads %q", outstanding[0].Reason)
	}
	sorted := report.SortedNotes()
	if len(sorted) != 2 || sorted[0] != "a display was connected" {
		t.Fatalf("the notes are not in a settled order: %v", sorted)
	}
	if summary := report.Summary(); !containsText(summary, "1 outstanding") {
		t.Fatalf("the summary reads %q", summary)
	}

	// An entry satisfied after a failure carries no stale reason.
	report.Satisfy(stellody)
	if len(report.Outstanding()) != 0 {
		t.Fatal("a satisfied entry is still reported outstanding")
	}
	if reportOf(t, report, stellody).Reason != "" {
		t.Fatal("a satisfied entry kept the reason it failed with")
	}
}
