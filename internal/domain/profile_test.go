package domain

import (
	"errors"
	"strings"
	"testing"
)

// The measured identities from appendix E, used so the tests exercise the
// shapes the product actually meets rather than invented ones.
const (
	claudeModelID  = "Claude_pzs8sxrjxfjjc!Claude"
	discordUpdater = `Discord\Update.exe --processStart Discord.exe`
	stellodyPath   = `C:\Program Files\Stellody\Stellody.exe`
	leftMonitorID  = "DISPLAY#HSJ1340#5&14514d51&0&UID4356"
	rightMonitorID = "DISPLAY#HSJ1340#5&14514d51&0&UID4354"
)

func mustIdentity(t *testing.T, kind IdentityKind, value string) ApplicationIdentity {
	t.Helper()
	identity, err := NewApplicationIdentity(kind, value)
	if err != nil {
		t.Fatalf("building identity %q: %v", value, err)
	}
	return identity
}

func mustDisplay(t *testing.T, monitorID string) DisplayIdentity {
	t.Helper()
	identity, err := NewDisplayIdentity(monitorID)
	if err != nil {
		t.Fatalf("building display identity %q: %v", monitorID, err)
	}
	return identity
}

func placementOn(t *testing.T, monitorID string) Placement {
	t.Helper()
	return Placement{
		Display: mustDisplay(t, monitorID),
		Rect:    Rect{X: -4079, Y: 1407, Width: 3872, Height: 2312},
		State:   ShowMaximised,
	}
}

func TestAnIdentityIsTrimmedAndKeptWholeOtherwise(t *testing.T) {
	t.Parallel()
	identity := mustIdentity(t, KindUpdaterCommand, "  "+discordUpdater+"  ")
	if identity.Value != discordUpdater {
		t.Fatalf("value not trimmed: %q", identity.Value)
	}
	if identity.String() != "updater:"+discordUpdater {
		t.Fatalf("unexpected rendering: %s", identity)
	}
}

func TestAnIdentityWithoutAValueIsRefused(t *testing.T) {
	t.Parallel()
	if _, err := NewApplicationIdentity(KindPath, "   "); !errors.Is(err, ErrEmptyIdentity) {
		t.Fatalf("expected ErrEmptyIdentity, got %v", err)
	}
	if _, err := NewDisplayIdentity(""); !errors.Is(err, ErrEmptyIdentity) {
		t.Fatalf("expected ErrEmptyIdentity, got %v", err)
	}
}

func TestAnIdentityOfAnUnknownKindIsRefused(t *testing.T) {
	t.Parallel()
	_, err := NewApplicationIdentity(IdentityKind(7), stellodyPath)
	if !errors.Is(err, ErrUnknownIdentityKind) {
		t.Fatalf("expected ErrUnknownIdentityKind, got %v", err)
	}
}

func TestIdentitiesMatchWithoutRegardToCase(t *testing.T) {
	t.Parallel()
	// A profile captured from one spelling of a Windows path has to match a
	// window reporting another.
	lower := mustIdentity(t, KindPath, strings.ToLower(stellodyPath))
	upper := mustIdentity(t, KindPath, strings.ToUpper(stellodyPath))
	if !lower.Equal(upper) {
		t.Fatal("the same path in different case did not match")
	}
	if lower.Equal(mustIdentity(t, KindAppUserModelID, stellodyPath)) {
		t.Fatal("the same value under a different kind matched")
	}
}

func TestASecondCopyOfOneProgramIsRecognisedAsTheSameProgram(t *testing.T) {
	t.Parallel()
	// The question SameProgram answers is "is this thing me", asked by a capture
	// that must never record this product. Two copies of one program live at two
	// paths, which Equal correctly calls different.
	installed := mustIdentity(t, KindPath, `C:\Users\Someone\AppData\Local\Programs\Stellody\Stellody.exe`)
	built := mustIdentity(t, KindPath, `D:\work\Stellody\build\bin\STELLODY.EXE`)
	if installed.Equal(built) {
		t.Fatal("two paths that differ were treated as one identity")
	}
	if !installed.SameProgram(built) {
		t.Fatal("a second copy of the same program was not recognised")
	}
	if !installed.SameProgram(installed) {
		t.Fatal("a program did not recognise itself")
	}
}

