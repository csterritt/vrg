package viewport_test

import (
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/viewport"
)

// flatRows prepares a run-off-edge model for content at text width w.
func flatRows(t *testing.T, content string, w int) *viewport.Rows {
	t.Helper()
	return viewport.Prepare(loadBuffer(t, content), viewport.Key{TextWidth: w})
}

// wrappedRows prepares a wrap-mode model for content at text width w.
func wrappedRows(t *testing.T, content string, w int) *viewport.Rows {
	t.Helper()
	return viewport.Prepare(loadBuffer(t, content), viewport.Key{TextWidth: w, Wrap: true})
}

// The [ / ] pan unit is max(1, floor(text width / 2)) — including odd
// and degenerate widths.
func TestHalfTextUnit(t *testing.T) {
	cases := []struct{ width, want int }{
		{0, 1}, {1, 1}, {2, 1}, {3, 1}, {4, 2}, {5, 2},
		{7, 3}, {9, 4}, {10, 5}, {80, 40},
	}
	for _, c := range cases {
		if got := viewport.HalfText(c.width); got != c.want {
			t.Fatalf("HalfText(%d) = %d, want %d", c.width, got, c.want)
		}
	}
}

// The one-column, ten-column, and half-text-width pan units move the
// offset by their sizes, symmetrically right and left.
func TestPanUnitsMoveColumns(t *testing.T) {
	rows := flatRows(t, strings.Repeat("x", 200)+"\n", 10)
	var v viewport.Viewport
	for _, step := range []struct{ d, want int }{
		{1, 1}, {1, 2},
		{10, 12},
		{viewport.HalfText(10), 17},
		{-1, 16}, {-10, 6}, {-viewport.HalfText(10), 1}, {-1, 0},
	} {
		v.Pan(step.d, rows, 4)
		if v.Off() != step.want {
			t.Fatalf("Pan(%d): off = %d, want %d", step.d, v.Off(), step.want)
		}
	}
}

// The offset clamps to [0, max(0, S)] where S is the
// paintable-boundary maximum of the widest currently rendered source
// line — for single-cell content, the widest line's final cell index.
func TestPanClampsToPaintableBoundary(t *testing.T) {
	rows := flatRows(t, strings.Repeat("x", 100)+"\nshort\n", 10)
	if got := viewport.MaxOff(rows, 0, 4); got != 99 {
		t.Fatalf("MaxOff = %d, want 99 — the 100-cell line's last cell", got)
	}
	var v viewport.Viewport
	v.Pan(1000, rows, 4)
	if v.Off() != 99 {
		t.Fatalf("off after an overshooting pan = %d, want 99", v.Off())
	}
	v.Pan(10, rows, 4)
	if v.Off() != 99 {
		t.Fatalf("further panning moved off to %d, want the clamped 99", v.Off())
	}
	v.Pan(-1000, rows, 4)
	if v.Off() != 0 {
		t.Fatalf("off after panning past the left edge = %d, want 0", v.Off())
	}
}

// Panning is a no-op in wrap mode: a wrap-mode model never moves the
// offset, and its extent reports a zero maximum.
func TestPanNoOpInWrapMode(t *testing.T) {
	on := wrappedRows(t, strings.Repeat("x", 100)+"\n", 10)
	var v viewport.Viewport
	v.Pan(10, on, 4)
	if v.Off() != 0 {
		t.Fatalf("off after panning a wrap model = %d, want 0", v.Off())
	}
	if got := viewport.MaxOff(on, 0, 4); got != 0 {
		t.Fatalf("wrap-mode MaxOff = %d, want 0", got)
	}
}

