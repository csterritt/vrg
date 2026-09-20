package viewport

import (
	"strings"
	"testing"

	"vrg/internal/filebuffer"
)

// A highlight at a wrap boundary never paints the blank filler cell: a
// cluster that cannot fit the row's remaining cells moves to the next
// row whole, and its span — cluster-expanded at the source — clips out
// of the previous row entirely, leaving the filler unpainted.
func TestWrapBoundaryBlankIsNotAMatchCell(t *testing.T) {
	// Text width 2: "a" fills row 0's first cell and 世 (width 2)
	// cannot fit the remaining one, so row 0 ends in a blank filler
	// and 世 paints alone on row 1.
	src := sourceOf(t, "a世")
	src.spans[0] = []filebuffer.Span{{Start: 1, End: 2}} // 世's cell range
	m := NewRowModel(RowModelKey{TextWidth: 2, Wrap: true}, src)
	if m.LineCount() != 2 {
		t.Fatalf("LineCount = %d, want 2", m.LineCount())
	}
	if got := rowText(m, 0); got != "a" {
		t.Fatalf("row 0 = %q, want %q", got, "a")
	}
	if spans := m.Highlights(0); len(spans) != 0 {
		t.Fatalf("row 0 spans = %+v, want none — the blank filler is no match cell", spans)
	}
	if got := rowText(m, 1); got != "世" {
		t.Fatalf("row 1 = %q, want %q", got, "世")
	}
	spans := m.Highlights(1)
	if len(spans) != 1 || spans[0] != (filebuffer.Span{Start: 0, End: 1}) {
		t.Fatalf("row 1 spans = %+v, want [{0 1}] covering 世's whole cell", spans)
	}
}

// A cluster split by either clip edge renders blank cells for its
// in-window columns; those blanks are never painted as match cells even
// when the cluster's own span covers them.
func TestClipBlankCellsNeverCarryMatch(t *testing.T) {
	// 世 occupies columns 2-3; its span covers its whole cell range.
	src := sourceOf(t, "ab世cd")
	src.spans[0] = []filebuffer.Span{{Start: 2, End: 3}}
	m := NewRowModel(RowModelKey{TextWidth: 10}, src)

	// Offset 3 splits 世 at the left edge: one clipping blank then "cd".
	cells, spans, _ := m.Clip(0, 3, 10)
	if got := cellsText(cells); got != " cd" {
		t.Fatalf("clip at offset 3 = %q, want %q", got, " cd")
	}
	if len(spans) != 0 {
		t.Fatalf("clip at offset 3 spans = %+v, want none — the blank is no match cell", spans)
	}

	// Window [1,3) covers b plus 世's first column: a trailing blank.
	cells, spans, _ = m.Clip(0, 1, 2)
	if got := cellsText(cells); got != "b " {
		t.Fatalf("clip at offset 1 width 2 = %q, want %q", got, "b ")
	}
	if len(spans) != 0 {
		t.Fatalf("clip at offset 1 spans = %+v, want none — the blank is no match cell", spans)
	}
}

// A span recorded for a match that started mid-cluster covers the
// cluster's whole cell range, so when the cluster is split into blanks
// at the window's left edge the span has no painted cell and counts as
// hidden left — indicator visibility counts from the cluster start.
func TestHiddenCountsSpanFromClusterStart(t *testing.T) {
	// 世 occupies columns 0-1, then x's; the span is 世's whole cell.
	src := sourceOf(t, "世"+strings.Repeat("x", 20))
	src.spans[0] = []filebuffer.Span{{Start: 0, End: 1}}
	m := NewRowModel(RowModelKey{TextWidth: 10}, src)

	h := m.Hidden(0, 1, 10) // offset 1 splits 世 into a blank
	if !h.LeftMatch {
		t.Fatalf("Hidden(0, 1, 10) = %+v, want LeftMatch — the blanked cluster hides the match", h)
	}

	// Wholly painted at offset 0 the match is visible: no indicator.
	if h := m.Hidden(0, 0, 10); h.LeftMatch {
		t.Fatalf("Hidden(0, 0, 10) = %+v, want no LeftMatch — the cluster paints whole", h)
	}
}

// A stop whose recorded bytes start mid-cluster reveals from the
// cluster's start column: TargetCell resolves to the cluster's first
// cell, so the minimal horizontal reveal aims at the whole cluster.
func TestRevealFromMidClusterStartAimsAtClusterStart(t *testing.T) {
	// 世 occupies columns 4-5 (bytes [4,7)); the stop's coverage begins
	// at byte 5, inside the cluster.
	line := strings.Repeat("x", 4) + "世" + strings.Repeat("x", 45)
	m := layoutOf(t, 10, false, line)
	v := &Viewport{}
	v.SetLayout(m, 5)
	v.Pan(10)
	v.Reveal(stopAt(1, 5, 7))
	if got := v.Offset(); got != 4 {
		t.Fatalf("offset = %d, want 4 — the cluster's start column", got)
	}
}
