//go:build windows

package win32

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Starting a packaged application the way Windows documents it (FR-067).
//
// The fault this is aimed at, measured on the reference machine: after a
// sign-in restore the taskbar buttons of the two packaged applications the agent
// had started were drawn grey, without their icons, until the user clicked
// anywhere on the taskbar. Applications started by path or through an updater
// were drawn properly every time. Three answers were tried and measured not to
// be it, so nobody should try them again:
//
//   - Asking every taskbar to repaint. The log recorded four taskbars asked on
//     2026-09-20; the buttons stayed grey.
//   - Telling the shell its icons may have changed, with SHChangeNotify and
//     SHCNE_ASSOCCHANGED. It shipped; the next boot was still grey.
//   - Waiting for the shell to be ready. Mid-session, with the shell up for
//     minutes, closing both applications and pressing Apply gave grey buttons
//     again.
//
// What was left is how they are started: ShellExecute on
// shell:AppsFolder\<model id>, which is a route through the shell's namespace
// rather than the activation interface Windows provides for packaged
// applications. Whether this route draws the buttons properly is a hypothesis
// until a boot says so; the launcher records which route started each one.

// clsidApplicationActivationManager is CLSID_ApplicationActivationManager,
// {45BA127D-10A8-46EA-8AB7-56EA9078943C}.
var clsidApplicationActivationManager = windows.GUID{
	Data1: 0x45BA127D, Data2: 0x10A8, Data3: 0x46EA,
	Data4: [8]byte{0x8A, 0xB7, 0x56, 0xEA, 0x90, 0x78, 0x94, 0x3C},
}

// iidApplicationActivationManager is IID_IApplicationActivationManager,
// {2E941141-7F97-4756-BA1D-9DECDE894A3D}.
var iidApplicationActivationManager = windows.GUID{
	Data1: 0x2E941141, Data2: 0x7F97, Data3: 0x4756,
	Data4: [8]byte{0xBA, 0x1D, 0x9D, 0xEC, 0xDE, 0x89, 0x4A, 0x3D},
}

const (
	// aoNoErrorUI is AO_NOERRORUI: a failure comes back to the caller rather
	// than being put in front of the user, since the caller has the shell route
	// to fall back on.
	aoNoErrorUI = 0x2

	// sFalse is what CoInitializeEx answers when the thread was already
	// initialised the same way. It is success; it still owes an uninitialise.
	sFalse = syscall.Errno(1)
)

// activationManager is an IApplicationActivationManager as COM hands it over:
// a pointer to its table of methods.
type activationManager struct {
	methods *activationManagerMethods
}

// activationManagerMethods is the start of the interface's method table, in the
// order the interface declares them: IUnknown's QueryInterface and AddRef, which
// are never called, then Release and ActivateApplication. The two methods after
// those are never reached, so they are left off rather than named.
type activationManagerMethods struct {
	_                   [2]uintptr
	release             uintptr
	activateApplication uintptr
}

// activatePackaged starts the packaged application a model id names, answering
// the process it started.
//
// COM wants a thread of its own, so the work runs on a goroutine locked to its
// thread and never unlocked: the thread ends with the goroutine and takes its
// COM state with it, rather than going back to the scheduler initialised.
func activatePackaged(modelID string) (uint32, error) {
	type outcome struct {
		process uint32
		err     error
	}
	answer := make(chan outcome, 1)
	go func() {
		// A panic in a hand-built call into COM must not end the agent. It
		// becomes the error, which the launcher records before falling back.
		defer func() {
			if caught := recover(); caught != nil {
				answer <- outcome{err: fmt.Errorf("the activation manager call failed: %v", caught)}
			}
		}()
		runtime.LockOSThread()
		process, err := activateOnThisThread(modelID)
		answer <- outcome{process: process, err: err}
	}()
	result := <-answer
	return result.process, result.err
}

// activateOnThisThread does the COM work on a thread already locked to the
// calling goroutine.
func activateOnThisThread(modelID string) (uint32, error) {
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil && err != sFalse {
		return 0, fmt.Errorf("preparing COM: %w", err)
	}
	defer windows.CoUninitialize()

	var manager *activationManager
	result, _, _ := pCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidApplicationActivationManager)),
		0,
		windows.CLSCTX_LOCAL_SERVER,
		uintptr(unsafe.Pointer(&iidApplicationActivationManager)),
		uintptr(unsafe.Pointer(&manager)))
	if failed(result) {
		return 0, fmt.Errorf("reaching the activation manager: %w", syscall.Errno(result))
	}
	defer func() {
		_, _, _ = syscall.SyscallN(manager.methods.release, uintptr(unsafe.Pointer(manager)))
	}()

	name, err := windows.UTF16PtrFromString(modelID)
	if err != nil {
		return 0, fmt.Errorf("preparing the model id: %w", err)
	}
	var process uint32
	result, _, _ = syscall.SyscallN(manager.methods.activateApplication,
		uintptr(unsafe.Pointer(manager)),
		uintptr(unsafe.Pointer(name)),
		0,
		aoNoErrorUI,
		uintptr(unsafe.Pointer(&process)))
	if failed(result) {
		return 0, fmt.Errorf("activating: %w", syscall.Errno(result))
	}
	return process, nil
}

// failed reads an HRESULT, which signals failure by its top bit.
func failed(result uintptr) bool {
	return int32(uint32(result)) < 0
}
