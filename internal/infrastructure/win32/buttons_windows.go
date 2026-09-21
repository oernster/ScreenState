//go:build windows

package win32

import (
	"context"

	"github.com/oernster/ScreenState/internal/application"
)

// Rebuilding a taskbar button without disturbing the window.
const (
	// swShowNA shows a window in its current state without activating it.
	// Hiding a window leaves its state alone, so a maximised window comes back
	// maximised. Neither SW_SHOWNOACTIVATE, which shows a window at its normal
	// rectangle, nor SetWindowPlacement, which activates a maximised window,
	// will do: both were measured going wrong on 2026-09-21.
	swShowNA = 8
	// gwHwndPrev names the window directly above another in the stacking order.
	gwHwndPrev = 3
	// hwndTop puts a window at the top of the stacking order, used when nothing
	// was above it.
	hwndTop = 0
	// swpNoSize and swpNoMove leave a window's size and place alone while it is
	// restacked.
	swpNoSize = 0x0001
	swpNoMove = 0x0002
)

// RebuildTaskbarButton has the shell drop a window's taskbar button and build
// it anew (FR-075).
//
// What it is for, measured on the reference machine on 2026-09-21: a taskbar
// marks the window last activated on its display. An application that starts
// puts its own window in front, so a desktop assembled at sign-in comes back
// marked on every display although the recorded desktop had no such marks.
// Hiding the window and showing it again is the only answer measured to clear
// it: everything else was tried and did nothing, including a posted click on
// the taskbars, telling the shell its icons may have changed and ending the
// attention state.
//
// The window comes back exactly as it was, in three respects, each learned by
// getting it wrong. Its state: it is shown with SW_SHOWNA. Its activation:
// SW_SHOWNA does not activate, where showing a maximised window through
// SetWindowPlacement did, which a shell watcher recorded for PigeonPost,
// Stellody and Claude. Its place in the stacking order: a window shown again
// goes on top, so it is put back beneath the nearest window above it that a
// person would call a window. Not simply the one directly above: that was a
// hidden input method window for the left Terminal, which came back on top of
// Claude beneath it (see nearestWindowAbove).
func (desktop *Desktop) RebuildTaskbarButton(
	ctx context.Context,
	id application.WindowID,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	handle := uintptr(id)
	if alive, _, _ := pIsWindow.Call(handle); alive == 0 {
		return application.ErrWindowGone
	}
	above := nearestWindowAbove(handle, windowDirectlyAbove, isCandidate)
	if above == 0 {
		above = hwndTop
	}
	// ShowWindow answers the window's previous visibility rather than whether
	// it worked, so there is nothing here to read as success.
	_, _, _ = pShowWindow.Call(handle, swHide)
	_, _, _ = pShowWindow.Call(handle, swShowNA)
	// A restack that fails leaves the window on top, which is untidy rather
	// than wrong, so there is nothing here worth failing the repair over.
	_, _, _ = pSetWindowPos.Call(handle, above, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate)
	return nil
}

// windowDirectlyAbove answers the window immediately above another in the
// stacking order, whatever it is; zero at the top.
func windowDirectlyAbove(handle uintptr) uintptr {
	above, _, _ := pGetWindow.Call(handle, gwHwndPrev)
	return above
}
