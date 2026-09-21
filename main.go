// Command ScreenState is the agent: it restores the default profile after
// sign-in, waits in the notification area and presents the manager when asked.
//
// This file is the composition root. It is the only place that knows both the
// application layer and the Windows layer, which is why the structural suite
// names it and fails any other file that learns both. Everything here is
// wiring: no rule about what a profile means lives in this file.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
	"github.com/oernster/ScreenState/internal/infrastructure/clock"
	"github.com/oernster/ScreenState/internal/infrastructure/instance"
	"github.com/oernster/ScreenState/internal/infrastructure/runlog"
	"github.com/oernster/ScreenState/internal/infrastructure/settings"
	"github.com/oernster/ScreenState/internal/infrastructure/setup"
	"github.com/oernster/ScreenState/internal/infrastructure/startup"
	"github.com/oernster/ScreenState/internal/infrastructure/store"
	"github.com/oernster/ScreenState/internal/infrastructure/update"
	"github.com/oernster/ScreenState/internal/infrastructure/win32"
	"github.com/oernster/ScreenState/internal/product"
	"github.com/oernster/ScreenState/internal/ui"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	windowsoptions "github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is stamped in at build time from the VERSION file. It is a var and
// not a const on purpose: the linker's -X flag reaches a var and silently does
// nothing to a const, which is a whole release shipped saying 0.0.0-dev.
var version = "0.0.0-dev"

// exitFailure is the code a run that could not do its work ends with.
const exitFailure = 1

const (
	windowTitle = product.Name
	// The manager holds a profile list beside the entries of the selected
	// profile, with the buttons in a rail down the right, so it wants width more
	// than height. It is resizable, unlike the setup window, because how many
	// entries a profile holds is the user's business rather than this program's.
	// The smallest height is the one at which the rail still shows every button
	// without scrolling: a 700 pixel page, measured in a browser on 2026-09-21,
	// plus 40 for the title bar, in case the size given here counts the frame.
	// That has not been measured on a build.
	windowWidth     = 1100
	windowHeight    = 760
	minWindowWidth  = 900
	minWindowHeight = 740
)

// dark and light are the surface colours of the palette, sampled from the
// product's own artwork. One of them paints the window before the page loads,
// so the manager never flashes the wrong ground.
var (
	dark  = options.RGBA{R: 0x0b, G: 0x0e, B: 0x14, A: 1}
	light = options.RGBA{R: 0xf4, G: 0xf6, B: 0xf9, A: 1}
)

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	// hidden is what the sign-in entry passes. A companion that opens a window
	// every time the machine is signed into is not waiting quietly, which is
	// what FR-046 offers; launched by hand it opens the manager instead, which
	// is what a user double-clicking a shortcut means by it.
	hidden := flag.Bool("hidden", false,
		"wait in the notification area without opening the manager")
	// quiet is what the setup program passes. Installing a program must not
	// rearrange the desktop, so a start from setup arranges nothing (FR-076).
	quiet := flag.Bool("quiet", false, "open the manager without arranging anything")
	flag.Parse()
	if *showVersion {
		fmt.Printf("%s %s\n", product.Name, version)
		return
	}
	if err := run(*hidden, *quiet); err != nil {
		// The log already carries this, where there was a log to carry it.
		fmt.Fprintf(os.Stderr, "%s: %v\n", product.Name, err)
		os.Exit(exitFailure)
	}
}

