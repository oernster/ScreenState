//go:build windows

package win32

import (
	"context"
	"unsafe"

	"github.com/oernster/ScreenState/internal/application"
)

// flashStop is FLASHW_STOP: end the attention the window is drawing to itself
// and put its caption back to the ordinary state, taking its taskbar button
// with it.
const flashStop = 0

// flashInfo is FLASHWINFO: which window, what to do, how many times and how
// often. Stopping needs none of the last two, which are left at zero.
type flashInfo struct {
	size    uint32
	window  uintptr
	flags   uint32
	count   uint32
	timeout uint32
}

// StopDrawingAttention puts a window's taskbar button back in its ordinary
// state and answers whether it was drawing attention (FR-073).
//
// Measured on the reference machine on 2026-09-21: after a sign-in restore the
// buttons of the windows the restore had placed were drawn lit; those windows
// reported their captions drawn active while another window held the
// foreground. FlashWindowEx with FLASHW_STOP puts that back.
//
// It moves nothing and activates nothing: the window keeps its place, its size
// and its show state, while the window the user is typing into keeps the
// keyboard.
func (desktop *Desktop) StopDrawingAttention(
	ctx context.Context,
	id application.WindowID,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	handle := uintptr(id)
	if alive, _, _ := pIsWindow.Call(handle); alive == 0 {
		return false, application.ErrWindowGone
	}
	information := flashInfo{window: handle, flags: flashStop}
	information.size = uint32(unsafe.Sizeof(information))
	// FlashWindowEx answers the state the window was in before the call rather
	// than whether the call worked, so a zero means it was not drawing
	// attention rather than that anything failed.
	wasLit, _, _ := pFlashWindowEx.Call(uintptr(unsafe.Pointer(&information)))
	return wasLit != 0, nil
}
