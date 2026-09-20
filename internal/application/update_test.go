package application

import (
	"context"
	"errors"
	"testing"
)

// fakeReleases is a release feed in memory. It answers a nil release for a feed
// that could not be reached, which is the contract the real adapter holds to.
type fakeReleases struct {
	release *Release
	err     error
	asked   int
}

func (feed *fakeReleases) Latest(context.Context) (*Release, error) {
	feed.asked++
	return feed.release, feed.err
}

// fakePrefs is the little the update check remembers, with error injection on
// each half.
type fakePrefs struct {
	enabled     bool
	skipped     string
	enabledErr  error
	skippedErr  error
	setOnErr    error
	setSkipErr  error
	lastSkipSet string
}

func (prefs *fakePrefs) UpdateCheckEnabled() (bool, error) {
	return prefs.enabled, prefs.enabledErr
}

func (prefs *fakePrefs) SetUpdateCheckEnabled(enabled bool) error {
	if prefs.setOnErr != nil {
		return prefs.setOnErr
	}
	prefs.enabled = enabled
	return nil
}

func (prefs *fakePrefs) SkippedVersion() (string, error) {
	return prefs.skipped, prefs.skippedErr
}

func (prefs *fakePrefs) SetSkippedVersion(version string) error {
	if prefs.setSkipErr != nil {
		return prefs.setSkipErr
	}
	prefs.lastSkipSet = version
	prefs.skipped = version
	return nil
}

// updateOver returns an update service over the given feed, switched on, at the
// given running version.
func updateOver(feed *fakeReleases, current string) (*UpdateService, *fakePrefs, *fakeLog) {
	prefs := &fakePrefs{enabled: true}
	log := &fakeLog{}
	return NewUpdateService(feed, prefs, log, current, windowsAsset), prefs, log
}

// aRelease is a published release carrying one download per platform.
func aRelease(version string) *Release {
	return &Release{
		Version: version,
		PageURL: "https://example.invalid/releases/" + version,
		Assets: []ReleaseAsset{
			{Name: "Setup.EXE", DownloadURL: "https://example.invalid/Setup.exe"},
			{Name: "app.dmg", DownloadURL: "https://example.invalid/app.dmg"},
			{Name: "app.flatpak", DownloadURL: "https://example.invalid/app.flatpak"},
		},
	}
}

