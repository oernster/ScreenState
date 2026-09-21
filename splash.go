package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/ui"
)

// theme is the product's one palette, which the splash is drawn in as the
// manager and the setup program are.
//
//go:embed assets/theme.css
var theme string

// artwork is the master of the application's artwork (EIR-003), from which
// the splash reduces its logo to each display's size.
//
//go:embed assets/application-icon.png
var artwork []byte

// newSplash builds the splash that says the desktop is being prepared
// (FR-078).
//
// Both inputs are built into the binary, so neither failing is something a
// user did; it is still no reason to stop the agent, which would take the
// restore down with it. A palette that cannot be read shows no splash at all,
// since it cannot be drawn in the product's colours; artwork that cannot be
// read shows the words alone. Either is said in the log.
func newSplash(log application.Log, dark func() bool) application.Splash {
	themes, err := ui.ThemesFrom(theme)
	if err != nil {
		log.Step(fmt.Sprintf("the splash cannot be drawn, so none will be shown: %v", err))
		return silentSplash{}
	}
	var logo image.Image
	decoded, err := png.Decode(bytes.NewReader(artwork))
	if err != nil {
		log.Step(fmt.Sprintf("the splash artwork could not be read, so the words are shown alone: %v", err))
	} else {
		logo = decoded
	}
	return ui.NewSplash(logo, themes, dark, log)
}

// silentSplash shows nothing. It is the splash for a run that cannot draw one,
// so the restore service always has a splash to tell and never a nil to check.
type silentSplash struct{}

func (silentSplash) Preparing(application.SplashMessage) {}

func (silentSplash) Ready(application.SplashMessage) {}
