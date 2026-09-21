// Package win32 is the only place in this product that knows Windows exists.
// It implements the application layer's ports: reading windows and displays,
// moving a window, saying whether an application is running and starting one.
//
// The file you are reading carries no Win32 call at all. It holds the rules
// that decide what a thing is called, which are string work rather than system
// work and are therefore settled by tests on any machine. Everything that has
// to ask Windows a question sits beside it behind a build tag.
package win32

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/oernster/ScreenState/internal/domain"
)

// interfacePrefix and the guid separator wrap a monitor's device interface name.
// Windows answers, for example:
//
//	\\?\DISPLAY#GSM784F#5&14514d51&0&UID4352#{e6f07b5f-ee97-4a90-b076-33f57bf4eaa7}
//
// The middle of that, the device instance path, is what survives a reboot and
// tells two screens of one model apart. Measured on 2026-09-19; see appendix E.
const (
	interfacePrefix = `\\?\`
	guidSeparator   = "#{"
)

// monitorIDFrom reduces a monitor's device interface name to the identity this
// product stores. It answers false for anything that does not look like one,
// since a display named by a string Windows did not give us is worse than a
// display the profile cannot name at all.
func monitorIDFrom(interfaceName string) (string, bool) {
	trimmed := strings.TrimSpace(interfaceName)
	trimmed = strings.TrimPrefix(trimmed, interfacePrefix)
	if cut := strings.Index(trimmed, guidSeparator); cut >= 0 {
		trimmed = trimmed[:cut]
	}
	if !strings.HasPrefix(strings.ToUpper(trimmed), "DISPLAY#") {
		return "", false
	}
	return trimmed, true
}

// versionedPrefix marks a directory whose name carries the version of the
// application inside it. Discord installs as app-1.0.9258 beside an updater
// that does not move, so the updater is the identity and the directory is not.
const versionedPrefix = "app-"

// updaterName is the program beside a versioned directory that starts the
// current version of the application.
const updaterName = "Update.exe"

// processStartFlag is how that updater is told which program to start.
const processStartFlag = "--processStart"

// updaterFor returns the command that starts the current version of an
// application installed under a versioned directory, plus whether the path
// looks like one at all. It does not check that the updater exists; that is a
// question for the filesystem and the caller asks it.
func updaterFor(imagePath string) (string, bool) {
	directory := filepath.Dir(imagePath)
	if !strings.HasPrefix(strings.ToLower(filepath.Base(directory)), versionedPrefix) {
		return "", false
	}
	updater := filepath.Join(filepath.Dir(directory), updaterName)
	return fmt.Sprintf("%s %s %s", updater, processStartFlag, filepath.Base(imagePath)), true
}

// identityFor decides how an application is named, by the rules in appendix E.
//
// An application is its own path, which is what starts it the way the user's
// own double-click does; both packaged applications on the reference machine
// were measured starting from their paths on 2026-09-21. A Store-packaged
// application installs into a directory carrying its version, so its path moves
// with every update: its model id, which carries no version, is kept beside the
// path and starts it again once that has happened (FR-071). An application
// under a versioned directory is named by the updater beside it.
//
// updaterExists is handed in rather than called directly, so the rule can be
// exercised without an installed copy of Discord.
func identityFor(
	imagePath string,
	modelID string,
	updaterExists func(path string) bool,
) (domain.ApplicationIdentity, error) {
	if trimmed := strings.TrimSpace(modelID); trimmed != "" {
		identity, err := domain.NewApplicationIdentity(domain.KindPath, imagePath)
		return identity.WithModelID(trimmed), err
	}
	if command, versioned := updaterFor(imagePath); versioned {
		if updater, _, found := strings.Cut(command, " "+processStartFlag+" "); found &&
			updaterExists(updater) {
			return domain.NewApplicationIdentity(domain.KindUpdaterCommand, command)
		}
	}
	return domain.NewApplicationIdentity(domain.KindPath, imagePath)
}

// packagesDirectory is where Windows installs a Store-packaged application.
const packagesDirectory = "windowsapps"

// familySeparator divides a model id into the package family and the
// application within it: Claude_pzs8sxrjxfjjc!Claude.
const familySeparator = "!"

// publisherSeparator divides an installed package directory's name from the
// publisher id that ends it: Claude_2.2553.1.0_x64__pzs8sxrjxfjjc.
const publisherSeparator = "__"

// matches reports whether a running program at imagePath is the application an
// identity names. It is how FR-025 knows not to launch a second copy and how an
// entry with no placement is satisfied.
func matches(identity domain.ApplicationIdentity, imagePath string) bool {
	switch identity.Kind {
	case domain.KindAppUserModelID:
		return matchesPackage(identity.Value, imagePath)
	case domain.KindUpdaterCommand:
		return matchesUpdater(identity.Value, imagePath)
	case domain.KindPath:
		if strings.EqualFold(filepath.Clean(identity.Value), filepath.Clean(imagePath)) {
			return true
		}
		// A packaged application's path carries its version, so an update moves
		// the running program to a new directory under the same package family
		// (FR-071).
		return identity.ModelID != "" && matchesPackage(identity.ModelID, imagePath)
	}
	return false
}

// matchesPackage reports whether a running program belongs to the package a
// model id names.
//
// The model id carries the package family, Claude_pzs8sxrjxfjjc, while the
// installed directory carries the version between the name and the publisher,
// Claude_2.2553.1.0_x64__pzs8sxrjxfjjc. Matching therefore takes the family
// apart rather than comparing the two strings, which never match.
func matchesPackage(modelID string, imagePath string) bool {
	family, _, found := strings.Cut(modelID, familySeparator)
	if !found || family == "" {
		return false
	}
	name, publisher, split := strings.Cut(family, "_")
	if !split || name == "" || publisher == "" {
		return false
	}
	directory := packageDirectoryOf(imagePath)
	if directory == "" {
		return false
	}
	return strings.HasPrefix(strings.ToLower(directory), strings.ToLower(name)+"_") &&
		strings.HasSuffix(strings.ToLower(directory), strings.ToLower(publisherSeparator+publisher))
}

// packageDirectoryOf returns the installed package directory holding a program,
// which is the segment directly below WindowsApps; empty where there is none.
func packageDirectoryOf(imagePath string) string {
	parts := strings.Split(filepath.ToSlash(imagePath), "/")
	for at, part := range parts {
		if strings.EqualFold(part, packagesDirectory) && at+1 < len(parts) {
			return parts[at+1]
		}
	}
	return ""
}

// matchesUpdater reports whether a running program is the application an
// updater command starts: the right program name, somewhere under the directory
// the updater itself sits in.
func matchesUpdater(command string, imagePath string) bool {
	updater, target, found := strings.Cut(command, " "+processStartFlag+" ")
	if !found {
		return false
	}
	target = strings.TrimSpace(target)
	if target == "" || !strings.EqualFold(filepath.Base(imagePath), target) {
		return false
	}
	root := strings.ToLower(filepath.ToSlash(filepath.Dir(strings.TrimSpace(updater))))
	return strings.HasPrefix(strings.ToLower(filepath.ToSlash(imagePath)), root+"/")
}

// launchFor returns what to run to start an application, as a program and its
// arguments.
//
// A Store-packaged application is started through the applications folder,
// which is how Windows itself starts one and the only way a model id can be
// used. An updater command is already a program and an argument list. A path is
// itself.
func launchFor(identity domain.ApplicationIdentity) (string, []string, error) {
	switch identity.Kind {
	case domain.KindAppUserModelID:
		return appsFolder + identity.Value, nil, nil
	case domain.KindUpdaterCommand:
		updater, target, found := strings.Cut(identity.Value, " "+processStartFlag+" ")
		if !found {
			return "", nil, fmt.Errorf("%w: %q", ErrUnusableIdentity, identity.Value)
		}
		return strings.TrimSpace(updater), []string{processStartFlag, strings.TrimSpace(target)}, nil
	case domain.KindPath:
		return identity.Value, nil, nil
	}
	return "", nil, fmt.Errorf("%w: %s", ErrUnusableIdentity, identity.Kind)
}

// partOfWindows reports whether a program lives under the Windows directory,
// which makes it part of Windows rather than an application the user installed.
// Measured on 2026-09-21: every hidden window of that shape under it belonged to
// Explorer, a service host, the task host or a driver's helper, none of which a
// profile could start or should. An empty directory answers false, since then
// nothing is known to be part of Windows.
func partOfWindows(imagePath, windowsDirectory string) bool {
	directory := strings.TrimRight(filepath.Clean(windowsDirectory), `\/`)
	if directory == "" || directory == "." {
		return false
	}
	image := filepath.Clean(imagePath)
	return len(image) > len(directory) &&
		strings.EqualFold(image[:len(directory)], directory) &&
		strings.ContainsRune(`\/`, rune(image[len(directory)]))
}

// appsFolder is the shell location every installed application appears in,
// which is how a model id is turned into something that can be started.
const appsFolder = `shell:AppsFolder\`

// joinArguments puts an argument list back into the one string the shell takes,
// quoting anything holding a space so that a path does not arrive as two
// arguments. Every application the reference machine runs lives under a path
// with a space in it somewhere, so this is the ordinary case rather than a
// defensive one.
func joinArguments(arguments []string) string {
	quoted := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		if strings.ContainsRune(argument, ' ') {
			quoted = append(quoted, `"`+argument+`"`)
			continue
		}
		quoted = append(quoted, argument)
	}
	return strings.Join(quoted, " ")
}
