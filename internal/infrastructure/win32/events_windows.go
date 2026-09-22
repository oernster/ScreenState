//go:build windows

package win32

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/product"
	"golang.org/x/sys/windows"
)

// Telling a restore when the desktop changed and when the user took it over
// (FR-079).
//
// A watch runs a thread of its own with a message loop, since Windows delivers
// both kinds of hook to the thread that set them. Window events say a window
// was created, shown, hidden, cloaked, uncloaked, destroyed or moved; this
// process's own windows are skipped, so the splash never wakes a restore. The
// low-level keyboard and mouse hooks say the user pressed a key or a button.
// Raw input would say so too. A process has one raw input target per
// device and the splash already holds it. A hidden window hears the displays
// change; registered with the shell, it also hears a taskbar button flash, be
// added or be taken away.

var (
	pSetWinEventHook       = user32.NewProc("SetWinEventHook")
	pUnhookWinEvent        = user32.NewProc("UnhookWinEvent")
	pSetWindowsHookEx      = user32.NewProc("SetWindowsHookExW")
	pUnhookWindowsHookEx   = user32.NewProc("UnhookWindowsHookEx")
	pCallNextHookEx        = user32.NewProc("CallNextHookEx")
	pGetAncestor           = user32.NewProc("GetAncestor")
	pGetMessage            = user32.NewProc("GetMessageW")
	pDispatchMessage       = user32.NewProc("DispatchMessageW")
	pPostThreadMessage     = user32.NewProc("PostThreadMessageW")
	pRegisterClassEx       = user32.NewProc("RegisterClassExW")
	pCreateWindowEx        = user32.NewProc("CreateWindowExW")
	pDestroyWindow         = user32.NewProc("DestroyWindow")
	pDefWindowProc         = user32.NewProc("DefWindowProcW")
	pGetCurrentThreadID    = kernel32.NewProc("GetCurrentThreadId")
	pGetModuleHandle       = kernel32.NewProc("GetModuleHandleW")
	registerWatcherClass   sync.Once
	watcherClassRegistered bool
	watcherClassName, _    = windows.UTF16PtrFromString(product.Name + "DesktopWatch")
	eventCallback          = windows.NewCallback(onWindowEvent)
	inputCallback          = windows.NewCallback(onInput)
	watcherWindowCallback  = windows.NewCallback(onWatcherMessage)
	activeWatchesLock      sync.Mutex
	activeWatches          = map[*desktopWatch]struct{}{}
)

// The events and messages a watch listens for.
const (
	eventObjectCreate    = 0x8000 // EVENT_OBJECT_CREATE, first of create, destroy, show, hide
	eventObjectHide      = 0x8003
	eventLocationChange  = 0x800B
	eventObjectCloaked   = 0x8017
	eventObjectUncloaked = 0x8018
	outOfContext         = 0x0000 // WINEVENT_OUTOFCONTEXT
	skipOwnProcess       = 0x0002 // WINEVENT_SKIPOWNPROCESS
	objectWindow         = 0      // OBJID_WINDOW
	childSelf            = 0      // CHILDID_SELF
	ancestorRoot         = 2      // GA_ROOT

	keyboardHook = 13 // WH_KEYBOARD_LL
	mouseHook    = 14 // WH_MOUSE_LL

	wmQuit          = 0x0012
	wmDisplayChange = 0x007E
	wmKeyDown       = 0x0100
	wmSysKeyDown    = 0x0104
	wmRButtonDown   = 0x0204
	wmMButtonDown   = 0x0207
	wmXButtonDown   = 0x020B
)

// Events begins watches of the desktop.
type Events struct{}

// NewEvents returns the desktop's events.
func NewEvents() *Events { return &Events{} }

// desktopWatch is one watch: a change is signalled on a channel holding one,
// so a burst of events costs one more look rather than one each.
type desktopWatch struct {
	changed       chan struct{}
	touched       chan struct{}
	touchOnce     sync.Once
	touchReported bool
	// flash is signalled like changed; flashed names the windows (FR-080).
	flash     chan struct{}
	flashLock sync.Mutex
	flashed   []application.WindowID
}

