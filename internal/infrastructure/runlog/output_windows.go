//go:build windows

package runlog

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// hasErrorOutput reports whether the run was given an error output. A windowed
// program started from a shortcut or at sign-in reads a handle of zero, so
// everything it writes there is lost. Measured in Bridge Talk on 2026-09-14
// against Go 1.26.3 and carried here rather than re-derived.
func hasErrorOutput() bool {
	handle, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE)
	return err == nil && handle != 0 && handle != windows.InvalidHandle
}

// sendAll points the run's error output at the log: first the handle the Go
// runtime looks up for each report it writes, then os.Stderr, which was fixed
// from that handle when the program started. Both are needed: setting one
// leaves the other pointing at nothing.
func sendAll(log *os.File) error {
	if err := windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(log.Fd())); err != nil {
		return fmt.Errorf("sending error output to %s: %w", log.Name(), err)
	}
	os.Stderr = log
	return nil
}
