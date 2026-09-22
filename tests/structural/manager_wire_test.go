package structural

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// wireRecords names, for each record the manager's page reads off an answer,
// the one variable name the page reads it under.
//
// A name is the only thing a script with no compiler behind it says about what
// an object is. The scripts once called three different records entry (a menu
// element too), so a check keyed on that name could not tell which
// record a read was of: it would have raised false alarms on correct code,
// which is how a guard gets worked around and then dropped. Each record now has
// a name of its own, which turns "what is this object" into something a scan can
// answer without guessing.
var wireRecords = []struct {
	variable string
	dto      string
}{
	{"profileEntry", "EntryDTO"},
	{"placement", "PlacementDTO"},
	{"candidate", "ReviewEntryDTO"},
	{"review", "ReviewDTO"},
	{"outcome", "EntryReportDTO"},
	{"report", "ReportDTO"},
}

// fieldRead matches a read of a field off a variable of the given name. What
// comes before the name may not be a word character or a dot, so a property
// that happens to share the name (state.report.x) is not taken for the variable.
func fieldRead(variable string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)(?:^|[^\w$.])` + regexp.QuoteMeta(variable) +
		`\.([a-zA-Z][a-zA-Z0-9]*)\b`)
}

// TestTheManagerRecordsAreReadAsTheyAreSent holds every record the page reads
// off an answer to the struct the agent marshals for it.
//
// The agent states a field once as a json tag and the page states it again as a
// property name; nothing compares the two, so a field misspelt or renamed on
// either side reads as undefined and a row quietly says "undefined" where a
// name belongs.
//
// Each variable name must also be seen at least once. A script that stops
// reading a record under its name would otherwise leave this passing while it
// guarded nothing.
func TestTheManagerRecordsAreReadAsTheyAreSent(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "frontend", "dist")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the manager page: %v", err)
	}
	scripts := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".js") {
			scripts[entry.Name()] = readSource(t, filepath.Join(dir, entry.Name()))
		}
	}
	if len(scripts) == 0 {
		t.Fatal("no manager scripts were found, the walk is wrong")
	}
	for _, record := range wireRecords {
		tags := jsonTagsOf(t, filepath.Join(root, "app.go"), record.dto)
		read := fieldRead(record.variable)
		var seen int
		for name, text := range scripts {
			for _, match := range read.FindAllStringSubmatch(text, -1) {
				seen++
				if !tags[match[1]] {
					t.Errorf("%s reads %s.%s, which %s never sends",
						name, record.variable, match[1], record.dto)
				}
			}
		}
		if seen == 0 {
			t.Errorf("no script reads a field of %s under the name %s, so it is not being checked",
				record.dto, record.variable)
		}
	}
}
