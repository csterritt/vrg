package viewport_test

import (
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/viewport"
)

// makeRevealLine creates a filebuffer.Line with the given display text
// and grapheme clusters from the shared policy (Issue #19).
func makeRevealLine(num int, display string) filebuffer.Line {
	return filebuffer.Line{
		Number:   num,
		Display:  display,
		Clusters: safepresentation.GraphemeClusters(display),
	}
}

// makeWideClusterLine creates a line with ASCII clusters followed by a
// single grapheme cluster of the given width at the given cell offset,
// followed by more ASCII clusters. The display text is a placeholder;
// the cluster width is what matters for clipping and reveal arithmetic
// (Issue #19).
func makeWideClusterLine(num int, preASCII, clusterWidth, postASCII int) filebuffer.Line {
	clusters := make([]filebuffer.Cluster, 0, preASCII+1+postASCII)
	display := strings.Builder{}
	// Pre ASCII clusters (1 cell each).
	for i := 0; i < preASCII; i++ {
		clusters = append(clusters, filebuffer.Cluster{
			StartByte: i,
			EndByte:   i + 1,
			Width:     1,
		})
		display.WriteByte('a')
	}
	// The wide cluster at cell preASCII.
	wideStart := display.Len()
	clusters = append(clusters, filebuffer.Cluster{
		StartByte: wideStart,
		EndByte:   wideStart + 3,
		Width:     clusterWidth,
	})
	display.WriteString("XXX")
	// Post ASCII clusters (1 cell each).
	for i := 0; i < postASCII; i++ {
		clusters = append(clusters, filebuffer.Cluster{
			StartByte: wideStart + 3 + i,
			EndByte:   wideStart + 4 + i,
			Width:     1,
		})
		display.WriteByte('b')
	}
	return filebuffer.Line{
		Number:   num,
		Display:  display.String(),
		Clusters: clusters,
	}
}

// --- Right-side reveal (Issue #19) ---

// TestRevealHorizontalRightSideSingleCell verifies that a single-cell
// target right of view moves the offset so the target cluster is
// painted at the right edge: offset = target - (textWidth - 1).
func TestRevealHorizontalRightSideSingleCell(t *testing.T) {
	line := makeRevealLine(1, makeLongString(300))
	v := viewport.New(fakeRows([]filebuffer.Line{line}), 10)
	v.SetLayout(10, viewport.WrapOff)
	// Window [0, 10). Target at cell 50 is right of view.
	v.RevealHorizontal(line, 50)
	want := 50 - (10 - 1) // 41
	if v.HOffset() != want {
		t.Fatalf("HOffset = %d, want %d (right-side single-cell reveal)", v.HOffset(), want)
	}
}

// TestRevealHorizontalLeftSide verifies that a target left of view
// moves the offset so the target start is at the left edge:
// offset = target.
func TestRevealHorizontalLeftSide(t *testing.T) {
	line := makeRevealLine(1, makeLongString(300))
	v := viewport.New(fakeRows([]filebuffer.Line{line}), 10)
	v.SetLayout(10, viewport.WrapOff)
	v.SetHOffset(50) // window [50, 60)
	// Target at cell 5 is left of view.
	v.RevealHorizontal(line, 5)
	if v.HOffset() != 5 {
		t.Fatalf("HOffset = %d, want 5 (left-side reveal)", v.HOffset())
	}
}

// --- Cluster-width arithmetic (Issue #19) ---

// TestRevealHorizontalTwoCellClusterRight verifies that a two-cell
// cluster right of view moves the offset so the entire cluster fits at
// the right edge: offset = target + clusterWidth - textWidth.
func TestRevealHorizontalTwoCellClusterRight(t *testing.T) {
	// 50 ASCII chars + CJK char (2 cells) at cell 50 + more ASCII.
	display := makeLongString(50) + "中" + makeLongString(50)
	line := makeRevealLine(1, display)
	v := viewport.New(fakeRows([]filebuffer.Line{line}), 10)
	v.SetLayout(10, viewport.WrapOff)
	// Window [0, 10). Target at cell 50 (the CJK char), clusterWidth 2.
	v.RevealHorizontal(line, 50)
	want := 50 + 2 - 10 // 42
	if v.HOffset() != want {
		t.Fatalf("HOffset = %d, want %d (two-cell cluster right-side reveal)", v.HOffset(), want)
	}
}

// --- Painted-cell visibility (Issue #19) ---

