package application

import (
	"context"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// A Store-packaged application installs under a directory carrying its version,
// so an update moves its path while its model id stays put (FR-071). The two
// installs of Claude below are the shape measured on 2026-09-21.
const claudeModelID = "Claude_pzs8sxrjxfjjc!Claude"

var (
	claudeCaptured = domain.ApplicationIdentity{
		Value: `C:\Program Files\WindowsApps\Claude_1.0.0.0_x64__pzs8sxrjxfjjc\app\Claude.exe`,
	}.WithModelID(claudeModelID)
	claudeUpdated = domain.ApplicationIdentity{
		Value: `C:\Program Files\WindowsApps\Claude_1.1.0.0_x64__pzs8sxrjxfjjc\app\Claude.exe`,
	}.WithModelID(claudeModelID)
)

// claudeProfile names Claude as captured before the update, placed on the
// primary display.
func claudeProfile(t *testing.T, claude domain.ApplicationIdentity) domain.Profile {
	t.Helper()
	profile, err := domain.NewProfile("Desk",
		domain.Entry{Application: claude, Running: true, Placements: []domain.Placement{
			aPlacement(primaryID, domain.Rect{X: 0, Y: 0, Width: 3440, Height: 1392})}},
	)
	if err != nil {
		t.Fatalf("the profile is not valid: %v", err)
	}
	return profile
}

// restoreAfterTheUpdate restores a profile naming Claude by `named` while the
// only window on the desktop is the updated Claude's.
func restoreAfterTheUpdate(t *testing.T, named domain.ApplicationIdentity) (*fakeDesktop, *Report) {
	t.Helper()
	window := aWindow(1, claudeUpdated, at(0))
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}, windows: []Window{window}}
	// The process table already recognises the moved program by its model id;
	// that half of FR-071 lives in infrastructure and is tested there.
	service := restoreUnder(desktop, newFakeProcesses(named), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})
	report, err := service.Restore(context.Background(), claudeProfile(t, named))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	return desktop, report
}

// FR-071: once an update has moved the path, the window of the running
// application is still the entry's, so it is placed and never put away.
func TestAPackagedWindowIsPlacedAfterAnUpdateMovesItsPath(t *testing.T) {
	t.Parallel()
	desktop, report := restoreAfterTheUpdate(t, claudeCaptured)

	if satisfied, outstanding := report.Counts(); satisfied != 1 || outstanding != 0 {
		t.Fatalf("%d satisfied, %d outstanding: %s", satisfied, outstanding, report.Summary())
	}
	if placements := desktop.placements(); len(placements) != 1 {
		t.Fatalf("placed %d windows, wanted the updated Claude's", len(placements))
	}
	if put := putAwayCalls(desktop, 1); len(put) != 0 {
		t.Fatal("the updated Claude was put away as a window the profile does not name")
	}
}

// A profile saved before FR-071 names a packaged application by its model id
// alone; the window now reports its path with that model id beside it.
func TestAWindowIsMatchedToAnEntryNamedByItsModelIDAlone(t *testing.T) {
	t.Parallel()
	named, err := domain.NewApplicationIdentity(domain.KindAppUserModelID, claudeModelID)
	if err != nil {
		t.Fatalf("the model id is not valid: %v", err)
	}
	desktop, report := restoreAfterTheUpdate(t, named)

	if satisfied, outstanding := report.Counts(); satisfied != 1 || outstanding != 0 {
		t.Fatalf("%d satisfied, %d outstanding: %s", satisfied, outstanding, report.Summary())
	}
	if put := putAwayCalls(desktop, 1); len(put) != 0 {
		t.Fatal("Claude was put away as a window the profile does not name")
	}
}
