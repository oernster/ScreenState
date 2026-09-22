package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// Amendment 1: a restore puts back the stacking order a profile records.
// PigeonPost stands in for Claude and Notepad for Windows Terminal, the pair
// the owner reported: Terminal captured behind Claude.

// rankedProfile returns a profile holding one entry per application, each with
// one placement on the primary display holding the rank given beside it.
func rankedProfile(t *testing.T, ranks map[domain.ApplicationIdentity]int,
	order ...domain.ApplicationIdentity) domain.Profile {
	t.Helper()
	entries := make([]domain.Entry, 0, len(order))
	for _, application := range order {
		entries = append(entries, domain.Entry{Application: application, Running: true,
			Placements: []domain.Placement{onPrimary.WithRank(ranks[application])}})
	}
	profile, err := domain.NewProfile("Desk", entries...)
	if err != nil {
		t.Fatalf("building the profile: %v", err)
	}
	return profile
}

// terminalOverClaude is a desktop with both windows open and the one that
// belongs behind drawn on top, which is how Apply found them on 2026-09-22.
func terminalOverClaude() *fakeDesktop {
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0)), aWindow(2, notepad, at(1))},
	}
	desktop.stacking.order = []WindowID{2, 1}
	return desktop
}

var claudeOverTerminal = map[domain.ApplicationIdentity]int{pigeonpost: 1, notepad: 2}

// FR-083: Apply restores the recorded order, rank 1 on top (OQ-15).
func TestAnApplyPutsBackTheRecordedStackingOrder(t *testing.T) {
	t.Parallel()
	desktop := terminalOverClaude()
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, notepad),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), log)

	profile := rankedProfile(t, claudeOverTerminal, notepad, pigeonpost)
	if _, err := service.Restore(context.Background(), profile); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if !desktop.drawnOver(1, 2) {
		t.Fatal("Terminal is still drawn over Claude")
	}
	if !log.saying("restacked 2 window(s)") {
		t.Error("the log does not say how many windows were restacked nor how long it took")
	}
}

// FR-083: at sign-in each application puts its own window in front as it
// starts, so the order is set once, after the taskbar buttons are rebuilt.
func TestASignInRestacksAfterTheButtonsAreRebuilt(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay},
		windows: []Window{aWindow(1, pigeonpost, at(0))}}
	processes := newFakeProcesses(pigeonpost)
	launcher := &fakeLauncher{}
	launcher.onLaunch = func(domain.ApplicationIdentity) {
		processes.start(notepad)
		desktop.addWindow(aWindow(2, notepad, at(1)))
		desktop.raise(2)
	}
	restackedBeforeARebuild := false
	desktop.onRebuild = func() { restackedBeforeARebuild = restackedBeforeARebuild || len(desktop.restacks()) > 0 }
	profile := rankedProfile(t, claudeOverTerminal, pigeonpost, notepad).WithDefault(true)
	service := restoreUnder(desktop, processes, launcher, newFakeStore(profile), newFakeClock(), &fakeLog{})

	if _, marked, err := service.RestoreDefault(context.Background()); err != nil || !marked {
		t.Fatalf("restoring at sign-in: %v, marked %v", err, marked)
	}
	if len(desktop.rebuiltButtons()) == 0 || restackedBeforeARebuild {
		t.Fatalf("the restack did not come after the rebuild: rebuilt %v", desktop.rebuiltButtons())
	}
	if !desktop.drawnOver(1, 2) {
		t.Fatal("Terminal, started last, is still drawn over Claude")
	}
}

// FR-085 and FR-086: a window that cannot be restacked is named; the rest are
// still stacked in their order relative to each other.
func TestAWindowThatCannotBeRestackedIsNamedAndTheRestKeepTheirOrder(t *testing.T) {
	t.Parallel()
	desktop := terminalOverClaude()
	desktop.windows = append(desktop.windows, aWindow(3, stellody, at(2)))
	desktop.stacking.order = []WindowID{2, 3, 1}
	desktop.stacking.refuses = map[WindowID]error{3: errors.New("access is denied")}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, notepad, stellody),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{})

	ranks := map[domain.ApplicationIdentity]int{pigeonpost: 1, stellody: 2, notepad: 3}
	report, err := service.Restore(context.Background(), rankedProfile(t, ranks, pigeonpost, stellody, notepad))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if !noteSaying(reportOf(t, report, stellody), "could not be put back in its place in the stacking order") {
		t.Fatalf("the report does not name Stellody: %+v", reportOf(t, report, stellody))
	}
	if !desktop.drawnOver(1, 2) {
		t.Fatal("Claude is not drawn over Terminal once Stellody refused")
	}
}

