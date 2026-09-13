package viewport_test

import (
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/viewport"
)

// --- Logical anchor tests (Issue #17) ---

// TestAnchorWidthRoundTrip verifies that the logical anchor is
// width-independent: after a rewrap at a different text width, the
// effective top row is the row containing the anchor's text location,
// not the row with the same former ordinal. A 200-char line at width
// 10 has 20 rows; column 25 is in row 2. At width 20 it has 10 rows;
// column 25 is in row 1. The effective top must move from 2 to 1, not
// stay at the old ordinal 2.
func TestAnchorWidthRoundTrip(t *testing.T) {
	display := makeLongString(200)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})

	// Build at width 10: 20 rows. Column 25 → row 2 (cells 20-29).
	key10 := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model10 := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key10)
	v := viewport.New(model10, 10) // contentHeight = 9, maxOffset = 11

	v.SetAnchor(viewport.Anchor{LineIndex: 0, Column: 25})
	if v.Offset() != 2 {
		t.Fatalf("after SetAnchor(0, 25), Offset = %d, want 2 (row containing column 25 at width 10)", v.Offset())
	}

	// Rewrap at width 20: 10 rows. Column 25 → row 1 (cells 20-39).
	key20 := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 20, WrapMode: viewport.WrapOn}
	model20 := viewport.BuildRowModel(buf, 20, viewport.WrapOn, key20)
	v.SetRows(model20)
	if v.Offset() != 1 {
		t.Fatalf("after rewrap to width 20, Offset = %d, want 1 (row containing column 25 at width 20, not old ordinal 2)", v.Offset())
	}
}

// TestAnchorWrapToggleRoundTrip verifies that turning wrap off then on
// preserves the logical column when no scroll, reveal, or clamp
// intervened. A 200-char line at width 10 has 20 rows; column 15 is in
// row 1. Turning wrap off makes it one row (row 0) but retains the
// column. Turning wrap on again restores row 1 as the effective top.
func TestAnchorWrapToggleRoundTrip(t *testing.T) {
	display := makeLongString(200)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})

	keyOn := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	modelOn := viewport.BuildRowModel(buf, 10, viewport.WrapOn, keyOn)
	v := viewport.New(modelOn, 10) // contentHeight = 9, maxOffset = 11

	v.SetAnchor(viewport.Anchor{LineIndex: 0, Column: 15})
	if v.Offset() != 1 {
		t.Fatalf("after SetAnchor(0, 15), Offset = %d, want 1", v.Offset())
	}

	// Toggle wrap off: one row per line. Column 15 is retained.
	keyOff := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	modelOff := viewport.BuildRowModel(buf, 10, viewport.WrapOff, keyOff)
	v.SetRows(modelOff)
	if v.Offset() != 0 {
		t.Fatalf("after wrap off, Offset = %d, want 0 (one row per line)", v.Offset())
	}
	if a := v.Anchor(); a.Column != 15 {
		t.Fatalf("after wrap off, Anchor.Column = %d, want 15 (retained logical column)", a.Column)
	}

	// Toggle wrap on: 20 rows. Column 15 → row 1.
	modelOn2 := viewport.BuildRowModel(buf, 10, viewport.WrapOn, keyOn)
	v.SetRows(modelOn2)
	if v.Offset() != 1 {
		t.Fatalf("after wrap on, Offset = %d, want 1 (column 15 restored to row 1)", v.Offset())
	}
}