// TestRevealHorizontalPaintedCellSplitRightEdge verifies that a target
// geometrically inside the window but whose cluster is split by the
// right edge (rendering as a blank) is hidden and must be revealed.
// The reveal moves the offset so the cluster fits fully.
func TestRevealHorizontalPaintedCellSplitRightEdge(t *testing.T) {
	// 9 ASCII chars + CJK char (2 cells) at cell 9 + more ASCII.
	// Window [0, 10): the CJK cluster at [9, 11) is split by the right
	// edge (cell 10 is outside). The split cluster renders as a blank,
	// so the target is not painted.
	display := makeLongString(9) + "中" + makeLongString(50)
	line := makeRevealLine(1, display)
	v := viewport.New(fakeRows([]filebuffer.Line{line}), 10)
	v.SetLayout(10, viewport.WrapOff)
	// Target at cell 9, clusterWidth 2. Split by right edge → not painted.
	v.RevealHorizontal(line, 9)
	want := 9 + 2 - 10 // 1
	if v.HOffset() != want {
		t.Fatalf("HOffset = %d, want %d (split-right-edge cluster reveal)", v.HOffset(), want)
	}
}

// TestRevealHorizontalPaintedCellSplitLeftEdge verifies that a target
// geometrically inside the window but whose cluster is split by the
// left edge (rendering as a blank) is hidden and must be revealed.
// The reveal moves the offset so the cluster start is at the left edge.
func TestRevealHorizontalPaintedCellSplitLeftEdge(t *testing.T) {
	// CJK char (2 cells) at cell 0 + ASCII.
	// Window [1, 11): the CJK cluster at [0, 2) is split by the left
	// edge (cell 0 is outside). The split cluster renders as a blank,
	// so the target is not painted.
	display := "中" + makeLongString(50)
	line := makeRevealLine(1, display)
	v := viewport.New(fakeRows([]filebuffer.Line{line}), 10)
	v.SetLayout(10, viewport.WrapOff)
	v.SetHOffset(1) // window [1, 11)
	// Target at cell 0, clusterWidth 2. Split by left edge → not painted.
	v.RevealHorizontal(line, 0)
	if v.HOffset() != 0 {
		t.Fatalf("HOffset = %d, want 0 (split-left-edge cluster reveal)", v.HOffset())
	}
}

// TestRevealHorizontalAlreadyPaintedNoMove verifies that an already-
// painted target (fully within the window, not split) does not move
// the offset.
func TestRevealHorizontalAlreadyPaintedNoMove(t *testing.T) {
	line := makeRevealLine(1, makeLongString(300))
	v := viewport.New(fakeRows([]filebuffer.Line{line}), 10)
	v.SetLayout(10, viewport.WrapOff)
	v.SetHOffset(5) // window [5, 15)
	// Target at cell 7, clusterWidth 1. Fully within [5, 15) → painted.
	v.RevealHorizontal(line, 7)
	if v.HOffset() != 5 {
		t.Fatalf("HOffset = %d, want 5 (already painted no-move)", v.HOffset())
	}
}

// TestRevealHorizontalTwoCellAlreadyPaintedNoMove verifies that an
// already-painted two-cell cluster does not move the offset.
func TestRevealHorizontalTwoCellAlreadyPaintedNoMove(t *testing.T) {
	// CJK char at cell 50 + ASCII.
	display := makeLongString(50) + "中" + makeLongString(50)
	line := makeRevealLine(1, display)
	v := viewport.New(fakeRows([]filebuffer.Line{line}), 10)
	v.SetLayout(10, viewport.WrapOff)
	v.SetHOffset(45) // window [45, 55)
	// Target at cell 50, clusterWidth 2. Fully within [45, 55) → painted.
	v.RevealHorizontal(line, 50)
	if v.HOffset() != 45 {
		t.Fatalf("HOffset = %d, want 45 (two-cell already painted no-move)", v.HOffset())
	}
}

// --- Oversized spans (Issue #19) ---

// TestRevealHorizontalOversizedMatchStartCell verifies that a match
// wider than the text area is revealed by its start cell alone. The
// reveal uses the start cell's cluster width (1), not the match width,
// so the start cell is painted at the right edge.
func TestRevealHorizontalOversizedMatchStartCell(t *testing.T) {
	line := makeRevealLine(1, makeLongString(300))
	v := viewport.New(fakeRows([]filebuffer.Line{line}), 10)
	v.SetLayout(10, viewport.WrapOff)
	// Target at cell 50 (start of a 100-cell match). clusterWidth 1.
	// The match is wider than textWidth, but only the start cell needs
	// to be visible. Right-edge arithmetic: offset = 50 + 1 - 10 = 41.
	v.RevealHorizontal(line, 50)
	want := 50 + 1 - 10 // 41
	if v.HOffset() != want {
		t.Fatalf("HOffset = %d, want %d (oversized match start-cell reveal)", v.HOffset(), want)
	}
}

