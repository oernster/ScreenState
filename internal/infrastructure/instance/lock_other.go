//go:build !windows

// Package instance keeps this product to one running copy per signed-in user
// (FR-047). Off Windows it holds nothing, so that the rest of the product can
// still be built and tested there.
package instance

import "github.com/oernster/ScreenState/internal/product"

// Name is the mutex this product would hold while it runs.
const Name = `Local\` + product.Name + ".SingleInstance"

// Lock is a held single-instance mutex, which holds nothing here.
type Lock struct{}

// Acquire always succeeds off Windows. It answers a working lock rather than
// nil, so no caller needs a platform check of its own.
func Acquire(string) (*Lock, bool, error) { return &Lock{}, true, nil }

// Release does nothing.
func (lock *Lock) Release() error { return nil }
