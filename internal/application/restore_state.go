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
	// applied counts the placements already applied, so a placement is never
	// applied twice as later windows of the same application appear.
	applied int
	// ran records that this restore has run the application, to start it
	// (FR-024) or to ask it for a window (FR-036, FR-069); windowsAtRun is how
	// many of its windows were showing then. A run is answered by a window
	// appearing. A run not yet answered is waited for, never followed by
	// another: that would be the second copy FR-025 forbids (FR-079).
	ran          bool
	windowsAtRun int
	// asked records that the last run asked a running application for a
	// window, rather than starting it.
	asked bool
	// waitingFor says what the entry is waiting on, for the report where the
	// user takes the desktop over before it comes (FR-079).
	waitingFor string
}

// unanswered reports whether this restore ran the application and no window
// has appeared since.
func (pending *pendingEntry) unanswered(shown int) bool {
	return pending.ran && shown <= pending.windowsAtRun
}

// stillWaiting says what an unanswered run is waiting on, in the words the
// report uses if the wait ends without it.
func (pending *pendingEntry) stillWaiting(shown int) string {
	switch {
	case shown > 0:
		return fmt.Sprintf("opened %d of the %d windows the profile records",
			shown, len(pending.placements()))
	case pending.asked:
		return "is running but showed no window to place"
	default:
		return "was started but showed no window to place"
	}
}

// noteRun records that the application was just run with the given number of
// its windows showing.
func (pending *pendingEntry) noteRun(shown int, asked bool) {
	pending.ran, pending.windowsAtRun, pending.asked = true, shown, asked
}

// placements returns the entry's placements.
func (pending *pendingEntry) placements() []domain.Placement { return pending.entry.Placements }

// wantsPlacement reports whether the entry says where its windows belong. An
// entry with none says only that the application should be running (FR-005).
func (pending *pendingEntry) wantsPlacement() bool { return len(pending.entry.Placements) > 0 }

// placedWindow is a window already placed, watched for the check FR-033 makes
// each time the desktop changes.
type placedWindow struct {
	application domain.ApplicationIdentity
	id          WindowID
	want        domain.Rect
	state       domain.ShowState
	// reapplied marks the one further attempt FR-033 allows. After it, FR-034
	// gives up rather than fight an application for its own window.
	reapplied bool
}

// restoreState is everything one restore is holding while it runs.
type restoreState struct {
	report  *Report
	pending []*pendingEntry
	placed  []*placedWindow
	// arranged records every window this restore placed and launched every
	// application it started. Unlike placed, which is the queue of windows still
	// to be checked, nothing is ever taken out of them: they are what the
	// restore did, which is what deciding whose taskbar button to build afresh
	// needs (FR-075).
	arranged []*placedWindow
	launched []domain.ApplicationIdentity
	// displays is the identities connected at the last pass, kept so that a
	// change part way through can be recorded rather than passed over (FR-057).
	displays []string
	read     bool
	// waiting is what the restore waits on between passes (FR-079).
	waiting waiting
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
	state.arranged = append(state.arranged, placed)
}

// arrangedWindows answers every window this restore placed.
func (state *restoreState) arrangedWindows() []*placedWindow {
	return snapshotPlaced(state.arranged)
}

// noteLaunched records that this restore started an application itself.
func (state *restoreState) noteLaunched(application domain.ApplicationIdentity) {
	state.launched = append(state.launched, application)
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
		if pending.waitingFor != "" {
			state.report.Fail(pending.entry.Application,
				"%s when the ceiling of %s passed", pending.waitingFor, ceiling)
			continue
		}
		state.report.Fail(pending.entry.Application,
			"still not satisfied when the ceiling of %s passed", ceiling)
	}
}

// abandonTouched stops waiting for every outstanding entry once the user has
// taken the desktop over, recording what each was still waiting on (FR-079).
func (state *restoreState) abandonTouched() {
	outstanding := state.pending
	state.pending = nil
	for _, pending := range outstanding {
		waited := pending.waitingFor
		if waited == "" {
			waited = "was still being waited for"
		}
		state.report.Fail(pending.entry.Application, "%s when the desktop was taken over", waited)
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
// agent first saw them, which is the order FR-037 matches placements in. A
// packaged application's window is its own once an update has moved the path
// (FR-071).
func windowsOf(windows []Window, application domain.ApplicationIdentity) []Window {
	var owned []Window
	for _, window := range windows {
		if window.Unreadable != "" {
			continue
		}
		if application.Recognises(window.Application) {
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
