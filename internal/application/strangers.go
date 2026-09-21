package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/oernster/ScreenState/internal/domain"
)

// putTheRestAway puts every window the profile does not name out of the way,
// once the arrangement has settled (FR-063).
//
// A profile is a statement of what the desktop should look like, so a window
// that is not in it is in the way: the applications that start with Windows and
// are not part of a session were arriving on top of the arrangement and had to
// be put away by hand. It runs once the arrangement is settled, so a window
// still being placed is never mistaken for a stranger. This product's own
// windows are never put away: the manager is where Apply was pressed.
//
// How they are put away is the user's to choose (FR-064). Minimising asks the
// application for nothing at all and is the default. Closing is the other arm,
// because most of the applications that start with Windows go to the
// notification area when their window closes, which is where their owner wanted
// them in the first place; A-3 measured that an ordinary windowed application
// ends instead, which is why it is offered rather than simply done.
//
// Closing is for the sign-in restore alone. Apply always minimises, whatever
// the setting says, because the windows in the way then belong to the session
// the user is in the middle of.
func (service *RestoreService) putTheRestAway(
	ctx context.Context,
	profile domain.Profile,
	state *restoreState,
	why trigger,
) {
	report := state.report
	windows, err := service.desktop.Windows(ctx)
	if err != nil {
		report.Note("the windows this profile does not name were left alone: %v", err)
		return
	}
	if service.closeWanted(why, report) {
		service.closeTheRest(ctx, profile, windows, state)
		return
	}
	put := service.minimiseStrangers(ctx, profile, windows, report)
	if len(put) == 0 {
		return
	}
	report.Note("put away %d window(s) this profile does not name: %s",
		len(put), strings.Join(put, ", "))
	service.log.Step(fmt.Sprintf("put away %d window(s) outside %q", len(put), profile.Name))
}

// CloseStrangers reports what the restore at the next sign-in will do with the
// windows a profile does not name: close them where it answers true, minimise
// them where it answers false (FR-064).
func (service *RestoreService) CloseStrangers() (bool, error) {
	return service.strangers.CloseStrangers()
}

// SetCloseStrangers records what the sign-in restore should do with them. It
// takes effect at the next sign-in, since nothing holds the answer between
// restores.
func (service *RestoreService) SetCloseStrangers(closing bool) error {
	if err := service.strangers.SetCloseStrangers(closing); err != nil {
		return err
	}
	if closing {
		service.log.Step("at sign-in, windows a profile does not name will be asked to close")
	} else {
		service.log.Step("windows a profile does not name will be minimised")
	}
	return nil
}

// closeWanted asks whether this restore may close the windows in its way.
//
// A restore the user pressed Apply for never may, so the setting is not even
// read: the answer cannot depend on it and a note about a file nobody needed to
// read would be noise. A setting that cannot be read means minimising, since
// the arm that asks nothing of anybody is the one to take when the answer is
// not known; the report says so rather than leaving the user to work out which
// of the two happened.
func (service *RestoreService) closeWanted(why trigger, report *Report) bool {
	if why != atSignIn {
		return false
	}
	wanted, err := service.strangers.CloseStrangers()
	if err != nil {
		report.Note("the setting for windows this profile does not name could not be"+
			" read, so they were minimised rather than closed: %v", err)
		return false
	}
	return wanted
}

// closeTheRest asks every stranger to close, then puts away whatever is still
// there (FR-064).
func (service *RestoreService) closeTheRest(
	ctx context.Context,
	profile domain.Profile,
	windows []Window,
	state *restoreState,
) {
	report := state.report
	var asked []string
	for _, window := range windows {
		if !service.isStranger(profile, window) {
			continue
		}
		if err := service.desktop.Close(ctx, window.ID); err != nil {
			report.Note("%s could not be asked to close: %v", describeWindow(window), err)
			continue
		}
		asked = append(asked, describeWindow(window))
	}
	if len(asked) == 0 {
		return
	}
	report.Note("asked %d window(s) this profile does not name to close: %s",
		len(asked), strings.Join(asked, ", "))
	service.log.Step(fmt.Sprintf("asked %d window(s) outside %q to close", len(asked), profile.Name))
	service.putAwayWhatRefused(ctx, profile, state)
}

// putAwayWhatRefused minimises the windows that were asked to close and are
// still there when the user takes the desktop over or the ceiling passes.
//
// Closing is a request. An application may answer it with a prompt about
// unsaved work; it may ignore the request outright. Either way the window is
// still on top of the arrangement the restore has just made. Minimising is what the
// setting's other arm would have done, so it is what happens to a refusal;
// the report names them, because a window that was asked to close and did not
// is something only the user can settle.
func (service *RestoreService) putAwayWhatRefused(
	ctx context.Context,
	profile domain.Profile,
	state *restoreState,
) {
	report := state.report
	windows, still := service.whatIsStillThere(ctx, profile, state)
	if !still {
		return
	}
	put := service.minimiseStrangers(ctx, profile, windows, report)
	if len(put) == 0 {
		return
	}
	report.Note("%d window(s) did not close, so they were put away instead: %s",
		len(put), strings.Join(put, ", "))
	service.log.Step(fmt.Sprintf("%d window(s) outside %q did not close and were put away",
		len(put), profile.Name))
}

// whatIsStillThere watches the windows asked to close go, answering the reading
// that still holds strangers plus whether it found any.
//
// It ends the moment they are all gone. Anything still there when the user
// takes the desktop over (or when the ceiling passes) is treated as a refusal
// (FR-064, FR-079): a window that has not gone by the time the user has moved
// on is not going.
func (service *RestoreService) whatIsStillThere(
	ctx context.Context,
	profile domain.Profile,
	state *restoreState,
) ([]Window, bool) {
	for {
		windows, err := service.desktop.Windows(ctx)
		if err != nil {
			return nil, false
		}
		var held []Window
		for _, window := range windows {
			if service.isStranger(profile, window) {
				held = append(held, window)
			}
		}
		if len(held) == 0 {
			return nil, false
		}
		if service.givenUp(state) {
			return held, true
		}
		if err := service.await(ctx, state); err != nil {
			return nil, false
		}
	}
}

// minimiseStrangers minimises every window in the reading that the profile does
// not name, answering the ones it put away. A window it could not put away is
// noted where it happened rather than counted as put away.
func (service *RestoreService) minimiseStrangers(
	ctx context.Context,
	profile domain.Profile,
	windows []Window,
	report *Report,
) []string {
	var put []string
	for _, window := range windows {
		if !service.isStranger(profile, window) {
			continue
		}
		if err := service.desktop.Place(ctx, window.ID, window.Rect, domain.ShowMinimised); err != nil {
			report.Note("%s could not be put away: %v", describeWindow(window), err)
			continue
		}
		put = append(put, describeWindow(window))
	}
	return put
}

// isStranger reports whether a window belongs to none of the profile's
// applications and so is in the way of the arrangement.
//
// A window already minimised is left alone rather than minimised again: there
// is nothing to do and a report listing it would say something happened.
func (service *RestoreService) isStranger(profile domain.Profile, window Window) bool {
	if window.Unreadable != "" || !window.Visible || window.State == domain.ShowMinimised {
		return false
	}
	if window.Application.Validate() != nil || service.self.SameProgram(window.Application) {
		return false
	}
	_, named := profile.Find(window.Application)
	return !named
}
