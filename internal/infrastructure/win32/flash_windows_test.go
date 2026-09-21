//go:build windows

package win32

import (
	"testing"
	"time"
)

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
