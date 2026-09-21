// Package product holds what this product is called, in one place.
//
// The name reaches a path, a log line, a mutex and a window title. A second
// copy of it is how a rename leaves one surface still announcing the old name,
// which has happened before in this account and was found only by reading the
// screen.
package product

// Name is the product's name as a person sees it.
const Name = "ScreenState"

// Description says what it is, for the places that show a line about it.
const Description = "Window layout profiles for Windows"

// SplashClass names the splash windows' class. It is never shown to anybody.
// It lives here because two packages need it: the one that draws the splash
// and the one whose input hook must not count a click on a splash as the user
// taking over the desktop (FR-078).
const SplashClass = Name + "Splash"
