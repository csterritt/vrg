package viewport_test

import (
	"strings"
	"testing"

	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// A wrapped source line becomes one rendered row per text-width band:
// rows partition the line's cells in order, the first row carries the
// line's identity, later rows are continuations, and even an empty
// line yields one row.
func TestWrapRowCountsASCII(t *testing.T) {
	buf := loadBuffer(t, "abcdefghijklm\nxy\n\n")
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 5, Wrap: true})
	want := []struct {
		line       int64
		start, end int
		cont       bool
	}{
		{1, 0, 5, false}, {1, 5, 10, true}, {1, 10, 13, true},
		{2, 0, 2, false},
		{3, 0, 0, false},
	}
	if rows.Len() != len(want) {
		t.Fatalf("Len = %d, want %d", rows.Len(), len(want))
	}
	for i, w := range want {
		r := rows.At(i)
		if r.Line.Number != w.line || r.Start != w.start || r.End != w.end {
			t.Fatalf("row %d = {line %d, cells [%d,%d)}, want {line %d, cells [%d,%d)}",
				i, r.Line.Number, r.Start, r.End, w.line, w.start, w.end)
		}
		if r.Continuation() != w.cont {
			t.Fatalf("row %d Continuation = %v, want %v", i, r.Continuation(), w.cont)
		}
	}
}

// A grapheme cluster that does not fit in a row's remaining cells
// moves to the next row whole — the vacated cells stay blank — so a
// two-cell glyph never straddles a row boundary.
func TestWrapMovesWideClusterToNextRow(t *testing.T) {
	// "aaa文b": 文 occupies cells 3-4; at text width 4 it cannot fit in
	// row 0's last cell, so row 0 ends after "aaa" leaving a blank.
	buf := loadBuffer(t, "aaa文b\n")
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 4, Wrap: true})
	if rows.Len() != 2 {
		t.Fatalf("Len = %d, want 2", rows.Len())
	}
	if r := rows.At(0); r.Start != 0 || r.End != 3 {
		t.Fatalf("row 0 covers cells [%d,%d), want [0,3) leaving the fourth cell blank", r.Start, r.End)
	}
	if r := rows.At(1); r.Start != 3 || r.End != 6 {
		t.Fatalf("row 1 covers cells [%d,%d), want [3,6)", r.Start, r.End)
	}
}

// A multi-rune grapheme cluster — here a regional-indicator flag pair —
// is a single unbreakable unit: it wraps to the next row whole rather
// than splitting across the boundary.
func TestWrapKeepsClustersTogether(t *testing.T) {
	// "x🇩🇪y": the flag's two cells form one cluster; at width 2 it
	// cannot share x's row or split, so it wraps whole.
	buf := loadBuffer(t, "x🇩🇪y\n")
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 2, Wrap: true})
	want := []struct{ start, end int }{{0, 1}, {1, 3}, {3, 4}}
	if rows.Len() != len(want) {
		t.Fatalf("Len = %d, want %d", rows.Len(), len(want))
	}
	for i, w := range want {
		if r := rows.At(i); r.Start != w.start || r.End != w.end {
			t.Fatalf("row %d covers cells [%d,%d), want [%d,%d)", i, r.Start, r.End, w.start, w.end)
		}
	}
}

// A tab's expansion is one cluster: it wraps whole, never splits
// across rows, and its cells stay blank.
func TestWrapTabIsOneCluster(t *testing.T) {
	// "ab\tc": the tab expands cells 2-7 (to column 8); at width 5 the
	// six-cell cluster cannot split, so it moves to the next row whole.
	buf := loadBuffer(t, "ab\tc\n")
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 5, Wrap: true})
	want := []struct{ start, end int }{{0, 2}, {2, 8}, {8, 9}}
	if rows.Len() != len(want) {
		t.Fatalf("Len = %d, want %d", rows.Len(), len(want))
	}
	for i, w := range want {
		if r := rows.At(i); r.Start != w.start || r.End != w.end {
			t.Fatalf("row %d covers cells [%d,%d), want [%d,%d)", i, r.Start, r.End, w.start, w.end)
		}
	}
}

// Every wrapped row boundary coincides with a cluster boundary the
// buffer exposed — Viewport wraps on FileBuffer's segmentation, never
// re-deriving it.
func TestWrapRowsAlignToClusterBoundaries(t *testing.T) {
	buf := loadBuffer(t, "a文b\te\u0301世\tend\n")
	for _, width := range []int{1, 2, 3, 4, 5, 7, 8, 9, 13} {
		rows := viewport.Prepare(buf, viewport.Key{TextWidth: width, Wrap: true})
		l := buf.Lines()[0]
		bounds := map[int]bool{0: true, len(l.Cells): true}
		for _, cl := range l.Clusters {
			bounds[cl.Start] = true
			bounds[cl.End] = true
		}
		for i := 0; i < rows.Len(); i++ {
			r := rows.At(i)
			if !bounds[r.Start] || !bounds[r.End] {
				t.Fatalf("width %d row %d covers [%d,%d), not on cluster boundaries %v",
					width, i, r.Start, r.End, bounds)
			}
		}
	}
}

