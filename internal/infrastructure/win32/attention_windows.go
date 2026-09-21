//go:build windows

package win32

// Clearing the red a finished flash series leaves on a taskbar button (FR-075).
//
// Measured on the reference machine on 2026-09-21: an application started
// without the front asks for it anyway; Windows refuses and flashes its button
// in a series that ends with the button left red. Rebuilding the button does
// not clear that, nor does FlashWindowEx with FLASHW_STOP: both were tried on
// Stellody and it stayed red. What the shell did for Claude, whose red did
// clear, was follow its rebuilt button with the shell hook notice "window
// activated". Posting that notice for Stellody to the taskbars cleared it,
// activated nothing and left the underline on the window that really had the
// front; a click on Stellody's button then brought it forward rather than
// minimising it. So the taskbars are told each rebuilt window was activated,
// then told the truth: the window that really has the front was.
//
// Which of the taskbar's windows acts on the notice is not known, so it goes to
// every taskbar and each window inside one, as measured.

var (
	pEnumChildWindows    = user32.NewProc("EnumChildWindows")
	pGetForegroundWindow = user32.NewProc("GetForegroundWindow")
)

// hshellWindowActivated is HSHELL_WINDOWACTIVATED.
const hshellWindowActivated = 4

// clearAttention tells the taskbars the window was activated, then tells them
// the window that has the front was. Nothing is activated.
func clearAttention(window uintptr) {
	message := shellHookMessage()
	if message == 0 {
		return
	}
	receivers := taskbarWindows()
	tell := func(activated uintptr) {
		for _, receiver := range receivers {
			// Whether a taskbar acted on it cannot be read back; a refusal leaves
			// the button as it was, which is where the rebuild left it.
			_, _, _ = pPostMessage.Call(receiver, message, hshellWindowActivated, activated)
		}
	}
	tell(window)
	if front, _, _ := pGetForegroundWindow.Call(); front != 0 && front != window {
		tell(front)
	}
}

// taskbarWindows answers every taskbar and every window inside one.
func taskbarWindows() []uintptr {
	var found []uintptr
	for _, window := range handles() {
		if isTaskbar(window) {
			found = append(found, window)
			found = append(found, childHandles(window)...)
		}
	}
	return found
}
