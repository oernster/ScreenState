package application

import (
	"context"
	"testing"
)

// S-10: a link the release feed names is opened in the user's browser, so a
// scheme that is not https never reaches the page. The release is treated as a
// feed that could not be read: logged, never offered.
func TestALinkThatIsNotHTTPSIsNeverOffered(t *testing.T) {
	t.Parallel()
	for label, change := range map[string]func(*Release){
		"a download":     func(release *Release) { release.Assets[0].DownloadURL = "search-ms:query=x" },
		"a release page": func(release *Release) { release.PageURL = "file:///C:/Windows/System32/calc.exe" },
		"plain http":     func(release *Release) { release.Assets[0].DownloadURL = "http://example.invalid/Setup.exe" },
		"no host":        func(release *Release) { release.PageURL = "https:///releases" },
	} {
		release := aRelease("99.0.0")
		change(release)
		service, _, log := updateOver(&fakeReleases{release: release}, "1.0.0")
		status, err := service.Check(context.Background(), false)
		if err != nil {
			t.Fatalf("%s: checking: %v", label, err)
		}
		if status.Available || status.Reached || status.DownloadURL != "" || status.PageURL != "" {
			t.Errorf("%s: the page was handed %+v", label, status)
		}
		if !log.saying("not https") {
			t.Errorf("%s: the log does not say why the release was passed over", label)
		}
	}
	service, _, _ := updateOver(&fakeReleases{release: aRelease("99.0.0")}, "1.0.0")
	if status, err := service.Check(context.Background(), false); err != nil || !status.Available {
		t.Fatalf("a release with https links was not offered: %+v (%v)", status, err)
	}
}