func TestIsNewerReadsVersionsTheWayATagIsWritten(t *testing.T) {
	t.Parallel()
	cases := []struct {
		released string
		running  string
		expected bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.1.0", "1.0.9", true},
		{"2.0.0", "1.9.9", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.1", false},
		{"v1.0.1", "1.0.0", true},
		{"V1.0.1", "1.0.0", true},
		{"  1.0.1  ", "1.0.0", true},
		{"1.0.1.5", "1.0.0", true},
		{"1.1", "1.0.9", true},
		// A pre-release suffix takes no part, so it is never newer than the
		// release it is a candidate for.
		{"1.0.0-beta", "1.0.0", false},
		{"1.0.0+build9", "1.0.0", false},
		// Anything unreadable compares as not newer, in either position.
		{"not a version", "1.0.0", false},
		{"1.0.0", "not a version", false},
		{"", "1.0.0", false},
		{"1.-2.0", "1.0.0", false},
	}
	for _, testCase := range cases {
		if got := IsNewer(testCase.released, testCase.running); got != testCase.expected {
			t.Errorf("IsNewer(%q, %q) = %t, want %t",
				testCase.released, testCase.running, got, testCase.expected)
		}
	}
}

func TestPlatformKeyNamesTheFileEachMachineCanOpen(t *testing.T) {
	t.Parallel()
	for goos, expected := range map[string]string{
		"windows": windowsAsset,
		"darwin":  macosAsset,
		"linux":   linuxAsset,
		"freebsd": linuxAsset,
		"":        linuxAsset,
	} {
		if got := PlatformKeyFor(goos); got != expected {
			t.Errorf("PlatformKeyFor(%q) = %q, want %q", goos, got, expected)
		}
	}
}

func TestSelectAssetURLPicksBySuffixWithoutRegardToCase(t *testing.T) {
	t.Parallel()
	assets := aRelease("1.0.1").Assets
	if got := SelectAssetURL(assets, windowsAsset); got != "https://example.invalid/Setup.exe" {
		t.Errorf("the Windows download is %q", got)
	}
	if got := SelectAssetURL(assets, linuxAsset); got != "https://example.invalid/app.flatpak" {
		t.Errorf("the Linux download is %q", got)
	}
	if got := SelectAssetURL(nil, windowsAsset); got != "" {
		t.Errorf("an empty release answered %q", got)
	}
	if got := SelectAssetURL(assets, ""); got != "" {
		t.Errorf("an unknown platform answered %q", got)
	}
	if got := SelectAssetURL(assets, ".msi"); got != "" {
		t.Errorf("a suffix nothing carries answered %q", got)
	}
}

// TestACheckThatIsOffAsksNothingOfTheNetwork is FR-059 stated as a shape. The
// count on the feed is the assertion: a status saying nothing would look the
// same whether or not the request went out.
func TestACheckThatIsOffAsksNothingOfTheNetwork(t *testing.T) {
	t.Parallel()
	feed := &fakeReleases{release: aRelease("9.9.9")}
	service, prefs, _ := updateOver(feed, "1.0.0")
	prefs.enabled = false

	status, err := service.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if feed.asked != 0 {
		t.Errorf("the release feed was asked %d times while the check is off", feed.asked)
	}
	if status.Enabled || status.Available || status.Reached {
		t.Errorf("a check that is off answered %+v", status)
	}
}

func TestCheckOffersANewerVersionWithTheFileForThisMachine(t *testing.T) {
	t.Parallel()
	service, _, log := updateOver(&fakeReleases{release: aRelease("1.2.0")}, "1.0.0")

	status, err := service.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !status.Available || !status.Reached || !status.Enabled {
		t.Fatalf("a newer version answered %+v", status)
	}
	if status.Latest != "1.2.0" || status.Current != "1.0.0" {
		t.Errorf("the status names %q against %q", status.Latest, status.Current)
	}
	if status.DownloadURL != "https://example.invalid/Setup.exe" {
		t.Errorf("the download offered is %q", status.DownloadURL)
	}
	if !log.saying("is available") {
		t.Error("the offer was not recorded")
	}
}

func TestCheckFallsBackToTheReleasePageWhereThereIsNoFileForThisMachine(t *testing.T) {
	t.Parallel()
	release := aRelease("1.2.0")
	release.Assets = []ReleaseAsset{{Name: "notes.txt", DownloadURL: "https://example.invalid/notes"}}
	service, _, _ := updateOver(&fakeReleases{release: release}, "1.0.0")

	status, err := service.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if status.DownloadURL != release.PageURL {
		t.Errorf("the offer points at %q, want the release page", status.DownloadURL)
	}
}

func TestCheckSaysNothingWhereThereIsNothingNewer(t *testing.T) {
	t.Parallel()
	service, _, _ := updateOver(&fakeReleases{release: aRelease("1.0.0")}, "1.0.0")

	status, err := service.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if status.Available {
		t.Error("a matching version was offered as an update")
	}
	if !status.Reached {
		t.Error("a feed that answered was reported as unreachable")
	}
}

// TestAnUnreachableFeedIsSilentAndIsNotAFault holds the failure contract: a
// machine that is offline is an ordinary state of the world.
func TestAnUnreachableFeedIsSilentAndIsNotAFault(t *testing.T) {
	t.Parallel()
	service, _, _ := updateOver(&fakeReleases{}, "1.0.0")

	status, err := service.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("an unreachable feed was reported as a fault: %v", err)
	}
	if status.Reached || status.Available {
		t.Errorf("an unreachable feed answered %+v", status)
	}
}

func TestAFeedThatCannotBeReadIsSilentAndRecorded(t *testing.T) {
	t.Parallel()
	service, _, log := updateOver(&fakeReleases{err: errors.New("that is not JSON")}, "1.0.0")

	status, err := service.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("an unreadable feed was reported to the caller: %v", err)
	}
	if status.Reached || status.Available {
		t.Errorf("an unreadable feed answered %+v", status)
	}
	if !log.saying("could not read the release feed") {
		t.Error("nothing was recorded about the feed that could not be read")
	}
}

