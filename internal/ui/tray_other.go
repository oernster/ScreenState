//go:build !windows

// Package ui is this product's Windows surface. Off Windows there is no tray to
// show, so it refuses rather than doing nothing: the rest of the product is
// built and tested on other platforms; a tray that silently succeeded there
// would be a test passing over a window nobody could see.
package ui

import (
	"context"
	"errors"

	"github.com/oernster/ScreenState/internal/application"
)

// ErrNotWindows is every call into this package on a machine that is not
// Windows.
var ErrNotWindows = errors.New("the tray runs on Windows only")

// Tray refuses everything on a machine that is not Windows.
type Tray struct{}

// NewTray returns a tray that refuses to run.
func NewTray(
	*application.TrayService,
	application.Log,
	func(application.ManagerRequest),
) *Tray {
	return &Tray{}
}

// Run refuses.
func (tray *Tray) Run(context.Context) error { return ErrNotWindows }

// ShowRunningManager has no running copy to ask off Windows.
func ShowRunningManager() bool { return false }

// Complain has nowhere to say it off Windows, where the log is the answer.
func Complain(string) {}

// TakeWindowFocus has no window to focus off Windows.
func TakeWindowFocus() bool { return false }

// RefreshTray does nothing: there is no tray to refresh.
func RefreshTray() {}
