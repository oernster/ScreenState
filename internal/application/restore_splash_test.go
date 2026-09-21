package application

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// fakeSplash records what a restore told the splash, in order.
type fakeSplash struct {
	mutex sync.Mutex
	told  []string
}

func (splash *fakeSplash) Preparing(message SplashMessage) {
	splash.record("preparing", message)
}

func (splash *fakeSplash) Ready(message SplashMessage) {
	splash.record("ready", message)
}

func (splash *fakeSplash) record(phase string, message SplashMessage) {
	splash.mutex.Lock()
	defer splash.mutex.Unlock()
	splash.told = append(splash.told,
		strings.TrimSpace(phase+": "+message.Headline+" | "+message.Detail))
}

func (splash *fakeSplash) sequence() []string {
	splash.mutex.Lock()
	defer splash.mutex.Unlock()
	return append([]string(nil), splash.told...)
}

// FR-078: the words, including the shortfall said in full English.
func TestTheSplashSaysWhatTheSpecificationSays(t *testing.T) {
	t.Parallel()
	cases := []struct {
		outstanding int
		detail      string
	}{
		{0, ""},
		{1, "1 application did not start"},
		{3, "3 applications did not start"},
	}
	for _, each := range cases {
		message := readyMessage(each.outstanding)
		if message.Headline != "Your desktop is ready" || message.Detail != each.detail {
			t.Errorf("%d outstanding said %+v", each.outstanding, message)
		}
	}
	if preparingMessage().Headline != "Please wait while your desktop is prepared" {
		t.Errorf("the waiting words are %+v", preparingMessage())
	}
}

// FR-078: a sign-in restore puts the splash up as it begins and says ready
// once it has ended, with nothing outstanding.
func TestARestoreSaysItIsPreparingThenReady(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	splash := &fakeSplash{}
	profile := oneEntry(pigeonpost, onPrimary).WithDefault(true)
	service := restoreShowing(splash, desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(profile), newFakeClock(), &fakeLog{}, &fakeStrangers{})

	if _, marked, err := service.RestoreDefault(context.Background()); err != nil || !marked {
		t.Fatalf("restoring at sign-in: %v, marked %v", err, marked)
	}
	want := []string{
		"preparing: Please wait while your desktop is prepared |",
		"ready: Your desktop is ready |",
	}
	if got := splash.sequence(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the splash was told %q", got)
	}
}

// FR-078: an entry left outstanding is said rather than hidden behind "ready".
func TestARestoreWithAShortfallSaysSo(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{displays: []Display{primaryDisplay}}
	launcher := &fakeLauncher{refuse: map[string]error{
		strings.ToLower(stellody.String()): errRefused,
	}}
	splash := &fakeSplash{}
	service := restoreShowing(splash, desktop, newFakeProcesses(nordvpn), launcher,
		newFakeStore(), newFakeClock(), &fakeLog{}, &fakeStrangers{})
	profile, _ := domain.NewProfile("Desk",
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{onPrimary}},
		domain.Entry{Application: nordvpn, Running: true})

	if _, err := service.Restore(context.Background(), profile); err != nil {
		t.Fatalf("the restore failed: %v", err)
	}
	told := splash.sequence()
	if len(told) != 2 || told[1] != "ready: Your desktop is ready | 1 application did not start" {
		t.Fatalf("the splash was told %q", told)
	}
}

// FR-078 with FR-061: a restore a newer request stood down says nothing more,
// so "ready" never flashes up in the middle of the newer restore.
func TestAReplacedRestoreDoesNotSayReady(t *testing.T) {
	t.Parallel()
	desktop := &fakeDesktop{
		displays: []Display{primaryDisplay},
		windows:  []Window{aWindow(1, pigeonpost, at(0))},
	}
	clock := newFakeClock()
	running := make(chan struct{})
	var signalled bool
	// The first restore parks here until a newer one stands it down.
	clock.onSleep = func(ctx context.Context, _ *fakeClock, _ int) {
		if signalled {
			return
		}
		signalled = true
		close(running)
		<-ctx.Done()
	}
	splash := &fakeSplash{}
	service := restoreShowing(splash, desktop, newFakeProcesses(pigeonpost), &fakeLauncher{},
		newFakeStore(), clock, &fakeLog{}, &fakeStrangers{})
	first, _ := domain.NewProfile("Desk",
		domain.Entry{Application: pigeonpost, Running: true, Placements: []domain.Placement{onPrimary}},
		domain.Entry{Application: stellody, Running: true, Placements: []domain.Placement{onPrimary}})
	second := oneEntry(pigeonpost, onPrimary)

	firstDone := make(chan struct{})
	go func() {
		_, _ = service.Restore(context.Background(), first)
		close(firstDone)
	}()
	<-running
	if _, err := service.Restore(context.Background(), second); err != nil {
		t.Fatalf("the replacing restore failed: %v", err)
	}
	<-firstDone

	want := []string{
		"preparing: Please wait while your desktop is prepared |",
		"preparing: Please wait while your desktop is prepared |",
		"ready: Your desktop is ready |",
	}
	if got := splash.sequence(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the splash was told %q", got)
	}
}
