package win32

import (
	"errors"
	"testing"

	"github.com/oernster/ScreenState/internal/domain"
)

// Every case below is taken from appendix E of the specification, measured on
// the reference machine on 2026-09-19. They are the readings the identity rules
// rest on, so the tests use them rather than invented strings.
const (
	claudeImage   = `C:\Program Files\WindowsApps\Claude_2.2553.1.0_x64__pzs8sxrjxfjjc\app\claude.exe`
	claudeModelID = `Claude_pzs8sxrjxfjjc!Claude`
	discordImage  = `C:\Users\Oliver\AppData\Local\Discord\app-1.0.9258\Discord.exe`
	discordUpdate = `C:\Users\Oliver\AppData\Local\Discord\Update.exe --processStart Discord.exe`
	stellodyImage = `C:\Programs\Stellody\Stellody.exe`
)

func always(string) bool { return true }
func never(string) bool  { return false }
func TestAMonitorIsNamedByWhatSurvivesAReboot(t *testing.T) {
	t.Parallel()
	interfaceName := `\\?\DISPLAY#GSM784F#5&14514d51&0&UID4352#{e6f07b5f-ee97-4a90-b076-33f57bf4eaa7}`
	identity, usable := monitorIDFrom(interfaceName)
	if !usable {
		t.Fatal("a well formed interface name was rejected")
	}
	if identity != "DISPLAY#GSM784F#5&14514d51&0&UID4352" {
		t.Fatalf("read as %q", identity)
	}
}

// The left and the right screen are the same model and differ only in the UID,
// which is the whole reason a model code cannot name a display.
func TestTwoScreensOfOneModelAreToldApart(t *testing.T) {
	t.Parallel()
	left, _ := monitorIDFrom(`\\?\DISPLAY#HSJ1340#5&14514d51&0&UID4356#{guid}`)
	right, _ := monitorIDFrom(`\\?\DISPLAY#HSJ1340#5&14514d51&0&UID4354#{guid}`)
	if left == right || left == "" {
		t.Fatalf("the two screens read as %q and %q", left, right)
	}
}

func TestAnUnusableInterfaceNameIsRefused(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "   ", `\\?\{e6f07b5f}`, "MONITOR\\GSM784F", `\\?\USB#VID_046D`} {
		if identity, usable := monitorIDFrom(name); usable {
			t.Fatalf("%q was accepted as %q", name, identity)
		}
	}
}

// Appendix E, the identity rules, each against the application it was measured
// on. A packaged application is its path, with its model id kept beside it.
func TestAnApplicationIsNamedByWhatSurvivesItsUpdates(t *testing.T) {
	t.Parallel()
	for name, testCase := range map[string]struct {
		image   string
		model   string
		exists  func(string) bool
		kind    domain.IdentityKind
		value   string
		modelID string
	}{
		"a Store package is named by its path, keeping its model id": {
			image: claudeImage, model: claudeModelID, exists: never,
			kind: domain.KindPath, value: claudeImage, modelID: claudeModelID,
		},
		"a versioned directory is named by its updater": {
			image: discordImage, model: "", exists: always,
			kind: domain.KindUpdaterCommand, value: discordUpdate,
		},
		"everything else is named by its path": {
			image: stellodyImage, model: "", exists: always,
			kind: domain.KindPath, value: stellodyImage,
		},
		"a versioned directory with no updater falls back to its path": {
			image: discordImage, model: "", exists: never,
			kind: domain.KindPath, value: discordImage,
		},
		"a package under a versioned directory keeps its model id too": {
			image: discordImage, model: claudeModelID, exists: always,
			kind: domain.KindPath, value: discordImage, modelID: claudeModelID,
		},
	} {
		identity, err := identityFor(testCase.image, testCase.model, testCase.exists)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if identity.Kind != testCase.kind || identity.Value != testCase.value {
			t.Fatalf("%s: read as %s", name, identity)
		}
		if identity.ModelID != testCase.modelID {
			t.Fatalf("%s: kept the model id %q", name, identity.ModelID)
		}
	}
}

// FR-071: the model id kept beside a packaged application's path is what starts
// it once an update has moved that path.
func TestAPackagedApplicationKeepsAWayBackAfterAnUpdate(t *testing.T) {
	t.Parallel()
	identity, err := identityFor(claudeImage, claudeModelID, never)
	if err != nil {
		t.Fatalf("naming it: %v", err)
	}
	fallback, found := identity.PackagedFallback()
	if !found {
		t.Fatal("there is no way to start it once its path has moved")
	}
	if fallback.Kind != domain.KindAppUserModelID || fallback.Value != claudeModelID {
		t.Fatalf("the fallback reads as %s", fallback)
	}
	program, _, err := launchFor(fallback)
	if err != nil || program != appsFolder+claudeModelID {
		t.Fatalf("the fallback does not start it: %q, %v", program, err)
	}
}

