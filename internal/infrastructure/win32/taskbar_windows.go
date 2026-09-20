//go:build windows

package win32

import (
	"context"
)

// The arguments to SHChangeNotify for "the icons may have changed".
//
// shcneAssocChanged is the event installers raise after changing which program
// opens which file. It is the documented way to make the shell resolve its
// icons again rather than draw what it resolved before.
const (
	shcneAssocChanged = 0x08000000
	shcnfIDList       = 0x0000
)

// RefreshShellIcons tells the shell that its icons may have changed, so that it
// resolves them again (FR-067).
//
// The fault it is aimed at, measured on the reference machine: after a sign-in
// the taskbar buttons of the applications the agent had started were drawn
// grey, without their icons; they stayed that way until the user clicked
// anywhere on the taskbar. Both are packaged applications, started by
// application user model id; the ones started by path were drawn properly.
//
// A plain repaint was tried first and is not the answer. On 2026-09-20 every
// taskbar was invalidated and painted at the end of the restore, which the log
// recorded as four taskbars asked; the buttons were still grey afterwards.
// So the shell is not holding a stale drawing of an icon it has: it is holding
// the answer that it had no icon. This asks it to work that out again.
//
// It is a notification and nothing else. The shell decides what to do with it,
// no window is touched and nothing can be moved or ended by it.
func (desktop *Desktop) RefreshShellIcons(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// SHChangeNotify answers nothing at all, so there is no result to read and
	// no failure to report: the shell either acts on it or does not.
	_, _, _ = pSHChangeNotify.Call(shcneAssocChanged, shcnfIDList, 0, 0)
	return nil
}
