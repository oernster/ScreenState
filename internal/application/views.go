package application

import (
	"fmt"

	"github.com/oernster/ScreenState/internal/domain"
)

// sizeSeparator stands between a width and a height where a person reads them.
const sizeSeparator = " × "

// PlacementView is one placement as the manager shows it. Every field is
// already words, because deciding how a rectangle reads is a judgement and this
// layer is where judgements live.
//
// It carries two readings of the same placement. Size and State are what a
// person reads first; Display and Rect are exactly what was recorded, kept so
// the manager can still say precisely what it captured when asked.
type PlacementView struct {
	// Display is the recorded monitor id.
	Display string
	// Rect is the recorded normal rectangle, coordinates and all.
	Rect string
	// Size is the normal rectangle's width and height alone.
	Size  string
	State string
}

// EntryView is one entry of a profile as the manager shows it.
type EntryView struct {
	// Application is the recorded identity, which is also what names it when
	// the user asks for the entry to be removed.
	Application string
	// Name is what the application is called, the line a person reads first.
	Name string
	// Program is the file the identity starts, beneath the name.
	Program string
	// Kind says how the application is recognised, so a reader can tell a path
	// from a packaged application's identity.
	Kind string
	// Running says the application should be running, whether or not any
	// placement is recorded for it.
	Running    bool
	Placements []PlacementView
}

// viewOf turns one entry into what the manager shows for it.
func viewOf(entry domain.Entry) EntryView {
	placements := make([]PlacementView, 0, len(entry.Placements))
	for _, placement := range entry.Placements {
		placements = append(placements, PlacementView{
			Display: placement.Display.String(),
			Rect:    placement.Rect.String(),
			Size:    sizeOf(placement.Rect),
			State:   placement.State.String(),
		})
	}
	return EntryView{
		Application: entry.Application.Value,
		Name:        entry.Application.Name(),
		Program:     entry.Application.Program(),
		Kind:        entry.Application.Kind.String(),
		Running:     entry.Running,
		Placements:  placements,
	}
}

// sizeOf says how large a rectangle is, without where it is.
func sizeOf(rect domain.Rect) string {
	return fmt.Sprintf("%d%s%d", rect.Width, sizeSeparator, rect.Height)
}
