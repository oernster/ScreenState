//go:build windows

package ui

import (
	"encoding/binary"
	"testing"
)

// record builds a raw input record of one kind with a body of the given size,
// writing one little-endian value at an offset into the body.
func record(kind uint32, bodySize, at int, value uint32, width int) []byte {
	headerSize := int(rawInputHeaderSize)
	bytes := make([]byte, headerSize+bodySize)
	binary.LittleEndian.PutUint32(bytes, kind)
	body := bytes[headerSize:]
	if width == fieldWidth16 {
		binary.LittleEndian.PutUint16(body[at:], uint16(value))
	} else {
		binary.LittleEndian.PutUint32(body[at:], value)
	}
	return bytes
}

// rawBodySize is comfortably larger than either record body read here.
const rawBodySize = 24

func TestAKeyGoingDownIsAPress(t *testing.T) {
	t.Parallel()
	header := rawInputHeaderSize
	for _, message := range []uint32{wmKeyDown, wmSysKeyDown} {
		if !isPress(record(rimTypeKeyboard, rawBodySize, keyboardMessageAt, message, fieldWidth32), header) {
			t.Errorf("key message %#x was not a press", message)
		}
	}
	const keyUp = 0x0101
	if isPress(record(rimTypeKeyboard, rawBodySize, keyboardMessageAt, keyUp, fieldWidth32), header) {
		t.Error("a key coming up was a press")
	}
}

func TestAButtonGoingDownIsAPressAndAMovementIsNot(t *testing.T) {
	t.Parallel()
	header := rawInputHeaderSize
	const leftDown, leftUp = 0x0001, 0x0002
	if !isPress(record(rimTypeMouse, rawBodySize, mouseButtonFlagsAt, leftDown, fieldWidth16), header) {
		t.Error("the left button going down was not a press")
	}
	if isPress(record(rimTypeMouse, rawBodySize, mouseButtonFlagsAt, leftUp, fieldWidth16), header) {
		t.Error("the left button coming up was a press")
	}
	if isPress(record(rimTypeMouse, rawBodySize, mouseButtonFlagsAt, 0, fieldWidth16), header) {
		t.Error("a movement was a press")
	}
}

func TestAShortRecordIsNotAPress(t *testing.T) {
	t.Parallel()
	header := rawInputHeaderSize
	if isPress(make([]byte, header-1), header) {
		t.Error("a record shorter than its header was a press")
	}
	short := record(rimTypeKeyboard, keyboardMessageAt, 0, 0, fieldWidth32)
	if isPress(short, header) {
		t.Error("a keyboard record too short to hold its message was a press")
	}
}
