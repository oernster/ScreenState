package runlog

import (
	"strings"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/product"
)

// stepIndent is what a step line opens with, before its stamp.
const stepIndent = "  "

// retained answers the log cut down to its most recent keep restores
// (NFR-OBS-001), each with the header of the run it belongs to. A log holding
// no more than that is answered whole: retention is counted in restores, never
// in bytes, so one long restore is not thrown away for its length.
func retained(text string, keep int) string {
	lines := strings.SplitAfter(text, "\n")
	var restores []int
	for at, line := range lines {
		if beginsARestore(line) {
			restores = append(restores, at)
		}
	}
	if len(restores) <= keep {
		return text
	}
	from := restores[len(restores)-keep]
	for at := from; at >= 0; at-- {
		if isRunHeader(lines[at]) {
			from = at
			break
		}
	}
	return strings.Join(lines[from:], "")
}

// beginsARestore reports whether a line is the step a restore writes as it
// begins: the indent, the stamp, one space, then application.RestoreBegins.
func beginsARestore(line string) bool {
	message, stepped := strings.CutPrefix(line, stepIndent)
	if !stepped || len(message) <= len(stepLayout) {
		return false
	}
	return strings.HasPrefix(message[len(stepLayout)+len(" "):], application.RestoreBegins)
}

// isRunHeader reports whether a line is the one a run opens with.
func isRunHeader(line string) bool {
	return strings.HasPrefix(line, product.Name+" ")
}
