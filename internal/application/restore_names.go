package application

import (
	"fmt"
	"sort"

	"github.com/oernster/ScreenState/internal/domain"
)

// legendFormat is one line of a legend: a position, then the monitor id.
const legendFormat = "%s is %s"

// arrivedSuffix marks a display that was connected part way through a restore
// at a position another display already held in it.
const arrivedSuffix = " (connected during the restore)"

// stableNames is what one restore calls each display, fixed the first time the
// restore sees it.
//
// Positions are worked out afresh from every reading, so a display coming or
// going moves the others' positions: after the left display goes, the centre
// one reads as left. A report whose notes were written under the old names
// with a legend under the new ones would pair "left display" with a monitor
// the notes never meant. So a restore never renames. A display keeps the name
// it had in the first reading it was in; one that arrives later takes its name
// from the reading it arrived in, marked where that name is already held, so
// each name means exactly one monitor for the whole restore.
type stableNames struct {
	byKey map[string]string
	ids   map[string]domain.DisplayIdentity
	taken map[string]bool
}

// newStableNames returns the names of a restore that has seen no display yet.
func newStableNames() stableNames {
	return stableNames{
		byKey: map[string]string{},
		ids:   map[string]domain.DisplayIdentity{},
		taken: map[string]bool{},
	}
}

// learn names every display in a reading that this restore has not seen yet,
// answering whether it named any.
func (names stableNames) learn(set displaySet) bool {
	learned := false
	for _, display := range set.displays {
		key := displayKey(display.Identity)
		if _, known := names.byKey[key]; known {
			continue
		}
		name := set.name(display.Identity)
		if names.taken[name] {
			name += arrivedSuffix
		}
		if names.taken[name] {
			// Two arrivals at one held position: the monitor id is the one name
			// that cannot collide.
			name = display.Identity.String()
		}
		names.byKey[key], names.ids[key], names.taken[name] = name, display.Identity, true
		learned = true
	}
	return learned
}

// name answers what this restore calls a display; that it is not connected
// where the restore has never seen it.
func (names stableNames) name(identity domain.DisplayIdentity) string {
	if name, known := names.byKey[displayKey(identity)]; known {
		return name
	}
	return displayGone
}

// legend pairs every display this restore has named with its monitor id, in a
// settled order: what turns each position in the report back into the exact
// display it meant, including one that has since gone away.
func (names stableNames) legend() []string {
	lines := make([]string, 0, len(names.byKey))
	for key, name := range names.byKey {
		lines = append(lines, fmt.Sprintf(legendFormat, name, names.ids[key]))
	}
	sort.Strings(lines)
	return lines
}

// legendOf pairs the displays of one reading with the names this restore gives
// them, which is what the log writes at each reading that differs.
func (names stableNames) legendOf(set displaySet) []string {
	lines := make([]string, 0, len(set.displays))
	for _, display := range set.displays {
		lines = append(lines, fmt.Sprintf(legendFormat, names.name(display.Identity), display.Identity))
	}
	sort.Strings(lines)
	return lines
}
