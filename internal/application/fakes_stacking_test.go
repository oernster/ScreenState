package application

import "context"

// fakeStacking is the stacking order of the fake desktop: which window is drawn
// over which (FR-081, FR-083). It is a list, top first, as Windows keeps one
// order for every top-level window.
//
// A test that states no order gets the windows in the order the desktop holds
// them, the first on top; a window the order does not name sits beneath all
// that it does, which is where a window new to the desktop is before anything
// restacks it.
type fakeStacking struct {
	// order is top first; nil means the order the windows are held in.
	order []WindowID
	// orderErr is the refusal a test wants from reading the order (FR-082).
	orderErr error
	// refuses holds the refusal for one window alone (FR-085).
	refuses map[WindowID]error
	// calls records every restack asked for, in order.
	calls [][]WindowID
	// unlisted names windows reading the order leaves out, which is a window
	// that went between reading the windows and reading their order.
	unlisted map[WindowID]bool
}

// raise puts a window on top, which is what an application starting does to
// its own window.
func (desktop *fakeDesktop) raise(id WindowID) {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	order := []WindowID{id}
	for _, other := range desktop.stackLocked() {
		if other != id {
			order = append(order, other)
		}
	}
	desktop.stacking.order = order
}

// StackingOrder answers every window, the one on top first.
func (desktop *fakeDesktop) StackingOrder(context.Context) ([]WindowID, error) {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	if desktop.stacking.orderErr != nil {
		return nil, desktop.stacking.orderErr
	}
	var listed []WindowID
	for _, id := range desktop.stackLocked() {
		if !desktop.stacking.unlisted[id] {
			listed = append(listed, id)
		}
	}
	return listed, nil
}

// Restack stacks the windows as the real desktop does: each directly beneath
// the one before, the first taking the highest place any of them held; a
// refused window stays where it was and the next goes beneath the last one
// that moved.
func (desktop *fakeDesktop) Restack(_ context.Context, ids []WindowID) map[WindowID]error {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	desktop.stacking.calls = append(desktop.stacking.calls, append([]WindowID(nil), ids...))
	order := desktop.stackLocked()
	asked := make(map[WindowID]bool, len(ids))
	for _, id := range ids {
		asked[id] = true
	}
	anchor := len(order)
	for at, id := range order {
		if asked[id] {
			anchor = at
			break
		}
	}
	refused := make(map[WindowID]error)
	moving := make(map[WindowID]bool)
	var moved []WindowID
	for _, id := range ids {
		if err := desktop.stacking.refuses[id]; err != nil {
			refused[id] = err
			continue
		}
		moving[id] = true
		moved = append(moved, id)
	}
	var rest []WindowID
	insertAt := 0
	for at, id := range order {
		if at == anchor {
			insertAt = len(rest)
		}
		if !moving[id] {
			rest = append(rest, id)
		}
	}
	if anchor == len(order) {
		insertAt = len(rest)
	}
	stacked := append(append(append([]WindowID(nil), rest[:insertAt]...), moved...), rest[insertAt:]...)
	desktop.stacking.order = stacked
	return refused
}

// stackLocked answers the order top first, the stated order followed by any
// window it does not name. The mutex is held by the caller.
func (desktop *fakeDesktop) stackLocked() []WindowID {
	order := append([]WindowID(nil), desktop.stacking.order...)
	named := make(map[WindowID]bool, len(order))
	for _, id := range order {
		named[id] = true
	}
	for _, window := range desktop.windows {
		if !named[window.ID] {
			order = append(order, window.ID)
		}
	}
	return order
}

// drawnOver reports whether upper is anywhere above lower now.
func (desktop *fakeDesktop) drawnOver(upper, lower WindowID) bool {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	for _, id := range desktop.stackLocked() {
		switch id {
		case upper:
			return true
		case lower:
			return false
		}
	}
	return false
}

// restacks answers every restack asked for.
func (desktop *fakeDesktop) restacks() [][]WindowID {
	desktop.mutex.Lock()
	defer desktop.mutex.Unlock()
	return append([][]WindowID(nil), desktop.stacking.calls...)
}
