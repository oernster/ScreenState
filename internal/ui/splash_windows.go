//go:build windows

package ui

import (
	"fmt"
	"image"
	"runtime"
	"sync"
	"unsafe"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/product"
	"golang.org/x/sys/windows"
)

// The splash that says the desktop is being prepared (FR-078).
//
// One window per display, all on a thread of their own with their own message
// loop, since a window belongs to the thread that made it. Each is a topmost
// tool window that never activates: it sits above everything without taking
// the keyboard from the window that holds it (FR-074), a tool window has no
// taskbar button and is not a window any capture, FR-064 or FR-075 acts on.
//
// It closes on events and never on a timer: a click on any splash closes them
// all, at any time; once the restore has ended, so does the user's next key
// press or mouse click anywhere, which raw input delivers to a window that does
// not hold the keyboard.

// splashClass names the splash windows' class, from its one home.
const splashClass = product.SplashClass

// wmSplashChanged asks every splash window to draw what the splash now says.
// WM_APP plus three, clear of the tray's own two.
const wmSplashChanged = 0x8003

// Splash is the desktop-being-prepared splash on every display.
type Splash struct {
	logo   image.Image
	themes Themes
	dark   func() bool
	log    application.Log

	mutex   sync.Mutex
	message application.SplashMessage
	ready   bool
	running bool
	// generation counts the splashes put up, so a splash being taken down
	// cannot tidy away the state of a newer one put up straight after it.
	generation int
	windows    []uintptr
	// listening says raw input has been asked for, which is done once the
	// restore has ended and only then: until then only a click on the splash
	// closes it.
	listening bool
	// shrunk holds the logo reduced to each size a display has needed, since
	// reducing the master is the slow part of drawing it.
	shrunk map[int][]byte
}

// splashing is the splash the window procedure belongs to. Win32 hands a
// callback no state of its own; there is one splash per run.
var splashing *Splash

// registerSplashClass makes the window class once per run.
var registerSplashClass sync.Once

// NewSplash returns a splash drawing the given artwork in the product palette.
// logo may be nil, in which case the words are drawn alone; dark says, when
// the splash goes up, whether the dark theme is the one to wear.
func NewSplash(logo image.Image, themes Themes, dark func() bool, log application.Log) *Splash {
	splash := &Splash{logo: logo, themes: themes, dark: dark, log: log}
	splashing = splash
	return splash
}

// Preparing puts the splash up on every display. One already showing is
// turned back to the waiting words.
func (splash *Splash) Preparing(message application.SplashMessage) {
	splash.mutex.Lock()
	splash.message = message
	splash.ready = false
	start := !splash.running
	splash.running = true
	if start {
		splash.generation++
	}
	generation := splash.generation
	windows := append([]uintptr(nil), splash.windows...)
	splash.mutex.Unlock()
	if start {
		go splash.run(generation)
		return
	}
	changed(windows)
}

// Ready turns the splash to say the restore has ended. A splash the user has
// already clicked away stays away: they asked for it gone.
func (splash *Splash) Ready(message application.SplashMessage) {
	splash.mutex.Lock()
	if !splash.running {
		splash.mutex.Unlock()
		return
	}
	splash.message = message
	splash.ready = true
	windows := append([]uintptr(nil), splash.windows...)
	splash.mutex.Unlock()
	changed(windows)
}

// changed tells each window to draw again. Posting rather than calling: the
// windows belong to the splash's own thread.
func changed(windows []uintptr) {
	for _, window := range windows {
		_, _, _ = pPostMessage.Call(window, wmSplashChanged, 0, 0)
	}
}

// said answers what the splash says and whether the restore has ended.
func (splash *Splash) said() (application.SplashMessage, bool) {
	splash.mutex.Lock()
	defer splash.mutex.Unlock()
	return splash.message, splash.ready
}

