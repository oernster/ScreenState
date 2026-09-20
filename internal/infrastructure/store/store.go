package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
)

// The errors this store raises, beyond the ones the application layer defines.
var (
	// ErrUnknownFormat is a profile written in a format version this build does
	// not understand. Such a file is left exactly as it is (DATA-003).
	ErrUnknownFormat = errors.New("profile was written in a newer format")
	// ErrUnreadable is a profile file that is not a profile: broken JSON, else
	// contents the domain refuses.
	ErrUnreadable = errors.New("profile could not be read")
)

// extension is what a stored profile is called.
const extension = ".json"

// Store keeps profiles as one file each under a directory of its own.
type Store struct {
	directory string
	log       application.Log
}

// Exclusion is a file in the store that is not being offered as a profile, with
// the reason. DATA-003 requires that a file in an unknown format is left alone,
// kept out of the list and the reason stated; this is how the manager states it.
type Exclusion struct {
	File   string
	Reason string
}

// New returns a store over a directory, creating it where it is not there yet.
func New(directory string, log application.Log) (*Store, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("preparing the profile store at %s: %w", directory, err)
	}
	return &Store{directory: directory, log: log}, nil
}

// Directory returns where the profiles are kept, which the manager shows the
// user and the setup program asks about before removing anything (DATA-005).
func (store *Store) Directory() string { return store.directory }

// Names lists the profiles that can be read, in a settled order.
//
// A file that cannot be read is left out rather than allowed to stop the
// listing: one unreadable profile costs the user that profile, never the rest.
func (store *Store) Names(ctx context.Context) ([]string, error) {
	profiles, _, err := store.readAll(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.Name)
	}
	sort.Slice(names, func(one, two int) bool {
		return strings.ToLower(names[one]) < strings.ToLower(names[two])
	})
	return names, nil
}

// Excluded returns the files in the store that are not being offered, each with
// the reason (DATA-003).
func (store *Store) Excluded(ctx context.Context) ([]Exclusion, error) {
	_, excluded, err := store.readAll(ctx)
	return excluded, err
}

// readAll reads every file in the store, returning the profiles it could read
// and the files it could not with the reason for each.
func (store *Store) readAll(ctx context.Context) ([]domain.Profile, []Exclusion, error) {
	// Checked before the directory is read as well as within it. Checking only
	// inside the loop let a cancelled call succeed over an empty store, which
	// is the one store where the loop never runs.
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	entries, err := os.ReadDir(store.directory)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the profile store: %w", err)
	}
	var profiles []domain.Profile
	var excluded []Exclusion
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), extension) {
			continue
		}
		profile, err := store.read(filepath.Join(store.directory, entry.Name()))
		if err != nil {
			// Recorded, not narrated. This runs on every listing; a run
			// that says the same thing three times teaches a reader to skim
			// the log. The caller states it once; DATA-003 asks that it be
			// stated, not that it be repeated.
			excluded = append(excluded, Exclusion{File: entry.Name(), Reason: err.Error()})
			continue
		}
		profiles = append(profiles, profile)
	}
	sort.Slice(excluded, func(one, two int) bool { return excluded[one].File < excluded[two].File })
	return profiles, excluded, nil
}

// read loads one file as a profile.
func (store *Store) read(path string) (domain.Profile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	profile, _, err := decode(raw)
	return profile, err
}

// Load reads one profile by name, ignoring case, since two profiles differing
// only in case would be one name to the user.
func (store *Store) Load(ctx context.Context, name string) (domain.Profile, error) {
	if err := ctx.Err(); err != nil {
		return domain.Profile{}, err
	}
	profile, err := store.read(store.pathFor(name))
	if errors.Is(err, ErrUnreadable) && errors.Is(err, os.ErrNotExist) {
		return domain.Profile{}, fmt.Errorf("%w: %q", application.ErrNoSuchProfile, name)
	}
	return profile, err
}

// Save writes a profile, replacing one of the same name.
//
// The write is atomic (FR-006): the bytes go to a temporary file beside the
// target and are then moved onto it, so an interruption leaves the previous
// profile or the new one and never half of either.
//
// Marking a profile as the default clears the mark from every other, which is
// FR-040 enforced where it cannot be forgotten rather than in the manager.
func (store *Store) Save(ctx context.Context, profile domain.Profile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	raw, err := encode(profile)
	if err != nil {
		return err
	}
	if err := store.write(store.pathFor(profile.Name), raw); err != nil {
		return err
	}
	if !profile.Default {
		return nil
	}
	return store.clearOtherDefaults(ctx, profile.Name)
}

// clearOtherDefaults unmarks every profile but the one just marked (FR-040).
func (store *Store) clearOtherDefaults(ctx context.Context, keep string) error {
	profiles, _, err := store.readAll(ctx)
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		if !profile.Default || strings.EqualFold(profile.Name, keep) {
			continue
		}
		raw, err := encode(profile.WithDefault(false))
		if err != nil {
			return err
		}
		if err := store.write(store.pathFor(profile.Name), raw); err != nil {
			return err
		}
		store.log.Step(fmt.Sprintf("profile %q is no longer the default", profile.Name))
	}
	return nil
}

// Delete removes a profile.
func (store *Store) Delete(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(store.pathFor(name)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %q", application.ErrNoSuchProfile, name)
		}
		return fmt.Errorf("deleting profile %q: %w", name, err)
	}
	return nil
}

// Default returns the profile marked as the one applied at sign-in, plus
// whether there is one. No default is an answer rather than a fault (FR-039).
//
// Where more than one file claims it, which FR-040 prevents but a hand-edited
// store could still hold, the first by name wins and the disagreement is
// recorded rather than passed over.
func (store *Store) Default(ctx context.Context) (domain.Profile, bool, error) {
	profiles, _, err := store.readAll(ctx)
	if err != nil {
		return domain.Profile{}, false, err
	}
	var marked []domain.Profile
	for _, profile := range profiles {
		if profile.Default {
			marked = append(marked, profile)
		}
	}
	if len(marked) == 0 {
		return domain.Profile{}, false, nil
	}
	sort.Slice(marked, func(one, two int) bool {
		return strings.ToLower(marked[one].Name) < strings.ToLower(marked[two].Name)
	})
	if len(marked) > 1 {
		store.log.Step(fmt.Sprintf(
			"%d profiles are marked as the default; %q was used",
			len(marked), marked[0].Name))
	}
	return marked[0], true, nil
}
