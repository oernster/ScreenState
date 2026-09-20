// Package settings keeps the small number of choices this product remembers
// between runs: whether the update check is wanted (FR-059), which released
// version the user has chosen to pass over (FR-058) and what a restore does
// with the windows a profile does not name (FR-064).
//
// It is deliberately separate from the profile store. A profile is the user's
// work and a setting is a preference; a file holding both would mean an
// uninstall that offers to forget one had to offer to forget the other.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/oernster/ScreenState/internal/infrastructure/store"
)

// FileName names the settings file, which sits beside the profiles folder in
// the same place the log does.
const FileName = "settings.json"

// document is the settings file's contents.
//
// UpdateCheck is a pointer so that a file written before the setting existed,
// or one a user has edited, can be told from a file that says no. Absent means
// the user has not chosen, which is on: a check they never turned off is one
// they never declined.
//
// CloseUnnamedWindows needs no pointer: absent means minimise, which is what a
// user who has chosen nothing gets and is the arm that asks nothing of any
// application. The name in the file says what the setting does in the words the
// manager uses, while the Go name beside it keeps the word the restore uses for
// a window no profile names.
type document struct {
	UpdateCheck         *bool  `json:"updateCheck,omitempty"`
	SkippedVersion      string `json:"skippedVersion,omitempty"`
	CloseUnnamedWindows bool   `json:"closeUnnamedWindows,omitempty"`
}

// Preferences is the settings file as the application layer sees it.
//
// Every read goes to disk rather than to a value held here. The file is small,
// it is read when a window opens or a check runs and never in a loop; a cached
// copy is how a setting changed in one place comes to be stale in another.
type Preferences struct {
	// mutex serialises the read, change and write of the whole document, so two
	// settings changed at once cannot each write the other's old value back.
	mutex sync.Mutex
	path  string
}

// New returns the preferences kept in the given directory, creating nothing:
// a settings file that does not exist yet is a user who has chosen nothing,
// which is an answer rather than a fault.
func New(directory string) *Preferences {
	return &Preferences{path: filepath.Join(directory, FileName)}
}

// Default returns the preferences kept beside this user's profiles.
func Default() (*Preferences, error) {
	profiles, err := store.DefaultDirectory()
	if err != nil {
		return nil, err
	}
	return New(filepath.Dir(profiles)), nil
}

// UpdateCheckEnabled reports whether the update check is wanted. It is on
// where the file says nothing, since a check the user has never declined is one
// they have not turned off.
func (prefs *Preferences) UpdateCheckEnabled() (bool, error) {
	prefs.mutex.Lock()
	defer prefs.mutex.Unlock()
	held, err := prefs.read()
	if err != nil {
		return false, err
	}
	if held.UpdateCheck == nil {
		return true, nil
	}
	return *held.UpdateCheck, nil
}

// SetUpdateCheckEnabled records whether the update check is wanted (FR-059).
func (prefs *Preferences) SetUpdateCheckEnabled(enabled bool) error {
	return prefs.change(func(held *document) { held.UpdateCheck = &enabled })
}

// CloseStrangers reports whether a restore should ask the windows a profile
// does not name to close rather than minimising them (FR-064). It is off where
// the file says nothing: putting a window away is the user's instruction, while
// closing one is a decision only they can make.
func (prefs *Preferences) CloseStrangers() (bool, error) {
	prefs.mutex.Lock()
	defer prefs.mutex.Unlock()
	held, err := prefs.read()
	if err != nil {
		return false, err
	}
	return held.CloseUnnamedWindows, nil
}

// SetCloseStrangers records what to do with the windows a profile does not name
// (FR-064).
func (prefs *Preferences) SetCloseStrangers(closing bool) error {
	return prefs.change(func(held *document) { held.CloseUnnamedWindows = closing })
}

// SkippedVersion returns the released version the user has chosen to pass over,
// empty where there is none.
func (prefs *Preferences) SkippedVersion() (string, error) {
	prefs.mutex.Lock()
	defer prefs.mutex.Unlock()
	held, err := prefs.read()
	if err != nil {
		return "", err
	}
	return held.SkippedVersion, nil
}

// SetSkippedVersion records a version not to offer again (FR-058).
func (prefs *Preferences) SetSkippedVersion(version string) error {
	return prefs.change(func(held *document) { held.SkippedVersion = version })
}

// change reads the file, applies one edit and writes it back under the lock, so
// the other settings in it survive.
func (prefs *Preferences) change(edit func(*document)) error {
	prefs.mutex.Lock()
	defer prefs.mutex.Unlock()
	held, err := prefs.read()
	if err != nil {
		return err
	}
	edit(&held)
	raw, err := json.MarshalIndent(held, "", "  ")
	if err != nil {
		return fmt.Errorf("preparing the settings: %w", err)
	}
	directory := filepath.Dir(prefs.path)
	if err := os.MkdirAll(directory, os.ModePerm); err != nil {
		return fmt.Errorf("creating %s: %w", directory, err)
	}
	if err := store.WriteAtomic(directory, prefs.path, raw); err != nil {
		return err
	}
	return nil
}

// read returns the settings as they stand.
//
// A file that is not there is an empty document rather than a fault: it means
// the user has chosen nothing. A file that cannot be PARSED is a fault, because
// carrying on over it would silently write the defaults over whatever the user
// had set.
func (prefs *Preferences) read() (document, error) {
	var held document
	raw, err := os.ReadFile(prefs.path)
	if os.IsNotExist(err) {
		return held, nil
	}
	if err != nil {
		return held, fmt.Errorf("reading the settings: %w", err)
	}
	if err := json.Unmarshal(raw, &held); err != nil {
		return held, fmt.Errorf("reading %s: %w", FileName, err)
	}
	return held, nil
}
