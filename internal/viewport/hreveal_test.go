package viewport

import (
	"strings"
	"testing"

	"vrg/internal/filebuffer"
)

// The right-edge rule: a hidden target right of the text window moves
// the offset by the minimum columns that paint its whole cluster at the
// right edge — target − (text width − 1) for a single-cell target, and
// target + cluster width − text width for a wider cluster.
func TestRevealFromRightIsMinimal(t *testing.T) {
	for _, tc := range []struct {
		name       string
		line       string
		start, end int
		want       int
	}{
		// On an all-x line byte, cell, and column coincide.
		{"single-cell target", strings.Repeat("x", 50), 30, 31, 21}, // 30 + 1 - 10
		{"nearer single-cell target", strings.Repeat("x", 50), 12, 13, 3},
		// 世 occupies columns 30-31: cells 0-29 and 31-50 are x's.
		{"two-cell target", strings.Repeat("x", 30) + "世" + strings.Repeat("x", 20), 30, 33, 22}, // 30 + 2 - 10
	} {
		m := layoutOf(t, 10, false, tc.line)
		v := &Viewport{}
		v.SetLayout(m, 5)
		v.Reveal(stopAt(1, tc.start, tc.end))
		if got := v.Offset(); got != tc.want {
			t.Errorf("%s: offset = %d, want %d", tc.name, got, tc.want)
		}
	}

	// At the reveal offset both cells of the wide cluster paint: the
	// window [22, 32) holds the last eight x's and all of 世.
	m := layoutOf(t, 10, false, strings.Repeat("x", 30)+"世"+strings.Repeat("x", 20))
	cells, _ := m.Clip(0, 22, 10)
	if got := cellsText(cells); got != "xxxxxxxx世" {
		t.Fatalf("clip at the reveal offset = %q, want %q fully painted",
			got, "xxxxxxxx世")
	}
}

// The left-edge rule: a target left of the window moves the offset to
// exactly the target column. A cluster split by the left edge — its
// start column left of the window, the in-window part rendered as
// blanks — is hidden the same way.
func TestRevealFromLeftIsMinimal(t *testing.T) {
	// A single-cell target at column 10: the offset drops to exactly 10.
	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, false, strings.Repeat("x", 50)), 5)
	v.Pan(20)
	v.Reveal(stopAt(1, 10, 11))
	if got := v.Offset(); got != 10 {
		t.Fatalf("left-of-window target: offset = %d, want 10", got)
	}

	// 世 occupies columns 4-5. Split at the left edge of window [5,15)
	// its start cell is hidden, so the offset moves to its start column
	// and it paints.
	line := strings.Repeat("x", 4) + "世" + strings.Repeat("x", 45)
	m := layoutOf(t, 10, false, line)
	v.SetLayout(m, 5) // the offset survives the relayout at 10
	v.Pan(-5)
	if got := v.Offset(); got != 5 {
		t.Fatalf("setup: offset = %d, want 5", got)
	}
	v.Reveal(stopAt(1, 4, 7)) // the 世 bytes
	if got := v.Offset(); got != 4 {
		t.Fatalf("cluster split at the left edge: offset = %d, want its start column 4", got)
	}
	cells, _ := m.Clip(0, 4, 10)
	if got := cellsText(cells); got != "世xxxxxxxx" {
		t.Fatalf("clip at the reveal offset = %q, want %q", got, "世xxxxxxxx")
	}
}

// A target whose start cell is already painted — mid-window, flush at
// the left edge, or flush at the right edge — leaves the offset exactly
// where it was: the reveal never scrolls gratuitously.
func TestRevealKeepsPaintedTarget(t *testing.T) {
	// 世 occupies columns 8-9.
	line := strings.Repeat("x", 8) + "世" + strings.Repeat("x", 40)
	for _, tc := range []struct {
		name       string
		off        int
		start, end int
	}{
		{"mid-window single cell", 10, 18, 19}, // byte 18 → column 17 ⊂ [10,20)
		{"wide cluster flush left", 8, 8, 11},  // 世 [8,10) ⊆ [8,18)
		{"wide cluster flush right", 0, 8, 11}, // 世 [8,10) ends at 0+10
	} {
		v := &Viewport{}
		v.SetLayout(layoutOf(t, 10, false, line), 5)
		v.Pan(tc.off)
		v.Reveal(stopAt(1, tc.start, tc.end))
		if got := v.Offset(); got != tc.off {
			t.Errorf("%s: offset = %d, want unchanged %d", tc.name, got, tc.off)
		}
	}
}

