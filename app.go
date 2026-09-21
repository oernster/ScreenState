package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/product"
	"github.com/oernster/ScreenState/internal/ui"
)

// donateURL is where FR-060 sends a user who wants to support the project. The
// wording beside it matters as much as the link: nothing is held back, so an
// ask that implied otherwise would be false.
const donateURL = "https://www.paypal.com/ncp/payment/6FMTGJYFJXFTE"

// viewNames turn a request from the tray into the word the page opens on.
var viewNames = map[application.ManagerRequest]string{
	application.ManagerProfiles: "profiles",
	application.ManagerCapture:  "capture",
	application.ManagerReport:   "report",
}

// App is the Wails facade for the manager window. Every method here is a call
// into the application layer: no rule about what a profile means lives in this
// file, which is why none of them can be got wrong here without a test in that
// layer failing first.
type App struct {
	ctx      context.Context
	manager  *application.ManagerService
	tray     *application.TrayService
	restores *application.RestoreService
	captures *application.CaptureService
	updates  *application.UpdateService
	log      application.Log
	version  string
	// splash is taken down whenever the manager is brought up (FR-078), since
	// it sits above every window and would cover the one the user asked for.
	splash interface{ Dismiss() }

	// onScreen says whether the window is meant to be on screen at all.
	//
	// A run started at sign-in carries the hidden flag and begins with it
	// false; nothing but a deliberate ask (the tray, a second launch, an
	// update offer) turns it true. It is read from the Wails thread, from the
	// tray's own thread and from the page, so it is atomic rather than guarded
	// by the mutex below, which belongs to the review.
	onScreen atomic.Bool

	// review holds the capture the user is looking at, between reading the
	// desktop and confirming what to keep. It lives here because a review is
	// not stored anywhere until it is confirmed (FR-011): a cancelled one
	// leaves nothing behind (FR-016), including on disk.
	mutex  sync.Mutex
	review application.Review
}

// NewApp builds the facade.
func NewApp(
	manager *application.ManagerService,
	tray *application.TrayService,
	restores *application.RestoreService,
	captures *application.CaptureService,
	updates *application.UpdateService,
	log application.Log,
	splash interface{ Dismiss() },
	version string,
	hidden bool,
) *App {
	app := &App{
		manager:  manager,
		tray:     tray,
		restores: restores,
		captures: captures,
		updates:  updates,
		log:      log,
		splash:   splash,
		version:  version,
	}
	app.onScreen.Store(!hidden)
	return app
}

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

// StateDTO is what the page needs before it can draw anything.
//
// AppName and Tagline are sent because the page must not write them down: a
// page has no compiler behind it, so a name written there survives a rename
// with nothing said.
type StateDTO struct {
	AppName      string `json:"appName"`
	Tagline      string `json:"tagline"`
	Version      string `json:"version"`
	DonateURL    string `json:"donateUrl"`
	PrefersDark  bool   `json:"prefersDark"`
	LaunchOnBoot bool   `json:"launchOnBoot"`
	UpdateCheck  bool   `json:"updateCheck"`
	// StartupError says why the sign-in setting could not be read, empty where
	// it could. A setting shown as off because the registry refused is a lie
	// the user would act on.
	StartupError string `json:"startupError"`
	// UpdateError says the same about the update setting, for the same reason.
	UpdateError string `json:"updateError"`
	// CloseUnnamed is what a restore does with the windows a profile does not
	// name: close them when it is true, minimise them when it is false
	// (FR-064).
	CloseUnnamed bool `json:"closeUnnamed"`
	// CloseUnnamedError says why that setting could not be read, for the same
	// reason as the two above.
	CloseUnnamedError string `json:"closeUnnamedError"`
	// CeilingMinutes is how long a restore waits for windows that have not
	// appeared (NFR-PERF-003), in the whole minutes the user sets it in, with
	// the bounds the setting may take. The bounds are sent rather than written
	// into the page, so the field cannot offer a value the program would change.
	CeilingMinutes int `json:"ceilingMinutes"`
	CeilingMinimum int `json:"ceilingMinimum"`
	CeilingMaximum int `json:"ceilingMaximum"`
	// CeilingError says why the ceiling could not be read, for the same reason
	// as the errors above.
	CeilingError string `json:"ceilingError"`
}

// minutes states a ceiling in the unit the user sets it in.
func minutes(ceiling time.Duration) int { return int(ceiling / application.CeilingStep) }

