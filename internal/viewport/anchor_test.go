package viewport

import (
	"strings"
	"testing"
)

// linesOf returns n one-cell-wide source lines — n rendered rows at any
// positive width — for tests that need a plain extent.
func linesOf(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "x"
	}
	return lines
}

// layoutOf builds the prepared row model over lines at the given text
// width and wrap mode — the swappable value a Viewport installs.
func layoutOf(t *testing.T, textWidth int, wrap bool, lines ...string) *RowModel {
	t.Helper()
	return NewRowModel(RowModelKey{TextWidth: textWidth, Wrap: wrap}, sourceOf(t, lines...))
}

// rowsLayout builds a row model with n rendered rows — one short line
// each — for scroll tests that need a plain extent.
func rowsLayout(t *testing.T, n int) *RowModel {
	t.Helper()
	return layoutOf(t, 10, false, linesOf(n)...)
}

// A width round trip keeps the top on the row holding the anchor's text
// location — not the row with the same former ordinal: narrowing wraps
// the anchored line across more rows and widening lands on the original
// row again.
func TestAnchorSurvivesWidthRoundTrip(t *testing.T) {
	// One long line ahead of many short rows keeps the file taller than
	// the window at every width below.
	lines := append([]string{strings.Repeat("x", 100)}, linesOf(60)...)

	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, true, lines...), 10)
	// Line 0 covers rows 0..9 at width 10; scroll to its row holding
	// cells [70, 80).
	for i := 0; i < 7; i++ {
		v.Down()
	}
	if got := v.Anchor(); got != (Location{Line: 0, Col: 70}) {
		t.Fatalf("anchor after scrolling = %+v, want (0, 70)", got)
	}

	// Narrow to width 4: cell 70 sits in line 0's row covering [68, 72)
	// — rendered row 17, not the former ordinal 7.
	v.SetLayout(layoutOf(t, 4, true, lines...), 10)
	if got := v.Top(); got != 17 {
		t.Fatalf("narrowed top = %d, want the row containing cell 70 (17)", got)
	}
	if got := v.Anchor(); got != (Location{Line: 0, Col: 70}) {
		t.Fatalf("anchor after narrowing = %+v, want unchanged (0, 70)", got)
	}

	// Widen back: the same text location is row 7 again.
	v.SetLayout(layoutOf(t, 10, true, lines...), 10)
	if got := v.Top(); got != 7 {
		t.Fatalf("widened top = %d, want 7", got)
	}
	if got := v.Anchor(); got != (Location{Line: 0, Col: 70}) {
		t.Fatalf("anchor after widening = %+v, want (0, 70)", got)
	}
}

// Wrap off then on preserves the logical column when no scroll, reveal,
// or clamp intervened: off shows the anchor's source line as a single
// row while retaining the column, and on restores the row containing
// that column.
func TestWrapToggleRoundTripKeepsLogicalColumn(t *testing.T) {
	lines := append([]string{strings.Repeat("x", 100)}, linesOf(60)...)

	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, true, lines...), 10)
	for i := 0; i < 7; i++ {
		v.Down()
	}

	// Wrap off: the anchor's source line is one rendered row — row 0 —
	// and the logical column 70 is retained rather than flattened to the
	// line's start.
	v.SetLayout(layoutOf(t, 10, false, lines...), 10)
	if got := v.Top(); got != 0 {
		t.Fatalf("wrap-off top = %d, want 0: the anchor's whole source line", got)
	}
	if got := v.Anchor(); got != (Location{Line: 0, Col: 70}) {
		t.Fatalf("wrap-off discarded the logical column: anchor = %+v, want (0, 70)", got)
	}

	// Wrap back on: the row containing column 70 is the top again.
	v.SetLayout(layoutOf(t, 10, true, lines...), 10)
	if got := v.Top(); got != 7 {
		t.Fatalf("wrap-on top = %d, want the row holding column 70 (7)", got)
	}
	if got := v.Anchor(); got != (Location{Line: 0, Col: 70}) {
		t.Fatalf("anchor after the round trip = %+v, want (0, 70)", got)
	}
}

