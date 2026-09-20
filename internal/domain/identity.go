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