// run holds the log open, takes the single-instance mutex, builds the adapters
// and runs the window until the user quits.
func run(hidden, quiet bool) error {
	started := clock.New().Now()

	directory, err := store.DefaultDirectory()
	if err != nil {
		return err
	}
	log, err := runlog.Open(filepath.Join(filepath.Dir(directory), runlog.FileName), version, started)
	if err != nil {
		return err
	}
	defer func() { _ = log.Close() }()

	steps := runlog.NewSteps(log, clock.New().Now)
	if err := runlog.Keep(log); err != nil {
		// Not a reason to stop. It means a crash would go unrecorded, which is
		// worth saying and is not worth refusing to arrange the desktop over.
		steps.Step(fmt.Sprintf("crash reports will not reach this log: %v", err))
	}

	lock, held, err := instance.Acquire(instance.Name)
	if err != nil {
		return err
	}
	if !held {
		// FR-054: a second launch presents the running instance's manager and
		// ends. Asking the copy that is already there is the whole of what this
		// launch does; it never arranges the desktop a second time.
		if ui.ShowRunningManager() {
			steps.Step("another copy is already running, so its manager was opened")
			return nil
		}
		// Ending here with nothing said is the worst of both: the program does
		// not start and the user is given no reason. Say which copy is in the
		// way and what ends it, where they can read it.
		steps.Step("another copy is already running and would not answer")
		ui.Complain("Another copy of " + product.Name + " is already running and did" +
			" not answer.\n\nEnd it from Task Manager, then start this one again.")
		return nil
	}
	defer func() { _ = lock.Release() }()

	return serve(steps, directory, hidden, quiet)
}

// serve builds the services over the real machine and runs the window.
func serve(steps *runlog.Steps, directory string, hidden, quiet bool) error {
	profiles, err := store.New(directory, steps)
	if err != nil {
		return err
	}
	// DATA-003: say which files are being left alone and why, every run,
	// because a profile that has quietly stopped appearing is the kind of thing
	// a user notices weeks later. The manager says it too (NFR-REL-002), so a
	// store that cannot be read at all is said here and the run goes on to open
	// the window where the user can read it, rather than ending before it.
	excluded, err := profiles.Unreadable(context.Background())
	if err != nil {
		steps.Step(fmt.Sprintf("the profile store could not be checked for unreadable files: %v", err))
	}
	for _, exclusion := range excluded {
		steps.Step(fmt.Sprintf("%s is not being offered: %s", exclusion.File, exclusion.Reason))
	}

	// The settings sit beside the profiles rather than inside them: a profile is
	// the user's work and a setting is a preference.
	preferences := settings.New(filepath.Dir(directory))

	drawn := readLook(steps)
	shown := newSplash(steps, drawn, setup.SystemPrefersDark)
	ticking := clock.New()
	restores := application.NewRestoreService(
		win32.NewDesktop(ticking),
		win32.NewProcesses(),
		win32.NewLauncher(steps),
		profiles,
		ticking,
		steps,
		application.DefaultPolicy(),
		preferences,
		self(),
		preferences,
		shown,
		win32.NewEvents(),
	)
	captures := application.NewCaptureService(
		win32.NewDesktop(ticking), win32.NewProcesses(), profiles, steps, self())
	manager := application.NewManagerService(profiles, startup.New(), steps)
	tray := application.NewTrayService(profiles, restores, captures, steps)

	updates := application.NewUpdateService(
		update.New(), preferences, steps, version,
		application.PlatformKeyFor(runtime.GOOS))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := NewApp(manager, tray, restores, captures, updates, steps, shown, version, hidden)
	switch {
	case quiet:
		// FR-076: setup started this, so the desktop is left exactly as it is.
		steps.Step("setup started this copy, so nothing was arranged")
	case !hidden:
		// FR-038: only a sign-in arranges the desktop by itself. A start by hand
		// is somebody asking for the manager; rearranging the windows they are
		// working in, with a splash over the window they asked for, is not what
		// they asked for. Apply is there when they want it.
		steps.Step("started by hand, so nothing was arranged")
	default:
		go signIn(ctx, manager, restores, steps)
	}
	go runTray(ctx, tray, steps, app, drawn.attention(setup.SystemPrefersDark))

	background := light
	if setup.SystemPrefersDark() {
		background = dark
	}
	return wails.Run(&options.App{
		Title:             windowTitle,
		Width:             windowWidth,
		Height:            windowHeight,
		MinWidth:          minWindowWidth,
		MinHeight:         minWindowHeight,
		StartHidden:       hidden,
		HideWindowOnClose: true,
		BackgroundColour:  &background,
		AssetServer:       &assetserver.Options{Assets: assets},
		OnStartup:         app.startup,
		OnDomReady:        app.domReady,
		Bind:              []interface{}{app},
		Windows: &windowsoptions.Options{
			WebviewUserDataPath: webviewData(),
		},
	})
}

