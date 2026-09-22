//go:build windows

package win32

import (
	"context"
	"fmt"

	"github.com/oernster/ScreenState/internal/application"
)

// Reading and setting the stacking order (FR-081, FR-083).
var pGetTopWindow = user32.NewProc("GetTopWindow")

const (
	// gwHwndNext names the window directly below another in the stacking order.
	gwHwndNext = 2
	// wsExTopmost marks a window kept above every window that is not. A window
	// stacked beneath one is made topmost itself, so no such window is ever
	// taken as the place to stack beneath.
	wsExTopmost = 0x00000008
)

// StackingOrder answers every top-level window, the one on top first, by
// walking down from the top window (FR-081).
func (desktop *Desktop) StackingOrder(ctx context.Context) ([]application.WindowID, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	top, _, callErr := pGetTopWindow.Call(0)
	if top == 0 {
		return nil, fmt.Errorf("%w: %v", ErrStackingUnread, callErr)
	}
	order := stackingOrder(top, windowDirectlyBelow)
	ids := make([]application.WindowID, 0, len(order))
	for _, handle := range order {
		ids = append(ids, application.WindowID(handle))
	}
	return ids, nil
}

// Restack stacks the windows so each is drawn directly beneath the one before
// it, the first taking the highest place any of them holds now (FR-083).
//
// SetWindowPos with SWP_NOACTIVATE changes the order and nothing else: no
// window is activated, so no taskbar button is marked (FR-084). The first
// window goes beneath the nearest window above that place a person would call
// a window, as keepStackingPlace does, so a hidden input method window is not
// taken for one; where there is none it goes to the top of the windows that
// are not topmost. Everything else on the desktop keeps its place relative to
// the profile's windows: a manager the user pressed Apply in stays in front.
func (desktop *Desktop) Restack(
	ctx context.Context,
	ids []application.WindowID,
) map[application.WindowID]error {
	refused := make(map[application.WindowID]error)
	if err := ctx.Err(); err != nil {
		for _, id := range ids {
			refused[id] = err
		}
		return refused
	}
	var alive []uintptr
	for _, id := range ids {
		if living, _, _ := pIsWindow.Call(uintptr(id)); living == 0 {
			refused[id] = application.ErrWindowGone
			continue
		}
		alive = append(alive, uintptr(id))
	}
	anchor := uintptr(hwndTop)
	if top, _, _ := pGetTopWindow.Call(0); top != 0 {
		if highest, found := highestOf(stackingOrder(top, windowDirectlyBelow), alive); found {
			anchor = nearestWindowAbove(highest, windowDirectlyAbove, isOrdinaryWindow)
		}
	}
	moved := restackBeneath(alive, anchor, func(window, above uintptr) error {
		if ok, _, callErr := pSetWindowPos.Call(window, above, 0, 0, 0, 0,
			swpNoMove|swpNoSize|swpNoActivate); ok == 0 {
			return fmt.Errorf("%w: %v", ErrRestackRefused, callErr)
		}
		return nil
	})
	for handle, err := range moved {
		refused[application.WindowID(handle)] = err
	}
	return refused
}

// windowDirectlyBelow answers the window immediately below another in the
// stacking order, whatever it is; zero at the bottom.
func windowDirectlyBelow(handle uintptr) uintptr {
	below, _, _ := pGetWindow.Call(handle, gwHwndNext)
	return below
}

// isOrdinaryWindow reports whether a window is one a person would call a
// window and is not kept above the others, which is what may be stacked
// beneath.
func isOrdinaryWindow(handle uintptr) bool {
	if !isCandidate(handle) {
		return false
	}
	style, _, _ := pGetWindowLongPtr.Call(handle, gwlExStyle)
	return style&wsExTopmost == 0
}
