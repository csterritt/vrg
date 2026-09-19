package viewport_test

import (
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
	v.Scroll(top, rows, height)
	return v
}

// An already-visible target row leaves the viewport unchanged —
// including the first and last visible rows — and reports no move.
func TestRevealVisibleTargetNoScroll(t *testing.T) {
	const rows, height = 100, 10
	for _, target := range []int{10, 15, 19} {
		v := atTop(10, rows, height)
		if moved := v.Reveal(target, rows, height); moved {
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
	if moved := v.Reveal(50, rows, height); !moved {
		t.Fatal("hidden target below reported no move")
	}
	if v.Top() != 47 {
		t.Fatalf("target row 50 with height 9 gave top = %d, want 47 (row 3)", v.Top())
	}

	v = atTop(50, rows, height)
	if moved := v.Reveal(10, rows, height); !moved {
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
	v.Reveal(2, rows, height)
	if v.Top() != 0 {
		t.Fatalf("target row 2 gave top = %d, want 0 — BOF beats one-third", v.Top())
	}

	v = atTop(0, rows, height)
	v.Reveal(59, rows, height)
	if v.Top() != 50 {
		t.Fatalf("target row 59 gave top = %d, want the EOF clamp %d", v.Top(), 50)
	}

	// A file barely taller than the viewport clamps likewise.
	v = atTop(0, 12, height)
	v.Reveal(11, 12, height)
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
	if moved := revisit.Reveal(8, rows, height); !moved || revisit.Top() != 5 {
		t.Fatalf("target 8 hidden above saved top 12: moved=%v top=%d, want moved to 5",
			moved, revisit.Top())
	}

	fresh := atTop(0, rows, height) // a first visit: the top of the file
	if moved := fresh.Reveal(8, rows, height); moved || fresh.Top() != 0 {
		t.Fatalf("target 8 visible from top 0: moved=%v top=%d, want no move",
			moved, fresh.Top())
	}
}
