package settings

import (
	"os"
	"path/filepath"
	"testing"
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
