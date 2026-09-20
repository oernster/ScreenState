package application

import (
	"context"
	"strings"
	"sync"
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
