package clock

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTheClockReadsTheWallClock(t *testing.T) {
	t.Parallel()
	before := time.Now()
	read := New().Now()
	if read.Before(before) || read.After(time.Now()) {
		t.Fatalf("the clock read %s, outside the moment it was asked", read)
	}
}

func TestASleepThatRunsItsCourseAnswersNothing(t *testing.T) {
	t.Parallel()
	started := time.Now()
	if err := New().Sleep(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("sleeping answered %v", err)
	}
	if time.Since(started) < time.Millisecond {
		t.Fatal("the sleep returned before the time was up")
	}
}

// A restore stood down mid-wait must be able to tell that from a wait that
// simply finished, which is the difference between FR-061 and the ceiling.
func TestASleepCutShortSaysSo(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	err := New().Sleep(ctx, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("answered %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("the sleep waited although the context had ended")
	}
}

// A wait of no time at all still reports a context that has ended, so a caller
// polling with a zero interval cannot spin on past a cancellation.
func TestASleepOfNoTimeStillReportsACancellation(t *testing.T) {
	t.Parallel()
	if err := New().Sleep(context.Background(), 0); err != nil {
		t.Fatalf("a zero wait answered %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := New().Sleep(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("a zero wait on a cancelled context answered %v", err)
	}
}
