//go:build windows

// Package ui is this product's Windows surface: the tray icon, its menu and the
// message loop that carries them. Every Win32 call for the interface lives
// here; the layers above it never see one.
//
// Nothing here decides anything. What the menu says and what choosing an entry
// does are settled in the application layer, which is testable without a
// desktop; this package draws what it is given and reports what was chosen.
package ui

import "golang.org/x/sys/windows"

var (
	user32  = windows.NewLazySystemDLL("user32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	kernel  = windows.NewLazySystemDLL("kernel32.dll")

	pRegisterClassEx     = user32.NewProc("RegisterClassExW")
	pCreateWindowEx      = user32.NewProc("CreateWindowExW")
	pDestroyWindow       = user32.NewProc("DestroyWindow")
	pDefWindowProc       = user32.NewProc("DefWindowProcW")
	pGetMessage          = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessage     = user32.NewProc("DispatchMessageW")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pPostMessage         = user32.NewProc("PostMessageW")
	pCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	pAppendMenu          = user32.NewProc("AppendMenuW")
	pTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	pDestroyMenu         = user32.NewProc("DestroyMenu")
	pGetCursorPos        = user32.NewProc("GetCursorPos")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pLoadCursor          = user32.NewProc("LoadCursorW")
	pLoadIcon            = user32.NewProc("LoadIconW")
	pMessageBox          = user32.NewProc("MessageBoxW")
	pRegisterWindowMsg   = user32.NewProc("RegisterWindowMessageW")

	pShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")
	pExtractIconEx   = shell32.NewProc("ExtractIconExW")

	pGetModuleHandle = kernel.NewProc("GetModuleHandleW")
)

// Window messages this product answers.
const (
	wmDestroy     = 0x0002
	wmClose       = 0x0010
	wmCommand     = 0x0111
	wmNull        = 0x0000
	wmRButtonUp   = 0x0205
	wmLButtonUp   = 0x0202
	wmContextMenu = 0x007B

	// wmTray is the message the shell sends this window when the user does
	// something to the tray icon. WM_APP plus one, which is this product's to
	// choose.
	wmTray = 0x8001

	// wmRefresh asks the window to read the menu and the tooltip again, which
	// is how work finishing on another goroutine reaches the interface without
	// that goroutine touching a window.
	wmRefresh = 0x8002
)

// Window and class settings.
const (
	wsPopup      = 0x80000000
	wsExToolWin  = 0x00000080
	cwUseDefault = ^uintptr(0x7FFFFFFF) // CW_USEDEFAULT

	idcArrow       = 32512
	idiApplication = 32512
)

// Menu flags.
const (
	mfString    = 0x0000
	mfGrayed    = 0x0001
	mfDisabled  = 0x0002
	mfChecked   = 0x0008
	mfSeparator = 0x0800

	tpmReturnCmd   = 0x0100
	tpmRightButton = 0x0002
)

// Tray icon flags.
const (
	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	// tipLength is the room Windows gives a tray tooltip, in UTF-16 units.
	tipLength = 128
)

// Message box flags. The report is information, so it carries the information
// icon rather than a warning: an entry left outstanding is something to read,
// not an error the user caused.
const (
	mbIconInfo    = 0x00000040
	mbIconWarning = 0x00000030
	mbOK          = 0x00000000
)

// point is the Win32 POINT, used to open the menu where the pointer is.
type point struct {
	x int32
	y int32
}

// message is the Win32 MSG the loop reads.
type message struct {
	hwnd    uintptr
	value   uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

// windowClass is WNDCLASSEXW, the class the hidden window is made from.
type windowClass struct {
	size       uint32
	style      uint32
	procedure  uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	smallIcon  uintptr
}

// notifyIconData is NOTIFYICONDATAW, the tray icon itself.
type notifyIconData struct {
	cbSize           uint32
	hwnd             uintptr
	id               uint32
	flags            uint32
	callbackMessage  uint32
	icon             uintptr
	tip              [tipLength]uint16
	state            uint32
	stateMask        uint32
	info             [256]uint16
	timeoutOrVersion uint32
	infoTitle        [64]uint16
	infoFlags        uint32
	guidItem         [16]byte
	balloonIcon      uintptr
}

// wide turns a Go string into the UTF-16 Windows takes. A string Windows
// cannot represent answers an empty one rather than stopping the run: a label
// that comes out blank is better than a tray that never appears.
func wide(text string) *uint16 {
	pointer, err := windows.UTF16PtrFromString(text)
	if err != nil {
		empty, _ := windows.UTF16PtrFromString("")
		return empty
	}
	return pointer
}

// windowsString turns a Go string into UTF-16 units for a fixed-size field.
func windowsString(text string) []uint16 {
	encoded, err := windows.UTF16FromString(text)
	if err != nil {
		return []uint16{0}
	}
	return encoded
}
