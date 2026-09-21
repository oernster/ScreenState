package application

import (
	"context"
	"strings"
	"sync"

	"github.com/oernster/ScreenState/internal/domain"
)

// The desktop a test says exists, the processes it says are running, the
// launcher it watches and the setting a restore reads about the windows a
// profile does not name. They are here rather than beside the clock and the
// log because together they are most of the fakes by weight: one file holding
// every fake went over the size the structural suite allows.

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
	// nudged counts the times the taskbars were sent a click; nudgeErr is the
	// refusal a test wants instead.
	nudged   int
	nudgeErr error
	// attention names the windows whose button was drawing attention; stopped
	// records the ones told to stop and stopErr the refusal a test wants.
	attention map[WindowID]bool
	stopped   []WindowID
	stopErr   error
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

// NudgeTaskbars records that the taskbars were sent a click.
func (desktop *fakeDesktop) NudgeTaskbars(context.Context) (int, error) {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	if desktop.nudgeErr != nil {
		return 0, desktop.nudgeErr
	}
	desktop.nudged++
	return taskbarsOnTheReferenceMachine, nil
}

// nudgeCount answers how many times the taskbars were sent a click.
func (desktop *fakeDesktop) nudgeCount() int {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	return desktop.nudged
}

// StopDrawingAttention records that a window's button was settled.
func (desktop *fakeDesktop) StopDrawingAttention(_ context.Context, id WindowID) (bool, error) {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	if desktop.stopErr != nil {
		return false, desktop.stopErr
	}
	desktop.stopped = append(desktop.stopped, id)
	return desktop.attention[id], nil
}

// settledButtons answers which windows were told to stop drawing attention.
func (desktop *fakeDesktop) settledButtons() []WindowID {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	settled := make([]WindowID, len(desktop.stopped))
	copy(settled, desktop.stopped)
	return settled
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
