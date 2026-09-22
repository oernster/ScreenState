package structural

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// managerPage holds the manager's page; its records are declared across the
// root package's files, split by subject.
var managerPage = filepath.Join("frontend", "dist")

// wireRecord names, for one record crossing between a page and its program, the
// one variable name the page holds it under and the struct the program declares.
type wireRecord struct {
	page     string
	pkg      string
	variable string
	dto      string
}

// wireRecords is every record either page reads off an answer or builds to send.
//
// A name is the only thing a script with no compiler behind it says about what
// an object is. The manager's scripts once called three different records entry
// (a menu element too), so a check keyed on that name could not tell which
// record a read was of: it would have raised false alarms on correct code,
// which is how a guard gets worked around and then dropped. Each record now has
// a name of its own on its page, which turns "what is this object" into
// something a scan can answer without guessing.
var wireRecords = []wireRecord{
	{managerPage, ".", "state", "StateDTO"},
	{managerPage, ".", "profile", "ProfileDTO"},
	{managerPage, ".", "unreadableFile", "UnreadableDTO"},
	{managerPage, ".", "profileEntry", "EntryDTO"},
	{managerPage, ".", "placement", "PlacementDTO"},
	{managerPage, ".", "candidate", "ReviewEntryDTO"},
	{managerPage, ".", "review", "ReviewDTO"},
	{managerPage, ".", "outcome", "EntryReportDTO"},
	{managerPage, ".", "report", "ReportDTO"},
	{managerPage, ".", "progress", "ProgressDTO"},
	{managerPage, ".", "found", "UpdateDTO"},
	{managerPage, ".", "about", "AboutDTO"},
	{managerPage, ".", "credit", "CreditDTO"},
	{pageDir, "installer", "state", "StateDTO"},
	{pageDir, "installer", "progress", "Progress"},
	{pageDir, "installer", "freshChoices", "OptionsDTO"},
	{pageDir, "installer", "choices", "OptionsDTO"},
}

// fieldRead matches a read of a field off a variable of the given name. What
// comes before the name may not be a word character or a dot, so a property
// that happens to share the name (report.state.x) is not taken for the variable.
// Nor may it be a quote, a slash or a hyphen, which is what stands before a file
// name in a string: the first run of this check took the artwork 'profile.png'
// for a read of profile.png.
func fieldRead(variable string) *regexp.Regexp {
	return regexp.MustCompile("(?m)(?:^|[^\\w$.'\"`/-])" + regexp.QuoteMeta(variable) +
		`\.([a-zA-Z][a-zA-Z0-9]*)\b`)
}

// literalOf matches an object literal given to a variable of the given name,
// which is how a page builds a record to send; literalKey matches one key in it.
func literalOf(variable string) *regexp.Regexp {
	return regexp.MustCompile(`\b(?:const|let|var)\s+` + regexp.QuoteMeta(variable) +
		`\s*=\s*\{([^{}]*)\}`)
}

var literalKey = regexp.MustCompile(`(?:^|[{,\s])([a-zA-Z][a-zA-Z0-9]*)\s*:`)

// fieldsNamed answers every field a page's scripts name on a record held under
// the given variable, whether read off it or written into its literal, each
// with the script naming it.
func fieldsNamed(scripts map[string]string, variable string) [][2]string {
	var named [][2]string
	read, literal := fieldRead(variable), literalOf(variable)
	for script, text := range scripts {
		for _, match := range read.FindAllStringSubmatch(text, -1) {
			named = append(named, [2]string{script, match[1]})
		}
		for _, body := range literal.FindAllStringSubmatch(text, -1) {
			for _, key := range literalKey.FindAllStringSubmatch(body[1], -1) {
				named = append(named, [2]string{script, key[1]})
			}
		}
	}
	return named
}

// scriptsOf reads every script of a page, not a list of named files: the
// manager's page grew a second script and a test that knew only the first would
// have stopped guarding half of it without saying so.
func scriptsOf(t *testing.T, page string) map[string]string {
	t.Helper()
	dir := filepath.Join(repoRoot(t), page)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", page, err)
	}
	scripts := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".js") {
			scripts[entry.Name()] = readSource(t, filepath.Join(dir, entry.Name()))
		}
	}
	if len(scripts) == 0 {
		t.Fatalf("no scripts were found under %s, the walk is wrong", page)
	}
	return scripts
}

// TestEveryRecordIsReadAsItIsSent holds every record either page reads or
// builds to the struct its program declares for it.
//
// The program states a field once as a json tag and the page states it again
// as a property name; the compiler sees only the struct and nothing at all sees
// the page. A field misspelt or renamed on either side therefore produces no
// error anywhere: a read comes back undefined and a row says "undefined" where
// a name belongs, while a key the program does not know is dropped and the
// choice behind it quietly ignored.
//
// Each variable name must also be seen at least once. A script that stops
// holding a record under its name would otherwise leave this passing while it
// guarded nothing.
func TestEveryRecordIsReadAsItIsSent(t *testing.T) {
	root := repoRoot(t)
	scripts := map[string]map[string]string{}
	for _, record := range wireRecords {
		if scripts[record.page] == nil {
			scripts[record.page] = scriptsOf(t, record.page)
		}
		tags := jsonTagsOf(t, filepath.Join(root, record.pkg), record.dto)
		named := fieldsNamed(scripts[record.page], record.variable)
		for _, field := range named {
			if !tags[field[1]] {
				t.Errorf("%s names %s.%s, which %s does not carry",
					filepath.ToSlash(filepath.Join(record.page, field[0])),
					record.variable, field[1], record.dto)
			}
		}
		if len(named) == 0 {
			t.Errorf("no script under %s names a field of %s as %s, so it is not being checked",
				filepath.ToSlash(record.page), record.dto, record.variable)
		}
	}
}
