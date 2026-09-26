package viewport

import (
	"slices"
	"strings"
	"testing"

	"vrg/internal/present"
)

// mixedLines builds the mixed-width fixture the extent policy is
// specified against: one long single-cell-cluster line among short
// ones — a 300-cell line yields a 299 maximum while it is visible
// and 9 once it scrolls out.
func mixedLines(t *testing.T, long, short, shorts int) *lineSource {
	t.Helper()
	return source(t, strings.Repeat("x", long)+"\n"+
		strings.Repeat(strings.Repeat("y", short)+"\n", shorts), nil)
}

// Each pan unit moves the horizontal offset by its contracted amount —
// Left/Right one column, TenLeft/TenRight ten, HalfLeft/HalfRight
// max(1, floor(text width / 2)) — clamped to [0, max(0, S)] where S is
// the paintable-boundary maximum of the widest visible line: 299 for
// the 300-cell line in this fixture.
func TestPanUnits(t *testing.T) {
	for _, tc := range []struct {
		name  string
		width int
		start int
		act   func(*Viewport)
		want  int
	}{
		{"right pans one column", 10, 0, (*Viewport).Right, 1},
		{"left pans one column", 10, 7, (*Viewport).Left, 6},
		{"ten columns right", 10, 0, (*Viewport).TenRight, 10},
		{"ten columns left", 10, 25, (*Viewport).TenLeft, 15},
		{"half the text width right", 10, 0, (*Viewport).HalfRight, 5},
		{"half of odd width floors", 11, 0, (*Viewport).HalfRight, 5},
		{"half the text width left", 12, 20, (*Viewport).HalfLeft, 14},
		{"half of width one is one column", 1, 0, (*Viewport).HalfRight, 1},
		{"right clamps at the extent maximum", 10, 298, (*Viewport).Right, 299},
		{"ten right clamps at the extent maximum", 10, 295, (*Viewport).TenRight, 299},
		{"half right clamps at the extent maximum", 10, 297, (*Viewport).HalfRight, 299},
		{"left clamps at zero", 10, 0, (*Viewport).Left, 0},
		{"ten left clamps at zero", 10, 5, (*Viewport).TenLeft, 0},
		{"half left clamps at zero", 10, 3, (*Viewport).HalfLeft, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var v Viewport
			v.Resize(tc.width, 4)
			v.SetRows(Prepare(mixedLines(t, 300, 10, 10), Key{Width: tc.width}))
			v.SetOffset(tc.start)
			tc.act(&v)
			if v.Offset() != tc.want {
				t.Fatalf("offset = %d, want %d", v.Offset(), tc.want)
			}
		})
	}
}

// SetOffset installs a stored offset clamped to the current maximum —
// the seam the file-change reset drives with SetOffset(0).
func TestSetOffsetClamps(t *testing.T) {
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(mixedLines(t, 300, 10, 10), Key{Width: 10}))
	v.SetOffset(500)
	if v.Offset() != 299 {
		t.Fatalf("offset = %d, want the clamped 299", v.Offset())
	}
	v.SetOffset(-3)
	if v.Offset() != 0 {
		t.Fatalf("offset = %d, want 0", v.Offset())
	}
}

// Panning is a strict no-op while a wrap model is installed: the
// stored offset is dormant — retained, not re-derived — and never
// leaks into the wrapped rendering.
func TestPanNoOpInWrapMode(t *testing.T) {
	src := mixedLines(t, 300, 10, 10)
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	for i := 0; i < 7; i++ {
		v.Right()
	}
	if v.Offset() != 7 {
		t.Fatalf("offset after pans = %d, want 7", v.Offset())
	}
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	if v.Offset() != 7 {
		t.Fatalf("offset under wrap = %d, want the retained 7", v.Offset())
	}
	for _, pan := range []func(*Viewport){
		(*Viewport).Left, (*Viewport).Right,
		(*Viewport).TenLeft, (*Viewport).TenRight,
		(*Viewport).HalfLeft, (*Viewport).HalfRight,
	} {
		pan(&v)
	}
	if v.Offset() != 7 {
		t.Fatalf("offset after wrap-mode pans = %d, want the dormant 7", v.Offset())
	}
	if got := rowText(v.Visible()[0]); got != strings.Repeat("x", 10) {
		t.Fatalf("wrapped row = %q, want the line's first ten cells — the dormant offset must not clip", got)
	}
}

