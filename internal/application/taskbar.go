package application

import (
	"context"
	"fmt"
)

// refreshTheTaskbar tells the shell its icons may have changed, once a sign-in
// restore has settled (FR-067).
//
// Measured on the reference machine: after a sign-in the taskbar buttons of the
// applications the agent had started were drawn grey and stayed that way until
// the user clicked anywhere on the taskbar. A restore the user asked for never
// showed it, so this is done at sign-in alone rather than after every restore.
//
// Asking every taskbar to repaint was tried first and measured not to fix it,
// which is what says the shell is not holding a stale drawing: it is holding
// the answer that there was no icon to draw. This asks it to work that out
// again.
//
// Failing at it changes nothing about the desktop, so it is noted in the log
// and nowhere else: a user who has their windows back does not need to be told
// that the shell was not spoken to.
func (service *RestoreService) refreshTheTaskbar(ctx context.Context, why trigger) {
	if why != atSignIn {
		return
	}
	if err := service.desktop.RefreshShellIcons(ctx); err != nil {
		service.log.Step(fmt.Sprintf("the shell was not told its icons may have changed: %v", err))
		return
	}
	service.log.Step("the shell was told its icons may have changed")
}
