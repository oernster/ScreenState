package application

import (
	"context"
	"errors"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// snapshotPending copies the pending list so that satisfying or failing an
// entry part way through a pass cannot disturb the pass itself.
func snapshotPending(pending []*pendingEntry) []*pendingEntry {
	copied := make([]*pendingEntry, len(pending))
	copy(copied, pending)
	return copied
}

// snapshotPlaced copies the placed list, for the same reason.
func snapshotPlaced(placed []*placedWindow) []*placedWindow {
	copied := make([]*placedWindow, len(placed))
	copy(copied, placed)
	return copied
}

// launchMissing starts every application the profile records as running that is
// not running (FR-024). It runs once, at the start, so those applications load
// alongside each other rather than one at a time.
//
// Nothing already running is launched again here (FR-025). The one exception,
// asking a running application to show a hidden window, belongs to the entry
// that needs it and is made later, by askToShow.
func (service *RestoreService) launchMissing(ctx context.Context, state *restoreState) {
	for _, pending := range snapshotPending(state.pending) {
		if !pending.entry.Running {
			// The profile records the application as not running. A restore
			// ends nothing and closes nothing (FR-029), so there is nothing to
			// do about it beyond saying so.
			state.report.NoteEntry(pending.entry.Application,
				"is recorded as not running, so nothing was done to it")
			state.satisfy(pending)
			continue
		}
		running, err := service.processes.Running(ctx, pending.entry.Application)
		if err != nil {
			state.fail(pending, "could not be read: %v", err)
			continue
		}
		if running {
			continue
		}
		if err := service.launcher.Launch(ctx, pending.entry.Application); err != nil {
			// FR-026: name the application and the reason, then carry on with
			// the rest rather than ending the restore.
			state.fail(pending, "could not be launched: %v", err)
			continue
		}
		pending.launched = true
		state.report.NoteEntry(pending.entry.Application, "was not running, so it was launched")
	}
}

// advance takes every entry as far as the windows now open allow (FR-055).
func (service *RestoreService) advance(
	ctx context.Context,
	state *restoreState,
	set displaySet,
	windows []Window,
) {
	for _, pending := range snapshotPending(state.pending) {
		service.advanceEntry(ctx, state, set, windows, pending)
	}
}

// advanceEntry takes one entry as far as it can go this pass.
func (service *RestoreService) advanceEntry(
	ctx context.Context,
	state *restoreState,
	set displaySet,
	windows []Window,
	pending *pendingEntry,
) {
	owned := windowsOf(windows, pending.entry.Application)
	if !pending.wantsPlacement() {
		service.advanceWithoutPlacement(ctx, state, pending)
		return
	}
	shown := visible(owned)
	if len(shown) <= pending.applied {
		service.askToShow(ctx, state, pending, shown)
		return
	}
	service.applyPlacements(ctx, state, set, shown, pending)
}

// advanceWithoutPlacement settles an entry that says only that the application
// should be running (FR-005). It is satisfied the moment it is running; where
// it is not, the restore keeps waiting for the launch to take effect.
func (service *RestoreService) advanceWithoutPlacement(
	ctx context.Context,
	state *restoreState,
	pending *pendingEntry,
) {
	running, err := service.processes.Running(ctx, pending.entry.Application)
	if err != nil {
		state.fail(pending, "could not be read: %v", err)
		return
	}
	if running {
		state.satisfy(pending)
	}
}

// applyPlacements applies the entry's placements to its windows, oldest window
// to first placement (FR-037); it settles the entry once every placement has
// been applied.
func (service *RestoreService) applyPlacements(
	ctx context.Context,
	state *restoreState,
	set displaySet,
	shown []Window,
	pending *pendingEntry,
) {
	placements := pending.placements()
	for pending.applied < len(placements) && pending.applied < len(shown) {
		placement := placements[pending.applied]
		window := shown[pending.applied]
		rect, substitution := set.place(placement, placement.Rect)
		if substitution != "" {
			state.report.NoteEntry(pending.entry.Application, "%s", substitution)
		}
		if err := service.desktop.Place(ctx, window.ID, rect, placement.State); err != nil {
			// FR-035: a window Windows will not let this process move.
			state.fail(pending, "a window could not be moved: %v", err)
			return
		}
		state.report.NoteEntry(pending.entry.Application, "placed at %s", describePlacement(
			domain.Placement{Display: placement.Display, Rect: rect, State: placement.State}))
		state.track(&placedWindow{
			application: pending.entry.Application,
			id:          window.ID,
			want:        rect,
			state:       placement.State,
			placedAt:    service.clock.Now(),
		})
		pending.applied++
	}
	if pending.applied < len(placements) {
		return
	}
	if extra := len(shown) - len(placements); extra > 0 {
		// FR-037: say so rather than leave the user wondering why one window of
		// an application moved and another did not.
		state.report.NoteEntry(pending.entry.Application,
			"has %d more window(s) open than the profile records, which were left alone", extra)
	}
	state.satisfy(pending)
}

// askToShow deals with an entry whose application is running with no window to
// place. FR-036 and FR-056: the application is run again, which signals the
// instance already running to show and draw its own window. Acting on the
// hidden window from outside was measured producing an empty frame, so it is
// not done.
func (service *RestoreService) askToShow(
	ctx context.Context,
	state *restoreState,
	pending *pendingEntry,
	shown []Window,
) {
	if len(shown) > 0 {
		// Some windows are open and placed; the entry is waiting for the rest
		// to appear. The ceiling decides how long that waiting lasts.
		return
	}
	running, err := service.processes.Running(ctx, pending.entry.Application)
	if err != nil {
		state.fail(pending, "could not be read: %v", err)
		return
	}
	if !running {
		// Still starting or never started. Either way there is nothing to ask.
		return
	}
	if !pending.askedToShow {
		if err := service.launcher.Launch(ctx, pending.entry.Application); err != nil {
			state.fail(pending, "is running with no window and could not be asked to show one: %v", err)
			return
		}
		pending.askedToShow = true
		pending.askedAt = service.clock.Now()
		state.report.NoteEntry(pending.entry.Application,
			"was running with no visible window, so it was asked to show one")
		return
	}
	if service.clock.Now().Sub(pending.askedAt) >= service.policy.SettleCheck {
		state.fail(pending, "is running but showed no window to place")
	}
}

// recheck makes the check FR-033 requires: a window is read again once it has
// had time to settle; it is put back once where the application has moved it. After
// that one further attempt, FR-034 gives up rather than fight for the window.
func (service *RestoreService) recheck(ctx context.Context, state *restoreState) {
	now := service.clock.Now()
	for _, placed := range snapshotPlaced(state.placed) {
		if !placed.due(now, service.policy.SettleCheck) {
			continue
		}
		window, err := service.desktop.Window(ctx, placed.id)
		if err != nil {
			service.recheckUnreadable(state, placed, err)
			continue
		}
		if window.Rect == placed.want && window.State == placed.state {
			state.forget(placed)
			continue
		}
		service.reapply(ctx, state, placed, window, now)
	}
}

// recheckUnreadable deals with a window that could not be read again. A window
// the user closed after it was placed is ordinary and is not a failure of the
// restore, which placed it as the profile asked.
func (service *RestoreService) recheckUnreadable(state *restoreState, placed *placedWindow, err error) {
	state.forget(placed)
	if errors.Is(err, ErrWindowGone) {
		state.report.NoteEntry(placed.application, "a window was closed after it was placed")
		return
	}
	state.report.NoteEntry(placed.application, "a window could not be read again after placing: %v", err)
}

// reapply puts a window back where the profile says, once. A window that has
// moved again after that is reported and left alone.
func (service *RestoreService) reapply(
	ctx context.Context,
	state *restoreState,
	placed *placedWindow,
	window Window,
	now time.Time,
) {
	if placed.reapplied {
		state.forget(placed)
		state.report.Fail(placed.application,
			"a window did not stay where it was put; it now sits at %s showing %s",
			window.Rect, window.State)
		return
	}
	if err := service.desktop.Place(ctx, placed.id, placed.want, placed.state); err != nil {
		state.forget(placed)
		state.report.Fail(placed.application, "a window moved itself and could not be moved back: %v", err)
		return
	}
	placed.reapplied = true
	placed.placedAt = now
	state.report.NoteEntry(placed.application, "moved itself after being placed, so it was placed again")
}
