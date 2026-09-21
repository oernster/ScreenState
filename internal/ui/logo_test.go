package ui

import (
	"image"
	"image/color"
	"testing"
)

// square answers a size by size image of one colour.
func square(size int, fill color.NRGBA) *image.NRGBA {
	picture := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			picture.SetNRGBA(x, y, fill)
		}
	}
	return picture
}

func TestTheLogoIsShrunkToTheSizeAsked(t *testing.T) {
	t.Parallel()
	red := color.NRGBA{R: 0xff, A: 0xff}
	got := shrink(square(8, red), 2)
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 2 {
		t.Fatalf("shrunk to %v", got.Bounds())
	}
	if got.NRGBAAt(1, 1) != red {
		t.Fatalf("a solid colour came out as %+v", got.NRGBAAt(1, 1))
	}
}

// The master is never enlarged (EIR-003): asking for more than it has answers
// what it has.
func TestTheLogoIsNeverEnlarged(t *testing.T) {
	t.Parallel()
	got := shrink(square(4, color.NRGBA{A: 0xff}), 64)
	if got.Bounds().Dx() != 4 {
		t.Fatalf("a 4 pixel image asked for 64 came back %v", got.Bounds())
	}
}

// A transparent neighbour does not tint the mark's edge: averaging a red pixel
// with a transparent black one gives half-transparent red, not dark red.
func TestTransparencyDoesNotDarkenTheEdge(t *testing.T) {
	t.Parallel()
	// The artwork is square, so the test image is: a red left column beside a
	// transparent right one.
	picture := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		picture.SetNRGBA(0, y, color.NRGBA{R: 0xff, A: 0xff})
		picture.SetNRGBA(1, y, color.NRGBA{})
	}
	got := shrink(picture, 1).NRGBAAt(0, 0)
	if got.R != 0xff || got.A == 0 || got.A == 0xff {
		t.Fatalf("the edge came out as %+v", got)
	}
}

func TestNothingToShrinkAnswersAnEmptyImage(t *testing.T) {
	t.Parallel()
	if got := shrink(image.NewNRGBA(image.Rect(0, 0, 0, 0)), 8); got.Bounds().Dx() != 0 {
		t.Fatalf("an empty source answered %v", got.Bounds())
	}
	if got := shrink(square(4, color.NRGBA{A: 0xff}), 0); got.Bounds().Dx() != 0 {
		t.Fatalf("a size of zero answered %v", got.Bounds())
	}
}

func TestTheBitmapIsBlueGreenRedAlphaPremultiplied(t *testing.T) {
	t.Parallel()
	picture := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	picture.SetNRGBA(0, 0, color.NRGBA{R: 0xff, G: 0x80, B: 0x00, A: 0x80})
	got := premultiplied(picture)
	want := []byte{0x00, 0x40, 0x80, 0x80}
	if string(got) != string(want) {
		t.Fatalf("got % x, want % x", got, want)
	}
}
