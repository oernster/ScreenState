// Command ScreenState is the agent: it restores the default profile after
// sign-in and writes down what it did.
//
// This file is the composition root. It is the only place that knows both the
// application layer and the Windows layer, which is why the structural suite
// names it and fails any other file that learns both. Everything here is
// wiring: no rule about what a profile means lives in this file.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
	"github.com/oernster/ScreenState/internal/infrastructure/clock"
	"github.com/oernster/ScreenState/internal/infrastructure/instance"
	"github.com/oernster/ScreenState/internal/infrastructure/runlog"
	"github.com/oernster/ScreenState/internal/infrastructure/store"
	"github.com/oernster/ScreenState/internal/infrastructure/win32"
	"github.com/oernster/ScreenState/internal/product"
	"github.com/oernster/ScreenState/internal/ui"
)

// version is stamped in at build time from the VERSION file. It is a var and
// not a const on purpose: the linker's -X flag reaches a var and silently does
// nothing to a const, which is a whole release shipped saying 0.0.0-dev.
var version = "0.0.0-dev"

// exitFailure is the code a run that could not do its work ends with.
const exitFailure = 1

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("%s %s\n", product.Name, version)
		return
	}
	if err := run(); err != nil {
		// The log already carries this, where there was a log to carry it.
		fmt.Fprintf(os.Stderr, "%s: %v\n", product.Name, err)
		os.Exit(exitFailure)
	}
}

// run does the whole of a sign-in: hold the log open, take the single-instance
// mutex, build the adapters and restore the default profile.
func run() error {
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
		// FR-054: a second launch stands down. It will present the running
		// copy's manager once there is one; for now it says so and stops,
		// rather than arranging the desktop twice at once.
		steps.Step("another copy is already running, so this one stopped")
		return nil
	}
	defer func() { _ = lock.Release() }()

	return signIn(steps, directory)
}

// signIn builds the services over the real machine and restores the default
// profile (FR-038).
func signIn(steps *runlog.Steps, directory string) error {
	profiles, err := store.New(directory, steps)
	if err != nil {
		return err
	}
	excluded, err := profiles.Excluded(context.Background())
	if err != nil {
		return err
	}
	for _, exclusion := range excluded {
		// DATA-003: say which files are being left alone and why, every run,
		// because a profile that has quietly stopped appearing is the kind of
		// thing a user notices weeks later.
		steps.Step(fmt.Sprintf("%s is not being offered: %s", exclusion.File, exclusion.Reason))
	}

	ticking := clock.New()
	restores := application.NewRestoreService(
		win32.NewDesktop(ticking),
		win32.NewProcesses(),
		win32.NewLauncher(),
		profiles,
		ticking,
		steps,
		application.DefaultPolicy(),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	report, marked, err := restores.RestoreDefault(ctx)
	if err != nil {
		return err
	}
	if marked {
		write(steps, report)
	}
	// FR-039 needs nothing more where no profile is marked: the restore service
	// has already said so in the log; the tray then offers the capture that
	// gets the user started.

	captures := application.NewCaptureService(
		win32.NewDesktop(ticking), win32.NewProcesses(), profiles, steps, self())
	tray := application.NewTrayService(profiles, restores, captures, steps)

	// The agent stays for the session from here. A restore at sign-in is only
	// half of what the product does; FR-041 lets the user apply a profile
	// whenever they like, which needs something on screen to ask.
	return ui.NewTray(tray, steps).Run(ctx)
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
