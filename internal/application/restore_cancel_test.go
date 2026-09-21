package application

import (
	"context"
	"testing"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// FR-049: a restore the user cancels stops before its next action, leaves what
// it placed and says it was cancelled; nothing is watched afterwards.
func TestTheUserCancellingARestoreStopsIt(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	clock := newFakeClock()
	log := &fakeLog{}
	events := &fakeEvents{clock: clock}
	service := restoreWatching(desktop, clock, log, events, pigeonpost, stellody)
	cancelled := make(chan bool, 1)
	// The first wait is where the user presses Stop: Cancel is asked from
	// elsewhere (as the manager asks it) while the wait holds until the
	// restore's context ends.
	clock.onSleep = func(ctx context.Context, _ *fakeClock, count int) {
		if count == 1 {
			go func() { cancelled <- service.Cancel() }()
			<-ctx.Done()
		}
	}
	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true, Placements: []domain.Placement{onPrimary}},
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{onPrimary}})

	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("a cancelled restore answered an error: %v", err)
	}
	select {
	case stopped := <-cancelled:
		if !stopped {
			t.Fatal("Cancel said there was no restore to stop")
		}
	case <-time.After(testPatience):
		t.Fatal("Cancel never returned")
	}
	if !anyContaining(report.Notes, "cancelled") {
		t.Fatalf("the report does not say the restore was cancelled: %v", report.Notes)
	}
	if !reportOf(t, report, pigeonpost).Satisfied {
		t.Error("the window already placed was not left placed")
	}
	if reportOf(t, report, stellody).Satisfied {
		t.Error("the entry still awaited was reported satisfied")
	}
	events.mutex.Lock()
	watches := events.watches
	events.mutex.Unlock()
	if watches != 1 {
		t.Fatalf("%d watches were begun: a cancelled restore went on watching", watches)
	}
}

// With nothing running there is nothing to cancel; Cancel says so.
func TestCancelWithNothingRunningSaysSo(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	service := restoreWatching(&fakeDesktop{}, clock, &fakeLog{}, &fakeEvents{clock: clock})
	if service.Cancel() {
		t.Fatal("Cancel claimed to stop a restore when none was running")
	}
}
