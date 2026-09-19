package viewport_test

import (
	"strings"
	"testing"

	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// The display target is the start cell of the first submatch on the
// destination line — a display location, not a byte offset: escaped
// bytes widen into several cells, so the target cell differs from the
// submatch's byte start, and with several submatches the earliest one
// supplies the cell.
func TestStopTargetIsFirstSubmatchStartCell(t *testing.T) {
	// Line 2 is "a<ESC>zz target": the ESC byte escapes to the two
	// cells '^' '[', so byte 5 — the 't' of "target" — is display
	// cell 6, and a naive byte offset would be wrong.
	buf := loadBuffer(t, "first\na\x1bzz target\n")
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 80, Wrap: true})

	st := searchindex.Stop{Number: 2, Submatches: []searchindex.Submatch{
		{Start: 5, End: 11},
		{Start: 9, End: 11},
	}}
	tg := rows.StopTarget(st)
	if tg.Line != 2 || tg.Cell != 6 {
		t.Fatalf("StopTarget = %+v, want {Line: 2, Cell: 6}", tg)
	}

	// A submatch on the ESC byte itself targets the first cell of its
	// two-cell escape.
	tg = rows.StopTarget(searchindex.Stop{Number: 2, Submatches: []searchindex.Submatch{
		{Start: 1, End: 2},
	}})
	if tg.Cell != 1 {
		t.Fatalf("StopTarget cell for the escaped byte = %d, want 1", tg.Cell)
	}
}

// A submatch whose bytes produced no display cell — a zero-width
// position at end of line — targets the marker cell one past the
// line's last cell, the Issue #23 marker position.
func TestStopTargetZeroWidthLandsPastLastCell(t *testing.T) {
	buf := loadBuffer(t, "hit\n")
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 80, Wrap: true})
	tg := rows.StopTarget(searchindex.Stop{Number: 1, Submatches: []searchindex.Submatch{
		{Start: 3, End: 3},
	}})
	if tg.Cell != 3 {
		t.Fatalf("zero-width target cell = %d, want 3 (one past the last cell)", tg.Cell)
	}
}

// The target row is the rendered row containing the display target:
// with the match at cell 0 it is the destination line's first row. A
// line number outside the prepared rows clamps to the nearest real row.
func TestTargetRow(t *testing.T) {
	buf := loadBuffer(t, "one\ntwo\nthree\n")
	rows := viewport.Prepare(buf, viewport.Key{TextWidth: 80, Wrap: true})
	stop := func(n int64) searchindex.Stop {
		return searchindex.Stop{Number: n, Submatches: []searchindex.Submatch{{Start: 0, End: 1}}}
	}
	cases := []struct {
		name string
		line int64
		want int
	}{
		{"first line", 1, 0},
		{"middle line", 2, 1},
		{"last line", 3, 2},
		{"line zero clamps", 0, 0},
		{"past EOF clamps", 99, 2},
	}
	for _, c := range cases {
		if got := rows.TargetRow(stop(c.line)); got != c.want {
			t.Fatalf("%s: TargetRow = %d, want %d", c.name, got, c.want)
		}
	}
	if got := viewport.Prepare(nil, viewport.Key{Wrap: true}).TargetRow(stop(7)); got != 0 {
		t.Fatalf("empty model TargetRow = %d, want 0", got)
	}
}

// atTop returns a viewport scrolled to top rendered rows down — the
// saved position a revisit would resume from.
func atTop(top, rows, height int) viewport.Viewport {
	var v viewport.Viewport
	v.Scroll(top, seqRows(rows), height)
	return v
}

// An already-visible target row leaves the viewport unchanged —
// including the first and last visible rows — and reports no move.
func TestRevealVisibleTargetNoScroll(t *testing.T) {
	const rows, height = 100, 10
	for _, target := range []int{10, 15, 19} {
		v := atTop(10, rows, height)
		if moved := v.Reveal(target, seqRows(rows), height); moved {
			t.Fatalf("target %d visible in [10,20) reported a move", target)
		}
		if v.Top() != 10 {
			t.Fatalf("target %d visible in [10,20) moved top to %d, want 10", target, v.Top())
		}
	}
}

