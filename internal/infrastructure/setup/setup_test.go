package setup

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// archiveOf builds a zip in memory from a name-to-contents map, so a test can
// say what the payload holds without a file on disk.
func archiveOf(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, contents := range entries {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("creating entry %q: %v", name, err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatalf("writing entry %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the archive: %v", err)
	}
	return buffer.Bytes()
}

func TestExtractZipWritesEveryEntry(t *testing.T) {
	t.Parallel()
	dest := filepath.Join(t.TempDir(), "install")
	payload := archiveOf(t, map[string]string{
		"agent.exe":           "binary",
		"assets/readme.txt":   "words",
		"assets/deep/one.txt": "more words",
	})

	if err := ExtractZip(payload, dest); err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}
	for name, expected := range map[string]string{
		"agent.exe":           "binary",
		"assets/readme.txt":   "words",
		"assets/deep/one.txt": "more words",
	} {
		raw, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if string(raw) != expected {
			t.Errorf("%s holds %q, want %q", name, string(raw), expected)
		}
	}
}

// TestExtractZipRefusesAnEntryThatClimbsOut is the fence. A crafted archive
// must not be able to write outside the install directory, so the entry is
// rejected by name before anything is opened.
func TestExtractZipRefusesAnEntryThatClimbsOut(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := filepath.Join(root, "install")
	payload := archiveOf(t, map[string]string{"../escaped.txt": "should never be written"})

	err := ExtractZip(payload, dest)
	if err == nil {
		t.Fatal("ExtractZip accepted an entry that climbs out of the install directory")
	}
	if !strings.Contains(err.Error(), "unsafe path") {
		t.Errorf("error is %q, which does not say what was refused", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "escaped.txt")); !os.IsNotExist(statErr) {
		t.Error("the escaping entry reached the disk")
	}
}

func TestExtractZipRejectsSomethingThatIsNotAnArchive(t *testing.T) {
	t.Parallel()
	if err := ExtractZip([]byte("not a zip at all"), t.TempDir()); err == nil {
		t.Fatal("ExtractZip accepted bytes that are not an archive")
	}
}

func TestDirSizeKBAddsUpTheTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("creating the nested directory: %v", err)
	}
	// Two files of a known size, so the answer is arithmetic rather than a
	// number read off a run.
	sizes := []int{bytesPerKB * 3, bytesPerKB * 5}
	for i, size := range sizes {
		path := filepath.Join([]string{dir, nested}[i], "file.bin")
		if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	measured, err := DirSizeKB(dir)
	if err != nil {
		t.Fatalf("DirSizeKB: %v", err)
	}
	if want := uint32(8); measured != want {
		t.Errorf("DirSizeKB = %d, want %d", measured, want)
	}
}

func TestDirSizeKBReportsAMissingDirectory(t *testing.T) {
	t.Parallel()
	if _, err := DirSizeKB(filepath.Join(t.TempDir(), "not there")); err == nil {
		t.Fatal("DirSizeKB accepted a directory that does not exist")
	}
}

func TestCopyFileReproducesTheContents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := filepath.Join(dir, "source.bin")
	target := filepath.Join(dir, "target.bin")
	if err := os.WriteFile(source, []byte("the setup program"), 0o644); err != nil {
		t.Fatalf("writing the source: %v", err)
	}
	if err := CopyFile(source, target); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading the copy: %v", err)
	}
	if string(raw) != "the setup program" {
		t.Errorf("the copy holds %q", string(raw))
	}
}

func TestCopyFileReportsAMissingSource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := CopyFile(filepath.Join(dir, "absent"), filepath.Join(dir, "out")); err == nil {
		t.Fatal("CopyFile accepted a source that does not exist")
	}
}

func TestRemoveTreeTakesTheWholeTree(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(filepath.Join(dir, "profiles"), 0o755); err != nil {
		t.Fatalf("creating the tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profiles", "one.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("writing a profile: %v", err)
	}
	if err := RemoveTree(dir); err != nil {
		t.Fatalf("RemoveTree: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("the tree survived RemoveTree")
	}
}

// TestInstallDirIsUnderTheUsersOwnData holds C-2: everything this product
// writes goes inside the signed-in user's own directories, which is what keeps
// Windows from asking for administrator rights.
func TestInstallDirIsUnderTheUsersOwnData(t *testing.T) {
	base := t.TempDir()
	t.Setenv(localAppData, base)

	dir, err := InstallDir()
	if err != nil {
		t.Fatalf("InstallDir: %v", err)
	}
	if want := filepath.Join(base, installSubdir, InstallFolder); dir != want {
		t.Errorf("InstallDir = %q, want %q", dir, want)
	}
}

func TestInstallDirSaysWhenWindowsNamesNowhere(t *testing.T) {
	t.Setenv(localAppData, "")
	if _, err := InstallDir(); err == nil {
		t.Fatal("InstallDir answered a path with no application data directory set")
	}
}

// TestStateDirIsTheFolderTheAgentWritesInto keeps the uninstall offer pointed at
// the place the store actually uses. It is derived from the store rather than
// rebuilt, so the two cannot drift; this asserts the derivation, which is the
// part that could be got wrong.
func TestStateDirIsTheFolderTheAgentWritesInto(t *testing.T) {
	base := t.TempDir()
	t.Setenv(localAppData, base)

	state, err := StateDir()
	if err != nil {
		t.Fatalf("StateDir: %v", err)
	}
	if want := filepath.Join(base, AppName); state != want {
		t.Errorf("StateDir = %q, want %q", state, want)
	}
	if state == filepath.Join(base, installSubdir, InstallFolder) {
		t.Error("the state directory and the install directory are the same place")
	}
}

// TestALoginEntryIsWrittenAsAQuotedPath is the ruling that came out of a broken
// entry: Go's own quoting escapes the separators, so a Windows path reached the
// registry doubled and Windows spent every sign-in looking for a place that does
// not exist.
func TestALoginEntryIsWrittenAsAQuotedPath(t *testing.T) {
	t.Parallel()
	path := `C:\Users\Someone\AppData\Local\Programs\Agent\agent.exe`
	value := runValue(path)
	if strings.Contains(value, `\\`) {
		t.Errorf("the login entry doubles its separators: %q", value)
	}
	if got := runTarget(value); got != path {
		t.Errorf("runTarget(runValue(p)) = %q, want %q", got, path)
	}
}

func TestALoginEntryIsReadBackWhateverItsShape(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		`"C:\Program Files\App\a.exe" -hidden`: `C:\Program Files\App\a.exe`,
		`"C:\App\a.exe"`:                       `C:\App\a.exe`,
		`C:\App With Spaces\a.exe`:             `C:\App With Spaces\a.exe`,
		`  "C:\App\a.exe"  `:                   `C:\App\a.exe`,
		`"unterminated`:                        `unterminated`,
	}
	for value, expected := range cases {
		if got := runTarget(value); got != expected {
			t.Errorf("runTarget(%q) = %q, want %q", value, got, expected)
		}
	}
}
