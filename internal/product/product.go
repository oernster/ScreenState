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
