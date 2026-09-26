package viewport

import (
	"strings"
	"testing"
)

// The logical anchor is a (source line, display-column) location
// independent of wrap width: after a rewrap the effective top is the
// row containing that location, not the row with the same former
// ordinal. A top mid-way through a wrapped line at width 10 — cell
// 150, row 15 — lands on the row covering cell 150 at width 20, row
// 7, and the round trip back restores row 15 exactly.
func TestAnchorRoundTripThroughRewrap(t *testing.T) {
	src := source(t, strings.Repeat("x", 200)+"\n"+strings.Repeat("tail\n", 30), nil)
	var v Viewport
	v.Resize(10, 6)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.SetTop(15) // rendered row 15 begins at cell 150 of line 0

	v.Resize(20, 6)
	v.SetRows(Prepare(src, Key{Width: 20, Wrap: true}))
	if v.Top() != 7 {
		t.Fatalf("top after widening = %d, want 7 — the row containing cell 150", v.Top())
	}

	v.Resize(10, 6)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	if v.Top() != 15 {
		t.Fatalf("top after narrowing back = %d, want 15 — the round trip loses nothing", v.Top())
	}
}

// The same width-independent anchor holds when the top sits on an
// ordinary later line: narrowing packs each line into more rows, so
// the ordinal drifts while the location stays. Here line 0's 200 cells
// wrap into 20 rows at width 10, so tail line i leads at row 19+i.
func TestAnchorRoundTripOnLaterLine(t *testing.T) {
	src := source(t, strings.Repeat("x", 200)+"\n"+strings.Repeat("tail\n", 30), nil)
	var v Viewport
	v.Resize(10, 6)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.SetTop(25) // line 6's lead row: anchor (6, 0)

	v.Resize(40, 6)
	v.SetRows(Prepare(src, Key{Width: 40, Wrap: true}))
	// Line 0 wraps into 5 rows at width 40, so line 6's lead row is 10.
	if v.Top() != 10 {
		t.Fatalf("top after widening = %d, want 10 — line 6's lead row", v.Top())
	}
	if r := v.Visible()[0]; r.Line != 6 {
		t.Fatalf("top row = line %d, want line 6", r.Line)
	}
}

// Turning wrap off shows the anchor's source line as one row but
// retains the logical column; turning wrap back on restores the row
// containing that column. Without column retention the second toggle
// would land on the line's first row.
func TestAnchorRoundTripThroughWrapToggle(t *testing.T) {
	src := source(t, strings.Repeat("x", 200)+"\n"+strings.Repeat("tail\n", 30), nil)
	var v Viewport
	v.Resize(10, 6)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.SetTop(15) // anchor (0, 150)

	v.SetRows(Prepare(src, Key{Width: 10, Wrap: false}))
	if v.Top() != 0 {
		t.Fatalf("top with wrap off = %d, want 0 — line 0 shown as one row", v.Top())
	}
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	if v.Top() != 15 {
		t.Fatalf("top with wrap restored = %d, want 15 — the logical column is retained", v.Top())
	}
}

// User vertical scrolling replaces the anchor with the resulting top
// row's location: after one down from row 15 the anchor is cell 160,
// so a rewrap lands on the row covering cell 160 — not the row that a
// stale anchor would have produced.
func TestScrollReplacesAnchor(t *testing.T) {
	src := source(t, strings.Repeat("x", 200)+"\n"+strings.Repeat("tail\n", 30), nil)
	var v Viewport
	v.Resize(10, 6)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.SetTop(15)
	v.Down() // top 16 → anchor (0, 160)

	v.Resize(20, 6)
	v.SetRows(Prepare(src, Key{Width: 20, Wrap: true}))
	if v.Top() != 8 {
		t.Fatalf("top after rewrap = %d, want 8 — the row containing cell 160", v.Top())
	}
}

