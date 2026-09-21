package domain

import (
	"fmt"
	"strings"
)

// Placement is where one window belongs: which display, what rectangle it
// occupies when neither minimised nor maximised, plus how it is shown.
//
// The rectangle is the NORMAL one even for a maximised window. Maximising puts
// a window on the display its normal rectangle sits on, so a restore sets the
// rectangle first and maximises second; storing only "maximised" would lose
// which display that is.
type Placement struct {
	Display DisplayIdentity
	Rect    Rect
	State   ShowState
}

// Validate reports why a placement cannot be used; nil when it can.
func (placement Placement) Validate() error {
	if err := placement.Display.Validate(); err != nil {
		return err
	}
	if !placement.Rect.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidRect, placement.Rect)
	}
	if !placement.State.Valid() {
		return fmt.Errorf("%w: %d", ErrUnknownShowState, uint8(placement.State))
	}
	return nil
}

// Entry is one application within a profile: which application, whether it
// should be running, plus where its windows belong.
//
// Placements is a list rather than a single value because one application can
// own several windows, measured on 2026-09-19 when a Notepad window joined a
// process that already held another. An entry with no placements is meaningful:
// it says the application should be running without saying where.
type Entry struct {
	Application ApplicationIdentity
	Running     bool
	Placements  []Placement
}

// WithPlacement returns a copy of the entry carrying one more placement. The
// original is left alone, so a caller holding an entry cannot have it changed
// underneath.
func (entry Entry) WithPlacement(placement Placement) Entry {
	placements := make([]Placement, len(entry.Placements), len(entry.Placements)+1)
	copy(placements, entry.Placements)
	entry.Placements = append(placements, placement)
	return entry
}

// WithRunning returns a copy of the entry recording whether the application
// should be running.
func (entry Entry) WithRunning(running bool) Entry {
	entry.Running = running
	return entry
}

// Validate reports why an entry cannot be used; nil when it can.
func (entry Entry) Validate() error {
	if err := entry.Application.Validate(); err != nil {
		return err
	}
	for index, placement := range entry.Placements {
		if err := placement.Validate(); err != nil {
			return fmt.Errorf("%s placement %d: %w", entry.Application, index, err)
		}
	}
	return nil
}

// Profile is a named end state: the applications that should be running and
// where their windows belong. It describes what the desktop should look like,
// never how to get there, which is what lets a restore place each entry
// independently of the others.
type Profile struct {
	Name    string
	Default bool
	Entries []Entry
}

// NewProfile returns a validated profile.
func NewProfile(name string, entries ...Entry) (Profile, error) {
	profile := Profile{Name: strings.TrimSpace(name), Entries: entries}
	return profile, profile.Validate()
}

// WithEntry returns a copy of the profile carrying one more entry.
func (profile Profile) WithEntry(entry Entry) Profile {
	entries := make([]Entry, len(profile.Entries), len(profile.Entries)+1)
	copy(entries, profile.Entries)
	profile.Entries = append(entries, entry)
	return profile
}

// WithDefault returns a copy of the profile marked as the one applied after
// sign-in. Passing false unmarks it instead.
func (profile Profile) WithDefault(isDefault bool) Profile {
	profile.Default = isDefault
	return profile
}

// Find returns the entry naming an application, plus whether there was one. A
// packaged application is found by its model id once an update has moved its
// path (FR-071).
func (profile Profile) Find(application ApplicationIdentity) (Entry, bool) {
	for _, entry := range profile.Entries {
		if entry.Application.Recognises(application) {
			return entry, true
		}
	}
	return Entry{}, false
}

// Validate reports why a profile cannot be used; nil when it can.
//
// An application may appear only once. Two entries for one application would
// give a restore two answers to the same question, with nothing to say which
// wins; the several windows of one application belong in one entry's
// placements.
func (profile Profile) Validate() error {
	if strings.TrimSpace(profile.Name) == "" {
		return ErrEmptyProfileName
	}
	seen := make(map[string]struct{}, len(profile.Entries))
	for _, entry := range profile.Entries {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("profile %q: %w", profile.Name, err)
		}
		key := strings.ToLower(entry.Application.String())
		if _, already := seen[key]; already {
			return fmt.Errorf("%w: profile %q names %s twice",
				ErrDuplicateEntry, profile.Name, entry.Application)
		}
		seen[key] = struct{}{}
	}
	return nil
}
