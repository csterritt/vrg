package viewport

import (
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
)

// cellsText joins the display text of a clipped window of cells.
func cellsText(cells []safepresentation.Cell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteString(c.Text)
	}
	return b.String()
}

// tenAfter returns one long line followed by n ten-cell lines — the
// mixed-width fixture whose visible set trades a 299-column extent for
// a 9-column one when the long line scrolls out.
func tenAfter(long int, n int) []string {
	lines := []string{strings.Repeat("x", long)}
	for i := 0; i < n; i++ {
		lines = append(lines, strings.Repeat("y", 10))
	}
	return lines
}

// The pan units: . and , move one column, > and < move ten, and ] and
// [ move half the text width — max(1, floor(text width / 2)).
func TestPanUnits(t *testing.T) {
	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, false, strings.Repeat("x", 100)), 5)

	v.Pan(1) // .
	if got := v.Offset(); got != 1 {
		t.Fatalf("one column right: offset = %d, want 1", got)
	}
	v.Pan(-1) // ,
	if got := v.Offset(); got != 0 {
		t.Fatalf("one column left: offset = %d, want 0", got)
	}
	v.Pan(10) // >
	if got := v.Offset(); got != 10 {
		t.Fatalf("ten columns right: offset = %d, want 10", got)
	}
	v.Pan(-10) // <
	if got := v.Offset(); got != 0 {
		t.Fatalf("ten columns left: offset = %d, want 0", got)
	}
	v.Pan(v.HalfPan()) // ] — floor(10/2) = 5
	if got := v.Offset(); got != 5 {
		t.Fatalf("half width right: offset = %d, want 5", got)
	}
	v.Pan(-v.HalfPan()) // [
	if got := v.Offset(); got != 0 {
		t.Fatalf("half width left: offset = %d, want 0", got)
	}
}

// The half-screen pan unit is max(1, floor(text width / 2)): odd
// widths round down and a degenerate width still pans by one column.
func TestHalfPanUnitByTextWidth(t *testing.T) {
	for _, tc := range []struct {
		width int
		want  int
	}{
		{1, 1}, {2, 1},
		{3, 1}, // odd: floor(3/2)=1
		{4, 2},
		{9, 4}, // odd: floor(9/2)=4
		{10, 5},
		{23, 11}, // odd: floor(23/2)=11
	} {
		v := &Viewport{}
		v.SetLayout(layoutOf(t, tc.width, false, strings.Repeat("x", 200)), 5)
		if got := v.HalfPan(); got != tc.want {
			t.Errorf("text width %d: half pan unit = %d, want %d", tc.width, got, tc.want)
		}
	}
}

// The offset clamps to [0, S]: a line of thirty single-cell clusters
// pans no further right than column 29 and never below zero.
func TestPanClampsToPaintableMaximum(t *testing.T) {
	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, false, strings.Repeat("x", 30)), 5)
	v.Pan(1000)
	if got := v.Offset(); got != 29 {
		t.Fatalf("pan past the maximum: offset = %d, want 29 (E-1)", got)
	}
	v.Pan(-1000)
	if got := v.Offset(); got != 0 {
		t.Fatalf("pan below zero: offset = %d, want 0", got)
	}
}

// Panning is a strict no-op in wrap mode: the offset is left
// untouched — not even re-clamped — because it is run-off-edge display
// state retained for the next re-entry.
func TestPanNoopInWrapMode(t *testing.T) {
	src := sourceOf(t, strings.Repeat("x", 100))
	v := &Viewport{}
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 10}, src), 5)
	v.Pan(10)
	if got := v.Offset(); got != 10 {
		t.Fatalf("setup pan: offset = %d, want 10", got)
	}

	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 10, Wrap: true}, src), 5)
	for _, n := range []int{1, -1, 10, -10, v.HalfPan(), -v.HalfPan()} {
		v.Pan(n)
	}
	if got := v.Offset(); got != 10 {
		t.Fatalf("pans in wrap mode changed the offset to %d, want the retained 10", got)
	}
}

// The offset is separate state retained through wrap toggles: a round
// trip that changes neither the visible set nor the text width leaves
// it untouched.
func TestPanOffsetRetainedThroughWrapToggle(t *testing.T) {
	src := sourceOf(t, strings.Repeat("x", 100), "short")
	v := &Viewport{}
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 10}, src), 5)
	v.Pan(10)

	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 10, Wrap: true}, src), 5)
	if got := v.Offset(); got != 10 {
		t.Fatalf("offset while wrapped = %d, want the retained 10", got)
	}
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 10}, src), 5)
	if got := v.Offset(); got != 10 {
		t.Fatalf("offset after the wrap round trip = %d, want the retained 10", got)
	}
}

