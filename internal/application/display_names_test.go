package application

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// screen returns a display with the given id and bounds.
func screen(id string, x, y, width, height int32, primary bool) Display {
	return Display{
		Identity: domain.DisplayIdentity{MonitorID: id},
		Bounds:   domain.Rect{X: x, Y: y, Width: width, Height: height},
		Primary:  primary,
	}
}

// The reference machine's four displays. Each is its maximised window from
// appendix E less the border a maximised window overhangs by: 8 pixels at 96
// dpi, 16 at 240. The monitor ids are appendix E's.
var referenceDisplays = []Display{
	screen("DISPLAY#GSM784F#5&14514d51&0&UID4352", 0, 0, 3440, 1392, true),
	screen("DISPLAY#HSJB30F#5&14514d51&0&UID4357", -223, 1440, 3840, 2280, false),
	screen("DISPLAY#HSJ1340#5&14514d51&0&UID4356", -4063, 1423, 3840, 2280, false),
	screen("DISPLAY#HSJ1340#5&14514d51&0&UID4354", 3617, 1423, 3840, 2280, false),
}

func TestDisplaysAreNamedByWhereTheySit(t *testing.T) {
	t.Parallel()
	for label, test := range map[string]struct {
		displays []Display
		want     []string
	}{
		"the reference machine: one above a row of three": {
			displays: referenceDisplays,
			want:     []string{"top display", "centre display", "left display", "right display"},
		},
		"a single display": {
			displays: []Display{screen("A", 0, 0, 1920, 1080, true)},
			want:     []string{"only display"},
		},
		"two side by side, listed right first": {
			displays: []Display{screen("R", 1920, 0, 1920, 1080, true), screen("L", 0, 0, 1920, 1080, false)},
			want:     []string{"right display", "left display"},
		},
		"two stacked": {
			displays: []Display{screen("B", 0, 1080, 1920, 1080, true), screen("T", 0, 0, 1920, 1080, false)},
			want:     []string{"bottom display", "top display"},
		},
		"three stacked": {
			displays: []Display{
				screen("T", 0, 0, 100, 100, false), screen("M", 0, 100, 100, 100, true),
				screen("B", 0, 200, 100, 100, false),
			},
			want: []string{"top display", "middle display", "bottom display"},
		},
		"four stacked": {
			displays: []Display{
				screen("1", 0, 0, 100, 100, true), screen("2", 0, 100, 100, 100, false),
				screen("3", 0, 200, 100, 100, false), screen("4", 0, 300, 100, 100, false),
			},
			want: []string{"top display", "2nd from top display", "3rd from top display", "bottom display"},
		},
		"a row of four": {
			displays: []Display{
				screen("1", 0, 0, 100, 100, true), screen("2", 100, 0, 100, 100, false),
				screen("3", 200, 0, 100, 100, false), screen("4", 300, 0, 100, 100, false),
			},
			want: []string{"left display", "centre left display", "centre right display", "right display"},
		},
		"a row of five": {
			displays: []Display{
				screen("1", 0, 0, 100, 100, true), screen("2", 100, 0, 100, 100, false),
				screen("3", 200, 0, 100, 100, false), screen("4", 300, 0, 100, 100, false),
				screen("5", 400, 0, 100, 100, false),
			},
			want: []string{"left display", "2nd from left display", "3rd from left display",
				"4th from left display", "right display"},
		},
		"rows above and below a main row, the outer ones in pairs": {
			displays: []Display{
				screen("TL", 0, 0, 100, 100, false), screen("TR", 100, 0, 100, 100, false),
				screen("U", 0, 100, 100, 100, false),
				screen("ML", 0, 200, 100, 100, true), screen("MC", 100, 200, 100, 100, false),
				screen("MR", 200, 200, 100, 100, false),
				screen("L", 0, 300, 100, 100, false),
				screen("B", 0, 400, 100, 100, false),
			},
			want: []string{"top left display", "top right display", "upper display",
				"left display", "centre display", "right display", "lower display", "bottom display"},
		},
		"two rows as full as each other: the primary's row is the main one": {
			displays: []Display{
				screen("TL", 0, 0, 100, 100, false), screen("TR", 100, 0, 100, 100, false),
				screen("BL", 0, 100, 100, 100, true), screen("BR", 100, 100, 100, 100, false),
			},
			want: []string{"top left display", "top right display", "left display", "right display"},
		},
		"two displays in the same place are told apart by their ids": {
			displays: []Display{screen("B", 0, 0, 100, 100, true), screen("A", 0, 0, 100, 100, false)},
			want:     []string{"right display", "left display"},
		},
	} {
		names := displayNames(test.displays)
		for at, display := range test.displays {
			if got := names[displayKey(display.Identity)]; got != test.want[at] {
				t.Errorf("%s: %s is named %q, want %q", label, display.Identity, got, test.want[at])
			}
		}
	}
}

func TestNoDisplaysNamesNothing(t *testing.T) {
	t.Parallel()
	if names := displayNames(nil); len(names) != 0 {
		t.Errorf("no displays named %v", names)
	}
}

func TestOrdinalsEndAsTheyAreSpoken(t *testing.T) {
	t.Parallel()
	for number, want := range map[int]string{
		1: "st", 2: "nd", 3: "rd", 4: "th", 11: "th", 12: "th", 13: "th", 21: "st", 22: "nd", 111: "th",
	} {
		if got := ordinalEnd(number); got != want {
			t.Errorf("%d ends %q, want %q", number, got, want)
		}
	}
}

func TestAPlacementIsNamedFromOneReadingOfTheDisplays(t *testing.T) {
	t.Parallel()
	log := &fakeLog{}
	namer := readDisplayNames(context.Background(), &fakeDesktop{displays: referenceDisplays}, log)
	if got := namer.name(referenceDisplays[2].Identity); got != "left display" {
		t.Errorf("the left display is named %q", got)
	}
	// A monitor id matches whatever its case, as it does everywhere else.
	lower := domain.DisplayIdentity{MonitorID: "display#hsj1340#5&14514d51&0&uid4354"}
	if got := namer.name(lower); got != "right display" {
		t.Errorf("the right display in lower case is named %q", got)
	}
	gone := domain.DisplayIdentity{MonitorID: "DISPLAY#GONE"}
	if got := namer.name(gone); got != displayGone {
		t.Errorf("a display that is not connected is named %q, want %q", got, displayGone)
	}
}

func TestDisplaysThatCannotBeReadCostTheNamesAndAreSaid(t *testing.T) {
	t.Parallel()
	log := &fakeLog{}
	desktop := &fakeDesktop{displaysErr: errors.New("the monitors are asleep")}
	namer := readDisplayNames(context.Background(), desktop, log)
	if got := namer.name(primaryID); got != displayUnread {
		t.Errorf("a display that could not be read is named %q, want %q", got, displayUnread)
	}
	if !log.saying("the monitors are asleep") {
		t.Error("the reason the displays could not be read was not logged")
	}
}
