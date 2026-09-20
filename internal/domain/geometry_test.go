package domain

import (
	"errors"
	"testing"
)

func TestShowStateRoundTripsThroughItsStoredSpelling(t *testing.T) {
	t.Parallel()
	for _, state := range []ShowState{ShowNormal, ShowMinimised, ShowMaximised} {
		parsed, err := ParseShowState(state.String())
		if err != nil {
			t.Fatalf("%s did not parse back: %v", state, err)
		}
		if parsed != state {
			t.Fatalf("%s parsed back as %s", state, parsed)
		}
		if !state.Valid() {
			t.Fatalf("%s reported itself invalid", state)
		}
	}
}

func TestTheZeroShowStateIsTheOrdinaryOne(t *testing.T) {
	t.Parallel()
	// A placement that says nothing about show state means a normal window,
	// never an invalid one.
	var unset ShowState
	if unset != ShowNormal || !unset.Valid() {
		t.Fatalf("the zero show state is %s", unset)
	}
}

func TestAnUnknownShowStateIsRejectedRatherThanGuessed(t *testing.T) {
	t.Parallel()
	unknown := ShowState(200)
	if unknown.Valid() {
		t.Fatal("an unknown state reported itself valid")
	}
	if unknown.String() != "ShowState(200)" {
		t.Fatalf("unhelpful rendering: %s", unknown)
	}
	if _, err := ParseShowState("sideways"); !errors.Is(err, ErrUnknownShowState) {
		t.Fatalf("expected ErrUnknownShowState, got %v", err)
	}
}

func TestARectangleNeedsAPositiveSize(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		rect  Rect
		valid bool
	}{
		"ordinary":     {Rect{X: 10, Y: 20, Width: 800, Height: 600}, true},
		"off the left": {Rect{X: -4079, Y: 1407, Width: 3872, Height: 2312}, true},
		"no width":     {Rect{Width: 0, Height: 600}, false},
		"no height":    {Rect{Width: 800, Height: 0}, false},
		"negative":     {Rect{Width: -800, Height: -600}, false},
	}
	for name, testCase := range cases {
		if testCase.rect.Valid() != testCase.valid {
			t.Fatalf("%s: wanted valid=%t", name, testCase.valid)
		}
	}
}

func TestARectangleReportsItsFarEdges(t *testing.T) {
	t.Parallel()
	// The measured maximised window on the left-hand display, which sits at a
	// negative origin because it is left of the primary.
	rect := Rect{X: -4079, Y: 1407, Width: 3872, Height: 2312}
	if rect.Right() != -207 {
		t.Fatalf("right edge %d", rect.Right())
	}
	if rect.Bottom() != 3719 {
		t.Fatalf("bottom edge %d", rect.Bottom())
	}
	if rect.String() != "x=-4079 y=1407 w=3872 h=2312" {
		t.Fatalf("unexpected rendering: %s", rect)
	}
}

func TestTouchingRectanglesDoNotIntersect(t *testing.T) {
	t.Parallel()
	// This decides which display a window belongs to, so an edge that merely
	// meets another display must not count as being on it.
	left := Rect{X: 0, Y: 0, Width: 100, Height: 100}
	right := Rect{X: 100, Y: 0, Width: 100, Height: 100}
	below := Rect{X: 0, Y: 100, Width: 100, Height: 100}
	overlapping := Rect{X: 99, Y: 99, Width: 100, Height: 100}

	if left.Intersects(right) || right.Intersects(left) {
		t.Fatal("rectangles meeting at a vertical edge were treated as overlapping")
	}
	if left.Intersects(below) || below.Intersects(left) {
		t.Fatal("rectangles meeting at a horizontal edge were treated as overlapping")
	}
	if !left.Intersects(overlapping) || !overlapping.Intersects(left) {
		t.Fatal("rectangles sharing a corner area were treated as separate")
	}
}

func TestIdentityKindRoundTripsThroughItsStoredSpelling(t *testing.T) {
	t.Parallel()
	for _, kind := range []IdentityKind{KindPath, KindAppUserModelID, KindUpdaterCommand} {
		parsed, err := ParseIdentityKind(kind.String())
		if err != nil {
			t.Fatalf("%s did not parse back: %v", kind, err)
		}
		if parsed != kind || !kind.Valid() {
			t.Fatalf("%s parsed back as %s", kind, parsed)
		}
	}
}

func TestTheZeroIdentityKindIsAPath(t *testing.T) {
	t.Parallel()
	var unset IdentityKind
	if unset != KindPath {
		t.Fatalf("the zero identity kind is %s", unset)
	}
}

func TestAnUnknownIdentityKindIsRejected(t *testing.T) {
	t.Parallel()
	unknown := IdentityKind(9)
	if unknown.Valid() {
		t.Fatal("an unknown kind reported itself valid")
	}
	if unknown.String() != "IdentityKind(9)" {
		t.Fatalf("unhelpful rendering: %s", unknown)
	}
	if _, err := ParseIdentityKind("registry"); !errors.Is(err, ErrUnknownIdentityKind) {
		t.Fatalf("expected ErrUnknownIdentityKind, got %v", err)
	}
}

func TestOverlappingRectanglesShareTheExpectedArea(t *testing.T) {
	t.Parallel()
	left := Rect{X: 0, Y: 0, Width: 100, Height: 100}
	right := Rect{X: 60, Y: 20, Width: 100, Height: 100}
	shared := left.Intersection(right)
	want := Rect{X: 60, Y: 20, Width: 40, Height: 80}
	if shared != want {
		t.Fatalf("shared %s, wanted %s", shared, want)
	}
	if shared.Area() != 40*80 {
		t.Fatalf("area %d", shared.Area())
	}
	if other := right.Intersection(left); other != want {
		t.Fatalf("intersection is not symmetric: %s", other)
	}
}

func TestRectanglesThatOnlyTouchShareNoArea(t *testing.T) {
	t.Parallel()
	left := Rect{X: 0, Y: 0, Width: 100, Height: 100}
	for name, other := range map[string]Rect{
		"edge to edge": {X: 100, Y: 0, Width: 100, Height: 100},
		"corner":       {X: 100, Y: 100, Width: 100, Height: 100},
		"clear away":   {X: 500, Y: 500, Width: 10, Height: 10},
		"above":        {X: 0, Y: -100, Width: 100, Height: 100},
	} {
		shared := left.Intersection(other)
		if shared.Valid() {
			t.Fatalf("%s: reported a shared rectangle %s", name, shared)
		}
		if shared.Area() != 0 {
			t.Fatalf("%s: reported area %d", name, shared.Area())
		}
	}
}

func TestAContainedRectangleIsItsOwnIntersection(t *testing.T) {
	t.Parallel()
	display := Rect{X: -3840, Y: 0, Width: 3840, Height: 2400}
	window := Rect{X: -2000, Y: 100, Width: 800, Height: 600}
	if shared := display.Intersection(window); shared != window {
		t.Fatalf("shared %s, wanted the window itself", shared)
	}
	if window.Area() != 800*600 {
		t.Fatalf("area %d", window.Area())
	}
}
