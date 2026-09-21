package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The real theme, read from its one home, so a token renamed there fails here
// before a build ships a splash that cannot be drawn.
func TestTheSplashPaletteIsReadFromTheProductTheme(t *testing.T) {
	t.Parallel()
	css, err := os.ReadFile(filepath.Join("..", "..", "assets", "theme.css"))
	if err != nil {
		t.Fatalf("reading the theme: %v", err)
	}
	themes, err := ThemesFrom(string(css))
	if err != nil {
		t.Fatalf("reading the palette: %v", err)
	}
	// #ffffff and #141820 are the --panel values the theme declared when this
	// was written; the test checks the reading, not the choice of colour.
	if themes.Light.Panel != (Colour{R: 0xff, G: 0xff, B: 0xff}) {
		t.Errorf("the light panel read as %+v", themes.Light.Panel)
	}
	if themes.Dark.Panel != (Colour{R: 0x14, G: 0x18, B: 0x20}) {
		t.Errorf("the dark panel read as %+v", themes.Dark.Panel)
	}
	if themes.Light.Text == themes.Dark.Text {
		t.Error("both themes read the same text colour, so one block was read twice")
	}
}

func TestAMissingTokenIsRefused(t *testing.T) {
	t.Parallel()
	css := ":root {\n--panel: #ffffff;\n}\n:root[data-theme='dark'] {\n--panel: #000000;\n}"
	if _, err := ThemesFrom(css); err == nil || !strings.Contains(err.Error(), "--border") {
		t.Fatalf("a theme without --border answered %v", err)
	}
}

func TestAMissingBlockIsRefused(t *testing.T) {
	t.Parallel()
	if _, err := ThemesFrom(":root {\n}"); err == nil {
		t.Fatal("a theme with one block was accepted")
	}
	if _, err := ThemesFrom(""); err == nil {
		t.Fatal("an empty theme was accepted")
	}
}

func TestOnlyAHashAndSixDigitsIsAColour(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"ffffff", "#fff", "#gggggg", "rgba(0, 0, 0, 1)", ""} {
		if _, err := colourFrom(text); err == nil {
			t.Errorf("%q was accepted as a colour", text)
		}
	}
	colour, err := colourFrom("#0059a8")
	if err != nil || colour != (Colour{R: 0x00, G: 0x59, B: 0xa8}) {
		t.Fatalf("#0059a8 read as %+v, %v", colour, err)
	}
}
