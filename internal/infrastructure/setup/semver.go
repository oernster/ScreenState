package setup

import (
	"strconv"
	"strings"
)

// Relation says how the version carried by the setup program compares with the
// one already installed. It is what decides whether setup offers an update, a
// repair or a way back, so it is named rather than left as a bare integer.
type Relation int

const (
	// Older means the setup program carries an earlier version than the
	// installed one.
	Older Relation = iota - 1
	// Same means the two versions match, so setup offers a repair or a
	// reinstall.
	Same
	// Newer means the setup program carries a later version, so setup offers an
	// update.
	Newer
)

// fields is the count of numeric fields compared: major, minor and patch.
const fields = 3

// Compare reports how version a stands against version b. Only the numeric
// major.minor.patch fields take part; a pre-release suffix is ignored, because
// a suffix cannot make an install newer or older in a way this program acts on.
func Compare(a, b string) Relation {
	left, right := parse(a), parse(b)
	for i := range fields {
		if left[i] == right[i] {
			continue
		}
		if left[i] > right[i] {
			return Newer
		}
		return Older
	}
	return Same
}

// parse reads the leading major.minor.patch of a version string. A missing or
// unreadable field counts as zero, so a malformed version compares as an early
// one rather than failing the setup program outright.
func parse(version string) [fields]int {
	core := strings.SplitN(strings.TrimSpace(version), "-", 2)[0]
	parts := strings.Split(core, ".")
	var out [fields]int
	for i := 0; i < fields && i < len(parts); i++ {
		number, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil {
			continue
		}
		out[i] = number
	}
	return out
}
