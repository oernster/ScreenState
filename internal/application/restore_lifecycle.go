package application

// The lifecycle of the restore in progress: a newer restore standing it
// down (FR-061), the user stopping it (FR-049) and it retiring as the most
// recent one once it has finished.

// standDown stops the restore in progress, if there is one, then waits for it
// to stop. It reports whether there was one.
func (service *RestoreService) standDown() bool {
	service.stopKeepingWatch()
	service.mutex.Lock()
	active := service.active
	if active == nil {
		service.mutex.Unlock()
		return false
	}
	active.report.WasReplaced = true
	active.report.Note("a newer restore was requested, so this one stopped; " +
		"every window already placed was left where it was")
	service.mutex.Unlock()

	active.cancel()
	<-active.done
	return true
}

// Cancel stops the restore in progress before its next action and waits for it
// to stop (FR-049). Every window already placed stays where it is and the report
// says the restore was cancelled. It reports whether there was a restore to
// stop.
func (service *RestoreService) Cancel() bool {
	service.stopKeepingWatch()
	service.mutex.Lock()
	active := service.active
	service.mutex.Unlock()
	if active == nil {
		return false
	}
	active.cancel()
	<-active.done
	service.log.Step("the restore was cancelled by the user")
	return true
}

// retire records a finished restore as the most recent one and clears it from
// the active slot, leaving a later restore alone if one has already replaced it
// there.
func (service *RestoreService) retire(active *activeRestore) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	service.last = active.report
	if service.active == active {
		service.active = nil
	}
}