// The re-entry clamp runs on every return to run-off-edge mode, not
// only after a width change: when the visible set changed while
// wrapped, the offset clamps against the new visible rows' maximum.
func TestPanReentryClampsAfterWrapScroll(t *testing.T) {
	src := sourceOf(t, tenAfter(300, 20)...)
	v := &Viewport{}
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 10}, src), 3)
	v.Pan(1000)
	if got := v.Offset(); got != 299 {
		t.Fatalf("setup pan: offset = %d, want 299", got)
	}

	// Wrapped at width 10 the long line occupies rows 0..29; scrolling
	// past it lands the anchor on a short line. The offset stays
	// retained at 299 while wrapped.
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 10, Wrap: true}, src), 3)
	for i := 0; i < 30; i++ {
		v.Down()
	}
	if got := v.Anchor(); got != (Location{Line: 1}) {
		t.Fatalf("anchor after wrap scroll = %+v, want line 1", got)
	}
	if got := v.Offset(); got != 299 {
		t.Fatalf("offset while wrapped = %d, want the retained 299", got)
	}

	// Re-entry clamps against the new visible rows — all ten-cell
	// lines — so the stored 299 falls to 9 rather than surviving.
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 10}, src), 3)
	if got := v.Offset(); got != 9 {
		t.Fatalf("offset after re-entry = %d, want the re-clamped 9", got)
	}
	v.Pan(1) // every pan re-evaluates the current maximum: still 9
	if got := v.Offset(); got != 9 {
		t.Fatalf("pan after re-entry = %d, want the clamped 9", got)
	}
}

// A width change while wrapped is the other re-entry clamp: shrinking
// the text width below a trailing wide cluster's width moves the
// maximum to the last fitting cluster's start.
func TestPanReentryClampsAfterWidthChange(t *testing.T) {
	src := sourceOf(t, "abc世", "tail")
	v := &Viewport{}
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 2}, src), 5)
	v.Pan(10)
	if got := v.Offset(); got != 3 {
		t.Fatalf("setup pan: offset = %d, want 3 (世's start column)", got)
	}
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 2, Wrap: true}, src), 5)
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 1}, src), 5)
	if got := v.Offset(); got != 2 {
		t.Fatalf("offset after narrowing = %d, want 2 — 世 no longer fits", got)
	}
}

// A file change resets the offset to zero before any reveal: stored
// per-file pan does not carry into the destination file.
func TestPanOffsetResetsOnFileChange(t *testing.T) {
	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, false, strings.Repeat("x", 100)), 5)
	v.Pan(20)
	if got := v.Offset(); got != 20 {
		t.Fatalf("setup pan: offset = %d, want 20", got)
	}
	v.ResetOffset()
	if got := v.Offset(); got != 0 {
		t.Fatalf("offset after the file-change reset = %d, want 0", got)
	}
}

// A grapheme cluster split by either clip edge renders blank cells for
// its in-window portion — a wide glyph never paints half-drawn.
func TestClipSplitClusterRendersBlanks(t *testing.T) {
	m := layoutOf(t, 10, false, "ab世cd")

	// 世 spans columns 2-3; an offset of 3 clips it — the in-window
	// cell renders blank, then c and d paint.
	cells, _ := m.Clip(0, 3, 10)
	if got := cellsText(cells); got != " cd" {
		t.Fatalf("clip at offset 3 = %q, want %q — the split glyph's cell is blank", got, " cd")
	}

	// The same split at the right edge: the window [1,3) covers b and
	// the first column of 世 — the half-shown glyph is blank again.
	cells, _ = m.Clip(0, 1, 2)
	if got := cellsText(cells); got != "b " {
		t.Fatalf("clip at offset 1 width 2 = %q, want %q", got, "b ")
	}

	// An offset on a cluster boundary clips cleanly: no blanks.
	cells, _ = m.Clip(0, 2, 10)
	if got := cellsText(cells); got != "世cd" {
		t.Fatalf("clip at offset 2 = %q, want %q", got, "世cd")
	}
}

