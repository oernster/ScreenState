package application

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// Every fake in this package is written by hand rather than generated, so that
// each one is as simple as the test using it needs and no test depends on a
// library's idea of what an interface should do when it is surprised.
//
// The clock and the log are here. The desktop and the collaborators around it
// are in fakes_desktop_test.go, the profile store in fakes_store_test.go: one
// file holding all of them went over the size the structural suite allows.

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

// fakeStep is how far the fake clock moves each time a restore waits for the
// desktop to change, so a restore nobody touches still reaches its ceiling.
const fakeStep = time.Second

// fakeEvents stands in for Windows telling a restore the desktop changed
// (FR-079).
//
// The restore's own watch answers every wait with a change after one fake
// second on the clock, which runs the clock's onSleep hook: that is where a
// test changes the world between passes, as it always has. touchAt makes the
// nth wait the user's first key press or click instead.
//
// The watch a finished restore keeps on its placed windows (FR-033) is inert
// unless a test sets afterwards: it answers "touched" at once, so nothing runs
// behind a test's back while it reads the results.
type fakeEvents struct {
	clock *fakeClock

	mutex    sync.Mutex
	watches  int
	waits    int
	touchAt  int
	watchErr error
	// tailWatchErr refuses only the watch a finished restore asks for.
	tailWatchErr error
	afterwards   func(ctx context.Context) (Wake, error)
	// series is the flash series FR-080 waits out; zero, as most tests leave
	// it, means nothing is waited for. flashAt makes the nth wait the shell
	// reporting those windows flashing.
	series  time.Duration
	flashAt map[int][]WindowID
	// takenOver is the user having pressed a key or clicked while nothing was
	// waiting, which only Touched hears (FR-087).
	takenOver bool
}

func (events *fakeEvents) FlashSeries() time.Duration { return events.series }

func (events *fakeEvents) Watch(context.Context) (DesktopWatch, error) {
	events.mutex.Lock()
	defer events.mutex.Unlock()
	if events.watchErr != nil {
		return nil, events.watchErr
	}
	if events.watches > 0 && events.tailWatchErr != nil {
		return nil, events.tailWatchErr
	}
	events.watches++
	return &fakeWatch{events: events, tail: events.watches > 1}, nil
}

// fakeWatch is one watch from fakeEvents.
type fakeWatch struct {
	events  *fakeEvents
	tail    bool
	flashed []WindowID
}

func (watch *fakeWatch) Flashing() []WindowID {
	watch.events.mutex.Lock()
	defer watch.events.mutex.Unlock()
	flashed := watch.flashed
	watch.flashed = nil
	return flashed
}

func (watch *fakeWatch) Touched() bool {
	watch.events.mutex.Lock()
	defer watch.events.mutex.Unlock()
	return !watch.tail && watch.events.takenOver
}

func (watch *fakeWatch) Next(ctx context.Context, deadline time.Time) (Wake, error) {
	events := watch.events
	events.mutex.Lock()
	afterwards := events.afterwards
	events.waits++
	touched := !watch.tail && events.waits == events.touchAt
	flashing := events.flashAt[events.waits]
	events.mutex.Unlock()
	if watch.tail {
		if afterwards == nil {
			return WakeTouched, nil
		}
		return afterwards(ctx)
	}
	if touched {
		return WakeTouched, nil
	}
	if err := events.clock.Sleep(ctx, fakeStep); err != nil {
		return WakeDeadline, err
	}
	if !events.clock.Now().Before(deadline) {
		return WakeDeadline, nil
	}
	if flashing != nil {
		events.mutex.Lock()
		watch.flashed = append(watch.flashed, flashing...)
		events.mutex.Unlock()
		return WakeFlashed, nil
	}
	return WakeChanged, nil
}

// fakeLog keeps the steps so a test can read what the restore said it did.
type fakeLog struct {
	mutex sync.Mutex
	steps []string
	// written is told each time a step is written, once a test waits for one.
	written chan struct{}
}

func (log *fakeLog) Step(message string) {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	log.steps = append(log.steps, message)
	if log.written != nil {
		select {
		case log.written <- struct{}{}:
		default:
		}
	}
}

// testPatience bounds how long a test waits for work running behind a restore
// that has already returned. It is the harness's limit, not the product's:
// reaching it fails the test rather than hanging the suite.
const testPatience = 5 * time.Second

// waitFor blocks until the log says something containing the fragment, for the
// work a finished restore leaves running (FR-033).
func (log *fakeLog) waitFor(t *testing.T, fragment string) {
	t.Helper()
	log.mutex.Lock()
	if log.written == nil {
		log.written = make(chan struct{}, 1)
	}
	written := log.written
	log.mutex.Unlock()
	limit := time.After(testPatience)
	for !log.saying(fragment) {
		select {
		case <-written:
		case <-limit:
			t.Fatalf("the log never said %q", fragment)
		}
	}
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
