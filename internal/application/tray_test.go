package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// trayOver returns a tray service over the given store and desktop, with the
// restore and capture services it sits on top of.
func trayOver(store *fakeStore, desktop *fakeDesktop, processes *fakeProcesses) (*TrayService, *RestoreService) {
	log := &fakeLog{}
	clock := newFakeClock()
	launcher := &fakeLauncher{}
	restores := restoreUnder(desktop, processes, launcher, store, clock, log)
	captures := NewCaptureService(desktop, store, log, screenst)
	return NewTrayService(store, restores, captures, log), restores
}

// kinds returns the kinds of a menu, which is the shape of it.
func kinds(items []MenuItem) []MenuKind {
	found := make([]MenuKind, 0, len(items))
	for _, item := range items {
		found = append(found, item.Kind)
	}
	return found
}

// itemOf returns the first entry of a kind, plus whether there was one.
func itemOf(items []MenuItem, kind MenuKind) (MenuItem, bool) {
	for _, item := range items {
		if item.Kind == kind {
			return item, true
		}
	}
	return MenuItem{}, false
}

// A user who has captured nothing yet still gets a menu that can do something:
// the capture that gets them started, plus a line saying where their profiles
// would be.
func TestAnEmptyStoreStillGivesAUsefulMenu(t *testing.T) {
	t.Parallel()
	tray, _ := trayOver(newFakeStore(), &fakeDesktop{displays: []Display{primaryDisplay}}, newFakeProcesses())

	items := tray.Menu(context.Background())
	message, held := itemOf(items, MenuMessage)
	if !held || message.Label != noProfiles {
		t.Fatalf("the menu reads %+v", items)
	}
	if message.Enabled {
		t.Fatal("a line that does nothing was offered as something to click")
	}
	if manager, held := itemOf(items, MenuManager); !held || !manager.Enabled {
		t.Error("the menu does not offer the manager (EIR-001)")
	}
	capture, held := itemOf(items, MenuCapture)
	if !held || !capture.Enabled {
		t.Fatal("a user with no profiles cannot capture one")
	}
	if quit, held := itemOf(items, MenuQuit); !held || !quit.Enabled {
		t.Fatal("the agent cannot be quit")
	}
	if _, held := itemOf(items, MenuProfile); held {
		t.Fatal("an empty store offered a profile")
	}
}

// FR-041 and FR-040: every profile is offered, in a settled order, with the
// default one marked.
func TestEveryProfileIsOfferedWithTheDefaultMarked(t *testing.T) {
	t.Parallel()
	sofa, _ := domain.NewProfile("sofa", domain.Entry{Application: pigeonpost, Running: true})
	desk, _ := domain.NewProfile("Desk", domain.Entry{Application: pigeonpost, Running: true})
	store := newFakeStore(sofa, desk.WithDefault(true))
	tray, _ := trayOver(store, &fakeDesktop{displays: []Display{primaryDisplay}}, newFakeProcesses())

	items := tray.Menu(context.Background())
	var profiles []MenuItem
	for _, item := range items {
		if item.Kind == MenuProfile {
			profiles = append(profiles, item)
		}
	}
	if len(profiles) != 2 {
		t.Fatalf("the menu offers %d profiles", len(profiles))
	}
	if profiles[0].Profile != "Desk" || profiles[1].Profile != "sofa" {
		t.Fatalf("offered in the order %q then %q", profiles[0].Profile, profiles[1].Profile)
	}
	if !profiles[0].Checked || profiles[1].Checked {
		t.Fatalf("the default is marked on %+v", profiles)
	}
	if !strings.Contains(profiles[0].Label, "default") {
		t.Fatalf("the default reads %q", profiles[0].Label)
	}
	// The name to act on is kept apart from the words shown, so the label can
	// say more without the profile becoming unfindable.
	if profiles[0].Profile != "Desk" {
		t.Fatalf("the marked entry names the profile %q", profiles[0].Profile)
	}
}

// A store that cannot be read and a store with nothing in it look identical to
// a user and mean opposite things, so the menu says which it is.
func TestAStoreThatCannotBeReadSaysSoInTheMenu(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.namesErr = errors.New("the disk is not there")
	tray, _ := trayOver(store, &fakeDesktop{displays: []Display{primaryDisplay}}, newFakeProcesses())

	items := tray.Menu(context.Background())
	message, held := itemOf(items, MenuMessage)
	if !held {
		t.Fatalf("the menu reads %+v", items)
	}
	if message.Label == noProfiles {
		t.Fatal("a store that could not be read was reported as a store with no profiles")
	}
	if !strings.Contains(message.Label, "could not be read") {
		t.Fatalf("the message reads %q", message.Label)
	}
}

