package viewport_test

import (
	"fmt"
	"strings"
	"testing"

	"vrg/internal/viewport"
)

// seqRows is a Model stand-in for scroll and clamp tests: one rendered
// row per source line, so a row index and its anchor coincide.
type seqRows int

func (s seqRows) Len() int { return int(s) }

func (s seqRows) AnchorAt(row int) viewport.Anchor {
	return viewport.Anchor{Line: int64(row + 1)}
}

func (s seqRows) RowOf(a viewport.Anchor) int {
	row := int(a.Line) - 1
	if row < 0 {
		row = 0
	}
	if row >= int(s) {
		row = int(s) - 1
	}
	return row
}

// numberedLines is a fixture of n short lines "line i".
func numberedLines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

// The logical anchor is width-independent: after a rewrap the
// effective top is the row containing the anchor's (line, cell)
// location — not the row with the same former ordinal — and a return
// to the original width restores the original top row.
func TestAnchorRewrapKeepsTextLocation(t *testing.T) {
	// A 95-cell line wraps at width 10 into rows [0,10)…[90,95);
	// scrolling to the row starting at cell 50 anchors at (1, 50).
	buf := loadBuffer(t, strings.Repeat("x", 95)+"\ntail\n")
	at10 := viewport.Prepare(buf, viewport.Key{TextWidth: 10, Wrap: true})
	at7 := viewport.Prepare(buf, viewport.Key{TextWidth: 7, Wrap: true})
	var v viewport.Viewport
	v.Scroll(5, at10, 4)
	if got := v.Anchor(); got != (viewport.Anchor{Line: 1, Cell: 50}) {
		t.Fatalf("anchor = %+v, want {Line 1, Cell 50}", got)
	}

	// At width 7 the same ordinal would be row 5 covering cells
	// [35,42); the anchor's cell 50 lives on row 7 — [49,56).
	v.Restore(at7, 4)
	if v.Top() != 7 {
		t.Fatalf("top after rewrap = %d, want 7 — the row containing cell 50, not ordinal 5", v.Top())
	}
	if got := v.Anchor(); got != (viewport.Anchor{Line: 1, Cell: 50}) {
		t.Fatalf("anchor after rewrap = %+v, want the retained {1, 50}", got)
	}

	// Back at width 10 the top is the original row again.
	v.Restore(at10, 4)
	if v.Top() != 5 {
		t.Fatalf("top after the round trip = %d, want 5", v.Top())
	}
}

// Turning wrap off shows the anchor's source line as one row but
// retains the logical column; turning wrap back on restores the row
// containing that column.
func TestAnchorSurvivesWrapToggle(t *testing.T) {
	buf := loadBuffer(t, strings.Repeat("x", 95)+"\ntail\n")
	on := viewport.Prepare(buf, viewport.Key{TextWidth: 10, Wrap: true})
	off := viewport.Prepare(buf, viewport.Key{TextWidth: 10, Wrap: false})
	var v viewport.Viewport
	v.Scroll(5, on, 4) // top row 5 covers cells [50,60): anchor (1, 50)

	v.Restore(off, 4)
	if v.Top() != 0 {
		t.Fatalf("wrap-off top = %d, want 0 — the line's single row", v.Top())
	}
	if got := v.Anchor(); got != (viewport.Anchor{Line: 1, Cell: 50}) {
		t.Fatalf("wrap-off anchor = %+v, want the retained column {1, 50}", got)
	}

	v.Restore(on, 4)
	if v.Top() != 5 {
		t.Fatalf("wrap-on top = %d, want 5 — the row containing cell 50", v.Top())
	}
}

// User vertical scrolling replaces the anchor with the resulting top
// row's location: the wrapped continuation row's start cell, or the
// next line's first cell once the top crosses a line boundary.
func TestScrollReplacesAnchor(t *testing.T) {
	buf := loadBuffer(t, strings.Repeat("x", 95)+"\ntail\n"+numberedLines(30))
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 10, Wrap: true})
	var v viewport.Viewport

	v.Scroll(5, rows, 4)
	if got := v.Anchor(); got != (viewport.Anchor{Line: 1, Cell: 50}) {
		t.Fatalf("anchor = %+v, want {1, 50}", got)
	}
	v.Scroll(1, rows, 4)
	if got := v.Anchor(); got != (viewport.Anchor{Line: 1, Cell: 60}) {
		t.Fatalf("anchor = %+v, want {1, 60}", got)
	}
	// Row 10 is the second line's first row.
	v.Scroll(4, rows, 4)
	if got := v.Anchor(); got != (viewport.Anchor{Line: 2, Cell: 0}) {
		t.Fatalf("anchor = %+v, want {2, 0}", got)
	}
}

