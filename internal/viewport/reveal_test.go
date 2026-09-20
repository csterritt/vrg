package viewport

import "testing"

// An already-visible target row leaves the viewport unchanged: the
// reveal never scrolls gratuitously, wherever inside the window the
// target sits.
func TestRevealVisibleTargetDoesNotScroll(t *testing.T) {
	for _, tc := range []struct {
		name         string
		rows, height int
		top, target  int
	}{
		{"target on the top row", 100, 10, 0, 0},
		{"target mid-window", 100, 10, 40, 45},
		{"target on the last visible row", 100, 10, 40, 49},
		{"file shorter than the window", 5, 10, 0, 4},
	} {
		v := &Viewport{anchor: Location{Line: tc.top}}
		v.SetLayout(rowsLayout(t, tc.rows), tc.height)
		v.Reveal(tc.target)
		if got := v.Top(); got != tc.top {
			t.Errorf("%s: top = %d, want unchanged %d", tc.name, got, tc.top)
		}
	}
}

// A hidden target lands at zero-based row floor(height/3) of the
// content area, whether it sat below or above the window — the window
// moves, not the target.
func TestRevealPlacesHiddenTargetOneThirdDown(t *testing.T) {
	for _, tc := range []struct {
		name        string
		height      int
		top, target int
		want        int
	}{
		{"below, height 10", 10, 0, 50, 47},     // 50 - 10/3
		{"above, height 10", 10, 60, 20, 17},    // 20 - 3
		{"below, odd height 23", 23, 0, 49, 42}, // 49 - floor(23/3)=7
		{"just below the window", 10, 0, 10, 7},
		{"just above the window", 10, 20, 19, 16},
	} {
		v := &Viewport{anchor: Location{Line: tc.top}}
		v.SetLayout(rowsLayout(t, 1000), tc.height)
		v.Reveal(tc.target)
		if got := v.Top(); got != tc.want {
			t.Errorf("%s: top = %d, want %d so the target sits at row floor(%d/3)",
				tc.name, got, tc.want, tc.height)
		}
	}
}

// At BOF the available content takes precedence over one-third
// placement: the top clamps to the first row instead of putting the
// target at row floor(h/3).
func TestRevealClampsAtBOF(t *testing.T) {
	v := &Viewport{anchor: Location{Line: 50}}
	v.SetLayout(rowsLayout(t, 100), 10)
	v.Reveal(2) // 2 - 3 = -1, clamped to 0
	if got := v.Top(); got != 0 {
		t.Fatalf("reveal near BOF: top = %d, want 0", got)
	}
}

// At EOF the available content takes precedence over one-third
// placement: the top clamps so the last rendered row sits at the
// bottom, and the target lands lower than row floor(h/3).
func TestRevealClampsAtEOF(t *testing.T) {
	v := &Viewport{}
	v.SetLayout(rowsLayout(t, 30), 10)
	v.Reveal(28) // 28 - 3 = 25, clamped to 30 - 10 = 20
	if got := v.Top(); got != 20 {
		t.Fatalf("reveal near EOF: top = %d, want 20", got)
	}
}

// The reveal's starting point is whatever top the viewport already
// holds: a fresh viewport is the first-visit top of the file, while a
// previously scrolled one is the saved position — and a target already
// inside the saved window leaves that saved top in place.
func TestRevealStartsFromHeldTop(t *testing.T) {
	// First visit: the zero-value viewport sits at the top of the file.
	v := &Viewport{}
	v.SetLayout(rowsLayout(t, 100), 10)
	v.Reveal(50)
	if got := v.Top(); got != 47 {
		t.Fatalf("first-visit reveal: top = %d, want 47", got)
	}

	// Revisit: the saved top shows the target already, so it survives.
	v = &Viewport{anchor: Location{Line: 45}}
	v.SetLayout(rowsLayout(t, 100), 10)
	v.Reveal(50) // 50 is inside [45, 55)
	if got := v.Top(); got != 45 {
		t.Fatalf("saved-viewport reveal: top = %d, want the saved 45", got)
	}
}
