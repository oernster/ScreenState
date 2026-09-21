package application

import (
	"context"
	"fmt"
)

// nudgeTheTaskbars posts a click to every taskbar once a restore has settled
// (FR-072), so Explorer draws the icons of the buttons it has left grey.
//
// It runs after every restore rather than at sign-in alone: the fault was
// measured after a sign-in and again mid-session, where closing both packaged
// applications and pressing Apply brought it back.
//
// Failing at it changes nothing about the desktop, so it is noted in the log
// and nowhere else: a user who has their windows back does not need to be told
// that a taskbar was not sent a click.
func (service *RestoreService) nudgeTheTaskbars(ctx context.Context) {
	sent, err := service.desktop.NudgeTaskbars(ctx)
	if err != nil {
		service.log.Step(fmt.Sprintf("the taskbars were not sent a click: %v", err))
		return
	}
	service.log.Step(fmt.Sprintf("%d taskbar(s) were sent a click", sent))
}