// Watch begins a watch on a thread of its own and answers once its hooks are
// set; otherwise it answers the reason they could not be.
func (*Events) Watch(ctx context.Context) (application.DesktopWatch, error) {
	watch := &desktopWatch{changed: make(chan struct{}, 1), touched: make(chan struct{}),
		flash: make(chan struct{}, 1)}
	ready := make(chan error, 1)
	go watch.run(ctx, ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return watch, nil
}

// Next waits for whichever comes first: a change, a flash, the user's first
// key or click, the deadline.
func (watch *desktopWatch) Next(ctx context.Context, deadline time.Time) (application.Wake, error) {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return application.WakeDeadline, nil
	}
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	touched := watch.touched
	if watch.touchReported {
		touched = nil
	}
	select {
	case <-ctx.Done():
		return application.WakeDeadline, ctx.Err()
	case <-touched:
		watch.touchReported = true
		return application.WakeTouched, nil
	case <-watch.changed:
		return application.WakeChanged, nil
	case <-watch.flash:
		return application.WakeFlashed, nil
	case <-timer.C:
		return application.WakeDeadline, nil
	}
}

// Touched answers, without waiting, whether the user has pressed a key or
// clicked since the watch began (FR-087). The channel is closed once, at the
// first key press or click; it stays closed.
func (watch *desktopWatch) Touched() bool {
	select {
	case <-watch.touched:
		return true
	default:
		return false
	}
}

// run sets the hooks, carries messages until the context ends, then takes the
// hooks down. A panic here must not end the agent; it ends the watch and the
// restore carries on to its ceiling.
func (watch *desktopWatch) run(ctx context.Context, ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	answered := false
	defer func() {
		if recovered := recover(); recovered != nil && !answered {
			ready <- fmt.Errorf("watching the desktop failed: %v", recovered)
		}
	}()

	hooks, window, err := setHooks()
	if err != nil {
		ready <- err
		answered = true
		return
	}
	defer takeDown(hooks, window)
	activeWatchesLock.Lock()
	activeWatches[watch] = struct{}{}
	activeWatchesLock.Unlock()
	defer func() {
		activeWatchesLock.Lock()
		delete(activeWatches, watch)
		activeWatchesLock.Unlock()
	}()

	thread, _, _ := pGetCurrentThreadID.Call()
	ready <- nil
	answered = true
	go func() {
		<-ctx.Done()
		_, _, _ = pPostThreadMessage.Call(thread, wmQuit, 0, 0)
	}()
	var msg [48]byte // MSG, larger than it needs to be on either build
	for {
		got, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&msg[0])), 0, 0, 0)
		if int32(got) <= 0 {
			return
		}
		_, _, _ = pDispatchMessage.Call(uintptr(unsafe.Pointer(&msg[0])))
	}
}

// hookSet is every hook one watch set.
type hookSet struct {
	events []uintptr
	input  []uintptr
}

// setHooks sets the window event hooks, the input hooks and the hidden window
// that hears the displays change. Anything set before a failure is taken down.
func setHooks() (hookSet, uintptr, error) {
	var hooks hookSet
	for _, span := range [][2]uintptr{
		{eventObjectCreate, eventObjectHide},
		{eventLocationChange, eventLocationChange},
		{eventObjectCloaked, eventObjectUncloaked},
	} {
		hook, _, err := pSetWinEventHook.Call(span[0], span[1], 0, eventCallback, 0, 0,
			outOfContext|skipOwnProcess)
		if hook == 0 {
			takeDown(hooks, 0)
			return hookSet{}, 0, fmt.Errorf("hooking the window events: %w", err)
		}
		hooks.events = append(hooks.events, hook)
	}
	instance, _, _ := pGetModuleHandle.Call(0)
	for _, kind := range []uintptr{keyboardHook, mouseHook} {
		hook, _, err := pSetWindowsHookEx.Call(kind, inputCallback, instance, 0)
		if hook == 0 {
			takeDown(hooks, 0)
			return hookSet{}, 0, fmt.Errorf("hooking the keyboard and mouse: %w", err)
		}
		hooks.input = append(hooks.input, hook)
	}
	window, err := watcherWindow(instance)
	if err != nil {
		takeDown(hooks, 0)
		return hookSet{}, 0, err
	}
	if err := hearTheShell(window); err != nil {
		takeDown(hooks, window)
		return hookSet{}, 0, err
	}
	return hooks, window, nil
}