// A match reveal that moves the viewport replaces the anchor with the
// resulting top row's location: a subsequent rewrap tracks the
// revealed position, not the pre-reveal one.
func TestMovingRevealReplacesAnchor(t *testing.T) {
	src := source(t, strings.Repeat("x", 200)+"\n"+strings.Repeat("tail\n", 60), nil)
	var v Viewport
	v.Resize(10, 6)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.SetTop(15) // anchor (0, 150)

	// Line 30's lead row is 49; the reveal places it at row
	// floor(6/3) = 2 → top 47, anchor (28, 0).
	if !v.Reveal(Target{Line: 30}) {
		t.Fatal("reveal of a hidden target reported no move")
	}
	if v.Top() != 47 {
		t.Fatalf("top after reveal = %d, want 47", v.Top())
	}

	v.Resize(20, 6)
	v.SetRows(Prepare(src, Key{Width: 20, Wrap: true}))
	// Line 0 wraps into 10 rows at width 20, so line 28's lead row is
	// 37 — not row 7, which the stale (0, 150) anchor would give.
	if v.Top() != 37 {
		t.Fatalf("top after rewrap = %d, want 37 — the reveal replaced the anchor", v.Top())
	}
}

// A no-scroll reveal does not discard a retained logical column: with
// wrap off the anchor's line shows as one row and a reveal of a
// visible target leaves the column alone, so the wrap-on round trip
// still restores the row containing it.
func TestNoScrollRevealKeepsAnchorColumn(t *testing.T) {
	src := source(t, strings.Repeat("x", 200)+"\n"+strings.Repeat("tail\n", 30), nil)
	var v Viewport
	v.Resize(10, 6)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.SetTop(15) // anchor (0, 150)

	v.SetRows(Prepare(src, Key{Width: 10, Wrap: false}))
	if v.Reveal(Target{Line: 0, Cell: 3}) {
		t.Fatal("reveal of a visible target reported a move")
	}
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	if v.Top() != 15 {
		t.Fatalf("top after wrap round trip = %d, want 15 — the column survived the no-scroll reveal", v.Top())
	}
}

// A moving reveal in run-off-edge mode replaces the anchor with the
// resulting top row's location — cell 0, the whole line — so the
// wrap-on round trip lands on the line's first row.
func TestMovingRevealInRunOffEdgeDropsColumn(t *testing.T) {
	src := source(t, strings.Repeat("x", 200)+"\n"+strings.Repeat("tail\n", 60), nil)
	var v Viewport
	v.Resize(10, 6)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.SetTop(15) // anchor (0, 150)

	v.SetRows(Prepare(src, Key{Width: 10, Wrap: false}))
	if !v.Reveal(Target{Line: 40}) {
		t.Fatal("reveal of a hidden target reported no move")
	}
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	// The reveal landed the top at row 38, so the anchor is (38, 0):
	// line 0's 20 rows put line 38's lead row at 57.
	if v.Top() != 57 {
		t.Fatalf("top after wrap round trip = %d, want 57 — the moving reveal replaced the anchor", v.Top())
	}
}

// End-of-file clamping pulls the effective top upward and rewrites the
// anchor to the resulting top — intentionally lossy: a later rewrap
// tracks the clamped location, not the pre-clamp one. Here the top
// sits mid-way through the file's final line, so the widened layout's
// last full page starts before the anchor cell and the anchor moves
// down to the clamped row's start.
func TestEOFClampRewritesAnchorLossy(t *testing.T) {
	// 30 short lines then a 200-cell line: at width 10 the long line's
	// rows are 30..49 and the last full page is top 44.
	src := source(t, strings.Repeat("tail\n", 30)+strings.Repeat("x", 200)+"\n", nil)
	var v Viewport
	v.Resize(10, 6)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.SetTop(44) // anchor (30, 140) — mid-way through the last line

	v.Resize(20, 6)
	v.SetRows(Prepare(src, Key{Width: 20, Wrap: true}))
	// Width 20 leaves 40 rows, so the last full page is 34 — the row
	// covering cell 80 of the long line. The anchor rewrites to
	// (30, 80): the clamped top's location.
	if v.Top() != 34 {
		t.Fatalf("top after widening = %d, want the clamped 34", v.Top())
	}

	v.Resize(10, 6)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	// The loss is permanent: the row containing (30, 80) is 38, not
	// the pre-clamp 44 a retained anchor would restore.
	if v.Top() != 38 {
		t.Fatalf("top after narrowing back = %d, want 38 — the clamped anchor is kept", v.Top())
	}
}
