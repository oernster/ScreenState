//go:build windows

package main

import (
	"fmt"
	"io"
	"os/exec"
	"time"
)

const (
	actionSettleDelay = 500 * time.Millisecond
	closeWaitLimit    = 10 * time.Second
	closePollInterval = 250 * time.Millisecond
)

// moveWindow settles OQ-3: can a process without administrator rights place a
// window on a chosen display and maximise it there, across displays of
// different scaling?
func moveWindow(out io.Writer, handle uintptr, displayIndex int) {
	displays := enumerateDisplays()
	if displayIndex < 0 || displayIndex >= len(displays) {
		fmt.Fprintf(out, "No display with index %d. There are %d.\n", displayIndex, len(displays))
		return
	}
	target := displays[displayIndex]
	before, found := findWindow(handle)
	if !found {
		fmt.Fprintf(out, "No window with handle 0x%X.\n", handle)
		return
	}
	fmt.Fprintf(out, "Before: display=%s state=%s rect=(x=%d y=%d w=%d h=%d)\n",
		before.MonitorName, before.state(),
		before.Rect.Left, before.Rect.Top, before.Rect.Width(), before.Rect.Height())

	procShowWindow.Call(handle, uintptr(swRestore))
	time.Sleep(actionSettleDelay)
	area := target.WorkArea
	moved, _, callErr := procSetWindowPos.Call(
		handle,
		0,
		uintptr(int32(area.Left)),
		uintptr(int32(area.Top)),
		uintptr(area.Width()),
		uintptr(area.Height()),
		uintptr(swpNoZOrder|swpNoActivate),
	)
	if moved == 0 {
		fmt.Fprintf(out, "SetWindowPos refused the move: %v\n", callErr)
	}
	time.Sleep(actionSettleDelay)
	procShowWindow.Call(handle, uintptr(swMaximize))
	time.Sleep(actionSettleDelay)

	after, found := findWindow(handle)
	if !found {
		fmt.Fprintf(out, "The window no longer exists.\n")
		return
	}
	fmt.Fprintf(out, "After:  display=%s state=%s rect=(x=%d y=%d w=%d h=%d)\n",
		after.MonitorName, after.state(),
		after.Rect.Left, after.Rect.Top, after.Rect.Width(), after.Rect.Height())
	fmt.Fprintf(out, "Wanted: display=%s state=maximised\n", target.GDIName)
	if after.MonitorName == target.GDIName && after.Maximised {
		fmt.Fprintf(out, "RESULT: the window went where it was told.\n")
		return
	}
	fmt.Fprintf(out, "RESULT: the window did NOT go where it was told.\n")
}

// closeWindow settles OQ-4: does closing a window leave the application
// running? It asks the window to close. It never terminates a process.
func closeWindow(out io.Writer, handle uintptr) {
	record, found := findWindow(handle)
	if !found {
		fmt.Fprintf(out, "No window with handle 0x%X.\n", handle)
		return
	}
	fmt.Fprintf(out, "Closing \"%s\" (pid %d, %s)\n", record.Title, record.ProcessID, record.ImagePath)
	procPostMessageW.Call(handle, uintptr(wmClose), 0, 0)

	deadline := time.Now().Add(closeWaitLimit)
	for time.Now().Before(deadline) {
		if !windowExists(handle) || !windowIsVisible(handle) {
			break
		}
		time.Sleep(closePollInterval)
	}
	gone := !windowExists(handle)
	hidden := !gone && !windowIsVisible(handle)
	alive := processIsRunning(record.ProcessID)
	fmt.Fprintf(out, "Window destroyed: %t\n", gone)
	fmt.Fprintf(out, "Window hidden but alive: %t\n", hidden)
	fmt.Fprintf(out, "Process still running: %t\n", alive)
	switch {
	case (gone || hidden) && alive:
		fmt.Fprintf(out, "RESULT: closing the window left the application running.\n")
	case !alive:
		fmt.Fprintf(out, "RESULT: closing the window ended the application. It cannot be\n")
		fmt.Fprintf(out, "treated as an entry recorded with no placement.\n")
	default:
		fmt.Fprintf(out, "RESULT: the window refused to close.\n")
	}
}

// showWindow settles the first half of OQ-6: can a window that is hidden be
// made visible from outside its application?
func showWindow(out io.Writer, handle uintptr) {
	if !windowExists(handle) {
		fmt.Fprintf(out, "No window with handle 0x%X.\n", handle)
		return
	}
	fmt.Fprintf(out, "Before: visible=%t cloaked=%t minimised=%t\n",
		windowIsVisible(handle), windowIsCloaked(handle), windowIsMinimised(handle))
	procShowWindow.Call(handle, uintptr(swShow))
	time.Sleep(actionSettleDelay)
	procShowWindow.Call(handle, uintptr(swRestore))
	time.Sleep(actionSettleDelay)
	procSetForegroundWindow.Call(handle)
	time.Sleep(actionSettleDelay)
	visible := windowIsVisible(handle) && !windowIsCloaked(handle)
	fmt.Fprintf(out, "After:  visible=%t cloaked=%t minimised=%t\n",
		windowIsVisible(handle), windowIsCloaked(handle), windowIsMinimised(handle))
	if visible {
		fmt.Fprintf(out, "RESULT: the window can be shown from outside the application.\n")
		return
	}
	fmt.Fprintf(out, "RESULT: the window could NOT be shown from outside the application.\n")
}

// launchAndWatch settles the second half of OQ-6 and part of OQ-2: start a
// command, then report every window that appears over the watch span. Running
// an application that is already running is how a tray-only application is
// usually asked to show itself.
func launchAndWatch(out io.Writer, command string, arguments []string, span time.Duration) {
	before := candidateHandles()
	fmt.Fprintf(out, "Launching: %s %v\n", command, arguments)
	process := exec.Command(command, arguments...)
	if err := process.Start(); err != nil {
		fmt.Fprintf(out, "RESULT: the launch failed: %v\n", err)
		return
	}
	fmt.Fprintf(out, "Started as pid %d. Watching for %s.\n", process.Process.Pid, span)
	deadline := time.Now().Add(span)
	appeared := 0
	for time.Now().Before(deadline) {
		time.Sleep(closePollInterval)
		for _, record := range enumerateWindows() {
			if !record.isCandidate() {
				continue
			}
			if _, seen := before[record.Handle]; seen {
				continue
			}
			before[record.Handle] = struct{}{}
			appeared++
			fmt.Fprintf(out, "  + %s  hwnd 0x%X  pid %d  %s  display %s\n",
				time.Now().Format("15:04:05"), record.Handle, record.ProcessID,
				record.Title, record.MonitorName)
		}
	}
	fmt.Fprintf(out, "RESULT: %d window(s) appeared during the watch.\n", appeared)
}

func candidateHandles() map[uintptr]struct{} {
	handles := make(map[uintptr]struct{})
	for _, record := range enumerateWindows() {
		if record.isCandidate() {
			handles[record.Handle] = struct{}{}
		}
	}
	return handles
}
