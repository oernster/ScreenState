package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// The splash's colours come from the product's one palette, assets/theme.css,
// which the manager and the setup program already wear. Reading them from that
// file rather than writing them down again here is what keeps the three
// surfaces one product: a colour changed in the theme reaches the splash with
// no second edit to forget.

// Colour is one palette colour.
type Colour struct {
	R, G, B uint8
}

// SplashPalette is the part of the palette the agent draws natively: the
// splash, plus the badge on the tray icon, which is Danger ringed in Panel.
type SplashPalette struct {
	Panel  Colour
	Border Colour
	Text   Colour
	Muted  Colour
	Danger Colour
}

// Themes is the splash palette in both of the product's themes.
type Themes struct {
	Light SplashPalette
	Dark  SplashPalette
}

// The blocks of theme.css each theme is declared in.
const (
	lightBlock = ":root {"
	darkBlock  = ":root[data-theme='dark'] {"
)

// The tokens a splash reads.
const (
	panelToken  = "--panel"
	borderToken = "--border"
	textToken   = "--text"
	mutedToken  = "--muted"
	dangerToken = "--danger"
)

// hexDigits is the length of a colour written #rrggbb, without the hash.
const hexDigits = 6

// errNoBlock is a theme block theme.css does not carry.
var errNoBlock = errors.New("the theme block is missing")

// ThemesFrom reads the splash palette out of theme.css.
func ThemesFrom(css string) (Themes, error) {
	light, err := paletteIn(css, lightBlock)
	if err != nil {
		return Themes{}, fmt.Errorf("the light theme: %w", err)
	}
	dark, err := paletteIn(css, darkBlock)
	if err != nil {
		return Themes{}, fmt.Errorf("the dark theme: %w", err)
	}
	return Themes{Light: light, Dark: dark}, nil
}

// paletteIn reads the palette's tokens out of one block.
func paletteIn(css, block string) (SplashPalette, error) {
	_, after, found := strings.Cut(css, block)
	if !found {
		return SplashPalette{}, fmt.Errorf("%w: %s", errNoBlock, block)
	}
	body, _, _ := strings.Cut(after, "}")
	var palette SplashPalette
	for _, wanted := range []struct {
		token string
		into  *Colour
	}{
		{panelToken, &palette.Panel},
		{borderToken, &palette.Border},
		{textToken, &palette.Text},
		{mutedToken, &palette.Muted},
		{dangerToken, &palette.Danger},
	} {
		colour, err := tokenIn(body, wanted.token)
		if err != nil {
			return SplashPalette{}, err
		}
		*wanted.into = colour
	}
	return palette, nil
}

// tokenIn reads one "--name: #rrggbb;" declaration out of a block body.
func tokenIn(body, token string) (Colour, error) {
	for _, line := range strings.Split(body, "\n") {
		name, value, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found || strings.TrimSpace(name) != token {
			continue
		}
		return colourFrom(strings.TrimSuffix(strings.TrimSpace(value), ";"))
	}
	return Colour{}, fmt.Errorf("%s is not declared", token)
}

// colourFrom reads "#rrggbb". Anything else is refused rather than guessed at.
func colourFrom(text string) (Colour, error) {
	digits, found := strings.CutPrefix(text, "#")
	if !found || len(digits) != hexDigits {
		return Colour{}, fmt.Errorf("%q is not a #rrggbb colour", text)
	}
	value, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return Colour{}, fmt.Errorf("%q is not a #rrggbb colour: %w", text, err)
	}
	const byteBits, byteMask = 8, 0xff
	return Colour{
		R: uint8(value >> (2 * byteBits) & byteMask),
		G: uint8(value >> byteBits & byteMask),
		B: uint8(value & byteMask),
	}, nil
}
