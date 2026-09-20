package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/oernster/ScreenState/internal/product"
)

// MenuKind says what a menu entry is for. The user interface draws each kind in
// whatever way its toolkit draws it; nothing about a menu's contents is decided
// there.
type MenuKind uint8

const (
	// MenuProfile applies the profile it names (FR-041).
	MenuProfile MenuKind = iota
	// MenuSeparator is a dividing line.
	MenuSeparator
	// MenuCapture opens a capture of the desktop as it stands (FR-010).
	MenuCapture
	// MenuManager opens the manager window (EIR-001).
	MenuManager
	// MenuReport opens the report of the most recent restore (FR-045).
	MenuReport
	// MenuQuit ends the agent.
	MenuQuit
	// MenuMessage is a line that says something and does nothing, such as the
	// statement that no profiles have been captured yet.
	MenuMessage
)

// ManagerRequest says which part of the manager to open. The tray asks for one
// and the window obeys; deciding it here rather than in the window means the
// reason a window opened can be asserted by a test.
type ManagerRequest uint8

const (
	// ManagerProfiles opens the profile list, which is the manager at rest.
	ManagerProfiles ManagerRequest = iota
	// ManagerCapture opens the review of a fresh capture, which is where a
	// profile is named and confirmed (FR-010, FR-011).
	ManagerCapture
	// ManagerReport opens the report of the most recent restore (FR-044).
	ManagerReport
)

// MenuItem is one entry in the tray menu.
type MenuItem struct {
	Kind MenuKind
	// Label is what the user reads.
	Label string
	// Profile names the profile a MenuProfile entry applies.
	Profile string
	// Checked marks the profile that is the default (FR-040).
	Checked bool
	// Enabled is false for an entry there is nothing behind, which is shown
	// greyed rather than hidden: an entry that disappears makes a user wonder
	// whether they imagined it.
	Enabled bool
}

// TrayService is what the tray asks: what to put in the menu, what the icon
// should say, what to do when the user chooses something.
//
// It holds no window and no icon. Everything here can be exercised without a
// desktop, which is the point of it not living in the user interface.
type TrayService struct {
	store    ProfileStore
	restores *RestoreService
	captures *CaptureService
	log      Log
}

// NewTrayService returns a tray service over the given use cases.
func NewTrayService(
	store ProfileStore,
	restores *RestoreService,
	captures *CaptureService,
	log Log,
) *TrayService {
	return &TrayService{store: store, restores: restores, captures: captures, log: log}
}

// The words the menu uses. They are here rather than in the user interface so
// that the menu reads the same wherever it is drawn, so that a test can
// name one without reaching into a toolkit.
const (
	managerLabel  = "Profiles and settings..."
	captureLabel  = "Capture the desktop..."
	quitLabel     = "Quit " + product.Name
	noProfiles    = "No profiles yet"
	noReport      = "No restore has run yet"
	reportLabel   = "Report of the last restore"
	defaultSuffix = "  (default)"
)

// Menu returns the tray menu as it should stand now (FR-041, FR-042, FR-045).
//
// A store that cannot be read is a menu that says so rather than a menu with no
// profiles in it: those two look identical to a user and mean opposite things.
func (service *TrayService) Menu(ctx context.Context) []MenuItem {
	items := service.profileItems(ctx)
	items = append(items, MenuItem{Kind: MenuSeparator})
	items = append(items, MenuItem{Kind: MenuManager, Label: managerLabel, Enabled: true})
	items = append(items, MenuItem{Kind: MenuCapture, Label: captureLabel, Enabled: true})
	items = append(items, service.reportItem())
	items = append(items, MenuItem{Kind: MenuSeparator})
	items = append(items, MenuItem{Kind: MenuQuit, Label: quitLabel, Enabled: true})
	return items
}

// profileItems lists the stored profiles, the default one marked.
func (service *TrayService) profileItems(ctx context.Context) []MenuItem {
	names, err := service.store.Names(ctx)
	if err != nil {
		service.log.Step(fmt.Sprintf("the profiles could not be listed: %v", err))
		return []MenuItem{{Kind: MenuMessage, Label: "The profiles could not be read"}}
	}
	if len(names) == 0 {
		return []MenuItem{{Kind: MenuMessage, Label: noProfiles}}
	}
	marked := ""
	if profile, held, err := service.store.Default(ctx); err == nil && held {
		marked = profile.Name
	}
	sort.Slice(names, func(one, two int) bool {
		return strings.ToLower(names[one]) < strings.ToLower(names[two])
	})
	items := make([]MenuItem, 0, len(names))
	for _, name := range names {
		item := MenuItem{Kind: MenuProfile, Label: name, Profile: name, Enabled: true}
		if strings.EqualFold(name, marked) {
			item.Checked = true
			item.Label = name + defaultSuffix
		}
		items = append(items, item)
	}
	return items
}

// reportItem is the entry that opens the report of the most recent restore. It
// carries the count of what was left outstanding, which is how a user sees at a
// glance that something needs looking at (FR-045).
func (service *TrayService) reportItem() MenuItem {
	report, held := service.restores.Last()
	if !held {
		return MenuItem{Kind: MenuReport, Label: noReport}
	}
	_, outstanding := report.Counts()
	if outstanding == 0 {
		return MenuItem{Kind: MenuReport, Label: reportLabel, Enabled: true}
	}
	return MenuItem{
		Kind:    MenuReport,
		Label:   fmt.Sprintf("%s: %d outstanding", reportLabel, outstanding),
		Enabled: true,
	}
}

// Tooltip is what the tray icon says when the pointer rests on it. It carries
// the state of the last restore, so the answer to "did it work" needs no click.
func (service *TrayService) Tooltip() string {
	report, held := service.restores.Last()
	if !held {
		return product.Name
	}
	return product.Name + ": " + report.Summary()
}

// NeedsAttention reports whether the most recent restore left anything
// outstanding, which is what FR-045 marks the tray icon for.
func (service *TrayService) NeedsAttention() bool {
	report, held := service.restores.Last()
	if !held {
		return false
	}
	_, outstanding := report.Counts()
	return outstanding > 0
}

// Apply restores the profile a menu entry names (FR-041).
//
// During a session the windows are already there, so this is the same restore
// the agent runs at sign-in; a restore already running is replaced by it, which
// is FR-061 and is what makes a second choice from the menu do what the user
// plainly meant.
func (service *TrayService) Apply(ctx context.Context, name string) (*Report, error) {
	profile, err := service.store.Load(ctx, name)
	if err != nil {
		service.log.Step(fmt.Sprintf("profile %q could not be read: %v", name, err))
		return nil, err
	}
	return service.restores.Restore(ctx, profile)
}

// Capture reads the desktop for review (FR-010). Nothing is written until the
// user confirms it, so this has no effect of its own.
func (service *TrayService) Capture(ctx context.Context, basedOn string) (Review, error) {
	return service.captures.Review(ctx, basedOn)
}

// Report returns the report of the most recent restore, plus whether there has
// been one.
func (service *TrayService) Report() (*Report, bool) { return service.restores.Last() }
