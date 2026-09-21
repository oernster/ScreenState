package application

import (
	"context"
	"fmt"
)

// settleTheButtons puts the taskbar button of every window this restore placed
// back in its ordinary state (FR-073).
//
// Placing a window leaves its button lit, which is the state Windows uses for a
// window that wants looking at; the user meets a desktop where everything the
// restore touched is asking for attention. Measured on the reference machine on
// 2026-09-21, where a window the restore had placed reported its caption drawn
// active while another window held the foreground.
//
// Only the windows this restore placed are settled. A window it never touched
// may be lit because something really does want the user; taking that off would
// hide a message meant for them.
//
// Failing at it changes nothing about the desktop, so it is noted in the log
// and nowhere else.
func (service *RestoreService) settleTheButtons(ctx context.Context, state *restoreState) {
	settled, lit := 0, 0
	for _, id := range state.arrangedWindows() {
		wasLit, err := service.desktop.StopDrawingAttention(ctx, id)
		if err != nil {
			service.log.Step(fmt.Sprintf("a taskbar button was not settled: %v", err))
			return
		}
		settled++
		if wasLit {
			lit++
		}
	}
	if settled == 0 {
		return
	}
	service.log.Step(fmt.Sprintf(
		"%d taskbar button(s) were settled, %d of which were drawing attention", settled, lit))
}
