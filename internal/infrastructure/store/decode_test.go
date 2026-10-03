package store

import (
	"errors"
	"strings"
	"testing"
)

// S-8: every build writes "running" on every entry, so an entry without it was
// edited by hand. Reading it as not running would change what the profile does
// without a word; it is refused instead.
func TestAnEntryThatDoesNotSayWhetherItRunsIsRefused(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"format":1,"name":"Desk","entries":[{"application":{"kind":"path","value":"C:\\a.exe"}}]}`)
	_, _, err := decode(raw)
	if !errors.Is(err, ErrUnreadable) {
		t.Fatalf("an entry without running decoded as %v", err)
	}
	if !strings.Contains(err.Error(), "running") {
		t.Fatalf("the reason does not name what is missing: %v", err)
	}
	if _, _, err := decode([]byte(`{"format":1,"name":"Desk","entries":[{"application":{"kind":"path","value":"C:\\a.exe"},"running":false}]}`)); err != nil {
		t.Fatalf("an entry that says it is not running was refused: %v", err)
	}
}

// S-8: a file that states no format is not from a newer version; nor is one
// stating a format no build writes. Saying so would send the user looking for an update that
// does not exist.
func TestAFileWithNoFormatIsNotCalledNewer(t *testing.T) {
	t.Parallel()
	for label, raw := range map[string]string{
		"no format":       `{"name":"Desk","entries":[]}`,
		"format zero":     `{"format":0,"name":"Desk","entries":[]}`,
		"negative format": `{"format":-1,"name":"Desk","entries":[]}`,
	} {
		_, _, err := decode([]byte(raw))
		if errors.Is(err, ErrUnknownFormat) {
			t.Errorf("%s was called a newer format: %v", label, err)
		}
		if !errors.Is(err, ErrUnreadable) {
			t.Errorf("%s decoded as %v", label, err)
		}
	}
	if _, _, err := decode([]byte(`{"format":2,"name":"Desk","entries":[]}`)); !errors.Is(err, ErrUnknownFormat) {
		t.Fatalf("a newer format answered %v", err)
	}
}
