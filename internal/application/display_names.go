package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/oernster/ScreenState/internal/domain"
)

// DisplayReader answers the displays connected now. It is the one question the
// manager asks of the desktop, so it is all the manager is given of it.
type DisplayReader interface {
	Displays(ctx context.Context) ([]Display, error)
}

// The words a display is named by, each spelt once. A display is named by where
// it sits because nothing else about it reads as a place: the number Windows
// Settings shows was measured disagreeing with the device name and with the
// physical arrangement (appendix E), so it cannot be what a person is told.
const (
	displaySuffix       = " display"
	displayOnly         = "only display"
	displayGone         = "display not connected"
	displayUnread       = "display not known"
	wordLeft            = "left"
	wordRight           = "right"
	wordCentre          = "centre"
	wordCentreLeft      = "centre left"
	wordCentreRight     = "centre right"
	wordTop             = "top"
	wordUpper           = "upper"
	wordBottom          = "bottom"
	wordLower           = "lower"
	wordMiddle          = "middle"
	fromFormat          = "%d%s from %s"
	rowOfThree          = 3
	rowOfFour           = 4
	displayCentreDivide = 2
)

// An ordinal's ending follows its last digit, except from 11 to 19 in every
// hundred, which all take th.
const (
	ordinalCycle      = 10
	ordinalCentury    = 100
	ordinalTeensFrom  = 11
	ordinalTeensTo    = 19
	ordinalDefaultEnd = "th"
)

// ordinalEnds are the endings of 1st, 2nd and 3rd; everything else takes th.
var ordinalEnds = map[int]string{1: "st", 2: "nd", 3: "rd"}

// displayNames answers what each connected display is called, keyed by its
// monitor id in lower case, since monitor ids are matched without regard to
// case.
//
// Displays whose vertical spans hold each other's centres form a row. The row
// holding the most displays is the main one and takes no vertical word; where
// two rows hold as many, the primary display's row is the main one. Within a
// row, displays are left, centre and right by the centres of their bounds.
// Rows above the main one are top (the farthest) and upper; rows below are
// bottom and lower. The reference machine reads top, left, centre and right.
func displayNames(displays []Display) map[string]string {
	names := map[string]string{}
	if len(displays) == 0 {
		return names
	}
	if len(displays) == 1 {
		names[displayKey(displays[0].Identity)] = displayOnly
		return names
	}
	rows := displayRows(displays)
	main := mainRow(rows)
	// Where the main row holds one display every row does, so the displays
	// stand in a column and each is named by its height in it alone.
	stacked := len(rows[main]) == 1
	for index, row := range rows {
		vertical := rowWord(index, main, len(rows))
		if stacked {
			vertical = stackWord(index, len(rows))
		}
		for position, display := range row {
			words := strings.Fields(vertical + " " + columnWord(position, len(row)))
			names[displayKey(display.Identity)] = strings.Join(words, " ") + displaySuffix
		}
	}
	return names
}

// displayKey is how a display is looked up among the names.
func displayKey(identity domain.DisplayIdentity) string {
	return strings.ToLower(identity.MonitorID)
}

// centreX and centreY are the middle of a rectangle, doubled so that no half
// pixel is lost to integer division; only their order is ever compared.
func centreX(rect domain.Rect) int64 {
	return int64(rect.X)*displayCentreDivide + int64(rect.Width)
}

func centreY(rect domain.Rect) int64 {
	return int64(rect.Y)*displayCentreDivide + int64(rect.Height)
}

// sharesRow reports whether each display's vertical centre lies within the
// other's span, which is what side by side means for two screens that need not
// be the same height or perfectly level.
func sharesRow(one, two domain.Rect) bool {
	holds := func(outer, inner domain.Rect) bool {
		middle := centreY(inner)
		return middle >= int64(outer.Y)*displayCentreDivide &&
			middle < int64(outer.Bottom())*displayCentreDivide
	}
	return holds(one, two) && holds(two, one)
}

