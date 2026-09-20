//go:build windows

package setup

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/oernster/ScreenState/internal/product"
)

// ErrAppRunning says the agent is open, so an install or an uninstall that
// would overwrite or delete the running executable must not start.
//
//lint:ignore ST1005 setup dialog text shown to the user verbatim
var ErrAppRunning = errors.New(product.Name + " is running. Please close it and try again.")

// ErrAppStillRunning says the agent was asked to close and was still there
// after the wait, so setup stops rather than writing over a locked file.
//
//lint:ignore ST1005 setup dialog text shown to the user verbatim
var ErrAppStillRunning = errors.New(product.Name + " could not be closed. Please close it manually and try again.")

const (
	// closeTimeout bounds the wait for the executable lock to release, so a
	// stuck process cannot hang setup for ever.
	closeTimeout = 5 * time.Second
	// closePollStep is how often the wait rechecks.
	closePollStep = 100 * time.Millisecond
	// forcedExitCode is reported for a process ended by setup.
	forcedExitCode = 1
	// deletionDelaySeconds is how long the detached shell waits before removing
	// the install directory, which has to outlive the copy of setup running
	// from inside it.
	deletionDelaySeconds = 3
)

// IsAppRunning reports whether the agent is currently running.
func IsAppRunning() bool { return len(processIDs(ExeName)) > 0 }

// processIDs returns the ids of every running process with the given executable
// name, compared without regard to case.
//
// Matching by name is deliberate. Descent is never consulted: a tree kill
// decides parentage from recorded parent process ids, which churn on a machine
// where a program has been started and ended repeatedly, so setup can end up
// recorded as a descendant and terminate itself. The window then vanishes with
// nothing said, because a terminate is not a crash.
//
// A failure taking or walking the snapshot yields no ids, so a snapshot error
// never blocks a legitimate install.
func processIDs(exeName string) []uint32 {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil
	}
	target := strings.ToLower(exeName)
	var found []uint32
	for {
		if strings.ToLower(windows.UTF16ToString(entry.ExeFile[:])) == target {
			found = append(found, entry.ProcessID)
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			return found
		}
	}
}

// CloseRunningApp ends every running instance and waits for the executable lock
// to release. Termination is forced rather than a polite window close, because
// the agent lives in the notification area and has no window to close: only
// ending the process frees the file.
func CloseRunningApp() error {
	for _, pid := range processIDs(ExeName) {
		terminate(pid)
	}
	deadline := time.Now().Add(closeTimeout)
	for IsAppRunning() {
		if time.Now().After(deadline) {
			return ErrAppStillRunning
		}
		time.Sleep(closePollStep)
	}
	return nil
}

// terminate forcibly ends one process. Every failure is ignored, because a
// process that has already exited needs no further action and one this user
// cannot open is not this user's to end.
func terminate(pid uint32) {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return
	}
	defer windows.CloseHandle(handle)
	_ = windows.TerminateProcess(handle, forcedExitCode)
}

// LaunchApp starts the installed agent detached, with the install directory as
// its working directory, so it outlives the setup program rather than ending
// with it.
func LaunchApp() error {
	dir, err := InstallDir()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(dir, ExeName))
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %s: %w", AppName, err)
	}
	return cmd.Process.Release()
}

// ScheduleDirDeletion spawns a detached shell that waits briefly, so this
// process can exit and release its own copy of the executable, then removes the
// install directory. Setup lives inside the directory it is deleting, which is
// why the removal has to outlive it.
func ScheduleDirDeletion(dir string) {
	line := fmt.Sprintf(`ping 127.0.0.1 -n %d >nul & rmdir /s /q "%s"`, deletionDelaySeconds, dir)
	cmd := exec.Command("cmd", "/C", line)
	cmd.SysProcAttr = hidden()
	_ = cmd.Start()
}
