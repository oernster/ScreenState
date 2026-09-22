package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// rankedPlacement is a placement holding a rank, on a display of the
// reference machine.
func rankedPlacement(rank int) domain.Placement {
	return domain.Placement{
		Display: domain.DisplayIdentity{MonitorID: "DISPLAY#HSJ1340#5&14514d51&0&UID4354"},
		Rect:    domain.Rect{X: 0, Y: 0, Width: 3440, Height: 1392},
		State:   domain.ShowMaximised,
		Rank:    rank,
	}
}

// DATA-006: a rank is written with its placement and read back unchanged.
func TestARankIsStoredWithItsPlacement(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()
	entry := domain.Entry{Application: domain.ApplicationIdentity{Value: `C:\a.exe`}, Running: true}.
		WithPlacement(rankedPlacement(2)).WithPlacement(rankedPlacement(1))
	profile, err := domain.NewProfile("Desk", entry)
	if err != nil {
		t.Fatalf("building: %v", err)
	}
	if err := store.Save(ctx, profile); err != nil {
		t.Fatalf("saving: %v", err)
	}
	loaded, err := store.Load(ctx, "Desk")
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	placements := loaded.Entries[0].Placements
	if placements[0].Rank != 2 || placements[1].Rank != 1 {
		t.Fatalf("read back %+v, want ranks 2 then 1", placements)
	}
}

// DATA-006: a placement with no rank writes no rank field, so a profile saved
// without a stacking order is the file an older build wrote.
func TestNoRankWritesNoField(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	entry := domain.Entry{Application: domain.ApplicationIdentity{Value: `C:\a.exe`}, Running: true}.
		WithPlacement(rankedPlacement(domain.NoRank))
	profile, _ := domain.NewProfile("Desk", entry)
	if err := store.Save(context.Background(), profile); err != nil {
		t.Fatalf("saving: %v", err)
	}
	raw, err := os.ReadFile(store.pathFor("Desk"))
	if err != nil {
		t.Fatalf("reading the file: %v", err)
	}
	if strings.Contains(string(raw), `"rank"`) {
		t.Fatalf("an unranked placement was written with a rank:\n%s", raw)
	}
}

// DATA-006: a file written before ranks were recorded is read as holding none.
func TestAFileWithoutRanksReadsAsHoldingNone(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	older := `{"format":1,"name":"Old","entries":[{"application":{"kind":"path","value":"C:\\a.exe"},` +
		`"running":true,"placements":[{"display":"D","rect":{"x":0,"y":0,"width":1,"height":1},"state":"normal"}]}]}`
	if err := os.WriteFile(filepath.Join(store.Directory(), "old.json"), []byte(older), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}
	loaded, err := store.Load(context.Background(), "Old")
	if err != nil {
		t.Fatalf("an older file was refused: %v", err)
	}
	if ranked, err := domain.StackingOf(loaded.Entries); ranked || err != nil {
		t.Fatalf("an older file reads as ranked: %v, %v", ranked, err)
	}
}

// DATA-007: ranks that cannot be used do not cost the profile; the restore
// decides what to do with them.
func TestRanksThatCannotBeUsedStillLoad(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	shared := `{"format":1,"name":"Edited","entries":[{"application":{"kind":"path","value":"C:\\a.exe"},` +
		`"running":true,"placements":[` +
		`{"display":"D","rect":{"x":0,"y":0,"width":1,"height":1},"state":"normal","rank":1},` +
		`{"display":"D","rect":{"x":0,"y":0,"width":1,"height":1},"state":"normal","rank":1}]}]}`
	if err := os.WriteFile(filepath.Join(store.Directory(), "edited.json"), []byte(shared), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}
	if _, err := store.Load(context.Background(), "Edited"); err != nil {
		t.Fatalf("a profile with shared ranks was refused: %v", err)
	}
}
