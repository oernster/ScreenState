//go:build windows

package window

import (
	"os"
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestAGreyedCloseIsNotAGuard records why greying the window menu's Close is
// not what keeps the manager open while a dialog is up: a greyed Close still
// reaches the window as WM_CLOSE. Measured 2026-09-22, against the hypothesis
// that it would not; the refusal therefore lives in the agent's beforeClose. It
// opens a window of its own, so it runs only when asked for:
//
//	$env:SCREENSTATE_DESKTOP_PROBE = '1'; go test -run TestAGreyedCloseIsNotAGuard -v ./internal/infrastructure/window
//
// The close command is sent as WM_SYSCOMMAND with SC_CLOSE, which is what the
// caption's cross, Alt+F4 and the taskbar's Close each send. The control sends
// it with Close enabled: if that delivers no WM_CLOSE, the test cannot see one.

const (
	probeVariable  = "SCREENSTATE_DESKTOP_PROBE"
	probeStyle     = 0x00CF0000 // WS_OVERLAPPEDWINDOW
	wmClose        = 0x0010
	wmSysCommand   = 0x0112
	probeClassName = "ScreenStateCloseProbe"
)

var (
	procRegisterClass = moduser32.NewProc("RegisterClassExW")
	procCreateWindow  = moduser32.NewProc("CreateWindowExW")
	procDefWindowProc = moduser32.NewProc("DefWindowProcW")
	procDestroyWindow = moduser32.NewProc("DestroyWindow")
	procSendMessage   = moduser32.NewProc("SendMessageW")
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

// closesAsked counts the WM_CLOSE messages the probe window receives. The
// window procedure swallows each, so the window survives to be asked again.
var closesAsked int

func probeProcedure(hwnd, message, wParam, lParam uintptr) uintptr {
	if message == wmClose {
		closesAsked++
		return 0
	}
	result, _, _ := procDefWindowProc.Call(hwnd, message, wParam, lParam)
	return result
}

func TestAGreyedCloseIsNotAGuard(t *testing.T) {
	if os.Getenv(probeVariable) == "" {
		t.Skipf("opens a window of its own; set %s to run it", probeVariable)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	class, _ := windows.UTF16PtrFromString(probeClassName)
	registration := probeClass{procedure: windows.NewCallback(probeProcedure), className: class}
	registration.size = uint32(unsafe.Sizeof(registration))
	if atom, _, err := procRegisterClass.Call(uintptr(unsafe.Pointer(&registration))); atom == 0 {
		t.Fatalf("registering the probe's class: %v", err)
	}
	hwnd, _, err := procCreateWindow.Call(0, uintptr(unsafe.Pointer(class)), 0, probeStyle,
		0, 0, 1, 1, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatalf("making the probe window: %v", err)
	}
	defer procDestroyWindow.Call(hwnd)
	askToClose := func() int {
		closesAsked = 0
		procSendMessage.Call(hwnd, wmSysCommand, scClose, 0)
		return closesAsked
	}

	if control := askToClose(); control == 0 {
		t.Fatal("Close with the command enabled delivered no WM_CLOSE, so this test cannot see one")
	}
	if !allowClose(windows.HWND(hwnd), false) {
		t.Fatal("the close command could not be greyed")
	}
	if asked := askToClose(); asked == 0 {
		t.Error("a greyed Close delivered no WM_CLOSE: greying is now a guard, so beforeClose's reason has changed")
	}
	if !allowClose(windows.HWND(hwnd), true) {
		t.Fatal("the close command could not be restored")
	}
	if asked := askToClose(); asked == 0 {
		t.Error("Close restored delivered no WM_CLOSE")
	}
}
