package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ReleaseAsset is one downloadable file attached to a release.
type ReleaseAsset struct {
	Name        string
	DownloadURL string
}

// Release is the newest published release as the feed describes it.
type Release struct {
	Version string
	PageURL string
	Assets  []ReleaseAsset
}

// ReleaseSource answers what the newest published release is.
//
// It reports no release and no error where the feed cannot be reached, which is
// the whole of the failure contract: a machine that is offline, behind a proxy
// or simply refused is an ordinary state of the world rather than a fault, so
// the user hears nothing about it. An error is reserved for a feed that
// answered something this product cannot read.
type ReleaseSource interface {
	Latest(ctx context.Context) (*Release, error)
}

// UpdatePreferences is the small amount the update check remembers between
// runs: whether the user wants it at all (FR-059) and which version they have
// chosen to pass over (FR-058).
type UpdatePreferences interface {
	UpdateCheckEnabled() (bool, error)
	SetUpdateCheckEnabled(enabled bool) error
	SkippedVersion() (string, error)
	SetSkippedVersion(version string) error
}

// UpdateStatus is what one check found.
type UpdateStatus struct {
	// Enabled is false where the user has turned the check off, in which case
	// nothing was asked of the network at all (FR-059).
	Enabled bool
	// Reached is false where the feed could not be reached. It is told apart
	// from "no update" because those two mean opposite things to a user who
	// asked the question themselves.
	Reached bool
	Current string
	Latest  string
	// Available is true only where a newer version was found AND the user has
	// not chosen to pass over that exact version.
	Available bool
	// Skipped is true where a newer version was found and passed over. The
	// manual check reports it as up to date rather than silently, so the answer
	// to "why does it not tell me" is on screen.
	Skipped     bool
	DownloadURL string
	PageURL     string
}

// The file suffix that names each platform's download, so the offer hands the
// user the file their machine can actually open.
const (
	windowsAsset = ".exe"
	macosAsset   = ".dmg"
	linuxAsset   = ".flatpak"
)

// PlatformKeyFor maps the Go runtime's name for an operating system onto the
// suffix its download carries. It is pure, so the table is a test rather than a
// thing to be read off a machine.
func PlatformKeyFor(goos string) string {
	switch goos {
	case "windows":
		return windowsAsset
	case "darwin":
		return macosAsset
	default:
		return linuxAsset
	}
}

// UpdateService asks the feed what the newest version is and decides whether to
// say anything about it (FR-058).
type UpdateService struct {
	source   ReleaseSource
	prefs    UpdatePreferences
	log      Log
	current  string
	platform string
}

// NewUpdateService returns an update service over the given collaborators.
func NewUpdateService(
	source ReleaseSource,
	prefs UpdatePreferences,
	log Log,
	current string,
	platform string,
) *UpdateService {
	return &UpdateService{
		source:   source,
		prefs:    prefs,
		log:      log,
		current:  current,
		platform: platform,
	}
}

// Enabled reports whether the update check is switched on (FR-059).
func (service *UpdateService) Enabled() (bool, error) {
	enabled, err := service.prefs.UpdateCheckEnabled()
	if err != nil {
		return false, fmt.Errorf("reading whether the update check is on: %w", err)
	}
	return enabled, nil
}

// SetEnabled turns the update check on or off (FR-059).
func (service *UpdateService) SetEnabled(enabled bool) error {
	if err := service.prefs.SetUpdateCheckEnabled(enabled); err != nil {
		return fmt.Errorf("writing whether the update check is on: %w", err)
	}
	service.log.Step(fmt.Sprintf("the update check is now %t", enabled))
	return nil
}

