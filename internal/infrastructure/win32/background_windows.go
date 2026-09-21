//go:build windows

package win32

import (
	"context"
	"fmt"

	"github.com/oernster/ScreenState/internal/domain"
	"golang.org/x/sys/windows"
)

// Background returns the applications running with a hidden window of the
// candidate shape and no candidate window shown (FR-005): NordVPN waiting in the
// notification area is the worked example.
//
// Measured on 2026-09-21 on the reference machine: 36 processes owned such a
// window and nothing shown. Four could not have their program read from a
// process holding no administrator rights (the window manager and three driver
// services), so they cannot be named and are left out; five lived under the
// Windows directory and are part of Windows. That left 25 applications once
// this product and a second copy of one were set aside. It is still an offer
// rather than a proposal, since helpers another application starts are among
// it; the review shows it unticked.
func (desktop *Desktop) Background(ctx context.Context) ([]domain.ApplicationIdentity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	windowsDirectory, err := windows.GetSystemWindowsDirectory()
	if err != nil {
		return nil, fmt.Errorf("reading where Windows is installed: %w", err)
	}
	shown := make(map[uint32]bool)
	var hidden []uint32
	for _, handle := range handles() {
		pid := processOf(handle)
		switch {
		case pid == 0:
		case isCandidate(handle):
			shown[pid] = true
		case !isVisible(handle) && hasCandidateShape(handle):
			hidden = append(hidden, pid)
		}
	}
	asked := make(map[uint32]bool, len(hidden))
	var found []domain.ApplicationIdentity
	for _, pid := range hidden {
		if shown[pid] || asked[pid] {
			continue
		}
		asked[pid] = true
		identity, imagePath, err := processIdentity(pid)
		if err != nil || partOfWindows(imagePath, windowsDirectory) {
			continue
		}
		found = append(found, identity)
	}
	return found, nil
}
