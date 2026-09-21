package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// FR-075: at sign-in every window the restore placed has its taskbar button
// built afresh, since every application was starting around then and a taskbar
// marks the window last activated on its display. The recorded desktop carried
// no such marks, so a restore that leaves them has not put the desktop back.
func TestASignInRestoreBuildsEveryButtonAfresh(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	log := &fakeLog{}
	profile := oneEntry(pigeonpost, onPrimary).WithDefault(true)
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost),
		&fakeLauncher{}, newFakeStore(profile), newFakeClock(), log)

	if _, marked, err := service.RestoreDefault(context.Background()); err != nil || !marked {
		t.Fatalf("restoring at sign-in: %v, marked %v", err, marked)
	}
	built := desktop.rebuiltButtons()
	if len(built) != 1 || built[0] != WindowID(1) {
		t.Fatalf("the buttons built afresh were %v", built)
	}
	if !log.saying("built afresh") {
		t.Error("the log does not record it")
	}
}

// An Apply leaves alone the windows of applications that were already running:
// placing no longer activates anything (FR-074), so nothing gained a mark and
// flickering them would cost the user for nothing.
func TestAnApplyLeavesTheButtonsOfRunningApplicationsAlone(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost),
		&fakeLauncher{}, newFakeStore(), newFakeClock(), log)

	if _, err := service.Restore(context.Background(), oneEntry(pigeonpost, onPrimary)); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if built := desktop.rebuiltButtons(); len(built) != 0 {
		t.Fatalf("an Apply flickered %v", built)
	}
	if log.saying("built afresh") {
		t.Error("the log claims buttons were built afresh")
	}
}

// An Apply that starts an application does build that application's button
// afresh: an application puts its own window in front as it starts, which is
// what leaves the mark.
func TestAnApplyBuildsTheButtonOfAnApplicationItStarted(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	processes := newFakeProcesses()
	launcher := &fakeLauncher{}
	launcher.onLaunch = func(domain.ApplicationIdentity) {
		processes.start(stellody)
		desktop.addWindow(aWindow(2, stellody, at(1)))
	}
	service := restoreUnder(desktop, processes, launcher,
		newFakeStore(), newFakeClock(), &fakeLog{})

	if _, err := service.Restore(context.Background(), oneEntry(stellody, onPrimary)); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	built := desktop.rebuiltButtons()
	if len(built) != 1 || built[0] != WindowID(2) {
		t.Fatalf("the buttons built afresh were %v", built)
	}
}

// FR-075: one button that cannot be rebuilt costs that button alone. The rest
// are still built afresh and the log still says how many were.
func TestOneRefusedButtonDoesNotStopTheRest(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays:       []Display{primaryDisplay},
		windows:        []Window{aWindow(1, pigeonpost, at(0)), aWindow(2, stellody, at(1))},
		rebuildRefuses: map[WindowID]error{1: errors.New("the window is not answering")},
	}
	log := &fakeLog{}
	profile, err := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true, Placements: []domain.Placement{onPrimary}},
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{onPrimary}},
	)
	if err != nil {
		t.Fatalf("the profile is not valid: %v", err)
	}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, stellody),
		&fakeLauncher{}, newFakeStore(profile.WithDefault(true)), newFakeClock(), log)

	if _, _, err := service.RestoreDefault(context.Background()); err != nil {
		t.Fatalf("restoring at sign-in: %v", err)
	}
	if built := desktop.rebuiltButtons(); len(built) != 1 || built[0] != WindowID(2) {
		t.Fatalf("rebuilt %v, wanted the second window's button despite the first refusing", built)
	}
	if !log.saying("was not rebuilt") {
		t.Error("the log does not say which button was not rebuilt")
	}
	if !log.saying("1 taskbar button(s) were built afresh, 1 could not be") {
		t.Error("the log does not count the buttons rebuilt")
	}
}

// A restore stopped while its buttons are being rebuilt rebuilds no more of
// them: carrying on past a refusal must not mean carrying on past a stop.
func TestAStoppedRestoreRebuildsNoMoreButtons(t *testing.T) {
	t.Parallel()
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	desktop := &fakeDesktop{
		displays:  []Display{primaryDisplay},
		windows:   []Window{aWindow(1, pigeonpost, at(0)), aWindow(2, stellody, at(1))},
		onRebuild: stop,
	}
	profile, err := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true, Placements: []domain.Placement{onPrimary}},
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{onPrimary}},
	)
	if err != nil {
		t.Fatalf("the profile is not valid: %v", err)
	}
	log := &fakeLog{}
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost, stellody),
		&fakeLauncher{}, newFakeStore(profile.WithDefault(true)), newFakeClock(), log)

	_, _, _ = service.RestoreDefault(ctx)
	if built := desktop.rebuiltButtons(); len(built) != 1 {
		t.Fatalf("rebuilt %v after the restore was stopped, wanted the first alone", built)
	}
	if log.saying("built afresh") {
		t.Error("a stopped restore still counted its buttons as though it had finished")
	}
}

// A button that cannot be rebuilt changes nothing about the desktop, so it is
// noted in the log and nowhere else: the user has their windows back.
func TestAButtonThatCannotBeRebuiltIsNotedAndNothingMore(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays:   []Display{primaryDisplay},
		windows:    []Window{aWindow(1, pigeonpost, at(0))},
		rebuildErr: errors.New("the window is not answering"),
	}
	log := &fakeLog{}
	profile := oneEntry(pigeonpost, onPrimary).WithDefault(true)
	service := restoreUnder(desktop, newFakeProcesses(pigeonpost),
		&fakeLauncher{}, newFakeStore(profile), newFakeClock(), log)

	report, _, err := service.RestoreDefault(context.Background())
	if err != nil {
		t.Fatalf("restoring at sign-in: %v", err)
	}
	if !log.saying("was not rebuilt") {
		t.Error("the log does not say the button was not rebuilt")
	}
	if anyContaining(report.SortedNotes(), "button") {
		t.Errorf("the report troubles the user with it: %v", report.SortedNotes())
	}
}
