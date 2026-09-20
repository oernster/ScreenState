//go:build windows

package win32

import (
	"context"
	"fmt"
	"unsafe"

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
type Launcher struct{}

// NewLauncher returns a launcher.
func NewLauncher() *Launcher { return &Launcher{} }

// Launch starts an application, each kind of identity in the way that kind is
// started.
//
// It is also how a running application is asked to show a window it is holding
// hidden (FR-056): running it again signals the instance already running, which
// shows and draws its own window, after which the second copy exits by itself.
// That was measured against NordVPN on 2026-09-20 and is the only mechanism
// that worked; acting on the hidden window directly produced an empty frame.
//
// The shell is used rather than a direct process start for two reasons: a model
// id can only be reached through it; it applies the working directory and the
// elevation rules the user's own double-click would.
func (launcher *Launcher) Launch(ctx context.Context, application domain.ApplicationIdentity) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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
	result, _, callErr := pShellExecute.Call(0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(target)),
		uintptr(unsafe.Pointer(parameters)),
		0, swNormal)
	// ShellExecuteW answers a value above 32 on success and an error code below
	// it on failure, which is the opposite of every other call here.
	if result <= shellExecuteFailure {
		return fmt.Errorf("starting %s: %w", application, callErr)
	}
	return nil
}

// shellExecuteFailure is the value ShellExecuteW stays above when it worked.
const shellExecuteFailure = 32
