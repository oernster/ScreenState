//go:build windows

package win32

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
	"golang.org/x/sys/windows"
)

// FR-074: placing a maximised window never activates it.
//
// It cannot be settled without windows. A test that opens windows and takes
// the front would disturb whoever runs the suite, so it runs only when asked
// for:
//
//	$env:SCREENSTATE_DESKTOP_PROBE = '1'; go test -run TestMaximisingDoesNotActivate -v ./internal/infrastructure/win32
//
// It makes two windows of its own and touches no other. The anchor holds the
// front; the subject is placed maximised through Desktop.Place (the path a
// restore takes) while the WM_ACTIVATE messages it receives are counted. A
// control maximises it the ordinary way first: if that registers no
// activation, the test cannot see one and says so rather than passing.
//
// Measured on 2026-09-21 before the fix: SetWindowPlacement with SW_MAXIMIZE,
// which the placing code used, activated the subject every time.

// probeVariable turns the test on.
const probeVariable = "SCREENSTATE_DESKTOP_PROBE"

// The messages and styles the test's own windows need.
const (
	probeStyle      = 0x00CF0000 // WS_OVERLAPPEDWINDOW
	probeActivate   = 0x0006     // WM_ACTIVATE
	probePeekRemove = 0x0001     // PM_REMOVE
	probeShowNormal = 1          // SW_SHOWNORMAL
	probeRestore    = 9          // SW_RESTORE
	probeSettle     = 400 * time.Millisecond
	probeTick       = 10 * time.Millisecond
	probeLeft       = 100
	probeTop        = 200
	probeWidth      = 500
	probeHeight     = 350
)

var (
	pProbeRegisterClass = user32.NewProc("RegisterClassExW")
	pProbeCreateWindow  = user32.NewProc("CreateWindowExW")
	pProbeDefWindowProc = user32.NewProc("DefWindowProcW")
	pProbeDestroy       = user32.NewProc("DestroyWindow")
	pProbePeek          = user32.NewProc("PeekMessageW")
	pProbeTranslate     = user32.NewProc("TranslateMessage")
	pProbeDispatch      = user32.NewProc("DispatchMessageW")
	pProbeForeground    = user32.NewProc("GetForegroundWindow")
	pProbeSetForeground = user32.NewProc("SetForegroundWindow")
	pProbeIsZoomed      = user32.NewProc("IsZoomed")
)

type probeClass struct {
	size       uint32
	style      uint32
	procedure  uintptr
	classExtra int32
	windExtra  int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSmall  uintptr
}

type probeMessage struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

// probeActivations counts the activations each window receives.
var probeActivations = map[uintptr]int{}

func probeProcedure(hwnd, message, wParam, lParam uintptr) uintptr {
	if message == probeActivate && wParam&0xFFFF != 0 {
		probeActivations[hwnd]++
	}
	result, _, _ := pProbeDefWindowProc.Call(hwnd, message, wParam, lParam)
	return result
}

// probePump delivers this thread's messages for a while, so every activation
// a call caused has reached the window procedure before anything is read.
func probePump() {
	var message probeMessage
	for until := time.Now().Add(probeSettle); time.Now().Before(until); time.Sleep(probeTick) {
		for {
			got, _, _ := pProbePeek.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, probePeekRemove)
			if got == 0 {
				break
			}
			pProbeTranslate.Call(uintptr(unsafe.Pointer(&message)))
			pProbeDispatch.Call(uintptr(unsafe.Pointer(&message)))
		}
	}
}

// probeWindows makes the anchor and the subject, removing both at the end.
func probeWindows(t *testing.T) (anchor, subject uintptr) {
	t.Helper()
	class, _ := windows.UTF16PtrFromString("ScreenStateMaximiseProbe")
	title, _ := windows.UTF16PtrFromString("ScreenState FR-074 probe")
	registration := probeClass{procedure: windows.NewCallback(probeProcedure), className: class}
	registration.size = uint32(unsafe.Sizeof(registration))
	if atom, _, err := pProbeRegisterClass.Call(uintptr(unsafe.Pointer(&registration))); atom == 0 {
		t.Fatalf("registering the probe's window class: %v", err)
	}
	make := func(left int) uintptr {
		hwnd, _, err := pProbeCreateWindow.Call(0, uintptr(unsafe.Pointer(class)),
			uintptr(unsafe.Pointer(title)), probeStyle, uintptr(left), probeTop,
			probeWidth, probeHeight, 0, 0, 0, 0)
		if hwnd == 0 {
			t.Fatalf("making a probe window: %v", err)
		}
		t.Cleanup(func() { pProbeDestroy.Call(hwnd) })
		return hwnd
	}
	return make(probeLeft), make(probeLeft + probeWidth + probeLeft)
}

// activationsOf restores the subject, gives the anchor the front, runs the act
// and answers how often the subject was activated, where the front ended up
// and whether the subject ended maximised.
func activationsOf(t *testing.T, anchor, subject uintptr, act func()) (int, bool, bool) {
	t.Helper()
	pShowWindow.Call(subject, probeRestore)
	pProbeSetForeground.Call(anchor)
	probePump()
	if front, _, _ := pProbeForeground.Call(); front != anchor {
		t.Skip("the probe could not hold the front, so no activation could be seen")
	}
	probeActivations[subject] = 0
	act()
	probePump()
	front, _, _ := pProbeForeground.Call()
	zoomed, _, _ := pProbeIsZoomed.Call(subject)
	return probeActivations[subject], front == anchor, zoomed != 0
}

func TestMaximisingDoesNotActivate(t *testing.T) {
	if os.Getenv(probeVariable) == "" {
		t.Skipf("opens windows and takes the front; set %s to run it", probeVariable)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	anchor, subject := probeWindows(t)
	pShowWindow.Call(anchor, probeShowNormal)
	probePump()

	control, _, _ := activationsOf(t, anchor, subject, func() { pShowWindow.Call(subject, swMaximize) })
	if control == 0 {
		t.Fatal("maximising the ordinary way registered no activation, so this test cannot see one")
	}

	desktop := NewDesktop(probeClock{})
	target := domain.Rect{X: probeLeft, Y: probeTop, Width: probeWidth, Height: probeHeight}
	var placeErr error
	activated, anchorKept, zoomed := activationsOf(t, anchor, subject, func() {
		placeErr = desktop.Place(context.Background(), application.WindowID(subject), target,
			domain.ShowMaximised)
	})
	if placeErr != nil {
		t.Fatalf("placing: %v", placeErr)
	}
	if activated != 0 || !anchorKept {
		t.Errorf("placing a maximised window activated it %d time(s); the anchor kept the front: %v",
			activated, anchorKept)
	}
	if !zoomed {
		t.Error("the window was not left maximised")
	}
}
