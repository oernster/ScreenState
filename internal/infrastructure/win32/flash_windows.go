//go:build windows

package win32

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/oernster/ScreenState/internal/application"
	"golang.org/x/sys/windows"
)

// Hearing a taskbar button flash and knowing how long a flash series lasts
// (FR-080).
//
// The hidden window each watch already keeps for the displays is also
// registered for the shell's hook messages, which name the window whose button
// flashes. The series length is read from Windows each time it is asked, since
// both of its parts are the user's settings.

var (
	pRegisterWindowMessage     = user32.NewProc("RegisterWindowMessageW")
	pRegisterShellHookWindow   = user32.NewProc("RegisterShellHookWindow")
	pDeregisterShellHookWindow = user32.NewProc("DeregisterShellHookWindow")
	pSystemParametersInfo      = user32.NewProc("SystemParametersInfoW")
	pGetCaretBlinkTime         = user32.NewProc("GetCaretBlinkTime")
	shellMessageOnce           sync.Once
	shellMessage               uintptr
)

// The shell hook, the settings the series is read from and the stand-in blink.
const (
	shellHookMessageName = "SHELLHOOK"
	hshellFlash          = 0x8006 // HSHELL_FLASH
	spiGetFlashCount     = 0x2004 // SPI_GETFOREGROUNDFLASHCOUNT
	caretNeverBlinks     = 0xFFFFFFFF
	// defaultCaretBlink is Windows' own default blink time, which stands in
	// where the caret does not blink or its time cannot be read (owner's
	// ruling, 2026-09-21).
	defaultCaretBlink = 530 * time.Millisecond
	// blinksPerFlash is one flash on and one off, each a blink long: measured
	// on the reference machine as flashes 1.060 seconds apart against a blink
	// of 530 milliseconds.
	blinksPerFlash = 2
)

// shellHookMessage answers the message number the shell's hook messages come
// under; zero where Windows would not give one.
func shellHookMessage() uintptr {
	shellMessageOnce.Do(func() {
		name, _ := windows.UTF16PtrFromString(shellHookMessageName)
		shellMessage, _, _ = pRegisterWindowMessage.Call(uintptr(unsafe.Pointer(name)))
	})
	return shellMessage
}

// hearTheShell registers a watch's hidden window for the shell's hook
// messages.
func hearTheShell(window uintptr) error {
	if shellHookMessage() == 0 {
		return fmt.Errorf("the shell's messages could not be named")
	}
	if ok, _, err := pRegisterShellHookWindow.Call(window); ok == 0 {
		return fmt.Errorf("listening to the shell: %w", err)
	}
	return nil
}

// stopHearingTheShell undoes hearTheShell.
func stopHearingTheShell(window uintptr) {
	_, _, _ = pDeregisterShellHookWindow.Call(window)
}

// onShellMessage tells every watch of a flash; it reports whether the message
// was the shell's.
func onShellMessage(message, code, window uintptr) bool {
	if message == 0 || message != shellHookMessage() {
		return false
	}
	if code == hshellFlash {
		eachWatch(func(watch *desktopWatch) { watch.flashedNow(application.WindowID(window)) })
	}
	return true
}

// flashedNow records a window flashing and wakes the watch.
func (watch *desktopWatch) flashedNow(window application.WindowID) {
	watch.flashLock.Lock()
	watch.flashed = append(watch.flashed, window)
	watch.flashLock.Unlock()
	select {
	case watch.flash <- struct{}{}:
	default:
	}
}

// Flashing answers the windows reported flashing since last asked.
func (watch *desktopWatch) Flashing() []application.WindowID {
	watch.flashLock.Lock()
	defer watch.flashLock.Unlock()
	flashed := watch.flashed
	watch.flashed = nil
	return flashed
}

// FlashSeries answers how long one flash series lasts on this machine: the
// foreground flash count times a flash, which is two caret blinks.
func (*Events) FlashSeries() time.Duration {
	var count uint32
	if ok, _, _ := pSystemParametersInfo.Call(spiGetFlashCount, 0,
		uintptr(unsafe.Pointer(&count)), 0); ok == 0 {
		return 0
	}
	blink, _, _ := pGetCaretBlinkTime.Call()
	return flashSeries(count, uint32(blink))
}

// flashSeries works the series out from the count and the blink time in
// milliseconds, with the default blink where the caret does not blink or its
// time is unknown.
func flashSeries(count, blinkMillis uint32) time.Duration {
	blink := time.Duration(blinkMillis) * time.Millisecond
	if blinkMillis == 0 || blinkMillis == caretNeverBlinks {
		blink = defaultCaretBlink
	}
	return time.Duration(count) * blinksPerFlash * blink
}
