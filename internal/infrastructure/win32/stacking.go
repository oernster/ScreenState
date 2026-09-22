package win32

// nearestWindowAbove answers the closest window above start in the stacking
// order that a person would call a window; zero when there is none.
//
// It exists because the window directly above is often machinery. Measured on
// the reference machine on 2026-09-21: the window directly above the left
// Windows Terminal was a hidden input method window, MSCTFIME UI. A rebuilt
// button restacked beneath that window came back above Claude, since showing a
// window again brings its own hidden windows to the top with it, so the left
// Terminal ended on top of the desktop. The window a person sees above it,
// Claude, is the one to go back beneath.
//
// above answers the window directly above another, zero at the top; counts says
// whether a window is one a person would call a window. The walk stops at a
// window it has already visited, since the stacking order can change while it
// is read and a loop must not hang the restore.
func nearestWindowAbove(
	start uintptr,
	above func(uintptr) uintptr,
	counts func(uintptr) bool,
) uintptr {
	visited := map[uintptr]bool{start: true}
	for current := above(start); current != 0; current = above(current) {
		if visited[current] {
			return 0
		}
		visited[current] = true
		if counts(current) {
			return current
		}
	}
	return 0
}

// stackingOrder walks the stacking order down from the window on top,
// answering every window it passes, top first (FR-081). It is walked rather
// than taken from the order Desktop.Windows answers in, which reverses the
// enumeration on purpose and measured on 2026-09-22 put the lower of two
// windows first (OQ-13). The walk stops at a window already visited, since the
// order can change while it is read.
func stackingOrder(top uintptr, below func(uintptr) uintptr) []uintptr {
	var order []uintptr
	visited := make(map[uintptr]bool)
	for current := top; current != 0 && !visited[current]; current = below(current) {
		visited[current] = true
		order = append(order, current)
	}
	return order
}

// highestOf answers which of the windows sits highest in an order read top
// first; false where none of them is in it.
func highestOf(order []uintptr, windows []uintptr) (uintptr, bool) {
	wanted := make(map[uintptr]bool, len(windows))
	for _, window := range windows {
		wanted[window] = true
	}
	for _, window := range order {
		if wanted[window] {
			return window, true
		}
	}
	return 0, false
}

// restackBeneath stacks each window directly beneath the one before it, the
// first directly beneath anchor (zero puts it on top). A window that cannot be
// moved stays where it is and is answered with why; the next goes beneath the
// last one that moved, so the rest keep their order relative to each other
// (FR-085, FR-086).
func restackBeneath(
	windows []uintptr,
	anchor uintptr,
	beneath func(window, above uintptr) error,
) map[uintptr]error {
	refused := make(map[uintptr]error)
	above := anchor
	for _, window := range windows {
		if err := beneath(window, above); err != nil {
			refused[window] = err
			continue
		}
		above = window
	}
	return refused
}
