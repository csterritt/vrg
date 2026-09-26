package viewport

import (
	"testing"

	"vrg/internal/present"
)

// The blank a wrap boundary substitutes for a split cluster's lead
// cell is marked Blank so the frame never paints it as a match cell,
// even when a highlight covers the cluster: the span still spans it,
// but the cell itself knows it is filler.
func TestWrapSplitBlankIsMarked(t *testing.T) {
	// "a\xc2\x85b": the six-cell \u0085 escape cluster is cells 1–6 —
	// wider than the width-2 row, it splits as a last resort and
	// row 1 leads with the blanked lead cell.
	src := source(t, "a\xc2\x85b\n",
		map[int][]present.Span{0: {{Start: 1, End: 7}}})
	m := Prepare(src, Key{Width: 2, Wrap: true})
	r := m.Row(1)
	if len(r.Cells) != 2 || !r.Cells[0].Blank || r.Cells[0].Text != " " {
		t.Fatalf("row 1 cells = %+v, want a marked blank leading", r.Cells)
	}
	if len(r.Spans) != 1 || r.Spans[0] != (present.Span{Start: 0, End: 2}) {
		t.Fatalf("row 1 spans = %+v, want [{0 2}] — the cover reaches the blank", r.Spans)
	}
	// The rows continuing the split cluster carry Cont cells —
	// already textless — none of them blank-marked.
	for _, i := range []int{2, 3} {
		for j, c := range m.Row(i).Cells {
			if c.Blank {
				t.Fatalf("row %d cell %d = %+v, want an unmarked Cont cell", i, j, c)
			}
		}
	}
}

// A cluster split by a clip edge paints its in-window cells as
// marked blanks: a highlight covering the cluster must not style
// them, and the indicator accounting — which sees the unclipped
// line — still counts the match as hidden on the split side.
func TestClipBlanksAreMarked(t *testing.T) {
	// 世 covers cells 4–5; at offset 0 width 5 its lead cell clips
	// to a blank inside the window.
	r := hid(t, 5, 0, "xxxx世yyyy\n", []present.Span{{Start: 4, End: 6}})
	if len(r.Cells) != 5 || !r.Cells[4].Blank || r.Cells[4].Text != " " {
		t.Fatalf("row cells = %+v, want a marked blank at 4", r.Cells)
	}
	if len(r.Spans) != 1 || r.Spans[0] != (present.Span{Start: 4, End: 5}) {
		t.Fatalf("row spans = %+v, want [{4 5}]", r.Spans)
	}
	// The left-edge split blanks the trailing cell instead.
	r = hid(t, 5, 5, "xxxx世yyyy\n", []present.Span{{Start: 4, End: 6}})
	if len(r.Cells) != 5 || !r.Cells[0].Blank || r.Cells[0].Text != " " {
		t.Fatalf("row cells = %+v, want a marked blank at 0", r.Cells)
	}
}
