package application

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// The fakes below are written by hand rather than generated, so that each one
// is as simple as the test using it needs and no test depends on a library's
// idea of what an interface should do when it is surprised.

// fakeClock is time under the test's control. Sleeping advances it rather than
// waiting, so a ceiling of fifteen minutes costs a test nothing.
type fakeClock struct {
	mutex sync.Mutex
	now   time.Time
	slept []time.Duration
	// onSleep runs after each sleep, which is where a test changes the world:
	// a window appearing, an application moving itself, a restore replaced.
	onSleep func(ctx context.Context, clock *fakeClock, count int)
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, time.September, 20, 9, 0, 0, 0, time.UTC)}
}

func (clock *fakeClock) Now() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return clock.now
}

func (clock *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	clock.mutex.Lock()
	clock.now = clock.now.Add(d)
	clock.slept = append(clock.slept, d)
	count := len(clock.slept)
	hook := clock.onSleep
	clock.mutex.Unlock()

	if hook != nil {
		hook(ctx, clock, count)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (clock *fakeClock) sleeps() int {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return len(clock.slept)
}

// fakeLog keeps the steps so a test can read what the restore said it did.
type fakeLog struct {
	mutex sync.Mutex
	steps []string
}

func (log *fakeLog) Step(message string) {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	log.steps = append(log.steps, message)
}

func (log *fakeLog) saying(fragment string) bool {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	for _, step := range log.steps {
		if strings.Contains(step, fragment) {
			return true
		}
	}
	return false
}

// placeCall is one request to move a window, kept so a test can assert the
// order things happened in as well as the state they ended in.
type placeCall struct {
	id    WindowID
	rect  domain.Rect
	state domain.ShowState
}

// fakeDesktop is the windows and displays a test says exist. Placing a window
// updates it, so a restore that places a window then reads it back sees what it
// asked for, exactly as a cooperative application would leave it.
type fakeDesktop struct {
	mutex       sync.Mutex
	windows     []Window
	displays    []Display
	placed      []placeCall
	windowsErr  error
	displaysErr error
	windowErr   error
	placeErr    map[WindowID]error
	closeErr    map[WindowID]error
	// closed records every window asked to close; refuses names the ones whose
	// application does not act on the request.
	closed  []WindowID
	refuses map[WindowID]bool
	// afterPlace runs once a window has been placed, which is how a test makes
	// an application move its own window afterwards.
	afterPlace func(desktop *fakeDesktop, id WindowID)
	// onWindows runs before the windows are read, which is how a test makes the
	// desktop fail in a way no error return can express.
	onWindows func()
}

func (desktop *fakeDesktop) Windows(context.Context) ([]Window, error) {
	if desktop.onWindows != nil {
		desktop.onWindows()
	}
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	if desktop.windowsErr != nil {
		return nil, desktop.windowsErr
	}
	copied := make([]Window, len(desktop.windows))
	copy(copied, desktop.windows)
	return copied, nil
}

func (desktop *fakeDesktop) Window(_ context.Context, id WindowID) (Window, error) {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	if desktop.windowErr != nil {
		return Window{}, desktop.windowErr
	}
	for _, window := range desktop.windows {
		if window.ID == id {
			return window, nil
		}
	}
	return Window{}, ErrWindowGone
}

func (desktop *fakeDesktop) Displays(context.Context) ([]Display, error) {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	if desktop.displaysErr != nil {
		return nil, desktop.displaysErr
	}
	copied := make([]Display, len(desktop.displays))
	copy(copied, desktop.displays)
	return copied, nil
}

func (desktop *fakeDesktop) Place(
	_ context.Context,
	id WindowID,
	rect domain.Rect,
	state domain.ShowState,
) error {
	desktop.mutex.Lock()
	if err, refused := desktop.placeErr[id]; refused {
		desktop.mutex.Unlock()
		return err
	}
	desktop.placed = append(desktop.placed, placeCall{id: id, rect: rect, state: state})
	for at, window := range desktop.windows {
		if window.ID == id {
			desktop.windows[at].Rect = rect
			desktop.windows[at].State = state
		}
	}
	hook := desktop.afterPlace
	desktop.mutex.Unlock()

	if hook != nil {
		hook(desktop, id)
	}
	return nil
}

// Close is the request FR-064 makes of a window, answered the way the test
// says an application would answer it.
//
// A window goes by default, which is what closing one usually does. A test
// naming it in refuses keeps it, which is the application that puts up a
// prompt about unsaved work or ignores the request outright.
func (desktop *fakeDesktop) Close(_ context.Context, id WindowID) error {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	if err, refused := desktop.closeErr[id]; refused {
		return err
	}
	desktop.closed = append(desktop.closed, id)
	if desktop.refuses[id] {
		return nil
	}
	kept := desktop.windows[:0]
	for _, window := range desktop.windows {
		if window.ID != id {
			kept = append(kept, window)
		}
	}
	desktop.windows = kept
	return nil
}

// closedCount answers how many times a window was asked to close.
func (desktop *fakeDesktop) closedCount(id WindowID) int {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	var seen int
	for _, asked := range desktop.closed {
		if asked == id {
			seen++
		}
	}
	return seen
}

// fakeStrangers is the setting for the windows a profile does not name, as a
// test states it.
type fakeStrangers struct {
	mutex    sync.Mutex
	closing  bool
	readErr  error
	writeErr error
}

func (strangers *fakeStrangers) CloseStrangers() (bool, error) {
	strangers.mutex.Lock()
	defer strangers.mutex.Unlock()
	if strangers.readErr != nil {
		return false, strangers.readErr
	}
	return strangers.closing, nil
}

func (strangers *fakeStrangers) SetCloseStrangers(closing bool) error {
	strangers.mutex.Lock()
	defer strangers.mutex.Unlock()
	if strangers.writeErr != nil {
		return strangers.writeErr
	}
	strangers.closing = closing
	return nil
}

// moveWindow is how a test makes an application move its own window after the
// restore placed it, which is the case FR-033 exists for.
func (desktop *fakeDesktop) moveWindow(id WindowID, rect domain.Rect) {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	for at, window := range desktop.windows {
		if window.ID == id {
			desktop.windows[at].Rect = rect
		}
	}
}

// addWindow is how a test makes a window appear part way through a restore.
func (desktop *fakeDesktop) addWindow(window Window) {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	desktop.windows = append(desktop.windows, window)
}

// showWindow is how a test makes a hidden window become visible, which is what
// a cooperative application does when it is run a second time.
func (desktop *fakeDesktop) showWindow(id WindowID) {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	for at, window := range desktop.windows {
		if window.ID == id {
			desktop.windows[at].Visible = true
		}
	}
}

// removeDisplay is how a test unplugs a screen during a restore.
func (desktop *fakeDesktop) removeDisplay(monitorID string) {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	var kept []Display
	for _, display := range desktop.displays {
		if display.Identity.MonitorID != monitorID {
			kept = append(kept, display)
		}
	}
	desktop.displays = kept
}

func (desktop *fakeDesktop) placements() []placeCall {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	copied := make([]placeCall, len(desktop.placed))
	copy(copied, desktop.placed)
	return copied
}

// fakeProcesses answers whether an application is running, from a set the test
// controls.
type fakeProcesses struct {
	mutex   sync.Mutex
	running map[string]bool
	err     error
}

func newFakeProcesses(running ...domain.ApplicationIdentity) *fakeProcesses {
	processes := &fakeProcesses{running: make(map[string]bool)}
	for _, application := range running {
		processes.running[strings.ToLower(application.String())] = true
	}
	return processes
}

func (processes *fakeProcesses) Running(
	_ context.Context,
	application domain.ApplicationIdentity,
) (bool, error) {
	processes.mutex.Lock()
	defer processes.mutex.Unlock()
	if processes.err != nil {
		return false, processes.err
	}
	return processes.running[strings.ToLower(application.String())], nil
}

func (processes *fakeProcesses) start(application domain.ApplicationIdentity) {
	processes.mutex.Lock()
	defer processes.mutex.Unlock()
	processes.running[strings.ToLower(application.String())] = true
}

// fakeLauncher records what was launched and can refuse to launch anything the
// test names.
type fakeLauncher struct {
	mutex    sync.Mutex
	launched []string
	refuse   map[string]error
	// onLaunch runs after a successful launch, which is how a test makes an
	// application appear once it has been started.
	onLaunch func(application domain.ApplicationIdentity)
}

func (launcher *fakeLauncher) Launch(_ context.Context, application domain.ApplicationIdentity) error {
	launcher.mutex.Lock()
	if err, refused := launcher.refuse[strings.ToLower(application.String())]; refused {
		launcher.mutex.Unlock()
		return err
	}
	launcher.launched = append(launcher.launched, application.String())
	hook := launcher.onLaunch
	launcher.mutex.Unlock()

	if hook != nil {
		hook(application)
	}
	return nil
}

func (launcher *fakeLauncher) launchCount(application domain.ApplicationIdentity) int {
	launcher.mutex.Lock()
	defer launcher.mutex.Unlock()
	count := 0
	for _, launched := range launcher.launched {
		if strings.EqualFold(launched, application.String()) {
			count++
		}
	}
	return count
}
