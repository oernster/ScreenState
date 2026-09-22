package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/ui"
)

// The window's own life: coming up, taking the keyboard, going away, being
// closed and the run ending. Kept apart from the shapes that cross to the page,
// which are app.go's.

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// domReady takes the keyboard once the page exists. It does so only for a
// window that is actually on screen. A run started at sign-in waits in the
// notification area; pulling the foreground away from whatever the user is doing would be
// the opposite of waiting quietly.
func (a *App) domReady(context.Context) {
	if a.ctx == nil || !wailsruntime.WindowIsNormal(a.ctx) {
		return
	}
	a.takeFocus()
}

// takeFocus gives the webview the keyboard. See the window package for the race
// it loses without this.
//
// It does nothing at all while the window is not meant to be on screen, which
// is the whole of FR-048 in one guard. A sign-in start still builds the window
// and still loads the page; the page then finds it has no keyboard, which is
// true and is not a fault, then asks for it. The repair ends in WindowShow,
// so a run that was supposed to wait in the notification area put its manager
// up over whatever the user was doing, a few hundred milliseconds after they
// signed in. The step is logged rather than passed over in silence: a guard
// nobody can see working is a guard nobody can tell has stopped.
func (a *App) takeFocus() {
	if !a.onScreen.Load() {
		a.log.Step("the page asked for the keyboard while the window was not on screen: left as it was")
		return
	}
	if ui.TakeWindowFocus() {
		return
	}
	if a.ctx != nil {
		wailsruntime.WindowShow(a.ctx)
	}
}

// TakeKeyboard is called by the page when it finds it has no keyboard. The page
// is the only thing that can tell: from here the window looks focused either
// way.
func (a *App) TakeKeyboard() { a.takeFocus() }

// ShowManager brings the window up on the view the tray asked for. It is called
// from the tray's own thread and from the message the second launch posts, so
// it touches nothing but the Wails runtime, which is safe from any goroutine.
func (a *App) ShowManager(request application.ManagerRequest) {
	if a.ctx == nil {
		return
	}
	a.bringUp()
	wailsruntime.EventsEmit(a.ctx, "open", viewNames[request])
	a.takeFocus()
}

// bringUp puts the window on screen and takes any splash down first, so the
// window the user asked for is the one they see (FR-078).
func (a *App) bringUp() {
	a.splash.Dismiss()
	a.onScreen.Store(true)
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
}

// Surface brings the window up without moving it to another panel. The page
// calls it when a check it ran by itself found something, so an offer reaches a
// user whose agent has been waiting in the notification area since sign-in.
func (a *App) Surface() {
	if a.ctx == nil {
		return
	}
	a.bringUp()
	a.takeFocus()
}

// OpenInBrowser opens a link in the user's own browser, which is where a
// download belongs: this product does not fetch it.
func (a *App) OpenInBrowser(url string) {
	if a.ctx != nil {
		wailsruntime.BrowserOpenURL(a.ctx, url)
	}
}

// Quit ends the agent, which is the same act the tray's own Quit performs.
func (a *App) Quit() {
	a.log.Step("quit from the manager")
	a.quitting.Store(true)
	wailsruntime.Quit(a.ctx)
}

// endRun ends the program when the tray has gone, whether it was quit or never
// appeared at all. It is unexported on purpose: Wails binds what is exported,
// and this is the composition root's business rather than the page's.
//
// The tray is the only way back to a window that hides rather than closing, so
// a run without one has nothing left to be. Without this the process stayed
// alive holding the single-instance lock with no tray and no window; every
// later start then found a copy that was running and could not be seen.
// Measured on 2026-09-20, with a process from an hour earlier still holding it.
func (a *App) endRun() {
	a.quitting.Store(true)
	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
}

// SetDialogOpen is called by the page as a dialog opens and closes. While one
// is open the window's own close command is greyed, so the cross in its caption
// looks as inert as it is; beforeClose is what makes it inert.
func (a *App) SetDialogOpen(open bool) {
	a.dialogOpen.Store(open)
	if !ui.AllowWindowClose(!open) {
		a.log.Step("the window's close command could not be greyed or restored")
	}
}

// beforeClose answers every request to close the window: the cross in its
// caption, Alt+F4 and the taskbar's Close alike, since Wails sends each here
// once the window is not set to hide itself. It answers true to keep the window.
//
// A dialog in the page is modal, so the window is not closed under it.
// Measured 2026-09-22: greying the close command does not stop Windows sending
// the close (TestAGreyedCloseIsNotAGuard); Wails set to hide on close hides the
// window before any hook of this product's is asked. So the refusal has to be
// made here. Otherwise closing the manager hides it, as Hide does; only a
// quit ends the run.
func (a *App) beforeClose(context.Context) bool {
	if a.quitting.Load() {
		return false
	}
	if a.dialogOpen.Load() {
		a.log.Step("the window was asked to close while a dialog was open: it stays open")
		return true
	}
	a.Hide()
	return true
}

// Hide puts the window away without ending the run, which is what closing the
// manager means: the agent goes on waiting in the notification area.
func (a *App) Hide() {
	a.onScreen.Store(false)
	if a.ctx != nil {
		wailsruntime.WindowHide(a.ctx)
	}
}
