package application

import (
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// A size is the width and height alone. The position is what the raw details
// are for; a person reading the entry wants to know how large, not where.
func TestSizeOfLeavesThePositionOut(t *testing.T) {
	t.Parallel()
	rect := domain.Rect{X: 3832, Y: 1696, Width: 3300, Height: 2000}
	if got, want := sizeOf(rect), "3300 × 2000"; got != want {
		t.Errorf("sizeOf answered %q, want %q", got, want)
	}
}