// The horizontal offset is separate state retained through wrap
// toggles: a trip into wrap mode and back keeps it whenever the
// re-entry clamp does not bite.
func TestPanOffsetRetainedThroughWrapToggle(t *testing.T) {
	content := strings.Repeat("x", 200) + "\n" + strings.Repeat("pad\n", 40)
	flat := flatRows(t, content, 10)
	on := wrappedRows(t, content, 10)
	var v viewport.Viewport
	v.Pan(40, flat, 4)
	v.Restore(on, 4) // wrap on: the offset is retained, not clamped
	if v.Off() != 40 {
		t.Fatalf("off after entering wrap mode = %d, want the retained 40", v.Off())
	}
	v.Scroll(2, on, 4) // scrolling in wrap mode must not clamp it either
	if v.Off() != 40 {
		t.Fatalf("off after a wrap-mode scroll = %d, want the retained 40", v.Off())
	}
	v.Restore(flat, 4) // back to run-off-edge: still within the extent
	if v.Off() != 40 {
		t.Fatalf("off after re-entry = %d, want the retained 40", v.Off())
	}
}

// Re-entry into run-off-edge mode re-clamps against the rows visible
// now: a visible set that changed while in wrap mode applies its own
// maximum — the retained offset is not restored.
func TestWrapReentryClampsToNewVisibleSet(t *testing.T) {
	content := strings.Repeat("x", 200) + "\n" + strings.Repeat("pad\n", 40)
	flat := flatRows(t, content, 10)
	on := wrappedRows(t, content, 10)
	var v viewport.Viewport
	v.Pan(50, flat, 4) // line 1 visible: max 199, so off = 50
	v.Restore(on, 4)
	v.Scroll(25, on, 4) // deep into the "pad" lines while wrapped
	v.Restore(flat, 4)
	// The visible rows are 3-cell "pad" lines now: max 2.
	if v.Off() != 2 {
		t.Fatalf("off after re-entry = %d, want the short lines' 2", v.Off())
	}
}

// The clamp follows the widest *currently rendered* source line: a
// 300-cell line among 10-cell lines allows 299 while it is visible and
// 9 once it scrolls out — and the re-clamped offset is not restored
// when the wide line becomes visible again.
func TestMaxOffFollowsVisibleLines(t *testing.T) {
	content := strings.Repeat("x", 300) + "\n" + strings.Repeat("0123456789\n", 40)
	rows := flatRows(t, content, 10)
	var v viewport.Viewport
	if got := viewport.MaxOff(rows, 0, 10); got != 299 {
		t.Fatalf("MaxOff with the long line visible = %d, want 299", got)
	}
	v.Pan(299, rows, 10)
	v.Scroll(1, rows, 10) // the long line scrolls out of the window
	if v.Off() != 9 {
		t.Fatalf("off after scrolling past the long line = %d, want re-clamped 9", v.Off())
	}
	v.Scroll(-1, rows, 10) // the long line is visible again — no restoration
	if v.Off() != 9 {
		t.Fatalf("off after returning to the long line = %d, want the clamped 9", v.Off())
	}
	// A pan re-evaluates from the live visible rows, not a cached
	// maximum: back at the long line the bound is 299 again.
	v.Pan(1, rows, 10)
	if v.Off() != 10 {
		t.Fatalf("off after a fresh pan = %d, want 10", v.Off())
	}
	v.Scroll(1, rows, 10)
	v.Pan(5, rows, 10) // extents changed under the stored offset
	if v.Off() != 9 {
		t.Fatalf("off after panning in the short-lines view = %d, want 9", v.Off())
	}
}

// Empty inputs clamp to zero: an absent buffer's empty model and a
// view of only empty lines both report a zero maximum.
func TestMaxOffEmptyViews(t *testing.T) {
	empty := viewport.Prepare(nil, viewport.Key{TextWidth: 10})
	if got := viewport.MaxOff(empty, 0, 10); got != 0 {
		t.Fatalf("empty model MaxOff = %d, want 0", got)
	}
	blanks := flatRows(t, "\n\n\n", 10)
	if got := viewport.MaxOff(blanks, 0, 10); got != 0 {
		t.Fatalf("all-empty view MaxOff = %d, want 0", got)
	}
	var v viewport.Viewport
	v.Pan(10, blanks, 10)
	if v.Off() != 0 {
		t.Fatalf("panning an all-empty view gave off = %d, want 0", v.Off())
	}
}

