//go:build !windows

package win32

import (
	"context"
	"time"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
)

// This file is what lets the whole product be built, vetted and tested on a
// machine that is not Windows. The rules above this package are portable by
// design and their tests need no desktop; without these stubs the package would
// not compile there and none of those tests could run.
//
// Every call answers ErrNotWindows. None answers nil: a caller holding one of
// these gets a working object that refuses, never a nil pointer to trip over.

// Desktop refuses everything on a machine that is not Windows.
type Desktop struct{}

// NewDesktop returns a desktop that refuses every call.
func NewDesktop(application.Clock) *Desktop { return &Desktop{} }

// Windows refuses: there are no windows of this kind here.
func (desktop *Desktop) Windows(context.Context) ([]application.Window, error) {
	return nil, ErrNotWindows
}

// Window refuses.
func (desktop *Desktop) Window(context.Context, application.WindowID) (application.Window, error) {
	return application.Window{}, ErrNotWindows
}

// Displays refuses.
func (desktop *Desktop) Displays(context.Context) ([]application.Display, error) {
	return nil, ErrNotWindows
}

// Place refuses.
func (desktop *Desktop) Place(
	context.Context,
	application.WindowID,
	domain.Rect,
	domain.ShowState,
) error {
	return ErrNotWindows
}

// NudgeTaskbars refuses: the taskbars it would click belong to Windows.
func (desktop *Desktop) NudgeTaskbars(context.Context) (int, error) {
	return 0, ErrNotWindows
}

// RebuildTaskbarButton refuses: the taskbar belongs to Windows.
func (desktop *Desktop) RebuildTaskbarButton(context.Context, application.WindowID) error {
	return ErrNotWindows
}

// Close refuses.
func (desktop *Desktop) Close(context.Context, application.WindowID) error {
	return ErrNotWindows
}

// Processes refuses everything on a machine that is not Windows.
type Processes struct{}

// NewProcesses returns a process reader that refuses every call.
func NewProcesses() *Processes { return &Processes{} }

// Running refuses.
func (processes *Processes) Running(context.Context, domain.ApplicationIdentity) (bool, error) {
	return false, ErrNotWindows
}

// Launcher refuses everything on a machine that is not Windows.
type Launcher struct{}

// NewLauncher returns a launcher that refuses every call.
func NewLauncher(application.Log) *Launcher { return &Launcher{} }

// Launch refuses.
func (launcher *Launcher) Launch(context.Context, domain.ApplicationIdentity) error {
	return ErrNotWindows
}

// Events refuses to watch on a machine that is not Windows.
type Events struct{}

// NewEvents returns events that refuse to watch.
func NewEvents() *Events { return &Events{} }

// Watch refuses.
func (*Events) Watch(context.Context) (application.DesktopWatch, error) {
	return nil, ErrNotWindows
}

// FlashSeries answers zero: there is no taskbar to flash.
func (*Events) FlashSeries() time.Duration { return 0 }
