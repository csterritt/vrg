package viewport

import (
	"slices"
	"testing"
)

// mappingRows is a prepared-rows fake whose display-target mapping is
// programmable: rowOf answers the rendered row containing a (line,
// cell) target, and seen records every target it was asked about. The
// identity mapping of countingRows covers unwrapped mode; this fake
// proves the reveal consumes a rendered row, not a source-line
// ordinal — the wrap-mode seam.
type mappingRows struct {
	n     int
	rowOf func(Target) int
	seen  []Target
}

func (r *mappingRows) Len() int { return r.n }

func (r *mappingRows) Row(i int) Row { return Row{Line: i} }

func (r *mappingRows) RowOf(t Target) int {
	r.seen = append(r.seen, t)
	return r.rowOf(t)
}

// An already-visible target row never scrolls: every position inside
// the visible window — first, middle, and last row — leaves the top
// unchanged and reports no movement.
func TestRevealVisibleTargetDoesNotScroll(t *testing.T) {
	for _, target := range []int{20, 25, 29} {
		var v Viewport
		v.Resize(10, 10)
		v.SetRows(rowsN(100))
		v.SetTop(20)
		if v.Reveal(Target{Line: target}) {
			t.Fatalf("Reveal of visible row %d reported a move", target)
		}
		if v.Top() != 20 {
			t.Fatalf("visible target %d moved top to %d, want 20", target, v.Top())
		}
	}
}

// A hidden target lands on zero-based row floor(height/3): the top
// moves to target − floor(height/3), whether the target is below or
// above the window.
func TestRevealHiddenTargetLandsOneThirdDown(t *testing.T) {
	for _, tc := range []struct {
		name   string
		top    int
		target int
		want   int
	}{
		{"below the window", 0, 40, 36}, // 40 - 4
		{"above the window", 50, 10, 6}, // 10 - 4
		{"just past the last row", 0, 12, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var v Viewport
			v.Resize(10, 12) // floor(12/3) = 4
			v.SetRows(rowsN(100))
			v.SetTop(tc.top)
			if !v.Reveal(Target{Line: tc.target}) {
				t.Fatalf("Reveal of hidden row %d reported no move", tc.target)
			}
			if v.Top() != tc.want {
				t.Fatalf("top = %d, want %d so row %d sits at row 4", v.Top(), tc.want, tc.target)
			}
		})
	}
}

// Near BOF the zero clamp wins over one-third placement: the target
// sits higher than the one-third row rather than pushing content down.
func TestRevealTargetNearBOFClampsToTop(t *testing.T) {
	var v Viewport
	v.Resize(10, 12)
	v.SetRows(rowsN(100))
	v.SetTop(10)
	if !v.Reveal(Target{Line: 2}) {
		t.Fatal("Reveal of hidden row 2 reported no move")
	}
	if v.Top() != 0 {
		t.Fatalf("top = %d, want 0 — BOF clamp beats the one-third row", v.Top())
	}
}

// Near EOF the last-full-page clamp wins: the target lands below the
// one-third row rather than leaving avoidable blanks under EOF.
func TestRevealTargetNearEOFClampsToLastPage(t *testing.T) {
	var v Viewport
	v.Resize(10, 12)
	v.SetRows(rowsN(30)) // maxTop = 18
	for _, target := range []int{25, 29} {
		v.SetTop(0)
		if !v.Reveal(Target{Line: target}) {
			t.Fatalf("Reveal of hidden row %d reported no move", target)
		}
		if v.Top() != 18 {
			t.Fatalf("target %d: top = %d, want the EOF clamp 18", target, v.Top())
		}
	}
}

// The display target is a (line, cell) display location resolved to
// its rendered row by the prepared-rows provider — not merely a
// source-line ordinal — so a wrap-style many-to-one mapping lands the
// containing row. The provider sees the exact target cell.
func TestRevealUsesRenderedRowContainingTarget(t *testing.T) {
	var v Viewport
	v.Resize(10, 10)
	rows := &mappingRows{n: 100, rowOf: func(t Target) int {
		// A wrap-like mapping: the cell chooses which of the line's
		// rendered rows contains it.
		return t.Line*2 + t.Cell/5
	}}
	v.SetRows(rows)
	if !v.Reveal(Target{Line: 7, Cell: 12}) {
		t.Fatal("Reveal reported no move")
	}
	// Row 7*2+2 = 16 holds the target cell; floor(10/3) = 3.
	if v.Top() != 13 {
		t.Fatalf("top = %d, want 13 — the rendered row, not the source line", v.Top())
	}
	if !slices.Equal(rows.seen, []Target{{Line: 7, Cell: 12}}) {
		t.Fatalf("provider saw targets %v, want exactly [{7 12}]", rows.seen)
	}
}

// The starting-viewport sequence on entry: a saved top is restored by
// SetTop and a reveal overrides it only when the target is hidden; a
// first visit starts at the top of the file before the reveal.
func TestRevealAfterSavedAndTopStartingPoints(t *testing.T) {
	// Revisit: the saved top already shows the target, so the reveal
	// is a no-scroll and the saved position survives untouched.
	var v Viewport
	v.Resize(10, 10)
	v.SetRows(rowsN(100))
	v.SetTop(40)
	if v.Reveal(Target{Line: 45}) {
		t.Fatal("revisit with a visible target reported a move")
	}
	if v.Top() != 40 {
		t.Fatalf("revisit top = %d, want the saved 40", v.Top())
	}

	// Revisit where the saved top hides the target: the reveal moves
	// the viewport and reports it, so the caller can replace the
	// saved state.
	v.SetTop(40)
	if !v.Reveal(Target{Line: 90}) {
		t.Fatal("revisit with a hidden target reported no move")
	}
	if v.Top() != 87 {
		t.Fatalf("revisit top = %d, want 87", v.Top())
	}

	// First visit: the top-of-file start then the same one-third
	// reveal.
	var f Viewport
	f.Resize(10, 10)
	f.SetRows(rowsN(100))
	if !f.Reveal(Target{Line: 30}) {
		t.Fatal("first-visit reveal reported no move")
	}
	if f.Top() != 27 {
		t.Fatalf("first-visit top = %d, want 27", f.Top())
	}
}

// A target resolving outside the content clamps to the nearest real
// row before placement — and the resulting top still clamps to the
// last full page.
func TestRevealClampsOutOfRangeTarget(t *testing.T) {
	var v Viewport
	v.Resize(10, 10) // floor(10/3) = 3; 20 rows → maxTop 10
	v.SetRows(&mappingRows{n: 20, rowOf: func(Target) int { return 99 }})
	if !v.Reveal(Target{Line: 99}) {
		t.Fatal("Reveal reported no move")
	}
	// Row 99 clamps to 19; 19-3 = 16 exceeds the last full page, so
	// the EOF clamp puts the target on the bottom row.
	if v.Top() != 10 {
		t.Fatalf("top = %d, want the EOF clamp 10", v.Top())
	}
}

// Unavailable content makes a reveal a no-op: no rows means no target
// row and no movement.
func TestRevealWithoutContentIsNoOp(t *testing.T) {
	var v Viewport
	v.Resize(10, 10)
	if v.Reveal(Target{Line: 5}) {
		t.Fatal("Reveal on nil rows reported a move")
	}
	if v.Top() != 0 {
		t.Fatalf("top = %d, want 0", v.Top())
	}
	v.SetRows(rowsN(0))
	if v.Reveal(Target{Line: 5}) {
		t.Fatal("Reveal on empty rows reported a move")
	}
	if v.Top() != 0 {
		t.Fatalf("top = %d, want 0", v.Top())
	}
}
