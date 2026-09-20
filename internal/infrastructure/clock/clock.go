// Package clock is the one place this product reads the time.
//
// The domain reads none at all and the application layer reads it through this
// interface, which is what lets a fifteen minute ceiling be tested in a
// millisecond and a settle-check delay be tested without waiting for it.
package clock

import (
	"context"
	"time"
)

// System is the real clock.
type System struct{}

// New returns the real clock.
func New() System { return System{} }

// Now is the wall clock.
func (System) Now() time.Time { return time.Now() }

// Sleep waits for the given time; it stops early where the context ends first
// and says which happened.
//
// It answers the context's error rather than nil on an early end, so a caller
// cannot mistake a restore that was stood down for one that simply waited. The
// timer is stopped on every path out, so a restore that is replaced does not
// leave a timer running for the rest of its ceiling.
func (System) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
