package application

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/oernster/ScreenState/internal/domain"
)

// EntryReport is what became of one profile entry during a restore.
//
// Satisfied and Reason are two halves of one answer: an entry is satisfied or
// it is not; an entry that is not always carries why. An entry may be
// satisfied and still carry notes, which is how a substituted display or a
// window that had to be asked for reaches the user without being called a
// failure.
type EntryReport struct {
	Application domain.ApplicationIdentity
	Satisfied   bool
	// Reason says why an unsatisfied entry was not satisfied. It is empty for a
	// satisfied one.
	Reason string
	// Notes record what happened on the way, whether or not it went well.
	Notes []string
}

// Report is what a restore did, in full, for the user to read afterwards
// (FR-044). It is the product's answer to a restore that silently half worked.
type Report struct {
	// Profile is the name of the profile restored.
	Profile string
	Started time.Time
	// Finished is the zero time while the restore is still running.
	Finished time.Time
	// Replaced is a restore this one stood down when it began (FR-061).
	Replaced bool
	// WasReplaced marks a restore that a newer request stood down (FR-061).
	WasReplaced bool
	// Notes record what happened to the restore as a whole rather than to one
	// entry, such as a display going away part way through (FR-057).
	Notes []string

	entries []EntryReport
	index   map[string]int
}

// NewReport starts the report of a restore of the named profile.
func NewReport(profile string, started time.Time) *Report {
	return &Report{
		Profile: profile,
		Started: started,
		index:   make(map[string]int),
	}
}

// Note records something about the restore as a whole.
func (report *Report) Note(format string, args ...any) {
	report.Notes = append(report.Notes, fmt.Sprintf(format, args...))
}

// entryAt returns the position of an application's report, creating it as
// outstanding on first mention. Every entry starts outstanding on purpose: a
// restore that ends early then reports what it had not reached, rather than
// reporting nothing about it.
func (report *Report) entryAt(application domain.ApplicationIdentity) int {
	key := strings.ToLower(application.String())
	if at, known := report.index[key]; known {
		return at
	}
	report.entries = append(report.entries, EntryReport{
		Application: application,
		Reason:      "not reached",
	})
	report.index[key] = len(report.entries) - 1
	return len(report.entries) - 1
}

// Track records an entry as outstanding before anything has been tried, so the
// report can be read at any moment and still name every entry.
func (report *Report) Track(application domain.ApplicationIdentity) {
	report.entryAt(application)
}

// NoteEntry records something that happened to one entry, without settling
// whether it was satisfied.
func (report *Report) NoteEntry(application domain.ApplicationIdentity, format string, args ...any) {
	at := report.entryAt(application)
	report.entries[at].Notes = append(report.entries[at].Notes, fmt.Sprintf(format, args...))
}

// Satisfy records an entry as satisfied, clearing any reason it was carrying.
func (report *Report) Satisfy(application domain.ApplicationIdentity) {
	at := report.entryAt(application)
	report.entries[at].Satisfied = true
	report.entries[at].Reason = ""
}

// Fail records an entry as not satisfied and why. It does not overwrite a
// satisfied entry: an entry with two placements, one of which could not be
// applied, is not satisfied, so the failure is recorded before the success can
// be claimed and the caller decides which it is.
func (report *Report) Fail(application domain.ApplicationIdentity, format string, args ...any) {
	at := report.entryAt(application)
	report.entries[at].Satisfied = false
	report.entries[at].Reason = fmt.Sprintf(format, args...)
}

// Finish closes the report at the given moment.
func (report *Report) Finish(at time.Time) { report.Finished = at }

// Entries returns the report of every entry, in the order the entries were
// first mentioned, which is the order the profile holds them.
func (report *Report) Entries() []EntryReport {
	copied := make([]EntryReport, len(report.entries))
	copy(copied, report.entries)
	return copied
}

// Outstanding returns the entries that were not satisfied.
func (report *Report) Outstanding() []EntryReport {
	var outstanding []EntryReport
	for _, entry := range report.entries {
		if !entry.Satisfied {
			outstanding = append(outstanding, entry)
		}
	}
	return outstanding
}

// Counts returns how many entries were satisfied and how many were not.
func (report *Report) Counts() (satisfied int, outstanding int) {
	for _, entry := range report.entries {
		if entry.Satisfied {
			satisfied++
			continue
		}
		outstanding++
	}
	return satisfied, outstanding
}

// Summary is the one line the tray shows: it decides whether FR-045 marks the
// icon, so it says plainly whether anything is outstanding.
func (report *Report) Summary() string {
	satisfied, outstanding := report.Counts()
	if outstanding == 0 {
		return fmt.Sprintf("%s: %d of %d satisfied, none outstanding",
			report.Profile, satisfied, satisfied)
	}
	return fmt.Sprintf("%s: %d of %d satisfied, %d outstanding",
		report.Profile, satisfied, satisfied+outstanding, outstanding)
}

// SortedNotes returns the whole-restore notes in a settled order, so two runs
// that did the same things read the same way.
func (report *Report) SortedNotes() []string {
	sorted := make([]string, len(report.Notes))
	copy(sorted, report.Notes)
	sort.Strings(sorted)
	return sorted
}