// Painted-cell visibility is over the rendered cells after grapheme
// clipping, not over window geometry: a target cell inside the window
// whose cluster is split by the right edge renders as a blank, so it
// counts as hidden and is revealed — by the minimum that paints the
// whole cluster.
func TestRevealClippedBlankCountsAsHidden(t *testing.T) {
	// 世 occupies columns 10-11: the window [1,11) covers its first
	// column only, rendering one clipping blank at its right edge.
	line := strings.Repeat("x", 10) + "世" + strings.Repeat("x", 20)
	m := layoutOf(t, 10, false, line)
	cells, _ := m.Clip(0, 1, 10)
	if got := cellsText(cells); got != strings.Repeat("x", 9)+" " {
		t.Fatalf("setup clip = %q, want a blank where 世 splits", got)
	}

	v := &Viewport{}
	v.SetLayout(m, 5)
	v.Pan(1)
	v.Reveal(stopAt(1, 10, 13)) // the 世 bytes
	if got := v.Offset(); got != 2 {
		t.Fatalf("clipped-blank target: offset = %d, want 2 — "+
			"the minimum that paints 世 whole", got)
	}
	cells, _ = m.Clip(0, 2, 10)
	if got := cellsText(cells); got != "xxxxxxxx世" {
		t.Fatalf("clip after the reveal = %q, want %q", got, "xxxxxxxx世")
	}
}

// A match span wider than the text area is revealed by its start cell
// alone: the offset moves only enough to paint the first submatch's
// start, never to fit the whole span.
func TestRevealOversizedSpanByStartCell(t *testing.T) {
	v := &Viewport{}
	v.SetLayout(layoutOf(t, 10, false, strings.Repeat("x", 50)), 5)
	// Coverage 20→45 spans 25 columns, wider than the ten-cell window;
	// the start cell still reveals to target − (text width − 1).
	v.Reveal(stopAt(1, 20, 45))
	if got := v.Offset(); got != 11 {
		t.Fatalf("oversized span: offset = %d, want 11 — its start cell at the right edge", got)
	}
}

// A cluster wider than the whole text area can never paint at any
// offset: the reveal sets the offset to its start column — the closest
// achievable position — and treats the target as geometrically
// revealed. Its in-window columns render as clipping blanks that carry
// no highlight, so indicator visibility still counts it as not
// visible, and a repeated reveal moves nothing: the fallback is
// stable, not a panning loop.
func TestRevealUnpaintableClusterFallsBackToStartColumn(t *testing.T) {
	// A tab at column 2 expands to width 6 — wider than text width 5.
	src := sourceOf(t, "xx\txxx")
	src.spans[0] = []filebuffer.Span{{Start: 2, End: 8}} // the tab's cells
	m := NewRowModel(RowModelKey{TextWidth: 5}, src)
	v := &Viewport{}
	v.SetLayout(m, 5)

	v.Reveal(stopAt(1, 2, 3)) // the tab's byte range
	if got := v.Offset(); got != 2 {
		t.Fatalf("unpaintable cluster: offset = %d, want its start column 2", got)
	}
	cells, spans := m.Clip(0, 2, 5)
	if got := cellsText(cells); got != "     " {
		t.Fatalf("unpaintable cluster renders %q, want five clipping blanks", got)
	}
	if len(spans) != 0 {
		t.Fatalf("clipping blanks carried spans %+v, want none — the cluster is not visible", spans)
	}

	v.Reveal(stopAt(1, 2, 3))
	if got := v.Offset(); got != 2 {
		t.Fatalf("repeated reveal moved the offset to %d, want the stable 2", got)
	}
}

// The reveal's movement is bounded by the visible-lines extent clamp
// like any other offset change: when the widest visible line has no
// fitting cluster at all, the maximum is zero and the reveal's wanted
// offset clamps to it — the clamped value is what applies.
func TestRevealOffsetClampsToVisibleExtent(t *testing.T) {
	// Line 1 is four tabs — every cluster wider than text width 5 —
	// making it the widest visible line with no fitting start, so the
	// maximum offset is 0 even though line 0's target wants 21.
	m := layoutOf(t, 5, false, strings.Repeat("x", 30), "\t\t\t\t")
	v := &Viewport{}
	v.SetLayout(m, 5)
	v.Reveal(stopAt(1, 25, 26))
	if got := v.Offset(); got != 0 {
		t.Fatalf("offset = %d, want the clamped 0", got)
	}
}

// In wrap mode the reveal is vertical only: the horizontal offset is
// retained run-off-edge state, never re-derived against a wrapped
// layout, so a horizontally hidden target cannot move it.
func TestRevealHorizontalNoopInWrapMode(t *testing.T) {
	src := sourceOf(t, strings.Repeat("x", 100))
	v := &Viewport{}
	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 10}, src), 5)
	v.Pan(10)

	v.SetLayout(NewRowModel(RowModelKey{TextWidth: 10, Wrap: true}, src), 5)
	v.Reveal(stopAt(1, 50, 51))
	if got := v.Offset(); got != 10 {
		t.Fatalf("reveal in wrap mode changed the offset to %d, want the retained 10", got)
	}
}
