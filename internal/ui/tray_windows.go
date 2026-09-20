//go:build windows

package ui

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/product"
	"golang.org/x/sys/windows"
)

// className names the hidden window's class. It is never shown to anybody.
const className = product.Name + "TrayWindow"

// firstCommand is the identifier the first menu entry takes. Zero is what
// TrackPopupMenu answers when the menu is dismissed, so no entry may use it.
const firstCommand = 1

// Tray is the tray icon, its menu and the loop that carries them.
//
// One tray exists per run and it owns its window, so the package-level state
// the Win32 callback needs has one owner rather than being shared.
type Tray struct {
	service *application.TrayService
	log     application.Log
	// onManager asks for the manager window. The tray never opens a window
	// itself: it runs on its own locked thread and the manager belongs to the
	// thread the webview runs on, so this hands the request across rather than
	// reaching over.
	onManager func(application.ManagerRequest)

	hwnd    uintptr
	icon    uintptr
	added   bool
	taskbar uint32
	// showManager is the message a second launch posts to ask this copy for
	// its manager (FR-054).
	showManager uint32

	mutex   sync.Mutex
	entries []application.MenuItem
	working bool
}

// current is the tray the window procedure belongs to. Win32 hands a callback
// no state of its own; this product has exactly one window, so one package
// variable is the whole of what is needed. It is written before the window
// exists and read only on the thread that owns it.
var current *Tray

// NewTray returns a tray over the given service. onManager is called when the
// user asks for the manager, by clicking the icon or by choosing it from the
// menu; it may be nil for a tray with no window behind it.
func NewTray(
	service *application.TrayService,
	log application.Log,
	onManager func(application.ManagerRequest),
) *Tray {
	return &Tray{service: service, log: log, onManager: onManager}
}

// manager asks for the manager window, saying so in the log where there is
// nothing to ask. A tray whose click does nothing is the failure that is
// hardest to tell from a tray that is broken.
func (tray *Tray) manager(request application.ManagerRequest) {
	if tray.onManager == nil {
		tray.log.Step("the manager was asked for and this run has no window")
		return
	}
	tray.onManager(request)
}

// Run shows the tray icon and carries messages until the user quits.
//
// It locks the goroutine to its thread, because a window belongs to the thread
// that made it: messages are delivered to that thread alone, so a loop that
// wandered to another would simply stop receiving them.
func (tray *Tray) Run(ctx context.Context) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := tray.open(); err != nil {
		return err
	}
	defer tray.close()

	// The shell tells every tray program when Explorer has restarted, which is
	// the moment a tray icon quietly disappears for good if nobody puts it back.
	name := wide("TaskbarCreated")
	registered, _, _ := pRegisterWindowMsg.Call(uintptr(unsafe.Pointer(name)))
	tray.taskbar = uint32(registered)
	tray.showManager = registerShowManager()

	tray.add()
	tray.log.Step("the tray icon is showing")
	tray.pump(ctx)
	return nil
}

// open makes the hidden window that receives the tray's messages.
func (tray *Tray) open() error {
	current = tray
	instance, _, _ := pGetModuleHandle.Call(0)
	cursor, _, _ := pLoadCursor.Call(0, idcArrow)

	class := windowClass{
		style:     0,
		procedure: windows.NewCallback(procedure),
		instance:  instance,
		cursor:    cursor,
		className: wide(className),
	}
	class.size = uint32(unsafe.Sizeof(class))
	if atom, _, err := pRegisterClassEx.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
		return fmt.Errorf("registering the tray window: %w", err)
	}
	hwnd, _, err := pCreateWindowEx.Call(wsExToolWin,
		uintptr(unsafe.Pointer(wide(className))),
		uintptr(unsafe.Pointer(wide(product.Name))),
		wsPopup, cwUseDefault, cwUseDefault, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("making the tray window: %w", err)
	}
	tray.hwnd = hwnd
	return nil
}

// close takes the icon away and destroys the window. A tray icon left behind
// after the program has gone is the litter every user has seen and nobody can
// clear without hovering over it.
func (tray *Tray) close() {
	tray.remove()
	if tray.hwnd != 0 {
		_, _, _ = pDestroyWindow.Call(tray.hwnd)
		tray.hwnd = 0
	}
	current = nil
}

// pump carries messages until the window is destroyed. It also watches the
// context, so a run stopped from outside ends the loop rather than leaving a
// tray icon behind with nothing behind it.
func (tray *Tray) pump(ctx context.Context) {
	go func() {
		<-ctx.Done()
		// Posting rather than calling: the window belongs to the other thread.
		_, _, _ = pPostMessage.Call(tray.hwnd, wmClose, 0, 0)
	}()

	var msg message
	for {
		got, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		// GetMessage answers zero on WM_QUIT and minus one on a failure, both
		// of which end the loop; carrying on past either spins for ever.
		if int32(got) <= 0 {
			return
		}
		_, _, _ = pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = pDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// procedure is the window procedure. It runs on the loop's thread and must
// return quickly: anything that can take time is handed to a goroutine.
func procedure(hwnd, value, wParam, lParam uintptr) uintptr {
	tray := current
	if tray == nil {
		result, _, _ := pDefWindowProc.Call(hwnd, value, wParam, lParam)
		return result
	}
	switch {
	case uint32(value) == tray.taskbar && tray.taskbar != 0:
		// Explorer restarted, so the icon is gone. Put it back.
		tray.added = false
		tray.add()
		return 0
	case uint32(value) == tray.showManager && tray.showManager != 0:
		// A second launch asked for the manager rather than starting a second
		// agent (FR-054).
		tray.manager(application.ManagerProfiles)
		return 0
	case value == wmTray:
		// A left click opens the manager and a right click opens the menu,
		// which is what every other tray program on the machine does. Both were
		// the menu before there was a window to open.
		if lParam == wmLButtonUp {
			tray.manager(application.ManagerProfiles)
			return 0
		}
		if lParam == wmRButtonUp || lParam == wmContextMenu {
			tray.showMenu()
		}
		return 0
	case value == wmCommand:
		tray.chose(uint32(wParam))
		return 0
	case value == wmRefresh:
		tray.refresh()
		return 0
	case value == wmClose:
		_, _, _ = pDestroyWindow.Call(hwnd)
		return 0
	case value == wmDestroy:
		tray.remove()
		_, _, _ = pPostQuitMessage.Call(0)
		return 0
	}
	result, _, _ := pDefWindowProc.Call(hwnd, value, wParam, lParam)
	return result
}
