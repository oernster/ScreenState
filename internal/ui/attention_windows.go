//go:build windows

package ui

import (
	"image"
	"unsafe"
)

// maskRowAlignment is the byte boundary every row of a monochrome bitmap is
// padded to: Windows pads them to a WORD.
const maskRowAlignment = 2

// byteBits is the number of pixels one byte of a monochrome bitmap holds.
const byteBits = 8

// attentionIcon answers the tray icon marked for attention in the palette in
// force now, building it the first time it is asked for (FR-045). Zero means
// there is nothing to mark it with, which leaves the plain icon showing. It is
// called on the tray's own thread only, so the cache needs no lock.
func (tray *Tray) attentionIcon() uintptr {
	palette, drawable := tray.attention.palette()
	if !drawable {
		return 0
	}
	if icon, built := tray.marked[palette]; built {
		return icon
	}
	size, _, _ := pGetSystemMetrics.Call(smallIconWidth)
	icon := iconFrom(tray.attention.picture(int(size), palette))
	if icon == 0 {
		// Said once, since the zero is kept: a user who never sees the badge
		// has the tooltip and the menu to go on; the log says why.
		tray.log.Step("the tray icon could not be marked, so an incomplete restore shows in the tooltip and the menu only")
	}
	tray.marked[palette] = icon
	return icon
}

// forgetAttention destroys every attention icon built, which the shell does not
// own and so will not free.
func (tray *Tray) forgetAttention() {
	for palette, icon := range tray.marked {
		if icon != 0 {
			_, _, _ = pDestroyIcon.Call(icon)
		}
		delete(tray.marked, palette)
	}
}

// iconFrom builds an icon from a picture with its transparency; zero when
// Windows refuses any part of it. The colour carries the alpha, so the mask is
// all zeros: Windows draws a 32-bit icon from its alpha and ignores the mask,
// yet still requires one.
func iconFrom(picture *image.NRGBA) uintptr {
	side := int32(min(picture.Bounds().Dx(), picture.Bounds().Dy()))
	if side <= 0 {
		return 0
	}
	header := bitmapInfoHeader{
		width: side, height: -side, // negative: the rows run top down
		planes: bitmapPlanes, bitCount: bitsPerPixel, compression: biRGB,
	}
	header.size = uint32(unsafe.Sizeof(header))
	var bits unsafe.Pointer
	colour, _, _ := pCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&header)),
		dibRGBColours, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if colour == 0 || bits == nil {
		return 0
	}
	defer func() { _, _, _ = pDeleteObject.Call(colour) }()
	pixels := straight(picture)
	copy(unsafe.Slice((*byte)(bits), len(pixels)), pixels)

	rowBytes := (int(side) + maskRowAlignment*byteBits - 1) / (maskRowAlignment * byteBits) * maskRowAlignment
	blank := make([]byte, rowBytes*int(side))
	mask, _, _ := pCreateBitmap.Call(uintptr(side), uintptr(side), 1, 1,
		uintptr(unsafe.Pointer(&blank[0])))
	if mask == 0 {
		return 0
	}
	defer func() { _, _, _ = pDeleteObject.Call(mask) }()

	info := iconInfo{isIcon: 1, mask: mask, colour: colour}
	// CreateIconIndirect copies both bitmaps, so they are freed above either way.
	icon, _, _ := pCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	return icon
}
