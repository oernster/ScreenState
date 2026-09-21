package main

import (
	"context"
	"testing"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
	"github.com/oernster/ScreenState/internal/infrastructure/store"
)

// FR-011 across the wire: an entry the user unticked in the review never reaches
// the profile. The store is the real one, over a directory of the test's own.
func TestOnlyTheTickedApplicationsAreSaved(t *testing.T) {
	t.Parallel()
	log := &recordingLog{}
	profiles, err := store.New(t.TempDir(), log)
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}
	self := domain.ApplicationIdentity{Value: `C:\Programs\ScreenState\ScreenState.exe`}
	captures := application.NewCaptureService(nil, nil, profiles, log, self)
	app := NewApp(nil, nil, nil, captures, nil, log, silentSplash{}, "0.0.0-test", false)

	pigeonpost := domain.ApplicationIdentity{Value: `C:\Programs\PigeonPost\PigeonPost.exe`}
	notepad := domain.ApplicationIdentity{Value: `C:\Windows\notepad.exe`}
	app.review = application.Review{
		Entries: []domain.Entry{
			{Application: pigeonpost, Running: true},
			{Application: notepad, Running: true},
		},
	}

	if err := app.SaveCapture("Desk", []string{pigeonpost.Value}, false); err != nil {
		t.Fatalf("saving: %v", err)
	}
	saved, err := profiles.Load(context.Background(), "Desk")
	if err != nil {
		t.Fatalf("reading the profile back: %v", err)
	}
	if len(saved.Entries) != 1 {
		t.Fatalf("saved %d entries, wanted PigeonPost alone: %+v", len(saved.Entries), saved.Entries)
	}
	if _, found := saved.Find(pigeonpost); !found {
		t.Error("PigeonPost was ticked and not saved")
	}
	if _, found := saved.Find(notepad); found {
		t.Error("Notepad was saved although it was unticked")
	}
}
