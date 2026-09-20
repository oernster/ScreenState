package application

import (
	"context"
	"fmt"
)

// refreshTheTaskbar asks the shell to paint its taskbars again once a sign-in
// restore has settled (FR-067).
//
// Measured on the reference machine: after a sign-in the taskbar buttons of the
// applications the agent had started were drawn without their icons and stayed
// that way until the user clicked anywhere on the taskbar. The icons were
// there; the drawing was stale. A restore the user asked for never showed it,
// so this is done at sign-in alone rather than after every restore.
//
// It is a repaint asked of the shell, so failing at it changes nothing about
// the desktop: it is noted in the log and nowhere else, since a user who has
// their windows back does not need to be told that a taskbar was not asked to
// redraw itself.
func (service *RestoreService) refreshTheTaskbar(ctx context.Context, why trigger) {
	if why != atSignIn {
		return
	}
	asked, err := service.desktop.RefreshTaskbar(ctx)
	if err != nil {
		service.log.Step(fmt.Sprintf("the taskbar was not asked to redraw itself: %v", err))
		return
	}
	service.log.Step(fmt.Sprintf("%d taskbar(s) were asked to redraw themselves", asked))
}
