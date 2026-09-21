package application

import "fmt"

// The words a splash says (FR-078). They are the specification's own, so a
// change to them is a change to the specification first.
const (
	preparingHeadline = "Please wait while your desktop is prepared"
	readyHeadline     = "Your desktop is ready"
	// oneShortfall and manyShortfall name the entries still outstanding when a
	// restore ends: "1 application", never "1 applications" or "(s)".
	oneShortfall  = "1 application did not start"
	manyShortfall = "%d applications did not start"
)

// preparingMessage is what the splash says while a restore runs.
func preparingMessage() SplashMessage {
	return SplashMessage{Headline: preparingHeadline}
}

// readyMessage is what the splash says when a restore ends with the given
// number of entries outstanding. "Ready" over a desktop missing an application
// is a claim this product would know to be false, so the shortfall is said.
func readyMessage(outstanding int) SplashMessage {
	message := SplashMessage{Headline: readyHeadline}
	switch {
	case outstanding == 1:
		message.Detail = oneShortfall
	case outstanding > 1:
		message.Detail = fmt.Sprintf(manyShortfall, outstanding)
	}
	return message
}

// announcePreparing puts the splash up as a restore begins.
func (service *RestoreService) announcePreparing() {
	service.splash.Preparing(preparingMessage())
}

// announceReady says a restore has ended. A restore a newer request stood down
// says nothing: the newer one has already put the waiting words back up and
// will say ready itself, so the user never sees "ready" flash past mid-restore.
//
// It is called before the restore closes its done channel, which is what a
// newer restore waits on before putting its own words up, so the two can never
// arrive in the wrong order. WasReplaced is read under the lock standDown
// writes it under.
func (service *RestoreService) announceReady(report *Report) {
	service.mutex.Lock()
	replaced := report.WasReplaced
	service.mutex.Unlock()
	if replaced {
		return
	}
	_, outstanding := report.Counts()
	service.splash.Ready(readyMessage(outstanding))
}