// webviewData pins the webview's own cache beside the profiles rather than
// letting it default into the roaming profile under the executable's name,
// where an uninstall would not know to look for it.
func webviewData() string {
	directory, err := store.DefaultDirectory()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(directory), "webview")
}

// signIn restores the default profile (FR-038) without holding up the window.
//
// It runs alongside rather than before, because a restore waits for windows to
// appear and may take minutes: a manager that could not be opened until it
// finished would be shut for the whole of the time a user most wants to look at
// it. FR-048 asks only that the restore complete without the window being
// opened, which it does.
func signIn(
	ctx context.Context,
	manager *application.ManagerService,
	restores *application.RestoreService,
	steps *runlog.Steps,
) {
	defer func() {
		if recovered := recover(); recovered != nil {
			steps.Step(fmt.Sprintf("the sign-in restore failed unexpectedly: %v", recovered))
		}
	}()
	// The only profile there is, is the one to apply (FR-062). Settled before
	// the restore rather than after it, so a profile stored by an earlier
	// version is arranged on this sign-in rather than the next one.
	if err := manager.SettleDefault(ctx); err != nil {
		steps.Step(fmt.Sprintf("the default marking was left as it was: %v", err))
	}
	report, marked, err := restores.RestoreDefault(ctx)
	if err != nil {
		steps.Step(fmt.Sprintf("the sign-in restore stopped: %v", err))
		return
	}
	if marked {
		write(steps, report)
		// FR-045: the tray says what this restore did, as it does after one
		// started from its own menu.
		ui.RefreshTray()
	}
	// FR-039 needs nothing more where no profile is marked: the restore service
	// has already said so in the log; the manager then offers the capture that
	// gets the user started.
}

// runTray carries the notification area icon for the session. It runs on a
// goroutine because the window owns the main thread; the tray locks a thread of
// its own, since a window belongs to the thread that made it.
func runTray(
	ctx context.Context,
	service *application.TrayService,
	steps *runlog.Steps,
	app *App,
	attention ui.Attention,
) {
	defer func() {
		if recovered := recover(); recovered != nil {
			steps.Step(fmt.Sprintf("the tray failed unexpectedly: %v", recovered))
		}
	}()
	// The loop ends when the user quits from the tray, when the context is
	// cancelled or when the tray could not be shown at all. All three mean the
	// run is over: the window hides rather than closing, so the tray is the only
	// way back to it; a process without one cannot be seen or reached. It is
	// ended here rather than left to Wails, which knows nothing about the tray.
	defer app.endRun()
	if err := ui.NewTray(service, steps, app.ShowManager, attention).Run(ctx); err != nil {
		steps.Step(fmt.Sprintf("the tray could not be shown: %v", err))
	}
}

// self is this product's own identity, which every capture excludes so that a
// profile never tries to arrange the agent that is arranging it (FR-013).
//
// It is the running program's own path. Reading it rather than writing it down
// means it stays right through a rename or a move.
func self() domain.ApplicationIdentity {
	path, err := os.Executable()
	if err != nil {
		// An agent that cannot name itself would capture itself into every
		// profile. An identity that matches nothing is the safer wrong answer:
		// it excludes nothing rather than arranging this program.
		return domain.ApplicationIdentity{}
	}
	identity, err := domain.NewApplicationIdentity(domain.KindPath, path)
	if err != nil {
		return domain.ApplicationIdentity{}
	}
	return identity
}

// write puts the report of a restore into the log, entry by entry, so that a
// user reading it afterwards can see what was satisfied and what was not
// without the manager being open (FR-044, FR-048).
// The summary itself is not repeated here: the restore service writes it as it
// finishes; a log that says the same thing twice teaches a reader to skim.
func write(steps *runlog.Steps, report *application.Report) {
	for _, note := range report.SortedNotes() {
		steps.Step("  " + note)
	}
	for _, entry := range report.Entries() {
		if entry.Satisfied {
			steps.Step(fmt.Sprintf("  satisfied: %s", entry.Application))
			continue
		}
		steps.Step(fmt.Sprintf("  outstanding: %s: %s", entry.Application, entry.Reason))
	}
}
