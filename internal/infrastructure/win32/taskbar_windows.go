//go:build windows

package win32

import (
	"context"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// taskbarClasses are the two window classes Explorer draws taskbars with: the
// first display's and every other display's.
var taskbarClasses = []string{"Shell_TrayWnd", "Shell_SecondaryTrayWnd"}

const (
	// The messages a left press and release send.
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	// mkLButton says the left button is down, which a press carries.
	mkLButton = 0x0001

	// clickAt is where on a taskbar the click lands, in its own coordinates: a
	// few pixels in from its top left corner. The buttons sit in the middle of
	// the bar, so that corner holds nothing to press.
	clickAt = 5

	// classNameMax bounds a window class name read into memory.
	classNameMax = 256
)

// NudgeTaskbars posts a left click to every taskbar and answers how many were
// sent one (FR-072).
//
// What it is for, measured on the reference machine on 2026-09-21: Explorer
// draws the taskbar button of an application started at sign-in without its
// icon on every display but the first; it leaves that button grey until the
// user clicks any taskbar. It does the same mid-session for an application
// started by hand with this product not running, so it is Windows rather than
// anything this product does.
//
// A posted click is the whole repair and was measured to be enough. It is the
// least this can be: the message goes straight to the taskbar's window, so the
// pointer is not moved, nothing is activated and the user's foreground window
// is left alone. The five answers that were measured NOT to work, so that
// nobody tries them again: asking the taskbars to repaint (RedrawWindow),
// telling the shell its icons may have changed (SHChangeNotify with
// SHCNE_ASSOCCHANGED), bringing each taskbar to the front and back, then
// broadcasting TaskbarCreated, a setting change or a theme change.
func (desktop *Desktop) NudgeTaskbars(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	position := uintptr(clickAt) | uintptr(clickAt)<<16
	sent := 0
	for _, window := range handles() {
		if !isTaskbar(window) {
			continue
		}
		// PostMessage answers whether the message reached the queue, which is
		// not whether Explorer acted on it. Nothing here can know that, so a
		// refusal is counted as not sent and nothing more is claimed.
		if posted, _, _ := pPostMessage.Call(window, wmLButtonDown, mkLButton, position); posted == 0 {
			continue
		}
		_, _, _ = pPostMessage.Call(window, wmLButtonUp, 0, position)
		sent++
	}
	return sent, nil
}

// isTaskbar reports whether a window is one of Explorer's taskbars.
func isTaskbar(window uintptr) bool {
	name := classOf(window)
	for _, class := range taskbarClasses {
		if strings.EqualFold(name, class) {
			return true
		}
	}
	return false
}

// classOf answers a window's class name, empty where it could not be read.
func classOf(window uintptr) string {
	buffer := make([]uint16, classNameMax)
	length, _, _ := pGetClassName.Call(window,
		uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if length == 0 || int(length) > len(buffer) {
		return ""
	}
	return windows.UTF16ToString(buffer[:length])
}
