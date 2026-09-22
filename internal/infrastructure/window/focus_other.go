//go:build !windows

// The Windows keyboard handover has no meaning on other platforms, where the
// webview takes focus with its window. See focus_windows.go for what this
// replaces.

package window

// TakeFocus reports that there was nothing to do.
func TakeFocus() bool { return false }

// AllowClose reports that there was no window menu to change.
func AllowClose(bool) bool { return false }
