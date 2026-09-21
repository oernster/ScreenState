// Package setup holds the per-user install policy behind the bespoke setup
// program: the paths and the payload extraction, which are portable and unit
// tested, plus the registry, shortcut and process work in the Windows files
// beside this one.
//
// Everything is per user. Nothing here needs administrator rights, so the whole
// flow runs without an elevation prompt (C-2).
//
// The setup program is a facade over this package and owns no install logic of
// its own, so what an install does can be read in one place and exercised
// without a window.
package setup

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/oernster/ScreenState/internal/infrastructure/store"
	"github.com/oernster/ScreenState/internal/product"
)

const (
	// AppName is the product's name as the user reads it, on the Start Menu
	// entry and in the Apps list. It is read from its one home rather than
	// written down: a second copy is how a rename leaves a surface announcing a
	// product that no longer exists.
	AppName = product.Name
	// Tagline is the line under the name in the setup window's header, read
	// from the same home the name is, so the page carries neither.
	Tagline = product.Description
	// InstallFolder names the install directory and the registry key. The
	// product's name carries no spaces, so a path built from it never needs
	// quoting for the sake of readability.
	InstallFolder = product.Name
	// ExeName is the installed agent executable.
	ExeName = product.Name + ".exe"
	// Publisher is recorded in the uninstall registry entry.
	Publisher = "Oliver Ernster"
	// UninstallFlag is the argument the Apps list passes back to the setup
	// program, recorded in the registry as part of the UninstallString. It is
	// held here rather than in the setup program because the entry that names
	// it is written here: two copies of it is a Remove button that opens setup
	// on the wrong screen.
	UninstallFlag = "-uninstall"
	// UninstallExeName is the copy of setup left inside the install directory,
	// so the Apps list has something to call after the downloaded setup file is
	// gone.
	UninstallExeName = "uninstall.exe"

	installSubdir = "Programs"
	dirPerm       = 0o755
)

// InstallDir returns the per-user install directory, which is LOCALAPPDATA
// joined with Programs and the product's folder. Installing there is what keeps
// the whole flow free of an administrator prompt.
func InstallDir() (string, error) {
	base := os.Getenv(localAppData)
	if base == "" {
		return "", fmt.Errorf("%s is not set", localAppData)
	}
	return filepath.Join(base, installSubdir, InstallFolder), nil
}

// localAppData is where Windows keeps a user's own application data.
const localAppData = "LOCALAPPDATA"

// StateDir returns the folder the agent writes its profiles and its log into.
//
// It is derived from the store's own answer rather than rebuilt here, so the
// place an uninstall offers to clear cannot drift from the place the agent
// actually writes. The store names the profiles folder inside it; the folder
// this returns is its parent, which is what also holds the log.
func StateDir() (string, error) {
	profiles, err := store.DefaultDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Dir(profiles), nil
}

// ExtractZip extracts a zip archive into dest, creating directories as needed
// and refusing any entry whose path would climb out of dest.
func ExtractZip(data []byte, dest string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open payload: %w", err)
	}
	if err := os.MkdirAll(dest, dirPerm); err != nil {
		return fmt.Errorf("create install dir %q: %w", dest, err)
	}
	for _, file := range reader.File {
		if err := extractEntry(file, dest); err != nil {
			return err
		}
	}
	return nil
}

// extractEntry writes one archive entry, rejecting a name that escapes dest.
// The check runs before anything is written, so a crafted archive cannot leave
// a file outside the install folder and then fail.
func extractEntry(file *zip.File, dest string) error {
	target := filepath.Join(dest, file.Name)
	fence := filepath.Clean(dest) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), fence) {
		return fmt.Errorf("unsafe path in payload: %q", file.Name)
	}
	if file.FileInfo().IsDir() {
		return os.MkdirAll(target, dirPerm)
	}
	if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
		return fmt.Errorf("create dir for %q: %w", target, err)
	}
	source, err := file.Open()
	if err != nil {
		return fmt.Errorf("open entry %q: %w", file.Name, err)
	}
	defer source.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, dirPerm)
	if err != nil {
		return fmt.Errorf("create %q: %w", target, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, source); err != nil {
		return fmt.Errorf("write %q: %w", target, err)
	}
	return nil
}

// bytesPerKB converts a byte count to the kilobytes the uninstall entry's
// EstimatedSize value is measured in.
const bytesPerKB = 1024

// DirSizeKB returns the total size of a directory tree in kilobytes, which is
// what the Apps list shows beside the entry.
func DirSizeKB(dir string) (uint32, error) {
	var total int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("size %q: %w", dir, err)
	}
	return uint32(total / bytesPerKB), nil
}

// RemoveTree deletes a directory tree. It is used for the saved profiles, which
// the uninstall screen offers to keep.
func RemoveTree(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove %q: %w", dir, err)
	}
	return nil
}

// CopyFile copies one file. It leaves a copy of the setup program inside the
// install directory, so the Apps list has an uninstaller to call once the
// downloaded setup file is gone.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %q: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, dirPerm)
	if err != nil {
		return fmt.Errorf("create %q: %w", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("write %q: %w", dst, err)
	}
	return nil
}

// HiddenFlag asks the agent to wait in the notification area rather than
// opening its manager.
//
// The sign-in entry carries it because a program that opens a window every time
// the machine is signed into is not waiting quietly, which is what the setting
// offers. Neighbouring entries in the same key do the same thing under their
// own spellings: Steam has -silent, Discord has --start-inactive.
const HiddenFlag = "-hidden"

// QuietFlag tells the agent that setup started it, so it opens its manager and
// arranges nothing (FR-038). Installing a program must not rearrange the
// desktop: measured on 2026-09-21, when each install started the agent, the
// start restored the profile and every install opened another Terminal window.
const QuietFlag = "-quiet"

// runValue is what the login entry holds: the path in quotes, then the flag.
//
// It is built here rather than with %q, which is Go's own quoting and escapes
// the separators inside the string; a Windows path written that way reaches the
// registry doubled and Windows spends every sign-in looking for a place that
// does not exist. A quoted path is what every other entry in that key looks
// like.
func runValue(exePath string) string {
	return `"` + exePath + `" ` + HiddenFlag
}

// runTarget reads a login entry back as the path it starts, so the entry can be
// checked against the file it names.
//
// The quoted form is read first because that is what is written: everything up
// to the closing quote is the path, whatever follows it is arguments. An
// unquoted value is taken whole rather than split on the first space, since a
// path with a space in it is ordinary.
func runTarget(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, `"`) {
		if end := strings.Index(trimmed[1:], `"`); end >= 0 {
			return trimmed[1 : end+1]
		}
	}
	return strings.Trim(trimmed, `"`)
}