// A match reveal that moves the viewport replaces the anchor with the
// resulting top row's location, just like a manual scroll.
func TestMovingRevealReplacesAnchor(t *testing.T) {
	buf := loadBuffer(t, numberedLines(100))
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 80, Wrap: true})
	var v viewport.Viewport
	v.Scroll(10, rows, 10)
	if got := v.Anchor(); got != (viewport.Anchor{Line: 11, Cell: 0}) {
		t.Fatalf("anchor = %+v, want {11, 0}", got)
	}
	if moved := v.Reveal(50, rows, 10); !moved {
		t.Fatal("hidden target reported no move")
	}
	// Target row 50 lands at floor(10/3) = 3: the top moved to 47.
	if v.Top() != 47 {
		t.Fatalf("top = %d, want 47", v.Top())
	}
	if got := v.Anchor(); got != (viewport.Anchor{Line: 48, Cell: 0}) {
		t.Fatalf("anchor = %+v, want the new top's {48, 0}", got)
	}
}

// A no-scroll reveal — the target row already visible — leaves a
// retained logical column intact: wrap toggling afterwards still
// restores the row containing it.
func TestNoScrollRevealRetainsLogicalColumn(t *testing.T) {
	buf := loadBuffer(t, strings.Repeat("x", 95)+"\n"+numberedLines(10))
	on := viewport.Prepare(buf, viewport.Key{TextWidth: 10, Wrap: true})
	off := viewport.Prepare(buf, viewport.Key{TextWidth: 10, Wrap: false})
	var v viewport.Viewport
	v.Scroll(5, on, 4) // anchor (1, 50)
	v.Restore(off, 4)  // top 0: the whole line as one row, column retained

	// Row 1 — the second source line — is already visible from top 0
	// at height 4, so the reveal does not scroll.
	if moved := v.Reveal(1, off, 4); moved {
		t.Fatal("visible target reported a move")
	}
	if got := v.Anchor(); got != (viewport.Anchor{Line: 1, Cell: 50}) {
		t.Fatalf("no-scroll reveal discarded the retained column: anchor = %+v, want {1, 50}", got)
	}

	v.Restore(on, 4)
	if v.Top() != 5 {
		t.Fatalf("wrap-on top = %d, want 5 — the retained column still anchors", v.Top())
	}
}

// The deliberate EOF clamp: growing the content height past the file's
// end pulls the effective top up and rewrites the anchor to that top.
// Shrinking back does not restore the old position — the loss is
// documented and intentional.
func TestEOFClampUpdatesAnchorLossy(t *testing.T) {
	buf := loadBuffer(t, numberedLines(100))
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 80, Wrap: true})
	var v viewport.Viewport
	v.Scroll(90, rows, 10) // top 90 = MaxTop(100, 10): anchor (91, 0)

	v.Clamp(rows, 30) // taller panel: tops past 70 would leave blanks
	if v.Top() != 70 {
		t.Fatalf("clamped top = %d, want 70", v.Top())
	}
	if got := v.Anchor(); got != (viewport.Anchor{Line: 71, Cell: 0}) {
		t.Fatalf("anchor = %+v, want the clamped top's {71, 0}", got)
	}

	// Shrinking back leaves the anchor where the clamp put it: the
	// pre-clamp top is gone for good.
	v.Clamp(rows, 10)
	if v.Top() != 70 {
		t.Fatalf("top after shrinking back = %d, want the lossy 70, not 90", v.Top())
	}
}

// The same loss applies when a rewrap's restore lands the anchor's row
// past the last valid top: the clamp pulls the top up and the anchor
// follows.
func TestRestoreEOFClampUpdatesAnchor(t *testing.T) {
	buf := loadBuffer(t, numberedLines(30))
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 80, Wrap: true})
	var v viewport.Viewport
	v.Scroll(20, rows, 10) // top 20 = MaxTop(30, 10): anchor (21, 0)

	// A rewrap at a taller content height clamps the restored top.
	v.Restore(rows, 25) // MaxTop(30, 25) = 5
	if v.Top() != 5 {
		t.Fatalf("restored top = %d, want the EOF clamp 5", v.Top())
	}
	if got := v.Anchor(); got != (viewport.Anchor{Line: 6, Cell: 0}) {
		t.Fatalf("anchor = %+v, want the clamped top's {6, 0}", got)
	}
}
