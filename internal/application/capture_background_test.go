package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// FR-005: an application running with every window hidden is offered apart from
// the entries, running with no placement, so the review can show it unticked.
// Anything the review already holds is not offered twice, nor is this product,
// nor an identity that names nothing; each application is offered once.
func TestAnApplicationWithOnlyHiddenWindowsIsOfferedApart(t *testing.T) {
	t.Parallel()
	gameglass := domain.ApplicationIdentity{Value: `C:\Programs\GameGlass\GameGlass Hub.exe`}
	desktop := &fakeDesktop{
		displays:   []Display{primaryDisplay},
		windows:    []Window{aWindow(1, pigeonpost, at(0))},
		background: []domain.ApplicationIdentity{nordvpn, pigeonpost, screenst, {}, gameglass, nordvpn},
	}
	log := &fakeLog{}
	service := NewCaptureService(desktop, newFakeProcesses(), newFakeStore(), log, screenst)

	review, err := service.Review(context.Background(), "")
	if err != nil {
		t.Fatalf("the capture failed: %v", err)
	}
	if len(review.Entries) != 1 || !review.Entries[0].Application.Equal(pigeonpost) {
		t.Fatalf("the entries became %+v", review.Entries)
	}
	if len(review.Background) != 2 {
		t.Fatalf("offered %d applications, wanted GameGlass and NordVPN: %+v",
			len(review.Background), review.Background)
	}
	if !review.Background[0].Application.Equal(gameglass) || !review.Background[1].Application.Equal(nordvpn) {
		t.Errorf("the offer is not in name order: %+v", review.Background)
	}
	for _, offered := range review.Background {
		if !offered.Running || len(offered.Placements) != 0 {
			t.Errorf("%s was offered as %+v rather than running with no placement",
				offered.Application, offered)
		}
	}
	if !log.saying("2 applications running with no window shown") {
		t.Error("the log does not say how many were offered")
	}
}

// Recapturing a profile already carries its applications that have no window
// (FR-012), so the offer leaves those out rather than listing them twice.
func TestAnApplicationTheRecaptureAlreadyKeepsIsNotOffered(t *testing.T) {
	t.Parallel()
	profile, err := domain.NewProfile("Desk", domain.Entry{Application: nordvpn, Running: true})
	if err != nil {
		t.Fatalf("building the profile: %v", err)
	}
	store := newFakeStore(profile)
	desktop := &fakeDesktop{
		displays:   []Display{primaryDisplay},
		background: []domain.ApplicationIdentity{nordvpn},
	}
	service := captureUnder(desktop, newFakeProcesses(nordvpn), store)

	review, err := service.Review(context.Background(), "Desk")
	if err != nil {
		t.Fatalf("the capture failed: %v", err)
	}
	if len(review.Entries) != 1 || len(review.Background) != 0 {
		t.Fatalf("NordVPN came back as %d entries and %d offers", len(review.Entries), len(review.Background))
	}
}

// The offer is extra, so a desktop that cannot say what runs in the background
// costs the offer and not the capture; the log says why.
func TestACaptureGoesAheadWhenTheBackgroundCannotBeRead(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays:      []Display{primaryDisplay},
		windows:       []Window{aWindow(1, pigeonpost, at(0))},
		backgroundErr: errors.New("access is denied"),
	}
	log := &fakeLog{}
	service := NewCaptureService(desktop, newFakeProcesses(), newFakeStore(), log, screenst)

	review, err := service.Review(context.Background(), "")
	if err != nil {
		t.Fatalf("the capture failed over its offer: %v", err)
	}
	if len(review.Entries) != 1 || len(review.Background) != 0 {
		t.Fatalf("the review became %+v", review)
	}
	if !log.saying("could not be read, so none are offered") {
		t.Error("the log does not say the offer was lost")
	}
}
