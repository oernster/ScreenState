//go:build windows

package win32

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
)

// Placing a window must not change which window is on top of which.
//
// Reported 2026-09-22: Windows Terminal was captured behind Claude on the
// primary display; after Apply it was on top of Claude. This settles whether
// Desktop.Place (the path a restore takes) is what moves it, with two windows
// of its own: one is put behind the other, then placed as a restore places it,
// then the stacking order is read again. It opens windows and takes the front,
// so it runs only when asked for:
//
//	$env:SCREENSTATE_DESKTOP_PROBE = '1'; go test -run TestPlacingKeepsTheStackingOrder -v ./internal/infrastructure/win32

// isAbove reports whether upper is anywhere above lower in the stacking order.
func isAbove(upper, lower uintptr) bool {
	visited := map[uintptr]bool{lower: true}
	for current := windowDirectlyAbove(lower); current != 0; current = windowDirectlyAbove(current) {
		if current == upper {
			return true
		}
		if visited[current] {
			return false
		}
		visited[current] = true
	}
	return false
}

func TestPlacingKeepsTheStackingOrder(t *testing.T) {
	if os.Getenv(probeVariable) == "" {
		t.Skipf("opens windows and takes the front; set %s to run it", probeVariable)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	front, back := probeWindows(t)
	desktop := NewDesktop(probeClock{})
	target := domain.Rect{X: probeLeft, Y: probeTop, Width: probeWidth, Height: probeHeight}

	for _, state := range []domain.ShowState{domain.ShowNormal, domain.ShowMaximised} {
		pShowWindow.Call(front, probeRestore)
		pShowWindow.Call(back, probeRestore)
		pSetWindowPos.Call(back, front, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
		probePump()
		if !isAbove(front, back) {
			t.Skipf("%s: the probe could not put one window behind the other", state)
		}
		if err := desktop.Place(context.Background(), application.WindowID(back), target, state); err != nil {
			t.Fatalf("%s: placing: %v", state, err)
		}
		probePump()
		if !isAbove(front, back) {
			t.Errorf("placing a window %s brought it above the window that was in front of it", state)
		}
	}
}
