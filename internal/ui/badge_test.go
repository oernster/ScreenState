package ui

import (
	"image"
	"image/color"
	"testing"
)

// The tray's small icon is sixteen pixels a side at the reference scale.
const traySide = 16

var (
	badgePalette = SplashPalette{
		Panel:  Colour{R: 0xff, G: 0xff, B: 0xff},
		Danger: Colour{R: 0xbe, G: 0x12, B: 0x3c},
	}
	markColour = color.NRGBA{R: 0x00, G: 0x59, B: 0xa8, A: 0xff}
)

// aMark is a picture filled with one colour, standing in for the artwork.
func aMark(side int, fill color.NRGBA) *image.NRGBA {
	picture := image.NewNRGBA(image.Rect(0, 0, side, side))
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			picture.SetNRGBA(x, y, fill)
		}
	}
	return picture
}

func opaque(colour Colour) color.NRGBA {
	return color.NRGBA{R: colour.R, G: colour.G, B: colour.B, A: channelMax}
}

// FR-045: the badge sits in the bottom-right corner, danger ringed in panel,
// and leaves the rest of the mark as it was.
func TestTheBadgeIsDrawnInTheCornerOverTheMark(t *testing.T) {
	t.Parallel()
	mark := aMark(traySide, markColour)
	result := badged(mark, badgePalette)

	// The badge spans half the side, so its centre is three quarters across.
	centre := traySide * 3 / 4
	if got := result.NRGBAAt(centre, centre); got != opaque(badgePalette.Danger) {
		t.Errorf("the badge's centre is %+v", got)
	}
	// The pixel at the badge's right edge lies wholly in the ring.
	if got := result.NRGBAAt(traySide-1, centre); got != opaque(badgePalette.Panel) {
		t.Errorf("the badge's edge is %+v", got)
	}
	if got := result.NRGBAAt(0, 0); got != markColour {
		t.Errorf("the mark's far corner became %+v", got)
	}
	if got := mark.NRGBAAt(centre, centre); got != markColour {
		t.Error("drawing the badge changed the picture it was handed")
	}
}

// The badge is opaque where it covers a transparent part of the artwork, so it
// shows against any taskbar.
func TestTheBadgeIsOpaqueOverTransparency(t *testing.T) {
	t.Parallel()
	result := badged(aMark(traySide, color.NRGBA{}), badgePalette)
	centre := traySide * 3 / 4
	if got := result.NRGBAAt(centre, centre); got != opaque(badgePalette.Danger) {
		t.Errorf("the badge over transparency is %+v", got)
	}
	if got := result.NRGBAAt(0, 0); got.A != 0 {
		t.Errorf("the transparent corner became %+v", got)
	}
}

// A picture too small to carry a badge is answered as it was.
func TestAPictureTooSmallForABadgeIsLeftAlone(t *testing.T) {
	t.Parallel()
	result := badged(aMark(1, markColour), badgePalette)
	if got := result.NRGBAAt(0, 0); got != markColour {
		t.Fatalf("a one-pixel picture became %+v", got)
	}
}

func TestLayingAColourOverAPixel(t *testing.T) {
	t.Parallel()
	below := color.NRGBA{R: 1, G: 2, B: 3, A: 4}
	if got := over(below, badgePalette.Danger, 0); got != below {
		t.Errorf("no coverage changed the pixel to %+v", got)
	}
	got := over(color.NRGBA{}, badgePalette.Danger, 0.5)
	if got.R != badgePalette.Danger.R || got.A != channelMax/2+1 {
		t.Errorf("half coverage over nothing gave %+v", got)
	}
	if coverage(0, 0, 0, 0, 0) != 0 {
		t.Error("a disc with no radius covered a pixel")
	}
}

// Windows takes an icon's colour as blue, green, red then alpha, not
// multiplied by alpha.
func TestAnIconsBytesAreStraightBlueGreenRedAlpha(t *testing.T) {
	t.Parallel()
	half := color.NRGBA{R: 0x10, G: 0x20, B: 0x30, A: 0x80}
	got := straight(aMark(1, half))
	want := []byte{0x30, 0x20, 0x10, 0x80}
	if string(got) != string(want) {
		t.Fatalf("the bytes are % x, wanted % x", got, want)
	}
}

// The palette follows the theme in force; no artwork means nothing to draw.
func TestTheAttentionPaletteFollowsTheTheme(t *testing.T) {
	t.Parallel()
	if _, drawable := (Attention{}).palette(); drawable {
		t.Fatal("an attention with no artwork claimed it could draw")
	}
	themes := Themes{Light: badgePalette, Dark: SplashPalette{Danger: Colour{R: 1}}}
	logo := aMark(traySide, markColour)
	light, _ := Attention{Logo: logo, Themes: themes}.palette()
	dark, _ := Attention{Logo: logo, Themes: themes, Dark: func() bool { return true }}.palette()
	if light != themes.Light || dark != themes.Dark {
		t.Fatalf("light read %+v and dark read %+v", light, dark)
	}
	picture := Attention{Logo: logo, Themes: themes}.picture(traySide, light)
	if picture.Bounds().Dx() != traySide {
		t.Fatalf("the picture is %d pixels a side", picture.Bounds().Dx())
	}
}
