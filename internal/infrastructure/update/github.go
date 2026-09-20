// Package update asks GitHub what the newest published release is (FR-058).
//
// It is the only outbound connection this product makes, which is what C-4
// names: an anonymous request carrying no identifier and nothing about the
// desktop, asking a public feed one question.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/product"
)

// endpoint is the feed. It answers ONLY a published release that is neither a
// draft nor a pre-release, which is the guard rather than anything checked
// here: a tag pushed mid-development is invisible to it, so work in progress
// can never raise a prompt.
const endpoint = "https://api.github.com/repos/oernster/" + product.Name +
	"/releases/latest"

// accept asks GitHub for the documented shape of its answer rather than
// whichever one it happens to default to.
const accept = "application/vnd.github+json"

// timeout bounds the request. A check nobody asked for must not hold anything
// up; a feed that has not answered in five seconds is one to ask again on the
// next run.
const timeout = 5 * time.Second

// Source reads the newest release from GitHub.
type Source struct {
	client *http.Client
	url    string
}

// New returns a source over the real feed.
func New() *Source {
	return &Source{client: &http.Client{Timeout: timeout}, url: endpoint}
}

// payload is the part of GitHub's answer this product reads. Everything else it
// sends is ignored rather than rejected: a feed that grows a field is not a
// feed this product has stopped understanding.
type payload struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Latest asks the feed what the newest release is.
//
// A feed that cannot be reached yields no release and no error. So does one
// answering anything other than success. That is the failure contract: a machine that
// is offline, behind a proxy or simply refused is an ordinary state of the
// world; the user hears nothing about it. An error is kept for an answer
// that arrived and could not be read, which is worth a line in the log.
func (source *Source) Latest(ctx context.Context) (*application.Release, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.url, nil)
	if err != nil {
		return nil, fmt.Errorf("preparing the update check: %w", err)
	}
	request.Header.Set("Accept", accept)

	response, err := source.client.Do(request)
	if err != nil {
		return nil, nil
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, nil
	}

	var held payload
	if err := json.NewDecoder(response.Body).Decode(&held); err != nil {
		return nil, fmt.Errorf("reading the release feed: %w", err)
	}
	if held.TagName == "" {
		return nil, fmt.Errorf("the release feed named no version")
	}

	release := &application.Release{Version: held.TagName, PageURL: held.HTMLURL}
	for _, asset := range held.Assets {
		if asset.Name == "" || asset.URL == "" {
			// A download with no name or nowhere to go cannot be offered, so it
			// is dropped rather than carried as far as the button.
			continue
		}
		release.Assets = append(release.Assets, application.ReleaseAsset{
			Name:        asset.Name,
			DownloadURL: asset.URL,
		})
	}
	return release, nil
}
