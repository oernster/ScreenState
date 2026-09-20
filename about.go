package main

import (
	_ "embed"

	"github.com/oernster/ScreenState/internal/product"
)

// licenceText is the licence this product is released under, carried inside the
// binary rather than read from beside it. A licence the program can only show
// when a file happens to sit next to it is one it cannot show at all once it is
// installed.
//
//go:embed LICENSE
var licenceText string

const (
	// appAuthor is who wrote it, said once here.
	appAuthor = "Oliver Ernster"
	// appCopyright carries no year: a year is a timestamp that goes stale in a
	// window nobody rebuilds to correct it.
	appCopyright = "© Oliver Ernster"
	// appLicence names the licence; licenceText above is the licence itself.
	appLicence = "GPL-3.0"
)

// CreditDTO names one open-source dependency and the licence it is offered
// under, for the About dialog.
type CreditDTO struct {
	Name    string `json:"name"`
	Licence string `json:"licence"`
}

// AboutDTO is what About shows. The artwork is the page's own; everything that
// can go out of date with a release comes from here.
type AboutDTO struct {
	Name      string      `json:"name"`
	Tagline   string      `json:"tagline"`
	Version   string      `json:"version"`
	Author    string      `json:"author"`
	Copyright string      `json:"copyright"`
	Licence   string      `json:"licence"`
	Credits   []CreditDTO `json:"credits"`
}

// About answers what this product is, who wrote it and what it is built on.
//
// Every licence below was read from the copy in the module cache rather than
// remembered: the third clause tells a three-clause BSD from a two-clause one,
// and gorilla/websocket is the two-clause form while the rest are three.
// Crediting a project under the wrong licence is worse than not crediting it.
func (a *App) About() AboutDTO {
	return AboutDTO{
		Name:      product.Name,
		Tagline:   product.Description,
		Version:   a.version,
		Author:    appAuthor,
		Copyright: appCopyright,
		Licence:   appLicence,
		Credits: []CreditDTO{
			{Name: "Go", Licence: "BSD-3-Clause"},
			{Name: "Wails", Licence: "MIT"},
			{Name: "wailsapp/go-webview2", Licence: "MIT"},
			{Name: "golang.org/x/sys", Licence: "BSD-3-Clause"},
			{Name: "labstack/echo", Licence: "MIT"},
			{Name: "gorilla/websocket", Licence: "BSD-2-Clause"},
			{Name: "google/uuid", Licence: "BSD-3-Clause"},
			{Name: "Microsoft Edge WebView2", Licence: "Microsoft software licence"},
		},
	}
}

// LicenceText answers the whole licence, for the dialog that shows it.
func (a *App) LicenceText() string { return licenceText }
