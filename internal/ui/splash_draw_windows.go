//go:build windows

package ui

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The splash's layout, in logical pixels: what they come to on a display is
// worked out from that display's scaling, so the card is the same size to the
// eye on the 96 and the 240 dpi screens of the reference machine.
const (
	cardWidth    = 560
	cardHeight   = 400
	borderWidth  = 1
	logoSize     = 160
	logoTop      = 48
	textGap      = 28
	detailGap    = 10
	sidePadding  = 32
	headlineSize = 24
	detailSize   = 16
)

// paint draws one splash window: the card, the artwork and the words.
func (splash *Splash) paint(window uintptr) {
	var paint paintStruct
	hdc, _, _ := pBeginPaint.Call(window, uintptr(unsafe.Pointer(&paint)))
	if hdc == 0 {
		return
	}
	defer func() { _, _, _ = pEndPaint.Call(window, uintptr(unsafe.Pointer(&paint))) }()

	palette := splash.themes.Light
	if splash.dark != nil && splash.dark() {
		palette = splash.themes.Dark
	}
	var client rectangle
	_, _, _ = pGetClientRect.Call(window, uintptr(unsafe.Pointer(&client)))
	fill(hdc, client, palette.Border)
	border := scaled(window, borderWidth)
	fill(hdc, rectangle{client.left + border, client.top + border,
		client.right - border, client.bottom - border}, palette.Panel)

	y := scaled(window, logoTop)
	if splash.logo != nil {
		pixels, side := splash.logoPixels(int(scaled(window, logoSize)))
		if side > 0 {
			drawLogo(hdc, (client.right-side)/2, y, side, pixels)
			y += side
		}
	}
	y += scaled(window, textGap)

	message, _ := splash.said()
	_, _, _ = pSetBkMode.Call(hdc, transparentBackground)
	padding := scaled(window, sidePadding)
	area := rectangle{client.left + padding, y, client.right - padding, client.bottom - padding}
	used := drawWords(hdc, message.Headline, area, scaled(window, headlineSize),
		fontWeightSemibold, palette.Text)
	if message.Detail != "" {
		area.top += used + scaled(window, detailGap)
		drawWords(hdc, message.Detail, area, scaled(window, detailSize),
			fontWeightNormal, palette.Muted)
	}
}

// fill paints a rectangle in one colour.
func fill(hdc uintptr, area rectangle, colour Colour) {
	brush, _, _ := pCreateSolidBrush.Call(colourRef(colour))
	if brush == 0 {
		return
	}
	defer func() { _, _, _ = pDeleteObject.Call(brush) }()
	_, _, _ = pFillRect.Call(hdc, uintptr(unsafe.Pointer(&area)), brush)
}

// drawWords draws centred, wrapped text from the top of area and answers the
// height it took.
func drawWords(hdc uintptr, words string, area rectangle, size int32, weight int, colour Colour) int32 {
	font, _, _ := pCreateFont.Call(uintptr(-size), 0, 0, 0, uintptr(weight), 0, 0, 0,
		defaultCharset, 0, 0, clearTypeQuality, 0, uintptr(unsafe.Pointer(wide(fontFace))))
	if font != 0 {
		previous, _, _ := pSelectObject.Call(hdc, font)
		defer func() {
			_, _, _ = pSelectObject.Call(hdc, previous)
			_, _, _ = pDeleteObject.Call(font)
		}()
	}
	_, _, _ = pSetTextColor.Call(hdc, colourRef(colour))
	text := wide(words)
	flags := uintptr(dtCentre | dtWordBreak | dtNoPrefix)
	// Measured first, so the line beneath knows where this one ends.
	measured := area
	_, _, _ = pDrawText.Call(hdc, uintptr(unsafe.Pointer(text)), ^uintptr(0),
		uintptr(unsafe.Pointer(&measured)), flags|dtCalcRect)
	_, _, _ = pDrawText.Call(hdc, uintptr(unsafe.Pointer(text)), ^uintptr(0),
		uintptr(unsafe.Pointer(&area)), flags)
	return measured.bottom - measured.top
}

