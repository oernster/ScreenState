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
