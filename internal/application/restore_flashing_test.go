package application

import (
	"context"
	"testing"
	"time"
)

// FR-080: before the buttons are rebuilt, the restore waits until none of them
// has flashed for one full flash series, begun again at each flash.

// testSeries is the flash series these tests wait out: long enough to tell a
// wait begun again from one that was not, on a fake clock stepping a second a
// wait.
const testSeries = 7 * time.Second

// flashingRestore runs a sign-in restore of one window already open under the
// given events, answering how long the wait before the rebuild took by the
// fake clock plus what the desktop and the log saw.
func flashingRestore(t *testing.T, ctx context.Context, events *fakeEvents,
	clock *fakeClock) (time.Duration, *fakeDesktop, *fakeLog) {
	t.Helper()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	log := &fakeLog{}
	events.clock = clock
	profile := oneEntry(pigeonpost, onPrimary).WithDefault(true)
	service := NewRestoreService(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(profile), clock, log, Policy{Ceiling: time.Minute}, &fakeCeilings{}, screenst,
		&fakeStrangers{}, &fakeSplash{}, events)
	began := clock.Now()
	if _, _, err := service.RestoreDefault(ctx); err != nil {
		t.Fatalf("restoring at sign-in: %v", err)
	}
	return clock.Now().Sub(began), desktop, log
}

// A flash from a window about to be rebuilt begins the wait again, so the
// rebuild comes a full series after the last flash.
func TestAFlashBeginsTheWaitAgain(t *testing.T) {
	t.Parallel()
	events := &fakeEvents{series: testSeries, flashAt: map[int][]WindowID{5: {1}}}
	waited, desktop, log := flashingRestore(t, context.Background(), events, newFakeClock())

	flashedAt := 5 * fakeStep
	if want := flashedAt + testSeries; waited != want {
		t.Fatalf("the wait took %s, wanted %s: a series after the flash", waited, want)
	}
	if built := desktop.rebuiltButtons(); len(built) != 1 {
		t.Fatalf("the buttons built afresh were %v", built)
	}
	if !log.saying("begun again 1 time(s)") {
		t.Errorf("the log does not say the wait began again: %v", log.steps)
	}
	if waitedAt, builtAt := stepIndex(log, "stop flashing"), stepIndex(log, "built afresh"); waitedAt < 0 || builtAt < waitedAt {
		t.Errorf("the buttons were not rebuilt after the wait: %v", log.steps)
	}
}

// stepIndex answers where the first step saying text is in the log; -1 where
// none is.
func stepIndex(log *fakeLog, text string) int {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	for index, step := range log.steps {
		if containsText(step, text) {
			return index
		}
	}
	return -1
}

// With no flash the wait is one series; a flash from a window not being
// rebuilt changes nothing.
func TestWithoutAFlashFromTheseWindowsTheWaitIsOneSeries(t *testing.T) {
	t.Parallel()
	events := &fakeEvents{series: testSeries, flashAt: map[int][]WindowID{2: {99}}}
	waited, desktop, _ := flashingRestore(t, context.Background(), events, newFakeClock())

	if waited != testSeries {
		t.Fatalf("the wait took %s, wanted one series of %s", waited, testSeries)
	}
	if len(desktop.rebuiltButtons()) != 1 {
		t.Fatal("the button was not rebuilt after the wait")
	}
}

// A desktop that cannot be watched hears no flash, so the wait is one series
// on the clock and the buttons are still rebuilt.
func TestWithoutAWatchTheWaitIsOneSeries(t *testing.T) {
	t.Parallel()
	events := &fakeEvents{series: testSeries, watchErr: errNoHook}
	waited, desktop, _ := flashingRestore(t, context.Background(), events, newFakeClock())

	if waited != testSeries {
		t.Fatalf("the wait took %s, wanted one series of %s", waited, testSeries)
	}
	if len(desktop.rebuiltButtons()) != 1 {
		t.Fatal("the button was not rebuilt after the wait")
	}
}

// The user's first key press or click ends the wait and the buttons are
// rebuilt at once (FR-079).
func TestAKeyPressEndsTheWaitForTheFlashing(t *testing.T) {
	t.Parallel()
	events := &fakeEvents{series: testSeries, touchAt: 2}
	waited, desktop, _ := flashingRestore(t, context.Background(), events, newFakeClock())

	if waited != fakeStep {
		t.Fatalf("the wait took %s after the key press, wanted %s", waited, fakeStep)
	}
	if len(desktop.rebuiltButtons()) != 1 {
		t.Fatal("the button was not rebuilt at the key press")
	}
}

// The ceiling ends the wait however long the series.
func TestTheCeilingEndsTheWaitForTheFlashing(t *testing.T) {
	t.Parallel()
	events := &fakeEvents{series: time.Hour}
	waited, desktop, _ := flashingRestore(t, context.Background(), events, newFakeClock())

	if waited != time.Minute {
		t.Fatalf("the wait took %s, wanted the ceiling of %s", waited, time.Minute)
	}
	if len(desktop.rebuiltButtons()) != 1 {
		t.Fatal("the button was not rebuilt at the ceiling")
	}
}

// A restore stopped during the wait rebuilds nothing: the user or a newer
// restore has the desktop now.
func TestARestoreStoppedDuringTheWaitRebuildsNothing(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clock := newFakeClock()
	clock.onSleep = func(context.Context, *fakeClock, int) { cancel() }
	_, desktop, _ := flashingRestore(t, ctx, &fakeEvents{series: testSeries}, clock)

	if built := desktop.rebuiltButtons(); len(built) != 0 {
		t.Fatalf("a stopped restore rebuilt %v", built)
	}
}
