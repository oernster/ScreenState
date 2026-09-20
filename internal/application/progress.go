package application

// RestoreProgress is how far a restore has got, for the window to show while it
// waits (FR-065).
//
// Satisfied and Total count entries rather than seconds, because entries are
// what the restore actually knows: an application may take two seconds or two
// minutes to put a window up, so a bar weighted by time would be a guess shown
// as a measurement. Running is false when no restore is in progress, which is
// an answer rather than an absence.
type RestoreProgress struct {
	Running   bool
	Profile   string
	Satisfied int
	Total     int
}

// Progress answers how far the restore in progress has got, for the window to
// show while it waits (FR-065). Running is false where there is no restore,
// which is what the window reads the moment one ends.
func (service *RestoreService) Progress() RestoreProgress {
	return *service.progress.Load()
}

// noteProgress records a reading for the window to pick up.
func (service *RestoreService) noteProgress(reading RestoreProgress) {
	service.progress.Store(&reading)
}
