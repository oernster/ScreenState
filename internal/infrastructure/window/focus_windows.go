//go:build windows

// Giving the setup page the keyboard, which on Windows nothing else reliably
// does.
//
// Wails hands the webview its keyboard from one place only: the main window's
// WM_SETFOCUS, which Windows raises solely on a change of focus. That handler is
// bound inside the asynchronous WebView2 controller callback, so whether the
// window's first focus arrives before or after there is a handler for it is a
// race. Losing it leaves the page with no keyboard at all and no way to ask for
// one: the button the page focused draws no ring and Enter does nothing, which
// reads as a dead keyboard rather than as focus sitting elsewhere.
//
// A mouse click fixes it, because a click focuses the WebView2 child window
// directly. This does the same thing without the click, which is the only route
// that does not depend on the race.
//
// The package names no product and knows nothing of the agent: the window is
// found by enumerating this process's own, so whichever binary calls it
// resolves its own window.

package window

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	moduser32                   = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows             = moduser32.NewProc("EnumWindows")
	procGetWindowThreadProcess  = moduser32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible         = moduser32.NewProc("IsWindowVisible")
	procGetWindow               = moduser32.NewProc("GetWindow")
	procAttachThreadInput       = moduser32.NewProc("AttachThreadInput")
	procSetFocus                = moduser32.NewProc("SetFocus")
	procSetForegroundWindowCall = moduser32.NewProc("SetForegroundWindow")
)

// gwChild asks GetWindow for a window's first child in z-order, which for the
// Wails window is the WebView2 host.
const gwChild = 5

// keepEnumerating and stopEnumerating are what the enumeration callback answers
// Windows: carry on; or this is the one.
const (
	keepEnumerating = 1
	stopEnumerating = 0
)

// attach and detach are the flag AttachThreadInput takes, named because a bare
// 1 and 0 at a call site say nothing about which direction they mean.
const (
	attach = 1
	detach = 0
)

// found carries one enumeration's result out of the callback.
var found struct {
	mu     sync.Mutex
	target windows.HWND
	pid    uint32
}

// onWindow is registered once, because every NewCallback consumes a slot from a
// process-wide table that is never reclaimed.
var onWindow = windows.NewCallback(func(handle uintptr, _ uintptr) uintptr {
	var pid uint32
	_, _, _ = procGetWindowThreadProcess.Call(handle, uintptr(unsafe.Pointer(&pid)))
	if pid != found.pid {
		return keepEnumerating
	}
	if visible, _, _ := procIsWindowVisible.Call(handle); visible == 0 {
		return keepEnumerating
	}
	if child, _, _ := procGetWindow.Call(handle, gwChild); child == 0 {
		return keepEnumerating
	}
	found.target = windows.HWND(handle)
	return stopEnumerating
})

// mainWindow returns this process's visible top-level window that owns a child.
//
// It is found by enumeration rather than by title, so a retitled window still
// resolves and another application wearing the same name never does.
func mainWindow() windows.HWND {
	found.mu.Lock()
	defer found.mu.Unlock()
	found.target = 0
	found.pid = windows.GetCurrentProcessId()
	_, _, _ = procEnumWindows.Call(onWindow, 0)
	return found.target
}

// TakeFocus gives the WebView2 child the keyboard, reporting whether it could.
//
// SetFocus only acts within the calling thread's input queue. This runs on a
// goroutine that is not the window's thread, so the two queues are attached for
// the duration. That is the documented way to focus a window owned by another
// thread.
func TakeFocus() bool {
	main := mainWindow()
	if main == 0 {
		return false
	}
	child, _, _ := procGetWindow.Call(uintptr(main), gwChild)
	if child == 0 {
		return false
	}

	var pid uint32
	windowThread, _, _ := procGetWindowThreadProcess.Call(uintptr(main), uintptr(unsafe.Pointer(&pid)))
	if windowThread == 0 {
		return false
	}

	current := uintptr(windows.GetCurrentThreadId())
	attached := false
	if windowThread != current {
		ret, _, _ := procAttachThreadInput.Call(current, windowThread, attach)
		attached = ret != 0
	}

	_, _, _ = procSetForegroundWindowCall.Call(uintptr(main))
	focused, _, _ := procSetFocus.Call(child)

	if attached {
		_, _, _ = procAttachThreadInput.Call(current, windowThread, detach)
	}
	return focused != 0
}
