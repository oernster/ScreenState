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

// ErrFileInTheWay is a file already sitting where a new profile would be
// written that the store cannot read as a profile. It is left exactly as it is
// (DATA-003) and the profile is not written.
var ErrFileInTheWay = errors.New("a file the store cannot read already has that profile's file name")

// stored is a profile together with the file it was read from. Every operation
// on a profile acts on that file, never on a filename rebuilt from the name, so
// what the manager lists is what sign-in applies, what a delete removes and
// what clearing a default rewrites (S-1, S-2).
type stored struct {
	profile domain.Profile
	path    string
}

// namesItsOwnFile reports whether a file's name is the one the store gives the
// profile it holds, now or as builds up to 1.3.0 named it. Windows does not
// tell the case of a filename apart, so neither does this.
func namesItsOwnFile(file, name string) bool {
	base := strings.TrimSuffix(file, filepath.Ext(file))
	return strings.EqualFold(base, escape(name)) || strings.EqualFold(base, legacyEscape(name))
}

// scan reads every file in the store. A file whose name is not the one its
// profile is kept under (renamed or copied by hand) and every file of two or
// more holding one name are not offered: each is named with the reason, the
// way a file that cannot be read is, so nothing is applied at sign-in that the
// manager does not show.
func (store *Store) scan(ctx context.Context) ([]stored, []application.UnreadableProfile, error) {
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
	var readable []stored
	var excluded []application.UnreadableProfile
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), extension) {
			continue
		}
		path := filepath.Join(store.directory, entry.Name())
		profile, err := store.read(path)
		if err != nil {
			// Recorded, not narrated. This runs on every listing; a run
			// that says the same thing three times teaches a reader to skim
			// the log. The caller states it once; DATA-003 asks that it be
			// stated, not that it be repeated.
			excluded = append(excluded, application.UnreadableProfile{File: entry.Name(), Reason: err.Error()})
			continue
		}
		if !namesItsOwnFile(entry.Name(), profile.Name) {
			excluded = append(excluded, application.UnreadableProfile{File: entry.Name(), Reason: fmt.Sprintf(
				"%v: it holds the profile %q, which is kept as %s; a file renamed or copied by hand is not offered",
				ErrUnreadable, profile.Name, escape(profile.Name)+extension)})
			continue
		}
		readable = append(readable, stored{profile: profile, path: path})
	}
	readable, excluded = withoutSharedNames(readable, excluded)
	sort.Slice(excluded, func(one, two int) bool { return excluded[one].File < excluded[two].File })
	return readable, excluded, nil
}

// withoutSharedNames moves every file holding a name another file also holds
// out of the readable ones: which of them is the profile is the user's call.
func withoutSharedNames(readable []stored, excluded []application.UnreadableProfile) ([]stored, []application.UnreadableProfile) {
	kept := readable[:0:0]
	for index, candidate := range readable {
		var others []string
		for other, rival := range readable {
			if other != index && strings.EqualFold(rival.profile.Name, candidate.profile.Name) {
				others = append(others, filepath.Base(rival.path))
			}
		}
		if len(others) == 0 {
			kept = append(kept, candidate)
			continue
		}
		excluded = append(excluded, application.UnreadableProfile{File: filepath.Base(candidate.path), Reason: fmt.Sprintf(
			"%v: %s holds the same profile name %q", ErrUnreadable, strings.Join(others, ", "), candidate.profile.Name)})
	}
	return kept, excluded
}

// find returns the stored profile of that name, ignoring case.
func find(profiles []stored, name string) (stored, bool) {
	for _, candidate := range profiles {
		if strings.EqualFold(candidate.profile.Name, strings.TrimSpace(name)) {
			return candidate, true
		}
	}
	return stored{}, false
}

// occupant returns a file sitting where a profile of that name would be, now
// or as an earlier build named it, whatever it holds.
func (store *Store) occupant(name string) (string, bool) {
	for _, base := range []string{escape(name), legacyEscape(name)} {
		path := filepath.Join(store.directory, base+extension)
		if _, err := os.Lstat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

// unoffered explains why a name that is not among the readable profiles is not
// simply absent: a file sits at its file name that is not offered.
func (store *Store) unoffered(name string) error {
	path, there := store.occupant(name)
	if !there {
		return fmt.Errorf("%w: %q", application.ErrNoSuchProfile, name)
	}
	if _, err := store.read(path); err != nil {
		return err
	}
	return fmt.Errorf("%w: %s is not offered as %q", ErrUnreadable, filepath.Base(path), name)
}

// target returns the file a profile is written to: the file it was read from
// where it is stored already, else its own file name where nothing sits there.
// A file in the way is never written over (FR-002, DATA-003).
func (store *Store) target(profiles []stored, name string) (string, error) {
	if existing, found := find(profiles, name); found {
		return existing.path, nil
	}
	path := store.pathFor(name)
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return path, nil
	} else if err != nil {
		return "", fmt.Errorf("checking %s: %w", filepath.Base(path), err)
	}
	if held, err := store.read(path); err == nil {
		return "", fmt.Errorf("%w: %q: %s already holds the profile %q",
			application.ErrProfileNameInUse, name, filepath.Base(path), held.Name)
	}
	return "", fmt.Errorf("%w: %s is left as it is", ErrFileInTheWay, filepath.Base(path))
}
