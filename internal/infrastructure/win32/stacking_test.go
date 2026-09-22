package win32

import (
	"errors"
	"reflect"
	"testing"
)

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

// downward answers the window below each one, from a list given top first.
func downward(topFirst ...uintptr) func(uintptr) uintptr {
	below := make(map[uintptr]uintptr, len(topFirst))
	for index := 1; index < len(topFirst); index++ {
		below[topFirst[index-1]] = topFirst[index]
	}
	return func(window uintptr) uintptr { return below[window] }
}

// FR-081: the order is walked from the top, every window on the way.
func TestTheStackingOrderIsWalkedFromTheTop(t *testing.T) {
	t.Parallel()
	got := stackingOrder(pigeonPost, downward(pigeonPost, claude, inputMethod, leftTerminal))
	want := []uintptr{pigeonPost, claude, inputMethod, leftTerminal}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walked %#x, want %#x", got, want)
	}
}

func TestAStackingOrderWalkThatLoopsEnds(t *testing.T) {
	t.Parallel()
	looping := func(window uintptr) uintptr {
		if window == claude {
			return pigeonPost
		}
		return claude
	}
	if got := stackingOrder(pigeonPost, looping); len(got) != 2 {
		t.Fatalf("walked %#x, want the two windows once each", got)
	}
}

func TestTheHighestOfSomeWindowsIsTheFirstMet(t *testing.T) {
	t.Parallel()
	order := []uintptr{pigeonPost, claude, inputMethod, leftTerminal}
	if got, found := highestOf(order, []uintptr{leftTerminal, claude}); !found || got != claude {
		t.Fatalf("got %#x, %v; want Claude", got, found)
	}
	if _, found := highestOf(order, []uintptr{0x1}); found {
		t.Fatal("found a window the order does not hold")
	}
}

// FR-083, FR-085 and FR-086: each goes beneath the last one that moved, so a
// refusal costs that window alone.
func TestRestackingChainsBeneathTheLastWindowThatMoved(t *testing.T) {
	t.Parallel()
	var calls [][2]uintptr
	refusal := errors.New("access is denied")
	refused := restackBeneath([]uintptr{claude, pigeonPost, leftTerminal}, inputMethod,
		func(window, above uintptr) error {
			calls = append(calls, [2]uintptr{window, above})
			if window == pigeonPost {
				return refusal
			}
			return nil
		})
	want := [][2]uintptr{{claude, inputMethod}, {pigeonPost, claude}, {leftTerminal, claude}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("stacked %#x, want %#x", calls, want)
	}
	if len(refused) != 1 || !errors.Is(refused[pigeonPost], refusal) {
		t.Fatalf("refused %v, want PigeonPost alone", refused)
	}
}