// --- Unpaintable cluster fallback (Issue #19) ---

// TestRevealHorizontalUnpaintableCluster verifies that a grapheme
// cluster wider than the entire text area sets the offset to its start
// column (geometric fallback). The in-window portion renders as
// clipping blanks.
func TestRevealHorizontalUnpaintableCluster(t *testing.T) {
	// 3 ASCII clusters, then a cluster of width 10 at cell 3, then
	// more ASCII. textWidth 5 < clusterWidth 10 → unpaintable.
	line := makeWideClusterLine(1, 3, 10, 20)
	v := viewport.New(fakeRows([]filebuffer.Line{line}), 10)
	v.SetLayout(5, viewport.WrapOff)
	// Target at cell 3 (the wide cluster start), clusterWidth 10.
	v.RevealHorizontal(line, 3)
	if v.HOffset() != 3 {
		t.Fatalf("HOffset = %d, want 3 (unpaintable cluster geometric fallback)", v.HOffset())
	}
}

// TestRevealHorizontalUnpaintableClusterNoLoop verifies that repeated
// navigation to an unpaintable cluster does not enter a panning loop.
// After the first reveal sets the offset to the cluster's start
// column, a second reveal is a no-op (geometrically revealed).
func TestRevealHorizontalUnpaintableClusterNoLoop(t *testing.T) {
	line := makeWideClusterLine(1, 3, 10, 20)
	v := viewport.New(fakeRows([]filebuffer.Line{line}), 10)
	v.SetLayout(5, viewport.WrapOff)
	// First reveal: offset = 3.
	v.RevealHorizontal(line, 3)
	if v.HOffset() != 3 {
		t.Fatalf("first reveal HOffset = %d, want 3", v.HOffset())
	}
	// Second reveal: no-op (geometrically revealed, no panning loop).
	v.RevealHorizontal(line, 3)
	if v.HOffset() != 3 {
		t.Fatalf("second reveal HOffset = %d, want 3 (no panning loop)", v.HOffset())
	}
}

// --- Wrap-mode no-op (Issue #19) ---

// TestRevealHorizontalNoOpInWrapMode verifies that horizontal reveal is
// a no-op in wrap mode.
func TestRevealHorizontalNoOpInWrapMode(t *testing.T) {
	line := makeRevealLine(1, makeLongString(300))
	v := viewport.New(fakeRows([]filebuffer.Line{line}), 10)
	v.SetLayout(10, viewport.WrapOn)
	v.RevealHorizontal(line, 50)
	if v.HOffset() != 0 {
		t.Fatalf("HOffset = %d, want 0 (no-op in wrap mode)", v.HOffset())
	}
}

// --- ClusterWidthAtCell helper (Issue #19) ---

// TestClusterWidthAtCell verifies that ClusterWidthAtCell returns the
// correct width for the grapheme cluster at a given display cell.
func TestClusterWidthAtCell(t *testing.T) {
	// ASCII: each cluster is 1 cell.
	ascii := safepresentation.GraphemeClusters("hello")
	if w := viewport.ClusterWidthAtCell(ascii, 0); w != 1 {
		t.Fatalf("ASCII cell 0 width = %d, want 1", w)
	}
	// CJK: the CJK cluster is 2 cells.
	cjk := safepresentation.GraphemeClusters("a中b")
	if w := viewport.ClusterWidthAtCell(cjk, 0); w != 1 {
		t.Fatalf("a中b cell 0 width = %d, want 1", w)
	}
	if w := viewport.ClusterWidthAtCell(cjk, 1); w != 2 {
		t.Fatalf("a中b cell 1 width = %d, want 2", w)
	}
	if w := viewport.ClusterWidthAtCell(cjk, 2); w != 2 {
		t.Fatalf("a中b cell 2 width = %d, want 2 (inside CJK cluster)", w)
	}
	if w := viewport.ClusterWidthAtCell(cjk, 3); w != 1 {
		t.Fatalf("a中b cell 3 width = %d, want 1", w)
	}
	// Out of range: fallback to 1.
	if w := viewport.ClusterWidthAtCell(cjk, 100); w != 1 {
		t.Fatalf("out-of-range cell width = %d, want 1 (fallback)", w)
	}
}