// Clipping rebase maps the line's highlight spans onto the window's
// cell indexes: a span on clipped-away cells drops and a surviving one
// shifts to its new index.
func TestClipRebasesHighlights(t *testing.T) {
	src := sourceOf(t, "abcdef")
	src.spans[0] = []filebuffer.Span{{Start: 0, End: 2}, {Start: 3, End: 5}}
	m := NewRowModel(RowModelKey{TextWidth: 10}, src)

	cells, spans := m.Clip(0, 2, 10)
	if got := cellsText(cells); got != "cdef" {
		t.Fatalf("clip at offset 2 = %q, want %q", got, "cdef")
	}
	want := []filebuffer.Span{{Start: 1, End: 3}} // source cells 3-4 → window cells 1-2
	if len(spans) != len(want) {
		t.Fatalf("clipped spans = %+v, want %+v", spans, want)
	}
	for i, s := range spans {
		if s != want[i] {
			t.Fatalf("clipped span %d = %+v, want %+v", i, s, want[i])
		}
	}
}

// The extent of a short-lines-only view is the widest line's last
// cluster start — the pan clamp needs no line wider than the window.
func TestMaxOffsetShortLinesOnly(t *testing.T) {
	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, false, "short", "lines", "only"), 10)
	v.Pan(100)
	if got := v.Offset(); got != 4 {
		t.Fatalf("offset = %d, want 4 — the widest line's last cluster start", got)
	}
}

// The clamp follows the widest *visible* line: the 300-cell line's 299
// applies only while it is rendered; once it scrolls out the stored
// offset re-clamps to the remaining lines' 9 — and scrolling back does
// not restore the earlier offset.
func TestMaxOffsetFollowsWidestVisibleLine(t *testing.T) {
	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, false, tenAfter(300, 20)...), 3)

	v.Pan(1000)
	if got := v.Offset(); got != 299 {
		t.Fatalf("offset with the long line visible = %d, want 299", got)
	}
	v.Down() // the long line leaves the window; lines 1-3 are ten cells
	if got := v.Offset(); got != 9 {
		t.Fatalf("offset after the long line scrolled out = %d, want the re-clamped 9", got)
	}
	v.Up() // back over the long line: the old offset is not restored
	if got := v.Offset(); got != 9 {
		t.Fatalf("offset after returning to the long line = %d, want the clamped 9", got)
	}
	v.Pan(1000) // every pan re-derives the maximum from the current rows
	if got := v.Offset(); got != 299 {
		t.Fatalf("re-pan with the long line visible = %d, want 299", got)
	}
}

// An empty buffer and a view of only empty lines clamp to zero, and a
// viewport with no layout at all — the placeholder case — pans as a
// no-op.
func TestMaxOffsetEmptyViews(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
	}{
		{"empty buffer", nil},
		{"all-empty lines", []string{"", "", ""}},
	} {
		v := &Viewport{}
		v.SetLayout(layoutOf(t, 10, false, tc.lines...), 10)
		v.Pan(100)
		if got := v.Offset(); got != 0 {
			t.Errorf("%s: offset = %d, want 0", tc.name, got)
		}
	}

	v := &Viewport{} // no layout installed: the placeholder view
	v.Pan(100)
	if got := v.Offset(); got != 0 {
		t.Errorf("no-layout pan: offset = %d, want 0", got)
	}
}

// A line ending in a two-cell cluster stops at that cluster's start —
// E-2, not E-1 — and at the maximum the cluster still paints whole.
func TestMaxOffsetStopsAtTrailingCluster(t *testing.T) {
	m := layoutOf(t, 10, false, "abc世")
	v := &Viewport{}
	v.SetLayout(m, 5)
	v.Pan(100)
	if got := v.Offset(); got != 3 {
		t.Fatalf("offset = %d, want 3 — E-2, the trailing cluster's start", got)
	}
	// At the maximum both cells of 世 remain fully painted: no blank,
	// no half glyph.
	cells, _ := m.Clip(0, 3, 10)
	if got := cellsText(cells); got != "世" {
		t.Fatalf("clip at the maximum = %q, want %q fully painted", got, "世")
	}
}