// DetectState reads what the page opens on.
func (a *App) DetectState(prefersDark bool) StateDTO {
	state := StateDTO{
		AppName:     product.Name,
		Tagline:     product.Description,
		Version:     a.version,
		DonateURL:   donateURL,
		PrefersDark: prefersDark,

		CeilingMinimum: minutes(application.MinimumCeiling),
		CeilingMaximum: minutes(application.MaximumCeiling),
	}
	if ceiling, err := a.restores.Ceiling(); err != nil {
		state.CeilingError = err.Error()
	} else {
		state.CeilingMinutes = minutes(ceiling)
	}
	if enabled, err := a.manager.StartsWithWindows(); err != nil {
		state.StartupError = err.Error()
	} else {
		state.LaunchOnBoot = enabled
	}
	if enabled, err := a.updates.Enabled(); err != nil {
		state.UpdateError = err.Error()
	} else {
		state.UpdateCheck = enabled
	}
	if closing, err := a.restores.CloseStrangers(); err != nil {
		state.CloseUnnamedError = err.Error()
	} else {
		state.CloseUnnamed = closing
	}
	return state
}

// ProfileDTO is one profile in the manager's list.
type ProfileDTO struct {
	Name    string `json:"name"`
	Default bool   `json:"isDefault"`
	Entries int    `json:"entries"`
}

// PlacementDTO is one placement of one entry.
type PlacementDTO struct {
	Display string `json:"display"`
	Rect    string `json:"rect"`
	State   string `json:"state"`
}

// EntryDTO is one application within a profile.
type EntryDTO struct {
	Application string         `json:"application"`
	Kind        string         `json:"kind"`
	Running     bool           `json:"running"`
	Placements  []PlacementDTO `json:"placements"`
}

// stated answers a slice the page can rely on being a list. A nil slice is
// marshalled as null rather than as an empty array, so a page that reads a
// length or walks it throws where nothing is wrong at all. Measured on
// 2026-09-20: a capture with nothing unreadable handed the manager a null, the
// throw left its promise rejected with nobody to catch it and the window sat on
// "Reading the desktop" with the capture already read and waiting behind it.
// Every slice leaving here for the page goes through this.
func stated[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

// ReviewEntryDTO is one candidate in a capture the user is reviewing.
type ReviewEntryDTO struct {
	Application string `json:"application"`
	Kind        string `json:"kind"`
	Windows     int    `json:"windows"`
}

// ReviewDTO is a capture as the review screen shows it. Nothing here has been
// written: a review is a proposal until the user confirms it (FR-011).
type ReviewDTO struct {
	Entries    []ReviewEntryDTO `json:"entries"`
	Unreadable []string         `json:"unreadable"`
	// Background are the applications running with every window hidden,
	// shown unticked: kept only where the user ticks them (FR-005).
	Background []ReviewEntryDTO `json:"background"`
}

// EntryReportDTO is one line of the report of the most recent restore.
type EntryReportDTO struct {
	Application string   `json:"application"`
	Satisfied   bool     `json:"satisfied"`
	Reason      string   `json:"reason"`
	Notes       []string `json:"notes"`
}

// ReportDTO is the report of the most recent restore (FR-044).
type ReportDTO struct {
	Held    bool             `json:"held"`
	Profile string           `json:"profile"`
	Summary string           `json:"summary"`
	Notes   []string         `json:"notes"`
	Entries []EntryReportDTO `json:"entries"`
}

// UpdateDTO is what one update check found, as the page shows it.
type UpdateDTO struct {
	// Enabled is false where the user has turned the check off, in which case
	// nothing was asked of the network at all (FR-059).
	Enabled bool `json:"enabled"`
	// Reached is false where the feed could not be reached, which is told apart
	// from finding nothing: those two mean opposite things to a user who asked.
	Reached     bool   `json:"reached"`
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	Available   bool   `json:"available"`
	Skipped     bool   `json:"skipped"`
	DownloadURL string `json:"downloadUrl"`
	PageURL     string `json:"pageUrl"`
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
	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
}

// Hide puts the window away without ending the run, which is what closing the
// manager means: the agent goes on waiting in the notification area.
func (a *App) Hide() {
	a.onScreen.Store(false)
	if a.ctx != nil {
		wailsruntime.WindowHide(a.ctx)
	}
}
