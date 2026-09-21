package domain

import "testing"

// Claude before and after an update: the path carries the version, the model id
// does not (FR-071, measured 2026-09-21).
const (
	claudeBefore = `C:\WindowsApps\Claude_1.0.0.0_x64__pzs8sxrjxfjjc\app\Claude.exe`
	claudeAfter  = `C:\WindowsApps\Claude_1.1.0.0_x64__pzs8sxrjxfjjc\app\Claude.exe`
)

// FR-071: a running window is recognised as a packaged application's by its
// model id once an update has moved the path, while Equal stays exact.
func TestAPackagedApplicationIsRecognisedAfterAnUpdateMovesItsPath(t *testing.T) {
	t.Parallel()
	captured := mustIdentity(t, KindPath, claudeBefore).WithModelID(claudeModelID)
	updated := mustIdentity(t, KindPath, claudeAfter).WithModelID(claudeModelID)
	legacy := mustIdentity(t, KindAppUserModelID, claudeModelID)

	for name, named := range map[string]ApplicationIdentity{
		"the old path with its model id": captured,
		"the model id alone":             legacy,
	} {
		if !named.Recognises(updated) || !updated.Recognises(named) {
			t.Errorf("%s did not recognise the updated window", name)
		}
	}
	if captured.Equal(updated) {
		t.Error("Equal matched two paths; it must stay exact")
	}
	profile, err := NewProfile("Desk", Entry{Application: captured})
	if err != nil {
		t.Fatalf("building the profile: %v", err)
	}
	if _, found := profile.Find(updated); !found {
		t.Error("the profile did not find its entry for the updated window")
	}
}

// Recognising is wider than Equal only through a model id both sides carry.
func TestRecognisingNeedsTheSameModelIDOnBothSides(t *testing.T) {
	t.Parallel()
	captured := mustIdentity(t, KindPath, claudeBefore).WithModelID(claudeModelID)
	cases := map[string]ApplicationIdentity{
		"a moved path with no model id":   mustIdentity(t, KindPath, claudeAfter),
		"a moved path with another id":    mustIdentity(t, KindPath, claudeAfter).WithModelID("Other_abc!Other"),
		"an unrelated path":               mustIdentity(t, KindPath, stellodyPath),
		"an updater naming the same text": mustIdentity(t, KindUpdaterCommand, claudeModelID),
	}
	for name, other := range cases {
		if captured.Recognises(other) || other.Recognises(captured) {
			t.Errorf("%s was recognised", name)
		}
	}
	plain := mustIdentity(t, KindPath, stellodyPath)
	if !plain.Recognises(mustIdentity(t, KindPath, stellodyPath)) {
		t.Error("an identity did not recognise its equal")
	}
	if plain.Recognises(mustIdentity(t, KindPath, claudeAfter)) {
		t.Error("two paths with no model id were recognised as one")
	}
}
