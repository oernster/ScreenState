package win32

import "github.com/oernster/ScreenState/internal/product"

// takesOver decides whether one key press or button press is the user taking
// over the desktop, which ends a restore's wait for anything that is not coming
// (FR-079).
//
// A key press always is. A button press is too, except on a splash: FR-078
// makes a click on a splash close the splash while the restore carries on.
// Counting it as well gave up every entry still awaited and skipped the wait
// for the taskbar buttons to stop flashing, so dismissing the message cost the
// restore it was announcing.
func takesOver(key, button bool, classUnderPointer string) bool {
	if key {
		return true
	}
	return button && classUnderPointer != product.SplashClass
}
