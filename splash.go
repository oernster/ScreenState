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

// look is the palette and the artwork, read out of the binary once for the two
// things the agent draws itself: the splash and the tray icon's badge.
type look struct {
	themes   ui.Themes
	drawable bool
	logo     image.Image
}

// readLook reads the palette and the artwork.
//
// Both are built into the binary, so neither failing is something a user did;
// it is still no reason to stop the agent, which would take the restore down
// with it. A palette that cannot be read shows no splash at all and never marks
// the tray icon, since neither can be drawn in the product's colours; artwork
// that cannot be read shows the splash's words alone and never marks the icon.
// Either is said in the log.
func readLook(log application.Log) look {
	themes, err := ui.ThemesFrom(theme)
	if err != nil {
		log.Step(fmt.Sprintf("the palette cannot be read, so no splash will be shown and the tray icon is never marked: %v", err))
		return look{}
	}
	drawn := look{themes: themes, drawable: true}
	decoded, err := png.Decode(bytes.NewReader(artwork))
	if err != nil {
		log.Step(fmt.Sprintf("the artwork could not be read, so the splash shows its words alone and the tray icon is never marked: %v", err))
	} else {
		drawn.logo = decoded
	}
	return drawn
}

// newSplash builds the splash that says the desktop is being prepared
// (FR-078).
func newSplash(log application.Log, drawn look, dark func() bool) application.Splash {
	if !drawn.drawable {
		return silentSplash{}
	}
	return ui.NewSplash(drawn.logo, drawn.themes, dark, log)
}

// attention is what the tray marks its icon from when a restore leaves
// something outstanding (FR-045). Its zero value leaves the icon plain.
func (drawn look) attention(dark func() bool) ui.Attention {
	if !drawn.drawable || drawn.logo == nil {
		return ui.Attention{}
	}
	return ui.Attention{Logo: drawn.logo, Themes: drawn.themes, Dark: dark}
}

// silentSplash shows nothing. It is the splash for a run that cannot draw one,
// so the restore service always has a splash to tell and never a nil to check.
type silentSplash struct{}

func (silentSplash) Preparing(application.SplashMessage) {}

func (silentSplash) Ready(application.SplashMessage) {}
