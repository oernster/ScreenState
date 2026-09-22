package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// Amendment 1: a capture records the stacking order as a rank on each
// placement, relative to the profile's own windows (FR-081, FR-082).

// capturedOrder is a desktop holding Claude (PigeonPost) drawn over Terminal
// (Notepad), with windows the profile will not hold between and around them.
func capturedOrder() *fakeDesktop {
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows: []Window{
			aWindow(1, pigeonpost, at(0)),
			aWindow(2, notepad, at(1)),
			aWindow(3, screenst, at(2)),
		},
	}
	desktop.stacking.order = []WindowID{9, 1, 3, 2}
	return desktop
}

// rankOf answers the rank of the one placement an application holds.
func rankOf(t *testing.T, profile domain.Profile, application domain.ApplicationIdentity) int {
	t.Helper()
	entry, found := profile.Find(application)
	if !found || len(entry.Placements) != 1 {
		t.Fatalf("the profile does not hold one placement for %s: %+v", application, profile)
	}
	return entry.Placements[0].Rank
}

// FR-081: ranks are among the profile's own placements, 1 on top.
func TestACaptureRecordsTheStackingOrderAmongItsOwnWindows(t *testing.T) {
	t.Parallel()
	service := captureUnder(capturedOrder(), newFakeStore())
	review, err := service.Review(context.Background())
	if err != nil || review.Unstacked != "" {
		t.Fatalf("the capture failed: %v, %q", err, review.Unstacked)
	}
	profile, err := service.Save(context.Background(), "Desk", review.Entries, false)
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	if rankOf(t, profile, pigeonpost) != 1 || rankOf(t, profile, notepad) != 2 {
		t.Fatalf("Claude holds %d and Terminal %d, want 1 and 2",
			rankOf(t, profile, pigeonpost), rankOf(t, profile, notepad))
	}
}

// FR-081: an entry removed in the review leaves no gap in the ranks saved.
func TestRanksAreAmongTheEntriesKept(t *testing.T) {
	t.Parallel()
	service := captureUnder(capturedOrder(), newFakeStore())
	review, err := service.Review(context.Background())
	if err != nil {
		t.Fatalf("the capture failed: %v", err)
	}
	kept, _ := domain.NewProfile("Desk", review.Entries...)
	terminal, _ := kept.Find(notepad)
	profile, err := service.Save(context.Background(), "Desk", []domain.Entry{terminal}, false)
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	if rank := rankOf(t, profile, notepad); rank != 1 {
		t.Fatalf("Terminal alone holds rank %d, want 1", rank)
	}
}

// FR-082: an order that cannot be read is said in the review; the profile is
// saved without ranks.
func TestAnOrderThatCannotBeReadIsSaidAndNothingIsRanked(t *testing.T) {
	t.Parallel()
	desktop := capturedOrder()
	desktop.stacking.orderErr = errors.New("access is denied")
	service := captureUnder(desktop, newFakeStore())
	review, err := service.Review(context.Background())
	if err != nil {
		t.Fatalf("the capture failed over the order alone: %v", err)
	}
	if !anyContaining([]string{review.Unstacked}, "could not be read") {
		t.Fatalf("the review does not say so: %q", review.Unstacked)
	}
	profile, err := service.Save(context.Background(), "Desk", review.Entries, false)
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	if ranked, _ := domain.StackingOf(profile.Entries); ranked {
		t.Fatalf("the profile holds ranks: %+v", profile)
	}
}

// FR-082: a window missing from the order means the order changed while it was
// read; half an order is not one, so none is kept.
func TestAnOrderMissingAWindowRanksNothing(t *testing.T) {
	t.Parallel()
	desktop := capturedOrder()
	desktop.stacking.unlisted = map[WindowID]bool{2: true}
	review, err := captureUnder(desktop, newFakeStore()).Review(context.Background())
	if err != nil {
		t.Fatalf("the capture failed: %v", err)
	}
	if !anyContaining([]string{review.Unstacked}, "changed while it was read") {
		t.Fatalf("the review does not say so: %q", review.Unstacked)
	}
	if ranked, err := domain.StackingOf(review.Entries); ranked || err != nil {
		t.Fatalf("the review still ranks: %v, %v", ranked, err)
	}
}
