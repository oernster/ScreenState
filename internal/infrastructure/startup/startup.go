// Package startup is the sign-in registry entry as the application layer sees
// it: one interface over the entry the setup program writes (FR-046), so the
// manager can turn it on and off (FR-053).
//
// It holds no registry code of its own. The setup package already owns that
// entry, its name and the shape of its value; two copies of those is exactly
// how a setting and Windows come to disagree in silence.
package startup

import (
	"fmt"
	"os"

	"github.com/oernster/ScreenState/internal/infrastructure/setup"
)

// Entry is the per-user sign-in entry.
type Entry struct {
	// executable is the file the entry names. It is the running program's own
	// path rather than the installed one, so an entry written from a build
	// under test starts that build and not something else.
	executable string
}

// New returns the sign-in entry, reading this program's own path.
//
// A path that cannot be read leaves the entry answering an error from both
// halves rather than writing a value naming nothing: an entry pointing at a
// file that is not there reads as on while nothing starts, which is the one
// state worse than being off.
func New() *Entry {
	path, err := os.Executable()
	if err != nil {
		return &Entry{}
	}
	return &Entry{executable: path}
}

// Enabled reports whether the entry is present and names a real file.
func (entry *Entry) Enabled() (bool, error) {
	return setup.IsLaunchOnBoot(), nil
}

// SetEnabled writes the entry or removes it.
func (entry *Entry) SetEnabled(enabled bool) error {
	if enabled && entry.executable == "" {
		return fmt.Errorf("this program cannot name its own file, so the entry would start nothing")
	}
	return setup.SetLaunchOnBoot(entry.executable, enabled)
}
