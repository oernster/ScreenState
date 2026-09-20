package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oernster/ScreenState/internal/infrastructure/setup"
	"github.com/oernster/ScreenState/internal/infrastructure/window"
)

// App is the Wails facade for the setup program. Everything the page can do
// goes through a method here; every one of them is a call into the install
// policy: no rule about what an install does lives in this file.
type App struct {
	ctx           context.Context
	payload       []byte
	version       string
	uninstallMode bool
	prefersDark   bool
}

// NewApp builds the facade. Started with the uninstall flag, which is what the
// Apps list passes back, setup opens on the removal screen rather than on the
// manage one.
func NewApp(payload []byte, version string, prefersDark bool) *App {
	uninstall := len(os.Args) > 1 && os.Args[1] == setup.UninstallFlag
	return &App{
		payload:       payload,
		version:       version,
		uninstallMode: uninstall,
		prefersDark:   prefersDark,
	}
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// domReady takes the keyboard once the page exists. See the window package for
// the race this loses without it.
func (a *App) domReady(context.Context) { a.show() }

// show gives the webview the keyboard, falling back to asking Wails for the
// window.
func (a *App) show() {
	if window.TakeFocus() {
		return
	}
	if a.ctx != nil {
		wailsruntime.WindowShow(a.ctx)
	}
}

// TakeKeyboard is called by the page when it finds it has no keyboard.
//
// The page is the only thing that can tell: from here the window looks focused
// either way. It is the second half of the repair, because domReady runs before
// the webview is necessarily ready to keep what it is given.
func (a *App) TakeKeyboard() { a.show() }

// StateDTO describes what setup should offer, given what is already on the
// machine.
//
// AppName is sent because the page must not write the product's name down. A
// page has no compiler behind it, so a name written there survives a rename
// with nothing said and setup then announces a product that no longer exists.
type StateDTO struct {
	AppName          string `json:"appName"`
	Tagline          string `json:"tagline"`
	Mode             string `json:"mode"`
	Relation         string `json:"relation"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`
	ThisVersion      string `json:"thisVersion"`
	InstallDir       string `json:"installDir"`
	StateDir         string `json:"stateDir"`
	LaunchOnBoot     bool   `json:"launchOnBoot"`
	StartMenu        bool   `json:"startMenu"`
	Desktop          bool   `json:"desktop"`
	PrefersDark      bool   `json:"prefersDark"`
}

// OptionsDTO carries the choices made on the install or reinstall screen.
type OptionsDTO struct {
	StartMenu    bool `json:"startMenu"`
	Desktop      bool `json:"desktop"`
	LaunchOnBoot bool `json:"launchOnBoot"`
}

// Progress is emitted on the "progress" event while a long operation runs.
type Progress struct {
	Pct int    `json:"pct"`
	Msg string `json:"msg"`
}

// relationNames turn the comparison into the word the page routes on.
var relationNames = map[setup.Relation]string{
	setup.Newer: "newer",
	setup.Same:  "same",
	setup.Older: "older",
}

// mode names the conversation this run is having. The page reads these rather
// than working the same decision out a second time from the parts.
const (
	modeInstall   = "install"
	modeManage    = "manage"
	modeUninstall = "uninstall"
)

// DetectState inspects the machine once and returns the mode setup opens in.
// One reading decides the screen, its heading, the options on it and the
// buttons under it, which is what stops those four drifting apart.
func (a *App) DetectState() StateDTO {
	dir, _ := setup.InstallDir()
	state, _ := setup.StateDir()
	installedVersion, installed := setup.InstalledVersion()
	shortcuts := setup.CurrentShortcuts()

	mode := modeInstall
	switch {
	case a.uninstallMode:
		mode = modeUninstall
	case installed:
		mode = modeManage
	}
	relation := setup.Same
	if installed {
		relation = setup.Compare(a.version, installedVersion)
	}
	return StateDTO{
		AppName:          setup.AppName,
		Tagline:          setup.Tagline,
		Mode:             mode,
		Relation:         relationNames[relation],
		Installed:        installed,
		InstalledVersion: installedVersion,
		ThisVersion:      a.version,
		InstallDir:       dir,
		StateDir:         state,
		LaunchOnBoot:     setup.IsLaunchOnBoot(),
		StartMenu:        shortcuts.StartMenu,
		Desktop:          shortcuts.Desktop,
		PrefersDark:      a.prefersDark,
	}
}

// AppRunning reports whether the agent is open, so the page can offer to close
// it rather than failing later on a locked executable.
func (a *App) AppRunning() bool { return setup.IsAppRunning() }

// CloseRunningApp ends the running agent so setup can proceed.
func (a *App) CloseRunningApp() error { return setup.CloseRunningApp() }

// Install performs a fresh install, an update, a step back a version or a
// reinstall. All four are the same act: write the files, then apply the options
// as given.
func (a *App) Install(choices OptionsDTO) error { return a.write(choices) }

// Repair writes the files again and re-registers the agent, leaving every
// option exactly as it stands. It is the quick fix for a damaged install, as
// distinct from a reinstall, which asks for the options again.
func (a *App) Repair() error {
	shortcuts := setup.CurrentShortcuts()
	return a.write(OptionsDTO{
		StartMenu:    shortcuts.StartMenu,
		Desktop:      shortcuts.Desktop,
		LaunchOnBoot: setup.IsLaunchOnBoot(),
	})
}

// The ladder the bar climbs. The steps are weighted by how long each actually
// takes rather than by how many there are: the shortcuts shell out to the
// Windows Script Host once each and are far the slowest part, so a bar weighted
// by step count would reach the end before the work did and sit there, which
// reads as a bar that never worked.
const (
	pctExtracting  = 10
	pctRegistering = 45
	pctChoices     = 60
	pctComplete    = 100
)

// write is the single install path behind Install and Repair.
func (a *App) write(choices OptionsDTO) error {
	// Ask whether the agent is running before touching a single file:
	// extracting over a locked executable fails part way and leaves a half
	// written install.
	if setup.IsAppRunning() {
		return setup.ErrAppRunning
	}
	dir, err := setup.InstallDir()
	if err != nil {
		return err
	}

	a.progress(pctExtracting, "Extracting files...")
	if err := setup.ExtractZip(a.payload, dir); err != nil {
		return fmt.Errorf("extract files: %w", err)
	}
	exePath := filepath.Join(dir, setup.ExeName)

	a.progress(pctRegistering, "Registering the agent...")
	if err := a.register(dir, exePath); err != nil {
		return err
	}

	a.progress(pctChoices, "Applying your choices...")
	setup.ApplyShortcuts(exePath, dir, setup.Shortcuts{
		StartMenu: choices.StartMenu,
		Desktop:   choices.Desktop,
	})
	if err := setup.SetLaunchOnBoot(exePath, choices.LaunchOnBoot); err != nil {
		return fmt.Errorf("configure the sign-in entry: %w", err)
	}

	a.progress(pctComplete, "Done.")
	return nil
}

// register leaves a copy of setup beside the agent and writes the Apps list
// entry that points at it.
func (a *App) register(dir, exePath string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate setup: %w", err)
	}
	uninstallExe := filepath.Join(dir, setup.UninstallExeName)
	if err := setup.CopyFile(self, uninstallExe); err != nil {
		return fmt.Errorf("write the uninstaller: %w", err)
	}
	sizeKB, _ := setup.DirSizeKB(dir)
	if err := setup.WriteUninstallEntry(setup.UninstallInfo{
		Version:      a.version,
		InstallDir:   dir,
		UninstallExe: uninstallExe,
		IconPath:     exePath,
		EstimatedKB:  sizeKB,
	}); err != nil {
		return fmt.Errorf("register the agent: %w", err)
	}
	return nil
}