// run makes the windows and carries their messages until the last is closed.
func (splash *Splash) run(generation int) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() {
		// A splash that fails must not take the agent with it; the restore it
		// describes goes on regardless.
		if recovered := recover(); recovered != nil {
			splash.log.Step(fmt.Sprintf("the splash failed unexpectedly: %v", recovered))
		}
		splash.finished(generation)
	}()

	if err := splash.open(); err != nil {
		splash.log.Step(fmt.Sprintf("the splash could not be shown: %v", err))
		return
	}
	var msg message
	for {
		got, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(got) <= 0 {
			return
		}
		_, _, _ = pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = pDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// finished records that the splash is down, so the next restore puts it up.
// A newer splash already up is left alone.
func (splash *Splash) finished(generation int) {
	splash.mutex.Lock()
	defer splash.mutex.Unlock()
	if splash.generation != generation {
		return
	}
	splash.running = false
	splash.windows = nil
	splash.listening = false
}

// open makes one window on each display.
func (splash *Splash) open() error {
	instance, _, _ := pGetModuleHandle.Call(0)
	var classErr error
	registerSplashClass.Do(func() {
		cursor, _, _ := pLoadCursor.Call(0, idcArrow)
		class := windowClass{
			procedure: windows.NewCallback(splashProcedure),
			instance:  instance,
			cursor:    cursor,
			className: wide(splashClass),
		}
		class.size = uint32(unsafe.Sizeof(class))
		if atom, _, err := pRegisterClassEx.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
			classErr = fmt.Errorf("registering the splash window: %w", err)
		}
	})
	if classErr != nil {
		return classErr
	}
	areas := displayAreas()
	if len(areas) == 0 {
		return fmt.Errorf("no display was found to show it on")
	}
	for _, area := range areas {
		window, err := splash.openOn(area, instance)
		if err != nil {
			return err
		}
		splash.mutex.Lock()
		splash.windows = append(splash.windows, window)
		splash.mutex.Unlock()
	}
	// Ready may have arrived while the windows were being made; its post found
	// no window to reach, so the state it left is acted on here.
	if _, ready := splash.said(); ready {
		splash.listen()
	}
	return nil
}

// openOn makes the splash window centred in one display's work area.
//
// It is made small at the area's centre first, so Windows gives it that
// display's scaling, then sized from the scaling it reports: the reference
// machine mixes 96 and 240 dpi and a size worked out anywhere else is wrong on
// one of them.
func (splash *Splash) openOn(area rectangle, instance uintptr) (uintptr, error) {
	centreX := (area.left + area.right) / 2
	centreY := (area.top + area.bottom) / 2
	window, _, err := pCreateWindowEx.Call(
		wsExTopmost|wsExToolWin|wsExNoActivate,
		uintptr(unsafe.Pointer(wide(splashClass))),
		uintptr(unsafe.Pointer(wide(product.Name))),
		wsPopup, uintptr(centreX), uintptr(centreY), 1, 1, 0, 0, instance, 0)
	if window == 0 {
		return 0, fmt.Errorf("making a splash window: %w", err)
	}
	roundCorners(window)
	width, height := scaled(window, cardWidth), scaled(window, cardHeight)
	_, _, _ = pSetWindowPos.Call(window, hwndTopmost,
		uintptr(centreX-width/2), uintptr(centreY-height/2), uintptr(width), uintptr(height),
		swpNoActivate|swpShowWindow)
	return window, nil
}

// closeAll takes every splash down. It runs on the splash's own thread, from
// the window procedure.
func (splash *Splash) closeAll() {
	splash.mutex.Lock()
	windows := append([]uintptr(nil), splash.windows...)
	splash.windows = nil
	// Down from this moment, so a restore starting now puts up a fresh splash
	// rather than posting to windows that are going.
	splash.running = false
	splash.mutex.Unlock()
	splash.deafen()
	for _, window := range windows {
		_, _, _ = pDestroyWindow.Call(window)
	}
	_, _, _ = pPostQuitMessage.Call(0)
}

// splashProcedure is the splash windows' window procedure.
func splashProcedure(hwnd, value, wParam, lParam uintptr) uintptr {
	splash := splashing
	if splash == nil {
		result, _, _ := pDefWindowProc.Call(hwnd, value, wParam, lParam)
		return result
	}
	switch value {
	case wmMouseActivate:
		// Clicked, it still does not take the keyboard (FR-074).
		return maNoActivate
	case wmLButtonDown, wmRButtonDown:
		splash.closeAll()
		return 0
	case wmSplashChanged:
		if _, ready := splash.said(); ready {
			splash.listen()
		}
		_, _, _ = pInvalidateRect.Call(hwnd, 0, 1)
		return 0
	case wmInput:
		if splash.pressed(lParam) {
			splash.closeAll()
		}
		// Windows asks that raw input be handed on, so it can tidy up after it.
		result, _, _ := pDefWindowProc.Call(hwnd, value, wParam, lParam)
		return result
	case wmPaint:
		splash.paint(hwnd)
		return 0
	}
	result, _, _ := pDefWindowProc.Call(hwnd, value, wParam, lParam)
	return result
}