// Panning with no prepared rows — the loading and unreadable
// placeholders — is a no-op at zero.
func TestPanWithoutContentIsNoOp(t *testing.T) {
	var v Viewport
	v.Resize(10, 4)
	for _, pan := range []func(*Viewport){
		(*Viewport).Right, (*Viewport).TenRight, (*Viewport).HalfRight,
	} {
		pan(&v)
	}
	if v.Offset() != 0 {
		t.Fatalf("offset = %d, want 0", v.Offset())
	}
}

// w w keeps the offset: the run-off-edge re-entry recomputes the
// maximum from the current visible rows, and an unchanged visible set
// leaves the stored offset inside it.
func TestOffsetRetainedThroughWrapToggle(t *testing.T) {
	src := mixedLines(t, 300, 10, 10)
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	v.SetOffset(42)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.SetRows(Prepare(src, Key{Width: 10}))
	if v.Offset() != 42 {
		t.Fatalf("offset after w w = %d, want the retained 42", v.Offset())
	}
}

// A wrap → run-off-edge re-entry whose visible set changed while in
// wrap mode clamps against the new visible rows' maximum — the clamp
// applies to the stored offset and is not restored when the wide line
// becomes visible again.
func TestWrapReentryClampsToNewVisibleSet(t *testing.T) {
	src := mixedLines(t, 300, 10, 10)
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	v.SetOffset(100)

	// Enter wrap and scroll past the long line's 30 wrapped rows;
	// the offset is dormant, not yet re-clamped.
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.SetTop(30)
	if v.Offset() != 100 {
		t.Fatalf("offset in wrap mode = %d, want the dormant 100", v.Offset())
	}

	// Re-entry: the anchor resolved to a short line, so the visible
	// set's maximum is 9 — the stored offset clamps to it.
	v.SetRows(Prepare(src, Key{Width: 10}))
	if v.Top() != 1 || v.Offset() != 9 {
		t.Fatalf("re-entry top=%d offset=%d, want top 1 offset 9", v.Top(), v.Offset())
	}
	v.SetTop(0)
	if v.Offset() != 9 {
		t.Fatalf("offset after scrolling back = %d, want the clamped 9 — no restoration", v.Offset())
	}
}

// Re-entry clamping applies on every return to run-off-edge mode — a
// width change while in wrap is one trigger: a cluster that fit the
// old text width may no longer fit, moving the paintable boundary
// left.
func TestWrapReentryClampsAfterWidthChange(t *testing.T) {
	src := source(t, "xxxx世\n", nil)
	var v Viewport
	v.Resize(2, 4)
	v.SetRows(Prepare(src, Key{Width: 2}))
	// 世 occupies cells 4–5 and fits a width-2 text area, so S = 4.
	v.SetOffset(4)
	v.SetRows(Prepare(src, Key{Width: 2, Wrap: true}))
	v.Resize(1, 4)
	v.SetRows(Prepare(src, Key{Width: 1, Wrap: true}))
	// At width 1 the 世 cluster cannot fit: the boundary falls back
	// to the last fitting cluster, cell 3.
	v.SetRows(Prepare(src, Key{Width: 1}))
	if v.Offset() != 3 {
		t.Fatalf("offset after narrower re-entry = %d, want 3", v.Offset())
	}
}

// The visible-lines extent policy: the clamp follows the widest
// currently rendered source line — a 300-cell line gives a 299
// maximum while visible and 9 once it scrolls out. The clamp is
// applied to the stored offset and scrolling back does not restore it.
func TestExtentFollowsWidestVisibleLine(t *testing.T) {
	src := mixedLines(t, 300, 10, 10)
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	v.SetOffset(299)
	v.Down()
	if v.Offset() != 9 {
		t.Fatalf("offset after the long line scrolled out = %d, want 9", v.Offset())
	}
	v.Up()
	if v.Offset() != 9 {
		t.Fatalf("offset after scrolling back = %d, want the clamped 9 — no restoration", v.Offset())
	}
}

// An empty buffer, a placeholder (no prepared rows), and a view of
// only empty lines all clamp the offset to 0.
func TestExtentEmptyViewsClampToZero(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows Rows
	}{
		{"placeholder: no prepared rows", nil},
		{"empty buffer", Prepare(source(t, "", nil), Key{Width: 10})},
		{"all-empty lines", Prepare(source(t, "\n\n\n", nil), Key{Width: 10})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var v Viewport
			v.Resize(10, 4)
			v.SetRows(Prepare(mixedLines(t, 300, 10, 10), Key{Width: 10}))
			v.SetOffset(50)
			v.SetRows(tc.rows)
			if v.Offset() != 0 {
				t.Fatalf("offset on %s = %d, want 0", tc.name, v.Offset())
			}
		})
	}
}

