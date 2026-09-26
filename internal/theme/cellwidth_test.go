package theme

import (
	"strings"
	"testing"

	"vrg/internal/present"
)

// Overlay sizing and padding run through the shared grapheme/cell
// helper, so wide and combining text measures by the cells it paints
// and every border column of the box aligns.
func TestOverlaySizesByMeasuredCellWidth(t *testing.T) {
	lines := []string{"世界", "éx", "👨‍👩‍👧", "ab"}
	w := 0
	for _, l := range lines {
		if n := present.CellWidth(l); n > w {
			w = n
		}
	}
	if w != 4 {
		t.Fatalf("fixture's widest line is %d cells, want 4", w)
	}
	for _, th := range []Theme{Plain(), Dark()} {
		rows := strings.Split(th.Overlay(lines), "\n")
		for i, row := range rows {
			if got := present.CellWidth(row); got != w+2 {
				t.Fatalf("overlay row %d = %q is %d cells, want %d — borders unaligned",
					i, row, got, w+2)
			}
		}
	}
	// Under Plain the padding is directly visible: the wide line fills
	// its cells exactly, the two-cell lines take two pad spaces each.
	box := Plain().Overlay(lines)
	for _, want := range []string{"│世界│", "│éx  │", "│👨‍👩‍👧  │", "│ab  │"} {
		if !strings.Contains(box, want) {
			t.Fatalf("overlay did not pad by measured cells: %q lacks %q", box, want)
		}
	}
}
