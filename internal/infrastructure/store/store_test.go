package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
)

// recordingLog keeps what the store said, so a test can check that a file left
// out of the listing was also explained rather than silently dropped.
type recordingLog struct {
	mutex sync.Mutex
	steps []string
}

func (log *recordingLog) Step(message string) {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	log.steps = append(log.steps, message)
}

func (log *recordingLog) saying(fragment string) bool {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	for _, step := range log.steps {
		if strings.Contains(step, fragment) {
			return true
		}
	}
	return false
}

// storeUnder returns a store over a fresh directory, with the log it wrote to.
func storeUnder(t *testing.T) (*Store, *recordingLog) {
	t.Helper()
	log := &recordingLog{}
	store, err := New(filepath.Join(t.TempDir(), "profiles"), log)
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}
	return store, log
}

// deskProfile is the worked example of section 5, reduced to the three identity
// kinds and the three show states, so a round trip exercises all of them.
func deskProfile(t *testing.T) domain.Profile {
	t.Helper()
	claude, _ := domain.NewApplicationIdentity(domain.KindAppUserModelID, "Claude_pzs8sxrjxfjjc!Claude")
	discord, _ := domain.NewApplicationIdentity(domain.KindUpdaterCommand,
		`C:\Discord\Update.exe --processStart Discord.exe`)
	nordvpn, _ := domain.NewApplicationIdentity(domain.KindPath, `C:\Programs\NordVPN\NordVPN.exe`)
	left, _ := domain.NewDisplayIdentity("DISPLAY#HSJ1340#5&14514d51&0&UID4356")
	primary, _ := domain.NewDisplayIdentity("DISPLAY#GSM784F#5&14514d51&0&UID4352")

	profile, err := domain.NewProfile("Desk",
		domain.Entry{Application: claude, Running: true, Placements: []domain.Placement{{
			Display: primary,
			Rect:    domain.Rect{X: 0, Y: 0, Width: 3440, Height: 1392},
			State:   domain.ShowMaximised,
		}}},
		domain.Entry{Application: discord, Running: true, Placements: []domain.Placement{{
			Display: left,
			Rect:    domain.Rect{X: -3840, Y: 0, Width: 1200, Height: 800},
			State:   domain.ShowMinimised,
		}, {
			Display: left,
			Rect:    domain.Rect{X: -2000, Y: 100, Width: 900, Height: 700},
			State:   domain.ShowNormal,
		}}},
		domain.Entry{Application: nordvpn, Running: true},
	)
	if err != nil {
		t.Fatalf("the profile is not valid: %v", err)
	}
	return profile
}

func TestAProfileComesBackExactlyAsItWentIn(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()
	written := deskProfile(t)

	if err := store.Save(ctx, written); err != nil {
		t.Fatalf("saving: %v", err)
	}
	read, err := store.Load(ctx, "Desk")
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if read.Name != written.Name || len(read.Entries) != len(written.Entries) {
		t.Fatalf("read back as %+v", read)
	}
	for at, entry := range written.Entries {
		got := read.Entries[at]
		if !got.Application.Equal(entry.Application) || got.Running != entry.Running {
			t.Fatalf("entry %d read back as %+v", at, got)
		}
		if len(got.Placements) != len(entry.Placements) {
			t.Fatalf("entry %d has %d placements", at, len(got.Placements))
		}
		for index, placement := range entry.Placements {
			if got.Placements[index] != placement {
				t.Fatalf("entry %d placement %d read back as %+v",
					at, index, got.Placements[index])
			}
		}
	}
}

// The user reads a profile by its name, so the case they typed is kept while
// two names differing only in case remain one profile.
func TestAProfileIsOneProfileWhateverTheCase(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	if err := store.Save(ctx, deskProfile(t)); err != nil {
		t.Fatalf("saving: %v", err)
	}
	for _, spelling := range []string{"Desk", "desk", "DESK", "  Desk  "} {
		read, err := store.Load(ctx, spelling)
		if err != nil {
			t.Fatalf("loading %q: %v", spelling, err)
		}
		if read.Name != "Desk" {
			t.Fatalf("loading %q gave the name %q", spelling, read.Name)
		}
	}
	names, err := store.Names(ctx)
	if err != nil || len(names) != 1 {
		t.Fatalf("the store lists %v (%v)", names, err)
	}
}

// A name is the user's to choose, so it may hold anything they can type.
func TestAProfileNameMayHoldAnything(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	for _, name := range []string{`Desk: 4k/2`, `..`, `con`, `Réunion`, `a b\c*d?e`, "ключ"} {
		profile, err := domain.NewProfile(name,
			domain.Entry{Application: domain.ApplicationIdentity{Value: `C:\a.exe`}, Running: true})
		if err != nil {
			t.Fatalf("%q is not a valid profile name: %v", name, err)
		}
		if err := store.Save(ctx, profile); err != nil {
			t.Fatalf("saving %q: %v", name, err)
		}
		read, err := store.Load(ctx, name)
		if err != nil {
			t.Fatalf("loading %q: %v", name, err)
		}
		if read.Name != name {
			t.Fatalf("%q came back as %q", name, read.Name)
		}
	}
	names, err := store.Names(ctx)
	if err != nil || len(names) != 6 {
		t.Fatalf("the store lists %d profiles: %v (%v)", len(names), names, err)
	}
}

func TestNamesAreListedInASettledOrder(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	for _, name := range []string{"sofa", "Desk", "away"} {
		profile, _ := domain.NewProfile(name,
			domain.Entry{Application: domain.ApplicationIdentity{Value: `C:\a.exe`}, Running: true})
		if err := store.Save(ctx, profile); err != nil {
			t.Fatalf("saving %q: %v", name, err)
		}
	}
	names, err := store.Names(ctx)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(names) != 3 || names[0] != "away" || names[1] != "Desk" || names[2] != "sofa" {
		t.Fatalf("listed as %v", names)
	}
}

func TestAProfileThatIsNotThereIsSaidSo(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	if _, err := store.Load(ctx, "Desk"); !errors.Is(err, application.ErrNoSuchProfile) {
		t.Fatalf("loading answered %v", err)
	}
	if err := store.Delete(ctx, "Desk"); !errors.Is(err, application.ErrNoSuchProfile) {
		t.Fatalf("deleting answered %v", err)
	}
}

func TestADeletedProfileIsGone(t *testing.T) {
	t.Parallel()
	store, _ := storeUnder(t)
	ctx := context.Background()

	if err := store.Save(ctx, deskProfile(t)); err != nil {
		t.Fatalf("saving: %v", err)
	}
	if err := store.Delete(ctx, "desk"); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	if names, _ := store.Names(ctx); len(names) != 0 {
		t.Fatalf("the store still lists %v", names)
	}
}
