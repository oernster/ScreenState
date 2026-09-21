package win32

import (
	"testing"

	"github.com/oernster/ScreenState/internal/product"
)

// FR-078 against FR-079: a click on a splash closes the splash and nothing
// more, so it is not the user taking over; every other press is.
func TestAClickOnTheSplashIsNotTheUserTakingOver(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		key, button bool
		class       string
		takesOver   bool
	}{
		"a click on a splash":             {button: true, class: product.SplashClass, takesOver: false},
		"a click on any other window":     {button: true, class: "Chrome_WidgetWin_1", takesOver: true},
		"a click on nothing readable":     {button: true, class: "", takesOver: true},
		"a key press, even over a splash": {key: true, class: product.SplashClass, takesOver: true},
		"neither a key nor a button":      {class: "Notepad", takesOver: false},
	} {
		if got := takesOver(want.key, want.button, want.class); got != want.takesOver {
			t.Errorf("%s: takes over %v, wanted %v", name, got, want.takesOver)
		}
	}
}
