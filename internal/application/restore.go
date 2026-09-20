package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/oernster/ScreenState/internal/domain"
)

// RestoreService puts the desktop into the end state a profile describes.
//
// It holds no window handles between restores and no state about the desktop:
// every pass reads the windows and the displays as they then stand, so a
// display unplugged or an application closed half way through is an ordinary
// reading rather than a surprise.
type RestoreService struct {
	desktop   Desktop
	processes Processes
	launcher  Launcher
	store     ProfileStore
	clock     Clock
	log       Log
	policy    Policy
	// self is this product's own identity, never put away by a restore: the
	// window the user pressed Apply in is not one of the strangers.
	self domain.ApplicationIdentity

	mutex  sync.Mutex
	active *activeRestore
	last   *Report
}

// activeRestore is the restore in progress, enough of it for a newer request to
// stand it down (FR-061).
type activeRestore struct {
	cancel context.CancelFunc
	done   chan struct{}
	report *Report
}

// NewRestoreService returns a restore service over the given collaborators.
func NewRestoreService(
	desktop Desktop,
	processes Processes,
	launcher Launcher,
	store ProfileStore,
	clock Clock,
	log Log,
	policy Policy,
	self domain.ApplicationIdentity,
) *RestoreService {
	return &RestoreService{
		desktop:   desktop,
		processes: processes,
		launcher:  launcher,
		store:     store,
		clock:     clock,
		log:       log,
		policy:    policy,
		self:      self,
	}
}

// Last returns the report of the most recent restore that finished, plus
// whether there has been one. It is what the tray reads to decide whether to
// mark itself and what the report window shows (FR-044, FR-045).
func (service *RestoreService) Last() (*Report, bool) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	return service.last, service.last != nil
}

// RestoreDefault restores the profile marked as the default, which is what
// happens at sign-in (FR-038). The second result is false where no profile is
// marked, which is an answer rather than a fault: the manager then states that
// no default is set (FR-039).
func (service *RestoreService) RestoreDefault(ctx context.Context) (*Report, bool, error) {
	profile, marked, err := service.store.Default(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("reading the default profile: %w", err)
	}
	if !marked {
		service.log.Step("no profile is marked as the default, so nothing was restored")
		return nil, false, nil
	}
	report, err := service.Restore(ctx, profile)
	return report, true, err
}

// Restore puts the desktop into the state the profile describes and returns the
// report of what it did.
//
// A restore requested while one is already running replaces it (FR-061): the
// running restore stops before its next action, every window it has already
// placed is left exactly where it is and this one then runs. Nothing is put
// back, because a restore closes and undoes nothing (FR-029).
func (service *RestoreService) Restore(ctx context.Context, profile domain.Profile) (*Report, error) {
	replaced := service.standDown()

	runCtx, cancel := context.WithCancel(ctx)
	report := NewReport(profile.Name, service.clock.Now())
	report.Replaced = replaced
	if replaced {
		report.Note("this restore replaced one that was already running")
	}

	active := &activeRestore{cancel: cancel, done: make(chan struct{}), report: report}
	service.mutex.Lock()
	service.active = active
	service.mutex.Unlock()

	err := service.guardedRun(runCtx, profile, report)

	cancel()
	report.Finish(service.clock.Now())
	service.retire(active)
	close(active.done)

	service.log.Step(report.Summary())
	return report, err
}

// standDown stops the restore in progress, if there is one, then waits for it
// to stop. It reports whether there was one.
func (service *RestoreService) standDown() bool {
	service.mutex.Lock()
	active := service.active
	if active == nil {
		service.mutex.Unlock()
		return false
	}
	active.report.WasReplaced = true
	active.report.Note("a newer restore was requested, so this one stopped; " +
		"every window already placed was left where it was")
	service.mutex.Unlock()

	active.cancel()
	<-active.done
	return true
}

// retire records a finished restore as the most recent one and clears it from
// the active slot, leaving a later restore alone if one has already replaced it
// there.
func (service *RestoreService) retire(active *activeRestore) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	service.last = active.report
	if service.active == active {
		service.active = nil
	}
}

// guardedRun runs a restore and turns a panic within it into a recorded
// failure. FR-052: an unexpected failure must reach the log and the report and
// must leave the agent able to restore again, since an agent that vanishes
// leaves a half arranged desktop and nothing to read.
func (service *RestoreService) guardedRun(
	ctx context.Context,
	profile domain.Profile,
	report *Report,
) (err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		message := fmt.Sprintf("the restore failed unexpectedly: %v", recovered)
		service.log.Step(message)
		report.Note("%s", message)
		err = fmt.Errorf("restoring %q: %v", profile.Name, recovered)
	}()
	return service.run(ctx, profile, report)
}

