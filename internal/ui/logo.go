package ui

import (
	"image"
	"image/color"
)

// The logo is drawn from the master artwork (EIR-003), reduced to the size a
// display needs and never enlarged: a mark scaled up from a small copy is
// blurred, which is why the master is the one used.

// shrink reduces an image to size by size pixels by averaging every source
// pixel that falls in each target pixel, which keeps thin strokes rather than
// dropping them the way picking one source pixel would. A size at or above the
// source's is answered at the source's own size: this function never enlarges.
func shrink(source image.Image, size int) *image.NRGBA {
	bounds := source.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	if size <= 0 || side <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, 0, 0))
	}
	size = min(size, side)
	result := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		top, bottom := y*side/size, (y+1)*side/size
		for x := 0; x < size; x++ {
			left, right := x*side/size, (x+1)*side/size
			result.SetNRGBA(x, y, average(source, bounds.Min, left, top, right, bottom))
		}
	}
	return result
}

// average is the mean of a block of source pixels, weighted by alpha so a
// transparent pixel's colour does not tint the edge of the mark.
func average(source image.Image, origin image.Point, left, top, right, bottom int) color.NRGBA {
	var red, green, blue, alpha, count uint64
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			r, g, b, a := source.At(origin.X+x, origin.Y+y).RGBA()
			// RGBA answers premultiplied values, which is what weighting by
			// alpha needs.
			red, green, blue, alpha = red+uint64(r), green+uint64(g), blue+uint64(b), alpha+uint64(a)
			count++
		}
	}
	if count == 0 || alpha == 0 {
		return color.NRGBA{}
	}
	const channelMax = 0xffff
	const byteShift = 8
	return color.NRGBA{
		R: uint8(red * channelMax / alpha >> byteShift),
		G: uint8(green * channelMax / alpha >> byteShift),
		B: uint8(blue * channelMax / alpha >> byteShift),
		A: uint8(alpha / count >> byteShift),
	}
}

// premultiplied answers an image as the rows of blue, green, red and alpha
// bytes, colour multiplied by alpha, top row first, that Windows draws a
// transparent bitmap from.
func premultiplied(picture *image.NRGBA) []byte {
	bounds := picture.Bounds()
	const bytesPerPixel = 4
	const channelMax = 0xff
	out := make([]byte, 0, bounds.Dx()*bounds.Dy()*bytesPerPixel)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := picture.NRGBAAt(x, y)
			scale := func(channel uint8) byte {
				return byte(uint32(channel) * uint32(pixel.A) / channelMax)
			}
			out = append(out, scale(pixel.B), scale(pixel.G), scale(pixel.R), pixel.A)
		}
	}
	return out
}
