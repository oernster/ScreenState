// Package runlog keeps the log a run leaves (FR-050): every step a restore
// attempted with its outcome, in Log.txt beside the profiles.
//
// It exists because the worst failures of this product happen minutes after
// sign-in with nobody watching; also because a windowed Windows build is given
// no error output at all: everything written there is lost, including the Go
// runtime's own report of a crash. Pointing the error output at the log means a
// run that dies leaves a record rather than a silence. Ported from Bridge Talk,
// where the rule was learned from a run that vanished with nothing to read.
package runlog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/oernster/ScreenState/internal/product"
)

const (
	// FileName names the log.
	FileName = "Log.txt"
	// MaxBytes is the size past which a run starts the log afresh, so the file
	// cannot grow without end on a machine that signs in every day.
	MaxBytes = 1 << 20

	startedLayout = "2006-01-02 15:04:05"
	stepLayout    = "15:04:05"
	folderPerm    = 0o755
	filePerm      = 0o644
)

// Open opens the log at path for a run started at started, making its folder
// where there is none, then writes the line that says which build is running.
//
// That line matters more than it looks: the first question about any report is
// which binary produced it; a version read off the running program is the only
// answer that cannot be wrong.
func Open(path string, version string, started time.Time) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), folderPerm); err != nil {
		return nil, fmt.Errorf("making the folder for %s: %w", path, err)
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if info, err := os.Stat(path); err == nil && info.Size() > MaxBytes {
		flags |= os.O_TRUNC
	}
	log, err := os.OpenFile(path, flags, filePerm)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	if _, err := fmt.Fprintf(log, "\n%s %s started %s\n",
		product.Name, version, started.Format(startedLayout)); err != nil {
		_ = log.Close()
		return nil, fmt.Errorf("writing to %s: %w", path, err)
	}
	return log, nil
}

// Keep sends what a run reports as it fails to the log, which stays open for
// the rest of the run. Where the run has no error output of its own, which is
// how a windowed program started at sign-in runs, everything goes to the log;
// otherwise the error output is left where it is and the crash report is copied
// to the log as well.
func Keep(log *os.File) error {
	if !hasErrorOutput() {
		return sendAll(log)
	}
	if err := debug.SetCrashOutput(log, debug.CrashOptions{}); err != nil {
		return fmt.Errorf("copying crash reports to %s: %w", log.Name(), err)
	}
	return nil
}

// Steps is the step log the application layer writes to. It answers nothing: a
// step that cannot be written must not stop a restore, because the restore
// matters more than the record of it.
type Steps struct {
	mutex sync.Mutex
	out   io.Writer
	now   func() time.Time
}

// NewSteps logs to out, stamping each step from now.
func NewSteps(out io.Writer, now func() time.Time) *Steps {
	return &Steps{out: out, now: now}
}

// Step writes one line. The lock is there because a restore and a capture can
// both be running; two half lines interleaved would be worse than either.
func (steps *Steps) Step(message string) {
	steps.mutex.Lock()
	defer steps.mutex.Unlock()
	_, _ = fmt.Fprintf(steps.out, "  %s %s\n", steps.now().Format(stepLayout), message)
}