// logoPixels answers the artwork reduced to at most size pixels a side as a
// transparent bitmap's bytes, with the side it came to. Reducing the master is
// the slow part, so each size is done once.
func (splash *Splash) logoPixels(size int) ([]byte, int32) {
	splash.mutex.Lock()
	defer splash.mutex.Unlock()
	if splash.shrunk == nil {
		splash.shrunk = map[int][]byte{}
	}
	pixels, found := splash.shrunk[size]
	if !found {
		pixels = premultiplied(shrink(splash.logo, size))
		splash.shrunk[size] = pixels
	}
	side := int32(0)
	if len(pixels) > 0 {
		side = int32(isqrt(len(pixels) / logoBytesPerRow))
	}
	return pixels, side
}

// isqrt is the side of a square holding the given number of pixels.
func isqrt(pixels int) int {
	side := 0
	for (side+1)*(side+1) <= pixels {
		side++
	}
	return side
}

// drawLogo blends the artwork onto the card, keeping its transparency.
func drawLogo(hdc uintptr, x, y, side int32, pixels []byte) {
	header := bitmapInfoHeader{
		width: side, height: -side, // negative: the rows run top down
		planes: bitmapPlanes, bitCount: bitsPerPixel, compression: biRGB,
	}
	header.size = uint32(unsafe.Sizeof(header))
	var bits unsafe.Pointer
	bitmap, _, _ := pCreateDIBSection.Call(hdc, uintptr(unsafe.Pointer(&header)),
		dibRGBColours, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == nil {
		return
	}
	defer func() { _, _, _ = pDeleteObject.Call(bitmap) }()
	copy(unsafe.Slice((*byte)(bits), len(pixels)), pixels)

	memory, _, _ := pCreateCompatibleDC.Call(hdc)
	if memory == 0 {
		return
	}
	defer func() { _, _, _ = pDeleteDC.Call(memory) }()
	previous, _, _ := pSelectObject.Call(memory, bitmap)
	defer func() { _, _, _ = pSelectObject.Call(memory, previous) }()

	// BLENDFUNCTION is four bytes passed by value: the operation, its flags,
	// the constant alpha and the alpha format.
	const constantAlphaShift, formatShift = 16, 24
	blend := uintptr(acSourceOver) | uintptr(opaqueAlpha)<<constantAlphaShift |
		uintptr(acSourceAlpha)<<formatShift
	_, _, _ = pAlphaBlend.Call(hdc, uintptr(x), uintptr(y), uintptr(side), uintptr(side),
		memory, 0, 0, uintptr(side), uintptr(side), blend)
}

// scaled turns logical pixels into a window's own, by its display's scaling.
func scaled(window uintptr, logical int32) int32 {
	dpi, _, _ := pGetDpiForWindow.Call(window)
	if dpi == 0 {
		dpi = baseDPI
	}
	return logical * int32(dpi) / baseDPI
}

// roundCorners asks Windows 11 to round the card's corners.
func roundCorners(window uintptr) {
	preference := uint32(dwmRound)
	_, _, _ = pDwmSetWindowAttribute.Call(window, dwmCornerPreference,
		uintptr(unsafe.Pointer(&preference)), unsafe.Sizeof(preference))
}

// Enumerating the displays: Windows hands each to a callback, which gathers
// them here. A callback is made once for the run, since Go can make only so
// many and never frees one.
var (
	areasLock  sync.Mutex
	areasFound []rectangle
	areaFound  = windows.NewCallback(func(monitor, _, _, _ uintptr) uintptr {
		info := monitorInfo{}
		info.size = uint32(unsafe.Sizeof(info))
		if ok, _, _ := pGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
			areasFound = append(areasFound, info.work)
		}
		return 1
	})
)

// displayAreas answers the work area of every display.
func displayAreas() []rectangle {
	areasLock.Lock()
	defer areasLock.Unlock()
	areasFound = nil
	_, _, _ = pEnumDisplayMonitors.Call(0, 0, areaFound, 0)
	return append([]rectangle(nil), areasFound...)
}
