package win32

import "testing"

// FR-005: a program under the Windows directory is part of Windows and is never
// offered; a directory that merely starts with the same letters is not it.
func TestAProgramUnderTheWindowsDirectoryIsPartOfWindows(t *testing.T) {
	t.Parallel()
	const windowsDirectory = `C:\WINDOWS`
	for image, want := range map[string]bool{
		`C:\WINDOWS\system32\svchost.exe`: true,
		`C:\Windows\Explorer.EXE`:         true,
		`c:\windows\SystemApps\MicrosoftWindows.Client.CBS_cw5n1h2txyewy\CrossDeviceResume.exe`: true,
		`C:\Program Files\NordVPN\NordVPN.exe`:                                                  false,
		`C:\WindowsApps\Claude_1.0.0.0_x64__pzs8sxrjxfjjc\app\Claude.exe`:                       false,
		`C:\WINDOWS`: false,
	} {
		if got := partOfWindows(image, windowsDirectory); got != want {
			t.Errorf("%s: part of Windows %v, wanted %v", image, got, want)
		}
	}
	if !partOfWindows(`C:\Windows\notepad.exe`, `C:\Windows\`) {
		t.Error("a Windows directory given with its trailing separator matched nothing")
	}
	if partOfWindows(`C:\Windows\notepad.exe`, "") {
		t.Error("an unknown Windows directory claimed a program for Windows")
	}
}