// The paintable boundary: a line ending in a two-cell cluster stops at
// that cluster's start — an offset inside the cluster would paint only
// clipping blanks — and at the maximum both its cells paint whole.
func TestPaintableBoundaryStopsAtFinalCluster(t *testing.T) {
	src := source(t, "xxxx世\n", nil)
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	for i := 0; i < 10; i++ {
		v.Right()
	}
	if v.Offset() != 4 {
		t.Fatalf("offset at maximum = %d, want 4 — 世's start, not 5", v.Offset())
	}
	if got := rowText(v.Visible()[0]); got != "世" {
		t.Fatalf("row at maximum = %q, want the whole 世 cluster painted", got)
	}
}

// A final cluster wider than the text width cannot paint whole at any
// offset: the maximum falls back to the last fitting cluster's start —
// or 0 when no cluster fits at all, the documented exception where the
// in-window portion renders as clipping blanks by design.
func TestPaintableBoundaryUnfittableFinalCluster(t *testing.T) {
	// "ab\xc2\x85": the C1 control's UTF-8 bytes escape as one
	// six-cell cluster.
	src := source(t, "ab\xc2\x85\n", nil)
	var v Viewport
	v.Resize(4, 4)
	v.SetRows(Prepare(src, Key{Width: 4}))
	v.SetOffset(99)
	if v.Offset() != 1 {
		t.Fatalf("maximum = %d, want 1 — the last fitting cluster's start", v.Offset())
	}
	if got := rowText(v.Visible()[0]); got != "b   " {
		t.Fatalf("row at maximum = %q, want %q — b painted, the unfittable cluster blank", got, "b   ")
	}

	// A line whose only cluster is wider than the text area has
	// maximum 0; at that offset its clipped cells render blank.
	src = source(t, "\xc2\x85\n", nil)
	var v2 Viewport
	v2.Resize(4, 4)
	v2.SetRows(Prepare(src, Key{Width: 4}))
	v2.SetOffset(99)
	if v2.Offset() != 0 {
		t.Fatalf("unfittable-only maximum = %d, want 0", v2.Offset())
	}
	if got := rowText(v2.Visible()[0]); got != "    " {
		t.Fatalf("row = %q, want four clipping blanks", got)
	}
}

// Every change to the visible line set re-clamps the stored offset —
// vertical scrolling, a moving reveal, a resize (list hide/show and
// gutter growth reach the viewport as width resizes), a row-model
// swap, and wrap-toggle re-entry — and every pan re-evaluates the
// maximum from the current visible rows rather than a cached value.
func TestReclampOnVisibleSetChanges(t *testing.T) {
	newVp := func(t *testing.T) *Viewport {
		t.Helper()
		v := &Viewport{}
		v.Resize(10, 4)
		v.SetRows(Prepare(mixedLines(t, 300, 10, 10), Key{Width: 10}))
		v.SetOffset(100)
		return v
	}

	t.Run("vertical scroll", func(t *testing.T) {
		v := newVp(t)
		v.Down() // the 300-cell line leaves the window → max 9
		if v.Offset() != 9 {
			t.Fatalf("offset after scroll = %d, want 9", v.Offset())
		}
	})
	t.Run("moving reveal", func(t *testing.T) {
		v := newVp(t)
		if !v.Reveal(Target{Line: 5}) {
			t.Fatal("reveal of a hidden line reported no move")
		}
		if v.Offset() != 9 {
			t.Fatalf("offset after reveal = %d, want 9", v.Offset())
		}
	})
	t.Run("width resize re-fits clusters", func(t *testing.T) {
		var v Viewport
		v.Resize(2, 4)
		v.SetRows(Prepare(source(t, "xxxx世\n", nil), Key{Width: 2}))
		v.SetOffset(4) // 世 fits at width 2 → S = 4
		v.Resize(1, 4) // at width 1 it no longer fits → S = 3
		if v.Offset() != 3 {
			t.Fatalf("offset after width resize = %d, want 3", v.Offset())
		}
	})
	t.Run("row-model swap", func(t *testing.T) {
		v := newVp(t)
		v.SetRows(Prepare(mixedLines(t, 20, 10, 10), Key{Width: 10}))
		if v.Offset() != 19 {
			t.Fatalf("offset after swap = %d, want 19", v.Offset())
		}
		// And a pan evaluates the new visible rows' maximum — 19,
		// not the old fixture's 299.
		v.SetOffset(15)
		v.TenRight()
		if v.Offset() != 19 {
			t.Fatalf("offset after pan = %d, want the newly computed 19", v.Offset())
		}
	})
}

