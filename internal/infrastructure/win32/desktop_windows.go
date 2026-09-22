//go:build windows

package win32

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
	"golang.org/x/sys/windows"
)

// Desktop reads and moves the real windows of the signed-in user.
//
// It holds one piece of state between calls, the moment each window was first
// seen, because Windows does not record when a window was created and FR-037
// matches placements to windows in first-seen order. See firstSeen for what
// that costs.
type Desktop struct {
	clock application.Clock

	mutex sync.Mutex
	seen  map[uintptr]time.Time
	order map[uintptr]int
	next  int
}

// NewDesktop returns a desktop reader over the given clock. It also tells
// Windows that this process understands mixed display scaling, which has to be
// said before any window is read: the reference machine mixes 96 and 240 dpi,
// and a process that has not said so is handed invented coordinates.
func NewDesktop(clock application.Clock) *Desktop {
	_, _, _ = pSetProcessDpiAwareness.Call(dpiPerMonitorV2)
	return &Desktop{
		clock: clock,
		seen:  make(map[uintptr]time.Time),
		order: make(map[uintptr]int),
	}
}

// enumeration collects windows during a single EnumWindows call. Windows allows
// a process only so many callbacks, so the callback is made once and the lock
// makes one enumeration at a time safe.
var (
	enumerationLock sync.Mutex
	enumerated      []uintptr
	collect         = windows.NewCallback(func(handle, _ uintptr) uintptr {
		enumerated = append(enumerated, handle)
		return 1 // carry on
	})
)

// handles returns every top-level window, in the order Windows enumerates them,
// which is front to back in the stacking order.
func handles() []uintptr {
	return gather(func() { _, _, _ = pEnumWindows.Call(collect, 0) })
}

// childHandles returns every window inside a window.
func childHandles(parent uintptr) []uintptr {
	return gather(func() { _, _, _ = pEnumChildWindows.Call(parent, collect, 0) })
}

// gather runs an enumeration that reports through collect and answers what it
// found. One runs at a time, since collect writes to one list.
func gather(enumerate func()) []uintptr {
	enumerationLock.Lock()
	defer enumerationLock.Unlock()
	enumerated = nil
	enumerate()
	found := make([]uintptr, len(enumerated))
	copy(found, enumerated)
	return found
}

// Windows returns every window a user would call a window.
//
// The candidate rule is FR-012 as measured on 2026-09-19: visible, not cloaked,
// not owned by another window, not a tool window and not untitled. On the
// reference machine that reduced 523 enumerated windows to the 8 a person would
// point at.
func (desktop *Desktop) Windows(ctx context.Context) ([]application.Window, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	found := handles()
	out := make([]application.Window, 0, len(found))
	// Enumeration runs front to back, so walking it backwards puts the windows
	// that have been behind the others for longest first. It is a stacking
	// order rather than an age, which is why firstSeen exists.
	for at := len(found) - 1; at >= 0; at-- {
		handle := found[at]
		if !isCandidate(handle) {
			continue
		}
		window, err := desktop.describe(handle)
		if err != nil {
			out = append(out, application.Window{
				ID:          application.WindowID(handle),
				Description: titleOf(handle),
				Unreadable:  err.Error(),
				Created:     desktop.firstSeen(handle),
			})
			continue
		}
		out = append(out, window)
	}
	return out, nil
}

// Window re-reads one window, for the settle check FR-033 makes.
func (desktop *Desktop) Window(ctx context.Context, id application.WindowID) (application.Window, error) {
	if err := ctx.Err(); err != nil {
		return application.Window{}, err
	}
	handle := uintptr(id)
	if alive, _, _ := pIsWindow.Call(handle); alive == 0 {
		return application.Window{}, application.ErrWindowGone
	}
	return desktop.describe(handle)
}

// describe reads one window into the shape the application layer works in.
func (desktop *Desktop) describe(handle uintptr) (application.Window, error) {
	placement := windowPlacement{}
	placement.length = uint32(unsafe.Sizeof(placement))
	ok, _, err := pGetWindowPlacement.Call(handle, uintptr(unsafe.Pointer(&placement)))
	if ok == 0 {
		return application.Window{}, fmt.Errorf("its position could not be read: %w", err)
	}
	identity, err := desktop.identityOf(handle)
	if err != nil {
		return application.Window{}, err
	}
	visible, _, _ := pIsWindowVisible.Call(handle)
	return application.Window{
		ID:          application.WindowID(handle),
		Application: identity,
		Rect:        fromRect(placement.rcNormalPosition),
		State:       stateFrom(placement.showCmd),
		Visible:     visible != 0 && !isCloaked(handle),
		Created:     desktop.firstSeen(handle),
		Description: titleOf(handle),
	}, nil
}

// firstSeen answers when this agent first saw a window, which is the order
// FR-037 asks for.
//
// Windows records no creation time for a window, so an age cannot be read from
// the system at all; that is why the requirement asks for first-seen order
// rather than creation order. During a restore the two agree, because the agent
// is watching while the windows appear: a window seen on a later pass is
// genuinely newer than one seen on an earlier one. Windows already open when
// the agent starts all share one moment; within that moment they keep the order
// the first enumeration gave them, which is a stacking order rather than an
// age. That distinction is a known limit and is recorded as such rather than
// being presented as a measurement.
func (desktop *Desktop) firstSeen(handle uintptr) time.Time {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	if when, known := desktop.seen[handle]; known {
		return when
	}
	when := desktop.clock.Now().Add(time.Duration(desktop.next))
	desktop.seen[handle] = when
	desktop.order[handle] = desktop.next
	desktop.next++
	return when
}

