package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeCeilings is the ceiling setting as a test states it. Its zero value is a
// user who has chosen none.
type fakeCeilings struct {
	mutex    sync.Mutex
	chosen   time.Duration
	set      bool
	readErr  error
	writeErr error
}

func (ceilings *fakeCeilings) Ceiling() (time.Duration, bool, error) {
	ceilings.mutex.Lock()
	defer ceilings.mutex.Unlock()
	if ceilings.readErr != nil {
		return 0, false, ceilings.readErr
	}
	return ceilings.chosen, ceilings.set, nil
}

func (ceilings *fakeCeilings) SetCeiling(ceiling time.Duration) error {
	ceilings.mutex.Lock()
	defer ceilings.mutex.Unlock()
	if ceilings.writeErr != nil {
		return ceilings.writeErr
	}
	ceilings.chosen, ceilings.set = ceiling, true
	return nil
}

// restoreWithCeiling is a restore over a desktop showing nothing the profile
// names, under the default policy and the given ceiling setting, so the restore
// waits for its ceiling and the fake clock says how long that was.
func restoreWithCeiling(t *testing.T, ceilings *fakeCeilings) (*RestoreService, *fakeClock) {
	t.Helper()
	clock := newFakeClock()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	service := NewRestoreService(desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), clock, &fakeLog{}, DefaultPolicy(), ceilings, screenst,
		&fakeStrangers{}, &fakeSplash{}, &fakeEvents{clock: clock})
	return service, clock
}

// waitedFor runs a restore that can never be satisfied and answers how long it
// waited before giving up, with its report.
func waitedFor(t *testing.T, service *RestoreService, clock *fakeClock) (time.Duration, *Report) {
	t.Helper()
	started := clock.Now()
	report, err := service.Restore(context.Background(), deskProfile(t))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	return clock.Now().Sub(started), report
}

// NFR-PERF-003: a user who has chosen nothing gets the fifteen minutes the
// specification sets.
func TestARestoreWaitsTheDefaultCeilingWhereNoneIsChosen(t *testing.T) {
	t.Parallel()
	service, clock := restoreWithCeiling(t, &fakeCeilings{})
	if waited, _ := waitedFor(t, service, clock); waited != DefaultCeiling {
		t.Fatalf("the restore waited %s, wanted the default %s", waited, DefaultCeiling)
	}
}

// NFR-PERF-003: the ceiling the user sets is the one the next restore runs to,
// read as it begins rather than when the service was built.
func TestARestoreWaitsTheCeilingTheUserChose(t *testing.T) {
	t.Parallel()
	ceilings := &fakeCeilings{}
	service, clock := restoreWithCeiling(t, ceilings)
	chosen := MinimumCeiling * 5
	if _, err := service.SetCeiling(chosen); err != nil {
		t.Fatalf("setting the ceiling: %v", err)
	}
	waited, report := waitedFor(t, service, clock)
	if waited != chosen {
		t.Fatalf("the restore waited %s, wanted the chosen %s", waited, chosen)
	}
	entry := reportOf(t, report, pigeonpost)
	if !containsText(entry.Reason, chosen.String()) {
		t.Fatalf("the report does not name the ceiling that passed: %q", entry.Reason)
	}
}

// A ceiling outside the bounds is held within them rather than refused, both
// when it is set and when a file edited by hand says otherwise.
func TestAChosenCeilingIsHeldWithinItsBounds(t *testing.T) {
	t.Parallel()
	service, _ := restoreWithCeiling(t, &fakeCeilings{})
	if kept, err := service.SetCeiling(MaximumCeiling * 2); err != nil || kept != MaximumCeiling {
		t.Fatalf("a ceiling above the maximum was kept as %s (%v)", kept, err)
	}
	edited, _ := restoreWithCeiling(t, &fakeCeilings{chosen: MinimumCeiling / 2, set: true})
	if ceiling, err := edited.Ceiling(); err != nil || ceiling != MinimumCeiling {
		t.Fatalf("a ceiling below the minimum read as %s (%v)", ceiling, err)
	}
}

// A setting that cannot be read is no reason not to restore: the default is
// used and the report says so.
func TestAnUnreadableCeilingFallsBackToTheDefaultAndSaysSo(t *testing.T) {
	t.Parallel()
	service, clock := restoreWithCeiling(t, &fakeCeilings{readErr: errors.New("the file is unreadable")})
	if _, err := service.Ceiling(); err == nil {
		t.Fatal("the manager was not told the setting could not be read")
	}
	waited, report := waitedFor(t, service, clock)
	if waited != DefaultCeiling {
		t.Fatalf("the restore waited %s, wanted the default %s", waited, DefaultCeiling)
	}
	if !anyContaining(report.SortedNotes(), "ceiling setting could not be read") {
		t.Fatalf("the report does not say the default was used: %v", report.SortedNotes())
	}
}

// A ceiling that cannot be written is said to the caller, never taken as kept.
func TestACeilingThatCannotBeWrittenIsSaid(t *testing.T) {
	t.Parallel()
	refused := errors.New("the disk is full")
	service, _ := restoreWithCeiling(t, &fakeCeilings{writeErr: refused})
	if _, err := service.SetCeiling(MinimumCeiling); !errors.Is(err, refused) {
		t.Fatalf("the refusal was not passed on: %v", err)
	}
}
