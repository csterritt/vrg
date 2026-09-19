package viewport_test

import (
	"testing"

	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// stop builds a navigation stop whose recorded byte spans arrive at
// the viewport as FileBuffer's cluster-expanded cell spans — empty for
// the zero-width and terminator-only spans Issue #23 paints as
// markers.
func stop(line int64, spans ...[2]int) searchindex.Stop {
	st := searchindex.Stop{Number: line}
	for _, s := range spans {
		st.Highlights = append(st.Highlights, searchindex.Span{Start: s[0], End: s[1]})
	}
	return st
}

// An end-of-line marker is a one-cell unit past the line's last
// cluster: it joins the wrapped row when a cell remains and occupies
// the next rendered row whole when the row is exactly full.
func TestEOLMarkerWrapRows(t *testing.T) {
	st := stop(1, [2]int{3, 3})

	// Width 3 fills the text row exactly: the marker wraps to its
	// own continuation row covering the marker cell [3,4).
	rows := viewport.Prepare(loadBufferStops(t, "hit\n", st), viewport.Key{TextWidth: 3, Wrap: true})
	if rows.Len() != 2 {
		t.Fatalf("Len = %d, want 2 — the marker occupies its own row", rows.Len())
	}
	if r := rows.At(1); r.Start != 3 || r.End != 4 || !r.Continuation() {
		t.Fatalf("marker row = {%d,%d} continuation %v, want {3,4} continuation",
			r.Start, r.End, r.Continuation())
	}

	// Width 5 leaves room: the marker joins the row, extending its
	// cell range to [0,4).
	rows = viewport.Prepare(loadBufferStops(t, "hit\n", st), viewport.Key{TextWidth: 5, Wrap: true})
	if rows.Len() != 1 {
		t.Fatalf("Len = %d, want 1 — the marker joins the partial row", rows.Len())
	}
	if r := rows.At(0); r.Start != 0 || r.End != 4 {
		t.Fatalf("row = {%d,%d}, want {0,4} — the marker cell included", r.Start, r.End)
	}

	// Width 2 wraps "hit" into {0,2} and {2,3}; the second row has a
	// cell to spare, so the marker joins it as {2,4).
	rows = viewport.Prepare(loadBufferStops(t, "hit\n", st), viewport.Key{TextWidth: 2, Wrap: true})
	want := [][2]int{{0, 2}, {2, 4}}
	if rows.Len() != len(want) {
		t.Fatalf("Len = %d, want %d", rows.Len(), len(want))
	}
	for i, w := range want {
		if r := rows.At(i); r.Start != w[0] || r.End != w[1] {
			t.Fatalf("row %d = {%d,%d}, want {%d,%d}", i, r.Start, r.End, w[0], w[1])
		}
	}
}

// In run-off-edge mode the marker extends the row's cell range the
// same way — hit plus its $ marker covers [0,4) — and an empty
// matched line's marker is its single cell: the marker-only row
// [0,1), extent one.
func TestEOLMarkerFlatRows(t *testing.T) {
	rows := viewport.Prepare(loadBufferStops(t, "hit\n", stop(1, [2]int{3, 3})),
		viewport.Key{TextWidth: 10})
	if r := rows.At(0); r.Start != 0 || r.End != 4 {
		t.Fatalf("flat row = {%d,%d}, want {0,4} — the marker cell included", r.Start, r.End)
	}

	rows = viewport.Prepare(loadBufferStops(t, "\n", stop(1, [2]int{0, 0})),
		viewport.Key{TextWidth: 10})
	if rows.Len() != 1 {
		t.Fatalf("Len = %d, want 1", rows.Len())
	}
	if r := rows.At(0); r.Start != 0 || r.End != 1 {
		t.Fatalf("marker-only row = {%d,%d}, want {0,1} — extent 1", r.Start, r.End)
	}
}

// Markers participate in the visible-lines extent policy like any
// other cluster: the end-of-line marker's start feeds the
// paintable-boundary maximum — hit's $ marker allows offset 3, where
// the marker is the only painted cell — and a marker-only line has
// extent 1 with maximum offset 0.
func TestMarkerExtentFeedsMaxOff(t *testing.T) {
	rows := viewport.Prepare(loadBufferStops(t, "hit\n", stop(1, [2]int{3, 3})),
		viewport.Key{TextWidth: 10})
	if got := viewport.MaxOff(rows, 0, 4); got != 3 {
		t.Fatalf("MaxOff = %d, want 3 — the marker's start", got)
	}
	var v viewport.Viewport
	v.Pan(10, rows, 4)
	if v.Off() != 3 {
		t.Fatalf("off = %d, want the marker boundary 3", v.Off())
	}
	if !viewport.CellVisible(rows.At(0).Line, 3, v.Off(), 10) {
		t.Fatal("marker cell not painted at the boundary offset")
	}

	only := viewport.Prepare(loadBufferStops(t, "\n", stop(1, [2]int{0, 0})),
		viewport.Key{TextWidth: 10})
	if got := viewport.MaxOff(only, 0, 4); got != 0 {
		t.Fatalf("marker-only MaxOff = %d, want 0 — extent 1, maximum offset 0", got)
	}
	var w viewport.Viewport
	w.Pan(10, only, 4)
	if w.Off() != 0 {
		t.Fatalf("panning the marker-only line gave off = %d, want 0", w.Off())
	}
}

// A hidden marker drives the indicators exactly like a hidden match:
// entirely hidden left upgrades the gutter to '*', entirely hidden
// right earns the right '*', and a painted marker earns neither. The
// terminator-only $ on hit\r\n follows the same rules — an ordinary
// marker at display column 3.
func TestMarkerHiddenDrivesIndicators(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		span    [2]int
	}{
		{"LF end-of-line marker", "hit\n", [2]int{3, 3}},
		{"CRLF terminator-only marker", "hit\r\n", [2]int{4, 4}},
	} {
		row := viewport.Prepare(loadBufferStops(t, tc.content, stop(1, tc.span)),
			viewport.Key{TextWidth: 10}).At(0)
		if got := viewport.LeftMark(row, 4); got != '*' {
			t.Fatalf("%s: LeftMark at off 4 = %q, want '*' — the marker is entirely hidden left",
				tc.name, got)
		}
		if got := viewport.LeftMark(row, 3); got != '_' {
			t.Fatalf("%s: LeftMark at off 3 = %q, want '_' — marker painted, text hidden left",
				tc.name, got)
		}
		if got := viewport.RightMark(row, 0, 3); got != '*' {
			t.Fatalf("%s: RightMark over width 3 = %q, want '*' — the marker is entirely hidden right",
				tc.name, got)
		}
		if got := viewport.RightMark(row, 0, 4); got != ' ' {
			t.Fatalf("%s: RightMark over width 4 = %q, want ' ' — the marker paints",
				tc.name, got)
		}
	}
}