// FR-045: the tray says whether the last restore left anything outstanding,
// before the user clicks anything.
func TestTheTraySaysWhetherTheLastRestoreWorked(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	tray, restores := trayOver(newFakeStore(), desktop, newFakeProcesses())

	// Before any restore, there is nothing to report and nothing to look at.
	if tray.NeedsAttention() {
		t.Fatal("the tray asked for attention before anything had run")
	}
	report, held := itemOf(tray.Menu(context.Background()), MenuReport)
	if !held || report.Enabled || report.Label != noReport {
		t.Fatalf("the report entry reads %+v", report)
	}
	if tray.Tooltip() != "ScreenState" {
		t.Fatalf("the tooltip reads %q", tray.Tooltip())
	}

	// A restore that leaves something outstanding is said so in three places.
	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: stellody, Running: true,
			Placements: []domain.Placement{onPrimary}})
	if _, err := restores.Restore(context.Background(), profile); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if !tray.NeedsAttention() {
		t.Fatal("the tray did not ask for attention after an outstanding entry")
	}
	report, _ = itemOf(tray.Menu(context.Background()), MenuReport)
	if !report.Enabled || !strings.Contains(report.Label, "1 outstanding") {
		t.Fatalf("the report entry reads %q", report.Label)
	}
	if !strings.Contains(tray.Tooltip(), "1 outstanding") {
		t.Fatalf("the tooltip reads %q", tray.Tooltip())
	}
}

// A restore that satisfied everything leaves a report worth reading without
// asking for attention.
func TestARestoreThatWorkedDoesNotAskForAttention(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	tray, restores := trayOver(newFakeStore(), desktop, newFakeProcesses(nordvpn))

	profile, _ := domain.NewProfile("Tray", domain.Entry{Application: nordvpn, Running: true})
	if _, err := restores.Restore(context.Background(), profile); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if tray.NeedsAttention() {
		t.Fatal("a restore that satisfied everything asked for attention")
	}
	report, _ := itemOf(tray.Menu(context.Background()), MenuReport)
	if !report.Enabled || report.Label != reportLabel {
		t.Fatalf("the report entry reads %+v", report)
	}
	if held, _ := tray.Report(); held == nil {
		t.Fatal("the report cannot be opened")
	}
}

// FR-041: choosing a profile restores it.
func TestChoosingAProfileRestoresIt(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	profile, _ := domain.NewProfile("Desk", domain.Entry{
		Application: pigeonpost, Running: true, Placements: []domain.Placement{onPrimary},
	})
	tray, _ := trayOver(newFakeStore(profile), desktop, newFakeProcesses(pigeonpost))

	report, err := tray.Apply(context.Background(), "Desk")
	if err != nil {
		t.Fatalf("applying: %v", err)
	}
	if report.Profile != "Desk" {
		t.Fatalf("restored %q", report.Profile)
	}
	if _, outstanding := report.Counts(); outstanding != 0 {
		t.Fatalf("outstanding: %s", report.Summary())
	}
	if len(desktop.placements()) != 1 {
		t.Fatalf("placed %d windows", len(desktop.placements()))
	}
}

// A profile that has gone since the menu was drawn is reported rather than
// restored as nothing.
func TestChoosingAProfileThatHasGoneSaysSo(t *testing.T) {
	t.Parallel()
	tray, _ := trayOver(newFakeStore(), &fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses())

	report, err := tray.Apply(context.Background(), "Desk")
	if !errors.Is(err, ErrNoSuchProfile) {
		t.Fatalf("answered %v", err)
	}
	if report != nil {
		t.Fatal("a profile that is not there produced a report")
	}
}

// Capturing from the tray reads the desktop and writes nothing.
func TestCapturingFromTheTrayWritesNothing(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	tray, _ := trayOver(store, desktop, newFakeProcesses(pigeonpost))

	review, err := tray.Capture(context.Background())
	if err != nil {
		t.Fatalf("capturing: %v", err)
	}
	if len(review.Entries) != 1 || !review.Entries[0].Application.Equal(pigeonpost) {
		t.Fatalf("captured %+v", review.Entries)
	}
	if names, _ := store.Names(context.Background()); len(names) != 0 {
		t.Fatalf("a capture wrote %v", names)
	}
}

// The menu's shape is settled: profiles, then the things that are always
// there, each group divided. EIR-001 names what the menu offers; the manager
// is on that list: a menu that cannot reach the window is a window a
// user has to find another way into.
func TestTheMenuHasASettledShape(t *testing.T) {
	t.Parallel()
	profile, _ := domain.NewProfile("Desk", domain.Entry{Application: pigeonpost, Running: true})
	tray, _ := trayOver(newFakeStore(profile), &fakeDesktop{displays: []Display{primaryDisplay}},
		newFakeProcesses())

	want := []MenuKind{
		MenuProfile, MenuSeparator,
		MenuManager, MenuCapture, MenuReport,
		MenuSeparator, MenuQuit,
	}
	got := kinds(tray.Menu(context.Background()))
	if len(got) != len(want) {
		t.Fatalf("the menu has %d entries: %v", len(got), got)
	}
	for at, kind := range want {
		if got[at] != kind {
			t.Fatalf("entry %d is kind %d, wanted %d", at, got[at], kind)
		}
	}
}
