package application

import (
	"context"
	"errors"
	"testing"
)

// NFR-REL-002: a profile file that cannot be read is presented to the user by
// name with its reason, while the rest are listed as usual. It used to be named
// in the log alone, which is not where anybody looks for a missing profile.
func TestTheManagerNamesAProfileItCannotRead(t *testing.T) {
	t.Parallel()
	store := newFakeStore(profileOf(t, "Desk", pigeonpost))
	store.unreadable = []UnreadableProfile{
		{File: "Evening.json", Reason: "profile was written in a newer format: format 3"},
	}
	service := NewManagerService(store, &fakeStartup{}, &fakeDesktop{}, &fakeLog{})

	unreadable, err := service.Unreadable(context.Background())
	if err != nil {
		t.Fatalf("asking what could not be read: %v", err)
	}
	if len(unreadable) != 1 || unreadable[0].File != "Evening.json" ||
		!containsText(unreadable[0].Reason, "newer format") {
		t.Fatalf("the unreadable file was stated as %+v", unreadable)
	}
	listed, err := service.Profiles(context.Background())
	if err != nil || len(listed) != 1 || listed[0].Name != "Desk" {
		t.Fatalf("the readable profile was not listed beside it: %+v (%v)", listed, err)
	}
}

// A store that cannot say what it left out says so, rather than answering an
// empty list that would read as "nothing is wrong".
func TestAStoreThatCannotBeCheckedIsSaid(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.unreadableErr = errors.New("access is denied")
	service := NewManagerService(store, &fakeStartup{}, &fakeDesktop{}, &fakeLog{})

	if _, err := service.Unreadable(context.Background()); !containsText(errText(err), "access is denied") {
		t.Fatalf("the refusal was not passed on: %v", err)
	}
}

// errText is an error's words, empty for none, so a nil error fails the check
// rather than the test.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
