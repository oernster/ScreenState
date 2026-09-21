package application

import (
	"context"
	"fmt"
	"time"
)

// A restore acts on events, never on a wait (FR-079).
//
// It reads the desktop when Windows says something changed and at no other
// time. Where it needs to know that something will not happen (an application
// asked for a window shows none, a window asked to close stays open) the user's
// first key press or mouse click is the event that says so: they have taken
// the desktop over. The ceiling bounds a restore nobody touches; it is one of
// two timers, the other being the flash series FR-080 waits out before the
// taskbar buttons are rebuilt.

// waiting is what a restore waits on: the watch, the ceiling and whether the
// user has taken over yet.
type waiting struct {
	watch    DesktopWatch
	deadline time.Time
	// ceiling is the span the deadline was set from, kept for the words that
	// name it when it passes.
	ceiling time.Duration
	touched bool
}

// deadlineOnly is the watch a restore falls back on where the desktop cannot be
// watched at all. It has no events to give, so it waits for the ceiling: the
// restore reads the desktop once, then once more when the ceiling passes.
type deadlineOnly struct {
	clock Clock
}

// Next waits for the deadline.
func (watch deadlineOnly) Next(ctx context.Context, deadline time.Time) (Wake, error) {
	if err := watch.clock.Sleep(ctx, deadline.Sub(watch.clock.Now())); err != nil {
		return WakeDeadline, err
	}
	return WakeDeadline, nil
}

// Flashing answers nothing: without a watch no flash is heard.
func (deadlineOnly) Flashing() []WindowID { return nil }

// watchDesktop begins watching for the restore. A watch that cannot be begun is
// said in the log and the report rather than ending the restore, which can
// still place every window already open.
func (service *RestoreService) watchDesktop(ctx context.Context, report *Report) DesktopWatch {
	watch, err := service.events.Watch(ctx)
	if err == nil {
		return watch
	}
	message := fmt.Sprintf("the desktop could not be watched, so windows that appear later"+
		" wait for the ceiling: %v", err)
	service.log.Step(message)
	report.Note("%s", message)
	return deadlineOnly{clock: service.clock}
}

// await waits for the next event, noting when the user takes over.
func (service *RestoreService) await(ctx context.Context, state *restoreState) error {
	wake, err := state.waiting.watch.Next(ctx, state.waiting.deadline)
	if err != nil {
		return err
	}
	if wake == WakeTouched {
		state.waiting.touched = true
	}
	return nil
}

// waitedOut ends the wait for every entry still outstanding once nothing more
// can be learnt by waiting: the ceiling has passed or the user has taken over.
// It reports whether it did.
func (service *RestoreService) waitedOut(state *restoreState) bool {
	if !state.hasPending() {
		return false
	}
	if !service.clock.Now().Before(state.waiting.deadline) {
		state.abandonPending(state.waiting.ceiling)
		service.log.Step("the ceiling passed with entries outstanding")
		return true
	}
	if state.waiting.touched {
		state.abandonTouched()
		service.log.Step("the desktop was taken over with entries outstanding")
		return true
	}
	return false
}

// givenUp reports whether waiting is over: the user has taken over or the
// ceiling has passed.
func (service *RestoreService) givenUp(state *restoreState) bool {
	return state.waiting.touched || !service.clock.Now().Before(state.waiting.deadline)
}

// keepWatching carries on FR-033's watch of the windows a restore placed after
// the restore itself has ended, so a window that moves itself once the splash
// says ready is still put back. It stops at the user's first key press or
// click, at the ceiling or when a newer restore begins.
//
// It is a watch of its own, begun as the restore's ends. The restore's watch
// saw no key press or click (givenUp would say so), so none has come between
// the two worth counting.
//
// What it does goes to the log, never to the report, which the restore has
// already handed over for reading: writing to it now would race whoever reads
// it.
func (service *RestoreService) keepWatching(ctx context.Context, state *restoreState) {
	if len(state.placed) == 0 || service.givenUp(state) {
		return
	}
	watchCtx, stop := context.WithCancel(ctx)
	watch, err := service.events.Watch(watchCtx)
	if err != nil {
		stop()
		service.log.Step(fmt.Sprintf("the placed windows could not be watched after the restore: %v", err))
		return
	}
	service.mutex.Lock()
	service.stopWatching = stop
	service.mutex.Unlock()
	ctx = watchCtx

	scratch := NewReport(state.report.Profile, service.clock.Now())
	tail := &restoreState{
		report:  scratch,
		placed:  snapshotPlaced(state.placed),
		waiting: waiting{watch: watch, deadline: state.waiting.deadline, ceiling: state.waiting.ceiling},
	}
	for _, placed := range tail.placed {
		scratch.Track(placed.application)
	}
	go func() {
		defer stop()
		defer func() {
			if recovered := recover(); recovered != nil {
				service.log.Step(fmt.Sprintf("watching the placed windows failed unexpectedly: %v", recovered))
			}
		}()
		for len(tail.placed) > 0 && !service.givenUp(tail) {
			if err := service.await(ctx, tail); err != nil {
				break
			}
			service.recheck(ctx, tail)
		}
		service.logAfterwards(scratch)
	}()
}

// logAfterwards writes what the watch after a restore did: every note and every
// real failure. Every placed window is tracked on the scratch report so a
// recheck can write to it, which leaves the ones nothing happened to at the
// reason every entry starts with; those are not news.
func (service *RestoreService) logAfterwards(scratch *Report) {
	for _, entry := range scratch.Entries() {
		for _, note := range entry.Notes {
			service.log.Step(fmt.Sprintf("after the restore, %s %s", entry.Application, note))
		}
		if entry.Reason != "" && entry.Reason != notReached {
			service.log.Step(fmt.Sprintf("after the restore, %s: %s", entry.Application, entry.Reason))
		}
	}
}

// stopKeepingWatch ends the watch a finished restore left running, if one is.
func (service *RestoreService) stopKeepingWatch() {
	service.mutex.Lock()
	stop := service.stopWatching
	service.stopWatching = nil
	service.mutex.Unlock()
	if stop != nil {
		stop()
	}
}
