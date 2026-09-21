package win32

import "testing"

// The stacking order measured before the left Terminal's rebuild on
// 2026-09-21, top first, with the hidden windows between them that the old
// restack went beneath. Handles are stand-ins; the order is the measurement.
const (
	pigeonPost   uintptr = 0x1081c
	claude       uintptr = 0x10aaa
	inputMethod  uintptr = 0x10a6e // MSCTFIME UI, hidden
	leftTerminal uintptr = 0x207ee
)

// stack answers the window above each one, from a list given top first.
func stack(topFirst ...uintptr) func(uintptr) uintptr {
	above := make(map[uintptr]uintptr, len(topFirst))
	for index := 1; index < len(topFirst); index++ {
		above[topFirst[index]] = topFirst[index-1]
	}
	return func(window uintptr) uintptr { return above[window] }
}

// seen answers whether a window is one a person would call a window.
func seen(windows ...uintptr) func(uintptr) bool {
	visible := make(map[uintptr]bool, len(windows))
	for _, window := range windows {
		visible[window] = true
	}
	return func(window uintptr) bool { return visible[window] }
}

func TestAHiddenWindowAboveIsPassedOver(t *testing.T) {
	t.Parallel()
	above := stack(pigeonPost, claude, inputMethod, leftTerminal)
	got := nearestWindowAbove(leftTerminal, above, seen(pigeonPost, claude, leftTerminal))
	if got != claude {
		t.Fatalf("the left Terminal goes back beneath %#x, want Claude %#x", got, claude)
	}
}

func TestAVisibleWindowDirectlyAboveIsTaken(t *testing.T) {
	t.Parallel()
	above := stack(pigeonPost, claude, leftTerminal)
	got := nearestWindowAbove(leftTerminal, above, seen(pigeonPost, claude, leftTerminal))
	if got != claude {
		t.Fatalf("got %#x, want Claude %#x", got, claude)
	}
}

func TestNothingAboveButMachineryMeansTheTop(t *testing.T) {
	t.Parallel()
	above := stack(inputMethod, leftTerminal)
	if got := nearestWindowAbove(leftTerminal, above, seen(leftTerminal)); got != 0 {
		t.Fatalf("got %#x, want zero for the top", got)
	}
}

func TestTheTopWindowHasNothingAbove(t *testing.T) {
	t.Parallel()
	above := stack(pigeonPost, claude)
	if got := nearestWindowAbove(pigeonPost, above, seen(pigeonPost, claude)); got != 0 {
		t.Fatalf("got %#x, want zero for the top", got)
	}
}

func TestAStackingOrderThatLoopsEnds(t *testing.T) {
	t.Parallel()
	looping := func(window uintptr) uintptr {
		if window == leftTerminal {
			return inputMethod
		}
		return leftTerminal
	}
	if got := nearestWindowAbove(leftTerminal, looping, seen()); got != 0 {
		t.Fatalf("got %#x, want zero once the walk comes round again", got)
	}
}
