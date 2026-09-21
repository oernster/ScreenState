package main

import (
	"context"
	"testing"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
	"github.com/oernster/ScreenState/internal/infrastructure/store"
)

// FR-005 across the wire: an application running in the background reaches the
// profile only where the user ticked it, then as running with no placement.
// The store is the real one, over a directory of the test's own.
func TestOnlyTheTickedBackgroundApplicationsAreSaved(t *testing.T) {
	t.Parallel()
	log := &recordingLog{}
	profiles, err := store.New(t.TempDir(), log)
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}
	self := domain.ApplicationIdentity{Value: `C:\Programs\ScreenState\ScreenState.exe`}
	captures := application.NewCaptureService(nil, nil, profiles, log, self)
	app := NewApp(nil, nil, nil, captures, nil, log, "0.0.0-test", false)

	pigeonpost := domain.ApplicationIdentity{Value: `C:\Programs\PigeonPost\PigeonPost.exe`}
	nordvpn := domain.ApplicationIdentity{Value: `C:\Program Files\NordVPN\NordVPN.exe`}
	gameglass := domain.ApplicationIdentity{Value: `C:\Program Files\GameGlass Hub\GameGlass Hub.exe`}
	app.review = application.Review{
		Entries: []domain.Entry{{Application: pigeonpost, Running: true}},
		Background: []domain.Entry{
			{Application: gameglass, Running: true},
			{Application: nordvpn, Running: true},
		},
	}

	if err := app.SaveCapture("Desk", []string{pigeonpost.Value, nordvpn.Value}, false); err != nil {
		t.Fatalf("saving: %v", err)
	}
	saved, err := profiles.Load(context.Background(), "Desk")
	if err != nil {
		t.Fatalf("reading the profile back: %v", err)
	}
	if len(saved.Entries) != 2 {
		t.Fatalf("saved %d entries, wanted PigeonPost and NordVPN: %+v", len(saved.Entries), saved.Entries)
	}
	kept, found := saved.Find(nordvpn)
	if !found || !kept.Running || len(kept.Placements) != 0 {
		t.Errorf("NordVPN was saved as %+v (found %v)", kept, found)
	}
	if _, found := saved.Find(gameglass); found {
		t.Error("GameGlass was saved although it was never ticked")
	}
}
