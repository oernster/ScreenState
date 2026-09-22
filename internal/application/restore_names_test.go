package application

import (
	"slices"
	"testing"
)

// readingOf returns the displays as a set, failing the test where there are none.
func readingOf(t *testing.T, displays ...Display) displaySet {
	t.Helper()
	set, err := newDisplaySet(displays)
	if err != nil {
		t.Fatalf("a reading of %d displays: %v", len(displays), err)
	}
	return set
}

func TestARestoreNamesEachDisplayOnceAndKeepsTheName(t *testing.T) {
	t.Parallel()
	left := screen("L", -100, 0, 100, 100, false)
	right := screen("R", 0, 0, 100, 100, true)
	arrived := screen("A", -100, 0, 100, 100, false)
	second := screen("B", -100, 0, 100, 100, false)
	names := newStableNames()

	if !names.learn(readingOf(t, left, right)) {
		t.Fatal("the first reading named nothing")
	}
	// The left display goes: the one left alone keeps its name rather than
	// becoming the only display.
	if names.learn(readingOf(t, right)) {
		t.Error("a reading holding nothing new named something")
	}
	if got := names.name(right.Identity); got != "right display" {
		t.Errorf("the display left alone was renamed %q", got)
	}
	// A display arriving where the left one was cannot take its name, which
	// still means the display that went.
	names.learn(readingOf(t, arrived, right))
	if got := names.name(arrived.Identity); got != "left display"+arrivedSuffix {
		t.Errorf("a display arriving at a held position is named %q", got)
	}
	// A second arrival there takes the one name that cannot collide.
	names.learn(readingOf(t, second, right))
	if got := names.name(second.Identity); got != second.Identity.String() {
		t.Errorf("a second arrival at a held position is named %q", got)
	}
	if got := names.name(screen("X", 0, 0, 1, 1, false).Identity); got != displayGone {
		t.Errorf("a display the restore never saw is named %q", got)
	}

	want := []string{
		"B is B",
		"left display (connected during the restore) is A",
		"left display is L",
		"right display is R",
	}
	if got := names.legend(); !slices.Equal(got, want) {
		t.Errorf("the legend is %v, want %v", got, want)
	}
	if got := names.legendOf(readingOf(t, right)); !slices.Equal(got, []string{"right display is R"}) {
		t.Errorf("the legend of one reading is %v", got)
	}
}
