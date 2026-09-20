//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// windowRecord holds every identifier the spike can read for one top-level
// window. OQ-2 asks which of these identifies the application across its own
// updates.
type windowRecord struct {
	Handle      uintptr
	Title       string
	Class       string
	ProcessID   uint32
	ImagePath   string
	ModelID     string
	LaunchHint  string
	Visible     bool
	Cloaked     bool
	Owned       bool
	ToolWindow  bool
	Minimised   bool
	Maximised   bool
	Rect        RECT
	NormalRect  RECT
	MonitorName string
}

func enumerateWindows() []windowRecord {
	displays := enumerateDisplays()
	monitorNames := make(map[uintptr]string, len(displays))
	for _, display := range displays {
		monitorNames[display.Handle] = display.Position
	}

	var records []windowRecord
	callback := syscall.NewCallback(func(handle uintptr, _ uintptr) uintptr {
		exStyle := windowExStyle(handle)
		placement := windowPlacementOf(handle)
		processID := windowProcessID(handle)
		imagePath := processImagePath(processID)
		record := windowRecord{
			Handle:      handle,
			Title:       windowText(handle),
			Class:       windowClass(handle),
			ProcessID:   processID,
			ImagePath:   imagePath,
			ModelID:     processApplicationUserModelID(processID),
			LaunchHint:  launchHint(imagePath),
			Visible:     windowIsVisible(handle),
			Cloaked:     windowIsCloaked(handle),
			Owned:       windowOwner(handle) != 0,
			ToolWindow:  exStyle&wsExToolWindow != 0 && exStyle&wsExAppWindow == 0,
			Minimised:   windowIsMinimised(handle),
			Maximised:   windowIsMaximised(handle),
			Rect:        windowRect(handle),
			NormalRect:  placement.RcNormalPosition,
			MonitorName: monitorNames[monitorOfWindow(handle)],
		}
		records = append(records, record)
		return 1
	})
	procEnumWindows.Call(callback, 0)
	return records
}

// isCandidate applies the rule FR-012 proposes: a window a user would call a
// window of an application.
func (record windowRecord) isCandidate() bool {
	if !record.Visible || record.Cloaked || record.ToolWindow || record.Owned {
		return false
	}
	return strings.TrimSpace(record.Title) != ""
}

func (record windowRecord) state() string {
	switch {
	case record.Minimised:
		return "minimised"
	case record.Maximised:
		return "maximised"
	default:
		return "normal"
	}
}

// launchHint reports how an application would be started without naming a path
// that carries a version number. Applications packaged by Squirrel, such as
// Discord, install into app-<version> beside an Update.exe that does not move.
func launchHint(imagePath string) string {
	if imagePath == "" {
		return ""
	}
	if strings.Contains(strings.ToLower(imagePath), `\windowsapps\`) {
		return "packaged: launch by application user model id"
	}
	directory := filepath.Dir(imagePath)
	parent := filepath.Dir(directory)
	if !strings.HasPrefix(strings.ToLower(filepath.Base(directory)), "app-") {
		return "fixed path"
	}
	updater := filepath.Join(parent, "Update.exe")
	if _, err := os.Stat(updater); err == nil {
		return fmt.Sprintf("versioned path, updater present: %s --processStart %s",
			updater, filepath.Base(imagePath))
	}
	return "versioned path, no updater found"
}

func reportWindows(out io.Writer, records []windowRecord, all bool) {
	shown := 0
	for _, record := range records {
		if !all && !record.isCandidate() {
			continue
		}
		shown++
		fmt.Fprintf(out, "hwnd 0x%X  pid %d  %s\n", record.Handle, record.ProcessID, record.state())
		fmt.Fprintf(out, "    title:            %s\n", record.Title)
		fmt.Fprintf(out, "    class:            %s\n", record.Class)
		fmt.Fprintf(out, "    image:            %s\n", record.ImagePath)
		fmt.Fprintf(out, "    model id:         %s\n", record.ModelID)
		fmt.Fprintf(out, "    launch hint:      %s\n", record.LaunchHint)
		fmt.Fprintf(out, "    display:          %s\n", record.MonitorName)
		fmt.Fprintf(out, "    rect:             x=%d y=%d w=%d h=%d\n",
			record.Rect.Left, record.Rect.Top, record.Rect.Width(), record.Rect.Height())
		fmt.Fprintf(out, "    normal rect:      x=%d y=%d w=%d h=%d\n",
			record.NormalRect.Left, record.NormalRect.Top,
			record.NormalRect.Width(), record.NormalRect.Height())
		fmt.Fprintf(out, "    visible=%t cloaked=%t owned=%t tool=%t candidate=%t\n\n",
			record.Visible, record.Cloaked, record.Owned, record.ToolWindow, record.isCandidate())
	}
	fmt.Fprintf(out, "Windows listed: %d of %d enumerated.\n", shown, len(records))
	fmt.Fprintf(out, "OQ-2: compare image path, model id and launch hint for Claude and\n")
	fmt.Fprintf(out, "Discord. A path carrying a version number cannot identify an\n")
	fmt.Fprintf(out, "application across its updates.\n")
}

func findWindow(handle uintptr) (windowRecord, bool) {
	for _, record := range enumerateWindows() {
		if record.Handle == handle {
			return record, true
		}
	}
	return windowRecord{}, false
}
