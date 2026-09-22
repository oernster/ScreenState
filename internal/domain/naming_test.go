package domain

import "testing"

// The identities below are the measured ones from appendix E, one per way an
// application is named, so each rule is held against what a capture records.
func TestNameAndProgramReadAsAPersonWould(t *testing.T) {
	t.Parallel()
	for label, test := range map[string]struct {
		identity ApplicationIdentity
		name     string
		program  string
	}{
		"a path": {
			identity: mustIdentity(t, KindPath, `C:\Users\Oliver\AppData\Local\Programs\PigeonPost\PigeonPost.exe`),
			name:     "PigeonPost", program: "PigeonPost.exe",
		},
		"a path with forward slashes": {
			identity: mustIdentity(t, KindPath, `C:/Programs/Stellody/Stellody.exe`),
			name:     "Stellody", program: "Stellody.exe",
		},
		"an updater, named by the program it starts rather than by itself": {
			identity: mustIdentity(t, KindUpdaterCommand,
				`C:\Users\Oliver\AppData\Local\Discord\Update.exe --processStart Discord.exe`),
			name: "Discord", program: "Discord.exe",
		},
		"an updater command missing its target": {
			identity: mustIdentity(t, KindUpdaterCommand, `C:\Discord\Update.exe`),
			name:     "Update", program: "Update.exe",
		},
		"a packaged path, named from its model id rather than its lower case file": {
			identity: mustIdentity(t, KindPath,
				`C:\Program Files\WindowsApps\Claude_2.2553.1.0_x64__pzs8sxrjxfjjc\app\claude.exe`).
				WithModelID(claudeModelID),
			name: "Claude", program: "claude.exe",
		},
		"a model id alone": {
			identity: mustIdentity(t, KindAppUserModelID, claudeModelID),
			name:     "Claude", program: claudeModelID,
		},
		"a model id whose package carries a namespace": {
			identity: mustIdentity(t, KindAppUserModelID,
				"Microsoft.WindowsCalculator_8wekyb3d8bbwe!App"),
			name: "WindowsCalculator", program: "Microsoft.WindowsCalculator_8wekyb3d8bbwe!App",
		},
		"a file with no extension": {
			identity: mustIdentity(t, KindPath, `C:\tools\runner`),
			name:     "runner", program: "runner",
		},
	} {
		if got := test.identity.Name(); got != test.name {
			t.Errorf("%s: Name answered %q, want %q", label, got, test.name)
		}
		if got := test.identity.Program(); got != test.program {
			t.Errorf("%s: Program answered %q, want %q", label, got, test.program)
		}
	}
}
