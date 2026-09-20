//go:build windows

package win32

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// taskbarClasses name the shell's own windows: the taskbar on the primary
// display and one of the second class on every other display. A machine with
// four displays has four of them, which is why both are asked for rather than
// the first one found being taken as the answer.
var taskbarClasses = []string{"Shell_TrayWnd", "Shell_SecondaryTrayWnd"}

// Redraw flags, from RedrawWindow. Together they mean: mark the whole window
// and everything in it as needing paint, then paint it now rather than when the
// shell next gets round to it.
const (
	rdwInvalidate  = 0x0001
	rdwErase       = 0x0004
	rdwAllChildren = 0x0080
	rdwUpdateNow   = 0x0100
)

// RefreshTaskbar asks every taskbar to paint itself again, answering how many
// were asked (FR-067).
//
// It is a repaint and nothing else: RedrawWindow marks another process's window
// as needing paint. It cannot move that window, close it or tell it anything,
// so the worst this can do to the shell is make it draw what it already holds.
//
// Why it is here at all: after a restore at sign-in, the taskbar buttons of the
// applications the agent started were drawn without their icons on the
// reference machine; they stayed that way until the user clicked anywhere on
// the taskbar. The icons themselves were right; the drawing of them was stale. This
// is the same invalidation that click causes, asked for rather than waited for.
func (desktop *Desktop) RefreshTaskbar(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	var asked int
	for _, class := range taskbarClasses {
		name, err := windows.UTF16PtrFromString(class)
		if err != nil {
			return asked, fmt.Errorf("naming the taskbar class: %w", err)
		}
		var handle uintptr
		for {
			found, _, _ := pFindWindowEx.Call(0, handle,
				uintptr(unsafe.Pointer(name)), 0)
			if found == 0 {
				break
			}
			handle = found
			_, _, _ = pRedrawWindow.Call(handle, 0, 0,
				rdwInvalidate|rdwErase|rdwAllChildren|rdwUpdateNow)
			asked++
		}
	}
	return asked, nil
}