// TestASkippedVersionIsSilentUnbiddenAndAnsweredWhenAsked is the rule that
// makes skipping mean something: it silences the check that speaks by itself,
// never the one the user pressed a button for.
func TestASkippedVersionIsSilentUnbiddenAndAnsweredWhenAsked(t *testing.T) {
	t.Parallel()
	service, prefs, _ := updateOver(&fakeReleases{release: aRelease("1.2.0")}, "1.0.0")
	prefs.skipped = "1.2.0"

	unbidden, err := service.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if unbidden.Available {
		t.Error("a version that was passed over was offered by itself")
	}
	if !unbidden.Skipped {
		t.Error("the status does not say the version was passed over")
	}

	asked, err := service.Check(context.Background(), false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !asked.Available {
		t.Error("a version that was passed over was withheld from a user who asked")
	}
}

func TestADifferentSkippedVersionStillPrompts(t *testing.T) {
	t.Parallel()
	service, prefs, _ := updateOver(&fakeReleases{release: aRelease("1.3.0")}, "1.0.0")
	prefs.skipped = "1.2.0"

	status, err := service.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !status.Available {
		t.Error("a version nobody passed over was withheld")
	}
}

func TestCheckReportsPreferencesItCannotRead(t *testing.T) {
	t.Parallel()
	t.Run("whether the check is on", func(t *testing.T) {
		t.Parallel()
		service, prefs, _ := updateOver(&fakeReleases{release: aRelease("1.2.0")}, "1.0.0")
		prefs.enabledErr = errors.New("the file is unreadable")
		if _, err := service.Check(context.Background(), true); err == nil {
			t.Fatal("Check answered over preferences it cannot read")
		}
	})
	t.Run("the version passed over", func(t *testing.T) {
		t.Parallel()
		service, prefs, _ := updateOver(&fakeReleases{release: aRelease("1.2.0")}, "1.0.0")
		prefs.skippedErr = errors.New("the file is unreadable")
		if _, err := service.Check(context.Background(), true); err == nil {
			t.Fatal("Check answered over preferences it cannot read")
		}
	})
}

func TestSkipRemembersTheVersionAndSaysSo(t *testing.T) {
	t.Parallel()
	service, prefs, log := updateOver(&fakeReleases{}, "1.0.0")

	if err := service.Skip("1.2.0"); err != nil {
		t.Fatalf("Skip: %v", err)
	}
	if prefs.lastSkipSet != "1.2.0" {
		t.Errorf("the version passed over was written as %q", prefs.lastSkipSet)
	}
	if !log.saying("will not be offered again") {
		t.Error("the skip was not recorded")
	}
}

func TestSkipReportsAStoreThatRefused(t *testing.T) {
	t.Parallel()
	service, prefs, _ := updateOver(&fakeReleases{}, "1.0.0")
	prefs.setSkipErr = errors.New("the disk is full")

	if err := service.Skip("1.2.0"); err == nil {
		t.Fatal("Skip reported success over a store that refused")
	}
}

func TestTurningTheCheckOnAndOffIsRememberedAndRecorded(t *testing.T) {
	t.Parallel()
	service, prefs, log := updateOver(&fakeReleases{}, "1.0.0")

	if err := service.SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if prefs.enabled {
		t.Error("the check is still on")
	}
	if !log.saying("the update check is now false") {
		t.Error("the change was not recorded")
	}
	enabled, err := service.Enabled()
	if err != nil {
		t.Fatalf("Enabled: %v", err)
	}
	if enabled {
		t.Error("Enabled reported on after it was turned off")
	}
}

func TestTurningTheCheckOnReportsAStoreThatRefused(t *testing.T) {
	t.Parallel()
	service, prefs, _ := updateOver(&fakeReleases{}, "1.0.0")
	prefs.setOnErr = errors.New("the disk is full")

	if err := service.SetEnabled(true); err == nil {
		t.Fatal("SetEnabled reported success over a store that refused")
	}
	prefs.enabledErr = errors.New("the file is unreadable")
	if _, err := service.Enabled(); err == nil {
		t.Fatal("Enabled answered over a store it cannot read")
	}
}
