//go:build windows

package win32

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
	"golang.org/x/sys/windows"
)

// isCandidate applies the rule measured on 2026-09-19: a window a person would
// point at is visible, not cloaked, not owned by another window, not a tool
// window and not untitled. Anything else is machinery.
func isCandidate(handle uintptr) bool {
	if visible, _, _ := pIsWindowVisible.Call(handle); visible == 0 {
		return false
	}
	if isCloaked(handle) {
		return false
	}
	if owner, _, _ := pGetWindow.Call(handle, gwOwner); owner != 0 {
		return false
	}
	style, _, _ := pGetWindowLongPtr.Call(handle, gwlExStyle)
	if style&wsExToolWindow != 0 {
		return false
	}
	length, _, _ := pGetWindowTextLength.Call(handle)
	return length > 0
}

// isCloaked reports whether the desktop window manager is hiding a window. A
// Store application that has been closed leaves one behind: visible by the old
// test, drawn by nobody.
func isCloaked(handle uintptr) bool {
	var cloaked uint32
	result, _, _ := pDwmGetWindowAttribute.Call(handle, dwmwaCloaked,
		uintptr(unsafe.Pointer(&cloaked)), unsafe.Sizeof(cloaked))
	return result == 0 && cloaked != 0
}

// titleOf reads a window's title, which names it in the review and in the
// report. It identifies nothing: a title changes with whatever the application
// is showing.
func titleOf(handle uintptr) string {
	buffer := make([]uint16, maxTitle)
	length, _, _ := pGetWindowText.Call(handle,
		uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if length == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:length])
}

// stateFrom turns a Win32 show command into the state a profile records.
func stateFrom(showCmd uint32) domain.ShowState {
	switch showCmd {
	case swMaximize:
		return domain.ShowMaximised
	case swMinimize, swMinNoActive:
		return domain.ShowMinimised
	default:
		return domain.ShowNormal
	}
}

// fromRect turns a Win32 rectangle, which is two corners, into the position and
// size a profile records. A size cannot then go negative by accident.
func fromRect(from rect) domain.Rect {
	return domain.Rect{
		X:      from.left,
		Y:      from.top,
		Width:  from.right - from.left,
		Height: from.bottom - from.top,
	}
}

// monitorCollection gathers monitors during a single EnumDisplayMonitors call,
// guarded for the same reason the window enumeration is.
var (
	monitorLock sync.Mutex
	monitors    []uintptr
	collectMon  = windows.NewCallback(func(handle, _, _, _ uintptr) uintptr {
		monitors = append(monitors, handle)
		return 1 // carry on
	})
)

// monitorHandles returns every connected monitor.
func monitorHandles() ([]uintptr, error) {
	monitorLock.Lock()
	defer monitorLock.Unlock()
	monitors = nil
	if ok, _, err := pEnumDisplayMonitors.Call(0, 0, collectMon, 0); ok == 0 {
		return nil, fmt.Errorf("enumerating the displays: %w", err)
	}
	found := make([]uintptr, len(monitors))
	copy(found, monitors)
	return found, nil
}

// describeMonitor reads one monitor: its two rectangles, whether it is the
// primary and the identity that survives a reboot.
func describeMonitor(handle uintptr) (application.Display, error) {
	info := monitorInfoEx{}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if ok, _, err := pGetMonitorInfo.Call(handle, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return application.Display{}, fmt.Errorf("reading a display: %w", err)
	}
	monitorID, err := identityOfDisplay(info.device)
	if err != nil {
		return application.Display{}, err
	}
	identity, err := domain.NewDisplayIdentity(monitorID)
	if err != nil {
		return application.Display{}, err
	}
	return application.Display{
		Identity: identity,
		Bounds:   fromRect(info.monitor),
		WorkArea: fromRect(info.work),
		Primary:  info.flags&monitorPrimary != 0,
	}, nil
}

// identityOfDisplay asks Windows for the device interface name behind a device
// name such as \\.\DISPLAY1, then reduces it to the part that survives a
// reboot. The device name itself is not an identity: it was measured
// disagreeing with the number Windows Settings shows and with where the screen
// physically sits.
func identityOfDisplay(deviceName [deviceNameSize]uint16) (string, error) {
	device := displayDevice{}
	device.cb = uint32(unsafe.Sizeof(device))
	ok, _, err := pEnumDisplayDevices.Call(
		uintptr(unsafe.Pointer(&deviceName[0])), 0,
		uintptr(unsafe.Pointer(&device)), eddInterfaceName)
	if ok == 0 {
		return "", fmt.Errorf("reading the display behind %s: %w",
			windows.UTF16ToString(deviceName[:]), err)
	}
	monitorID, usable := monitorIDFrom(windows.UTF16ToString(device.deviceID[:]))
	if !usable {
		return "", fmt.Errorf("%w: %s", ErrNoMonitorID,
			windows.UTF16ToString(deviceName[:]))
	}
	return monitorID, nil
}

// identityOf names the application owning a window, by the three rules in
// appendix E.
func (desktop *Desktop) identityOf(handle uintptr) (domain.ApplicationIdentity, error) {
	var pid uint32
	_, _, _ = pGetWindowThreadProcessID.Call(handle, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return domain.ApplicationIdentity{}, fmt.Errorf("its process could not be read")
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return domain.ApplicationIdentity{}, fmt.Errorf("its process could not be opened: %w", err)
	}
	defer func() { _ = windows.CloseHandle(process) }()

	imagePath, err := imageOf(process)
	if err != nil {
		return domain.ApplicationIdentity{}, err
	}
	return identityFor(imagePath, modelIDOf(process), exists)
}

// exists reports whether a file is there, which is how the updater rule is
// settled: a versioned directory with no updater beside it is named by its path
// like anything else.
func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// imageOf returns the full path of the program a process is running.
func imageOf(process windows.Handle) (string, error) {
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(process, 0, &buffer[0], &size); err != nil {
		return "", fmt.Errorf("its program could not be read: %w", err)
	}
	return windows.UTF16ToString(buffer[:size]), nil
}

// modelIDOf returns a process's application user model id, empty for a program
// that is not Store packaged. The absence is an answer rather than a fault:
// most applications have none and are named by their path instead.
func modelIDOf(process windows.Handle) string {
	length := uint32(maxTitle)
	buffer := make([]uint16, length)
	result, _, _ := pGetApplicationUserModelID.Call(uintptr(process),
		uintptr(unsafe.Pointer(&length)), uintptr(unsafe.Pointer(&buffer[0])))
	if result != 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:length])
}
