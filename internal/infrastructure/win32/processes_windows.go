//go:build windows

package win32

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/oernster/ScreenState/internal/application"
	"github.com/oernster/ScreenState/internal/domain"
	"golang.org/x/sys/windows"
)

// Processes answers whether an application is running, which no window can: an
// application holding every window hidden is still running and is recorded as
// such (FR-005).
type Processes struct{}

// NewProcesses returns a process reader.
func NewProcesses() *Processes { return &Processes{} }

// Running reports whether the application an identity names is running.
//
// It walks the processes this user can open rather than asking about one, since
// an identity is a rule about a program rather than a process id. A process
// this user may not open is not this user's application, so a refusal is
// skipped rather than reported: another user's copy of an application is not
// the one a profile means.
func (processes *Processes) Running(
	ctx context.Context,
	application domain.ApplicationIdentity,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, fmt.Errorf("listing the running programs: %w", err)
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()

	entry := windows.ProcessEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return false, fmt.Errorf("reading the running programs: %w", err)
	}
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if imagePath, readable := imageOfProcess(entry.ProcessID); readable &&
			matches(application, imagePath) {
			return true, nil
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			// The list ends with ERROR_NO_MORE_FILES, which is the ordinary
			// way out rather than a failure.
			return false, nil
		}
	}
}

// imageOfProcess returns the program a process id is running, plus whether it
// could be read at all.
func imageOfProcess(pid uint32) (string, bool) {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", false
	}
	defer func() { _ = windows.CloseHandle(process) }()
	imagePath, err := imageOf(process)
	if err != nil {
		return "", false
	}
	return imagePath, true
}

// Launcher starts an application by its recorded identity.
type Launcher struct {
	log application.Log
}

// NewLauncher returns a launcher that records in the log which route started a
// packaged application, so a boot can be read afterwards.
func NewLauncher(log application.Log) *Launcher { return &Launcher{log: log} }

// Launch starts an application, each kind of identity in the way that kind is
// started.
//
// It is also how a running application is asked to show a window it is holding
// hidden (FR-056): running it again signals the instance already running, which
// shows and draws its own window, after which the second copy exits by itself.
// That was measured against NordVPN on 2026-09-20 and is the only mechanism
// that worked; acting on the hidden window directly produced an empty frame.
//
// An application is started from its path through the shell, which applies the
// working directory and the elevation rules the user's own double-click would.
// A packaged application whose path an update has moved keeps a model id beside
// it and is started through the activation manager instead (FR-067, FR-071);
// the log says which of the two started it, since only the log can say
// afterwards why a taskbar button looks as it does. A profile written before
// FR-071 names such an application by its model id alone and goes straight to
// the activation manager.
func (launcher *Launcher) Launch(ctx context.Context, application domain.ApplicationIdentity) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if application.Kind == domain.KindAppUserModelID {
		return launcher.activate(application, application.Value)
	}
	err := shellOpen(application)
	if err == nil {
		launcher.log.Step(fmt.Sprintf("%s was started from its path", application))
		return nil
	}
	fallback, packaged := application.PackagedFallback()
	if !packaged {
		return err
	}
	launcher.log.Step(fmt.Sprintf("%s did not start from its path (%v), so its model id was used",
		application, err))
	return launcher.activate(application, fallback.Value)
}

// activate starts a packaged application through the activation manager; where
// that fails it goes through the shell, so the worst case is the older route.
func (launcher *Launcher) activate(application domain.ApplicationIdentity, modelID string) error {
	process, err := activatePackaged(modelID)
	if err == nil {
		launcher.log.Step(fmt.Sprintf(
			"%s was started by the activation manager, as process %d", application, process))
		return nil
	}
	launcher.log.Step(fmt.Sprintf(
		"the activation manager did not start %s (%v), so the shell was asked instead",
		application, err))
	identity, err := domain.NewApplicationIdentity(domain.KindAppUserModelID, modelID)
	if err != nil {
		return fmt.Errorf("starting %s: %w", application, err)
	}
	return shellOpen(identity)
}

// shellOpen starts an application through ShellExecute, which is how every
// identity was started before the activation manager and how a packaged
// application still is when that fails.
func shellOpen(application domain.ApplicationIdentity) error {
	program, arguments, err := launchFor(application)
	if err != nil {
		return err
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return fmt.Errorf("preparing to start %s: %w", application, err)
	}
	target, err := windows.UTF16PtrFromString(program)
	if err != nil {
		return fmt.Errorf("preparing to start %s: %w", application, err)
	}
	var parameters *uint16
	if len(arguments) > 0 {
		parameters, err = windows.UTF16PtrFromString(joinArguments(arguments))
		if err != nil {
			return fmt.Errorf("preparing to start %s: %w", application, err)
		}
	}
	// Shown without activating (FR-077): the agent holds no right to the
	// foreground at sign-in, so an application asked to come to the front is
	// refused and its taskbar button is marked red instead.
	result, _, callErr := pShellExecute.Call(0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(target)),
		uintptr(unsafe.Pointer(parameters)),
		0, swShowNoActivate)
	// ShellExecuteW answers a value above 32 on success and an error code below
	// it on failure, which is the opposite of every other call here.
	if result <= shellExecuteFailure {
		return fmt.Errorf("starting %s: %w", application, callErr)
	}
	return nil
}

// shellExecuteFailure is the value ShellExecuteW stays above when it worked.
const shellExecuteFailure = 32