// TestAnchorReplacedByScroll verifies that user vertical scrolling
// replaces the logical anchor with the resulting top row's location.
// Two 200-char lines at width 10 produce 40 rows. Scrolling from row 0
// to row 2 replaces the anchor with (line 0, column 20).
func TestAnchorReplacedByScroll(t *testing.T) {
	lines := []filebuffer.Line{
		makeLineWithClusters(1, makeLongString(200)),
		makeLineWithClusters(2, makeLongString(200)),
	}
	buf := makeBufFromLines(lines)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	v := viewport.New(model, 10) // contentHeight = 9, maxOffset = 31

	v.SetAnchor(viewport.Anchor{LineIndex: 0, Column: 0})
	v.ScrollDown()
	v.ScrollDown()
	if v.Offset() != 2 {
		t.Fatalf("after 2 ScrollDown, Offset = %d, want 2", v.Offset())
	}
	// Row 2 covers cells 20-29 of line 0.
	a := v.Anchor()
	if a.LineIndex != 0 || a.Column != 20 {
		t.Fatalf("after scroll, Anchor = (%d, %d), want (0, 20) (replaced by top row location)", a.LineIndex, a.Column)
	}
}

// TestAnchorReplacedByReveal verifies that a match reveal that moves
// the viewport replaces the logical anchor with the new top row's
// location. A 200-char line at width 10 has 20 rows. The anchor starts
// at (0, 0). Revealing row 15 (hidden) moves the offset; the anchor is
// replaced with the new top row's location.
func TestAnchorReplacedByReveal(t *testing.T) {
	display := makeLongString(200)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	v := viewport.New(model, 10) // contentHeight = 9, maxOffset = 11

	v.SetAnchor(viewport.Anchor{LineIndex: 0, Column: 0})
	// Reveal row 15 (hidden from [0, 9)). offset = 15 - 3 = 12, clamped to 11.
	v.Reveal(15)
	if v.Offset() != 11 {
		t.Fatalf("after Reveal(15), Offset = %d, want 11 (clamped)", v.Offset())
	}
	// Row 11 covers cells 110-119 of line 0.
	a := v.Anchor()
	if a.LineIndex != 0 || a.Column != 110 {
		t.Fatalf("after reveal, Anchor = (%d, %d), want (0, 110) (replaced by new top row location)", a.LineIndex, a.Column)
	}
}

// TestAnchorNoScrollRevealRetainsAnchor verifies that a no-scroll
// reveal (target already visible) does not discard a retained logical
// column. The anchor stays at its set position.
func TestAnchorNoScrollRevealRetainsAnchor(t *testing.T) {
	display := makeLongString(200)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	v := viewport.New(model, 10) // contentHeight = 9, maxOffset = 11

	v.SetAnchor(viewport.Anchor{LineIndex: 0, Column: 25}) // → row 2
	if v.Offset() != 2 {
		t.Fatalf("after SetAnchor(0, 25), Offset = %d, want 2", v.Offset())
	}
	// Reveal row 5 (visible from [2, 11)). No scroll.
	v.Reveal(5)
	if v.Offset() != 2 {
		t.Fatalf("after no-scroll Reveal(5), Offset = %d, want 2 (no scroll)", v.Offset())
	}
	a := v.Anchor()
	if a.LineIndex != 0 || a.Column != 25 {
		t.Fatalf("after no-scroll reveal, Anchor = (%d, %d), want (0, 25) (retained)", a.LineIndex, a.Column)
	}
}

