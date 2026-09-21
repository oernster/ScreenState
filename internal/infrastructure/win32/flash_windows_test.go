//go:build windows

package win32

import (
	"testing"
	"time"
)

// FR-064, FR-079: a taskbar button added or taken away wakes a restore. The
// sign-in restore waiting for Spotify, GameGlass Hub and NordVPN to close sat
// for 34 seconds after all three had gone to the notification area, because
// the shell's word that their buttons had gone was heard and dropped.
func TestATaskbarButtonComingOrGoingWakesTheWatch(t *testing.T) {
	if shellHookMessage() == 0 {
		t.Skip("Windows would not name the shell's messages")
	}
	for name, code := range map[string]uintptr{
		"a button taken away": hshellWindowDestroyed,
		"a button added":      hshellWindowCreated,
	} {
		watch := &desktopWatch{changed: make(chan struct{}, 1), touched: make(chan struct{}),
			flash: make(chan struct{}, 1)}
		activeWatchesLock.Lock()
		activeWatches[watch] = struct{}{}
		activeWatchesLock.Unlock()
		heard := onShellMessage(shellHookMessage(), code, 0)
		activeWatchesLock.Lock()
		delete(activeWatches, watch)
		activeWatchesLock.Unlock()
		if !heard {
			t.Errorf("%s was not taken as the shell's", name)
		}
		select {
		case <-watch.changed:
		default:
			t.Errorf("%s did not wake the watch", name)
		}
	}
}

// FR-080: the series is the flash count times two blinks, with Windows'
// default blink where the caret does not blink or its time is unknown.
func TestTheFlashSeriesIsReadFromTheSettings(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name         string
		count, blink uint32
		want         time.Duration
	}{
		{"the reference machine", 7, 530, 7420 * time.Millisecond},
		{"another machine's settings", 3, 400, 2400 * time.Millisecond},
		{"a caret that never blinks", 7, caretNeverBlinks, 7 * blinksPerFlash * defaultCaretBlink},
		{"a blink time that cannot be read", 7, 0, 7 * blinksPerFlash * defaultCaretBlink},
		{"no flashes at all", 0, 530, 0},
	} {
		if got := flashSeries(c.count, c.blink); got != c.want {
			t.Errorf("%s: %s, wanted %s", c.name, got, c.want)
		}
	}
}

// The series read from this machine is whatever its settings say; it is
// never negative.
func TestTheFlashSeriesOfThisMachineCanBeRead(t *testing.T) {
	t.Parallel()
	if series := (&Events{}).FlashSeries(); series < 0 {
		t.Fatalf("the series read %s", series)
	}
}