// The ladder an uninstall climbs, weighted the same way: the shortcuts and the
// registry are quick, the scheduled removal of the files is handed to a shell
// that outlives this process, so the bar is nearly home by the time it starts.
const (
	pctShortcuts = 25
	pctRegistry  = 55
	pctProfiles  = 75
	pctFiles     = 90
)

// Uninstall removes the shortcuts, the sign-in entry, the registry record and
// the installed files. The captured profiles and the log go too when the user
// asks for them.
func (a *App) Uninstall(removeState bool) error {
	// The scheduled deletion cannot remove a locked executable, so a running
	// agent has to close first.
	if setup.IsAppRunning() {
		return setup.ErrAppRunning
	}
	dir, err := setup.InstallDir()
	if err != nil {
		return err
	}

	a.progress(pctShortcuts, "Removing shortcuts...")
	setup.RemoveShortcuts()
	_ = setup.SetLaunchOnBoot("", false)

	a.progress(pctRegistry, "Removing registry entries...")
	_ = setup.RemoveUninstallEntry()

	if removeState {
		a.progress(pctProfiles, "Removing your profiles...")
		if state, stateErr := setup.StateDir(); stateErr == nil {
			_ = setup.RemoveTree(state)
		}
	}

	a.progress(pctFiles, "Removing files...")
	setup.ScheduleDirDeletion(dir)

	a.progress(pctComplete, "Done.")
	return nil
}

// LaunchApp starts the installed agent, backing the "start it when setup
// finishes" option.
func (a *App) LaunchApp() error { return setup.LaunchApp() }

// SetLaunchOnBoot toggles the sign-in entry live from the manage screen, where
// there is nothing to install and so nothing for a go-ahead button to apply.
func (a *App) SetLaunchOnBoot(enabled bool) error {
	dir, err := setup.InstallDir()
	if err != nil {
		return err
	}
	return setup.SetLaunchOnBoot(filepath.Join(dir, setup.ExeName), enabled)
}

// SetShortcuts applies the shortcut boxes live from the manage screen, so
// unticking one takes the shortcut away there and then.
func (a *App) SetShortcuts(startMenu, desktop bool) error {
	dir, err := setup.InstallDir()
	if err != nil {
		return err
	}
	setup.ApplyShortcuts(filepath.Join(dir, setup.ExeName), dir, setup.Shortcuts{
		StartMenu: startMenu,
		Desktop:   desktop,
	})
	return nil
}

// Quit closes the setup program.
func (a *App) Quit() { wailsruntime.Quit(a.ctx) }

// progress reports how far a long operation has got.
func (a *App) progress(pct int, msg string) {
	wailsruntime.EventsEmit(a.ctx, "progress", Progress{Pct: pct, Msg: msg})
}