// run is the restore itself: launch what is missing, then place each entry as
// its window appears, until every entry is settled or the ceiling passes.
func (service *RestoreService) run(ctx context.Context, profile domain.Profile, report *Report) error {
	started := report.Started
	deadline := started.Add(service.policy.Ceiling)
	service.log.Step(fmt.Sprintf("restoring %q, %d entries, ceiling %s",
		profile.Name, len(profile.Entries), service.policy.Ceiling))

	state := newRestoreState(profile, report)
	service.launchMissing(ctx, state)

	for {
		if err := ctx.Err(); err != nil {
			return service.stopped(err, state)
		}
		set, err := service.readDisplays(ctx, state)
		if err != nil {
			return err
		}
		windows, err := service.desktop.Windows(ctx)
		if err != nil {
			return fmt.Errorf("reading the windows: %w", err)
		}

		service.advance(ctx, state, set, windows)
		service.recheck(ctx, state)

		if state.settled() {
			service.putTheRestAway(ctx, profile, report)
			return nil
		}
		if state.hasPending() && !service.clock.Now().Before(deadline) {
			state.abandonPending(service.policy.Ceiling)
			service.log.Step("the ceiling passed with entries outstanding")
			continue
		}
		if err := service.clock.Sleep(ctx, service.policy.Poll); err != nil {
			return service.stopped(err, state)
		}
	}
}

// putTheRestAway minimises every window the profile does not name (FR-063).
//
// A profile is a statement of what the desktop should look like, so a window
// that is not in it is in the way: the applications that start with Windows and
// are not part of a session were arriving on top of the arrangement and had to
// be put away by hand. Minimising is the whole of it. Nothing is closed and
// nothing is ended, which FR-029 forbids and which the measurement in A-3 says
// would end some applications outright.
//
// It runs once the arrangement is settled, so a window still being placed is
// never mistaken for a stranger. This product's own windows are never put away:
// the manager is where Apply was pressed.
func (service *RestoreService) putTheRestAway(
	ctx context.Context,
	profile domain.Profile,
	report *Report,
) {
	windows, err := service.desktop.Windows(ctx)
	if err != nil {
		report.Note("the windows this profile does not name were left alone: %v", err)
		return
	}
	var put []string
	for _, window := range windows {
		if !service.isStranger(profile, window) {
			continue
		}
		if err := service.desktop.Place(ctx, window.ID, window.Rect, domain.ShowMinimised); err != nil {
			report.Note("%s could not be put away: %v", describeWindow(window), err)
			continue
		}
		put = append(put, describeWindow(window))
	}
	if len(put) == 0 {
		return
	}
	report.Note("put away %d window(s) this profile does not name: %s",
		len(put), strings.Join(put, ", "))
	service.log.Step(fmt.Sprintf("put away %d window(s) outside %q", len(put), profile.Name))
}

// isStranger reports whether a window belongs to none of the profile's
// applications and so is in the way of the arrangement.
//
// A window already minimised is left alone rather than minimised again: there
// is nothing to do and a report listing it would say something happened.
func (service *RestoreService) isStranger(profile domain.Profile, window Window) bool {
	if window.Unreadable != "" || !window.Visible || window.State == domain.ShowMinimised {
		return false
	}
	if window.Application.Validate() != nil || service.self.SameProgram(window.Application) {
		return false
	}
	_, named := profile.Find(window.Application)
	return !named
}

// describeWindow names a window for the report, by its description where it has
// one and by its application where it has not.
func describeWindow(window Window) string {
	if description := strings.TrimSpace(window.Description); description != "" {
		return description
	}
	return window.Application.String()
}

// stopped turns the end of the context into the right answer: a replacement is
// FR-061 working as specified, anything else is the caller cancelling.
func (service *RestoreService) stopped(cause error, state *restoreState) error {
	state.abandonRemaining("the restore stopped before it could be satisfied")
	if state.report.WasReplaced {
		return ErrRestoreReplaced
	}
	if errors.Is(cause, context.Canceled) {
		state.report.Note("the restore was cancelled")
		return nil
	}
	return cause
}

// readDisplays reads the connected displays and records any change since the
// last pass, which a restore continues through rather than abandoning (FR-057).
func (service *RestoreService) readDisplays(ctx context.Context, state *restoreState) (displaySet, error) {
	displays, err := service.desktop.Displays(ctx)
	if err != nil {
		return displaySet{}, fmt.Errorf("reading the displays: %w", err)
	}
	set, err := newDisplaySet(displays)
	if err != nil {
		return displaySet{}, err
	}
	state.noteDisplayChange(set)
	return set, nil
}
