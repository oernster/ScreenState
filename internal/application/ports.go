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
	// Created orders the windows of one application, oldest first. It is when
	// the agent first saw the window rather than when Windows made it, which
	// the system does not record. It satisfies FR-037, which matches several
	// placements to several windows in first-seen order, because a handle
	// cannot be matched across a session.
	Created time.Time
	// Unreadable is why the desktop could not read this window, empty for one
	// it could. Such a window is still reported rather than dropped, so that a
	// capture can name it in the review instead of quietly losing it (FR-014).
	Unreadable string
	// Description names a window for a person: its title. It is what the review
	// shows for an unreadable window, whose application identity may be all
	// that could not be read. It is never stored and never reaches the log or a
	// report (NFR-PRIV-001), since a title can say what the user is working on.
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
	// NudgeTaskbars posts a click to every taskbar and answers how many were
	// sent one (FR-072). Explorer draws the button of an application started at
	// sign-in without its icon on every display but the first and leaves it so
	// until any taskbar is clicked. It touches no window of any application.
	NudgeTaskbars(ctx context.Context) (int, error)
	// RebuildTaskbarButton has the shell drop a window's taskbar button and
	// build it anew, which is the only measured way to clear the mark a taskbar
	// puts on the window last activated on its display (FR-075). The window
	// keeps its place, its size and its show state; it is not activated. The
	// window flickers while the button is rebuilt.
	RebuildTaskbarButton(ctx context.Context, id WindowID) error
	// StackingOrder answers every top-level window, the one drawn on top first
	// (FR-081). It is read by walking the stacking order itself, never taken
	// from the order Windows answers in, which is arranged for FR-037 rather
	// than for this: measured 2026-09-22, it put the lower of two windows
	// first (OQ-13).
	StackingOrder(ctx context.Context) ([]WindowID, error)
	// Restack stacks the windows so each is drawn directly beneath the one
	// before it, the first taking the highest place any of them holds now. It
	// activates none of them (FR-083, FR-084). It answers the windows it could
	// not restack, each with why; the rest are still stacked in order relative
	// to each other (FR-085, FR-086).
	Restack(ctx context.Context, ids []WindowID) map[WindowID]error
	// Close asks a window to close, which is a request rather than an order:
	// the application decides what to do with it and may show a prompt, take
	// itself to the notification area or end. It is used on a window no
	// profile names and only where the user has asked for it (FR-064).
	Close(ctx context.Context, id WindowID) error
}

// StrangerPreferences is the one thing a restore has to ask the user about: what
// to do with a window the profile does not name (FR-064).
//
// It is read at the moment the restore needs it rather than held, so a setting
// changed while the agent waits in the notification area is the setting that
// governs the next sign-in without anything having to be restarted.
type StrangerPreferences interface {
	CloseStrangers() (bool, error)
	SetCloseStrangers(closing bool) error
}

// CeilingPreferences is how long the user has chosen a restore keeps waiting for
// windows that have not appeared (NFR-PERF-003).
//
// It is read as each restore begins rather than held, for the same reason as
// StrangerPreferences: a ceiling changed in the manager is the ceiling the next
// restore runs to without anything having to be restarted.
type CeilingPreferences interface {
	// Ceiling answers the ceiling the user chose; false where they have chosen
	// none, which is an answer rather than a fault.
	Ceiling() (time.Duration, bool, error)
	SetCeiling(ceiling time.Duration) error
}

// Splash tells the user a restore is arranging the desktop and when it is done
// (FR-078). It answers nothing: a splash that cannot be shown must not stop a
// restore, which matters more than the telling of it.
type Splash interface {
	// Preparing puts the splash up. Where one is already showing it changes
	// back to the waiting words.
	Preparing(message SplashMessage)
	// Ready changes the splash to say the restore has ended. The splash then
	// closes at the user's next key press or mouse click.
	Ready(message SplashMessage)
}

// SplashMessage is what a splash says: a headline, with a line beneath it
// where there is anything to add.
type SplashMessage struct {
	Headline string
	Detail   string
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
	// Unreadable lists the files in the store that are not being offered as
	// profiles, each with the reason: a file that cannot be read, is not a
	// profile or was written in a format this build does not know. Each is left
	// exactly as it is (NFR-REL-002, DATA-003).
	Unreadable(ctx context.Context) ([]UnreadableProfile, error)
}

