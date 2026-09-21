package application

import (
	"context"
	"testing"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// FR-038 and FR-039: sign-in restores the default profile; no default is an
// answer rather than a fault.
func TestTheDefaultProfileIsTheOneRestoredAtSignIn(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	log := &fakeLog{}
	store := newFakeStore()
	service := restoreUnder(desktop, newFakeProcesses(), &fakeLauncher{},
		store, newFakeClock(), log)

	if _, marked, err := service.RestoreDefault(context.Background(), true); err != nil || marked {
		t.Fatalf("marked=%v err=%v with no default set", marked, err)
	}
	if !log.saying("no profile is marked as the default") {
		t.Fatal("the log does not say why nothing was restored")
	}

	profile, _ := domain.NewProfile("Desk", domain.Entry{Application: nordvpn})
	if err := store.Save(context.Background(), profile.WithDefault(true)); err != nil {
		t.Fatalf("saving: %v", err)
	}
	report, marked, err := service.RestoreDefault(context.Background(), true)
	if err != nil || !marked {
		t.Fatalf("marked=%v err=%v", marked, err)
	}
	if report.Profile != "Desk" {
		t.Fatalf("restored %q", report.Profile)
	}
}

// A cancelled restore stops where it is and says so, without being an error.
func TestACancelledRestoreStopsAndSaysSo(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	clock := newFakeClock()
	ctx, cancel := context.WithCancel(context.Background())
	clock.onSleep = func(_ context.Context, _ *fakeClock, count int) {
		if count == 2 {
			cancel()
		}
	}
	defer cancel()
	service := restoreUnder(desktop, newFakeProcesses(), &fakeLauncher{},
		newFakeStore(), clock, &fakeLog{})

	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{
			aPlacement(primaryID, domain.Rect{X: 0, Y: 0, Width: 800, Height: 600})}})

	report, err := service.Restore(ctx, profile)
	if err != nil {
		t.Fatalf("a cancelled restore answered an error: %v", err)
	}
	if !anyContaining(report.Notes, "cancelled") {
		t.Fatalf("the report does not record the cancellation: %v", report.Notes)
	}
	if reportOf(t, report, stellody).Satisfied {
		t.Fatal("an entry was reported satisfied by a cancelled restore")
	}
}

func TestTheCeilingIsHeldWithinItsBounds(t *testing.T) {
	t.Parallel()
	policy := DefaultPolicy()
	if policy.WithCeiling(time.Second).Ceiling != MinimumCeiling {
		t.Fatal("a ceiling below the minimum was not raised to it")
	}
	if policy.WithCeiling(24*time.Hour).Ceiling != MaximumCeiling {
		t.Fatal("a ceiling above the maximum was not lowered to it")
	}
	if chosen := policy.WithCeiling(5 * time.Minute); chosen.Ceiling != 5*time.Minute {
		t.Fatalf("a ceiling within the bounds became %s", chosen.Ceiling)
	}
	if DefaultPolicy().Ceiling != DefaultCeiling {
		t.Fatal("the default policy does not carry the specification's ceiling")
	}
}
