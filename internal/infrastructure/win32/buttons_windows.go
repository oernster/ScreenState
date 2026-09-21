//go:build windows

package win32

import (
	"context"

	"github.com/oernster/ScreenState/internal/application"
)

// RebuildTaskbarButton has the shell drop a window's taskbar button and build
// it anew (FR-075).
//
// What it is for, measured on the reference machine on 2026-09-21: a taskbar
// marks the window last activated on its display. An application that starts
// puts its own window in front, so a desktop assembled at sign-in comes
// back marked on every display although the recorded desktop had no such marks.
// Hiding the window and showing it again is the only answer measured to clear
// it: everything else was tried and did nothing, including a posted click on
// the taskbars, telling the shell its icons may have changed and ending the
// attention state, which is a different thing altogether.
//
// The window is shown again without being activated, so it keeps its place, its
// size and its show state, while the window the user is working in keeps the
// keyboard. It flickers while the button is rebuilt, which is the price of the
// repair.
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
	// ShowWindow answers the window's previous visibility rather than whether
	// it worked, so there is nothing here to read as success. A window that
	// does not come back is read as gone by the caller's own next pass.
	_, _, _ = pShowWindow.Call(handle, swHide)
	_, _, _ = pShowWindow.Call(handle, swShowNoActivate)
	return nil
}
