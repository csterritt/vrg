package viewport

import "testing"

// The scroll units of the PRD's scroll-unit bullet: up/down move one
// rendered row, u/d move a half page, and page up/page down move a full
// page of the content height.
func TestScrollUnits(t *testing.T) {
	v := &Viewport{}
	v.SetExtent(100, 10)

	v.Down()
	if got := v.Top(); got != 1 {
		t.Fatalf("down: top = %d, want 1", got)
	}
	v.Down()
	if got := v.Top(); got != 2 {
		t.Fatalf("down again: top = %d, want 2", got)
	}
	v.Up()
	if got := v.Top(); got != 1 {
		t.Fatalf("up: top = %d, want 1", got)
	}
	v.HalfDown()
	if got := v.Top(); got != 6 {
		t.Fatalf("half page down: top = %d, want 6 (1 + 10/2)", got)
	}
	v.HalfUp()
	if got := v.Top(); got != 1 {
		t.Fatalf("half page up: top = %d, want 1", got)
	}
	v.PageDown()
	if got := v.Top(); got != 11 {
		t.Fatalf("page down: top = %d, want 11 (1 + content height 10)", got)
	}
	v.PageUp()
	if got := v.Top(); got != 1 {
		t.Fatalf("page up: top = %d, want 1", got)
	}
}

// The half-page unit is max(1, floor(h/2)) — odd heights round down and
// a degenerate height still scrolls by one row rather than freezing.
func TestHalfPageUnitByHeight(t *testing.T) {
	for _, tc := range []struct {
		height int
		want   int
	}{
		{1, 1}, // floor(1/2)=0, raised to 1
		{2, 1},
		{3, 1}, // odd: floor(3/2)=1
		{4, 2},
		{9, 4}, // odd: floor(9/2)=4
		{10, 5},
		{23, 11}, // odd: floor(23/2)=11
	} {
		v := &Viewport{}
		v.SetExtent(1000, tc.height)
		v.HalfDown()
		if got := v.Top(); got != tc.want {
			t.Errorf("height %d: half page down moved to top %d, want %d", tc.height, got, tc.want)
		}
		v.HalfUp()
		if got := v.Top(); got != 0 {
			t.Errorf("height %d: half page up returned to top %d, want 0", tc.height, got)
		}
	}
}

// A full page is exactly the content height: page down then page up
// returns to the origin on a long file.
func TestPageUnitIsContentHeight(t *testing.T) {
	v := &Viewport{}
	v.SetExtent(100, 7)
	v.PageDown()
	if got := v.Top(); got != 7 {
		t.Fatalf("page down: top = %d, want 7", got)
	}
	v.PageDown()
	if got := v.Top(); got != 14 {
		t.Fatalf("second page down: top = %d, want 14", got)
	}
	v.PageUp()
	if got := v.Top(); got != 7 {
		t.Fatalf("page up: top = %d, want 7", got)
	}
}

// Scrolling up at the beginning of the file is a no-op for every
// upward unit: the top row never goes below zero.
func TestScrollClampsAtBOF(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  func(*Viewport)
	}{
		{"up", (*Viewport).Up},
		{"u", (*Viewport).HalfUp},
		{"page up", (*Viewport).PageUp},
	} {
		v := &Viewport{}
		v.SetExtent(50, 10)
		tc.act(v)
		if got := v.Top(); got != 0 {
			t.Errorf("%s at BOF: top = %d, want 0", tc.name, got)
		}
	}
}

// Scrolling down past the end clamps so the last rendered row sits at
// the bottom of the panel: no avoidable blank rows below EOF.
func TestScrollClampsAtEOF(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  func(*Viewport)
	}{
		{"down", (*Viewport).Down},
		{"d", (*Viewport).HalfDown},
		{"page down", (*Viewport).PageDown},
	} {
		v := &Viewport{}
		v.SetExtent(25, 10)
		for i := 0; i < 30; i++ {
			tc.act(v)
		}
		if got := v.Top(); got != 15 {
			t.Errorf("%s past EOF: top = %d, want 15 (25 rows - height 10)", tc.name, got)
		}
	}
}

// A file shorter than or exactly as tall as the viewport cannot scroll
// at all; its unused rows are left naturally rather than clamped away.
func TestShortFileNeverScrolls(t *testing.T) {
	for _, rows := range []int{0, 1, 9, 10} {
		v := &Viewport{}
		v.SetExtent(rows, 10)
		v.Down()
		v.HalfDown()
		v.PageDown()
		if got := v.Top(); got != 0 {
			t.Errorf("file of %d rows in height 10: top = %d, want 0", rows, got)
		}
	}
}

// Shrinking the extent re-clamps the top: the lossy end-of-file clamp
// pulls the viewport up so no avoidable blank rows remain.
func TestShrinkReclampsTop(t *testing.T) {
	v := &Viewport{}
	v.SetExtent(50, 10)
	for i := 0; i < 5; i++ {
		v.PageDown()
	}
	if got := v.Top(); got != 40 {
		t.Fatalf("scrolled to top %d, want 40", got)
	}
	v.SetExtent(12, 10)
	if got := v.Top(); got != 2 {
		t.Fatalf("after shrink: top = %d, want 2 (12 rows - height 10)", got)
	}
}
