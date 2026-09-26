package viewport

import (
	"slices"
	"strings"
	"testing"

	"vrg/internal/present"
)

// hline installs a one-line run-off-edge row model in a viewport w
// cells wide — the fixture shape every horizontal-reveal case shares.
func hline(t *testing.T, w int, text string, spans []present.Span) *Viewport {
	t.Helper()
	v := &Viewport{}
	v.Resize(w, 4)
	v.SetRows(Prepare(source(t, text, map[int][]present.Span{0: spans}), Key{Width: w}))
	return v
}

// A single-cell target right of the window moves the offset by the
// minimum columns that paints it — T − (text width − 1) — so the start
// cell lands on the last text column, no further.
func TestHRevealRightOfViewPaintsStartCell(t *testing.T) {
	// "hit" covers cells 295–297; the target is its start cell 295.
	v := hline(t, 10, strings.Repeat("x", 295)+"hit\n",
		[]present.Span{{Start: 295, End: 298}})
	v.Reveal(Target{Line: 0, Cell: 295})
	if v.Offset() != 286 {
		t.Fatalf("offset = %d, want 286 = 295 − (10 − 1)", v.Offset())
	}
	if got := rowText(v.Visible()[0]); got != "xxxxxxxxxh" {
		t.Fatalf("row = %q, want %q — the start cell paints at the right edge",
			got, "xxxxxxxxxh")
	}
}

// A two-cell target cluster right of the window moves to
// T + cluster width − text width so the whole cluster paints at the
// right edge — never a half glyph.
func TestHRevealRightOfViewWideClusterPaintsWhole(t *testing.T) {
	// 世 occupies cells 295–296; the match covers it, start cell 295.
	v := hline(t, 10, strings.Repeat("x", 295)+"世x\n",
		[]present.Span{{Start: 295, End: 298}})
	v.Reveal(Target{Line: 0, Cell: 295})
	if v.Offset() != 287 {
		t.Fatalf("offset = %d, want 287 = 295 + 2 − 10", v.Offset())
	}
	r := v.Visible()[0]
	if got := rowText(r); got != "xxxxxxxx世" {
		t.Fatalf("row = %q, want %q — both cells of 世 painted", got, "xxxxxxxx世")
	}
	if len(r.Cells) != 10 || !r.Cells[9].Cont {
		t.Fatalf("row cells = %+v, want 世's trailing cell on the last column", r.Cells)
	}
}

// A target left of the window moves the offset to the target column
// exactly — including a cluster split by the left edge, whose
// in-window cell was blank.
func TestHRevealLeftOfViewLandsOnTargetColumn(t *testing.T) {
	v := hline(t, 10, strings.Repeat("x", 295)+"hit\n", nil)
	v.SetOffset(100)
	v.Reveal(Target{Line: 0, Cell: 5})
	if v.Offset() != 5 {
		t.Fatalf("offset = %d, want 5 — the target column", v.Offset())
	}

	// 世 at cells 4–5, offset 5: the window opens on its trailing
	// cell, which paints blank — hidden, so the offset moves to 4.
	v2 := hline(t, 10, "xxxx世xxxxx\n", []present.Span{{Start: 4, End: 6}})
	v2.SetOffset(5)
	if got := rowText(v2.Visible()[0]); got != " xxxxx" {
		t.Fatalf("pre-reveal row = %q, want a leading clipping blank", got)
	}
	v2.Reveal(Target{Line: 0, Cell: 4})
	if v2.Offset() != 4 {
		t.Fatalf("offset = %d, want 4 — the cluster's start", v2.Offset())
	}
	if got := rowText(v2.Visible()[0]); got != "世xxxxx" {
		t.Fatalf("row = %q, want %q — the cluster whole", got, "世xxxxx")
	}
}

// A target whose start cell is already painted — anywhere in the
// window, including a cluster flush with the right edge — leaves the
// offset alone.
func TestHRevealPaintedTargetKeepsOffset(t *testing.T) {
	for _, tc := range []struct {
		name   string
		text   string
		spans  []present.Span
		off    int
		target int
	}{
		{"single cell mid-window", strings.Repeat("x", 295) + "hit\n",
			[]present.Span{{Start: 295, End: 298}}, 40, 45},
		{"single cell on the last column", strings.Repeat("x", 60) + "\n",
			[]present.Span{{Start: 50, End: 51}}, 41, 50},
		{"wide cluster flush with the right edge", strings.Repeat("x", 49) + "世" + strings.Repeat("x", 10) + "\n",
			[]present.Span{{Start: 49, End: 51}}, 41, 49},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := hline(t, 10, tc.text, tc.spans)
			v.SetOffset(tc.off)
			v.Reveal(Target{Line: 0, Cell: tc.target})
			if v.Offset() != tc.off {
				t.Fatalf("offset = %d, want the unchanged %d — the target was painted",
					v.Offset(), tc.off)
			}
		})
	}
}

// A target geometrically inside the window but rendered as a clipping
// blank — a two-cell glyph split by the right edge — counts as hidden
// and is revealed: visibility is painted cells, not positions.
func TestHRevealClippedBlankCountsAsHidden(t *testing.T) {
	// 世 occupies cells 9–10; at offset 0 the window [0,10) holds its
	// lead cell but clips its trailing cell, painting a blank at 9.
	v := hline(t, 10, "xxxxxxxxx世xx\n", []present.Span{{Start: 9, End: 11}})
	if got := rowText(v.Visible()[0]); got != "xxxxxxxxx " {
		t.Fatalf("pre-reveal row = %q, want the target cell blank", got)
	}
	v.Reveal(Target{Line: 0, Cell: 9})
	if v.Offset() != 1 {
		t.Fatalf("offset = %d, want 1 = 9 + 2 − 10", v.Offset())
	}
	if got := rowText(v.Visible()[0]); got != "xxxxxxxx世" {
		t.Fatalf("row = %q, want %q — 世 painted whole", got, "xxxxxxxx世")
	}
}

