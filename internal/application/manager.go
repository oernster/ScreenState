package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/oernster/ScreenState/internal/domain"
)

// ProfileSummary is one profile as the manager's list shows it: enough to draw
// a row, without the entries behind it.
type ProfileSummary struct {
	Name    string
	Default bool
	Entries int
}

// PlacementView is one placement as the manager shows it. Every field is
// already words, because deciding how a rectangle reads is a judgement and this
// layer is where judgements live.
type PlacementView struct {
	Display string
	Rect    string
	State   string
}

// EntryView is one entry of a profile as the manager shows it.
type EntryView struct {
	// Application is the recorded identity, which is also what names it when
	// the user asks for the entry to be removed.
	Application string
	// Kind says how the application is recognised, so a reader can tell a path
	// from a packaged application's identity.
	Kind string
	// Running says the application should be running, whether or not any
	// placement is recorded for it.
	Running    bool
	Placements []PlacementView
}

// Startup is the per-user registry entry that starts the agent at sign-in
// (FR-046), which the manager turns on and off (FR-053).
//
// It takes no context. A registry read answers immediately and has nothing to
// cancel, so a context here would be furniture that every caller has to supply
// and no implementation can honour.
type Startup interface {
	// Enabled reports whether the entry is present and names a real file.
	Enabled() (bool, error)
	// SetEnabled writes the entry or removes it.
	SetEnabled(enabled bool) error
}

// ManagerService is what the manager window asks. It holds no window and knows
// no toolkit, so every rule below is settled by a test rather than by opening
// it and looking.
type ManagerService struct {
	store   ProfileStore
	startup Startup
	log     Log
}

// NewManagerService returns a manager service over the given collaborators.
func NewManagerService(store ProfileStore, startup Startup, log Log) *ManagerService {
	return &ManagerService{store: store, startup: startup, log: log}
}

// Profiles lists the stored profiles for the manager, the default one marked
// and the rest in the order a reader expects to find them.
func (service *ManagerService) Profiles(ctx context.Context) ([]ProfileSummary, error) {
	names, err := service.store.Names(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing the profiles: %w", err)
	}
	sort.Slice(names, func(one, two int) bool {
		return strings.ToLower(names[one]) < strings.ToLower(names[two])
	})
	summaries := make([]ProfileSummary, 0, len(names))
	for _, name := range names {
		profile, err := service.store.Load(ctx, name)
		if err != nil {
			// A profile that cannot be read costs the user that profile rather
			// than the list. It is named in the log and left out of the list,
			// which is what the store already does for a file it excludes.
			service.log.Step(fmt.Sprintf("%s is not being listed: %v", name, err))
			continue
		}
		summaries = append(summaries, ProfileSummary{
			Name:    profile.Name,
			Default: profile.Default,
			Entries: len(profile.Entries),
		})
	}
	return summaries, nil
}

// Entries returns the entries of one profile as the manager shows them
// (EIR-002).
func (service *ManagerService) Entries(ctx context.Context, name string) ([]EntryView, error) {
	profile, err := service.store.Load(ctx, name)
	if err != nil {
		return nil, err
	}
	views := make([]EntryView, 0, len(profile.Entries))
	for _, entry := range profile.Entries {
		views = append(views, viewOf(entry))
	}
	return views, nil
}

// viewOf turns one entry into what the manager shows for it.
func viewOf(entry domain.Entry) EntryView {
	placements := make([]PlacementView, 0, len(entry.Placements))
	for _, placement := range entry.Placements {
		placements = append(placements, PlacementView{
			Display: placement.Display.String(),
			Rect:    placement.Rect.String(),
			State:   placement.State.String(),
		})
	}
	return EntryView{
		Application: entry.Application.Value,
		Kind:        entry.Application.Kind.String(),
		Running:     entry.Running,
		Placements:  placements,
	}
}

// Rename changes a profile's name, keeping everything else about it (FR-042).
//
// Two names differing only in case are one name to the user (FR-002), so a
// rename that only changes case is a rewrite of the same profile and must not
// delete anything afterwards: the store addresses a profile by its lowercased
// name, so the old file IS the new file.
func (service *ManagerService) Rename(ctx context.Context, from, to string) error {
	profile, err := service.store.Load(ctx, from)
	if err != nil {
		return err
	}
	renamed, err := domain.NewProfile(to, profile.Entries...)
	if err != nil {
		return err
	}
	renamed = renamed.WithDefault(profile.Default)

	sameProfile := strings.EqualFold(strings.TrimSpace(from), renamed.Name)
	if !sameProfile {
		taken, err := service.nameTaken(ctx, renamed.Name)
		if err != nil {
			return err
		}
		if taken {
			return fmt.Errorf("%w: %q", ErrProfileNameInUse, renamed.Name)
		}
	}
	if err := service.store.Save(ctx, renamed); err != nil {
		return fmt.Errorf("writing profile %q: %w", renamed.Name, err)
	}
	if !sameProfile {
		if err := service.store.Delete(ctx, profile.Name); err != nil {
			return fmt.Errorf("removing the old profile %q: %w", profile.Name, err)
		}
	}
	service.log.Step(fmt.Sprintf("profile %q was renamed to %q", profile.Name, renamed.Name))
	return nil
}

