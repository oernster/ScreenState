//go:build windows

package win32

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/oernster/ScreenState/internal/application"
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
// attention state, which is a different thing altogether.
//
// The window's placement is read before it is hidden and written back
// afterwards, unchanged. That is what brings it back exactly as it was: showing
// a window with SW_SHOWNOACTIVATE displays it at its most recent size and
// position, which is NOT its show state, so a maximised window came back at its
// normal rectangle. Measured on 2026-09-21, when Claude came back filling the
// left of its screen rather than the whole of it.
//
// SetWindowPlacement also leaves the active window alone, so the window the user
// is working in keeps the keyboard. The window flickers while the button is
// rebuilt, which is the price of the repair.
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
	held := windowPlacement{}
	held.length = uint32(unsafe.Sizeof(held))
	if read, _, err := pGetWindowPlacement.Call(handle, uintptr(unsafe.Pointer(&held))); read == 0 {
		// Without the placement the window cannot be put back as it was; a
		// button is not worth a window left in the wrong state.
		return fmt.Errorf("reading where the window sits: %w", err)
	}
	// ShowWindow answers the window's previous visibility rather than whether
	// it worked, so there is nothing here to read as success.
	_, _, _ = pShowWindow.Call(handle, swHide)
	if restored, _, err := pSetWindowPlacement.Call(handle,
		uintptr(unsafe.Pointer(&held))); restored == 0 {
		return fmt.Errorf("showing the window again: %w", err)
	}
	return nil
}
