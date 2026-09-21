package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNoListReachesThePageAsNull holds the rule the capture defect broke.
//
// A nil slice is marshalled as null rather than as an empty array, so a page
// that reads a length off it throws. That throw lands after the call the page
// was waiting on has already answered, which puts it outside the guard around
// the call: the promise rejects with nobody holding it and the window keeps
// saying the words it was waiting with. Measured on 2026-09-20 against a
// capture that found nothing unreadable.
//
// It covers the values the page is handed with no error beside them. A DTO
// returned alongside an error is not here: that path shows the error and never
// reads the value.
func TestNoListReachesThePageAsNull(t *testing.T) {
	t.Parallel()
	for name, value := range map[string]any{
		"no report has run yet": noReport(),
		"a report with nothing to note": ReportDTO{
			Held: true, Profile: "Desk", Summary: "one of one",
			Notes:   stated[string](nil),
			Entries: []EntryReportDTO{{Application: "Notepad", Notes: stated[string](nil)}},
		},
		"a capture with nothing unreadable": ReviewDTO{
			Entries:    []ReviewEntryDTO{{Application: "Notepad", Kind: "window", Windows: 1}},
			Unreadable: stated[string](nil),
		},
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if strings.Contains(string(encoded), "null") {
			t.Errorf("%s states a null the page would read a length from: %s", name, encoded)
		}
	}
}

// TestStatedAnswersAList proves the helper every list on the wire goes through.
func TestStatedAnswersAList(t *testing.T) {
	t.Parallel()
	encoded, err := json.Marshal(stated[string](nil))
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	if string(encoded) != "[]" {
		t.Errorf("nothing was stated as %s rather than as an empty list", encoded)
	}
	held := stated([]string{"one"})
	if len(held) != 1 || held[0] != "one" {
		t.Errorf("a list that held something came back as %v", held)
	}
}
