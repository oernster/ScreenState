// Package domain holds what a profile is and what makes one valid. It calls
// nothing: no Windows, no filesystem, no clock. Everything here can be reasoned
// about and tested without a desktop, which is the point of keeping it separate
// from the code that moves real windows.
package domain

import "fmt"

// ShowState is how a window is displayed, independently of its rectangle. A
// minimised or maximised window still has a normal rectangle underneath, which
// is what Rect carries and what a restore puts back first.
type ShowState uint8

const (
	// ShowNormal is a window occupying its normal rectangle.
	ShowNormal ShowState = iota
	// ShowMinimised is a window reduced to the taskbar.
	ShowMinimised
	// ShowMaximised is a window filling the work area of its display.
	ShowMaximised
)

// showStateNames gives each state one spelling, used for storage and for the
// report. The zero value is deliberately the ordinary case, so a placement that
// says nothing about show state means a normal window rather than an invalid
// one.
var showStateNames = map[ShowState]string{
	ShowNormal:    "normal",
	ShowMinimised: "minimised",
	ShowMaximised: "maximised",
}

// String returns the stored spelling of a show state.
func (state ShowState) String() string {
	name, known := showStateNames[state]
	if !known {
		return fmt.Sprintf("ShowState(%d)", uint8(state))
	}
	return name
}

// Valid reports whether a show state is one this product knows.
func (state ShowState) Valid() bool {
	_, known := showStateNames[state]
	return known
}

// ParseShowState turns a stored spelling back into a show state.
func ParseShowState(text string) (ShowState, error) {
	for state, name := range showStateNames {
		if name == text {
			return state, nil
		}
	}
	return ShowNormal, fmt.Errorf("%w: %q", ErrUnknownShowState, text)
}

// Rect is a window's normal rectangle in virtual desktop coordinates, the
// coordinate space Windows uses across every display. It is expressed as a
// position plus a size rather than as two corners, because a size cannot then
// be negative by accident.
//
// The coordinates are signed on purpose: a display left of or above the primary
// one has negative coordinates; three of the reference machine's four displays
// do. A maximised window also sits a border width outside its display,
// which is why a rectangle is never assumed to lie within one.
type Rect struct {
	X      int32
	Y      int32
	Width  int32
	Height int32
}

// Valid reports whether a rectangle has a positive size. Position is not
// checked here: a rectangle is valid in itself, while whether it lands on a
// connected display is a question about the desktop rather than about the
// rectangle.
func (rect Rect) Valid() bool {
	return rect.Width > 0 && rect.Height > 0
}

// Right returns the coordinate one past the rightmost column of the rectangle.
func (rect Rect) Right() int32 { return rect.X + rect.Width }

// Bottom returns the coordinate one past the bottom row of the rectangle.
func (rect Rect) Bottom() int32 { return rect.Y + rect.Height }

// Intersects reports whether two rectangles share any area. A restore uses it
// to decide which display a rectangle belongs to, so touching edges do not
// count: a window whose right edge meets a display's left edge is not on it.
func (rect Rect) Intersects(other Rect) bool {
	return rect.X < other.Right() && other.X < rect.Right() &&
		rect.Y < other.Bottom() && other.Y < rect.Bottom()
}

// Intersection returns the rectangle two rectangles share. Where they share no
// area the result has no size, which Valid reports as invalid, so a caller that
// forgets to check cannot mistake an empty overlap for a real one.
func (rect Rect) Intersection(other Rect) Rect {
	left := max(rect.X, other.X)
	top := max(rect.Y, other.Y)
	right := min(rect.Right(), other.Right())
	bottom := min(rect.Bottom(), other.Bottom())
	if right <= left || bottom <= top {
		return Rect{X: left, Y: top}
	}
	return Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}

// Area returns how much surface a rectangle covers, zero for one with no size.
// It answers in 64 bits because two 32 bit edges multiply beyond 32.
//
// It exists so a window straddling two displays can be said to belong to one of
// them: Windows itself judges by the larger share rather than by which display
// holds the title bar, so a restore that judged differently would disagree with
// what the user sees when they maximise that window by hand.
func (rect Rect) Area() int64 {
	if !rect.Valid() {
		return 0
	}
	return int64(rect.Width) * int64(rect.Height)
}

// String renders a rectangle the way the spike's logs did, so a report and a
// measurement can be compared by eye.
func (rect Rect) String() string {
	return fmt.Sprintf("x=%d y=%d w=%d h=%d", rect.X, rect.Y, rect.Width, rect.Height)
}
