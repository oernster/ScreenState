package runlog

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/product"
)

// The step log is what the application layer writes to. Checked by the
// compiler rather than by a reader comparing two files.
var _ application.Log = (*Steps)(nil)

func at(hour, minute, second int) time.Time {
	return time.Date(2026, time.September, 20, hour, minute, second, 0, time.UTC)
}

// The first line of a run answers the first question anybody asks of a report:
// which build produced it.
func TestARunSaysWhichBuildItIs(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "deeper", FileName)
	log, err := Open(path, "1.2.3", at(9, 0, 0))
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	written := string(raw)
	if !strings.Contains(written, product.Name+" 1.2.3 started 2026-09-20 09:00:00") {
		t.Fatalf("the log opens with %q", written)
	}
}

// A machine that signs in every day must not grow a log without end; a run must
// not lose the log of the run before it either.
func TestTheLogIsKeptUntilItIsTooBig(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), FileName)

	first, err := Open(path, "1.0.0", at(9, 0, 0))
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	NewSteps(first, func() time.Time { return at(9, 0, 1) }).Step("the first run")
	if err := first.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}

	second, err := Open(path, "1.0.0", at(10, 0, 0))
	if err != nil {
		t.Fatalf("opening again: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "the first run") {
		t.Fatal("the second run lost the log of the first")
	}

	// Past the limit the log starts afresh, so the file cannot grow for ever.
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), MaxBytes+1), 0o644); err != nil {
		t.Fatalf("filling it: %v", err)
	}
	third, err := Open(path, "1.0.0", at(11, 0, 0))
	if err != nil {
		t.Fatalf("opening a full log: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	raw, _ = os.ReadFile(path)
	if len(raw) > MaxBytes {
		t.Fatalf("the log was not started afresh: %d bytes", len(raw))
	}
}

// Each step is stamped, so a report minutes after sign-in can be read against
// what the desktop was doing at the time.
func TestEveryStepIsStamped(t *testing.T) {
	t.Parallel()
	var written bytes.Buffer
	steps := NewSteps(&written, func() time.Time { return at(9, 4, 5) })
	steps.Step("restoring \"Desk\"")

	if !strings.Contains(written.String(), "09:04:05 restoring \"Desk\"") {
		t.Fatalf("written as %q", written.String())
	}
}

// A restore and a capture can be running at once; two half lines woven
// together would be worse than either.
func TestStepsFromTwoPlacesDoNotInterleave(t *testing.T) {
	t.Parallel()
	var written safeBuffer
	steps := NewSteps(&written, func() time.Time { return at(9, 0, 0) })

	var waiting sync.WaitGroup
	for range 50 {
		waiting.Add(1)
		go func() {
			defer waiting.Done()
			steps.Step("a step of a settled length")
		}()
	}
	waiting.Wait()

	lines := strings.Split(strings.TrimRight(written.String(), "\n"), "\n")
	if len(lines) != 50 {
		t.Fatalf("wrote %d lines", len(lines))
	}
	for _, line := range lines {
		if line != "  09:00:00 a step of a settled length" {
			t.Fatalf("a line came out as %q", line)
		}
	}
}

// safeBuffer is a buffer several goroutines may write to, which bytes.Buffer
// is not. Without it this test would be measuring the test's own race.
type safeBuffer struct {
	mutex   sync.Mutex
	written bytes.Buffer
}

func (buffer *safeBuffer) Write(raw []byte) (int, error) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	return buffer.written.Write(raw)
}

func (buffer *safeBuffer) String() string {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	return buffer.written.String()
}

// A run that has an error output of its own keeps it and has its crash report
// copied to the log as well.
//
// This is the branch a test binary can reach: a test is started with an error
// output; a run without one cannot be made from inside the process. The
// other branch, where the log takes everything, belongs to a windowed build
// that does not exist yet; proving it needs a child process that really
// crashes, as Bridge Talk does; that test belongs with the windowed build
// rather than before it.
func TestARunWithAnErrorOutputKeepsIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	log, err := Open(path, "1.0.0", at(9, 0, 0))
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer func() { _ = log.Close() }()

	if !hasErrorOutput() {
		t.Skip("this test binary was started without an error output")
	}
	if err := Keep(log); err != nil {
		t.Fatalf("keeping the log: %v", err)
	}
	// The runtime holds its own handle on the log for the rest of the process,
	// which is what a real run wants and what stops this test's directory being
	// cleaned up. Letting it go is part of the test, not a workaround.
	defer func() { _ = debug.SetCrashOutput(nil, debug.CrashOptions{}) }()

	if os.Stderr == log {
		t.Fatal("the error output was taken although the run had one")
	}
}

// A log that cannot be opened says why, rather than leaving a run with nowhere
// to write and nothing to read afterwards.
func TestALogThatCannotBeOpenedSaysWhy(t *testing.T) {
	t.Parallel()
	occupied := filepath.Join(t.TempDir(), "occupied")
	if err := os.WriteFile(occupied, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("planting: %v", err)
	}
	if _, err := Open(filepath.Join(occupied, FileName), "1.0.0", at(9, 0, 0)); err == nil {
		t.Fatal("opening a log inside a file answered no error")
	}
}
