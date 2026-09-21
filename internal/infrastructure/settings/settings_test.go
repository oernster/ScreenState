package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAFreshInstallWantsTheUpdateCheck holds the default: a check the user has
// never declined is one they have not turned off, so an absent file reads as
// on rather than as off.
func TestAFreshInstallWantsTheUpdateCheck(t *testing.T) {
	t.Parallel()
	prefs := New(t.TempDir())

	enabled, err := prefs.UpdateCheckEnabled()
	if err != nil {
		t.Fatalf("UpdateCheckEnabled: %v", err)
	}
	if !enabled {
		t.Error("a fresh install has the update check off")
	}
	skipped, err := prefs.SkippedVersion()
	if err != nil {
		t.Fatalf("SkippedVersion: %v", err)
	}
	if skipped != "" {
		t.Errorf("a fresh install has passed over %q", skipped)
	}
}

func TestEachSettingSurvivesTheOther(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prefs := New(dir)

	if err := prefs.SetUpdateCheckEnabled(false); err != nil {
		t.Fatalf("SetUpdateCheckEnabled: %v", err)
	}
	if err := prefs.SetSkippedVersion("1.2.0"); err != nil {
		t.Fatalf("SetSkippedVersion: %v", err)
	}
	if err := prefs.SetCloseStrangers(true); err != nil {
		t.Fatalf("SetCloseStrangers: %v", err)
	}

	// Read through a second reader over the same file, so what is asserted is
	// the file rather than anything the first one happens to hold.
	reader := New(dir)
	enabled, err := reader.UpdateCheckEnabled()
	if err != nil {
		t.Fatalf("UpdateCheckEnabled: %v", err)
	}
	if enabled {
		t.Error("turning the check off did not survive writing the skipped version")
	}
	skipped, err := reader.SkippedVersion()
	if err != nil {
		t.Fatalf("SkippedVersion: %v", err)
	}
	if skipped != "1.2.0" {
		t.Errorf("the version passed over reads back as %q", skipped)
	}
	closing, err := reader.CloseStrangers()
	if err != nil {
		t.Fatalf("CloseStrangers: %v", err)
	}
	if !closing {
		t.Error("the choice about unnamed windows did not survive the other two")
	}
}

// TestAFreshInstallMinimisesTheWindowsItDoesNotKnow holds the default arm of
// FR-064: a user who has chosen nothing gets the one that asks nothing of any
// application, since closing a window is a decision only they can make.
func TestAFreshInstallMinimisesTheWindowsItDoesNotKnow(t *testing.T) {
	t.Parallel()
	prefs := New(t.TempDir())

	closing, err := prefs.CloseStrangers()
	if err != nil {
		t.Fatalf("CloseStrangers: %v", err)
	}
	if closing {
		t.Error("a fresh install closes the windows a profile does not name")
	}
}

// TestAFreshInstallHasChosenNoCeiling holds NFR-PERF-003's default: a file that
// says nothing is a user who has chosen none, which the restore turns into the
// specification's fifteen minutes.
func TestAFreshInstallHasChosenNoCeiling(t *testing.T) {
	t.Parallel()
	if _, set, err := New(t.TempDir()).Ceiling(); err != nil || set {
		t.Fatalf("a fresh install has chosen a ceiling (set %v, %v)", set, err)
	}
}

// TestTheCeilingSurvivesBesideTheOthers reads a chosen ceiling back through a
// second reader, beside the other settings, so the file is what is asserted.
func TestTheCeilingSurvivesBesideTheOthers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prefs := New(dir)
	chosen := 7 * time.Minute
	if err := prefs.SetCloseStrangers(true); err != nil {
		t.Fatalf("SetCloseStrangers: %v", err)
	}
	if err := prefs.SetCeiling(chosen); err != nil {
		t.Fatalf("SetCeiling: %v", err)
	}
	ceiling, set, err := New(dir).Ceiling()
	if err != nil || !set || ceiling != chosen {
		t.Fatalf("the ceiling reads back as %s (set %v, %v), wanted %s", ceiling, set, err, chosen)
	}
	if closing, err := New(dir).CloseStrangers(); err != nil || !closing {
		t.Fatalf("the choice about unnamed windows did not survive the ceiling (%v)", err)
	}
	damaged := t.TempDir()
	if err := os.WriteFile(filepath.Join(damaged, FileName), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("writing a damaged file: %v", err)
	}
	if _, _, err := New(damaged).Ceiling(); err == nil {
		t.Error("a damaged settings file read as no ceiling chosen")
	}
}

// TestAnUnreadableCeilingIsAFault holds the rule for a ceiling edited by hand
// into something that is not a duration: saying so beats quietly waiting the
// default while the file says otherwise.
func TestAnUnreadableCeilingIsAFault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	edited := []byte(`{"ceiling": "a quarter of an hour"}`)
	if err := os.WriteFile(filepath.Join(dir, FileName), edited, 0o644); err != nil {
		t.Fatalf("writing the file: %v", err)
	}
	if _, _, err := New(dir).Ceiling(); err == nil {
		t.Error("a ceiling that is not a duration read as no ceiling chosen")
	}
}

func TestTurningTheCheckBackOnIsRemembered(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prefs := New(dir)

	if err := prefs.SetUpdateCheckEnabled(false); err != nil {
		t.Fatalf("SetUpdateCheckEnabled: %v", err)
	}
	if err := prefs.SetUpdateCheckEnabled(true); err != nil {
		t.Fatalf("SetUpdateCheckEnabled: %v", err)
	}
	enabled, err := New(dir).UpdateCheckEnabled()
	if err != nil {
		t.Fatalf("UpdateCheckEnabled: %v", err)
	}
	if !enabled {
		t.Error("turning the check back on was not remembered")
	}
}

// TestADamagedFileIsAFaultRatherThanADefault is the ruling that matters most
// here. Carrying on over an unreadable file would silently write the defaults
// over whatever the user had set, which is worse than saying so.
func TestADamagedFileIsAFaultRatherThanADefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("writing a damaged file: %v", err)
	}
	prefs := New(dir)

	if _, err := prefs.UpdateCheckEnabled(); err == nil {
		t.Error("a damaged settings file read as the default")
	}
	if _, err := prefs.SkippedVersion(); err == nil {
		t.Error("a damaged settings file read as the default")
	}
	if err := prefs.SetUpdateCheckEnabled(true); err == nil {
		t.Error("a damaged settings file was written over")
	}
}

func TestTheFileIsMadeWhereThereIsNoDirectoryYet(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "not", "there", "yet")
	if err := New(dir).SetSkippedVersion("2.0.0"); err != nil {
		t.Fatalf("SetSkippedVersion: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
		t.Errorf("the settings file was not written: %v", err)
	}
}

// TestDefaultPutsTheSettingsBesideTheProfiles keeps the settings where the
// uninstall screen already offers to clear.
func TestDefaultPutsTheSettingsBesideTheProfiles(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)

	prefs, err := Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if directory := filepath.Dir(prefs.path); filepath.Dir(directory) != base {
		t.Errorf("the settings live in %q, which is not under %q", directory, base)
	}
}