// TestAnchorEOFClampLoss verifies that EOF clamping pulls the effective
// top upward and updates the anchor to that top, with the intentional
// loss that a subsequent shrink need not restore the old top. A
// 200-char line at width 10 has 20 rows. At contentHeight 9, the anchor
// at (0, 100) → row 10. Enlarging the panel to contentHeight 15 clamps
// offset 10 to maxOffset 5, updating the anchor to (0, 50). Shrinking
// back to contentHeight 9 recomputes from the anchor → row 5, not 10.
func TestAnchorEOFClampLoss(t *testing.T) {
	display := makeLongString(200)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	v := viewport.New(model, 10) // contentHeight = 9, maxOffset = 11

	v.SetAnchor(viewport.Anchor{LineIndex: 0, Column: 100}) // → row 10
	if v.Offset() != 10 {
		t.Fatalf("after SetAnchor(0, 100), Offset = %d, want 10", v.Offset())
	}

	// Enlarge panel: contentHeight 15, maxOffset 5. Offset clamped 10 → 5.
	v.SetPanelHeight(16) // contentHeight = 15
	if v.Offset() != 5 {
		t.Fatalf("after enlarge, Offset = %d, want 5 (EOF clamp)", v.Offset())
	}
	// Anchor updated to row 5's location: (0, 50).
	a := v.Anchor()
	if a.LineIndex != 0 || a.Column != 50 {
		t.Fatalf("after EOF clamp, Anchor = (%d, %d), want (0, 50) (updated to clamped top)", a.LineIndex, a.Column)
	}

	// Shrink back: contentHeight 9, maxOffset 11. Offset recomputed from
	// anchor (0, 50) → row 5, not 10. This is the intentional loss.
	v.SetPanelHeight(10) // contentHeight = 9
	if v.Offset() != 5 {
		t.Fatalf("after shrink back, Offset = %d, want 5 (lossy: anchor not restored to 10)", v.Offset())
	}
}

// TestAnchorScrollAcrossLineBoundary verifies that scrolling across a
// source-line boundary replaces the anchor with the new line's start.
// Two 200-char lines at width 10 produce 40 rows. Scrolling from row 0
// to row 20 (line 1, row 0) replaces the anchor with (line 1, column 0).
func TestAnchorScrollAcrossLineBoundary(t *testing.T) {
	lines := []filebuffer.Line{
		makeLineWithClusters(1, makeLongString(200)),
		makeLineWithClusters(2, makeLongString(200)),
	}
	buf := makeBufFromLines(lines)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	v := viewport.New(model, 10) // contentHeight = 9, maxOffset = 31

	v.SetAnchor(viewport.Anchor{LineIndex: 0, Column: 0})
	// Scroll to row 20 (line 1, cells 0-9).
	for i := 0; i < 20; i++ {
		v.ScrollDown()
	}
	if v.Offset() != 20 {
		t.Fatalf("after 20 ScrollDown, Offset = %d, want 20", v.Offset())
	}
	a := v.Anchor()
	if a.LineIndex != 1 || a.Column != 0 {
		t.Fatalf("after scroll to line 1, Anchor = (%d, %d), want (1, 0)", a.LineIndex, a.Column)
	}
}

// TestAnchorRoundTripAfterScroll verifies that after scrolling (which
// replaces the anchor), a rewrap recomputes from the new anchor, not
// the old one. Scroll to row 2 (anchor (0, 20)), then rewrap at width
// 20. Column 20 is in row 1 at width 20.
func TestAnchorRoundTripAfterScroll(t *testing.T) {
	display := makeLongString(200)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key10 := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model10 := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key10)
	v := viewport.New(model10, 10) // contentHeight = 9, maxOffset = 11

	v.SetAnchor(viewport.Anchor{LineIndex: 0, Column: 0})
	v.ScrollDown()
	v.ScrollDown() // → row 2, anchor (0, 20)
	if v.Offset() != 2 {
		t.Fatalf("after 2 ScrollDown, Offset = %d, want 2", v.Offset())
	}

	// Rewrap at width 20: 10 rows. Column 20 → row 1 (cells 20-39).
	key20 := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 20, WrapMode: viewport.WrapOn}
	model20 := viewport.BuildRowModel(buf, 20, viewport.WrapOn, key20)
	v.SetRows(model20)
	if v.Offset() != 1 {
		t.Fatalf("after rewrap to width 20, Offset = %d, want 1 (anchor (0, 20) → row 1)", v.Offset())
	}
}

// TestAnchorDefaultIsTopOfFile verifies that a new viewport defaults to
// anchor (0, 0) — the top of the file.
func TestAnchorDefaultIsTopOfFile(t *testing.T) {
	line := makeLineWithClusters(1, "hello")
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	v := viewport.New(model, 10)
	a := v.Anchor()
	if a.LineIndex != 0 || a.Column != 0 {
		t.Fatalf("default Anchor = (%d, %d), want (0, 0) (top of file)", a.LineIndex, a.Column)
	}
}

