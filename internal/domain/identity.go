package domain

import (
	"errors"
	"fmt"
	"strings"
)

// The errors the domain raises. They are sentinels rather than types, so a
// caller tests them with errors.Is and no caller needs to know a struct.
var (
	// ErrUnknownShowState is a stored show state this product does not know.
	ErrUnknownShowState = errors.New("unknown show state")
	// ErrEmptyIdentity is an application or display identity with no value.
	ErrEmptyIdentity = errors.New("identity has no value")
	// ErrUnknownIdentityKind is an application identity of no known kind.
	ErrUnknownIdentityKind = errors.New("unknown application identity kind")
	// ErrEmptyProfileName is a profile with no name.
	ErrEmptyProfileName = errors.New("profile has no name")
	// ErrInvalidRect is a rectangle with a width or height of zero or less.
	ErrInvalidRect = errors.New("rectangle has no area")
	// ErrDuplicateEntry is a profile naming one application twice.
	ErrDuplicateEntry = errors.New("profile names the same application twice")
)

// IdentityKind says how an application is named, because no single value names
// every application. Measured on 2026-09-19 and recorded in appendix E of the
// specification: a Store-packaged application carries a versioned install path
// and a version-free model id, an application under a versioned directory is
// reached through an updater that does not move; everything else is its own
// path.
type IdentityKind uint8

const (
	// KindPath names an application by its executable path. The default, correct
	// for anything installed into a directory that does not carry a version.
	KindPath IdentityKind = iota
	// KindAppUserModelID names a Store-packaged application by its model id,
	// which carries no version. Claude is the worked example.
	KindAppUserModelID
	// KindUpdaterCommand names an application through the updater command that
	// launches its current version. Discord is the worked example.
	KindUpdaterCommand
)

var identityKindNames = map[IdentityKind]string{
	KindPath:           "path",
	KindAppUserModelID: "model-id",
	KindUpdaterCommand: "updater",
}

// String returns the stored spelling of an identity kind.
func (kind IdentityKind) String() string {
	name, known := identityKindNames[kind]
	if !known {
		return fmt.Sprintf("IdentityKind(%d)", uint8(kind))
	}
	return name
}

// Valid reports whether an identity kind is one this product knows.
func (kind IdentityKind) Valid() bool {
	_, known := identityKindNames[kind]
	return known
}

// ParseIdentityKind turns a stored spelling back into an identity kind.
func ParseIdentityKind(text string) (IdentityKind, error) {
	for kind, name := range identityKindNames {
		if name == text {
			return kind, nil
		}
	}
	return KindPath, fmt.Errorf("%w: %q", ErrUnknownIdentityKind, text)
}

// ApplicationIdentity names an application in a way that survives its updates.
// It is deliberately not a window class: a class was measured carrying a GUID
// that changes every session, so matching on one would fail after every reboot.
type ApplicationIdentity struct {
	Kind  IdentityKind
	Value string
	// ModelID is the application user model id of a Store-packaged application
	// held beside its path (FR-071). The path is what starts it, which is what
	// the user's own double-click does; the model id is what still starts it
	// once an update has moved that path. It is empty for everything else and
	// names no application on its own, so it takes no part in Equal; Recognises
	// uses it to match a running window once the path has moved.
	ModelID string
}

// WithModelID returns a copy of the identity carrying a packaged application's
// model id beside its path (FR-071).
func (identity ApplicationIdentity) WithModelID(modelID string) ApplicationIdentity {
	identity.ModelID = strings.TrimSpace(modelID)
	return identity
}

// PackagedFallback returns the identity that still starts this application once
// an update has moved its path, plus whether there is one.
func (identity ApplicationIdentity) PackagedFallback() (ApplicationIdentity, bool) {
	if identity.ModelID == "" {
		return ApplicationIdentity{}, false
	}
	fallback, err := NewApplicationIdentity(KindAppUserModelID, identity.ModelID)
	return fallback, err == nil
}

// NewApplicationIdentity returns a validated application identity.
func NewApplicationIdentity(kind IdentityKind, value string) (ApplicationIdentity, error) {
	identity := ApplicationIdentity{Kind: kind, Value: strings.TrimSpace(value)}
	return identity, identity.Validate()
}

// Validate reports why an application identity cannot be used; nil when it can.
func (identity ApplicationIdentity) Validate() error {
	if !identity.Kind.Valid() {
		return fmt.Errorf("%w: %d", ErrUnknownIdentityKind, uint8(identity.Kind))
	}
	if strings.TrimSpace(identity.Value) == "" {
		return fmt.Errorf("%w: application identity of kind %s", ErrEmptyIdentity, identity.Kind)
	}
	return nil
}

// Equal reports whether two identities name the same application. Comparison
// ignores case, because Windows paths and model ids are matched that way and a
// profile captured from one spelling must match a window reporting another.
func (identity ApplicationIdentity) Equal(other ApplicationIdentity) bool {
	return identity.Kind == other.Kind &&
		strings.EqualFold(identity.Value, other.Value)
}

