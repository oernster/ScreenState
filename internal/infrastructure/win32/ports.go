package win32

import "github.com/oernster/ScreenState/internal/application"

// These three lines are the whole claim this package makes: that what it offers
// is what the application layer asked for. The compiler checks it on every
// platform, so the Windows half and the stubs beside it cannot drift apart from
// each other or from the ports; nobody has to compare three files by eye.
var (
	_ application.Desktop   = (*Desktop)(nil)
	_ application.Processes = (*Processes)(nil)
	_ application.Launcher  = (*Launcher)(nil)
)