func TestAnotherProgramIsNotTakenForThisOne(t *testing.T) {
	t.Parallel()
	mine := mustIdentity(t, KindPath, stellodyPath)
	other := mustIdentity(t, KindPath, `C:\Program Files\Other\Other.exe`)
	if mine.SameProgram(other) {
		t.Fatal("a different program was taken for this one")
	}
	// The file name is only compared between two paths: an identity of another
	// kind is a different way of naming a program altogether.
	if mine.SameProgram(mustIdentity(t, KindAppUserModelID, stellodyPath)) {
		t.Fatal("the same value under a different kind was taken for this program")
	}
	if mustIdentity(t, KindUpdaterCommand, discordUpdater).SameProgram(mine) {
		t.Fatal("an updater command was taken for a path")
	}
	// A value carrying no separator at all is its own file name.
	bare := mustIdentity(t, KindPath, "Stellody.exe")
	if !bare.SameProgram(mustIdentity(t, KindPath, `C:\elsewhere\stellody.exe`)) {
		t.Fatal("a bare file name did not match the same file in a directory")
	}
}

func TestTwoDisplaysOfOneModelAreToldApartByTheirIdentity(t *testing.T) {
	t.Parallel()
	// Measured: the left and right screens share model HSJ1340 and differ only
	// in the UID. Treating them as one would place windows on the wrong screen.
	left := mustDisplay(t, leftMonitorID)
	right := mustDisplay(t, rightMonitorID)
	if left.Equal(right) {
		t.Fatal("two screens of the same model matched each other")
	}
	if !left.Equal(mustDisplay(t, strings.ToLower(leftMonitorID))) {
		t.Fatal("one screen did not match itself in different case")
	}
	if left.String() != leftMonitorID {
		t.Fatalf("unexpected rendering: %s", left)
	}
}

func TestAPlacementNeedsADisplayARectangleAndAKnownState(t *testing.T) {
	t.Parallel()
	good := placementOn(t, leftMonitorID)
	if err := good.Validate(); err != nil {
		t.Fatalf("a complete placement was refused: %v", err)
	}

	noDisplay := good
	noDisplay.Display = DisplayIdentity{}
	if err := noDisplay.Validate(); !errors.Is(err, ErrEmptyIdentity) {
		t.Fatalf("expected ErrEmptyIdentity, got %v", err)
	}

	noArea := good
	noArea.Rect = Rect{X: 10, Y: 10}
	if err := noArea.Validate(); !errors.Is(err, ErrInvalidRect) {
		t.Fatalf("expected ErrInvalidRect, got %v", err)
	}

	badState := good
	badState.State = ShowState(42)
	if err := badState.Validate(); !errors.Is(err, ErrUnknownShowState) {
		t.Fatalf("expected ErrUnknownShowState, got %v", err)
	}
}

func TestAnEntryMayCarrySeveralWindows(t *testing.T) {
	t.Parallel()
	// Measured on 2026-09-19: one process held two windows, so an entry cannot
	// assume one window per application.
	entry := Entry{Application: mustIdentity(t, KindPath, stellodyPath)}
	entry = entry.WithPlacement(placementOn(t, leftMonitorID))
	entry = entry.WithPlacement(placementOn(t, rightMonitorID))
	if len(entry.Placements) != 2 {
		t.Fatalf("expected two placements, got %d", len(entry.Placements))
	}
	if err := entry.Validate(); err != nil {
		t.Fatalf("a valid entry was refused: %v", err)
	}
}

func TestAddingAPlacementLeavesTheOriginalEntryAlone(t *testing.T) {
	t.Parallel()
	original := Entry{Application: mustIdentity(t, KindPath, stellodyPath)}
	original = original.WithPlacement(placementOn(t, leftMonitorID))
	extended := original.WithPlacement(placementOn(t, rightMonitorID))

	if len(original.Placements) != 1 {
		t.Fatalf("the original entry grew to %d placements", len(original.Placements))
	}
	if len(extended.Placements) != 2 {
		t.Fatalf("the copy holds %d placements", len(extended.Placements))
	}
	if original.Placements[0].Display.Equal(extended.Placements[1].Display) {
		t.Fatal("the copy wrote over the original's placement")
	}
}