// Run-off-edge mode renders each source line as exactly one row
// spanning all its cells, however wide the line or narrow the text
// area — clipping is a render concern, not a row-model one.
func TestRunOffEdgeOneRowPerLine(t *testing.T) {
	buf := loadBuffer(t, "abcdefghijklm\nxy\n\n")
	for _, w := range []int{0, 1, 5, 80} {
		rows := viewport.Prepare(buf, viewport.Key{TextWidth: w, Wrap: false})
		if rows.Len() != 3 {
			t.Fatalf("width %d: Len = %d, want one row per source line", w, rows.Len())
		}
		if r := rows.At(0); r.Start != 0 || r.End != 13 || r.Continuation() {
			t.Fatalf("width %d: row 0 = %+v, want the whole line", w, r)
		}
	}
}

// The prepared row model records the key it was built for — path,
// content revision, text width, wrap mode — so a stale model is
// detectable and the model replaces atomically as one value.
func TestRowsCarryTheirKey(t *testing.T) {
	buf := loadBuffer(t, "abcdefghijklm\n")
	key := viewport.Key{Path: "p", Revision: 3, TextWidth: 5, Wrap: true}
	rows := viewport.Prepare(buf, key)
	if rows.Key() != key {
		t.Fatalf("Key = %+v, want %+v", rows.Key(), key)
	}
	// A different mode or width is a different model, not a mutation.
	if got := viewport.Prepare(buf, viewport.Key{Path: "p", Revision: 3, TextWidth: 5, Wrap: false}); got.Len() != 1 {
		t.Fatalf("run-off-edge Len = %d, want 1 — the toggled model is a separate value", got.Len())
	}
	if got := viewport.Prepare(buf, viewport.Key{Path: "p", Revision: 4, TextWidth: 6, Wrap: true}); got.Len() != 3 || got.Key() == key {
		t.Fatalf("reprepared model = %+v, want a new 3-row model under a new key", got.Key())
	}
}

// The display target resolves to the rendered row holding the match
// start cell inside the wrapped line — not merely the line's first
// row. In run-off-edge mode every target on the line resolves to its
// single row.
func TestWrappedTargetRow(t *testing.T) {
	// 95 cells at width 10 → rows 0-9 of the model; a match starting at
	// cell 90 sits on the last of them.
	buf := loadBuffer(t, strings.Repeat("x", 95)+"\nsecond\n")
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 10, Wrap: true})
	stop := func(start int) searchindex.Stop {
		return searchindex.Stop{Number: 1, Submatches: []searchindex.Submatch{{Start: start, End: start + 1}}}
	}
	for _, c := range []struct{ start, want int }{
		{0, 0}, {9, 0}, {10, 1}, {45, 4}, {89, 8}, {90, 9}, {94, 9},
	} {
		if got := rows.TargetRow(stop(c.start)); got != c.want {
			t.Fatalf("TargetRow(start %d) = %d, want row %d", c.start, got, c.want)
		}
	}
	flat := viewport.Prepare(buf, viewport.Key{TextWidth: 10, Wrap: false})
	if got := flat.TargetRow(stop(90)); got != 0 {
		t.Fatalf("run-off-edge TargetRow = %d, want the line's one row", got)
	}
}

// A source line taller than several screens: a match near its end
// reveals the wrapped row containing the match start, landing at
// floor(content height / 3) per the Issue #14 placement rule.
func TestRevealDeepInWrappedLine(t *testing.T) {
	// 250 cells at width 10 → 25 rows, then 20 short lines; content
	// height 6 → floor(6/3) = 2, so a target on row 24 moves the top
	// to 22 without hitting the EOF clamp.
	var sb strings.Builder
	sb.WriteString(strings.Repeat("x", 240) + "needle end\n")
	for i := 0; i < 20; i++ {
		sb.WriteString("pad\n")
	}
	buf := loadBuffer(t, sb.String())
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 10, Wrap: true})
	if rows.Len() != 45 {
		t.Fatalf("Len = %d, want 25 wrapped rows + 20 pad rows", rows.Len())
	}
	target := rows.TargetRow(searchindex.Stop{Number: 1, Submatches: []searchindex.Submatch{
		{Start: 240, End: 246},
	}})
	if target != 24 {
		t.Fatalf("TargetRow = %d, want 24 (cells 240-249)", target)
	}
	var v viewport.Viewport
	if !v.Reveal(target, rows.Len(), 6) {
		t.Fatal("hidden target reported no move")
	}
	if v.Top() != 22 {
		t.Fatalf("top = %d, want 22 — target row 24 lands at index floor(6/3) = 2", v.Top())
	}
}
