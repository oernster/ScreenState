//go:build !windows

package ui

import (
	"image"

	"github.com/oernster/ScreenState/internal/application"
)

// Splash shows nothing off Windows, where there is no desktop to prepare.
type Splash struct{}

// NewSplash returns a splash that shows nothing.
func NewSplash(image.Image, Themes, func() bool, application.Log) *Splash {
	return &Splash{}
}

// Preparing shows nothing.
func (*Splash) Preparing(application.SplashMessage) {}

// Ready shows nothing.
func (*Splash) Ready(application.SplashMessage) {}
