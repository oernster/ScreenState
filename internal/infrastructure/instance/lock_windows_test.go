//go:build windows

package instance

import (
	"strings"
	"testing"

	"github.com/oernster/ScreenState/internal/product"
)

// FR-047: one agent per signed-in user. FR-054: the second launch stands down.
func TestASecondCopyDoesNotGetTheLock(t *testing.T) {
	name := `Local\` + product.Name + ".Test.ASecondCopy"

	first, held, err := Acquire(name)
	if err != nil || !held {
		t.Fatalf("the first copy did not get the lock: held=%v err=%v", held, err)
	}

	second, held, err := Acquire(name)
	if err != nil {
		t.Fatalf("the second copy answered an error rather than standing down: %v", err)
	}
	if held {
		t.Fatal("two copies hold the lock at once")
	}
	if second != nil {
		t.Fatal("a copy that did not get the lock was given one to release")
	}

	// Letting it go lets a later copy start, which is what makes the agent
	// restartable rather than needing a sign-out.
	if err := first.Release(); err != nil {
		t.Fatalf("releasing: %v", err)
	}
	third, held, err := Acquire(name)
	if err != nil || !held {
		t.Fatalf("a later copy could not start: held=%v err=%v", held, err)
	}
	if err := third.Release(); err != nil {
		t.Fatalf("releasing: %v", err)
	}
}

// The lock is scoped to this user's own session, so two people signed in at
// once each get their own agent (OOS-7).
func TestTheLockIsScopedToThisUsersSession(t *testing.T) {
	t.Parallel()
	if !strings.HasPrefix(Name, `Local\`) {
		t.Fatalf("the lock is named %q, which is not scoped to this session", Name)
	}
	if !strings.Contains(Name, product.Name) {
		t.Fatalf("the lock is named %q, which does not name this product", Name)
	}
}

// A name Windows cannot use is refused rather than quietly letting a second
// agent start.
func TestAnUnusableNameIsRefused(t *testing.T) {
	t.Parallel()
	if _, held, err := Acquire("bad\x00name"); err == nil || held {
		t.Fatalf("held=%v err=%v", held, err)
	}
}