// Check asks the feed what the newest version is.
//
// honourSkip is true for the check that speaks unbidden and false for one the
// user asked for: skipping exists to silence a check that speaks by itself, so
// a user who presses the button is entitled to the answer either way.
//
// It makes no network connection at all where the check is off, which is what
// FR-059 promises. That test comes first, before the source is touched.
func (service *UpdateService) Check(ctx context.Context, honourSkip bool) (UpdateStatus, error) {
	status := UpdateStatus{Current: service.current}
	enabled, err := service.Enabled()
	if err != nil {
		return status, err
	}
	if !enabled {
		return status, nil
	}
	status.Enabled = true

	release, err := service.source.Latest(ctx)
	if err != nil {
		// The feed answered something unreadable. It is worth a line in the log
		// and worth nothing on screen: a user did not ask for a report on
		// somebody else's JSON.
		service.log.Step(fmt.Sprintf("the update check could not read the release feed: %v", err))
		return status, nil
	}
	if release == nil {
		return status, nil
	}
	status.Reached = true
	status.Latest = release.Version
	status.PageURL = release.PageURL
	status.DownloadURL = SelectAssetURL(release.Assets, service.platform)
	if status.DownloadURL == "" {
		// No file for this machine is not a reason to say nothing: the release
		// page is always somewhere the user can go.
		status.DownloadURL = release.PageURL
	}
	if !IsNewer(release.Version, service.current) {
		return status, nil
	}

	skipped, err := service.prefs.SkippedVersion()
	if err != nil {
		return status, fmt.Errorf("reading the version that was passed over: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(skipped), strings.TrimSpace(release.Version)) {
		status.Skipped = true
		if honourSkip {
			return status, nil
		}
	}
	status.Available = true
	service.log.Step(fmt.Sprintf("version %s is available; this is %s",
		release.Version, service.current))
	return status, nil
}

// Skip records a version the user does not want to hear about again (FR-058).
func (service *UpdateService) Skip(version string) error {
	if err := service.prefs.SetSkippedVersion(version); err != nil {
		return fmt.Errorf("writing the version to pass over: %w", err)
	}
	service.log.Step(fmt.Sprintf("version %s will not be offered again", version))
	return nil
}

// SelectAssetURL returns the download whose name ends in the platform's suffix,
// compared without regard to case. It answers an empty string where none does.
func SelectAssetURL(assets []ReleaseAsset, suffix string) string {
	if suffix == "" {
		return ""
	}
	for _, asset := range assets {
		if strings.HasSuffix(strings.ToLower(asset.Name), strings.ToLower(suffix)) {
			return asset.DownloadURL
		}
	}
	return ""
}

// versionFields is the count of numeric fields compared in a version.
const versionFields = 3

// IsNewer reports whether the released version is later than the running one.
//
// Anything it cannot read compares as NOT newer, so a malformed tag can never
// raise a prompt. That direction is deliberate: the cost of a missed offer is a
// user who updates a day later, while the cost of a spurious one is a user who
// stops believing the ones that are real.
func IsNewer(released, running string) bool {
	left, leftOK := versionOf(released)
	right, rightOK := versionOf(running)
	if !leftOK || !rightOK {
		return false
	}
	for i := range versionFields {
		if left[i] != right[i] {
			return left[i] > right[i]
		}
	}
	return false
}

// versionOf reads a version into its numeric fields, reporting whether it could
// be read at all. A leading v is dropped, in either case, since that is how a
// tag is usually written; a pre-release suffix takes no part, because a suffix
// cannot make a release newer in a way this product acts on.
func versionOf(version string) ([versionFields]int, bool) {
	var out [versionFields]int
	core := strings.TrimSpace(version)
	core = strings.TrimPrefix(strings.TrimPrefix(core, "v"), "V")
	core = strings.SplitN(core, "-", 2)[0]
	core = strings.SplitN(core, "+", 2)[0]
	if core == "" {
		return out, false
	}
	for i, part := range strings.Split(core, ".") {
		if i >= versionFields {
			break
		}
		number, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || number < 0 {
			return out, false
		}
		out[i] = number
	}
	return out, true
}
