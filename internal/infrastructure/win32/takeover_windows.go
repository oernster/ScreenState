//go:build windows

package win32

import "unsafe"

var pWindowFromPoint = user32.NewProc("WindowFromPoint")

// mouseHookInfo is MSLLHOOKSTRUCT, which a low-level mouse hook is handed.
// Only the point is read.
type mouseHookInfo struct {
	pt        point
	mouseData uint32
	flags     uint32
	time      uint32
	extraInfo uintptr
}

// pressedOn answers the class a button press landed on; empty for a key, whose
// hook detail is not a mouse structure at all.
func pressedOn(button bool, detail uintptr) string {
	if !button {
		return ""
	}
	return classUnderPointer(detail)
}

// classUnderPointer answers the class of the top-level window a low-level mouse
// hook's press landed on; empty where there is none.
func classUnderPointer(detail uintptr) string {
	if detail == 0 {
		return ""
	}
	// The hook hands the structure's address as a uintptr. Reading it through
	// the variable's own address is the form vet accepts; Windows keeps the
	// structure alive for the length of the callback this runs inside.
	info := *(**mouseHookInfo)(unsafe.Pointer(&detail))
	// WindowFromPoint takes the POINT by value, packed into one argument:
	// x in the low half, y in the high half.
	const halfBits = 32
	packed := uintptr(uint32(info.pt.x)) | uintptr(uint32(info.pt.y))<<halfBits
	window, _, _ := pWindowFromPoint.Call(packed)
	if window == 0 {
		return ""
	}
	if root, _, _ := pGetAncestor.Call(window, ancestorRoot); root != 0 {
		window = root
	}
	return classOf(window)
}