// A hidden target row lands at zero-based row floor(height / 3):
// the top moves to target − floor(h/3), whether the target sits below
// or above the window.
func TestRevealHiddenTargetOneThird(t *testing.T) {
	const rows, height = 100, 9 // floor(9/3) = 3

	v := atTop(0, rows, height)
	if moved := v.Reveal(50, seqRows(rows), height); !moved {
		t.Fatal("hidden target below reported no move")
	}
	if v.Top() != 47 {
		t.Fatalf("target row 50 with height 9 gave top = %d, want 47 (row 3)", v.Top())
	}

	v = atTop(50, rows, height)
	if moved := v.Reveal(10, seqRows(rows), height); !moved {
		t.Fatal("hidden target above reported no move")
	}
	if v.Top() != 7 {
		t.Fatalf("target row 10 with height 9 gave top = %d, want 7 (row 3)", v.Top())
	}
}

// At BOF and EOF, available content takes precedence over one-third
// placement: the target lands higher than floor(h/3) near the top of
// the file and lower near the end.
func TestRevealBOFEOFClamps(t *testing.T) {
	const rows, height = 60, 10 // floor(10/3) = 3, MaxTop = 50

	v := atTop(20, rows, height)
	v.Reveal(2, seqRows(rows), height)
	if v.Top() != 0 {
		t.Fatalf("target row 2 gave top = %d, want 0 — BOF beats one-third", v.Top())
	}

	v = atTop(0, rows, height)
	v.Reveal(59, seqRows(rows), height)
	if v.Top() != 50 {
		t.Fatalf("target row 59 gave top = %d, want the EOF clamp %d", v.Top(), 50)
	}

	// A file barely taller than the viewport clamps likewise.
	v = atTop(0, 12, height)
	v.Reveal(11, seqRows(12), height)
	if v.Top() != 2 {
		t.Fatalf("target row 11 of 12 gave top = %d, want 2", v.Top())
	}
}

// The reveal applies relative to whatever top the viewport starts
// from — the saved per-file viewport on a revisit or the top of the
// file on a first visit: the same target stays put when visible from
// the saved top but moves a first-visit viewport it is hidden from.
func TestRevealStartsFromCurrentTop(t *testing.T) {
	const rows, height = 100, 10

	revisit := atTop(12, rows, height) // a saved position
	if moved := revisit.Reveal(8, seqRows(rows), height); !moved || revisit.Top() != 5 {
		t.Fatalf("target 8 hidden above saved top 12: moved=%v top=%d, want moved to 5",
			moved, revisit.Top())
	}

	fresh := atTop(0, rows, height) // a first visit: the top of the file
	if moved := fresh.Reveal(8, seqRows(rows), height); moved || fresh.Top() != 0 {
		t.Fatalf("target 8 visible from top 0: moved=%v top=%d, want no move",
			moved, fresh.Top())
	}
}

// --- Issue #19: minimal horizontal reveal ---------------------------

// A target cell hidden right of the text window moves the offset by
// exactly enough to paint its whole cluster at the right edge: a
// single-cell target lands at T − (width − 1), a two-cell cluster at
// T + 2 − width so both cells fit.
func TestRevealOffRightOfView(t *testing.T) {
	rows := flatRows(t, strings.Repeat("x", 300)+"\n", 10)
	var v viewport.Viewport
	if !v.RevealOff(viewport.Target{Line: 1, Cell: 50}, 0, rows) {
		t.Fatal("hidden-right target reported no move")
	}
	if v.Off() != 41 {
		t.Fatalf("off = %d, want 41 = 50 − (10 − 1)", v.Off())
	}
	if !viewport.CellVisible(rows.At(0).Line, 50, v.Off(), 10) {
		t.Fatal("target cell not painted after the right reveal")
	}

	// 文 is one two-cell cluster spanning cells 298–299: revealing it
	// needs offset 290 so the whole cluster paints.
	wide := flatRows(t, strings.Repeat("x", 298)+"文\n", 10)
	var w viewport.Viewport
	if !w.RevealOff(viewport.Target{Line: 1, Cell: 298}, 0, wide) {
		t.Fatal("hidden-right two-cell target reported no move")
	}
	if w.Off() != 290 {
		t.Fatalf("two-cell target off = %d, want 290 = 298 + 2 − 10", w.Off())
	}
	if !viewport.CellVisible(wide.At(0).Line, 298, w.Off(), 10) {
		t.Fatal("two-cell cluster not fully painted at the right edge")
	}
}

