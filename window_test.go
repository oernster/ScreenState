package main

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// recordingLog is the step log as a test can read it back.
type recordingLog struct {
	mutex sync.Mutex
	steps []string
}

func (log *recordingLog) Step(message string) {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	log.steps = append(log.steps, message)
}

func (log *recordingLog) said(fragment string) bool {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	for _, step := range log.steps {
		if strings.Contains(step, fragment) {
			return true
		}
	}
	return false
}

// appStartedHidden builds the facade as a sign-in start builds it, with no
// Wails context: the guard under test runs before anything touches one.
func appStartedHidden(hidden bool) (*App, *recordingLog) {
	log := &recordingLog{}
	return NewApp(nil, nil, nil, nil, nil, log, silentSplash{}, "0.0.0-test", hidden), log
}

// TestASignInStartLeavesTheWindowOffScreen is FR-048 as a test.
//
// The manager appeared over the desktop after a sign-in on 2026-09-20, from a
// Run entry that carried the hidden flag and a window Wails was told to start
// hidden. The page is what put it up: it loads either way, finds it has no
// keyboard, which is true of a window nobody can see, then asks for it. The
// repair for that ask ends in WindowShow.
func TestASignInStartLeavesTheWindowOffScreen(t *testing.T) {
	t.Parallel()
	app, log := appStartedHidden(true)
	app.TakeKeyboard()
	if !log.said("not on screen") {
		t.Fatal("a hidden start answered the page's ask for the keyboard rather than leaving the window alone")
	}
}

// TestAStartByHandTakesTheKeyboard holds the other half: the guard must not
// leave an ordinary launch without a keyboard, which is the fault it repairs.
func TestAStartByHandTakesTheKeyboard(t *testing.T) {
	t.Parallel()
	app, log := appStartedHidden(false)
	app.TakeKeyboard()
	if log.said("not on screen") {
		t.Fatal("a start by hand was treated as a window nobody can see")
	}
}

// TestTheWindowStaysOpenUnderADialog holds the modal rule: while a dialog is up
// in the page, a close from the window's frame (the cross, Alt+F4, the
// taskbar's Close) is refused and said in the log; once it has gone, closing
// hides the window as it always did.
func TestTheWindowStaysOpenUnderADialog(t *testing.T) {
	t.Parallel()
	app, log := appStartedHidden(false)
	app.SetDialogOpen(true)
	if !app.beforeClose(context.Background()) || !app.onScreen.Load() {
		t.Fatal("the window was closed with a dialog open in it")
	}
	if !log.said("while a dialog was open") {
		t.Error("the refused close was not logged")
	}
	app.SetDialogOpen(false)
	if !app.beforeClose(context.Background()) || app.onScreen.Load() {
		t.Fatal("closing with no dialog open did not hide the window")
	}
}

// TestAQuitIsNeverRefused holds the other half: a dialog keeps the window open,
// never the run alive when the user has asked it to end.
func TestAQuitIsNeverRefused(t *testing.T) {
	t.Parallel()
	app, _ := appStartedHidden(false)
	app.SetDialogOpen(true)
	app.endRun()
	if app.beforeClose(context.Background()) {
		t.Fatal("a quit was refused because a dialog was open")
	}
}

// TestClosingTheManagerPutsTheWindowBackOffScreen holds what happens after the
// window has been up: closing it is not quitting, so a later ask from the page
// must not bring it back on its own.
func TestClosingTheManagerPutsTheWindowBackOffScreen(t *testing.T) {
	t.Parallel()
	app, log := appStartedHidden(false)
	app.Hide()
	app.TakeKeyboard()
	if !log.said("not on screen") {
		t.Fatal("the window was closed and the page's ask still reached the repair")
	}
}