// TestRowFromCell verifies that RowModel.RowFromCell maps a source line
// and display-column offset to the rendered row containing that column.
func TestRowFromCell(t *testing.T) {
	display := makeLongString(60)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	// 60 chars at width 10 → 6 rows.
	// Column 0 → row 0, column 9 → row 0, column 10 → row 1, column 25 → row 2.
	cases := []struct {
		line, col, want int
	}{
		{0, 0, 0},
		{0, 9, 0},
		{0, 10, 1},
		{0, 25, 2},
		{0, 59, 5},
	}
	for _, tc := range cases {
		got := model.RowFromCell(tc.line, tc.col)
		if got != tc.want {
			t.Fatalf("RowFromCell(%d, %d) = %d, want %d", tc.line, tc.col, got, tc.want)
		}
	}
}

// TestRowAnchor verifies that RowModel.RowAnchor returns the source
// line and display-column offset for a given rendered row.
func TestRowAnchor(t *testing.T) {
	display := makeLongString(60)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	// 60 chars at width 10 → 6 rows.
	// Row 0 → (line 0, col 0), row 2 → (line 0, col 20), row 5 → (line 0, col 50).
	cases := []struct {
		row        int
		wantLine   int
		wantColumn int
	}{
		{0, 0, 0},
		{2, 0, 20},
		{5, 0, 50},
	}
	for _, tc := range cases {
		a := model.RowAnchor(tc.row)
		if a.LineIndex != tc.wantLine || a.Column != tc.wantColumn {
			t.Fatalf("RowAnchor(%d) = (%d, %d), want (%d, %d)", tc.row, a.LineIndex, a.Column, tc.wantLine, tc.wantColumn)
		}
	}
}

// TestRowAnchorMultiLine verifies RowAnchor across multiple source
// lines. Two 30-char lines at width 10 produce 6 rows. Row 3 is line 1,
// column 0.
func TestRowAnchorMultiLine(t *testing.T) {
	lines := []filebuffer.Line{
		makeLineWithClusters(1, makeLongString(30)),
		makeLineWithClusters(2, makeLongString(30)),
	}
	buf := makeBufFromLines(lines)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	a := model.RowAnchor(3)
	if a.LineIndex != 1 || a.Column != 0 {
		t.Fatalf("RowAnchor(3) = (%d, %d), want (1, 0) (line 1 start)", a.LineIndex, a.Column)
	}
}

// TestRowFromCellRunOffEdge verifies that RowFromCell in run-off-edge
// mode maps every column of a line to the same row (one row per line).
func TestRowFromCellRunOffEdge(t *testing.T) {
	display := makeLongString(60)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOff, key)
	// In run-off-edge mode, line 0 is row 0 regardless of column.
	if row := model.RowFromCell(0, 25); row != 0 {
		t.Fatalf("RowFromCell(0, 25) in run-off-edge = %d, want 0 (one row per line)", row)
	}
}

// TestRowAnchorRunOffEdge verifies that RowAnchor in run-off-edge mode
// returns column 0 for every row (each row is a full source line).
func TestRowAnchorRunOffEdge(t *testing.T) {
	lines := []filebuffer.Line{
		makeLineWithClusters(1, makeLongString(60)),
		makeLineWithClusters(2, "short"),
	}
	buf := makeBufFromLines(lines)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOff, key)
	a := model.RowAnchor(0)
	if a.LineIndex != 0 || a.Column != 0 {
		t.Fatalf("RowAnchor(0) in run-off-edge = (%d, %d), want (0, 0)", a.LineIndex, a.Column)
	}
	a = model.RowAnchor(1)
	if a.LineIndex != 1 || a.Column != 0 {
		t.Fatalf("RowAnchor(1) in run-off-edge = (%d, %d), want (1, 0)", a.LineIndex, a.Column)
	}
}
