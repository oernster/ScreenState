package main

import (
	"context"
	"fmt"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
)

// Profiles lists the stored profiles for the manager's list.
func (a *App) Profiles() ([]ProfileDTO, error) {
	listed, err := a.manager.Profiles(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]ProfileDTO, 0, len(listed))
	for _, summary := range listed {
		out = append(out, ProfileDTO{
			Name:    summary.Name,
			Default: summary.Default,
			Entries: summary.Entries,
		})
	}
	return out, nil
}

// Entries returns the entries of one profile (EIR-002).
func (a *App) Entries(name string) ([]EntryDTO, error) {
	views, err := a.manager.Entries(context.Background(), name)
	if err != nil {
		return nil, err
	}
	out := make([]EntryDTO, 0, len(views))
	for _, view := range views {
		placements := make([]PlacementDTO, 0, len(view.Placements))
		for _, placement := range view.Placements {
			placements = append(placements, PlacementDTO{
				Display: placement.Display,
				Rect:    placement.Rect,
				State:   placement.State,
			})
		}
		out = append(out, EntryDTO{
			Application: view.Application,
			Kind:        view.Kind,
			Running:     view.Running,
			Placements:  placements,
		})
	}
	return out, nil
}

// Rename changes a profile's name, keeping everything else (FR-042).
func (a *App) Rename(from, to string) error {
	return a.manager.Rename(context.Background(), from, to)
}

// Delete removes a profile. The confirmation naming it (FR-043) is the page's,
// and by the time this is called the user has already been asked.
func (a *App) Delete(name string) error {
	return a.manager.Delete(context.Background(), name)
}

// SetDefault marks one profile as the one applied at sign-in (FR-040).
func (a *App) SetDefault(name string) error {
	return a.manager.SetDefault(context.Background(), name)
}

// ClearDefault leaves no profile marked, so nothing is applied at sign-in
// (FR-039).
func (a *App) ClearDefault() error {
	return a.manager.ClearDefault(context.Background())
}

// RemoveEntry drops one application from a profile (FR-042).
func (a *App) RemoveEntry(name, application string) error {
	return a.manager.RemoveEntry(context.Background(), name, application)
}

// Apply restores a profile now (FR-041). It runs on the caller's goroutine,
// which is Wails' own worker rather than the window's thread, so a restore that
// waits minutes for a window does not freeze the manager.
func (a *App) Apply(name string) (ReportDTO, error) {
	report, err := a.tray.Apply(context.Background(), name)
	if err != nil {
		return ReportDTO{}, err
	}
	return reportOf(name, report), nil
}

// Capture reads the desktop and returns the candidates for review (FR-010).
// Nothing is written: the review is held here until it is confirmed or dropped.
func (a *App) Capture(basedOn string) (ReviewDTO, error) {
	review, err := a.captures.Review(context.Background(), basedOn)
	if err != nil {
		return ReviewDTO{}, err
	}
	a.mutex.Lock()
	a.review = review
	a.mutex.Unlock()

	entries := make([]ReviewEntryDTO, 0, len(review.Entries))
	for _, entry := range review.Entries {
		entries = append(entries, ReviewEntryDTO{
			Application: entry.Application.Value,
			Kind:        entry.Application.Kind.String(),
			Windows:     len(entry.Placements),
		})
	}
	return ReviewDTO{Entries: entries, Unreadable: review.Unreadable}, nil
}

// CancelCapture drops the review without writing anything (FR-016). It is
// called when the user leaves the review screen, so a capture that is walked
// away from leaves nothing behind rather than waiting to be confirmed later.
func (a *App) CancelCapture() {
	a.mutex.Lock()
	a.review = application.Review{}
	a.mutex.Unlock()
	a.log.Step("a capture was cancelled, so nothing was written")
}

// SaveCapture writes the entries the user kept, under the name they gave
// (FR-011). keep names the applications remaining in the review; everything
// else is dropped.
func (a *App) SaveCapture(name string, keep []string, replace bool) error {
	a.mutex.Lock()
	review := a.review
	a.mutex.Unlock()

	wanted := make(map[string]bool, len(keep))
	for _, application := range keep {
		wanted[application] = true
	}
	entries := make([]domain.Entry, 0, len(keep))
	for _, entry := range review.Entries {
		if wanted[entry.Application.Value] {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return fmt.Errorf("a profile needs at least one application in it")
	}
	if _, err := a.captures.Save(context.Background(), name, entries, replace); err != nil {
		return err
	}
	a.mutex.Lock()
	a.review = application.Review{}
	a.mutex.Unlock()
	return nil
}

// Report returns the report of the most recent restore (FR-044).
func (a *App) Report() ReportDTO {
	report, held := a.restores.Last()
	if !held {
		return ReportDTO{}
	}
	return reportOf(report.Profile, report)
}

// StartsWithWindows reports whether the agent runs at sign-in (FR-053).
func (a *App) StartsWithWindows() (bool, error) { return a.manager.StartsWithWindows() }

// SetStartsWithWindows turns the sign-in entry on or off (FR-053).
func (a *App) SetStartsWithWindows(enabled bool) error {
	return a.manager.SetStartsWithWindows(enabled)
}

// reportOf turns a report into what the page shows for it.
func reportOf(profile string, report *application.Report) ReportDTO {
	if report == nil {
		return ReportDTO{}
	}
	entries := make([]EntryReportDTO, 0)
	for _, entry := range report.Entries() {
		entries = append(entries, EntryReportDTO{
			Application: entry.Application.Value,
			Satisfied:   entry.Satisfied,
			Reason:      entry.Reason,
			Notes:       entry.Notes,
		})
	}
	return ReportDTO{
		Held:    true,
		Profile: profile,
		Summary: report.Summary(),
		Notes:   report.SortedNotes(),
		Entries: entries,
	}
}

// CheckForUpdates asks whether a newer version has been released (FR-058).
//
// unbidden is true for the check the agent runs by itself and false for one the
// user pressed a button for. Skipping silences the first, never the second: a
// user who asks is entitled to the answer either way.
func (a *App) CheckForUpdates(unbidden bool) (UpdateDTO, error) {
	status, err := a.updates.Check(context.Background(), unbidden)
	if err != nil {
		return UpdateDTO{}, err
	}
	return UpdateDTO{
		Enabled:     status.Enabled,
		Reached:     status.Reached,
		Current:     status.Current,
		Latest:      status.Latest,
		Available:   status.Available,
		Skipped:     status.Skipped,
		DownloadURL: status.DownloadURL,
		PageURL:     status.PageURL,
	}, nil
}

// SkipVersion records a released version the user does not want offered again
// (FR-058).
func (a *App) SkipVersion(version string) error { return a.updates.Skip(version) }

// UpdateCheckEnabled reports whether the update check is on (FR-059).
func (a *App) UpdateCheckEnabled() (bool, error) { return a.updates.Enabled() }

// SetUpdateCheckEnabled turns the update check on or off. Turned off, the
// product makes no network connection at all, which is what FR-059 promises and
// what C-4 rests on.
func (a *App) SetUpdateCheckEnabled(enabled bool) error { return a.updates.SetEnabled(enabled) }
