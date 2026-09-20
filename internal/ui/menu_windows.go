//go:build windows

package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unsafe"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/product"
)

// iconOf answers the icon this program carries, falling back to the shell's
// generic application icon where the binary carries none. A build without an
// icon resource is ordinary before packaging, so this must not fail: a tray
// with a dull icon beats no tray at all. Ported from WhatDay.
func iconOf() uintptr {
	if path, err := os.Executable(); err == nil {
		var large, small uintptr
		count, _, _ := pExtractIconEx.Call(uintptr(unsafe.Pointer(wide(path))),
			0, uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), 1)
		if count > 0 && small != 0 {
			return small
		}
		if count > 0 && large != 0 {
			return large
		}
	}
	generic, _, _ := pLoadIcon.Call(0, idiApplication)
	return generic
}

// iconData describes the tray icon as it should stand now, tooltip included.
func (tray *Tray) iconData() notifyIconData {
	data := notifyIconData{
		hwnd:            tray.hwnd,
		id:              1,
		flags:           nifMessage | nifIcon | nifTip,
		callbackMessage: wmTray,
		icon:            tray.icon,
	}
	data.cbSize = uint32(unsafe.Sizeof(data))
	// The tooltip carries what the last restore did, so "did it work" is
	// answered by resting the pointer rather than by opening anything.
	copy(data.tip[:len(data.tip)-1], windowsString(tray.service.Tooltip()))
	return data
}

// add shows the tray icon, then shows it again after Explorer has restarted.
func (tray *Tray) add() {
	if tray.icon == 0 {
		tray.icon = iconOf()
	}
	data := tray.iconData()
	added, _, _ := pShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&data)))
	tray.added = added != 0
	if !tray.added {
		// Said out loud, because a user who cannot see the icon has no other
		// way to find out that the shell refused it.
		tray.log.Step("the tray refused the icon, so this run has no tray icon")
	}
}

// remove takes the icon away.
func (tray *Tray) remove() {
	if !tray.added {
		return
	}
	data := tray.iconData()
	_, _, _ = pShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&data)))
	tray.added = false
}

// refresh reads the tooltip again, which is how the icon comes to say what the
// restore that just finished did.
func (tray *Tray) refresh() {
	if !tray.added {
		return
	}
	data := tray.iconData()
	_, _, _ = pShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&data)))
}

// showMenu builds the menu from the application layer and opens it where the
// pointer is.
func (tray *Tray) showMenu() {
	items := tray.service.Menu(context.Background())
	tray.mutex.Lock()
	tray.entries = items
	working := tray.working
	tray.mutex.Unlock()

	menu, _, _ := pCreatePopupMenu.Call()
	defer func() { _, _, _ = pDestroyMenu.Call(menu) }()

	for at, item := range items {
		flags, label := flagsFor(item, working)
		var text uintptr
		if label != "" {
			text = uintptr(unsafe.Pointer(wide(label)))
		}
		_, _, _ = pAppendMenu.Call(menu, flags, uintptr(firstCommand+at), text)
	}

	var at point
	_, _, _ = pGetCursorPos.Call(uintptr(unsafe.Pointer(&at)))
	// A popup menu closes on a click elsewhere only while its owner is in
	// front; the owner here is a window nobody can see.
	_, _, _ = pSetForegroundWindow.Call(tray.hwnd)
	command, _, _ := pTrackPopupMenu.Call(menu, tpmReturnCmd|tpmRightButton,
		uintptr(at.x), uintptr(at.y), 0, tray.hwnd, 0)
	_, _, _ = pPostMessage.Call(tray.hwnd, wmNull, 0, 0)

	if command != 0 {
		tray.chose(uint32(command))
	}
}

// flagsFor turns one menu item into the Win32 flags and the words for it.
//
// While a restore is running, the entries that would start another are shown
// greyed rather than removed: an entry that vanishes makes a user wonder
// whether they imagined it; FR-061 means a second choice would replace the
// first rather than be refused, which is not what a greyed entry promises.
func flagsFor(item application.MenuItem, working bool) (uintptr, string) {
	if item.Kind == application.MenuSeparator {
		return mfSeparator, ""
	}
	flags := uintptr(mfString)
	if !item.Enabled || (working && item.Kind == application.MenuCapture) {
		flags |= mfGrayed | mfDisabled
	}
	if item.Checked {
		flags |= mfChecked
	}
	label := item.Label
	if working && item.Kind == application.MenuProfile {
		label = item.Label + "  (a restore is running)"
	}
	return flags, label
}

