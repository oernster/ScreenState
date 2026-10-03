//go:build windows

package setup

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// S-9: setup ends only the installed agent, the one process README:68 says it
// ends, never another program that happens to share its file name. The test
// binary stands in for a copy run from somewhere else: it reads the process
// table and nothing more.
func TestOnlyTheInstalledCopyIsFound(t *testing.T) {
	t.Parallel()
	running, err := os.Executable()
	if err != nil {
		t.Fatalf("reading this test's own path: %v", err)
	}
	self := uint32(os.Getpid())
	name := filepath.Base(running)

	if found := installedAgentIDs(name, t.TempDir()); slices.Contains(found, self) {
		t.Fatalf("a copy of %s outside the install folder was taken for the agent", name)
	}
	if found := installedAgentIDs(name, filepath.Dir(running)); !slices.Contains(found, self) {
		t.Fatalf("the copy inside the install folder was not found among %v", found)
	}
}
