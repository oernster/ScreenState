//go:build !windows

package setup

import "errors"

// ErrUnsupported says this platform has no bespoke setup program. The product
// arranges Windows desktops and runs nowhere else; these stubs exist to keep
// the package building and vetting everywhere, not to offer a second install
// route.
var ErrUnsupported = errors.New("the setup program runs on Windows only")

// ErrAppRunning matches the Windows sentinel so the setup facade compiles
// anywhere.
var ErrAppRunning = ErrUnsupported

// ErrAppStillRunning matches the Windows sentinel so the setup facade compiles
// anywhere.
var ErrAppStillRunning = ErrUnsupported

// UninstallInfo mirrors the Windows record so the facade's signatures hold.
type UninstallInfo struct {
	Version      string
	InstallDir   string
	UninstallExe string
	IconPath     string
	EstimatedKB  uint32
}

// Shortcuts mirrors the Windows record so the facade's signatures hold.
type Shortcuts struct {
	StartMenu bool
	Desktop   bool
}

// WriteUninstallEntry has nothing to register off Windows.
func WriteUninstallEntry(UninstallInfo) error { return ErrUnsupported }

// RemoveUninstallEntry has nothing to remove off Windows.
func RemoveUninstallEntry() error { return ErrUnsupported }

// InstalledVersion reports nothing installed off Windows.
func InstalledVersion() (string, bool) { return "", false }

// SetLaunchOnBoot has no login entry to write off Windows.
func SetLaunchOnBoot(string, bool) error { return ErrUnsupported }

// IsLaunchOnBoot reports no login entry off Windows.
func IsLaunchOnBoot() bool { return false }

// SystemPrefersDark reports light off Windows, so the setup window has a
// definite starting theme everywhere.
func SystemPrefersDark() bool { return false }

// ApplyShortcuts places nothing off Windows.
func ApplyShortcuts(string, string, Shortcuts) {}

// CurrentShortcuts reports no shortcuts off Windows.
func CurrentShortcuts() Shortcuts { return Shortcuts{} }

// RemoveShortcuts removes nothing off Windows.
func RemoveShortcuts() {}

// DesktopDir is unavailable off Windows.
func DesktopDir() (string, error) { return "", ErrUnsupported }

// StartMenuProgramsDir is unavailable off Windows.
func StartMenuProgramsDir() (string, error) { return "", ErrUnsupported }

// IsAppRunning reports nothing running off Windows.
func IsAppRunning() bool { return false }

// CloseRunningApp has nothing to close off Windows.
func CloseRunningApp() error { return ErrUnsupported }

// LaunchApp has nothing to launch off Windows.
func LaunchApp() error { return ErrUnsupported }

// ScheduleDirDeletion schedules nothing off Windows.
func ScheduleDirDeletion(string) {}
