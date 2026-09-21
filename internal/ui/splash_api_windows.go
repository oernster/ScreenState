//go:build windows

package ui

// The Win32 the splash needs beyond what the tray already declares.

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	gdi32   = windows.NewLazySystemDLL("gdi32.dll")
	msimg32 = windows.NewLazySystemDLL("msimg32.dll")
	dwmapi  = windows.NewLazySystemDLL("dwmapi.dll")

	pSetWindowPos          = user32.NewProc("SetWindowPos")
	pInvalidateRect        = user32.NewProc("InvalidateRect")
	pBeginPaint            = user32.NewProc("BeginPaint")
	pEndPaint              = user32.NewProc("EndPaint")
	pFillRect              = user32.NewProc("FillRect")
	pDrawText              = user32.NewProc("DrawTextW")
	pGetDpiForWindow       = user32.NewProc("GetDpiForWindow")
	pGetClientRect         = user32.NewProc("GetClientRect")
	pEnumDisplayMonitors   = user32.NewProc("EnumDisplayMonitors")
	pGetMonitorInfo        = user32.NewProc("GetMonitorInfoW")
	pRegisterRawInput      = user32.NewProc("RegisterRawInputDevices")
	pGetRawInputData       = user32.NewProc("GetRawInputData")
	pCreateSolidBrush      = gdi32.NewProc("CreateSolidBrush")
	pCreateFont            = gdi32.NewProc("CreateFontW")
	pSelectObject          = gdi32.NewProc("SelectObject")
	pDeleteObject          = gdi32.NewProc("DeleteObject")
	pSetTextColor          = gdi32.NewProc("SetTextColor")
	pSetBkMode             = gdi32.NewProc("SetBkMode")
	pCreateCompatibleDC    = gdi32.NewProc("CreateCompatibleDC")
	pDeleteDC              = gdi32.NewProc("DeleteDC")
	pCreateDIBSection      = gdi32.NewProc("CreateDIBSection")
	pAlphaBlend            = msimg32.NewProc("AlphaBlend")
	pDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

// Messages the splash answers.
const (
	wmPaint         = 0x000F
	wmMouseActivate = 0x0021
	wmInput         = 0x00FF
	wmLButtonDown   = 0x0201
	wmRButtonDown   = 0x0204
	wmKeyDown       = 0x0100
	wmSysKeyDown    = 0x0104

	// maNoActivate answers WM_MOUSEACTIVATE: take the click, not the keyboard.
	maNoActivate = 3
)

// Window styles and placement.
const (
	wsExTopmost    = 0x00000008
	wsExNoActivate = 0x08000000

	hwndTopmost   = ^uintptr(0) // HWND_TOPMOST, minus one
	swpNoActivate = 0x0010
	swpShowWindow = 0x0040

	// baseDPI is the scaling a logical pixel is measured at.
	baseDPI = 96

	// dwmCornerPreference and dwmRound ask Windows 11 for rounded corners,
	// which every other surface of the product has. Older Windows ignores it.
	dwmCornerPreference = 33
	dwmRound            = 2
)

// Drawing.
const (
	transparentBackground = 1 // TRANSPARENT for SetBkMode
	dtCentre              = 0x0001
	dtWordBreak           = 0x0010
	dtNoPrefix            = 0x0800
	dtCalcRect            = 0x0400
	defaultCharset        = 1
	clearTypeQuality      = 5
	fontWeightNormal      = 400
	fontWeightSemibold    = 600
	fontFace              = "Segoe UI"

	biRGB           = 0
	dibRGBColours   = 0
	acSourceOver    = 0
	acSourceAlpha   = 1
	opaqueAlpha     = 255
	bitsPerPixel    = 32
	bitmapPlanes    = 1
	logoBytesPerRow = 4
)

// Raw input, which reaches a window that does not hold the keyboard.
const (
	usagePageGeneric = 0x01
	usageMouse       = 0x02
	usageKeyboard    = 0x06
	ridevInputSink   = 0x00000100
	ridevRemove      = 0x00000001
	ridInput         = 0x10000003
	rimTypeMouse     = 0
	rimTypeKeyboard  = 1

	// mouseButtonsDown is every RI_MOUSE_*_BUTTON_DOWN flag: left, right,
	// middle, fourth and fifth. A movement or a release closes nothing.
	mouseButtonsDown = 0x0001 | 0x0004 | 0x0010 | 0x0040 | 0x0100
)

// rectangle is the Win32 RECT.
type rectangle struct {
	left, top, right, bottom int32
}

// paintStruct is PAINTSTRUCT.
type paintStruct struct {
	hdc        uintptr
	erase      int32
	paint      rectangle
	restore    int32
	incUpdate  int32
	rgbReserve [32]byte
}

// monitorInfo is MONITORINFO: the display's whole area, its work area (the
// part the taskbar leaves) and its flags.
type monitorInfo struct {
	size    uint32
	monitor rectangle
	work    rectangle
	flags   uint32
}

// bitmapInfoHeader is BITMAPINFOHEADER.
type bitmapInfoHeader struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
}

// rawInputDevice is RAWINPUTDEVICE.
type rawInputDevice struct {
	usagePage uint16
	usage     uint16
	flags     uint32
	target    uintptr
}

// rawInputHeaderSize is the size of RAWINPUTHEADER, which leads every raw
// input record: its type and size, four bytes each, then the device handle and
// the wParam, a pointer each. Only the type at its start is read.
const rawInputHeaderSize = 2*fieldWidth32 + 2*unsafe.Sizeof(uintptr(0))

// colourRef is a Colour as Windows takes one: 0x00bbggrr.
func colourRef(colour Colour) uintptr {
	const greenShift, blueShift = 8, 16
	return uintptr(colour.R) | uintptr(colour.G)<<greenShift | uintptr(colour.B)<<blueShift
}
