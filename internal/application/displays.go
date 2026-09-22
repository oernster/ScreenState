package application

import "github.com/oernster/ScreenState/internal/domain"

// displaySet is the connected displays as they stood at one moment, with the
// questions a capture and a restore both ask of them.
//
// It is a value rather than a service on purpose: a restore reads the displays
// once per pass and then answers every question from that reading, so a display
// disconnected half way through a pass cannot give two entries two different
// answers within it.
type displaySet struct {
	displays []Display
	// names says where each display sits, from this same reading, keyed as
	// displayNames keys it.
	names map[string]string
}

// newDisplaySet returns the displays as a set, reporting ErrNoDisplays when
// there are none. Nothing can be placed without a display; a placement
// computed against none would be a guess at coordinates.
func newDisplaySet(displays []Display) (displaySet, error) {
	if len(displays) == 0 {
		return displaySet{}, ErrNoDisplays
	}
	return displaySet{displays: displays, names: displayNames(displays)}, nil
}

// name answers where a display sits in this reading; that it is not connected
// where it is not in it.
func (set displaySet) name(identity domain.DisplayIdentity) string {
	if name, connected := set.names[displayKey(identity)]; connected {
		return name
	}
	return displayGone
}

// primary returns the display a placement falls back to. Where Windows names
// none, the first is taken: a fallback that exists beats a correct refusal,
// since the alternative is leaving the window where the user cannot reach it.
func (set displaySet) primary() Display {
	for _, display := range set.displays {
		if display.Primary {
			return display
		}
	}
	return set.displays[0]
}

// find returns the connected display of that identity, plus whether it is
// connected at all.
func (set displaySet) find(identity domain.DisplayIdentity) (Display, bool) {
	for _, display := range set.displays {
		if display.Identity.Equal(identity) {
			return display, true
		}
	}
	return Display{}, false
}

// holding returns the display a rectangle belongs to, which is the one sharing
// the largest area with it. A window straddling two displays belongs to the one
// holding more of it, which is how Windows itself decides where to maximise it.
//
// The second result is false for a rectangle sharing area with no display at
// all, which is an off-screen window rather than a fault.
func (set displaySet) holding(rect domain.Rect) (Display, bool) {
	var best Display
	var bestArea int64
	for _, display := range set.displays {
		area := display.Bounds.Intersection(rect).Area()
		if area > bestArea {
			best, bestArea = display, area
		}
	}
	return best, bestArea > 0
}

// resolve returns the display a placement should be applied to, plus whether
// that is not the display the placement names.
//
// FR-031: a placement naming a display that is not connected is applied to the
// primary display and the substitution is recorded by the caller. Saying so
// matters as much as doing it, since a window that has silently moved screens
// looks like the product getting it wrong.
func (set displaySet) resolve(placement domain.Placement) (Display, bool) {
	if display, connected := set.find(placement.Display); connected {
		return display, false
	}
	return set.primary(), true
}

// fit returns a rectangle of the same size placed on the given display, moved
// only as far as it must be to lie within that display (FR-032).
//
// The size is kept whole wherever it fits, because a window silently resized is
// a window the user has to put right by hand. A window larger than the display
// is clamped to it in that dimension, since there is nowhere else for it to go.
func fit(rect domain.Rect, display Display) domain.Rect {
	area := display.WorkArea
	if !area.Valid() {
		area = display.Bounds
	}
	fitted := rect
	fitted.Width = min(rect.Width, area.Width)
	fitted.Height = min(rect.Height, area.Height)
	fitted.X = min(max(rect.X, area.X), area.Right()-fitted.Width)
	fitted.Y = min(max(rect.Y, area.Y), area.Bottom()-fitted.Height)
	return fitted
}

// place returns where a recorded placement lands on the displays as they now
// stand: on the display it names where that is connected, on the primary
// display where it is not. It lies within that display either way. It answers
// the display used too, plus whether that one was substituted.
//
// A placement moved to another display keeps its size and is slid into that
// display rather than being scaled or centred. It cannot be carried across by
// the offset between the two displays, because the display it was recorded
// against is the one that is no longer there, so its origin cannot be read.
func (set displaySet) place(placement domain.Placement, recorded domain.Rect) (domain.Rect, Display, bool) {
	display, substituted := set.resolve(placement)
	return fit(recorded, display), display, substituted
}