// displayRows groups the displays into rows, top to bottom, each row left to
// right. Ties are broken on the monitor id, so the same desktop always reads the
// same way.
func displayRows(displays []Display) [][]Display {
	ordered := append([]Display(nil), displays...)
	sort.SliceStable(ordered, func(one, two int) bool {
		a, b := ordered[one], ordered[two]
		if centreY(a.Bounds) != centreY(b.Bounds) {
			return centreY(a.Bounds) < centreY(b.Bounds)
		}
		return displayKey(a.Identity) < displayKey(b.Identity)
	})
	var rows [][]Display
	for _, display := range ordered {
		last := len(rows) - 1
		if last >= 0 && sharesRow(rows[last][0].Bounds, display.Bounds) {
			rows[last] = append(rows[last], display)
			continue
		}
		rows = append(rows, []Display{display})
	}
	for _, row := range rows {
		sort.SliceStable(row, func(one, two int) bool {
			a, b := row[one], row[two]
			if centreX(a.Bounds) != centreX(b.Bounds) {
				return centreX(a.Bounds) < centreX(b.Bounds)
			}
			return displayKey(a.Identity) < displayKey(b.Identity)
		})
	}
	return rows
}

// mainRow is the index of the row holding the most displays, the primary
// display's row where two rows hold as many.
func mainRow(rows [][]Display) int {
	main := 0
	for index, row := range rows {
		switch {
		case len(row) > len(rows[main]):
			main = index
		case len(row) == len(rows[main]) && holdsPrimary(row) && !holdsPrimary(rows[main]):
			main = index
		}
	}
	return main
}

// holdsPrimary reports whether a row holds the primary display.
func holdsPrimary(row []Display) bool {
	for _, display := range row {
		if display.Primary {
			return true
		}
	}
	return false
}

// rowWord is the vertical word for a row: none for the main row, top or bottom
// for the farthest row on its side, upper or lower for any between.
func rowWord(index, main, rows int) string {
	switch {
	case index == main:
		return ""
	case index == 0:
		return wordTop
	case index < main:
		return wordUpper
	case index == rows-1:
		return wordBottom
	default:
		return wordLower
	}
}

// stackWord is the vertical word for a display in a column of the given height.
func stackWord(index, rows int) string {
	switch {
	case index == 0:
		return wordTop
	case index == rows-1:
		return wordBottom
	case rows == rowOfThree:
		return wordMiddle
	default:
		return ordinalFrom(index+1, wordTop)
	}
}

// ordinalFrom counts a place from an end: 2nd from left, 3rd from top.
func ordinalFrom(number int, end string) string {
	return fmt.Sprintf(fromFormat, number, ordinalEnd(number), end)
}

// columnWord is the horizontal word for a display within a row of the given
// width: none for a display alone in its row.
func columnWord(position, width int) string {
	last := width - 1
	switch {
	case width == 1:
		return ""
	case position == 0:
		return wordLeft
	case position == last:
		return wordRight
	case width == rowOfThree:
		return wordCentre
	case width == rowOfFour && position == 1:
		return wordCentreLeft
	case width == rowOfFour:
		return wordCentreRight
	default:
		return ordinalFrom(position+1, wordLeft)
	}
}

// ordinalEnd is the ending that makes a number an ordinal: 2nd, 3rd, 11th.
func ordinalEnd(number int) string {
	if teen := number % ordinalCentury; teen >= ordinalTeensFrom && teen <= ordinalTeensTo {
		return ordinalDefaultEnd
	}
	if end, known := ordinalEnds[number%ordinalCycle]; known {
		return end
	}
	return ordinalDefaultEnd
}

// displayNamer answers the name of the display a placement names, from one
// reading of the displays connected now.
type displayNamer struct {
	set  displaySet
	read bool
}

// readDisplayNames reads the displays once for a whole view. A reading that
// fails (or finds none) costs the names rather than the view: every placement
// then says its display is not known and the reason is logged.
func readDisplayNames(ctx context.Context, reader DisplayReader, log Log) displayNamer {
	displays, err := reader.Displays(ctx)
	if err == nil {
		var set displaySet
		if set, err = newDisplaySet(displays); err == nil {
			return displayNamer{set: set, read: true}
		}
	}
	log.Step(fmt.Sprintf("the displays could not be read, so none is named: %v", err))
	return displayNamer{}
}

// name answers what the display is called: its position where it is connected,
// that it is not connected where it is not.
func (namer displayNamer) name(identity domain.DisplayIdentity) string {
	if !namer.read {
		return displayUnread
	}
	return namer.set.name(identity)
}