// Displays returns every connected display, named by what survives a reboot.
func (desktop *Desktop) Displays(ctx context.Context) ([]application.Display, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	monitors, err := monitorHandles()
	if err != nil {
		return nil, err
	}
	out := make([]application.Display, 0, len(monitors))
	for _, monitor := range monitors {
		display, err := describeMonitor(monitor)
		if err != nil {
			// A display with no usable identity is left out rather than named
			// by a number: the numbers were measured disagreeing with each
			// other and with where the screens physically are.
			continue
		}
		out = append(out, display)
	}
	if len(out) == 0 {
		return nil, ErrNoMonitorID
	}
	return out, nil
}

// Place puts a window where a placement says, in the order measured on
// 2026-09-19 and recorded as the answer to OQ-3: restore it, set the rectangle,
// then set the show state.
//
// The order is not a preference. Maximising acts on whichever display the
// window's rectangle is on, so maximising first would maximise it where it
// already was; and a maximised window ignores a move until it has been restored.
func (desktop *Desktop) Place(
	ctx context.Context,
	id application.WindowID,
	target domain.Rect,
	state domain.ShowState,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	handle := uintptr(id)
	if alive, _, _ := pIsWindow.Call(handle); alive == 0 {
		return application.ErrWindowGone
	}
	// Placing a window must leave it where it was among the others: maximising
	// without activating brings it to the top (see keepStackingPlace).
	restack := keepStackingPlace(handle)
	defer restack()
	// ShowWindow answers whether the window was previously visible rather than
	// whether it worked, so there is nothing here to read as success.
	//
	// SW_SHOWNOACTIVATE rather than SW_RESTORE, because restoring activates the
	// window and a taskbar marks the window last activated on its display
	// (FR-074).
	_, _, _ = pShowWindow.Call(handle, swShowNoActivate)
	moved, _, err := pSetWindowPos.Call(handle, 0,
		uintptr(target.X), uintptr(target.Y),
		uintptr(target.Width), uintptr(target.Height),
		swpNoZOrder|swpNoActivate)
	if moved == 0 {
		return fmt.Errorf("moving the window: %w", err)
	}
	switch state {
	case domain.ShowMaximised:
		maximiseWithoutActivating(handle)
	case domain.ShowMinimised:
		// Minimised without activating, so a restore does not pull the
		// keyboard away from whatever the user is already typing into.
		_, _, _ = pShowWindow.Call(handle, swMinNoActive)
	case domain.ShowNormal:
		// The window is already restored, which is what normal means.
	}
	return nil
}

// maximiseWithoutActivating maximises a window without making it the active
// one (FR-074).
//
// A taskbar marks the window last activated on its display, so a restore that
// activated what it maximised left one lit button per display. Every direct way
// to maximise activates: ShowWindow with SW_MAXIMIZE, WM_SYSCOMMAND with
// SC_MAXIMIZE and SetWindowPlacement with SW_MAXIMIZE, visible or hidden, with
// the asynchronous flag or without. An earlier comment here claimed the last one
// did not; a probe on 2026-09-21 counted the activation (see
// TestMaximisingDoesNotActivate). What does not activate is two steps: the
// placement is set to minimised with the flag that makes its next restore
// maximise it, then the window is shown with SW_SHOWNOACTIVATE, which restores
// it. It ends maximised, on the display its normal rectangle is on, with the
// active window where it was. The cost is the minimise and restore animation.
//
// The placement is read and written back with only the show state and that
// flag changed, so the rectangle it carries is whatever Windows itself last
// reported. That matters because a placement's rectangle is in workspace
// coordinates, which are not always the screen coordinates the rest of this
// file works in; reading before writing keeps this out of that difference.
func maximiseWithoutActivating(handle uintptr) {
	placement := windowPlacement{}
	placement.length = uint32(unsafe.Sizeof(placement))
	if read, _, _ := pGetWindowPlacement.Call(handle,
		uintptr(unsafe.Pointer(&placement))); read == 0 {
		return
	}
	placement.showCmd = swMinNoActive
	placement.flags |= wpfRestoreToMaximized
	if set, _, _ := pSetWindowPlacement.Call(handle,
		uintptr(unsafe.Pointer(&placement))); set == 0 {
		return
	}
	_, _, _ = pShowWindow.Call(handle, swShowNoActivate)
}

// Close asks a window to close, which is what pressing the cross on its title
// bar does (FR-064).
//
// It is posted rather than sent, so this thread is never held while another
// application decides what to do: an application that puts up a prompt about
// unsaved work would otherwise stop the restore until somebody answered it.
// Success here means the request reached the window's queue and nothing more;
// whether the window goes is read afterwards, by looking for it again.
func (desktop *Desktop) Close(ctx context.Context, id application.WindowID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	handle := uintptr(id)
	if alive, _, _ := pIsWindow.Call(handle); alive == 0 {
		return application.ErrWindowGone
	}
	posted, _, err := pPostMessage.Call(handle, wmClose, 0, 0)
	if posted == 0 {
		return fmt.Errorf("asking the window to close: %w", err)
	}
	return nil
}
