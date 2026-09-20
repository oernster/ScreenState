package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// productName is the one place this product's name appears in a path.
const productName = "ScreenState"

// profilesDirectory is the folder holding the profiles inside it.
const profilesDirectory = "profiles"

// localAppData is where Windows keeps a user's own application data. DATA-001
// puts the store there and C-2 keeps everything this product writes inside the
// signed-in user's own directories.
const localAppData = "LOCALAPPDATA"

// DefaultDirectory returns where this user's profiles are kept.
//
// It falls back to the Go runtime's idea of a user cache directory where the
// environment does not name one, which is what lets the store be exercised on a
// machine that is not Windows. On Windows the two answer the same place.
func DefaultDirectory() (string, error) {
	if local := strings.TrimSpace(os.Getenv(localAppData)); local != "" {
		return filepath.Join(local, productName, profilesDirectory), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("finding this user's application data directory: %w", err)
	}
	return filepath.Join(base, productName, profilesDirectory), nil
}

// safe is the set of characters a profile name may contribute to a filename
// unchanged. Everything else is escaped, so a profile called "Desk: 4k/2" is
// stored without inventing a directory or losing half its name.
const safe = "abcdefghijklmnopqrstuvwxyz0123456789 -_()"

// escape renders a profile name as a filename. It lowercases first, so that two
// names differing only in case are one profile, which is how the user reads
// them. The name as typed is kept inside the file, never derived back from the
// filename.
func escape(name string) string {
	lowered := strings.ToLower(strings.TrimSpace(name))
	var built strings.Builder
	for _, letter := range lowered {
		if letter < 0x80 && strings.ContainsRune(safe, letter) {
			built.WriteRune(letter)
			continue
		}
		built.WriteString(fmt.Sprintf("%%%04X", letter))
	}
	return built.String()
}

// pathFor returns the file a profile of that name is kept in.
func (store *Store) pathFor(name string) string {
	return filepath.Join(store.directory, escape(name)+extension)
}

// rename is os.Rename, named here so that a test can make the last step of an
// atomic write fail and prove that the previous profile survives it. Nothing
// else replaces it.
var rename = os.Rename

// write puts bytes at a path so that an interruption leaves the previous
// contents or the new ones, never a mixture (FR-006).
//
// The temporary file is created in the same directory as the target, because a
// move between directories is a copy and a delete rather than one act; a copy
// can be interrupted half way. It is removed on every path out, so a
// failure leaves no litter behind for the next listing to trip over.
func (store *Store) write(path string, raw []byte) (err error) {
	temporary, err := os.CreateTemp(store.directory, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("preparing to write %s: %w", filepath.Base(path), err)
	}
	name := temporary.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(name)
		}
	}()

	if _, err = temporary.Write(raw); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("writing %s: %w", filepath.Base(path), err)
	}
	// The contents reach the disk before the move, so that a power failure
	// cannot leave the new name pointing at a file with nothing in it.
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("writing %s: %w", filepath.Base(path), err)
	}
	if err = temporary.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", filepath.Base(path), err)
	}
	if err = rename(name, path); err != nil {
		return fmt.Errorf("replacing %s: %w", filepath.Base(path), err)
	}
	return nil
}
