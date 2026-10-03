package store

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	profiles, _, err := store.scan(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.profile.Name)
	}
	sort.Slice(names, func(one, two int) bool {
		return strings.ToLower(names[one]) < strings.ToLower(names[two])
	})
	return names, nil
}

// Unreadable returns the files in the store that are not being offered, each
// with the reason (NFR-REL-002, DATA-003). The manager states them and the log
// names them once a run; the files themselves are never touched.
func (store *Store) Unreadable(ctx context.Context) ([]application.UnreadableProfile, error) {
	_, excluded, err := store.scan(ctx)
	return excluded, err
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
	profiles, _, err := store.scan(ctx)
	if err != nil {
		return domain.Profile{}, err
	}
	if found, ok := find(profiles, name); ok {
		return found.profile, nil
	}
	return domain.Profile{}, store.unoffered(name)
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
	profiles, _, err := store.scan(ctx)
	if err != nil {
		return err
	}
	path, err := store.target(profiles, profile.Name)
	if err != nil {
		return err
	}
	if err := store.write(path, raw); err != nil {
		return err
	}
	if !profile.Default {
		return nil
	}
	return store.clearOtherDefaults(ctx, profile.Name)
}

// clearOtherDefaults unmarks every profile but the one just marked (FR-040).
func (store *Store) clearOtherDefaults(ctx context.Context, keep string) error {
	profiles, _, err := store.scan(ctx)
	if err != nil {
		return err
	}
	for _, each := range profiles {
		profile := each.profile
		if !profile.Default || strings.EqualFold(profile.Name, keep) {
			continue
		}
		raw, err := encode(profile.WithDefault(false))
		if err != nil {
			return err
		}
		if err := store.write(each.path, raw); err != nil {
			return err
		}
		store.log.Step(fmt.Sprintf("profile %q is no longer the default", profile.Name))
	}
	return nil
}

// Delete removes a profile.
func (store *Store) Delete(ctx context.Context, name string) error {
	profiles, _, err := store.scan(ctx)
	if err != nil {
		return err
	}
	found, ok := find(profiles, name)
	if !ok {
		return store.unoffered(name)
	}
	if err := os.Remove(found.path); err != nil {
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
	profiles, _, err := store.scan(ctx)
	if err != nil {
		return domain.Profile{}, false, err
	}
	var marked []domain.Profile
	for _, each := range profiles {
		if each.profile.Default {
			marked = append(marked, each.profile)
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
