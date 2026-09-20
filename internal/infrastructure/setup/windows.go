//go:build windows

package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	uninstallKeyPath = `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + InstallFolder
	runKeyPath       = `Software\Microsoft\Windows\CurrentVersion\Run`
	themeKeyPath     = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	themeValueName   = "AppsUseLightTheme"
	runValueName     = InstallFolder
	shortcutName     = AppName + ".lnk"
)

// UninstallInfo carries the values written to the HKCU uninstall registry
// entry, which is what puts the agent in Settings and in the Apps list.
type UninstallInfo struct {
	Version      string
	InstallDir   string
	UninstallExe string
	IconPath     string
	EstimatedKB  uint32
}

// hidden keeps a shelled-out child process from flashing a console window over
// the progress the user is trying to watch. A windowless parent is given a
// brand new console by Windows unless it says otherwise.
func hidden() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

// WriteUninstallEntry registers the agent under the current user's uninstall
// list. NoModify and NoRepair are both zero, so Windows offers Modify and
// Repair alongside Uninstall and each one reopens this setup program rather
// than sending the user back to the download.
func WriteUninstallEntry(info UninstallInfo) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKeyPath, registry.WRITE)
	if err != nil {
		return fmt.Errorf("create uninstall key: %w", err)
	}
	defer key.Close()

	quoted := fmt.Sprintf("%q", info.UninstallExe)
	text := map[string]string{
		"DisplayName":     AppName,
		"DisplayVersion":  info.Version,
		"InstallLocation": info.InstallDir,
		"UninstallString": quoted + " " + UninstallFlag,
		"ModifyPath":      quoted,
		"DisplayIcon":     info.IconPath,
		"Publisher":       Publisher,
	}
	for name, value := range text {
		if err := key.SetStringValue(name, value); err != nil {
			return fmt.Errorf("set %q: %w", name, err)
		}
	}
	numbers := map[string]uint32{"NoModify": 0, "NoRepair": 0, "EstimatedSize": info.EstimatedKB}
	for name, value := range numbers {
		if err := key.SetDWordValue(name, value); err != nil {
			return fmt.Errorf("set %q: %w", name, err)
		}
	}
	return nil
}

// RemoveUninstallEntry deletes the uninstall registry entry.
func RemoveUninstallEntry() error {
	if err := registry.DeleteKey(registry.CURRENT_USER, uninstallKeyPath); err != nil {
		return fmt.Errorf("delete uninstall key: %w", err)
	}
	return nil
}

// InstalledVersion returns the installed version and whether the agent is
// installed at all, read from the uninstall registry entry.
func InstalledVersion() (string, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, uninstallKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()
	value, _, err := key.GetStringValue("DisplayVersion")
	if err != nil {
		return "", false
	}
	return value, true
}

// SetLaunchOnBoot adds or removes the current user's Run entry, which is what
// FR-046 asks for: the agent arranges the desktop after a sign-in, so it has to
// be started by one.
func SetLaunchOnBoot(exePath string, enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open run key: %w", err)
	}
	defer key.Close()
	if !enabled {
		_ = key.DeleteValue(runValueName)
		return nil
	}
	if err := key.SetStringValue(runValueName, runValue(exePath)); err != nil {
		return fmt.Errorf("set run value: %w", err)
	}
	return nil
}

// IsLaunchOnBoot reports whether the login entry is present AND names a real
// file.
//
// Presence alone is not enough. An entry left behind by an install that has
// been moved or removed is present, so the box would read as on while nothing
// launches, which is the worst of both: the setting says yes and Windows
// disagrees in silence. Checking the file means such an entry reads as off;
// turning it on then rewrites it correctly.
func IsLaunchOnBoot() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetStringValue(runValueName)
	if err != nil {
		return false
	}
	_, err = os.Stat(runTarget(value))
	return err == nil
}

// lightThemeOff is the value the theme key holds when Windows is set to a dark
// app theme. The value name reads the other way round, so the comparison is
// named rather than left as a bare zero.
const lightThemeOff = 0

// SystemPrefersDark reports whether Windows is set to a dark app theme, which
// is what the setup window opens in unless the user toggles it. A missing or
// unreadable value reads as light, matching a fresh Windows install.
func SystemPrefersDark() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, themeKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue(themeValueName)
	if err != nil {
		return false
	}
	return value == lightThemeOff
}

// createShortcut writes a .lnk through the Windows Script Host, which avoids
// handling COM directly for one call.
func createShortcut(linkPath, target, workDir string) error {
	script := fmt.Sprintf(
		`$s=(New-Object -ComObject WScript.Shell).CreateShortcut(%q);`+
			`$s.TargetPath=%q;$s.IconLocation=%q;$s.WorkingDirectory=%q;$s.Save()`,
		linkPath, target, target, workDir)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = hidden()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create shortcut %q: %w: %s", linkPath, err, string(out))
	}
	return nil
}

// Shortcuts says which shortcuts the user asked for.
type Shortcuts struct {
	StartMenu bool
	Desktop   bool
}

// ApplyShortcuts creates the shortcuts that are wanted and removes the ones
// that are not, so unticking a box takes the shortcut away rather than leaving
// a stale one behind. Each location is best effort: a failure in one never
// stops the other, because a missing shortcut is not worth failing an install
// over.
func ApplyShortcuts(exePath, workDir string, want Shortcuts) {
	place := func(dir string, wanted bool) {
		link := filepath.Join(dir, shortcutName)
		if !wanted {
			_ = os.Remove(link)
			return
		}
		_ = createShortcut(link, exePath, workDir)
	}
	if dir, err := StartMenuProgramsDir(); err == nil {
		place(dir, want.StartMenu)
	}
	if dir, err := DesktopDir(); err == nil {
		place(dir, want.Desktop)
	}
}

// CurrentShortcuts reports which shortcuts exist, so every screen opens with
// its boxes already reflecting the machine rather than all ticked.
func CurrentShortcuts() Shortcuts {
	present := func(dir string, err error) bool {
		if err != nil {
			return false
		}
		_, statErr := os.Stat(filepath.Join(dir, shortcutName))
		return statErr == nil
	}
	start, startErr := StartMenuProgramsDir()
	desktop, desktopErr := DesktopDir()
	return Shortcuts{
		StartMenu: present(start, startErr),
		Desktop:   present(desktop, desktopErr),
	}
}

// RemoveShortcuts deletes both shortcuts.
func RemoveShortcuts() {
	ApplyShortcuts("", "", Shortcuts{})
}

// DesktopDir returns the current user's Desktop directory.
func DesktopDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, "Desktop"), nil
}

// appData is where Windows keeps the roaming half of a user's application data,
// which is where the Start Menu lives.
const appData = "APPDATA"

// StartMenuProgramsDir returns the current user's Start Menu Programs
// directory.
func StartMenuProgramsDir() (string, error) {
	base := os.Getenv(appData)
	if base == "" {
		return "", fmt.Errorf("%s is not set", appData)
	}
	return filepath.Join(base, "Microsoft", "Windows", "Start Menu", "Programs"), nil
}
