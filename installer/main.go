// Command installer is the bespoke setup program for the agent.
//
// It is a second program in the same module rather than a step of the build
// script, so the install policy it drives can be read and exercised on its own.
// The agent is a native Windows program with no window of its own; setup needs
// one, so it is built as a Wails application and wears a palette sampled from
// the agent's own artwork.
//
// It carries the built agent as an embedded zip and covers install, update,
// repair, reinstall, going back a version and uninstall, all per user with no
// administrator rights.
package main

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/oernster/ScreenState/internal/infrastructure/setup"
	"github.com/oernster/ScreenState/internal/product"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	windowsoptions "github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed payload.zip
var payload []byte

// appVersion is overridden at build time with -ldflags "-X main.appVersion=x.y.z",
// so the setup program never holds a version literal of its own. It is a var
// and not a const because -X reaches a var and silently does nothing to a
// const, which is a whole release shipped announcing the wrong number.
var appVersion = "dev"

const (
	windowTitle = product.Name + " Setup"
	// The window is fixed, so its height has to clear the tallest screen: the
	// install one, which carries four options under the path box. The figures
	// are the reference implementation's, at the same type sizes and the same
	// option count; they are rechecked whenever the type changes, because text
	// that grows without the window growing with it turns a fixed dialog into a
	// scrolling one.
	windowWidth  = 860
	windowHeight = 780
	// webviewFolder holds the setup window's own WebView2 cache. It is pinned
	// under TEMP rather than left to default into the roaming profile, so
	// running setup leaves no folder behind beside the agent's own.
	webviewFolder = product.Name + "Setup"
)

// dark and light are the surface colours of the setup palette, which is sampled
// from the agent's own artwork. One of them paints the window before the page
// loads, so setup never flashes the wrong ground.
var (
	dark  = options.RGBA{R: 0x0b, G: 0x0e, B: 0x14, A: 1}
	light = options.RGBA{R: 0xf4, G: 0xf6, B: 0xf9, A: 1}
)

func main() {
	prefersDark := setup.SystemPrefersDark()
	background := light
	if prefersDark {
		background = dark
	}
	app := NewApp(payload, appVersion, prefersDark)
	_ = wails.Run(&options.App{
		Title:            windowTitle,
		Width:            windowWidth,
		Height:           windowHeight,
		DisableResize:    true,
		BackgroundColour: &background,
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		Bind:             []interface{}{app},
		Windows: &windowsoptions.Options{
			WebviewUserDataPath: filepath.Join(os.TempDir(), webviewFolder),
		},
	})
}
