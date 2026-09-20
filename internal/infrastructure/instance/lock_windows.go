//go:build windows

// Package instance keeps this product to one running copy per signed-in user
// (FR-047), with a named Windows mutex.
//
// The Local namespace scopes the name to the user's own session, so two people
// signed in at once each get their own agent, which is what OOS-7 requires.
// Ported from WhatDay.
package instance

import (
	"errors"
	"fmt"

	"github.com/oernster/ScreenState/internal/product"
	"golang.org/x/sys/windows"
)

// Name is the mutex this product holds while it runs.
const Name = `Local\` + product.Name + ".SingleInstance"

// Lock is a held single-instance mutex.
type Lock struct{ handle windows.Handle }

// Acquire takes the mutex called name. It answers held false with no error
// where another copy already holds it, which is an answer rather than a fault:
// the second copy then stands down.
func Acquire(name string) (lock *Lock, held bool, err error) {
	wide, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, false, fmt.Errorf("naming the mutex %q: %w", name, err)
	}
	handle, err := windows.CreateMutex(nil, false, wide)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(handle)
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("creating the mutex %q: %w", name, err)
	}
	return &Lock{handle: handle}, true, nil
}

// Release lets the mutex go, so a later copy may start.
func (lock *Lock) Release() error { return windows.CloseHandle(lock.handle) }
