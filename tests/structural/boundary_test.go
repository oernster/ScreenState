// Package structural enforces the architecture with tests rather than convention.
//
// Every assertion here has been proved to bite by planting a violation and watching
// it fail. An assertion never seen to fail is not yet a guard.
package structural

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// modulePath prefixes every internal import.
const modulePath = "github.com/oernster/ScreenState/"

// compositionRoot names the files allowed to import both application and
// infrastructure. The agent's main.go builds the adapters and hands them to the
// services; nothing else may know both sides.
//
// The whitelist was written before main.go rather than after, so any other file
// that wires the two layers together is a test failure.
var compositionRoot = map[string]bool{"main.go": true}

// forbiddenInDomain names the packages that would make the domain impure. Time
// reaches this product through an injected clock, never through the domain.
var forbiddenInDomain = []string{
	"net", "net/http", "os", "path/filepath", "math/rand",
	"database/sql", "os/exec", "syscall", "unsafe", "log",
}

// forbiddenCallsInDomain names calls that read the wall clock or the global random
// source directly, which would make domain behaviour irreproducible.
var forbiddenCallsInDomain = []string{"time.Now", "rand.Intn", "rand.Float64"}

// repoRoot walks up from the test's directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for range 6 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find go.mod above the test directory")
	return ""
}

// goFiles returns every Go source file in the repository.
func goFiles(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	var found []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "venv" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repository: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("no Go files found, the walk is wrong")
	}
	return found
}

// importsOf parses a file and returns its import paths.
func importsOf(t *testing.T, path string) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	out := make([]string, 0, len(parsed.Imports))
	for _, item := range parsed.Imports {
		out = append(out, strings.Trim(item.Path.Value, `"`))
	}
	return out
}

// layerOf returns which architectural layer a file belongs to.
func layerOf(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) >= 2 && parts[0] == "internal" {
		return parts[1]
	}
	return ""
}

func TestDomainHasNoOutwardImports(t *testing.T) {
	root := repoRoot(t)
	for _, path := range goFiles(t) {
		if layerOf(root, path) != "domain" {
			continue
		}
		for _, imported := range importsOf(t, path) {
			if !strings.HasPrefix(imported, modulePath) {
				continue
			}
			inner := strings.TrimPrefix(imported, modulePath)
			if !strings.HasPrefix(inner, "internal/domain") {
				t.Errorf("%s imports %s: the domain depends on nothing", path, imported)
			}
		}
	}
}

func TestDomainIsPure(t *testing.T) {
	root := repoRoot(t)
	for _, path := range goFiles(t) {
		if layerOf(root, path) != "domain" || strings.HasSuffix(path, "_test.go") {
			continue
		}
		for _, imported := range importsOf(t, path) {
			for _, banned := range forbiddenInDomain {
				if imported == banned {
					t.Errorf("%s imports %q: the domain performs no IO", path, banned)
				}
			}
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for _, call := range forbiddenCallsInDomain {
			if strings.Contains(string(raw), call+"(") {
				t.Errorf("%s calls %s: inject the clock instead", path, call)
			}
		}
	}
}

func TestApplicationDoesNotImportInfrastructure(t *testing.T) {
	root := repoRoot(t)
	for _, path := range goFiles(t) {
		if layerOf(root, path) != "application" {
			continue
		}
		for _, imported := range importsOf(t, path) {
			if strings.Contains(imported, "internal/infrastructure") ||
				strings.Contains(imported, "internal/ui") {
				t.Errorf("%s imports %s: the application depends on its ports only", path, imported)
			}
		}
	}
}

func TestCompositionRootIsWhitelisted(t *testing.T) {
	root := repoRoot(t)
	for _, path := range goFiles(t) {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		var application, infrastructure bool
		for _, imported := range importsOf(t, path) {
			application = application || strings.Contains(imported, "internal/application")
			infrastructure = infrastructure || strings.Contains(imported, "internal/infrastructure")
		}
		if !application || !infrastructure {
			continue
		}
		relative, _ := filepath.Rel(root, path)
		if !compositionRoot[filepath.ToSlash(relative)] {
			t.Errorf("%s wires application to infrastructure: only the composition root may", relative)
		}
	}
}

// TestEveryExportedTypeIsDocumented keeps the package surface self-describing. The
// ports are read by whoever writes the Windows layer against them, so a port with
// no doc comment is a question they cannot answer from the code.
func TestEveryExportedTypeIsDocumented(t *testing.T) {
	for _, path := range goFiles(t) {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		for _, declaration := range parsed.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE || general.Doc != nil {
				continue
			}
			for _, spec := range general.Specs {
				typed, ok := spec.(*ast.TypeSpec)
				if ok && typed.Name.IsExported() && typed.Doc == nil {
					t.Errorf("%s: exported type %s has no doc comment", path, typed.Name.Name)
				}
			}
		}
	}
}