// The marker cell is a navigable reveal target: the destination
// reveal resolves the first submatch to the marker's cell and the
// target row is the rendered row holding it — the marker's own row
// after a full wrap row, the line's single row otherwise.
func TestMarkerIsRevealTarget(t *testing.T) {
	buf := loadBufferStops(t, "top\nhit\n", stop(2, [2]int{3, 3}))
	st := searchindex.Stop{Number: 2, Submatches: []searchindex.Submatch{{Start: 3, End: 3}}}

	wrapped := viewport.Prepare(buf, viewport.Key{TextWidth: 3, Wrap: true})
	if tg := wrapped.StopTarget(st); tg.Line != 2 || tg.Cell != 3 {
		t.Fatalf("StopTarget = %+v, want {Line 2, Cell 3} — the marker cell", tg)
	}
	// Line 1 is row 0; line 2's text row is 1 and its marker row is 2.
	if got := wrapped.TargetRow(st); got != 2 {
		t.Fatalf("TargetRow = %d, want 2 — the marker's own row", got)
	}
	if got := wrapped.RowOf(viewport.Anchor{Line: 2, Cell: 3}); got != 2 {
		t.Fatalf("RowOf the marker position = %d, want 2", got)
	}

	flat := viewport.Prepare(buf, viewport.Key{TextWidth: 10})
	if got := flat.TargetRow(st); got != 1 {
		t.Fatalf("flat TargetRow = %d, want 1 — the marker joins the line's row", got)
	}

	// The marker-only line's marker is equally a target: the empty
	// line's cell 0.
	only := viewport.Prepare(loadBufferStops(t, "\n", stop(1, [2]int{0, 0})),
		viewport.Key{TextWidth: 10})
	ost := searchindex.Stop{Number: 1, Submatches: []searchindex.Submatch{{Start: 0, End: 0}}}
	if tg := only.StopTarget(ost); tg.Cell != 0 {
		t.Fatalf("marker-only StopTarget cell = %d, want 0", tg.Cell)
	}
	if got := only.TargetRow(ost); got != 0 {
		t.Fatalf("marker-only TargetRow = %d, want 0", got)
	}
}

// A reveal aimed at a marker target row moves the viewport like any
// other reveal: the marker row below the window lands inside it by
// the one-third placement clamped to EOF.
func TestRevealLandsOnMarkerRow(t *testing.T) {
	buf := loadBufferStops(t, "a\nb\nc\nd\nhit\n", stop(5, [2]int{3, 3}))
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 3, Wrap: true})
	st := searchindex.Stop{Number: 5, Submatches: []searchindex.Submatch{{Start: 3, End: 3}}}
	// Lines a–d are rows 0–3, hit's text row is 4, its marker row 5.
	row := rows.TargetRow(st)
	if row != 5 {
		t.Fatalf("TargetRow = %d, want 5 — the marker's own row", row)
	}
	var v viewport.Viewport
	if !v.Reveal(row, rows, 4) {
		t.Fatal("the reveal did not move the viewport — fixture wrong")
	}
	// Row 5 is the last: the EOF clamp puts the top at 2 and the
	// marker row is the last visible row.
	if v.Top() != 2 {
		t.Fatalf("top = %d, want 2 — the marker row visible at the bottom", v.Top())
	}
}
