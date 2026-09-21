//go:build windows

package ui

import (
	"image"
	"testing"
)

// recordingLog keeps every step it is told.
type recordingLog struct{ lines []string }

func (log *recordingLog) Step(message string) { log.lines = append(log.lines, message) }

// Windows builds an icon from the badged artwork; it refuses nothing a tray
// would be handed. An empty picture is answered with no icon rather than a
// call Windows would fail.
func TestWindowsBuildsTheAttentionIcon(t *testing.T) {
	t.Parallel()
	icon := iconFrom(badged(aMark(traySide, markColour), badgePalette))
	if icon == 0 {
		t.Fatal("Windows refused the attention icon")
	}
	if destroyed, _, _ := pDestroyIcon.Call(icon); destroyed == 0 {
		t.Error("the icon built could not be destroyed, so it was not an icon")
	}
	if iconFrom(image.NewNRGBA(image.Rect(0, 0, 0, 0))) != 0 {
		t.Error("an empty picture answered an icon")
	}
}

// The tray builds each palette's icon once. It frees what it built when it
// closes; a tray with no artwork builds nothing.
func TestTheTrayBuildsEachAttentionIconOnce(t *testing.T) {
	t.Parallel()
	steps := &recordingLog{}
	tray := NewTray(nil, steps, nil, Attention{
		Logo:   aMark(traySide, markColour),
		Themes: Themes{Light: badgePalette, Dark: badgePalette},
	})
	first := tray.attentionIcon()
	if first == 0 || tray.attentionIcon() != first {
		t.Fatalf("the icon was built as %d, then %d", first, tray.attentionIcon())
	}
	tray.forgetAttention()
	if len(tray.marked) != 0 {
		t.Fatal("closing kept the icons it built")
	}

	plain := NewTray(nil, steps, nil, Attention{})
	if plain.attentionIcon() != 0 || len(steps.lines) != 0 {
		t.Fatalf("a tray with no artwork marked its icon or complained: %v", steps.lines)
	}
}