// watcherWindow makes the hidden window that hears the displays change. It is a
// top-level window rather than a message-only one, which is not sent the
// broadcast.
func watcherWindow(instance uintptr) (uintptr, error) {
	registerWatcherClass.Do(func() {
		class := windowClassEx{procedure: watcherWindowCallback, instance: instance,
			className: watcherClassName}
		class.size = uint32(unsafe.Sizeof(class))
		atom, _, _ := pRegisterClassEx.Call(uintptr(unsafe.Pointer(&class)))
		watcherClassRegistered = atom != 0
	})
	if !watcherClassRegistered {
		return 0, fmt.Errorf("the display watch could not be registered")
	}
	window, _, err := pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(watcherClassName)), 0, 0,
		0, 0, 0, 0, 0, 0, instance, 0)
	if window == 0 {
		return 0, fmt.Errorf("making the display watch: %w", err)
	}
	return window, nil
}

// takeDown removes every hook and the hidden window.
func takeDown(hooks hookSet, window uintptr) {
	for _, hook := range hooks.events {
		_, _, _ = pUnhookWinEvent.Call(hook)
	}
	for _, hook := range hooks.input {
		_, _, _ = pUnhookWindowsHookEx.Call(hook)
	}
	if window != 0 {
		stopHearingTheShell(window)
		_, _, _ = pDestroyWindow.Call(window)
	}
}

// eachWatch runs something for every watch now running.
func eachWatch(do func(*desktopWatch)) {
	activeWatchesLock.Lock()
	defer activeWatchesLock.Unlock()
	for watch := range activeWatches {
		do(watch)
	}
}

// changedNow tells every watch the desktop changed.
func changedNow() {
	eachWatch(func(watch *desktopWatch) {
		select {
		case watch.changed <- struct{}{}:
		default:
		}
	})
}

// onWindowEvent hears a window event. Only a top-level window itself counts: a
// caret, a cursor or a control inside a window moving is not the desktop
// changing.
func onWindowEvent(_, _, window, object, child, _, _ uintptr) uintptr {
	if object != objectWindow || child != childSelf || window == 0 {
		return 0
	}
	if root, _, _ := pGetAncestor.Call(window, ancestorRoot); root != window {
		return 0
	}
	changedNow()
	return 0
}

// onInput hears every key and button. It always hands it on: this watches
// input, it never takes it.
func onInput(code, message, detail uintptr) uintptr {
	if int32(code) >= 0 {
		key := message == wmKeyDown || message == wmSysKeyDown
		button := message == wmLButtonDown || message == wmRButtonDown ||
			message == wmMButtonDown || message == wmXButtonDown
		// The window under the pointer is read only for a press, never for the
		// movement that makes up almost everything this hook hears.
		if (key || button) && takesOver(key, button, pressedOn(button, detail)) {
			eachWatch(func(watch *desktopWatch) {
				watch.touchOnce.Do(func() { close(watch.touched) })
			})
		}
	}
	next, _, _ := pCallNextHookEx.Call(0, code, message, detail)
	return next
}

// onWatcherMessage hears the displays change and the shell report a taskbar
// button flashing, added or taken away.
func onWatcherMessage(window, message, wParam, lParam uintptr) uintptr {
	if onShellMessage(message, wParam, lParam) {
		return 0
	}
	if message == wmDisplayChange {
		changedNow()
		return 0
	}
	result, _, _ := pDefWindowProc.Call(window, message, wParam, lParam)
	return result
}

// windowClassEx is WNDCLASSEXW.
type windowClassEx struct {
	size       uint32
	style      uint32
	procedure  uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	smallIcon  uintptr
}