// A clip edge landing inside a grapheme cluster renders the clipped
// portion as blank cells — never a half glyph — at either edge.
func TestSplitClusterClipsToBlankCells(t *testing.T) {
	// Cells: a b 世(2 cells) c d — a six-cell line.
	src := source(t, "ab世cd\n", nil)
	for _, tc := range []struct {
		name  string
		width int
		off   int
		want  string
	}{
		{"no clip paints whole", 10, 0, "ab世cd"},
		{"right edge splits 世", 3, 0, "ab "},
		{"世 whole inside the window", 4, 1, "b世c"},
		{"世 leads the window", 4, 2, "世cd"},
		{"left edge splits 世", 4, 3, " cd"},
		{"window on 世's lead cell alone", 1, 2, " "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var v Viewport
			v.Resize(tc.width, 4)
			v.SetRows(Prepare(src, Key{Width: tc.width}))
			v.SetOffset(tc.off)
			if got := rowText(v.Visible()[0]); got != tc.want {
				t.Fatalf("row = %q, want %q", got, tc.want)
			}
		})
	}
}

// Clipping translates spans into window cells: coverage ranges shift
// and clip to the window, and marker positions follow their cell —
// hidden when the cell is clipped away.
func TestClipTranslatesSpans(t *testing.T) {
	src := source(t, "abcdefgh\n", map[int][]present.Span{
		0: {{Start: 1, End: 7}, {Start: 3, End: 3}},
	})
	var v Viewport
	v.Resize(4, 4)
	v.SetRows(Prepare(src, Key{Width: 4}))
	v.SetOffset(2)
	got := v.Visible()[0].Spans
	want := []present.Span{{Start: 0, End: 4}, {Start: 1, End: 1}}
	if !slices.Equal(got, want) {
		t.Fatalf("clipped spans = %+v, want %+v", got, want)
	}
	v.SetOffset(6)
	got = v.Visible()[0].Spans
	// The marker at 3 is hidden left; the coverage tail remains.
	want = []present.Span{{Start: 0, End: 1}}
	if !slices.Equal(got, want) {
		t.Fatalf("clipped spans = %+v, want %+v", got, want)
	}
}

// Uniform lines wider than the text area can all have hidden-left
// text at a nonzero offset while the widest still paints a fitting
// cluster — geometry Issue #20's `_` indicator must be able to show
// on every visible line at once, so no clamp rule may forbid it.
func TestUniformLinesAllHiddenLeft(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 4; i++ {
		sb.WriteString(strings.Repeat(string(rune('a'+i)), 30) + "\n")
	}
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(source(t, sb.String(), nil), Key{Width: 10}))
	v.SetOffset(15)
	if v.Offset() != 15 {
		t.Fatalf("offset = %d, want 15 — every line has hidden-left text and still paints", v.Offset())
	}
	for i, r := range v.Visible() {
		want := strings.Repeat(string(rune('a'+i)), 10)
		if got := rowText(r); got != want {
			t.Fatalf("row %d = %q, want %q", i, got, want)
		}
	}
}

// The extent clamp is computed from the prepared layout's visible row
// range only: a pan queries the provider for the visible rows' lines
// and a scroll touches only the newly visible window — never a
// whole-buffer scan.
func TestExtentQueriesOnlyVisibleRows(t *testing.T) {
	src := source(t, strings.Repeat(strings.Repeat("x", 50)+"\n", 1000), nil)
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	v.SetTop(8)
	src.cellsQ, src.spansQ = nil, nil

	v.Right()
	want := []int{8, 9, 10, 11}
	if !slices.Equal(src.cellsQ, want) {
		t.Fatalf("pan queried lines %v, want the visible %v", src.cellsQ, want)
	}

	src.cellsQ = nil
	v.Down()
	for _, q := range src.cellsQ {
		if q < 9 || q > 12 {
			t.Fatalf("scroll queried line %d outside the visible 9..12", q)
		}
	}
}
