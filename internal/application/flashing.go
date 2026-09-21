package application

import (
	"context"
	"fmt"
)

// letTheFlashingEnd waits until none of the given windows has had its taskbar
// button flash for one full flash series (FR-080).
//
// What it is for, measured by a shell trace on 2026-09-21: an application
// started without the front (FR-077) asks for it anyway; Windows refuses and
// flashes its button instead, in a series that ran on for up to 7 seconds after
// the last entry was settled. A rebuild clears the mark only as it stands, so a
// flash after it puts the mark back. Nothing says a series has ended, so this is
// the one wait the owner allowed that is not an event: one full series with no
// flash, begun again at each flash from one of these windows.
//
// The user's first key press or click and the ceiling end it, as they end every
// wait (FR-079). It answers an error only where the restore was stopped.
func (service *RestoreService) letTheFlashingEnd(
	ctx context.Context,
	state *restoreState,
	windows []*placedWindow,
) error {
	series := service.events.FlashSeries()
	if series <= 0 || service.givenUp(state) {
		return nil
	}
	watch := state.waiting.watch
	// Flashes heard while the restore ran are over by now or will flash again;
	// only those from here on count.
	watch.Flashing()
	began := service.clock.Now()
	quietUntil := began.Add(series)
	restarts := 0
	for {
		deadline := quietUntil
		if state.waiting.deadline.Before(deadline) {
			deadline = state.waiting.deadline
		}
		if !service.clock.Now().Before(deadline) {
			break
		}
		wake, err := watch.Next(ctx, deadline)
		if err != nil {
			return err
		}
		if wake == WakeTouched {
			state.waiting.touched = true
			break
		}
		if wake == WakeFlashed && anyOf(watch.Flashing(), windows) {
			quietUntil = service.clock.Now().Add(series)
			restarts++
		}
	}
	service.log.Step(fmt.Sprintf("waited %s for the taskbar buttons to stop flashing,"+
		" begun again %d time(s)", service.clock.Now().Sub(began), restarts))
	return nil
}

// anyOf reports whether any of the flashed windows is one of the given windows.
func anyOf(flashed []WindowID, windows []*placedWindow) bool {
	for _, id := range flashed {
		for _, placed := range windows {
			if placed.id == id {
				return true
			}
		}
	}
	return false
}