// chose acts on the entry a command identifies.
func (tray *Tray) chose(command uint32) {
	tray.mutex.Lock()
	items := tray.entries
	tray.mutex.Unlock()

	at := int(command) - firstCommand
	if at < 0 || at >= len(items) {
		return
	}
	item := items[at]
	if !item.Enabled {
		return
	}
	switch item.Kind {
	case application.MenuProfile:
		tray.applyProfile(item.Profile)
	case application.MenuCapture:
		tray.showCapture()
	case application.MenuReport:
		tray.showReport()
	case application.MenuQuit:
		tray.log.Step("quit from the tray")
		_, _, _ = pDestroyWindow.Call(tray.hwnd)
	case application.MenuSeparator, application.MenuMessage:
		// Neither does anything, which is the whole of what they are for.
	}
}

// applyProfile restores a profile on a goroutine of its own (FR-041).
//
// A restore can run for minutes waiting for a window to appear; the window
// procedure must return at once or the whole desktop's menus stop answering.
// The recover sits on the goroutine that can panic, because one on the caller
// would be a guard that looks present and is not.
func (tray *Tray) applyProfile(name string) {
	tray.mutex.Lock()
	tray.working = true
	tray.mutex.Unlock()

	go func() {
		defer func() {
			tray.mutex.Lock()
			tray.working = false
			tray.mutex.Unlock()
			if recovered := recover(); recovered != nil {
				tray.log.Step(fmt.Sprintf("applying %q failed unexpectedly: %v", name, recovered))
			}
			// Posting rather than touching the window from here: the window
			// belongs to the thread that made it.
			_, _, _ = pPostMessage.Call(tray.hwnd, wmRefresh, 0, 0)
		}()
		if _, err := tray.service.Apply(context.Background(), name); err != nil {
			tray.log.Step(fmt.Sprintf("applying %q: %v", name, err))
		}
	}()
}

// showReport puts the report of the last restore on screen (FR-044, FR-045).
func (tray *Tray) showReport() {
	report, held := tray.service.Report()
	if !held {
		return
	}
	icon := uintptr(mbIconInfo)
	if tray.service.NeedsAttention() {
		icon = mbIconWarning
	}
	tray.say(reportText(report), product.Name+" report", icon)
}

// reportText renders a report for a person to read.
func reportText(report *application.Report) string {
	var built strings.Builder
	built.WriteString(report.Summary())
	built.WriteString("\n")
	for _, note := range report.SortedNotes() {
		built.WriteString("\n" + note)
	}
	for _, entry := range report.Entries() {
		if entry.Satisfied {
			built.WriteString(fmt.Sprintf("\n\nSatisfied: %s", entry.Application))
		} else {
			built.WriteString(fmt.Sprintf("\n\nOutstanding: %s\n  %s",
				entry.Application, entry.Reason))
		}
		for _, note := range entry.Notes {
			built.WriteString("\n  " + note)
		}
	}
	return built.String()
}

// showCapture reads the desktop and shows what a capture would record.
//
// It writes nothing. FR-011 says a profile is written from the entries left in
// a review the user confirms; there is nowhere yet to hold that review or
// to name the profile: that is the manager window. Until it exists this says
// plainly what it can and cannot do, rather than writing a profile the user
// never reviewed.
func (tray *Tray) showCapture() {
	review, err := tray.service.Capture(context.Background(), "")
	if err != nil {
		tray.log.Step(fmt.Sprintf("capturing: %v", err))
		tray.say("The desktop could not be read:\n\n"+err.Error(),
			product.Name+" capture", mbIconWarning)
		return
	}
	var built strings.Builder
	built.WriteString(fmt.Sprintf("A capture would record %d application(s):\n",
		len(review.Entries)))
	for _, entry := range review.Entries {
		built.WriteString(fmt.Sprintf("\n%s\n  %d window(s) placed",
			entry.Application, len(entry.Placements)))
	}
	for _, unreadable := range review.Unreadable {
		built.WriteString("\n\nCould not be read: " + unreadable)
	}
	built.WriteString("\n\nNothing has been saved. Naming a profile and choosing" +
		" what goes into it needs the manager window, which is not built yet.")
	tray.say(built.String(), product.Name+" capture", mbIconInfo)
}

// say puts a message on screen. It is shown from the loop's own thread, so the
// menu is already gone by the time it appears.
func (tray *Tray) say(text string, title string, icon uintptr) {
	_, _, _ = pMessageBox.Call(tray.hwnd,
		uintptr(unsafe.Pointer(wide(text))),
		uintptr(unsafe.Pointer(wide(title))), icon|mbOK)
}
