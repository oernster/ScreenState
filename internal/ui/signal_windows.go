//go:build windows

package ui

import (
	"unsafe"

	"github.com/oernster/ScreenState/internal/infrastructure/window"
	"github.com/oernster/ScreenState/internal/product"
)

// showManagerMessage is the name of the message a second launch posts to the
// copy already running (FR-054). Registering a message by name is the
// documented way for two programs to agree on one without either inventing a
// number that something else may already be using (EIR-004).
var showManagerMessage = product.Name + ".ShowManager"

// registerShowManager returns the message identifier, registering it on first
// use. Windows answers the same identifier to every caller that registers the
// same name, which is what lets the two processes agree without sharing
// anything else.
func registerShowManager() uint32 {
	registered, _, _ := pRegisterWindowMsg.Call(
		uintptr(unsafe.Pointer(wide(showManagerMessage))))
	return uint32(registered)
}

// ShowRunningManager asks the copy already running to present its manager,
// reporting whether there was one to ask.
//
// The window is found by its class, which is this product's own and is built
// from the product's name rather than written down. A launch that finds no
// window answers false rather than waiting: the mutex says a copy holds the
// lock, so a copy that has no window yet is one still starting; a second launch
// is not worth blocking over.
func ShowRunningManager() bool {
	hwnd, _, _ := pFindWindow.Call(
		uintptr(unsafe.Pointer(wide(className))), 0)
	if hwnd == 0 {
		return false
	}
	posted, _, _ := pPostMessage.Call(hwnd, uintptr(registerShowManager()), 0, 0)
	return posted != 0
}

// Complain says something to a user who has no window to be told in.
//
// A launch that ends before any window opens has nowhere to put a message: this
// program is built without a console, so everything written to a standard
// output goes nowhere at all and the user is left with a program that did not
// start and nothing to read. Measured on 2026-09-20: a copy that had lost its
// tray went on holding the lock; every later start then ended in silence.
func Complain(words string) {
	const iconWarning = 0x00000030
	_, _, _ = pMessageBox.Call(0,
		uintptr(unsafe.Pointer(wide(words))),
		uintptr(unsafe.Pointer(wide(product.Name))),
		iconWarning)
}

// TakeWindowFocus gives the manager's webview the keyboard, reporting whether
// it could. It is here rather than called directly, so the one file that knows
// both the application layer and the Windows layer stays the composition root.
func TakeWindowFocus() bool { return window.TakeFocus() }

// AllowWindowClose greys the manager's close command or restores it, reporting
// whether it could. It is here for the same reason as TakeWindowFocus.
func AllowWindowClose(allowed bool) bool { return window.AllowClose(allowed) }
