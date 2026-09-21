//go:build windows

package win32

import (
	"context"
	"testing"
	"time"
)

// probeClock is enough of a clock for a reading. Nothing here waits.
type probeClock struct{}

func (probeClock) Now() time.Time                             { return time.Now() }
func (probeClock) Sleep(context.Context, time.Duration) error { return nil }

// TestTheDesktopCanBeRead is an integration test rather than a unit test: it
// asks the machine it runs on what is actually there.
//
// It is the only kind of test that can say anything about the Win32 half, since
// a fake desktop proves nothing about whether the real calls work. It asserts
// what must hold wherever it runs rather than what happens to be true here, so
// it says nothing about how many displays or windows there should be.
//
// It skips where there is no desktop to read, which is the case in a service or
// a build agent. A skip is an honest answer; a pass on no data would not be.
func TestTheDesktopCanBeRead(t *testing.T) {
	desktop := NewDesktop(probeClock{})
	ctx := context.Background()

	displays, err := desktop.Displays(ctx)
	if err != nil {
		t.Skipf("no desktop to read here: %v", err)
	}
	for _, display := range displays {
		if err := display.Identity.Validate(); err != nil {
			t.Errorf("a display has no usable identity: %v", err)
		}
		if !display.Bounds.Valid() {
			t.Errorf("display %s has no size: %s", display.Identity, display.Bounds)
		}
		if !display.WorkArea.Valid() {
			t.Errorf("display %s has no work area: %s", display.Identity, display.WorkArea)
		}
		t.Logf("display %s bounds %s work %s primary=%v",
			display.Identity, display.Bounds, display.WorkArea, display.Primary)
	}

	windows, err := desktop.Windows(ctx)
	if err != nil {
		t.Fatalf("reading the windows: %v", err)
	}
	for _, window := range windows {
		if window.Unreadable != "" {
			t.Logf("unreadable: %q: %s", window.Description, window.Unreadable)
			continue
		}
		if err := window.Application.Validate(); err != nil {
			t.Errorf("window %q has no usable identity: %v", window.Description, err)
		}
		t.Logf("window %q %s %s %s visible=%v",
			window.Description, window.Application, window.Rect, window.State, window.Visible)
	}
	t.Logf("read %d display(s) and %d window(s)", len(displays), len(windows))
}

// TestARunningApplicationIsFoundOnThisMachine checks the identity rules against
// real processes rather than against strings.
//
// It takes the applications actually on screen and asks whether they are
// running, which must be true: their windows are open. It is the one end to end
// check of the matching rule that can be made without starting or moving
// anything, so it is the one this suite makes.
//
// It reads and never acts. Starting an application or moving a window would
// disturb the desktop of whoever ran the tests, so neither is tested here and
// both remain unverified until they are run against a real desktop deliberately.
func TestARunningApplicationIsFoundOnThisMachine(t *testing.T) {
	desktop := NewDesktop(probeClock{})
	processes := NewProcesses()
	ctx := context.Background()

	if _, err := desktop.Displays(ctx); err != nil {
		t.Skipf("no desktop to read here: %v", err)
	}
	windows, err := desktop.Windows(ctx)
	if err != nil {
		t.Fatalf("reading the windows: %v", err)
	}

	checked := 0
	for _, window := range windows {
		if window.Unreadable != "" || window.Application.Validate() != nil {
			continue
		}
		running, err := processes.Running(ctx, window.Application)
		if err != nil {
			t.Fatalf("asking whether %s is running: %v", window.Application, err)
		}
		if !running {
			t.Errorf("%s has a window open and was not found running", window.Application)
		}
		checked++
	}
	if checked == 0 {
		t.Skip("no readable window to check against")
	}
	t.Logf("recognised %d application(s) by their identity", checked)
}
