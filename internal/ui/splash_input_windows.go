//go:build windows

package ui

import (
	"encoding/binary"
	"unsafe"
)

// Hearing the user's next key press or click once the desktop is ready.
//
// The splash never holds the keyboard, so ordinary key and mouse messages go to
// whichever window does. Raw input with the sink flag is how a window that does
// not hold it is told: Windows sends it a copy of every key and button while
// the ordinary messages carry on to their owner, so the press that closes the
// splash still reaches what the user pressed it in.

// Where the fields this reads sit inside the record after its header: the
// mouse's button flags after its two byte flags field and two bytes of
// padding; the keyboard's message after its make code, flags, reserved and
// virtual key fields, two bytes each.
const (
	mouseButtonFlagsAt = 4
	keyboardMessageAt  = 8
	fieldWidth16       = 2
	fieldWidth32       = 4
)

// listen asks Windows for raw key and mouse input. It is done once the
// restore has ended, so a press while it runs closes nothing: until then only a
// click on the splash itself does.
func (splash *Splash) listen() {
	splash.mutex.Lock()
	defer splash.mutex.Unlock()
	if splash.listening || len(splash.windows) == 0 {
		return
	}
	target := splash.windows[0]
	devices := []rawInputDevice{
		{usagePage: usagePageGeneric, usage: usageMouse, flags: ridevInputSink, target: target},
		{usagePage: usagePageGeneric, usage: usageKeyboard, flags: ridevInputSink, target: target},
	}
	registered, _, _ := pRegisterRawInput.Call(uintptr(unsafe.Pointer(&devices[0])),
		uintptr(len(devices)), unsafe.Sizeof(devices[0]))
	splash.listening = registered != 0
	// Both arms are said, so a splash that stayed up can be told apart in the
	// log from one that was never listening.
	if !splash.listening {
		splash.log.Step("the splash could not hear the keyboard or mouse, so only a click on it closes it")
		return
	}
	splash.log.Step("the splash is ready and closes at the next key press or click")
}

// deafen gives the raw input back, so nothing is delivered to a window that
// has gone.
func (splash *Splash) deafen() {
	splash.mutex.Lock()
	defer splash.mutex.Unlock()
	if !splash.listening {
		return
	}
	devices := []rawInputDevice{
		{usagePage: usagePageGeneric, usage: usageMouse, flags: ridevRemove},
		{usagePage: usagePageGeneric, usage: usageKeyboard, flags: ridevRemove},
	}
	_, _, _ = pRegisterRawInput.Call(uintptr(unsafe.Pointer(&devices[0])),
		uintptr(len(devices)), unsafe.Sizeof(devices[0]))
	splash.listening = false
}

// pressed reports whether a raw input record is a key going down or a mouse
// button going down, once the restore has ended.
func (splash *Splash) pressed(handle uintptr) bool {
	if _, ready := splash.said(); !ready {
		return false
	}
	const headerSize = rawInputHeaderSize
	var size uint32
	_, _, _ = pGetRawInputData.Call(handle, ridInput, 0,
		uintptr(unsafe.Pointer(&size)), headerSize)
	if uintptr(size) <= headerSize {
		return false
	}
	record := make([]byte, size)
	read, _, _ := pGetRawInputData.Call(handle, ridInput, uintptr(unsafe.Pointer(&record[0])),
		uintptr(unsafe.Pointer(&size)), headerSize)
	if read == ^uintptr(0) || uintptr(read) <= headerSize {
		return false
	}
	return isPress(record[:read], headerSize)
}

// isPress reads a raw input record: a key or button going down is a press; a
// movement, a release or anything shorter than it claims is not.
func isPress(record []byte, headerSize uintptr) bool {
	if uintptr(len(record)) < headerSize {
		return false
	}
	kind := binary.LittleEndian.Uint32(record[:fieldWidth32])
	body := record[headerSize:]
	switch kind {
	case rimTypeMouse:
		if len(body) < mouseButtonFlagsAt+fieldWidth16 {
			return false
		}
		flags := binary.LittleEndian.Uint16(body[mouseButtonFlagsAt:])
		return flags&mouseButtonsDown != 0
	case rimTypeKeyboard:
		if len(body) < keyboardMessageAt+fieldWidth32 {
			return false
		}
		message := binary.LittleEndian.Uint32(body[keyboardMessageAt:])
		return message == wmKeyDown || message == wmSysKeyDown
	}
	return false
}