// UnreadableProfile is a file in the store that is not being offered as a
// profile, with the reason.
type UnreadableProfile struct {
	File   string
	Reason string
}

// Clock is the only source of time in this layer. The domain reads none at all
// and nothing here calls time.Now, so a test over a fake clock settles the
// ceiling without waiting for it.
type Clock interface {
	Now() time.Time
	// Sleep waits, returning the context's error where it ends first. A
	// restore sleeps only to reach the ceiling. It does so only where the desktop
	// could not be watched at all (FR-079).
	Sleep(ctx context.Context, d time.Duration) error
}

// Log records what was attempted and what came of it, step by step (FR-050).
// It answers nothing: a step that cannot be written must not stop a restore,
// since the restore matters more than the record of it.
type Log interface {
	Step(message string)
}

// DesktopEvents tells a restore when the desktop has changed and when the user
// has taken it over (FR-079). A restore reads the desktop only when told to:
// it never looks again on a timer.
type DesktopEvents interface {
	// Watch begins watching the desktop. The watch ends when ctx ends.
	Watch(ctx context.Context) (DesktopWatch, error)
	// FlashSeries answers how long one flash series of a taskbar button lasts
	// on this machine, read when asked (FR-080); zero where it cannot be known,
	// which means nothing is waited for.
	FlashSeries() time.Duration
}

// DesktopWatch is one watch of the desktop, begun by DesktopEvents.
type DesktopWatch interface {
	// Next waits for whichever comes first: the desktop changing, the user's
	// first key press or mouse click since the watch began; the deadline.
	// It answers which. Where the context ends first it answers its error. The
	// user taking over is answered once; after that only changes and the
	// deadline are.
	Next(ctx context.Context, deadline time.Time) (Wake, error)
	// Flashing answers the windows whose taskbar button the shell reported
	// flashing since it was last asked, then forgets them (FR-080).
	Flashing() []WindowID
	// Touched answers, without waiting, whether the user has pressed a key or
	// clicked since the watch began. It is how a step that runs between waits
	// learns the user took over while nothing was listening (FR-087).
	Touched() bool
}

// Wake says why DesktopWatch.Next returned.
type Wake int

const (
	// WakeChanged is Windows reporting a window created, shown, hidden,
	// cloaked, uncloaked, destroyed or moved; the displays changing.
	WakeChanged Wake = iota
	// WakeTouched is the user's first key press or mouse click: they have
	// taken over the desktop, so nothing is waited for any longer.
	WakeTouched
	// WakeDeadline is the ceiling passing.
	WakeDeadline
	// WakeFlashed is the shell reporting a taskbar button flashing; Flashing
	// says whose (FR-080).
	WakeFlashed
)

// Policy holds the timing a restore runs to. It is a value rather than a
// literal in the code for two reasons: NFR-PERF-003 makes the ceiling the
// user's to set; a test needs to state it rather than wait for it. It is the
// only timing there is: everything else a restore waits for is an event
// (FR-079), save the flash series FR-080 waits out, which is read from Windows.
type Policy struct {
	// Ceiling bounds how long a restore keeps waiting for windows that have not
	// appeared (FR-023, NFR-PERF-003).
	Ceiling time.Duration
}

// The ceiling's default and bounds, each named so that no number in this
// package stands on its own.
const (
	DefaultCeiling = 15 * time.Minute

	// MinimumCeiling and MaximumCeiling bound what a user may set the ceiling
	// to (NFR-PERF-003).
	MinimumCeiling = time.Minute
	MaximumCeiling = 60 * time.Minute

	// CeilingStep is the unit a user sets the ceiling in: whole minutes, both
	// in the manager and in the file that keeps the choice.
	CeilingStep = time.Minute
)

// DefaultPolicy returns the timing a restore runs to when the user has set
// none of their own.
func DefaultPolicy() Policy {
	return Policy{Ceiling: DefaultCeiling}
}

// WithCeiling returns a copy of the policy waiting for the given time, held
// within the bounds NFR-PERF-003 sets rather than refused: a ceiling outside
// them is a setting to correct, never a reason not to restore.
func (policy Policy) WithCeiling(ceiling time.Duration) Policy {
	policy.Ceiling = min(max(ceiling, MinimumCeiling), MaximumCeiling)
	return policy
}