// Delete removes a profile. The confirmation FR-043 requires belongs to the
// window: by the time this is called the user has already been asked.
func (service *ManagerService) Delete(ctx context.Context, name string) error {
	if err := service.store.Delete(ctx, name); err != nil {
		return err
	}
	service.log.Step(fmt.Sprintf("profile %q was deleted", name))
	return nil
}

// SetDefault marks one profile as the one applied at sign-in and clears the
// mark from every other (FR-040).
//
// The clearing is done first. Marking first would leave two profiles marked if
// the run ended in between. Two marked profiles is a state nothing else in this
// product knows how to read; none marked is an ordinary one (FR-039).
func (service *ManagerService) SetDefault(ctx context.Context, name string) error {
	wanted, err := service.store.Load(ctx, name)
	if err != nil {
		return err
	}
	if err := service.clearDefaults(ctx, wanted.Name); err != nil {
		return err
	}
	if err := service.store.Save(ctx, wanted.WithDefault(true)); err != nil {
		return fmt.Errorf("marking %q as the default: %w", wanted.Name, err)
	}
	service.log.Step(fmt.Sprintf("profile %q is now the default", wanted.Name))
	return nil
}

// ClearDefault unmarks whichever profile is the default, leaving none. It is
// what makes the marking a toggle rather than a one-way door: FR-039 says an
// agent with no default applies nothing at sign-in, so having none is a choice
// a user is entitled to make.
func (service *ManagerService) ClearDefault(ctx context.Context) error {
	if err := service.clearDefaults(ctx, ""); err != nil {
		return err
	}
	service.log.Step("no profile is marked as the default")
	return nil
}

// clearDefaults unmarks every profile except the one named, which may be empty
// to unmark all of them. A profile that cannot be read is reported rather than
// skipped: leaving a second profile marked is the one outcome this must not
// produce quietly.
func (service *ManagerService) clearDefaults(ctx context.Context, except string) error {
	names, err := service.store.Names(ctx)
	if err != nil {
		return fmt.Errorf("listing the profiles: %w", err)
	}
	for _, name := range names {
		if strings.EqualFold(name, except) {
			continue
		}
		profile, err := service.store.Load(ctx, name)
		if err != nil {
			return fmt.Errorf("reading profile %q: %w", name, err)
		}
		if !profile.Default {
			continue
		}
		if err := service.store.Save(ctx, profile.WithDefault(false)); err != nil {
			return fmt.Errorf("unmarking profile %q: %w", name, err)
		}
	}
	return nil
}

// RemoveEntry drops one application from a profile (FR-042). A profile may end
// up with no entries at all, which is a profile that arranges nothing rather
// than an error: deleting it is a separate act the user asks for separately.
func (service *ManagerService) RemoveEntry(ctx context.Context, name, application string) error {
	profile, err := service.store.Load(ctx, name)
	if err != nil {
		return err
	}
	kept := make([]domain.Entry, 0, len(profile.Entries))
	for _, entry := range profile.Entries {
		if entry.Application.Value == application {
			continue
		}
		kept = append(kept, entry)
	}
	if len(kept) == len(profile.Entries) {
		return fmt.Errorf("%w: %q holds no entry for %q", ErrNoSuchEntry, profile.Name, application)
	}
	profile.Entries = kept
	if err := service.store.Save(ctx, profile); err != nil {
		return fmt.Errorf("writing profile %q: %w", profile.Name, err)
	}
	service.log.Step(fmt.Sprintf("%s was removed from profile %q", application, profile.Name))
	return nil
}

// StartsWithWindows reports whether the agent is set to run at sign-in
// (FR-053).
func (service *ManagerService) StartsWithWindows() (bool, error) {
	enabled, err := service.startup.Enabled()
	if err != nil {
		return false, fmt.Errorf("reading the sign-in entry: %w", err)
	}
	return enabled, nil
}

// SetStartsWithWindows turns the sign-in entry on or off (FR-053).
func (service *ManagerService) SetStartsWithWindows(enabled bool) error {
	if err := service.startup.SetEnabled(enabled); err != nil {
		return fmt.Errorf("writing the sign-in entry: %w", err)
	}
	service.log.Step(fmt.Sprintf("start with Windows is now %t", enabled))
	return nil
}

// nameTaken reports whether the store already holds a profile of that name,
// ignoring case (FR-002).
func (service *ManagerService) nameTaken(ctx context.Context, name string) (bool, error) {
	names, err := service.store.Names(ctx)
	if err != nil {
		return false, fmt.Errorf("listing the profiles: %w", err)
	}
	for _, stored := range names {
		if strings.EqualFold(stored, name) {
			return true, nil
		}
	}
	return false, nil
}
