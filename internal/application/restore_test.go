package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// The worked example of section 5, reduced to what a test can hold: an
// application already running with a window to place, one that has to be
// launched and one recorded as running with no placement at all.
func TestARestorePlacesEveryEntryAndSaysSo(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay, leftDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	processes := newFakeProcesses(pigeonpost, nordvpn)
	launcher := &fakeLauncher{}
	clock := newFakeClock()
	log := &fakeLog{}

	// Stellody does not start by itself, so it appears only once launched.
	launcher.onLaunch = func(application domain.ApplicationIdentity) {
		if !application.Equal(stellody) {
			return
		}
		processes.start(stellody)
		desktop.addWindow(aWindow(2, stellody, at(1)))
	}

	profile, err := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true, Placements: []domain.Placement{
			aPlacement(primaryID, domain.Rect{X: 0, Y: 0, Width: 3440, Height: 1392})}},
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{
			aPlacement(leftID, domain.Rect{X: -3840, Y: 0, Width: 3840, Height: 2352})}},
		domain.Entry{Application: nordvpn, Running: true},
	)
	if err != nil {
		t.Fatalf("the profile is not valid: %v", err)
	}

	service := restoreUnder(desktop, processes, launcher, newFakeStore(), clock, log)
	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}

	satisfied, outstanding := report.Counts()
	if satisfied != 3 || outstanding != 0 {
		t.Fatalf("%d satisfied, %d outstanding: %s", satisfied, outstanding, report.Summary())
	}
	if launcher.launchCount(pigeonpost) != 0 {
		t.Fatal("an application already running was launched again")
	}
	if launcher.launchCount(stellody) != 1 {
		t.Fatalf("Stellody was launched %d times", launcher.launchCount(stellody))
	}
	placements := desktop.placements()
	if len(placements) != 2 {
		t.Fatalf("placed %d windows, wanted 2", len(placements))
	}
	if placements[0].state != domain.ShowMaximised {
		t.Fatalf("Claude was left showing %s", placements[0].state)
	}
	if !log.saying("none outstanding") {
		t.Fatal("the log does not record the summary")
	}
}

// FR-005: an entry recording an application as running with no placement is
// satisfied by the application running; nothing of its is moved.
func TestAnEntryWithNoPlacementMovesNothing(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, nordvpn, at(0))},
	}
	service := restoreUnder(desktop, newFakeProcesses(nordvpn), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})

	profile, _ := domain.NewProfile("Tray", domain.Entry{Application: nordvpn, Running: true})
	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if len(desktop.placements()) != 0 {
		t.Fatal("a window was moved for an entry that records no placement")
	}
	if !reportOf(t, report, nordvpn).Satisfied {
		t.Fatal("the entry was not satisfied by the application running")
	}
}

// FR-029: an entry recorded as not running is left entirely alone, because a
// restore ends nothing and closes nothing.
func TestAnEntryRecordedNotRunningIsLeftAlone(t *testing.T) {
	t.Parallel()
	launcher := &fakeLauncher{}
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	service := restoreUnder(desktop, newFakeProcesses(nordvpn), launcher,
		newFakeStore(), newFakeClock(), &fakeLog{})

	profile, _ := domain.NewProfile("Quiet", domain.Entry{Application: nordvpn})
	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if launcher.launchCount(nordvpn) != 0 {
		t.Fatal("an application recorded as not running was launched")
	}
	entry := reportOf(t, report, nordvpn)
	if !entry.Satisfied || !noteSaying(entry, "nothing was done to it") {
		t.Fatalf("the report does not say it was left alone: %+v", entry)
	}
}

// FR-023: the ceiling ends the waiting and names what was left outstanding.
func TestTheCeilingEndsTheWaitAndNamesWhatIsOutstanding(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	clock := newFakeClock()
	service := restoreUnder(desktop, newFakeProcesses(), &fakeLauncher{},
		newFakeStore(), clock, &fakeLog{})

	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{
			aPlacement(primaryID, domain.Rect{X: 0, Y: 0, Width: 800, Height: 600})}})

	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	entry := reportOf(t, report, stellody)
	if entry.Satisfied {
		t.Fatal("an entry whose window never appeared was reported satisfied")
	}
	if entry.Reason == "" {
		t.Fatal("an outstanding entry carries no reason")
	}
	if clock.sleeps() < 30 {
		t.Fatalf("the restore gave up after %d polls, well before the ceiling", clock.sleeps())
	}
}

// FR-055: an entry is placed as its own window appears, without waiting for the
// entries that are still starting.
func TestEachEntryIsPlacedAsItsOwnWindowAppears(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	clock := newFakeClock()
	clock.onSleep = func(_ context.Context, _ *fakeClock, count int) {
		if count == 5 {
			desktop.addWindow(aWindow(2, stellody, at(1)))
		}
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, stellody), &fakeLauncher{},
		newFakeStore(), clock, &fakeLog{})

	rect := domain.Rect{X: 0, Y: 0, Width: 800, Height: 600}
	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true,
			Placements: []domain.Placement{aPlacement(primaryID, rect)}},
		domain.Entry{Application: stellody, Running: true,
			Placements: []domain.Placement{aPlacement(primaryID, rect)}})

	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	placements := desktop.placements()
	if len(placements) != 2 || placements[0].id != 1 || placements[1].id != 2 {
		t.Fatalf("placements out of order: %+v", placements)
	}
	if _, outstanding := report.Counts(); outstanding != 0 {
		t.Fatalf("outstanding entries: %s", report.Summary())
	}
}

