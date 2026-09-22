package structural

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// pageDir holds the setup program's page. Nothing in it is compiled or type
// checked by anything, which is why the rules it has to keep are held here
// instead.
var pageDir = filepath.Join("installer", "frontend", "dist")

// facade is the file declaring the setup program's bound surface.
var facade = filepath.Join("installer", "app.go")

// pageFiles returns every source file of the setup page.
func pageFiles(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join(repoRoot(t), pageDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", pageDir, err)
	}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".js") ||
			strings.HasSuffix(name, ".css") {
			found = append(found, filepath.Join(dir, name))
		}
	}
	if len(found) == 0 {
		t.Fatalf("no page files found under %s, the walk is wrong", pageDir)
	}
	return found
}

// TestTheSetupPageNamesNothing keeps the product's name and its tagline out of a
// file no compiler reads.
//
// The name is held in one place and reaches the page on the state the program
// hands over. Written into the page instead, it survives a rename in silence:
// nothing type checks a string in a page, so setup goes on announcing a product
// that no longer exists and only reading the screen finds it. That has happened
// in this account before, in sixteen places at once.
func TestTheSetupPageNamesNothing(t *testing.T) {
	name := productNameValue(t)
	tagline := productTaglineValue(t)
	root := repoRoot(t)
	for _, path := range pageFiles(t) {
		text := readSource(t, path)
		relative, _ := filepath.Rel(root, path)
		for label, forbidden := range map[string]string{
			"the product's name": name,
			"the tagline":        tagline,
		} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s writes %s down: it arrives on the state instead",
					filepath.ToSlash(relative), label)
			}
		}
	}
}

// productTaglineValue reads the tagline out of its home, so this test cannot
// drift from the value it is guarding.
func productTaglineValue(t *testing.T) string {
	t.Helper()
	source := readSource(t, filepath.Join(repoRoot(t), "internal", "product", nameHome))
	_, after, found := strings.Cut(source, "const Description = \"")
	if !found {
		t.Fatalf("%s does not declare Description", nameHome)
	}
	value, _, found := strings.Cut(after, `"`)
	if !found || value == "" {
		t.Fatalf("%s declares an empty description", nameHome)
	}
	return value
}

// boundCall matches a call into the setup program from the page.
var boundCall = regexp.MustCompile(`\bbackend\(\)\.([A-Z][a-zA-Z0-9]*)\s*\(`)

// TestThePageCallsOnlyWhatIsBound holds the other half of the boundary: a call
// into the setup program that no method answers fails at the moment a user
// presses the button, which is the worst place to find it.
func TestThePageCallsOnlyWhatIsBound(t *testing.T) {
	bound := exportedMethodsOf(t, filepath.Join(repoRoot(t), facade), "App")
	if len(bound) == 0 {
		t.Fatalf("%s declares no exported method on App, so this check would pass over anything", facade)
	}
	for _, path := range pageFiles(t) {
		if !strings.HasSuffix(path, ".js") {
			continue
		}
		relative, _ := filepath.Rel(repoRoot(t), path)
		for _, match := range boundCall.FindAllStringSubmatch(readSource(t, path), -1) {
			if !bound[match[1]] {
				t.Errorf("%s calls %s, which the setup program does not bind",
					filepath.ToSlash(relative), match[1])
			}
		}
	}
}

// jsonTagsOf returns the json names of every field on a struct, which is the
// program's statement of the wire. It looks through every file of the package
// in dir rather than one named file. A facade split by subject moves a struct
// between files; a test naming the old one would stop finding it.
func jsonTagsOf(t *testing.T, dir, typeName string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	tags := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		collectJSONTags(t, filepath.Join(dir, name), typeName, tags)
	}
	if len(tags) == 0 {
		t.Fatalf("no file in %s declares json tags on %s", dir, typeName)
	}
	return tags
}

// collectJSONTags adds the json names of a struct's fields, where the file at
// path declares it, to tags.
func collectJSONTags(t *testing.T, path, typeName string, tags map[string]bool) {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		typed, ok := node.(*ast.TypeSpec)
		if !ok || typed.Name.Name != typeName {
			return true
		}
		structure, ok := typed.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structure.Fields.List {
			if field.Tag == nil {
				continue
			}
			_, after, found := strings.Cut(field.Tag.Value, `json:"`)
			if !found {
				continue
			}
			name, _, found := strings.Cut(after, `"`)
			if found && name != "" {
				tags[strings.SplitN(name, ",", 2)[0]] = true
			}
		}
		return true
	})
}

// exportedMethodsOf returns the exported methods declared on a type, which is
// the surface Wails binds and therefore the surface a page may reach for.
func exportedMethodsOf(t *testing.T, path, typeName string) map[string]bool {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	methods := map[string]bool{}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv == nil || !function.Name.IsExported() {
			continue
		}
		for _, receiver := range function.Recv.List {
			pointer, ok := receiver.Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			named, ok := pointer.X.(*ast.Ident)
			if ok && named.Name == typeName {
				methods[function.Name.Name] = true
			}
		}
	}
	// An empty answer is not a failure here: the root package is read file by
	// file and most of its files declare no method at all. Each caller asserts
	// that the methods it gathered add up to something, which is the check that
	// stops a scan passing over everything.
	return methods
}
