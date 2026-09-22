//go:build windows

package window

import "golang.org/x/sys/windows"

// Keeping the main window open while a dialog drawn inside its page is up.
//
// A dialog in the page is a backdrop over the page's content and nothing more:
// the caption the window's frame draws is Windows', not the page's, so its
// close button went on working over an open dialog and hid the window with the
// dialog still up in it. A true modal dialog disables the window that owns it,
// which is not open to this one: the dialog lives inside that window's own
// webview and would be disabled with it. Greying the window menu's Close shows
// the cross as unavailable. It is presentation only: measured 2026-09-22, a
// greyed Close still reaches the window as WM_CLOSE when the command is sent
// (TestAGreyedCloseIsNotAGuard), so the refusal itself is made where the close
// arrives, in the agent's beforeClose.

var (
	procGetSystemMenu  = moduser32.NewProc("GetSystemMenu")
	procEnableMenuItem = moduser32.NewProc("EnableMenuItem")
)

// The window menu's Close and the states EnableMenuItem can give it.
const (
	scClose      = 0xF060 // SC_CLOSE
	mfByCommand  = 0x0000 // MF_BYCOMMAND: the item is named by its command
	mfEnabled    = 0x0000 // MF_ENABLED
	mfGrayed     = 0x0001 // MF_GRAYED
	keepTheMenu  = 0      // GetSystemMenu's bRevert: FALSE answers the window's own menu
	noSuchItem   = 0xFFFFFFFF
	menuItemMask = 0xFFFFFFFF
)

// AllowClose greys the close command of this process's main window or restores
// it, reporting whether it could.
func AllowClose(allowed bool) bool {
	main := mainWindow()
	if main == 0 {
		return false
	}
	return allowClose(main, allowed)
}

// allowClose greys or restores one window's close command.
func allowClose(handle windows.HWND, allowed bool) bool {
	menu, _, _ := procGetSystemMenu.Call(uintptr(handle), keepTheMenu)
	if menu == 0 {
		return false
	}
	state := uintptr(mfByCommand | mfGrayed)
	if allowed {
		state = mfByCommand | mfEnabled
	}
	previous, _, _ := procEnableMenuItem.Call(menu, scClose, state)
	return previous&menuItemMask != noSuchItem
}