// A line ending in a multi-cell cluster stops at that cluster's start
// — smaller than E − 1 — so the maximum offset can never land inside
// the cluster and leave only blanks painted.
func TestMaxOffTrailingWideCluster(t *testing.T) {
	// "abcde文": a–e occupy cells 0–4 and 文 is one cluster spanning
	// cells 5–6. The last fitting start is 5.
	rows := flatRows(t, "abcde文\n", 10)
	if got := viewport.MaxOff(rows, 0, 4); got != 5 {
		t.Fatalf("MaxOff = %d, want 5 — the trailing cluster's start", got)
	}
	// At text width 1 the trailing two-cell cluster cannot fit at all:
	// the maximum falls back to the last fitting cluster's start,
	// cell 4.
	narrow := flatRows(t, "abcde文\n", 1)
	if got := viewport.MaxOff(narrow, 0, 4); got != 4 {
		t.Fatalf("MaxOff at width 1 = %d, want the fallback 4", got)
	}
}

// When the final cluster is wider than the text width the maximum
// falls back to the last fitting cluster's start — and to 0 when no
// cluster of the line fits at all.
func TestMaxOffUnpaintableFinalCluster(t *testing.T) {
	// "ab\t": the tab expands cells 2–7 as one six-cell cluster, too
	// wide for text width 5, so the maximum is the 'b' at cell 1.
	rows := flatRows(t, "ab\t\n", 5)
	if got := viewport.MaxOff(rows, 0, 4); got != 1 {
		t.Fatalf("MaxOff = %d, want the last fitting start 1", got)
	}
	// A lone tab is one eight-cell cluster: nothing fits, maximum 0.
	none := flatRows(t, "\t\n", 5)
	if got := viewport.MaxOff(none, 0, 4); got != 0 {
		t.Fatalf("MaxOff = %d, want 0 — no cluster fits", got)
	}
	var v viewport.Viewport
	v.Pan(10, none, 4)
	if v.Off() != 0 {
		t.Fatalf("panning an unpaintable line gave off = %d, want 0", v.Off())
	}
}

// A reveal that moves the viewport re-clamps the stored offset against
// the newly visible rows.
func TestRevealReclampsOffset(t *testing.T) {
	content := strings.Repeat("x", 300) + "\n" + strings.Repeat("pad\n", 40)
	rows := flatRows(t, content, 10)
	var v viewport.Viewport
	v.Pan(200, rows, 10)
	if !v.Reveal(20, rows, 10) {
		t.Fatal("the reveal did not move the viewport — fixture wrong")
	}
	if v.Off() != 2 {
		t.Fatalf("off after the reveal = %d, want the pad lines' 2", v.Off())
	}
}

// Clamp — the content-height/row-count change path — re-clamps the
// stored offset when the visible window shrinks beneath it.
func TestClampReclampsOffset(t *testing.T) {
	var b strings.Builder
	b.WriteString(strings.Repeat("pad\n", 10))
	b.WriteString(strings.Repeat("x", 300) + "\n")
	b.WriteString(strings.Repeat("pad\n", 20))
	rows := flatRows(t, b.String(), 10)
	var v viewport.Viewport
	// Height 20 shows rows 0–19 including the long line at row 10.
	v.Pan(200, rows, 20)
	if v.Off() != 200 {
		t.Fatalf("off = %d, want 200", v.Off())
	}
	// Shrinking the window to 3 rows leaves only "pad" lines visible.
	v.Clamp(rows, 3)
	if v.Off() != 2 {
		t.Fatalf("off after the window shrank = %d, want re-clamped 2", v.Off())
	}
}

