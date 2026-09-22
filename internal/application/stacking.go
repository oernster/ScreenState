package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/oernster/ScreenState/internal/domain"
)

// restoreTheStacking stacks the windows this restore placed in the order the
// profile records, rank 1 on top (FR-083), without activating any of them
// (FR-084).
//
// It runs once, last among the steps that move windows and before the splash
// says ready, because each earlier step can change the order: an application
// starting puts its own window in front; rebuilding a taskbar button shows the
// window again. Stacking once at the end is the only point at which the order
// can be set and then left alone.
//
// It leaves the order as it finds it where the profile records none (FR-088),
// where the ranks it records cannot be used (DATA-007) and where the user has
// already taken the desktop over (FR-087): from their first key press or click
// they are working and drawing windows over the one they are using would put
// their work behind something they did not ask for.
func (service *RestoreService) restoreTheStacking(
	ctx context.Context,
	profile domain.Profile,
	state *restoreState,
) {
	if ctx.Err() != nil {
		return
	}
	ranked, err := domain.StackingOf(profile.Entries)
	if err != nil {
		state.report.Note("the stacking order the profile records was ignored, because %v", err)
		service.log.Step(fmt.Sprintf("the stacking order was ignored: %v", err))
		return
	}
	if !ranked {
		service.log.Step("the profile records no stacking order, so the order found was kept")
		return
	}
	if state.waiting.touched || state.waiting.watch.Touched() {
		state.report.Note("the stacking order was left as the user found it," +
			" since a key press or click took the desktop over first")
		service.log.Step("the stacking order was not restored: the desktop was taken over")
		return
	}
	windows := inRankOrder(state.arrangedWindows())
	ids := make([]WindowID, 0, len(windows))
	for _, placed := range windows {
		ids = append(ids, placed.id)
	}
	began := service.clock.Now()
	refused := service.desktop.Restack(ctx, ids)
	for _, placed := range windows {
		if why, failed := refused[placed.id]; failed {
			state.report.NoteEntry(placed.application,
				"a window could not be put back in its place in the stacking order: %v", why)
		}
	}
	// NFR-PERF-008 is read off this line: how long the restack took.
	service.log.Step(fmt.Sprintf("restacked %d window(s) in %s, %d could not be",
		len(ids)-len(refused), service.clock.Now().Sub(began), len(refused)))
}

// inRankOrder answers the placed windows ordered by rank, rank 1 first. A
// ranked placement with no window placed is simply not among them, so the rest
// keep their order relative to each other (FR-086).
func inRankOrder(placed []*placedWindow) []*placedWindow {
	sort.SliceStable(placed, func(one, two int) bool { return placed[one].rank < placed[two].rank })
	return placed
}
