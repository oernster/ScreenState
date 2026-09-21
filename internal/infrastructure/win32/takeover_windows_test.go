//go:build windows

package win32

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestThePressIsTracedToItsTopLevelWindow reads the real desktop, as the probe
// tests do: a press in the middle of the primary taskbar lands on a window deep
// inside it, so answering the taskbar's own class proves both the point handed
// to Windows and the walk up to the top-level window. It moves nothing.
func TestThePressIsTracedToItsTopLevelWindow(t *testing.T) {
	user := windows.NewLazySystemDLL("user32.dll")
	findWindow := user.NewProc("FindWindowW")
	windowRect := user.NewProc("GetWindowRect")

	class, err := windows.UTF16PtrFromString(taskbarClasses[0])
	if err != nil {
		t.Fatalf("naming the taskbar: %v", err)
	}
	taskbar, _, _ := findWindow.Call(uintptr(unsafe.Pointer(class)), 0)
	if taskbar == 0 {
		t.Skip("no taskbar to read here")
	}
	var bounds rect
	if ok, _, err := windowRect.Call(taskbar, uintptr(unsafe.Pointer(&bounds))); ok == 0 {
		t.Fatalf("reading the taskbar's rectangle: %v", err)
	}
	const middle = 2
	info := mouseHookInfo{pt: point{
		x: bounds.left + (bounds.right-bounds.left)/middle,
		y: bounds.top + (bounds.bottom-bounds.top)/middle,
	}}
	if got := pressedOn(true, uintptr(unsafe.Pointer(&info))); got != taskbarClasses[0] {
		t.Fatalf("a press on the taskbar was traced to %q", got)
	}
	if got := pressedOn(false, uintptr(unsafe.Pointer(&info))); got != "" {
		t.Errorf("a key press was traced to %q, as though it had a pointer", got)
	}
}
