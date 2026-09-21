package ui

import (
	"image"
	"image/color"
	"image/draw"
)

// The tray icon asks for attention when the last restore left something
// outstanding (FR-045). There is no second piece of artwork for that: the badge
// is drawn in code over the product's own mark, a disc in the palette's danger
// colour ringed in its panel colour, so it stands clear of the mark and of any
// taskbar behind it.

// badgeShare is the part of the icon's side the badge spans: a half, so it
// reads at the tray's sixteen pixels without hiding the mark behind it.
const badgeShare = 2

// ringShare is the part of the badge's width its ring takes.
const ringShare = 6

// badgeSamples is how many samples a side each pixel is measured at, so the
// disc's edge is smoothed rather than stepped at so few pixels.
const badgeSamples = 4

// half finds the middle of a pixel and rounds a channel to the nearest value.
const half = 0.5

// channelMax is the value of a fully present colour channel or alpha.
const channelMax = 0xff

// Attention is what the tray draws its attention icon from: the product's
// artwork, its palette and whether the dark theme is in force. The zero value
// draws nothing, so the tray keeps its plain icon rather than failing.
type Attention struct {
	Logo   image.Image
	Themes Themes
	Dark   func() bool
}

// palette is the palette in force now, plus whether there is anything to draw.
func (attention Attention) palette() (SplashPalette, bool) {
	if attention.Logo == nil {
		return SplashPalette{}, false
	}
	if attention.Dark != nil && attention.Dark() {
		return attention.Themes.Dark, true
	}
	return attention.Themes.Light, true
}

// picture is the artwork reduced to size pixels a side with the badge on it.
func (attention Attention) picture(size int, palette SplashPalette) *image.NRGBA {
	return badged(shrink(attention.Logo, size), palette)
}

// badged returns a copy of the picture carrying the attention badge in its
// bottom-right corner. The picture is left as it was.
func badged(picture *image.NRGBA, palette SplashPalette) *image.NRGBA {
	bounds := picture.Bounds()
	result := image.NewNRGBA(bounds)
	draw.Draw(result, bounds, picture, bounds.Min, draw.Src)
	side := min(bounds.Dx(), bounds.Dy())
	width := side / badgeShare
	if width <= 0 {
		return result
	}
	ring := max(width/ringShare, 1)
	outer := float64(width) / 2
	inner := outer - float64(ring)
	centreX := float64(bounds.Max.X) - outer
	centreY := float64(bounds.Max.Y) - outer
	for y := bounds.Max.Y - width; y < bounds.Max.Y; y++ {
		for x := bounds.Max.X - width; x < bounds.Max.X; x++ {
			pixel := result.NRGBAAt(x, y)
			pixel = over(pixel, palette.Panel, coverage(x, y, centreX, centreY, outer))
			pixel = over(pixel, palette.Danger, coverage(x, y, centreX, centreY, inner))
			result.SetNRGBA(x, y, pixel)
		}
	}
	return result
}

// coverage is the share of a pixel lying inside a disc, measured on a grid of
// samples across it.
func coverage(x, y int, centreX, centreY, radius float64) float64 {
	if radius <= 0 {
		return 0
	}
	inside := 0
	for row := 0; row < badgeSamples; row++ {
		for column := 0; column < badgeSamples; column++ {
			dx := float64(x) + (float64(column)+half)/badgeSamples - centreX
			dy := float64(y) + (float64(row)+half)/badgeSamples - centreY
			if dx*dx+dy*dy <= radius*radius {
				inside++
			}
		}
	}
	return float64(inside) / (badgeSamples * badgeSamples)
}

// over lays an opaque colour over a pixel at the given coverage, both in
// straight alpha.
func over(below color.NRGBA, colour Colour, share float64) color.NRGBA {
	if share <= 0 {
		return below
	}
	under := float64(below.A) / channelMax * (1 - share)
	alpha := share + under
	mix := func(top, bottom uint8) uint8 {
		return uint8((float64(top)*share+float64(bottom)*under)/alpha + half)
	}
	return color.NRGBA{
		R: mix(colour.R, below.R),
		G: mix(colour.G, below.G),
		B: mix(colour.B, below.B),
		A: uint8(alpha*channelMax + half),
	}
}

// straight answers a picture as the rows of blue, green, red and alpha bytes,
// top row first and colour not multiplied by alpha, which is how Windows takes
// the colour of a 32-bit icon.
func straight(picture *image.NRGBA) []byte {
	bounds := picture.Bounds()
	const bytesPerPixel = 4
	out := make([]byte, 0, bounds.Dx()*bounds.Dy()*bytesPerPixel)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := picture.NRGBAAt(x, y)
			out = append(out, pixel.B, pixel.G, pixel.R, pixel.A)
		}
	}
	return out
}