// Every visible line can simultaneously hide text to the left at a
// nonzero offset while the widest still paints a fitting cluster —
// geometry Issue #20's `_` indicator must be free to show on every
// line, so the clamp must not forbid it.
func TestUniformLinesAllHiddenLeftIsLegal(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 10; i++ {
		b.WriteString(strings.Repeat("x", 50) + "\n")
	}
	rows := flatRows(t, b.String(), 20)
	if got := viewport.MaxOff(rows, 0, 10); got != 49 {
		t.Fatalf("MaxOff = %d, want 49 — every line's last cell stays paintable", got)
	}
	var v viewport.Viewport
	v.Pan(49, rows, 10)
	if v.Off() != 49 {
		t.Fatalf("off = %d, want the legal 49 — every line hides 49 cells left", v.Off())
	}
}

// ResetOff is the file-change reset: navigation to another file starts
// from its left edge ahead of Issue #19's horizontal reveal.
func TestResetOff(t *testing.T) {
	rows := flatRows(t, strings.Repeat("x", 100)+"\n", 10)
	var v viewport.Viewport
	v.Pan(20, rows, 4)
	v.ResetOff()
	if v.Off() != 0 {
		t.Fatalf("off after reset = %d, want 0", v.Off())
	}
}

// countingExtent is an Extent fake recording which rows the horizontal
// clamp asks for — the render-cost guard proving extent evaluation
// touches only the visible row range of the prepared layout.
type countingExtent struct {
	n       int
	width   int
	line    filebuffer.Line
	queries []int
}

func (c *countingExtent) Len() int          { return c.n }
func (c *countingExtent) Key() viewport.Key { return viewport.Key{TextWidth: c.width} }

func (c *countingExtent) AnchorAt(row int) viewport.Anchor {
	return viewport.Anchor{Line: int64(row + 1)}
}

func (c *countingExtent) RowOf(a viewport.Anchor) int {
	row := int(a.Line) - 1
	if row < 0 {
		row = 0
	}
	if row >= c.n {
		row = c.n - 1
	}
	return row
}

func (c *countingExtent) At(i int) viewport.Row {
	c.queries = append(c.queries, i)
	return viewport.Row{Line: c.line, Start: 0, End: len(c.line.Cells)}
}

// wantQueries asserts the extent evaluation asked for exactly the
// visible range [top, top+height), in order and nothing more.
func (c *countingExtent) wantQueries(t *testing.T, top, height int) {
	t.Helper()
	if len(c.queries) != height {
		t.Fatalf("extent evaluation queried %d rows, want the %d visible ones: %v",
			len(c.queries), height, c.queries)
	}
	for i, q := range c.queries {
		if q != top+i {
			t.Fatalf("extent query %d read row %d, want visible row %d", i, q, top+i)
		}
	}
}

// Extent evaluation — on a pan, on a scroll, and through the MaxOff
// entry point — touches only the prepared layout's visible rows, never
// scanning the whole buffer.
func TestExtentEvaluationTouchesOnlyVisibleRows(t *testing.T) {
	line := filebuffer.Line{
		Mapped: safepresentation.MapContent([]byte(strings.Repeat("x", 100))),
		Number: 1,
	}
	fake := &countingExtent{n: 1000, width: 10, line: line}
	var v viewport.Viewport
	v.Scroll(40, fake, 10) // top 40; the clamp inside already queried [40,50)

	fake.queries = nil
	v.Pan(5, fake, 10)
	fake.wantQueries(t, 40, 10)
	if v.Off() != 5 {
		t.Fatalf("off = %d, want 5", v.Off())
	}

	fake.queries = nil
	v.Scroll(5, fake, 10) // top 45: the re-clamp reads the new window
	fake.wantQueries(t, 45, 10)

	fake.queries = nil
	if got := viewport.MaxOff(fake, 45, 10); got != 99 {
		t.Fatalf("MaxOff = %d, want the fake line's 99", got)
	}
	fake.wantQueries(t, 45, 10)
}