func TestAnEntryRecordsWhetherItsApplicationShouldRun(t *testing.T) {
	t.Parallel()
	entry := Entry{Application: mustIdentity(t, KindAppUserModelID, claudeModelID)}
	running := entry.WithRunning(true)
	if entry.Running {
		t.Fatal("the original entry was changed")
	}
	if !running.Running {
		t.Fatal("the copy did not record that the application should run")
	}
}

func TestAnEntryIsRefusedWhenItsApplicationOrPlacementIsWrong(t *testing.T) {
	t.Parallel()
	if err := (Entry{}).Validate(); !errors.Is(err, ErrEmptyIdentity) {
		t.Fatalf("expected ErrEmptyIdentity, got %v", err)
	}
	entry := Entry{Application: mustIdentity(t, KindPath, stellodyPath)}
	entry = entry.WithPlacement(Placement{Display: mustDisplay(t, leftMonitorID)})
	if err := entry.Validate(); !errors.Is(err, ErrInvalidRect) {
		t.Fatalf("expected ErrInvalidRect, got %v", err)
	}
	if !strings.Contains(entry.Validate().Error(), "placement 0") {
		t.Fatalf("the error does not say which placement: %v", entry.Validate())
	}
}

func TestAProfileNeedsAName(t *testing.T) {
	t.Parallel()
	if _, err := NewProfile("   "); !errors.Is(err, ErrEmptyProfileName) {
		t.Fatalf("expected ErrEmptyProfileName, got %v", err)
	}
	profile, err := NewProfile("  Desk  ")
	if err != nil {
		t.Fatalf("a named profile was refused: %v", err)
	}
	if profile.Name != "Desk" {
		t.Fatalf("the name was not trimmed: %q", profile.Name)
	}
}

func TestAProfileMayNameAnApplicationOnlyOnce(t *testing.T) {
	t.Parallel()
	// Two entries for one application would give a restore two answers to the
	// same question. Several windows belong in one entry's placements.
	entry := Entry{Application: mustIdentity(t, KindPath, stellodyPath)}
	_, err := NewProfile("Desk", entry, entry)
	if !errors.Is(err, ErrDuplicateEntry) {
		t.Fatalf("expected ErrDuplicateEntry, got %v", err)
	}
}

func TestAProfileRefusesAnInvalidEntryAndSaysWhich(t *testing.T) {
	t.Parallel()
	_, err := NewProfile("Desk", Entry{})
	if !errors.Is(err, ErrEmptyIdentity) {
		t.Fatalf("expected ErrEmptyIdentity, got %v", err)
	}
	if !strings.Contains(err.Error(), `profile "Desk"`) {
		t.Fatalf("the error does not name the profile: %v", err)
	}
}

func TestAProfileGrowsWithoutChangingTheOriginal(t *testing.T) {
	t.Parallel()
	original, err := NewProfile("Desk")
	if err != nil {
		t.Fatalf("building the profile: %v", err)
	}
	extended := original.WithEntry(Entry{Application: mustIdentity(t, KindPath, stellodyPath)})
	if len(original.Entries) != 0 {
		t.Fatalf("the original profile grew to %d entries", len(original.Entries))
	}
	if len(extended.Entries) != 1 {
		t.Fatalf("the copy holds %d entries", len(extended.Entries))
	}
}

func TestOnlyACopyIsMarkedAsTheDefaultProfile(t *testing.T) {
	t.Parallel()
	original, err := NewProfile("Desk")
	if err != nil {
		t.Fatalf("building the profile: %v", err)
	}
	marked := original.WithDefault(true)
	if original.Default {
		t.Fatal("the original profile was marked")
	}
	if !marked.Default || marked.WithDefault(false).Default {
		t.Fatal("the mark did not follow the copy")
	}
}

func TestAProfileFindsTheEntryForAnApplication(t *testing.T) {
	t.Parallel()
	stellody := mustIdentity(t, KindPath, stellodyPath)
	claude := mustIdentity(t, KindAppUserModelID, claudeModelID)
	profile, err := NewProfile("Desk", Entry{Application: stellody})
	if err != nil {
		t.Fatalf("building the profile: %v", err)
	}

	found, ok := profile.Find(stellody)
	if !ok || !found.Application.Equal(stellody) {
		t.Fatal("the entry that is there was not found")
	}
	if _, ok := profile.Find(claude); ok {
		t.Fatal("an entry that is not there was found")
	}
}
