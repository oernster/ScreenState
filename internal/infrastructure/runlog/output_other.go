//go:build !windows

package runlog

import (
	"errors"
	"os"
)

// hasErrorOutput answers yes off Windows, so Keep copies the crash report to
// the log and leaves the error output where it is. A run given none was
// measured on a windowed Windows build alone, which is the only place this
// product runs.
func hasErrorOutput() bool { return true }

// sendAll is not built for this platform, so it says so rather than doing
// nothing and letting a caller believe the log is catching a crash.
func sendAll(*os.File) error {
	return errors.New("sending error output to a file is not built for this platform")
}