// FR-025 rests on this: a running program is recognised as the application an
// entry names, although the two strings never match.
func TestARunningProgramIsRecognised(t *testing.T) {
	t.Parallel()
	claude, _ := domain.NewApplicationIdentity(domain.KindAppUserModelID, claudeModelID)
	packaged, _ := domain.NewApplicationIdentity(domain.KindPath, claudeImage)
	packaged = packaged.WithModelID(claudeModelID)
	updated := `C:\Program Files\WindowsApps\Claude_2.9999.0.0_x64__pzs8sxrjxfjjc\app\claude.exe`
	discord, _ := domain.NewApplicationIdentity(domain.KindUpdaterCommand, discordUpdate)
	stellody, _ := domain.NewApplicationIdentity(domain.KindPath, stellodyImage)

	for name, testCase := range map[string]struct {
		identity domain.ApplicationIdentity
		image    string
		want     bool
	}{
		"the Store package, installed under its versioned directory": {claude, claudeImage, true},
		"a different Store package":                                  {claude, `C:\Program Files\WindowsApps\Spotify_1.2_x64__zpdnekdrzrea0\Spotify.exe`, false},
		"the same name from another publisher":                       {claude, `C:\Program Files\WindowsApps\Claude_9.0_x64__somebodyelse\app\claude.exe`, false},
		"the updated application under its updater":                  {discord, discordImage, true},
		"a newer version of it":                                      {discord, `C:\Users\Oliver\AppData\Local\Discord\app-1.0.9300\Discord.exe`, true},
		"the same program name somewhere else entirely":              {discord, `C:\Elsewhere\Discord.exe`, false},
		"the packaged application at the path recorded":              {packaged, claudeImage, true},
		"the packaged application after an update moved it":          {packaged, updated, true},
		"a different package at a path never recorded":               {packaged, `C:\Program Files\WindowsApps\Spotify_1.2_x64__zpdnekdrzrea0\Spotify.exe`, false},
		"the plain path, spelled differently":                        {stellody, `C:\Programs\Stellody\.\Stellody.exe`, true},
		"a different program":                                        {stellody, `C:\Programs\Stellody\Updater.exe`, false},
	} {
		if got := matches(testCase.identity, testCase.image); got != testCase.want {
			t.Fatalf("%s: matched=%v", name, got)
		}
	}
}

func TestAMalformedIdentityMatchesNothing(t *testing.T) {
	t.Parallel()
	for name, identity := range map[string]domain.ApplicationIdentity{
		"a model id with no application": {Kind: domain.KindAppUserModelID, Value: "Claude_pzs8sxrjxfjjc"},
		"a model id with no publisher":   {Kind: domain.KindAppUserModelID, Value: "Claude!Claude"},
		"an updater with no target":      {Kind: domain.KindUpdaterCommand, Value: `C:\Discord\Update.exe`},
		"an updater with an empty target": {
			Kind: domain.KindUpdaterCommand, Value: `C:\Discord\Update.exe --processStart `,
		},
		"a kind this product does not know": {Kind: domain.IdentityKind(9), Value: "anything"},
	} {
		if matches(identity, claudeImage) || matches(identity, discordImage) {
			t.Fatalf("%s matched a running program", name)
		}
	}
	// A program outside WindowsApps cannot belong to a package.
	claude, _ := domain.NewApplicationIdentity(domain.KindAppUserModelID, claudeModelID)
	if matches(claude, stellodyImage) {
		t.Fatal("a plain path matched a package identity")
	}
}

func TestEachIdentityKnowsHowItIsStarted(t *testing.T) {
	t.Parallel()
	claude, _ := domain.NewApplicationIdentity(domain.KindAppUserModelID, claudeModelID)
	program, arguments, err := launchFor(claude)
	if err != nil || program != `shell:AppsFolder\`+claudeModelID || len(arguments) != 0 {
		t.Fatalf("a Store package starts as %q %v (%v)", program, arguments, err)
	}

	discord, _ := domain.NewApplicationIdentity(domain.KindUpdaterCommand, discordUpdate)
	program, arguments, err = launchFor(discord)
	if err != nil || program != `C:\Users\Oliver\AppData\Local\Discord\Update.exe` {
		t.Fatalf("the updater starts as %q (%v)", program, err)
	}
	if len(arguments) != 2 || arguments[0] != "--processStart" || arguments[1] != "Discord.exe" {
		t.Fatalf("the updater is given %v", arguments)
	}

	stellody, _ := domain.NewApplicationIdentity(domain.KindPath, stellodyImage)
	if program, arguments, err = launchFor(stellody); err != nil ||
		program != stellodyImage || len(arguments) != 0 {
		t.Fatalf("a path starts as %q %v (%v)", program, arguments, err)
	}
}

func TestAnIdentityThatCannotBeStartedSaysSo(t *testing.T) {
	t.Parallel()
	for name, identity := range map[string]domain.ApplicationIdentity{
		"an updater with no target":         {Kind: domain.KindUpdaterCommand, Value: `C:\Discord\Update.exe`},
		"a kind this product does not know": {Kind: domain.IdentityKind(9), Value: "anything"},
	} {
		if _, _, err := launchFor(identity); !errors.Is(err, ErrUnusableIdentity) {
			t.Fatalf("%s answered %v", name, err)
		}
	}
}