// When the last cluster is wider than the text width the maximum falls
// back to the last cluster that does fit — and to zero when none does.
func TestMaxOffsetFallsBackPastUnfittableCluster(t *testing.T) {
	// 世 (width 2) does not fit a one-cell window: c at column 2 is the
	// last fitting start.
	v := &Viewport{}
	v.SetLayout(layoutOf(t, 1, false, "abc世"), 5)
	v.Pan(100)
	if got := v.Offset(); got != 2 {
		t.Fatalf("offset = %d, want 2 — the last fitting cluster's start", got)
	}

	// A line whose only cluster is wider than the window has no
	// paintable boundary beyond the origin.
	v = &Viewport{}
	v.SetLayout(layoutOf(t, 1, false, "世"), 5)
	v.Pan(100)
	if got := v.Offset(); got != 0 {
		t.Fatalf("all-unfittable line: offset = %d, want 0", got)
	}

	// A lone wide cluster paints whole at offset 0 only — its own
	// start is the maximum even when it fits.
	v = &Viewport{}
	v.SetLayout(layoutOf(t, 5, false, "世"), 5)
	v.Pan(100)
	if got := v.Offset(); got != 0 {
		t.Fatalf("single-cluster line: offset = %d, want 0", got)
	}
}

// A reveal that moves the window re-clamps the same way scrolling
// does: the offset follows the newly visible rows' maximum.
func TestRevealReclampsOffset(t *testing.T) {
	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, false, tenAfter(300, 20)...), 3)
	v.Pan(1000)
	if got := v.Offset(); got != 299 {
		t.Fatalf("setup pan: offset = %d, want 299", got)
	}
	v.Reveal(15) // hidden target → top 14, rows 14-16 all ten-cell lines
	if got := v.Offset(); got != 9 {
		t.Fatalf("offset after the reveal = %d, want the re-clamped 9", got)
	}
}

// Any change that swaps the layout's text width — a resize, the file
// list hiding or showing, gutter growth — re-derives the paintable
// boundary: every visible-set change re-clamps the stored offset.
func TestWidthChangeReclampsOffset(t *testing.T) {
	src := sourceOf(t, "abc世", "tail")
	v := &Viewport{}
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 2}, src), 5)
	v.Pan(10)
	if got := v.Offset(); got != 3 {
		t.Fatalf("setup pan: offset = %d, want 3 (世's start)", got)
	}
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 1}, src), 5)
	if got := v.Offset(); got != 2 {
		t.Fatalf("offset after narrowing = %d, want 2 — 世 no longer fits", got)
	}
}

// Every visible line may simultaneously have hidden-left text: a
// uniform-width file panned to a nonzero offset keeps a fitting
// cluster painted on the widest line — which is every line. Nothing in
// the extent policy forbids the all-lines-hidden-left geometry Issue
// 20's `_` indicator needs.
func TestUniformLinesMayAllHideTextLeft(t *testing.T) {
	var lines []string
	for i := 0; i < 6; i++ {
		lines = append(lines, strings.Repeat("x", 50))
	}
	m := layoutOf(t, 10, false, lines...)
	v := &Viewport{}
	v.SetLayout(m, 6)
	v.Pan(20)
	if got := v.Offset(); got != 20 {
		t.Fatalf("offset = %d, want 20 — inside the [0,49] clamp with text hidden left on every line", got)
	}
	// Every visible row still paints ten whole cells at the offset:
	// the widest line keeps its fitting cluster.
	for i := 0; i < 6; i++ {
		cells, _ := m.Clip(i, 20, 10)
		if got := cellsText(cells); got != strings.Repeat("x", 10) {
			t.Fatalf("row %d clip = %q, want ten painted cells", i, got)
		}
	}
}

// Extent evaluation reads the prepared layout's per-line data — the
// cluster scan happened at build time — so panning and the visible-set
// re-clamp never touch source lines outside the visible window.
func TestExtentEvaluationTouchesOnlyVisibleRows(t *testing.T) {
	var lines []string
	for i := 0; i < 10000; i++ {
		lines = append(lines, "line")
	}
	src := sourceOf(t, lines...)
	v := &Viewport{}
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 10}, src), 10)

	src.cellsQueried = nil
	src.clustersQueried = nil
	v.Down()
	v.Down()
	v.Pan(1)
	v.Pan(-1)

	lo, hi := 0, 12 // every visible window the ops spanned, rows 0..11
	for _, q := range src.clustersQueried {
		if q < lo || q >= hi {
			t.Fatalf("extent evaluation queried line %d, want only visible rows [%d,%d)", q, lo, hi)
		}
	}
	for _, q := range src.cellsQueried {
		if q < lo || q >= hi {
			t.Fatalf("extent evaluation queried cells of line %d, want only visible rows [%d,%d)", q, lo, hi)
		}
	}
}
