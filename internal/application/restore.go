package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

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
	// strangers says what to do with a window the profile does not name. It is
	// asked each time rather than read once, so the setting in the manager is
	// the setting the next restore runs under.
	strangers StrangerPreferences
	// self is this product's own identity, never put away by a restore: the
	// window the user pressed Apply in is not one of the strangers.
	self domain.ApplicationIdentity
	// splash tells the user the desktop is being arranged and when it is done
	// (FR-078).
	splash Splash

	mutex  sync.Mutex
	active *activeRestore
	last   *Report

	// progress is the last reading of the restore in progress, kept apart from
	// the report so the window can read it while the restore is still writing
	// to that report. It is never nil once the service is built.
	progress atomic.Pointer[RestoreProgress]
}

// trigger says what asked for a restore.
//
// It decides one thing and nothing else: whether the windows the profile does
// not name may be closed rather than minimised (FR-064). Closing belongs to the
// sign-in restore, where the desktop is being made from nothing and the
// applications in the way are the ones that started with Windows. Every other
// restore is a different act: the user is looking at a desktop they are working
// in, so a window of theirs being asked to close is a surprise nobody signed up
// for. That covers Apply and it covers the restore that runs when the agent is
// started by hand, which is the same restore reached by a different route.
type trigger int

const (
	// byHand is a restore the user pressed Apply for.
	byHand trigger = iota
	// atSignIn is the restore that runs because the user signed in (FR-038).
	atSignIn
)

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
	strangers StrangerPreferences,
	splash Splash,
) *RestoreService {
	service := &RestoreService{
		desktop:   desktop,
		processes: processes,
		launcher:  launcher,
		store:     store,
		clock:     clock,
		log:       log,
		policy:    policy,
		self:      self,
		strangers: strangers,
		splash:    splash,
	}
	service.progress.Store(&RestoreProgress{})
	return service
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
// happens at sign-in (FR-038) and also what happens when the agent is started
// by hand with no copy of it already running. The second result is false where
// no profile is marked, which is an answer rather than a fault: the manager
// then states that no default is set (FR-039).
//
// atSignIn says which of the two this is, because it decides whether the
// windows the profile does not name may be closed (FR-064). The agent knows:
// the entry Windows starts it from passes a flag no other launch carries.
func (service *RestoreService) RestoreDefault(
	ctx context.Context,
	signedIn bool,
) (*Report, bool, error) {
	profile, marked, err := service.store.Default(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("reading the default profile: %w", err)
	}
	if !marked {
		service.log.Step("no profile is marked as the default, so nothing was restored")
		return nil, false, nil
	}
	why := byHand
	if signedIn {
		why = atSignIn
	}
	report, err := service.restore(ctx, profile, why)
	return report, true, err
}

// Restore puts the desktop into the state the profile describes and returns the
// report of what it did. It is the restore the user pressed Apply for; the one
// that runs at sign-in goes through RestoreDefault.
func (service *RestoreService) Restore(ctx context.Context, profile domain.Profile) (*Report, error) {
	return service.restore(ctx, profile, byHand)
}

// restore is the restore itself, whatever asked for it.
//
// A restore requested while one is already running replaces it (FR-061): the
// running restore stops before its next action, every window it has already
// placed is left exactly where it is and this one then runs. Nothing is put
// back, because a restore undoes nothing (FR-029).
func (service *RestoreService) restore(
	ctx context.Context,
	profile domain.Profile,
	why trigger,
) (*Report, error) {
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

	service.announcePreparing()
	err := service.guardedRun(runCtx, profile, report, why)

	cancel()
	report.Finish(service.clock.Now())
	service.announceReady(report)
	// Cleared before the done channel is closed, because a restore replacing
	// this one waits on that channel and then states its own reading: clearing
	// afterwards would wipe the newer one.
	service.noteProgress(RestoreProgress{})
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
	why trigger,
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
	return service.run(ctx, profile, report, why)
}

// run is the restore itself: launch what is missing, then place each entry as
// its window appears, until every entry is settled or the ceiling passes.
func (service *RestoreService) run(
	ctx context.Context,
	profile domain.Profile,
	report *Report,
	why trigger,
) error {
	started := report.Started
	deadline := started.Add(service.policy.Ceiling)
	service.log.Step(fmt.Sprintf("restoring %q, %d entries, ceiling %s",
		profile.Name, len(profile.Entries), service.policy.Ceiling))

	state := newRestoreState(profile, report)
	total := len(profile.Entries)
	service.noteProgress(RestoreProgress{Running: true, Profile: profile.Name, Total: total})
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

		service.advance(ctx, state, set, windows, why)
		service.recheck(ctx, state)
		// Read after the pass rather than before it, so the bar shows what has
		// happened rather than what is about to be tried.
		satisfied, _ := report.Counts()
		service.noteProgress(RestoreProgress{
			Running: true, Profile: profile.Name, Satisfied: satisfied, Total: total,
		})

		if state.settled() {
			service.putTheRestAway(ctx, profile, report, why)
			service.nudgeTheTaskbars(ctx)
			service.rebuildTheButtons(ctx, state, why)
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
