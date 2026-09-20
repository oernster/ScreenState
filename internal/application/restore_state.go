package application

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// pendingEntry is one profile entry not yet satisfied, with what has already
// been tried for it. Each entry converges on its own (FR-055), so everything
// that decides what to try next for one entry is held here rather than being
// inferred from the state of the restore as a whole.
type pendingEntry struct {
	entry domain.Entry
	// launched records that this restore started the application (FR-024);
	// launchedAt is when. A launch that has not had time to settle is never
	// taken for an application holding a window hidden (FR-025).
	launched   bool
	launchedAt time.Time
	// applied counts the placements already applied, so a placement is never
	// applied twice as later windows of the same application appear.
	applied int
	// askedToShow records the second launch that asks a running application to
	// show a window it is holding hidden (FR-056); askedAt is when.
	askedToShow bool
	askedAt     time.Time
}

// placements returns the entry's placements.
func (pending *pendingEntry) placements() []domain.Placement { return pending.entry.Placements }

// wantsPlacement reports whether the entry says where its windows belong. An
// entry with none says only that the application should be running (FR-005).
func (pending *pendingEntry) wantsPlacement() bool { return len(pending.entry.Placements) > 0 }

// placedWindow is a window already placed, waiting for the check FR-033 makes
// once it has had time to settle.
type placedWindow struct {
	application domain.ApplicationIdentity
	id          WindowID
	want        domain.Rect
	state       domain.ShowState
	placedAt    time.Time
	// reapplied marks the one further attempt FR-033 allows. After it, FR-034
	// gives up rather than fight an application for its own window.
	reapplied bool
}

// due reports whether a placed window is ready to be checked again.
func (placed *placedWindow) due(now time.Time, after time.Duration) bool {
	return !now.Before(placed.placedAt.Add(after))
}

// restoreState is everything one restore is holding while it runs.
type restoreState struct {
	report  *Report
	pending []*pendingEntry
	placed  []*placedWindow
	// displays is the identities connected at the last pass, kept so that a
	// change part way through can be recorded rather than passed over (FR-057).
	displays []string
	read     bool
}

// newRestoreState returns the state of a restore about to begin, with every
// entry of the profile outstanding and named in the report.
func newRestoreState(profile domain.Profile, report *Report) *restoreState {
	state := &restoreState{report: report}
	for _, entry := range profile.Entries {
		report.Track(entry.Application)
		state.pending = append(state.pending, &pendingEntry{entry: entry})
	}
	return state
}

// hasPending reports whether any entry is still unsatisfied.
func (state *restoreState) hasPending() bool { return len(state.pending) > 0 }

// settled reports whether the restore has nothing left to do: no entry
// outstanding and no placed window still to be checked.
func (state *restoreState) settled() bool {
	return len(state.pending) == 0 && len(state.placed) == 0
}

// satisfy records an entry as satisfied and stops work on it.
func (state *restoreState) satisfy(pending *pendingEntry) {
	state.report.Satisfy(pending.entry.Application)
	state.drop(pending)
}

// fail records why an entry could not be satisfied and stops work on it.
func (state *restoreState) fail(pending *pendingEntry, format string, args ...any) {
	state.report.Fail(pending.entry.Application, format, args...)
	state.drop(pending)
}

// drop removes an entry from the pending list, leaving the order of the rest
// alone so the restore keeps working through the profile in its own order.
func (state *restoreState) drop(pending *pendingEntry) {
	for at, candidate := range state.pending {
		if candidate == pending {
			state.pending = append(state.pending[:at], state.pending[at+1:]...)
			return
		}
	}
}

// track records a window just placed, for the settle check FR-033 makes.
func (state *restoreState) track(placed *placedWindow) {
	state.placed = append(state.placed, placed)
}

// forget stops checking a placed window.
func (state *restoreState) forget(placed *placedWindow) {
	for at, candidate := range state.placed {
		if candidate == placed {
			state.placed = append(state.placed[:at], state.placed[at+1:]...)
			return
		}
	}
}

// abandonPending stops waiting for every outstanding entry once the ceiling has
// passed, recording each one and why (FR-023).
func (state *restoreState) abandonPending(ceiling time.Duration) {
	outstanding := state.pending
	state.pending = nil
	for _, pending := range outstanding {
		state.report.Fail(pending.entry.Application,
			"still not satisfied when the ceiling of %s passed", ceiling)
	}
}

// abandonRemaining records every entry still outstanding with one reason, for a
// restore that stopped rather than one that ran out of time.
func (state *restoreState) abandonRemaining(reason string) {
	outstanding := state.pending
	state.pending = nil
	state.placed = nil
	for _, pending := range outstanding {
		state.report.Fail(pending.entry.Application, "%s", reason)
	}
}

// noteDisplayChange records displays arriving or going away while a restore is
// in progress. The restore continues either way: abandoning it would leave the
// desktop half arranged, which is worse than the state it started from
// (FR-057).
func (state *restoreState) noteDisplayChange(set displaySet) {
	now := make([]string, 0, len(set.displays))
	for _, display := range set.displays {
		now = append(now, display.Identity.String())
	}
	sort.Strings(now)
	if !state.read {
		state.displays, state.read = now, true
		return
	}
	for _, change := range differences(state.displays, now) {
		state.report.Note("%s", change)
	}
	state.displays = now
}

// differences returns one line per display that arrived or went away between
// two readings.
func differences(before []string, after []string) []string {
	held := make(map[string]struct{}, len(before))
	for _, identity := range before {
		held[identity] = struct{}{}
	}
	still := make(map[string]struct{}, len(after))
	for _, identity := range after {
		still[identity] = struct{}{}
	}
	var changes []string
	for _, identity := range after {
		if _, known := held[identity]; !known {
			changes = append(changes, fmt.Sprintf("display %s was connected during the restore", identity))
		}
	}
	for _, identity := range before {
		if _, remains := still[identity]; !remains {
			changes = append(changes, fmt.Sprintf("display %s went away during the restore", identity))
		}
	}
	return changes
}

// windowsOf returns the readable windows of one application in the order the
// agent first saw them, which is the order FR-037 matches placements in.
func windowsOf(windows []Window, application domain.ApplicationIdentity) []Window {
	var owned []Window
	for _, window := range windows {
		if window.Unreadable != "" {
			continue
		}
		if window.Application.Equal(application) {
			owned = append(owned, window)
		}
	}
	sort.SliceStable(owned, func(one, two int) bool {
		return owned[one].Created.Before(owned[two].Created)
	})
	return owned
}

// visible returns the windows a placement can be applied to. A hidden window
// has nothing to move: the application is holding it; only the application
// can draw it (FR-056).
func visible(windows []Window) []Window {
	var shown []Window
	for _, window := range windows {
		if window.Visible {
			shown = append(shown, window)
		}
	}
	return shown
}

// describePlacement renders a placement for the report.
func describePlacement(placement domain.Placement) string {
	return strings.TrimSpace(fmt.Sprintf("%s on %s, %s",
		placement.Rect, placement.Display, placement.State))
}
