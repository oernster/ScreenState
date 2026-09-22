// Package store keeps the user's profiles on disk, one file each, under the
// signed-in user's own local application data directory (DATA-001, C-2).
//
// It is deliberately files rather than a database. A profile is a few hundred
// bytes that changes when the user says so, never concurrently. A file each means a
// profile that cannot be read costs the user that profile rather than all of
// them.
package store

import (
	"encoding/json"
	"fmt"

	"github.com/oernster/ScreenState/internal/domain"
)

// formatVersion is the version every profile is written in. It is stored in the
// file (DATA-002) so that a future version can tell what it is reading; a file
// from a future version is left alone rather than guessed at (DATA-003).
const formatVersion = 1

// document is a stored profile. The field names are the file format: changing
// one changes what every existing profile means, which is what the version is
// there to make visible.
type document struct {
	Format  int             `json:"format"`
	Name    string          `json:"name"`
	Default bool            `json:"default"`
	Entries []entryDocument `json:"entries"`
}

// entryDocument is one application within a stored profile.
type entryDocument struct {
	Application identityDocument    `json:"application"`
	Running     bool                `json:"running"`
	Placements  []placementDocument `json:"placements,omitempty"`
}

// identityDocument names an application. The kind is stored as the word the
// domain uses rather than as a number, so a person reading the file can tell
// what it says and a reordered constant cannot silently change its meaning.
type identityDocument struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	// ModelID is written only for a packaged application, whose path carries a
	// version (FR-071). A file written before it is read back without one.
	ModelID string `json:"modelId,omitempty"`
}

// placementDocument is where one window belongs.
type placementDocument struct {
	Display string       `json:"display"`
	Rect    rectDocument `json:"rect"`
	State   string       `json:"state"`
	// Rank is the placement's place in the recorded stacking order, 1 on top.
	// It is left out where none was recorded, so a file written before ranks
	// were recorded reads as holding none and an older build reading a newer
	// file ignores it (DATA-006).
	Rank int `json:"rank,omitempty"`
}

// rectDocument is a window's normal rectangle, as a position and a size.
type rectDocument struct {
	X      int32 `json:"x"`
	Y      int32 `json:"y"`
	Width  int32 `json:"width"`
	Height int32 `json:"height"`
}

// toDocument turns a profile into what is written.
func toDocument(profile domain.Profile) document {
	written := document{
		Format:  formatVersion,
		Name:    profile.Name,
		Default: profile.Default,
		Entries: make([]entryDocument, 0, len(profile.Entries)),
	}
	for _, entry := range profile.Entries {
		placements := make([]placementDocument, 0, len(entry.Placements))
		for _, placement := range entry.Placements {
			placements = append(placements, placementDocument{
				Display: placement.Display.MonitorID,
				Rect: rectDocument{
					X: placement.Rect.X, Y: placement.Rect.Y,
					Width: placement.Rect.Width, Height: placement.Rect.Height,
				},
				State: placement.State.String(),
				Rank:  placement.Rank,
			})
		}
		written.Entries = append(written.Entries, entryDocument{
			Application: identityDocument{
				Kind:    entry.Application.Kind.String(),
				Value:   entry.Application.Value,
				ModelID: entry.Application.ModelID,
			},
			Running:    entry.Running,
			Placements: placements,
		})
	}
	return written
}

// toProfile turns a stored document back into a profile, refusing anything the
// domain would not accept. A file that cannot become a valid profile is not a
// profile, however well formed its JSON is.
func (written document) toProfile() (domain.Profile, error) {
	entries := make([]domain.Entry, 0, len(written.Entries))
	for _, stored := range written.Entries {
		entry, err := stored.toEntry()
		if err != nil {
			return domain.Profile{}, err
		}
		entries = append(entries, entry)
	}
	profile, err := domain.NewProfile(written.Name, entries...)
	if err != nil {
		return domain.Profile{}, err
	}
	return profile.WithDefault(written.Default), nil
}

// toEntry turns one stored entry back into a domain entry.
func (stored entryDocument) toEntry() (domain.Entry, error) {
	kind, err := domain.ParseIdentityKind(stored.Application.Kind)
	if err != nil {
		return domain.Entry{}, err
	}
	application, err := domain.NewApplicationIdentity(kind, stored.Application.Value)
	if err != nil {
		return domain.Entry{}, err
	}
	application = application.WithModelID(stored.Application.ModelID)
	entry := domain.Entry{Application: application, Running: stored.Running}
	for _, stored := range stored.Placements {
		placement, err := stored.toPlacement()
		if err != nil {
			return domain.Entry{}, fmt.Errorf("%s: %w", application, err)
		}
		entry = entry.WithPlacement(placement)
	}
	return entry, nil
}

// toPlacement turns one stored placement back into a domain placement.
func (stored placementDocument) toPlacement() (domain.Placement, error) {
	display, err := domain.NewDisplayIdentity(stored.Display)
	if err != nil {
		return domain.Placement{}, err
	}
	state, err := domain.ParseShowState(stored.State)
	if err != nil {
		return domain.Placement{}, err
	}
	placement := domain.Placement{
		Display: display,
		Rect: domain.Rect{
			X: stored.Rect.X, Y: stored.Rect.Y,
			Width: stored.Rect.Width, Height: stored.Rect.Height,
		},
		State: state,
		Rank:  stored.Rank,
	}
	return placement, placement.Validate()
}

// encode renders a profile as the bytes that go to disk, indented because a
// user who opens one should be able to read it.
func encode(profile domain.Profile) ([]byte, error) {
	raw, err := json.MarshalIndent(toDocument(profile), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("writing profile %q: %w", profile.Name, err)
	}
	return append(raw, '\n'), nil
}

// decode reads a stored profile, reporting the format version separately so a
// caller can tell a file it must not touch from a file that is broken.
func decode(raw []byte) (domain.Profile, int, error) {
	var written document
	if err := json.Unmarshal(raw, &written); err != nil {
		return domain.Profile{}, 0, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	if written.Format != formatVersion {
		return domain.Profile{}, written.Format, fmt.Errorf("%w: format %d",
			ErrUnknownFormat, written.Format)
	}
	profile, err := written.toProfile()
	if err != nil {
		return domain.Profile{}, written.Format, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	return profile, written.Format, nil
}