// A match wider than the text area is revealed by its start cell
// alone: the offset paints the first cell at the right edge while the
// rest of the span stays hidden right.
func TestHRevealOversizedSpanShowsStartCell(t *testing.T) {
	v := hline(t, 10, strings.Repeat("x", 300)+"\n",
		[]present.Span{{Start: 100, End: 250}})
	v.Reveal(Target{Line: 0, Cell: 100})
	if v.Offset() != 91 {
		t.Fatalf("offset = %d, want 91 = 100 + 1 − 10", v.Offset())
	}
	// The span clips to the last window cell — the start cell is
	// painted, the tail of the match is not required.
	if got := v.Visible()[0].Spans; !slices.Equal(got, []present.Span{{Start: 9, End: 10}}) {
		t.Fatalf("clipped spans = %+v, want [{9 10}]", got)
	}
	// A repeat reveal sees the painted start cell and does not move.
	v.Reveal(Target{Line: 0, Cell: 100})
	if v.Offset() != 91 {
		t.Fatalf("offset after repeat reveal = %d, want the stable 91", v.Offset())
	}
}

// A target cluster wider than the whole text area cannot paint at any
// offset: the reveal sets the offset to its start column — the closest
// achievable position, beyond the paintable-boundary pan maximum — the
// in-window cells render as clipping blanks, and the geometric
// fallback makes repeated reveals a no-op rather than a panning loop.
func TestHRevealUnpaintableClusterUsesStartColumn(t *testing.T) {
	// "ab" + a six-cell \u0085 escape cluster at cells 2–7, text width
	// 4: no offset paints it, and its line's paintable boundary is 1.
	v := hline(t, 4, "ab\xc2\x85cd\n", []present.Span{{Start: 2, End: 8}})
	v.Reveal(Target{Line: 0, Cell: 2})
	if v.Offset() != 2 {
		t.Fatalf("offset = %d, want 2 — the target's start column", v.Offset())
	}
	if got := rowText(v.Visible()[0]); got != "    " {
		t.Fatalf("row = %q, want four clipping blanks — the unpaintable cluster "+
			"still counts as not visible for Issue #20's indicators", got)
	}
	// Geometrically revealed: the repeat reveal does not move further.
	v.Reveal(Target{Line: 0, Cell: 2})
	if v.Offset() != 2 {
		t.Fatalf("offset after repeat reveal = %d, want the stable 2 — no panning loop",
			v.Offset())
	}
}

// A marker target is one painted cell: an end-of-line marker past the
// window reveals to position it on the last column, and a marker on a
// clipped wide cluster's lead cell still paints its inverse cell — it
// counts as visible without needing the cluster whole.
func TestHRevealMarkerCells(t *testing.T) {
	// "abc" is 3 cells; the zero-width match at end of line marks
	// cell 3, one cell past the width-3 window.
	v := hline(t, 3, "abc\n", []present.Span{{Start: 3, End: 3}})
	v.Reveal(Target{Line: 0, Cell: 3})
	if v.Offset() != 1 {
		t.Fatalf("offset = %d, want 1 = 3 + 1 − 3", v.Offset())
	}
	if got := v.Visible()[0].Spans; !slices.Equal(got, []present.Span{{Start: 2, End: 2}}) {
		t.Fatalf("clipped spans = %+v, want the marker at [{2 2}]", got)
	}

	// The marker at cell 9 sits inside the window on 世's lead cell:
	// the marker paints that cell regardless of the clipped cluster.
	v2 := hline(t, 10, "xxxxxxxxx世xx\n", []present.Span{{Start: 9, End: 9}})
	v2.Reveal(Target{Line: 0, Cell: 9})
	if v2.Offset() != 0 {
		t.Fatalf("offset = %d, want 0 — the marker cell was already painted", v2.Offset())
	}
}

// The horizontal reveal accompanies the vertical one: a target on a
// hidden row and a hidden column moves both.
func TestHRevealMovesBothAxes(t *testing.T) {
	src := source(t, strings.Repeat("y\n", 8)+strings.Repeat("x", 295)+"hit\n",
		map[int][]present.Span{8: {{Start: 295, End: 298}}})
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	v.Reveal(Target{Line: 8, Cell: 295})
	// 9 rows in a height-4 viewport: the last-full-page clamp pins the
	// top at 5 with the target row on the bottom row.
	if v.Top() != 5 {
		t.Fatalf("top = %d, want the EOF-clamped 5", v.Top())
	}
	if v.Offset() != 286 {
		t.Fatalf("offset = %d, want 286 — the horizontal reveal", v.Offset())
	}
}

// In wrap mode there is no horizontal reveal: the dormant offset is
// untouched even by a target far down a wrapped line.
func TestHRevealNoOpInWrapMode(t *testing.T) {
	src := source(t, strings.Repeat("x", 300)+"\n",
		map[int][]present.Span{0: {{Start: 295, End: 298}}})
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	v.SetOffset(40)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.Reveal(Target{Line: 0, Cell: 295})
	if v.Offset() != 40 {
		t.Fatalf("offset = %d, want the dormant 40 — wrap mode has no horizontal reveal",
			v.Offset())
	}
}
