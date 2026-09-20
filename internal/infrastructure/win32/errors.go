package win32

import "errors"

// The errors this layer raises. They are sentinels, so a caller tests them with
// errors.Is and nothing above this package needs to know a Windows error code.
var (
	// ErrUnusableIdentity is an application identity this layer cannot act on,
	// such as an updater command missing its target.
	ErrUnusableIdentity = errors.New("application identity cannot be used")
	// ErrNotWindows is every call into this package on a machine that is not
	// Windows. The package builds and vets everywhere so the rest of the
	// product can be tested anywhere; it only works on the one platform.
	ErrNotWindows = errors.New("this product runs on Windows only")
	// ErrNoMonitorID is a connected display whose device interface name
	// Windows did not give in a form that survives a reboot. Such a display is
	// left out rather than named by a number, since the numbers were measured
	// disagreeing with each other.
	ErrNoMonitorID = errors.New("display has no usable identity")
)
