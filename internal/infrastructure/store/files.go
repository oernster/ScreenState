package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/oernster/ScreenState/internal/product"
)

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
		return filepath.Join(local, product.Name, profilesDirectory), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("finding this user's application data directory: %w", err)
	}
	return filepath.Join(base, product.Name, profilesDirectory), nil
}

// safe is the set of characters a profile name may contribute to a filename
// unchanged. Everything else is escaped, so a profile called "Desk: 4k/2" is
// stored without inventing a directory or losing half its name.
const safe = "abcdefghijklmnopqrstuvwxyz0123456789 -_()"

// The two escapes a filename may carry. A letter in the Basic Multilingual
// Plane is four hex digits after the mark; one beyond it is six after the mark
// and a plus sign, which no four-digit escape begins with. So no escape reads
// as the start of another and a letter followed by a digit never reads as one
// wider letter.
const (
	escapeBasic  = "%%%04X"
	escapeAstral = "%%+%06X"
	basicPlane   = 0xFFFF
)

// escape renders a profile name as a filename. Two names share a filename
// exactly when strings.EqualFold says they are one name, which is the
// comparison the manager makes before it refuses a name already in use
// (FR-002). The name as typed is kept inside the file, never derived back from
// the filename.
func escape(name string) string {
	var built strings.Builder
	for _, letter := range strings.TrimSpace(name) {
		letter = folded(letter)
		switch {
		case letter < utf8.RuneSelf && strings.ContainsRune(safe, letter):
			built.WriteRune(letter)
		case letter <= basicPlane:
			built.WriteString(fmt.Sprintf(escapeBasic, letter))
		default:
			built.WriteString(fmt.Sprintf(escapeAstral, letter))
		}
	}
	return built.String()
}

// folded returns the one letter standing for every letter strings.EqualFold
// treats as this one: the lowest lower-case letter among them, else the lowest
// of them. Each set of letters EqualFold joins gives one answer and no two sets
// give the same one, since the sets do not overlap. ASCII letters fold to their
// lower case, so the file of a profile with an ASCII name is named as before.
func folded(letter rune) rune {
	lowest, lowestLower := letter, rune(-1)
	member := letter
	for {
		if member < lowest {
			lowest = member
		}
		if unicode.IsLower(member) && (lowestLower < 0 || member < lowestLower) {
			lowestLower = member
		}
		member = unicode.SimpleFold(member)
		if member == letter {
			break
		}
	}
	if lowestLower >= 0 {
		return lowestLower
	}
	return lowest
}

// legacyEscape is the filename builds up to 1.3.0 gave a profile: lowercased,
// every other letter as a variable-width escape. It was not one to one (S-3),
// so nothing is written under it any more; a file already named this way is
// still recognised as the profile it holds and is replaced in place.
func legacyEscape(name string) string {
	lowered := strings.ToLower(strings.TrimSpace(name))
	var built strings.Builder
	for _, letter := range lowered {
		if letter < utf8.RuneSelf && strings.ContainsRune(safe, letter) {
			built.WriteRune(letter)
			continue
		}
		built.WriteString(fmt.Sprintf(escapeBasic, letter))
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

// write puts bytes at a path inside this store's own directory.
func (store *Store) write(path string, raw []byte) error {
	return WriteAtomic(store.directory, path, raw)
}

// WriteAtomic puts bytes at a path so that an interruption leaves the previous
// contents or the new ones, never a mixture (FR-006).
//
// The temporary file is created in the given directory, which must be the
// target's own: a move between directories is a copy and a delete rather than
// one act; a copy can be interrupted half way. It is removed on every path
// out, so a failure leaves no litter behind for the next listing to trip over.
//
// It is exported because the profiles are not the only thing this product
// writes. The settings file wants the same promise; two copies of an atomic
// write is how one of them comes to be the careless one.
func WriteAtomic(directory, path string, raw []byte) (err error) {
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".*")
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
