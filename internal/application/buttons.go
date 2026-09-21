package application

import (
	"context"
	"fmt"

	"github.com/oernster/ScreenState/internal/domain"
)

// rebuildTheButtons has Explorer build afresh the taskbar button of every
// window this restore placed that carries a mark the restore is answerable for
// (FR-075), once those buttons have stopped flashing (FR-080).
//
// What it is for, measured on the reference machine on 2026-09-21: a taskbar
// marks the window last activated on its display. An application that starts
// puts its own window in front, so a desktop assembled at sign-in comes
// back with a marked button on every display. The desktop the user recorded had
// no such marks, so a restore that leaves them has not put the desktop back.
//
// Hiding a window and showing it again is the only answer measured to work: the
// button is dropped and built anew, without the mark. It costs a flicker, which
// the owner ruled on 2026-09-21 is the lesser evil.
//
// Which windows: at sign-in every window this restore placed, since every
// application was starting around then, whether this product started it or
// Windows did. Otherwise only the windows of applications this restore itself
// started, because nothing else can have gained a mark: placing a window no
// longer activates it (FR-074).
//
// It answers an error only where the restore was stopped while it waited.
func (service *RestoreService) rebuildTheButtons(
	ctx context.Context,
	state *restoreState,
	why trigger,
) error {
	var marked []*placedWindow
	for _, placed := range state.arrangedWindows() {
		if why == atSignIn || state.started(placed.application) {
			marked = append(marked, placed)
		}
	}
	if len(marked) == 0 {
		return nil
	}
	if err := service.letTheFlashingEnd(ctx, state, marked); err != nil {
		return err
	}
	rebuilt := 0
	for _, placed := range marked {
		if err := service.desktop.RebuildTaskbarButton(ctx, placed.id); err != nil {
			service.log.Step(fmt.Sprintf("a taskbar button was not rebuilt: %v", err))
			return nil
		}
		rebuilt++
	}
	service.log.Step(fmt.Sprintf("%d taskbar button(s) were built afresh", rebuilt))
	return nil
}

// started reports whether this restore launched an application itself.
func (state *restoreState) started(application domain.ApplicationIdentity) bool {
	for _, launched := range state.launched {
		if launched.Equal(application) {
			return true
		}
	}
	return false
}
