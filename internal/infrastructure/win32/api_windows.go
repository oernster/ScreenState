//go:build windows

package win32

import "golang.org/x/sys/windows"

// The Win32 surface this product uses, in one place. Nothing here decides
// anything: every call is a reading or a move. The rules about what to read
// and where to move it live above this package.
//
// The build is 64 bit only, which is why GetWindowLongPtrW is called rather
// than the 32 bit GetWindowLongW that Windows keeps beside it.
var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	ole32    = windows.NewLazySystemDLL("ole32.dll")

	pEnumWindows              = user32.NewProc("EnumWindows")
	pGetClassName             = user32.NewProc("GetClassNameW")
	pIsWindow                 = user32.NewProc("IsWindow")
	pIsWindowVisible          = user32.NewProc("IsWindowVisible")
	pGetWindow                = user32.NewProc("GetWindow")
	pGetWindowLongPtr         = user32.NewProc("GetWindowLongPtrW")
	pGetWindowTextLength      = user32.NewProc("GetWindowTextLengthW")
	pGetWindowText            = user32.NewProc("GetWindowTextW")
	pGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	pGetWindowPlacement       = user32.NewProc("GetWindowPlacement")
	pSetWindowPos             = user32.NewProc("SetWindowPos")
	pShowWindow               = user32.NewProc("ShowWindow")
	pSetWindowPlacement       = user32.NewProc("SetWindowPlacement")
	pPostMessage              = user32.NewProc("PostMessageW")
	pEnumDisplayMonitors      = user32.NewProc("EnumDisplayMonitors")
	pGetMonitorInfo           = user32.NewProc("GetMonitorInfoW")
	pEnumDisplayDevices       = user32.NewProc("EnumDisplayDevicesW")
	pSetProcessDpiAwareness   = user32.NewProc("SetProcessDpiAwarenessContext")

	pGetApplicationUserModelID = kernel32.NewProc("GetApplicationUserModelId")
	pDwmGetWindowAttribute     = dwmapi.NewProc("DwmGetWindowAttribute")
	pShellExecute              = shell32.NewProc("ShellExecuteW")
	pCoCreateInstance          = ole32.NewProc("CoCreateInstance")
)

// Window reading and moving.
const (
	gwOwner        = 4            // GW_OWNER
	gwlExStyle     = ^uintptr(19) // GWL_EXSTYLE (-20)
	wsExToolWindow = 0x00000080
	wsExNoActivate = 0x08000000

	// dwmwaCloaked asks whether a window is hidden by the desktop window
	// manager rather than by its own application. A Store application that has
	// been closed leaves a visible, cloaked window behind, which is not a window
	// any user would say is open.
	dwmwaCloaked = 14

	swHide   = 0
	swNormal = 1
	// swMaximize is also the show state SetWindowPlacement is given to maximise
	// a window without activating it (FR-074).
	swMaximize = 3
	// swShowNoActivate shows a window in its current size and place without
	// making it the active one, which SW_RESTORE would (FR-074).
	swShowNoActivate = 4
	swMinimize       = 6
	swMinNoActive    = 7

	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	// wmClose is the message a title bar's cross sends. It is a request the
	// application answers however it likes, which is the whole reason FR-064
	// offers it rather than the agent ending anything itself.
	wmClose = 0x0010

	// maxTitle bounds a window title read into memory. A title is used to tell
	// a window apart in the review, never to identify anything, so a long one
	// is truncated rather than refused.
	maxTitle = 512
)

// Display reading.
const (
	monitorPrimary = 0x00000001 // MONITORINFOF_PRIMARY
	// eddInterfaceName asks EnumDisplayDevices for the device interface name,
	// which carries the device instance path that survives a reboot. Without
	// it Windows answers a name that does not.
	eddInterfaceName = 0x00000001
	deviceNameSize   = 32
	deviceStringSize = 128
	deviceIDSize     = 128
	deviceKeySize    = 128
)

// dpiPerMonitorV2 makes this process see real pixel coordinates on every
// display rather than the scaled coordinates Windows invents for a process that
// has not said it understands mixed scaling. Without it, a rectangle read on a
// 240 dpi screen and applied on a 96 dpi one lands in the wrong place; the
// reference machine has both.
var dpiPerMonitorV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)

// rect is the Win32 RECT: two corners rather than a position and a size, which
// is why nothing above this package uses one.
type rect struct {
	left   int32
	top    int32
	right  int32
	bottom int32
}

// monitorInfoEx is MONITORINFOEXW: a monitor's two rectangles, its flags and
// the device name that leads to its identity.
type monitorInfoEx struct {
	cbSize  uint32
	monitor rect
	work    rect
	flags   uint32
	device  [deviceNameSize]uint16
}

// point is the Win32 POINT, carried inside a window placement.
type point struct {
	x int32
	y int32
}

// windowPlacement is WINDOWPLACEMENT. rcNormalPosition is the rectangle a
// window occupies when it is neither minimised nor maximised, which is the one
// a profile records: a maximised window still has one and it decides which
// display maximising puts it on.
type windowPlacement struct {
	length           uint32
	flags            uint32
	showCmd          uint32
	ptMinPosition    point
	ptMaxPosition    point
	rcNormalPosition rect
}

// displayDevice is DISPLAY_DEVICEW. deviceID carries the interface name that
// monitorIDFrom reduces to a display identity.
type displayDevice struct {
	cb           uint32
	deviceName   [deviceNameSize]uint16
	deviceString [deviceStringSize]uint16
	stateFlags   uint32
	deviceID     [deviceIDSize]uint16
	deviceKey    [deviceKeySize]uint16
}