// User vertical scrolling replaces the anchor with the resulting top
// row's location — in wrap mode the top may be a continuation row, so
// the anchor can land mid-line.
func TestScrollReplacesAnchor(t *testing.T) {
	lines := append([]string{strings.Repeat("x", 50)}, linesOf(60)...)

	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, true, lines...), 5)
	v.Down()
	v.Down()
	if got := v.Anchor(); got != (Location{Line: 0, Col: 20}) {
		t.Fatalf("anchor after two rows of scroll = %+v, want (0, 20)", got)
	}

	// The next three rows cross into the next source line: the anchor
	// becomes that line's first row.
	for i := 0; i < 3; i++ {
		v.Down()
	}
	if got := v.Anchor(); got != (Location{Line: 1, Col: 0}) {
		t.Fatalf("anchor after scrolling onto line 1 = %+v, want (1, 0)", got)
	}
}

// A match reveal that moves the viewport replaces the anchor with the
// resulting top row's location.
func TestMovingRevealReplacesAnchor(t *testing.T) {
	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, true, linesOf(100)...), 10)

	v.Reveal(stopAt(51, 0, 1)) // hidden below: lands at floor(10/3)=3 → top 47
	if got := v.Top(); got != 47 {
		t.Fatalf("reveal top = %d, want 47", got)
	}
	if got := v.Anchor(); got != (Location{Line: 47, Col: 0}) {
		t.Fatalf("anchor after moving reveal = %+v, want (47, 0)", got)
	}
}

// A reveal whose target is already visible neither scrolls nor discards
// a retained logical column.
func TestNoScrollRevealKeepsLogicalColumn(t *testing.T) {
	lines := append([]string{strings.Repeat("x", 50)}, linesOf(60)...)

	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, true, lines...), 10)
	v.Down()
	v.Down() // anchor (0, 20), top row 2

	v.Reveal(stopAt(6, 0, 1)) // target row 9 is inside the window [2, 12): no scroll
	if got := v.Top(); got != 2 {
		t.Fatalf("no-scroll reveal moved the top to %d, want 2", got)
	}
	if got := v.Anchor(); got != (Location{Line: 0, Col: 20}) {
		t.Fatalf("no-scroll reveal discarded the column: anchor = %+v, want (0, 20)", got)
	}
}

// The EOF clamp pulls the effective top up so no avoidable blank rows
// sit below the file, and updates the anchor to the resulting top: a
// later shrink does not restore the old position — the loss is the
// documented behavior.
func TestEOFClampUpdatesAnchorLossy(t *testing.T) {
	m := layoutOf(t, 10, true, linesOf(30)...)
	v := &Viewport{}
	v.SetLayout(m, 10)
	for i := 0; i < 30; i++ {
		v.Down()
	}
	if got := v.Top(); got != 20 {
		t.Fatalf("scrolled to top %d, want the EOF clamp 20 (30-10)", got)
	}

	// The window grows: top 20 would leave avoidable blank rows, so it
	// clamps to 10 and the anchor follows — the position at row 20 is
	// deliberately lost.
	v.SetLayout(m, 20)
	if got := v.Top(); got != 10 {
		t.Fatalf("grown-window top = %d, want 10 (30-20)", got)
	}
	if got := v.Anchor(); got != (Location{Line: 10, Col: 0}) {
		t.Fatalf("anchor after the EOF clamp = %+v, want the clamped top (10, 0)", got)
	}

	// Shrinking back does not restore the old top: the anchor is the
	// clamped row now.
	v.SetLayout(m, 10)
	if got := v.Top(); got != 10 {
		t.Fatalf("shrunk top = %d, want the retained clamp 10, not 20", got)
	}
}
