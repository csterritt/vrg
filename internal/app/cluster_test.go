package app

import (
	"testing"

	"vrg/internal/present"
	"vrg/internal/theme"
)

// The blanks wrap and clip substitute for a split cluster's
// unpaintable cells are never painted as match cells — a highlight
// covering the cluster styles only its real cells — while a marker
// still paints its own position even on a blanked cell.
func TestRenderCellsBlankFillersNeverMatchStyled(t *testing.T) {
	th := theme.Dark()
	inv := func(s string) string { return "\x1b[30;47m" + s + "\x1b[37;40m" }
	for _, tc := range []struct {
		name  string
		cells []present.Cell
		spans []present.Span
		textW int
		want  string
	}{
		// A blank filler between two covered cells stays plain.
		{"covered blank stays plain",
			[]present.Cell{{Text: "x", Lead: true}, {Text: " ", Blank: true}, {Text: "y", Lead: true}},
			[]present.Span{{Start: 0, End: 3}}, 10,
			inv("x") + " " + inv("y")},
		// A cluster split at the clip edge paints its last visible
		// cell blank — unstyled even though the span covers it. The
		// cell is the marked blank clipRow substitutes for the split
		// glyph, and the clipped span still covers it.
		{"clip-edge split blank stays plain",
			[]present.Cell{{Text: "x", Lead: true}, {Text: " ", Blank: true}},
			[]present.Span{{Start: 0, End: 2}}, 2,
			inv("x") + " "},
		// A marker paints its one cell wherever it sits — a blanked
		// cell included.
		{"marker on a blank still paints",
			[]present.Cell{{Text: " ", Blank: true}},
			[]present.Span{{Start: 0, End: 0}}, 10,
			inv(" ")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderCells(tc.cells, tc.spans, tc.textW, th, false); got != tc.want {
				t.Fatalf("renderCells = %q, want %q", got, tc.want)
			}
		})
	}
}