// FR-086: a ranked application that never showed a window leaves the others in
// their recorded order.
func TestAMissingWindowLeavesTheOthersInOrder(t *testing.T) {
	t.Parallel()
	desktop := terminalOverClaude()
	launcher := &fakeLauncher{refuse: map[string]error{
		strings.ToLower(stellody.String()): errors.New("the file is not there"),
	}}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, notepad),
		launcher, newFakeStore(), newFakeClock(), &fakeLog{})

	ranks := map[domain.ApplicationIdentity]int{pigeonpost: 1, stellody: 2, notepad: 3}
	if _, err := service.Restore(context.Background(), rankedProfile(t, ranks, pigeonpost, stellody, notepad)); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if !desktop.drawnOver(1, 2) {
		t.Fatal("Claude is not drawn over Terminal with Stellody missing")
	}
}

// FR-087: once the user has pressed a key or clicked, the order is left alone.
func TestTheOrderIsLeftAloneOnceTheUserHasTakenOver(t *testing.T) {
	t.Parallel()
	desktop := terminalOverClaude()
	clock := newFakeClock()
	events := &fakeEvents{clock: clock, takenOver: true}
	service := NewRestoreService(desktop, newFakeProcesses(pigeonpost, notepad), &fakeLauncher{},
		newFakeStore(), clock, &fakeLog{}, DefaultPolicy(), &fakeCeilings{}, screenst,
		&fakeStrangers{}, &fakeSplash{}, events)

	report, err := service.Restore(context.Background(), rankedProfile(t, claudeOverTerminal, pigeonpost, notepad))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if calls := desktop.restacks(); len(calls) != 0 {
		t.Fatalf("restacked %v after the user took over", calls)
	}
	if !anyContaining(report.Notes, "left as the user found it") {
		t.Fatalf("the report does not say so: %v", report.Notes)
	}
}

// FR-083 where the desktop cannot be watched: no key press or click can be
// heard, so nothing says the user took over and the order is still restored.
func TestTheOrderIsRestoredWhereTheDesktopCannotBeWatched(t *testing.T) {
	t.Parallel()
	desktop := terminalOverClaude()
	clock := newFakeClock()
	events := &fakeEvents{clock: clock, watchErr: errors.New("no hooks")}
	service := NewRestoreService(desktop, newFakeProcesses(pigeonpost, notepad), &fakeLauncher{},
		newFakeStore(), clock, &fakeLog{}, DefaultPolicy(), &fakeCeilings{}, screenst,
		&fakeStrangers{}, &fakeSplash{}, events)

	if _, err := service.Restore(context.Background(), rankedProfile(t, claudeOverTerminal, pigeonpost, notepad)); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if !desktop.drawnOver(1, 2) {
		t.Fatal("Terminal is still drawn over Claude")
	}
}

// FR-088: a profile saved before the order was recorded keeps the order found.
func TestAProfileWithNoRanksKeepsTheOrderFound(t *testing.T) {
	t.Parallel()
	desktop := terminalOverClaude()
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, notepad),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), log)

	unranked := map[domain.ApplicationIdentity]int{}
	if _, err := service.Restore(context.Background(), rankedProfile(t, unranked, pigeonpost, notepad)); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if calls := desktop.restacks(); len(calls) != 0 || !desktop.drawnOver(2, 1) {
		t.Fatalf("the order found was not kept: restacks %v", calls)
	}
	if !log.saying("records no stacking order") {
		t.Error("the log does not say why nothing was restacked")
	}
}

// DATA-007: ranks that cannot be used are ignored, with the reason reported.
func TestRanksThatCannotBeUsedAreIgnoredAndReported(t *testing.T) {
	t.Parallel()
	desktop := terminalOverClaude()
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, notepad),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), &fakeLog{})

	shared := map[domain.ApplicationIdentity]int{pigeonpost: 1, notepad: 1}
	report, err := service.Restore(context.Background(), rankedProfile(t, shared, pigeonpost, notepad))
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if calls := desktop.restacks(); len(calls) != 0 {
		t.Fatalf("restacked %v from ranks that cannot be used", calls)
	}
	if !anyContaining(report.Notes, "two placements share rank 1") {
		t.Fatalf("the report does not say why: %v", report.Notes)
	}
}