// Recognises reports whether a running window's identity is the application this
// one names, which is wider than Equal in one way only (FR-071): a Store-packaged
// application is recognised by its model id once an update has moved its path.
// A window reports its new path with the unchanged model id beside it, so it is
// matched to an entry holding the old path. It is matched just the same to an
// entry saved before the path was kept, which names the application by its model
// id alone.
//
// Equal stays exact: it is what says whether two stored entries are the same one.
func (identity ApplicationIdentity) Recognises(other ApplicationIdentity) bool {
	if identity.Equal(other) {
		return true
	}
	mine := identity.packageModelID()
	return mine != "" && strings.EqualFold(mine, other.packageModelID())
}

// packageModelID is the model id naming a packaged application, whether it is
// the identity itself or kept beside a path; empty for anything else.
func (identity ApplicationIdentity) packageModelID() string {
	if identity.Kind == KindAppUserModelID {
		return identity.Value
	}
	return identity.ModelID
}

// SameProgram reports whether another identity names this same program, in the
// wider sense that covers a second copy of it installed somewhere else.
//
// An identity is an executable's full path, which tells two programs apart
// correctly and tells two COPIES of one program apart just as correctly. That
// is wrong for the one question that asks whether something is this product:
// a capture taken by the copy in a build directory did not recognise the
// installed copy as itself, so it offered to arrange it like any other
// application (reported 2026-09-20).
//
// The file name is what two copies of one program share, so it is compared as
// well; only as a fallback. Two unrelated programs sharing a file name in
// different directories cost one excluded window each; the other way round
// costs a profile that arranges this product while it is arranging the desktop.
func (identity ApplicationIdentity) SameProgram(other ApplicationIdentity) bool {
	if identity.Equal(other) {
		return true
	}
	if identity.Kind != KindPath || other.Kind != KindPath {
		return false
	}
	mine := fileName(identity.Value)
	return mine != "" && strings.EqualFold(mine, fileName(other.Value))
}

// fileName is the last segment of a path. Both separators are cut on, because
// the domain may not import path/filepath and a stored path may carry either.
func fileName(value string) string {
	if cut := strings.LastIndexAny(value, `\/`); cut >= 0 {
		return value[cut+1:]
	}
	return value
}

// UpdaterStartFlag is how an updater is told which program to start, which is
// what an updater identity holds between the updater's path and that program.
const UpdaterStartFlag = "--processStart"

// packageSeparator ends the package name in a model id, before the publisher
// hash; packageApplication ends the package family, before the application.
const (
	packageSeparator   = "_"
	packageApplication = "!"
	packageNamespace   = "."
)

// Program is the file the identity starts, as a person would recognise it: the
// executable a path names, the program an updater is told to start; the model id
// itself for a packaged application known by nothing else.
//
// It is the second line of what the manager shows, so the user can see exactly
// which file was recorded without reading the whole path.
func (identity ApplicationIdentity) Program() string {
	switch identity.Kind {
	case KindUpdaterCommand:
		if _, target, found := strings.Cut(identity.Value, " "+UpdaterStartFlag+" "); found {
			return fileName(strings.TrimSpace(target))
		}
		return fileName(identity.Value)
	case KindAppUserModelID:
		return identity.Value
	default:
		return fileName(identity.Value)
	}
}

// Name is what the application is called, for the line a person reads first.
// It is derived from the identity rather than read from the executable, so it
// needs no file to exist and cannot disagree with what was recorded.
//
// A packaged application is named from its model id where it has one, because
// its path ends in whatever casing the package chose: Claude's measured path
// ends in claude.exe while its model id begins Claude. The model id names the
// package before the publisher hash; the last dotted part of that is the name.
// Everything else is its program without the extension.
func (identity ApplicationIdentity) Name() string {
	if modelID := identity.packageModelID(); modelID != "" {
		family, _, _ := strings.Cut(modelID, packageApplication)
		pkg, _, _ := strings.Cut(family, packageSeparator)
		if cut := strings.LastIndex(pkg, packageNamespace); cut >= 0 {
			pkg = pkg[cut+1:]
		}
		if pkg != "" {
			return pkg
		}
	}
	program := identity.Program()
	if cut := strings.LastIndex(program, "."); cut > 0 {
		return program[:cut]
	}
	return program
}

// String renders an identity for the report.
func (identity ApplicationIdentity) String() string {
	return fmt.Sprintf("%s:%s", identity.Kind, identity.Value)
}

// DisplayIdentity names a physical display in a way that survives a reboot. It
// holds the monitor id, measured unchanged across a reboot and unique per
// physical display: two screens of the same model differ only in the UID part
// of it, so a model code alone would place a window on the wrong screen.
//
// It is never the device name or the number Windows Settings shows. Those were
// measured disagreeing with each other and with the physical arrangement.
type DisplayIdentity struct {
	MonitorID string
}

// NewDisplayIdentity returns a validated display identity.
func NewDisplayIdentity(monitorID string) (DisplayIdentity, error) {
	identity := DisplayIdentity{MonitorID: strings.TrimSpace(monitorID)}
	return identity, identity.Validate()
}

// Validate reports why a display identity cannot be used; nil when it can.
func (identity DisplayIdentity) Validate() error {
	if strings.TrimSpace(identity.MonitorID) == "" {
		return fmt.Errorf("%w: display identity", ErrEmptyIdentity)
	}
	return nil
}

// Equal reports whether two identities name the same display.
func (identity DisplayIdentity) Equal(other DisplayIdentity) bool {
	return strings.EqualFold(identity.MonitorID, other.MonitorID)
}

// String renders a display identity for the report.
func (identity DisplayIdentity) String() string { return identity.MonitorID }
