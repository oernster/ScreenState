//go:build windows

package main

import (
	"fmt"
	"io"
	"sort"
	"time"
)

const watchPollInterval = 500 * time.Millisecond

type watchedWindow struct {
	Title       string
	ImagePath   string
	FirstSeen   time.Duration
	LastChange  time.Duration
	Moves       int
	StateSwaps  int
	Rect        RECT
	State       string
	MonitorName string
}

// watch recorded window appearances and movements at sign-in for OQ-5 and
// OQ-7. Both are closed: settling is defined by the profile's own entries
// rather than by any timing, so nothing here blocks a requirement. It is kept
// for seeing what a real sign-in does, never as evidence about who moved a
// window.
func watch(out io.Writer, span time.Duration) {
	start := time.Now()
	sinceBoot := time.Duration(tickCount()) * time.Millisecond
	fmt.Fprintf(out, "Watching for %s. Started %s, which is %s after this machine booted.\n\n",
		span, start.Format("2006-01-02 15:04:05"), sinceBoot.Round(time.Second))

	seen := make(map[uintptr]*watchedWindow)
	lastAppearance := time.Duration(0)
	longestQuiet := time.Duration(0)
	deadline := start.Add(span)

	for time.Now().Before(deadline) {
		elapsed := time.Since(start)
		present := make(map[uintptr]struct{})
		for _, record := range enumerateWindows() {
			if !record.isCandidate() {
				continue
			}
			present[record.Handle] = struct{}{}
			existing, known := seen[record.Handle]
			if !known {
				seen[record.Handle] = &watchedWindow{
					Title:       record.Title,
					ImagePath:   record.ImagePath,
					FirstSeen:   elapsed,
					LastChange:  elapsed,
					Rect:        record.Rect,
					State:       record.state(),
					MonitorName: record.MonitorName,
				}
				quiet := elapsed - lastAppearance
				if quiet > longestQuiet {
					longestQuiet = quiet
				}
				lastAppearance = elapsed
				fmt.Fprintf(out, "%8s  appeared  %-34s  %s  %s (x=%d y=%d w=%d h=%d)\n",
					elapsed.Round(time.Millisecond), trim(record.Title, 34),
					record.MonitorName, record.state(),
					record.Rect.Left, record.Rect.Top, record.Rect.Width(), record.Rect.Height())
				continue
			}
			noteChanges(out, elapsed, existing, record)
		}
		for handle, window := range seen {
			if _, still := present[handle]; still {
				continue
			}
			delete(seen, handle)
			fmt.Fprintf(out, "%8s  vanished  %-34s\n",
				elapsed.Round(time.Millisecond), trim(window.Title, 34))
		}
		time.Sleep(watchPollInterval)
	}

	summarise(out, seen, lastAppearance, longestQuiet, span)
}

func noteChanges(out io.Writer, elapsed time.Duration, window *watchedWindow, record windowRecord) {
	if record.Rect != window.Rect {
		window.Moves++
		window.LastChange = elapsed
		fmt.Fprintf(out, "%8s  moved     %-34s  %s (x=%d y=%d w=%d h=%d)\n",
			elapsed.Round(time.Millisecond), trim(record.Title, 34), record.MonitorName,
			record.Rect.Left, record.Rect.Top, record.Rect.Width(), record.Rect.Height())
		window.Rect = record.Rect
		window.MonitorName = record.MonitorName
	}
	if record.state() != window.State {
		window.StateSwaps++
		window.LastChange = elapsed
		fmt.Fprintf(out, "%8s  state     %-34s  %s to %s\n",
			elapsed.Round(time.Millisecond), trim(record.Title, 34), window.State, record.state())
		window.State = record.state()
	}
	if record.Title != window.Title {
		window.Title = record.Title
	}
}

func summarise(out io.Writer, seen map[uintptr]*watchedWindow, lastAppearance, longestQuiet, span time.Duration) {
	type row struct {
		handle uintptr
		window *watchedWindow
	}
	rows := make([]row, 0, len(seen))
	for handle, window := range seen {
		rows = append(rows, row{handle: handle, window: window})
	}
	sort.Slice(rows, func(first, second int) bool {
		return rows[first].window.FirstSeen < rows[second].window.FirstSeen
	})

	fmt.Fprintf(out, "\nSummary\n\n")
	fmt.Fprintf(out, "%-10s %-8s %-6s %-6s %-34s %s\n",
		"first seen", "changed", "moves", "states", "title", "image")
	for _, entry := range rows {
		fmt.Fprintf(out, "%-10s %-8s %-6d %-6d %-34s %s\n",
			entry.window.FirstSeen.Round(time.Second),
			entry.window.LastChange.Round(time.Second),
			entry.window.Moves,
			entry.window.StateSwaps,
			trim(entry.window.Title, 34),
			entry.window.ImagePath)
	}
	fmt.Fprintf(out, "\nLast window appeared %s into the watch.\n", lastAppearance.Round(time.Second))
	fmt.Fprintf(out, "Longest gap between appearances: %s.\n", longestQuiet.Round(time.Second))
	fmt.Fprintf(out, "Watch span: %s.\n\n", span)
	fmt.Fprintf(out, "Moves recorded here say nothing about who made them. A drag by\n")
	fmt.Fprintf(out, "hand and a window repositioning itself look identical in this log,\n")
	fmt.Fprintf(out, "which is how the run on 2026-09-20 recorded the owner moving three\n")
	fmt.Fprintf(out, "windows and was nearly read as applications moving their own. Treat\n")
	fmt.Fprintf(out, "a move as evidence only from a run nobody touched.\n")
}

func trim(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "~"
}
