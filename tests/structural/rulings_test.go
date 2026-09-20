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

// The rulings below are not style. Each one is a decision recorded in the
// specification that a later edit could undo without anybody noticing, so each is
// held by a test rather than by a reader remembering it.

// terminationVerbs are the things a restore may never do to a window or a process
// (FR-029). The interface is the place to stop it: a method that does not exist
// cannot be called, so no reviewer has to catch the call.
var terminationVerbs = []string{"close", "terminate", "kill", "quit", "exit", "destroy"}

// terminationCalls name the ways a Go program ends somebody else's program. None
// of them belongs above the Windows layer; none belongs in this product at all.
// The layers this test covers are simply the ones that could reach for one while
// looking innocent.
var terminationCalls = []string{
	"TerminateProcess", "taskkill", "WM_CLOSE", "WM_QUIT",
	".Kill()", ".Signal(", "EndTask",
}

// windowsOnly names what may not appear above the infrastructure layer. The
// product is for Windows; the decisions about what a profile means are not.
var windowsOnly = []string{"syscall", "unsafe", "golang.org/x/sys"}

// portableLayers are the layers held to both rules.
var portableLayers = map[string]bool{"domain": true, "application": true}

// TestNothingAboveInfrastructureCanEndAProgram is FR-029 expressed as a shape
// rather than as a rule: the ruling that a restore closes nothing came from a
// measurement, that closing a window ends some applications outright and takes
// whatever was unsaved in them, so it must not be reachable by accident later.
func TestNothingAboveInfrastructureCanEndAProgram(t *testing.T) {
	root := repoRoot(t)
	for _, path := range goFiles(t) {
		if !portableLayers[layerOf(root, path)] || strings.HasSuffix(path, "_test.go") {
			continue
		}
		for _, method := range interfaceMethods(t, path) {
			lowered := strings.ToLower(method)
			for _, verb := range terminationVerbs {
				if strings.Contains(lowered, verb) {
					t.Errorf("%s: the port offers %s: a restore closes and terminates nothing (FR-029)",
						filepath.Base(path), method)
				}
			}
		}
		raw := readSource(t, path)
		for _, call := range terminationCalls {
			if strings.Contains(raw, call) {
				t.Errorf("%s reaches for %s: a restore closes and terminates nothing (FR-029)",
					filepath.Base(path), call)
			}
		}
	}
}

// TestTheDecisionsStayPortable keeps Windows out of the two layers that decide
// what a profile means. It is what lets every rule in this product be tested on a
// machine with no desktop at all.
func TestTheDecisionsStayPortable(t *testing.T) {
	root := repoRoot(t)
	for _, path := range goFiles(t) {
		if !portableLayers[layerOf(root, path)] {
			continue
		}
		for _, imported := range importsOf(t, path) {
			for _, banned := range windowsOnly {
				if imported == banned || strings.HasPrefix(imported, banned) {
					t.Errorf("%s imports %s: %s decides what a profile means, not how Windows does it",
						filepath.Base(path), imported, layerOf(root, path))
				}
			}
		}
	}
}

// timingsHome is the one file allowed to state a duration. Everything else reads
// the policy it is given.
const timingsHome = "ports.go"

// timingLiterals are the ways a duration gets written into code.
var timingLiterals = []string{"time.Second", "time.Minute", "time.Hour", "time.Millisecond"}

// TestEveryTimingHasOneHome holds the ceiling, the settle-check delay and the poll
// interval in a single place. A second copy of one of them is how a value the user
// set stops being the value the product uses, with nothing raising a word about it.
func TestEveryTimingHasOneHome(t *testing.T) {
	root := repoRoot(t)
	for _, path := range goFiles(t) {
		if layerOf(root, path) != "application" || strings.HasSuffix(path, "_test.go") {
			continue
		}
		if filepath.Base(path) == timingsHome {
			continue
		}
		raw := readSource(t, path)
		for _, literal := range timingLiterals {
			if strings.Contains(raw, literal) {
				t.Errorf("%s states %s: the timings live in %s and reach the services as a Policy",
					filepath.Base(path), literal, timingsHome)
			}
		}
	}
}

// nameHome is the one file allowed to write the product's name down.
const nameHome = "product.go"

// TestTheProductIsNamedOnce keeps the name in one place. It reaches a path, a
// mutex, a log line and a menu entry; a second copy is how a rename leaves one
// surface still announcing a product that no longer exists, which has happened
// in this account before and was found only by reading the screen.
//
// It governs string literals, which are what a user reads and what a path is
// built from. A comment naming the product is prose and breaks nothing.
//
// The name is looked for anywhere inside a literal rather than as a literal of
// its own: "Quit ScreenState" is exactly the copy that survives a rename; a
// check for the quoted name alone walks straight past it. The first version of
// this test did walk past it; a planted violation is what said so.
func TestTheProductIsNamedOnce(t *testing.T) {
	root := repoRoot(t)
	name := productNameValue(t)
	for _, path := range goFiles(t) {
		if strings.HasSuffix(path, "_test.go") || filepath.Base(path) == nameHome {
			continue
		}
		for _, literal := range stringLiterals(t, path) {
			if !strings.Contains(literal, name) {
				continue
			}
			relative, _ := filepath.Rel(root, path)
			t.Errorf("%s writes the product's name down in %q: it is held in %s and read from there",
				filepath.ToSlash(relative), literal, nameHome)
		}
	}
}

// stringLiterals returns every string literal in a file except the import
// paths, which carry the module's name and are not a copy of anything.
func stringLiterals(t *testing.T, path string) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	imported := make(map[*ast.BasicLit]struct{}, len(parsed.Imports))
	for _, item := range parsed.Imports {
		imported[item.Path] = struct{}{}
	}
	var found []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		if _, isImport := imported[literal]; isImport {
			return true
		}
		found = append(found, strings.Trim(literal.Value, "`\""))
		return true
	})
	return found
}

// productNameValue reads the name out of its home, so this test cannot drift
// from the value it is guarding.
func productNameValue(t *testing.T) string {
	t.Helper()
	source := readSource(t, filepath.Join(repoRoot(t), "internal", "product", nameHome))
	_, after, found := strings.Cut(source, "const Name = \"")
	if !found {
		t.Fatalf("%s does not declare Name", nameHome)
	}
	value, _, found := strings.Cut(after, `"`)
	if !found || value == "" {
		t.Fatalf("%s declares an empty name", nameHome)
	}
	return value
}

// interfaceMethods returns the names of every method declared on an interface in
// a file, which is the surface a caller can reach for.
func interfaceMethods(t *testing.T, path string) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	var methods []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		declared, ok := node.(*ast.InterfaceType)
		if !ok {
			return true
		}
		for _, field := range declared.Methods.List {
			for _, name := range field.Names {
				methods = append(methods, name.Name)
			}
		}
		return true
	})
	return methods
}

// readSource returns a file's text.
func readSource(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}
