package viewport_test

import (
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/viewport"
)

// --- Grapheme cluster highlight expansion through wrap (Issue #21) ---

// makeExpandedLine creates a filebuffer.Line whose Highlights are already
// expanded to grapheme-cluster boundaries (the post-Issue-#21 form
// FileBuffer produces). Clusters come from the shared policy.
func makeExpandedLine(num int, display string, highlights ...[2]int) filebuffer.Line {
	return filebuffer.Line{
		Number:     num,
		Display:    display,
		Highlights: highlights,
		Clusters:   safepresentation.GraphemeClusters(display),
	}
}

// TestWrapBoundaryBlankNotHighlighted verifies that a blank filler cell
// introduced at a wrap boundary (a wide cluster that does not fit in the
// remaining row cells) is never painted as a match cell. The highlight
// expands to the wide cluster's full cell range, but the wrap blank
// before the cluster on the first row must not be highlighted.
func TestWrapBoundaryBlankNotHighlighted(t *testing.T) {
	// 9 ASCII chars + 1 wide char (2 cells) = 11 cells at width 10.
	// Row 0: 9 ASCII chars + 1 blank (wide can't fit in 1 cell).
	// Row 1: wide char (2 cells).
	// The highlight covers the wide cluster: cells [9, 11).
	// On row 0, the wrap blank is at local cell 9 (the filler). The
	// highlight [9, 11) shifted to row 0's local coords would be [9, 10)
	// but the blank filler cell at local 9 must NOT be highlighted.
	display := "012345678" + "中" // 9 ASCII + 1 CJK (2 cells)
	line := makeExpandedLine(1, display, [2]int{9, 11})
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 2 {
		t.Fatalf("RowCount = %d, want 2 (9 ASCII + wide at width 10)", model.RowCount())
	}
	rows := model.Rows(0, 2)
	// Row 0: 9 ASCII chars + 1 blank filler. The highlight [9, 11)
	// shifted by rowStartCell=0 → [9, 11), clamped to row width 10 →
	// [9, 10). But local cell 9 is the blank filler, not the wide
	// cluster. The blank filler must not be highlighted.
	row0 := rows[0]
	for _, hl := range row0.Highlights {
		if hl[0] == 9 || hl[1] == 10 {
			t.Fatalf("row 0 highlight %v covers the wrap-blank filler cell at local 9 (must not be highlighted)", hl)
		}
	}
	// Row 1: the wide cluster at local cells [0, 2). The highlight must
	// be [0, 2) here.
	row1 := rows[1]
	if len(row1.Highlights) != 1 {
		t.Fatalf("row 1 Highlights len = %d, want 1 (wide cluster fully on row 1)", len(row1.Highlights))
	}
	if row1.Highlights[0] != [2]int{0, 2} {
		t.Fatalf("row 1 highlight = %v, want [0, 2) (wide cluster on row 1)", row1.Highlights[0])
	}
}

// TestWrapBoundaryCombiningBlankNotHighlighted verifies that a combining
// mark's cluster, when it wraps to a new row, does not paint the
// preceding row's blank filler as a match cell. The highlight expands to
// the base+combining cluster; the wrap blank is not part of the cluster.
func TestWrapBoundaryCombiningBlankNotHighlighted(t *testing.T) {
	// 9 ASCII chars + "e\u0301" (1 cell). Total 10 cells at width 10.
	// The é cluster fits exactly at the end of row 0 (cells [9, 10)).
	// No wrap blank. To force a wrap blank, use width 9: 9 ASCII chars
	// fill row 0, the é cluster moves to row 1, leaving no blank (it
	// fits exactly). Use width 10 with 10 ASCII + é: row 0 = 10 ASCII,
	// row 1 = é (1 cell). The highlight [10, 11) is fully on row 1.
	display := "0123456789" + "e\u0301" // 10 ASCII + é (1 cell) = 11 cells
	line := makeExpandedLine(1, display, [2]int{10, 11})
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 2 {
		t.Fatalf("RowCount = %d, want 2", model.RowCount())
	}
	rows := model.Rows(0, 2)
	// Row 0: 10 ASCII chars, no highlight (the é cluster is on row 1).
	row0 := rows[0]
	if len(row0.Highlights) != 0 {
		t.Fatalf("row 0 Highlights len = %d, want 0 (é cluster on row 1, no wrap-blank highlight)", len(row0.Highlights))
	}
	// Row 1: é cluster at local cell [0, 1). Highlight [0, 1).
	row1 := rows[1]
	if len(row1.Highlights) != 1 {
		t.Fatalf("row 1 Highlights len = %d, want 1", len(row1.Highlights))
	}
	if row1.Highlights[0] != [2]int{0, 1} {
		t.Fatalf("row 1 highlight = %v, want [0, 1) (é cluster on row 1)", row1.Highlights[0])
	}
}

// TestClipSplitGlyphBlankNotHighlighted verifies that a wide glyph split
// by a clip edge renders as blank cells and those blank cells are not
// painted as match cells. The highlight covers the wide cluster, but
// the split-blank cells must not be highlighted.
func TestClipSplitGlyphBlankNotHighlighted(t *testing.T) {
	// "中" (2 cells) at the start, then ASCII. Clip window [1, 3):
	// the wide cluster [0, 2) is split by the left edge. Cell 1 is a
	// blank filler. The highlight [0, 2) shifted by -1 → [−1, 1),
	// clamped to [0, 1). The blank cell at local 0 must not be
	// highlighted.
	display := "中" + strings.Repeat("a", 20)
	line := makeExpandedLine(1, display, [2]int{0, 2})
	// Build a viewport in run-off-edge mode and clip.
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 3, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 3, viewport.WrapOff, key)
	v := viewport.New(model, 24)
	v.SetLayout(3, viewport.WrapOff)
	v.SetHOffset(1) // clip window [1, 4): splits the wide cluster [0, 2)
	clipped := v.ClipLine(model.Rows(0, 1)[0])
	// The clipped line's display starts with a blank (split glyph).
	if clipped.Display == "" || clipped.Display[0] != ' ' {
		t.Fatalf("clipped display = %q, want leading blank (split glyph)", clipped.Display)
	}
	// The highlight must not cover the blank cell at local 0.
	for _, hl := range clipped.Highlights {
		if hl[0] == 0 {
			t.Fatalf("clipped highlight %v covers split-blank cell at local 0 (must not be highlighted)", hl)
		}
	}
}
