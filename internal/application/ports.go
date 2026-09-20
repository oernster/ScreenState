// Package application holds what capture and restore DO, expressed against
// interfaces rather than against Windows. Everything here can be exercised with
// hand written fakes, on any machine, with no desktop and no clock.
//
// The layer below it, infrastructure, implements these interfaces and is
// imported by nothing here. The layer above, the user interface, calls the
// services in this package and never reaches past them.
package application

import (
	"context"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// WindowID names one live window for as long as the desktop keeps it. It is
// deliberately opaque: the Windows handle behind it does not survive a session,
// so nothing outside infrastructure may store one or reason about its value.
type WindowID uint64

// Window is one top-level window as the desktop reports it now.
//
// It carries no display: which display a window belongs to is a judgement about
// rectangles, made in this layer against the displays as they then stand, so
// that the rule is tested rather than trusted to whatever Windows answered.
type Window struct {
	ID WindowID
	// Application names the application owning the window.
	Application domain.ApplicationIdentity
	// Rect is the window's NORMAL rectangle, the one it occupies when neither
	// minimised nor maximised. A maximised window still has one and it is what
	// decides which display maximising puts it on.
	Rect domain.Rect
	// State is how the window is shown now.
	State domain.ShowState
	// Visible is false for a window the application is holding hidden, which is
	// how an application that starts into the tray presents itself.
	Visible bool
	// Created orders the windows of one application by age, oldest first. It
	// satisfies FR-037, which matches several placements to several windows by
	// creation order, because a handle cannot be matched across a session.
	Created time.Time
	// Unreadable is why the desktop could not read this window, empty for one
	// it could. Such a window is still reported rather than dropped, so that a
	// capture can name it in the review instead of quietly losing it (FR-014).
	Unreadable string
	// Description names a window for a person. It is what the review shows for
	// an unreadable window, whose application identity may be all that could
	// not be read.
	Description string
}

// Display is one connected display as the desktop reports it now.
type Display struct {
	Identity domain.DisplayIdentity
	// Bounds is the whole display in virtual desktop coordinates.
	Bounds domain.Rect
	// WorkArea is Bounds less the taskbar and anything else reserved.
	WorkArea domain.Rect
	// Primary marks the display a placement falls back to when the display it
	// names is not connected (FR-031).
	Primary bool
}

// Desktop reads and changes the windows and displays that exist now. It is the
// whole of what this product does to the machine.
//
// Place sets a window's normal rectangle and then its show state, in that order
// and as one step, because maximising acts on whichever display the normal
// rectangle sits on (FR-027). Nothing here closes a window or ends a process:
// FR-029 forbids both and the interface offers no way to express them, so no
// future caller can reach for one.
type Desktop interface {
	// Windows returns every top-level window owned by the signed-in user.
	Windows(ctx context.Context) ([]Window, error)
	// Window re-reads one window, for the check FR-033 makes after placing it.
	// It reports ErrWindowGone once the window no longer exists.
	Window(ctx context.Context, id WindowID) (Window, error)
	// Displays returns every connected display.
	Displays(ctx context.Context) ([]Display, error)
	// Place sets the window's normal rectangle, then its show state.
	Place(ctx context.Context, id WindowID, rect domain.Rect, state domain.ShowState) error
}

// Processes answers whether an application is running, which a window cannot:
// an application holding every window hidden still counts as running and is
// recorded as such (FR-005), while launching one that is already running is
// forbidden (FR-025).
type Processes interface {
	Running(ctx context.Context, application domain.ApplicationIdentity) (bool, error)
}

// Launcher starts an application by its recorded identity, each kind of
// identity in the way that kind is started.
//
// Launch is also how a running application is asked to show a window it is
// holding hidden (FR-056): the second copy signals the instance already running
// and exits. That is the same act from this layer's point of view, so it is the
// same method; what differs is only why the caller asked for it.
type Launcher interface {
	Launch(ctx context.Context, application domain.ApplicationIdentity) error
}

// ProfileStore keeps the signed-in user's profiles. Every write is atomic: an
// interruption leaves the previous profile or the new one, never half of either
// (FR-006).
type ProfileStore interface {
	// Names lists the stored profile names.
	Names(ctx context.Context) ([]string, error)
	// Load reads one profile, reporting ErrNoSuchProfile when there is none.
	Load(ctx context.Context, name string) (domain.Profile, error)
	// Save writes a profile, replacing one of the same name.
	Save(ctx context.Context, profile domain.Profile) error
	// Delete removes a profile, reporting ErrNoSuchProfile when there is none.
	Delete(ctx context.Context, name string) error
	// Default returns the profile marked as the one applied at sign-in. The
	// second result is false when no profile is marked, which is an answer
	// rather than a fault (FR-039).
	Default(ctx context.Context) (domain.Profile, bool, error)
}

// Clock is the only source of time in this layer. The domain reads none at all
// and nothing here calls time.Now, so a test over a fake clock settles the
// ceiling and the settle-check delay without waiting for either.
type Clock interface {
	Now() time.Time
	// Sleep waits, returning the context's error where it ends first.
	Sleep(ctx context.Context, d time.Duration) error
}

// Log records what was attempted and what came of it, step by step (FR-050).
// It answers nothing: a step that cannot be written must not stop a restore,
// since the restore matters more than the record of it.
type Log interface {
	Step(message string)
}

// Policy holds the timings a restore runs to. They are values rather than
// literals in the code for two reasons: NFR-PERF-003 makes the ceiling the
// user's to set; a test needs to state them rather than wait for them.
type Policy struct {
	// Ceiling bounds how long a restore keeps waiting for windows that have not
	// appeared (FR-023, NFR-PERF-003).
	Ceiling time.Duration
	// SettleCheck is how long after placing a window the agent re-reads it, to
	// catch an application that moved its own window afterwards (FR-033,
	// NFR-PERF-004).
	SettleCheck time.Duration
	// Poll is how often a restore looks again for the windows it is waiting
	// for. The specification fixes no value for it: it is the cost of waiting,
	// traded against how soon a window that has just appeared gets placed;
	// it is bounded by NFR-PERF-005.
	Poll time.Duration
}

// The default timings, each named so that no number in this package stands on
// its own. The ceiling and the settle-check delay are the specification's
// values; the poll interval is this layer's own choice.
const (
	DefaultCeiling     = 15 * time.Minute
	DefaultSettleCheck = 10 * time.Second
	DefaultPoll        = time.Second

	// MinimumCeiling and MaximumCeiling bound what a user may set the ceiling
	// to (NFR-PERF-003).
	MinimumCeiling = time.Minute
	MaximumCeiling = 60 * time.Minute
)

// DefaultPolicy returns the timings a restore runs to when the user has set
// none of their own.
func DefaultPolicy() Policy {
	return Policy{
		Ceiling:     DefaultCeiling,
		SettleCheck: DefaultSettleCheck,
		Poll:        DefaultPoll,
	}
}

// WithCeiling returns a copy of the policy waiting for the given time, held
// within the bounds NFR-PERF-003 sets rather than refused: a ceiling outside
// them is a setting to correct, never a reason not to restore.
func (policy Policy) WithCeiling(ceiling time.Duration) Policy {
	policy.Ceiling = min(max(ceiling, MinimumCeiling), MaximumCeiling)
	return policy
}