// A target hidden left of the window — including a wide cluster whose
// start cell is left of the offset — lands at its start column: the
// smallest leftward move that paints the whole cluster.
func TestRevealOffLeftOfView(t *testing.T) {
	rows := flatRows(t, strings.Repeat("x", 300)+"\n", 10)
	var v viewport.Viewport
	v.Pan(50, rows, 4)
	if !v.RevealOff(viewport.Target{Line: 1, Cell: 20}, 0, rows) {
		t.Fatal("hidden-left target reported no move")
	}
	if v.Off() != 20 {
		t.Fatalf("off = %d, want 20 — the target column", v.Off())
	}

	// 文 spans cells 20–21; at offset 50 its start is hidden left.
	wide := flatRows(t, strings.Repeat("x", 20)+"文"+strings.Repeat("x", 100)+"\n", 10)
	var w viewport.Viewport
	w.Pan(50, wide, 4)
	if !w.RevealOff(viewport.Target{Line: 1, Cell: 20}, 0, wide) {
		t.Fatal("hidden-left two-cell target reported no move")
	}
	if w.Off() != 20 {
		t.Fatalf("two-cell target off = %d, want 20 — the cluster's start", w.Off())
	}
}

// A target whose start cell is already painted leaves the offset
// unchanged — at the window's left edge, middle, and last cell, and
// for a wide cluster painted whole at the right edge.
func TestRevealOffPaintedTargetKeepsOffset(t *testing.T) {
	// 文 spans cells 18–19: at offset 10 and width 10 it paints whole
	// against the right edge.
	rows := flatRows(t, strings.Repeat("x", 18)+"文"+strings.Repeat("x", 100)+"\n", 10)
	var v viewport.Viewport
	v.Pan(10, rows, 4)
	for _, cell := range []int{10, 15, 18} {
		if v.RevealOff(viewport.Target{Line: 1, Cell: cell}, 0, rows) {
			t.Fatalf("painted target cell %d reported a move", cell)
		}
		if v.Off() != 10 {
			t.Fatalf("painted target cell %d moved off to %d, want 10", cell, v.Off())
		}
	}
}

// A target geometrically inside the window but clipped to a blank —
// its two-cell cluster cut by the right edge — counts as hidden: the
// reveal moves until the whole cluster paints.
func TestRevealOffClippedBlankCountsHidden(t *testing.T) {
	// 文 spans cells 9–10: at offset 0 and width 10 its start cell is
	// inside the window's geometry but the cluster does not fit, so
	// the cell renders as a clipping blank.
	rows := flatRows(t, strings.Repeat("x", 9)+"文"+strings.Repeat("x", 100)+"\n", 10)
	line := rows.At(0).Line
	if viewport.CellVisible(line, 9, 0, 10) {
		t.Fatal("fixture wrong: cell 9 should be clipped-blank hidden at offset 0")
	}
	var v viewport.Viewport
	if !v.RevealOff(viewport.Target{Line: 1, Cell: 9}, 0, rows) {
		t.Fatal("clipped-blank target reported no move")
	}
	if v.Off() != 1 {
		t.Fatalf("off = %d, want 1 = 9 + 2 − 10", v.Off())
	}
	if !viewport.CellVisible(line, 9, v.Off(), 10) {
		t.Fatal("target still not painted after the reveal")
	}
}

// A match wider than the text area is revealed by its start cell
// alone: the reveal paints that cell and a repeated reveal — the
// span's tail still hidden — moves nothing further.
func TestRevealOffOversizedSpanByStartCell(t *testing.T) {
	// A 250-cell match starting at cell 50 of a 300-cell line far
	// exceeds the 10-cell text width; the reveal contract is still
	// just the start cell.
	rows := flatRows(t, strings.Repeat("x", 300)+"\n", 10)
	var v viewport.Viewport
	if !v.RevealOff(viewport.Target{Line: 1, Cell: 50}, 0, rows) {
		t.Fatal("oversized match's start reported no move")
	}
	if v.Off() != 41 {
		t.Fatalf("off = %d, want 41 — the start cell at the right edge", v.Off())
	}
	if v.RevealOff(viewport.Target{Line: 1, Cell: 50}, 0, rows) {
		t.Fatal("repeat reveal moved the offset chasing full-span visibility")
	}
}