// FR-031: a placement naming a display that is not connected goes to the
// primary display; the substitution is recorded rather than passed over.
func TestAPlacementOnAMissingDisplayMovesToThePrimaryOne(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, stellody, at(0))},
	}
	service := restoreUnder(desktop, newFakeProcesses(stellody), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})

	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{
			aPlacement(leftID, domain.Rect{X: -3840, Y: 0, Width: 3840, Height: 2352})}})

	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	entry := reportOf(t, report, stellody)
	if !entry.Satisfied || !noteSaying(entry, "is not connected") {
		t.Fatalf("the substitution was not recorded: %+v", entry)
	}
	placed := desktop.placements()[0].rect
	if placed.X < primaryDisplay.WorkArea.X || placed.Right() > primaryDisplay.WorkArea.Right() {
		t.Fatalf("the window landed outside the primary display at %s", placed)
	}
}

// FR-057: a display going away during a restore does not abandon it.
func TestARestoreContinuesWhenADisplayGoesAway(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay, leftDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	clock := newFakeClock()
	clock.onSleep = func(_ context.Context, _ *fakeClock, count int) {
		if count == 2 {
			desktop.removeDisplay(leftID.MonitorID)
			desktop.addWindow(aWindow(2, stellody, at(1)))
		}
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, stellody), &fakeLauncher{},
		newFakeStore(), clock, &fakeLog{})

	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true, Placements: []domain.Placement{
			aPlacement(primaryID, domain.Rect{X: 0, Y: 0, Width: 800, Height: 600})}},
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{
			aPlacement(leftID, domain.Rect{X: -3840, Y: 0, Width: 800, Height: 600})}})

	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	if _, outstanding := report.Counts(); outstanding != 0 {
		t.Fatalf("the restore abandoned entries: %s", report.Summary())
	}
	if !anyContaining(report.Notes, "went away during the restore") {
		t.Fatalf("the display change was not recorded: %v", report.Notes)
	}
}

// FR-061, the ruling this specification was baselined on: the newer request
// wins, the running restore stops and nothing already placed is put back.
func TestANewerRestoreReplacesTheOneRunning(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	clock := newFakeClock()
	running := make(chan struct{})
	var signalled bool
	// The first restore parks here until its own context ends, which is the
	// only thing that ends it: a newer restore standing it down. Letting it
	// spin instead would let it reach its ceiling and finish on its own before
	// the replacement was asked for, which is a race rather than a test.
	clock.onSleep = func(ctx context.Context, _ *fakeClock, _ int) {
		if signalled {
			return
		}
		signalled = true
		close(running)
		<-ctx.Done()
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), clock, &fakeLog{})

	rect := domain.Rect{X: 0, Y: 0, Width: 800, Height: 600}
	first, _ := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true,
			Placements: []domain.Placement{aPlacement(primaryID, rect)}},
		// This entry never appears, so the first restore is still waiting when
		// the second is asked for.
		domain.Entry{Application: stellody, Running: true,
			Placements: []domain.Placement{aPlacement(primaryID, rect)}})
	second, _ := domain.NewProfile("Sofa",
		domain.Entry{Application: pigeonpost, Running: true,
			Placements: []domain.Placement{aPlacement(primaryID, rect)}})

	type outcome struct {
		report *Report
		err    error
	}
	firstDone := make(chan outcome, 1)
	go func() {
		report, err := service.Restore(context.Background(), first)
		firstDone <- outcome{report: report, err: err}
	}()
	<-running

	secondReport, err := service.Restore(context.Background(), second)
	if err != nil {
		t.Fatalf("the replacing restore failed: %v", err)
	}
	replaced := <-firstDone

	if !errors.Is(replaced.err, ErrRestoreReplaced) {
		t.Fatalf("the replaced restore answered %v", replaced.err)
	}
	if !replaced.report.WasReplaced {
		t.Fatal("the replaced restore's report does not say it was replaced")
	}
	if !anyContaining(replaced.report.Notes, "left where it was") {
		t.Fatalf("the replaced report does not say nothing was put back: %v", replaced.report.Notes)
	}
	if !secondReport.Replaced {
		t.Fatal("the new restore's report does not say it replaced one")
	}
	if last, held := service.Last(); !held || last.Profile != "Sofa" {
		t.Fatalf("the most recent report is %v", last)
	}
	for _, placement := range desktop.placements() {
		if placement.state != domain.ShowMaximised {
			t.Fatalf("a window was put back rather than left: %+v", placement)
		}
	}
}
