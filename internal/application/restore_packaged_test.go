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

// S-4: two programs of one package share its model id. Each window goes to the
// entry naming its own program exactly; no window is placed for two entries in
// one restore.
func TestTwoProgramsOfOnePackageEachKeepTheirOwnWindow(t *testing.T) {
	t.Parallel()
	mainApp := domain.ApplicationIdentity{Value: `C:\Package\main.exe`}.WithModelID(claudeModelID)
	helper := domain.ApplicationIdentity{Value: `C:\Package\helper.exe`}.WithModelID(claudeModelID)
	mainRect := domain.Rect{X: 0, Y: 0, Width: 1000, Height: 700}
	helperRect := domain.Rect{X: 1100, Y: 0, Width: 900, Height: 600}
	for _, order := range [][2]domain.ApplicationIdentity{{mainApp, helper}, {helper, mainApp}} {
		rects := map[string]domain.Rect{mainApp.Value: mainRect, helper.Value: helperRect}
		var entries []domain.Entry
		for _, application := range order {
			entries = append(entries, domain.Entry{Application: application, Running: true,
				Placements: []domain.Placement{{Display: primaryID, Rect: rects[application.Value], State: domain.ShowNormal}}})
		}
		profile, err := domain.NewProfile("Desk", entries...)
		if err != nil {
			t.Fatalf("the profile is not valid: %v", err)
		}
		desktop := &fakeDesktop{displays: []Display{primaryDisplay},
			windows: []Window{aWindow(1, helper, at(0)), aWindow(2, mainApp, at(1))}}
		service := restoreUnder(desktop, newFakeProcesses(mainApp, helper), &fakeLauncher{},
			newFakeStore(), newFakeClock(), &fakeLog{})
		if _, err := service.Restore(context.Background(), profile); err != nil {
			t.Fatalf("restoring: %v", err)
		}
		placedTo := map[WindowID]domain.Rect{}
		for _, call := range desktop.placements() {
			if earlier, twice := placedTo[call.id]; twice && earlier != call.rect {
				t.Fatalf("window %d was placed for two entries: %+v then %+v", call.id, earlier, call.rect)
			}
			placedTo[call.id] = call.rect
		}
		if placedTo[1] != helperRect || placedTo[2] != mainRect {
			t.Fatalf("entries in the order %v placed %+v", order, placedTo)
		}
	}
}

// S-4 again: two entries recognising one window by the model id alone share
// nothing. The window goes to one of them; the other is not reported as
// satisfied by a window it never had.
func TestOneWindowIsNeverClaimedByTwoEntries(t *testing.T) {
	t.Parallel()
	first := domain.ApplicationIdentity{Value: `C:\Package\first.exe`}.WithModelID(claudeModelID)
	second := domain.ApplicationIdentity{Value: `C:\Package\second.exe`}.WithModelID(claudeModelID)
	profile, err := domain.NewProfile("Desk",
		domain.Entry{Application: first, Running: true, Placements: []domain.Placement{
			aPlacement(primaryID, domain.Rect{X: 0, Y: 0, Width: 1000, Height: 700})}},
		domain.Entry{Application: second, Running: true, Placements: []domain.Placement{
			aPlacement(primaryID, domain.Rect{X: 1100, Y: 0, Width: 900, Height: 600})}})
	if err != nil {
		t.Fatalf("the profile is not valid: %v", err)
	}
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}, windows: []Window{aWindow(1, claudeUpdated, at(0))}}
	service := restoreUnder(desktop, newFakeProcesses(first, second), &fakeLauncher{},
		newFakeStore(), newFakeClock(), &fakeLog{})
	report, err := service.Restore(context.Background(), profile)
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if satisfied, _ := report.Counts(); satisfied != 1 {
		t.Fatalf("%d entries satisfied by one window: %s", satisfied, report.Summary())
	}
}
