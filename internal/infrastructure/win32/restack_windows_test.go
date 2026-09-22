//go:build windows

package win32

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/oernster/ScreenState/internal/application"
)

// FR-081, FR-083 and FR-084 against the real desktop, with two windows of the
// probe's own. They open windows and take the front, so they run only when
// asked for:
//
//	$env:SCREENSTATE_DESKTOP_PROBE = '1'; go test -run 'Restack|StackingOrder' -v ./internal/infrastructure/win32

// swapProbes puts lower above upper and confirms it, skipping where the probe
// could not arrange its own windows.
func swapProbes(t *testing.T, upper, lower uintptr) {
	t.Helper()
	pShowWindow.Call(upper, probeRestore)
	pShowWindow.Call(lower, probeRestore)
	pSetWindowPos.Call(upper, lower, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	probePump()
	if !isAbove(lower, upper) {
		t.Skip("the probe could not put one window behind the other")
	}
}

// FR-081: the order read puts the window on top first.
func TestTheStackingOrderReadPutsTheTopWindowFirst(t *testing.T) {
	if os.Getenv(probeVariable) == "" {
		t.Skipf("opens windows and takes the front; set %s to run it", probeVariable)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	front, back := probeWindows(t)
	swapProbes(t, back, front)

	order, err := NewDesktop(probeClock{}).StackingOrder(context.Background())
	if err != nil {
		t.Fatalf("reading the order: %v", err)
	}
	position := map[application.WindowID]int{}
	for at, id := range order {
		position[id] = at + 1
	}
	top, beneath := position[application.WindowID(front)], position[application.WindowID(back)]
	if top == 0 || beneath == 0 || top > beneath {
		t.Fatalf("the window on top is at %d and the one beneath at %d, of %d", top, beneath, len(order))
	}
}

// FR-083: a restack puts the recorded order back.
func TestRestackingPutsTheRecordedOrderBack(t *testing.T) {
	if os.Getenv(probeVariable) == "" {
		t.Skipf("opens windows and takes the front; set %s to run it", probeVariable)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	first, second := probeWindows(t)
	swapProbes(t, first, second)

	ids := []application.WindowID{application.WindowID(first), application.WindowID(second)}
	if refused := NewDesktop(probeClock{}).Restack(context.Background(), ids); len(refused) != 0 {
		t.Fatalf("refused: %v", refused)
	}
	probePump()
	if !isAbove(first, second) {
		t.Error("the window ranked first is not drawn over the second")
	}
}

// FR-084: restacking activates nothing. The control restacks the ordinary way,
// which activates: if it registers no activation, this test cannot see one.
func TestRestackingDoesNotActivate(t *testing.T) {
	if os.Getenv(probeVariable) == "" {
		t.Skipf("opens windows and takes the front; set %s to run it", probeVariable)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	anchor, subject := probeWindows(t)

	control, _, _ := activationsOf(t, anchor, subject, func() {
		pSetWindowPos.Call(subject, hwndTop, 0, 0, 0, 0, swpNoMove|swpNoSize)
	})
	if control == 0 {
		t.Fatal("restacking the ordinary way registered no activation, so this test cannot see one")
	}
	ids := []application.WindowID{application.WindowID(subject), application.WindowID(anchor)}
	var refused map[application.WindowID]error
	activated, anchorKept, _ := activationsOf(t, anchor, subject, func() {
		refused = NewDesktop(probeClock{}).Restack(context.Background(), ids)
	})
	if len(refused) != 0 {
		t.Fatalf("refused: %v", refused)
	}
	if activated != 0 || !anchorKept {
		t.Errorf("restacking activated the subject %d time(s); the anchor kept the front: %v",
			activated, anchorKept)
	}
	if !isAbove(subject, anchor) {
		t.Error("the subject, ranked first, is not drawn over the anchor")
	}
}
