package main

import (
	"context"
	"sync"

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
	version string,
) *App {
	return &App{
		manager:  manager,
		tray:     tray,
		restores: restores,
		captures: captures,
		updates:  updates,
		log:      log,
		version:  version,
	}
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
func (a *App) takeFocus() {
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
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.EventsEmit(a.ctx, "open", viewNames[request])
	a.takeFocus()
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
}

// DetectState reads what the page opens on.
func (a *App) DetectState(prefersDark bool) StateDTO {
	state := StateDTO{
		AppName:     product.Name,
		Tagline:     product.Description,
		Version:     a.version,
		DonateURL:   donateURL,
		PrefersDark: prefersDark,
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
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
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
	if a.ctx != nil {
		wailsruntime.WindowHide(a.ctx)
	}
}
