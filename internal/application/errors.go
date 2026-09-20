package application

import "errors"

// The errors this layer raises and the ones it expects an implementation of its
// interfaces to raise. They are sentinels, tested with errors.Is, so no caller
// needs to know a struct and no implementation needs to import one.
var (
	// ErrWindowGone is a window that no longer exists. A Desktop reports it
	// from Window; a restore reads it as an answer rather than a fault, since a
	// window the user closed mid restore is ordinary.
	ErrWindowGone = errors.New("window no longer exists")
	// ErrNoSuchProfile is a profile name the store does not hold.
	ErrNoSuchProfile = errors.New("no such profile")
	// ErrProfileNameInUse is a save under a name already stored (FR-002).
	ErrProfileNameInUse = errors.New("profile name is already in use")
	// ErrNoSuchEntry is a request to remove an application a profile does not
	// hold. It is told apart from a successful removal on purpose: a silent
	// success would let the manager report a change that never happened.
	ErrNoSuchEntry = errors.New("no such entry in the profile")
	// ErrNoDisplays is a desktop reporting no connected display. Nothing can be
	// placed, so a restore says so rather than guessing at coordinates.
	ErrNoDisplays = errors.New("no display is connected")
	// ErrReviewCancelled is a capture the user abandoned. It ends the capture
	// with no profile written (FR-016).
	ErrReviewCancelled = errors.New("capture was cancelled")
	// ErrRestoreReplaced is the restore in progress being stood down because a
	// newer one was asked for (FR-061). The replaced restore reports it; the
	// user sees the new restore's report, not an error.
	ErrRestoreReplaced = errors.New("restore was replaced by a newer one")
)
