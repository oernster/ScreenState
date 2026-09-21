package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/oernster/ScreenState/internal/domain"
)

// Review is what a capture offers the user before anything is written: the
// entries it read, plus the windows it could not read.
//
// Nothing is stored until the user confirms it (FR-011); a cancelled review
// writes nothing at all (FR-016). A capture has no effect of its own.
type Review struct {
	// Entries are the candidates, one per application, in the order their
	// oldest window was created.
	Entries []domain.Entry
	// Unreadable names every window omitted because its state could not be
	// read, so the user is told rather than left to notice (FR-014).
	Unreadable []string
}

// CaptureService reads the desktop as it stands and turns it into a profile.
type CaptureService struct {
	desktop Desktop
	store   ProfileStore
	log     Log
	// self is this product's own identity, excluded from every capture so a
	// profile never tries to arrange the agent that is arranging it (FR-013).
	self domain.ApplicationIdentity
}

// NewCaptureService returns a capture service over the given collaborators.
func NewCaptureService(
	desktop Desktop,
	store ProfileStore,
	log Log,
	self domain.ApplicationIdentity,
) *CaptureService {
	return &CaptureService{
		desktop: desktop,
		store:   store,
		log:     log,
		self:    self,
	}
}

// Review reads the desktop and returns the entries for the user to confirm
// (FR-010). Only what is on screen is read (FR-005), so capturing a desktop
// afresh and capturing it again over an existing profile are the same thing.
func (service *CaptureService) Review(ctx context.Context) (Review, error) {
	windows, err := service.desktop.Windows(ctx)
	if err != nil {
		return Review{}, fmt.Errorf("reading the windows: %w", err)
	}
	displays, err := service.desktop.Displays(ctx)
	if err != nil {
		return Review{}, fmt.Errorf("reading the displays: %w", err)
	}
	set, err := newDisplaySet(displays)
	if err != nil {
		return Review{}, err
	}

	review := service.fromWindows(windows, set)
	service.log.Step(fmt.Sprintf("capture read %d entries and %d unreadable windows",
		len(review.Entries), len(review.Unreadable)))
	return review, nil
}

// fromWindows turns the windows now open into one entry per application.
func (service *CaptureService) fromWindows(windows []Window, set displaySet) Review {
	ordered := make([]Window, len(windows))
	copy(ordered, windows)
	sort.SliceStable(ordered, func(one, two int) bool {
		return ordered[one].Created.Before(ordered[two].Created)
	})

	var review Review
	at := make(map[string]int)
	for _, window := range ordered {
		if window.Unreadable != "" {
			review.Unreadable = append(review.Unreadable,
				fmt.Sprintf("%s: %s", service.describe(window), window.Unreadable))
			continue
		}
		if window.Application.Validate() != nil || service.self.SameProgram(window.Application) {
			continue
		}
		// Only what is on screen is captured (FR-005): a window that is not
		// shown has nothing to arrange, so it makes no entry at all.
		if !window.Visible {
			continue
		}
		key := strings.ToLower(window.Application.String())
		position, known := at[key]
		if !known {
			review.Entries = append(review.Entries, domain.Entry{
				Application: window.Application,
				Running:     true,
			})
			position = len(review.Entries) - 1
			at[key] = position
		}
		review.Entries[position] = review.Entries[position].
			WithPlacement(placementFor(window, set))
	}
	return review
}

// describe names a window for a person, falling back to its application where
// the description could not be read and to the window itself where neither was.
func (service *CaptureService) describe(window Window) string {
	if description := strings.TrimSpace(window.Description); description != "" {
		return description
	}
	if window.Application.Validate() == nil {
		return window.Application.String()
	}
	return fmt.Sprintf("window %d", uint64(window.ID))
}

// placementFor records where one window sits: the display holding most of it,
// its normal rectangle and how it is shown.
func placementFor(window Window, set displaySet) domain.Placement {
	placement := domain.Placement{Rect: window.Rect, State: window.State}
	if display, found := set.holding(window.Rect); found {
		placement.Display = display.Identity
		return placement
	}
	// A window sharing area with no connected display is off the visible
	// desktop. It is recorded against the primary display, so that restoring
	// the profile brings it back somewhere reachable (FR-032).
	placement.Display = set.primary().Identity
	return placement
}

// Save writes the confirmed review as a profile (FR-011). A name already in use
// is refused rather than overwritten, unless the caller says to replace it,
// which is how FR-002 states that the name is in use before anything is lost.
func (service *CaptureService) Save(
	ctx context.Context,
	name string,
	entries []domain.Entry,
	replace bool,
) (domain.Profile, error) {
	profile, err := domain.NewProfile(name, entries...)
	if err != nil {
		return domain.Profile{}, err
	}
	if !replace {
		taken, err := service.nameTaken(ctx, profile.Name)
		if err != nil {
			return domain.Profile{}, err
		}
		if taken {
			return domain.Profile{}, fmt.Errorf("%w: %q", ErrProfileNameInUse, profile.Name)
		}
	}
	if err := service.store.Save(ctx, profile); err != nil {
		return domain.Profile{}, fmt.Errorf("writing profile %q: %w", profile.Name, err)
	}
	service.log.Step(fmt.Sprintf("capture wrote profile %q with %d entries",
		profile.Name, len(profile.Entries)))
	return service.markFirstAsDefault(ctx, profile)
}

// markFirstAsDefault marks a profile as the one applied at sign-in when no
// profile is marked at all (FR-039, FR-040).
//
// Capturing a desktop and signing in to find nothing arranged is the product
// appearing not to work: measured on 2026-09-20, where the only profile stored
// was unmarked, so every start said nothing was restored and the user had to
// press Apply by hand. A profile the user has just made is the only candidate
// there is, so it is marked rather than left for a step nobody knew about. It
// only ever fills an empty marking: a second capture never takes the marking
// off the profile the user chose.
func (service *CaptureService) markFirstAsDefault(
	ctx context.Context,
	profile domain.Profile,
) (domain.Profile, error) {
	if profile.Default {
		return profile, nil
	}
	if _, marked, err := service.store.Default(ctx); err != nil || marked {
		// A store that cannot say leaves the marking alone: the profile is
		// written, which is what was asked for; the report says the rest.
		if err != nil {
			service.log.Step(fmt.Sprintf(
				"could not tell whether a profile is marked as the default: %v", err))
		}
		return profile, nil
	}
	marked := profile.WithDefault(true)
	if err := service.store.Save(ctx, marked); err != nil {
		service.log.Step(fmt.Sprintf("could not mark %q as the default: %v", profile.Name, err))
		return profile, nil
	}
	service.log.Step(fmt.Sprintf(
		"%q is the only profile, so it is marked as the default and is applied at sign-in",
		marked.Name))
	return marked, nil
}

// nameTaken reports whether the store already holds a profile of that name,
// ignoring case, since two profiles differing only in case would be one name to
// the user (FR-002).
func (service *CaptureService) nameTaken(ctx context.Context, name string) (bool, error) {
	names, err := service.store.Names(ctx)
	if err != nil {
		return false, fmt.Errorf("listing the profiles: %w", err)
	}
	for _, stored := range names {
		if strings.EqualFold(stored, name) {
			return true, nil
		}
	}
	return false, nil
}