// A target cluster wider than the whole text area can never paint:
// the reveal sets the offset to its start column — the closest
// achievable position — and treats the target as geometrically
// revealed, so a repeated reveal moves nothing (no panning loop).
// CellVisible still reports it unpainted: Issue #20's indicators
// count it as not visible.
func TestRevealOffUnpaintableClusterFallback(t *testing.T) {
	// The tab expands cells 2–7 as one six-cell cluster — unpaintable
	// at text width 5.
	rows := flatRows(t, "ab\t\n", 5)
	var v viewport.Viewport
	if !v.RevealOff(viewport.Target{Line: 1, Cell: 2}, 0, rows) {
		t.Fatal("unpaintable target reported no move")
	}
	if v.Off() != 2 {
		t.Fatalf("off = %d, want the start column 2", v.Off())
	}
	if viewport.CellVisible(rows.At(0).Line, 2, v.Off(), 5) {
		t.Fatal("unpaintable cluster reported visible — indicators must count it hidden")
	}
	if v.RevealOff(viewport.Target{Line: 1, Cell: 2}, 0, rows) {
		t.Fatal("repeated reveal moved the offset — the fallback must not loop")
	}
}

// The end-of-line marker position — one cell past the line's last —
// is a one-cell target following the same reveal rules.
func TestRevealOffMarkerCell(t *testing.T) {
	rows := flatRows(t, "hit\n", 2)
	var v viewport.Viewport
	if !v.RevealOff(viewport.Target{Line: 1, Cell: 3}, 0, rows) {
		t.Fatal("marker target reported no move")
	}
	if v.Off() != 2 {
		t.Fatalf("off = %d, want 2 = 3 + 1 − 2", v.Off())
	}
}

// The horizontal reveal is a strict no-op under a wrap model and on an
// empty model.
func TestRevealOffNoOpWrapAndEmpty(t *testing.T) {
	var v viewport.Viewport
	wrapped := wrappedRows(t, strings.Repeat("x", 300)+"\n", 10)
	if v.RevealOff(viewport.Target{Line: 1, Cell: 250}, 0, wrapped) || v.Off() != 0 {
		t.Fatalf("wrap-mode reveal gave off = %d, want a no-op", v.Off())
	}
	empty := viewport.Prepare(nil, viewport.Key{TextWidth: 10})
	if v.RevealOff(viewport.Target{Line: 1, Cell: 5}, 0, empty) || v.Off() != 0 {
		t.Fatalf("empty-model reveal gave off = %d, want a no-op", v.Off())
	}
}

// CellVisible is the painted-cell visibility oracle: a cell counts
// only when its whole grapheme cluster fits inside the window — a
// cluster clipped at either edge renders its in-window cells as
// blanks. The cell past the line's end is the one-cell marker
// position.
func TestCellVisible(t *testing.T) {
	// "ab文cd": 文 spans cells 2–3; the line is six cells wide.
	line := flatRows(t, "ab文cd\n", 10).At(0).Line
	cases := []struct {
		name         string
		cell, off, w int
		want         bool
	}{
		{"single cell inside", 0, 0, 5, true},
		{"wide cluster inside", 2, 0, 5, true},
		{"wide cluster painted at its start", 2, 2, 2, true},
		{"continuation cell of a painted cluster", 3, 0, 5, true},
		{"clipped at the right edge", 2, 0, 3, false},
		{"clipped at the left edge", 2, 3, 5, false},
		{"marker at the line end", 6, 2, 5, true},
		{"marker outside the window", 6, 0, 5, false},
	}
	for _, c := range cases {
		if got := viewport.CellVisible(line, c.cell, c.off, c.w); got != c.want {
			t.Fatalf("%s: CellVisible(cell=%d, off=%d, w=%d) = %v, want %v",
				c.name, c.cell, c.off, c.w, got, c.want)
		}
	}
}
