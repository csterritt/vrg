package viewport_test

import (
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/viewport"
)

// --- Helpers for Issue #18 pan/extent tests ---

// makeNArrowLine creates a line with n single-cell ASCII clusters.
// The display text is n bytes long, each cluster width 1.
func makeNArrowLine(num, n int) filebuffer.Line {
	display := makeLongString(n)
	return filebuffer.Line{
		Number:   num,
		Display:  display,
		Clusters: makeClusters(display),
	}
}

// makeWideEndLine creates a line with n single-cell clusters followed
// by one wide cluster of the given cell width. The wide cluster's
// display text is "W" (one byte) but its Width is set to wideWidth.
func makeWideEndLine(num, n, wideWidth int) filebuffer.Line {
	display := makeLongString(n) + "W"
	clusters := makeClusters(makeLongString(n))
	clusters = append(clusters, filebuffer.Cluster{
		StartByte: n,
		EndByte:   n + 1,
		Width:     wideWidth,
	})
	return filebuffer.Line{
		Number:   num,
		Display:  display,
		Clusters: clusters,
	}
}

// makeWideStartLine creates a line with one wide cluster of the given
// cell width followed by n single-cell clusters.
func makeWideStartLine(num, wideWidth, n int) filebuffer.Line {
	display := "W" + makeLongString(n)
	clusters := []filebuffer.Cluster{{
		StartByte: 0,
		EndByte:   1,
		Width:     wideWidth,
	}}
	for i := 0; i < n; i++ {
		clusters = append(clusters, filebuffer.Cluster{
			StartByte: 1 + i,
			EndByte:   2 + i,
			Width:     1,
		})
	}
	return filebuffer.Line{
		Number:   num,
		Display:  display,
		Clusters: clusters,
	}
}

// makeUnfittableWideLine creates a line with n single-cell clusters
// followed by a wide cluster whose width exceeds textWidth, so the
// final cluster does not fit the text area.
func makeUnfittableWideLine(num, n, wideWidth int) filebuffer.Line {
	return makeWideEndLine(num, n, wideWidth)
}

// runOffModel builds a run-off-edge RowModel from the given lines at
// the given text width and returns it.
func runOffModel(lines []filebuffer.Line, textWidth int) *viewport.RowModel {
	buf := makeBufFromLines(lines)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: textWidth, WrapMode: viewport.WrapOff}
	return viewport.BuildRowModel(buf, textWidth, viewport.WrapOff, key)
}

// wrapModel builds a wrap-mode RowModel from the given lines at the
// given text width and returns it.
func wrapModel(lines []filebuffer.Line, textWidth int) *viewport.RowModel {
	buf := makeBufFromLines(lines)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: textWidth, WrapMode: viewport.WrapOn}
	return viewport.BuildRowModel(buf, textWidth, viewport.WrapOn, key)
}

// newRunOffViewport creates a viewport in run-off-edge mode with the
// given lines, panel height, and text width. The viewport's layout is
// set to run-off-edge at the given text width.
func newRunOffViewport(lines []filebuffer.Line, panelHeight, textWidth int) *viewport.Viewport {
	model := runOffModel(lines, textWidth)
	v := viewport.New(model, panelHeight)
	v.SetLayout(textWidth, viewport.WrapOff)
	return v
}

// --- Pan unit tests (Issue #18 AC1) ---

// TestPanRightOneColumn verifies that Pan(1) shifts the offset right by
// exactly one column.
func TestPanRightOneColumn(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(1)
	if v.HOffset() != 1 {
		t.Fatalf("HOffset = %d, want 1 (Pan(1))", v.HOffset())
	}
}

// TestPanLeftOneColumn verifies that Pan(-1) shifts the offset left by
// exactly one column.
func TestPanLeftOneColumn(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(5)
	v.Pan(-1)
	if v.HOffset() != 4 {
		t.Fatalf("HOffset = %d, want 4 (Pan(-1) from 5)", v.HOffset())
	}
}

// TestPanRightTenColumns verifies that Pan(10) shifts the offset right
// by ten columns.
func TestPanRightTenColumns(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(10)
	if v.HOffset() != 10 {
		t.Fatalf("HOffset = %d, want 10 (Pan(10))", v.HOffset())
	}
}

// TestPanLeftTenColumns verifies that Pan(-10) shifts the offset left by
// ten columns.
func TestPanLeftTenColumns(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(20)
	v.Pan(-10)
	if v.HOffset() != 10 {
		t.Fatalf("HOffset = %d, want 10 (Pan(-10) from 20)", v.HOffset())
	}
}

// TestPanRightHalfWidth verifies that Pan(HalfPanWidth()) shifts right
// by max(1, floor(textWidth/2)).
func TestPanRightHalfWidth(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	v := newRunOffViewport(lines, 10, 10) // textWidth 10, half = 5
	half := v.HalfPanWidth()
	if half != 5 {
		t.Fatalf("HalfPanWidth = %d, want 5 (floor(10/2))", half)
	}
	v.Pan(half)
	if v.HOffset() != 5 {
		t.Fatalf("HOffset = %d, want 5 (Pan(half))", v.HOffset())
	}
}

// TestPanLeftHalfWidth verifies that Pan(-HalfPanWidth()) shifts left by
// max(1, floor(textWidth/2)).
func TestPanLeftHalfWidth(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(20)
	v.Pan(-v.HalfPanWidth())
	if v.HOffset() != 15 {
		t.Fatalf("HOffset = %d, want 15 (Pan(-half) from 20)", v.HOffset())
	}
}

// TestHalfPanWidthMinOne verifies that HalfPanWidth returns at least 1
// even for a text width of 1.
func TestHalfPanWidthMinOne(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	v := newRunOffViewport(lines, 10, 1)
	if v.HalfPanWidth() != 1 {
		t.Fatalf("HalfPanWidth = %d, want 1 (max(1, floor(1/2)))", v.HalfPanWidth())
	}
}

// TestHalfPanWidthOdd verifies HalfPanWidth for odd text widths.
func TestHalfPanWidthOdd(t *testing.T) {
	cases := []struct {
		textWidth int
		want      int
	}{
		{3, 1},  // floor(3/2) = 1
		{7, 3},  // floor(7/2) = 3
		{11, 5}, // floor(11/2) = 5
	}
	for _, tc := range cases {
		lines := []filebuffer.Line{makeNArrowLine(1, 300)}
		v := newRunOffViewport(lines, 10, tc.textWidth)
		if got := v.HalfPanWidth(); got != tc.want {
			t.Fatalf("HalfPanWidth(textWidth=%d) = %d, want %d", tc.textWidth, got, tc.want)
		}
	}
}

// --- Pan clamping tests (Issue #18 AC1) ---

// TestPanLeftAtZeroNoOp verifies that panning left at offset 0 is a
// no-op (clamped to 0).
func TestPanLeftAtZeroNoOp(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(-1)
	if v.HOffset() != 0 {
		t.Fatalf("HOffset = %d, want 0 (Pan(-1) at 0 is no-op)", v.HOffset())
	}
	v.Pan(-10)
	if v.HOffset() != 0 {
		t.Fatalf("HOffset = %d, want 0 (Pan(-10) at 0 is no-op)", v.HOffset())
	}
	v.Pan(-v.HalfPanWidth())
	if v.HOffset() != 0 {
		t.Fatalf("HOffset = %d, want 0 (Pan(-half) at 0 is no-op)", v.HOffset())
	}
}

// TestPanRightAtMaxNoOp verifies that panning right past the maximum is
// clamped to the maximum and further pans do nothing.
func TestPanRightAtMaxNoOp(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)} // 300 cells, max = 299
	v := newRunOffViewport(lines, 10, 10)
	// Pan to the maximum.
	v.Pan(299)
	if v.HOffset() != 299 {
		t.Fatalf("HOffset = %d, want 299 (max)", v.HOffset())
	}
	// Further pans do nothing.
	v.Pan(1)
	if v.HOffset() != 299 {
		t.Fatalf("HOffset = %d, want 299 (Pan(1) at max is no-op)", v.HOffset())
	}
	v.Pan(10)
	if v.HOffset() != 299 {
		t.Fatalf("HOffset = %d, want 299 (Pan(10) at max is no-op)", v.HOffset())
	}
	v.Pan(v.HalfPanWidth())
	if v.HOffset() != 299 {
		t.Fatalf("HOffset = %d, want 299 (Pan(half) at max is no-op)", v.HOffset())
	}
}

// --- Wrap-mode no-op (Issue #18 AC5) ---

// TestPanNoOpInWrapMode verifies that panning in wrap mode does not
// change the horizontal offset.
func TestPanNoOpInWrapMode(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	model := wrapModel(lines, 10)
	v := viewport.New(model, 10)
	v.SetLayout(10, viewport.WrapOn)
	v.Pan(1)
	if v.HOffset() != 0 {
		t.Fatalf("HOffset = %d, want 0 (Pan in wrap mode is no-op)", v.HOffset())
	}
	v.Pan(10)
	if v.HOffset() != 0 {
		t.Fatalf("HOffset = %d, want 0 (Pan(10) in wrap mode is no-op)", v.HOffset())
	}
	v.Pan(v.HalfPanWidth())
	if v.HOffset() != 0 {
		t.Fatalf("HOffset = %d, want 0 (Pan(half) in wrap mode is no-op)", v.HOffset())
	}
}

// --- Offset retention through wrap toggles (Issue #18 AC6) ---

// TestOffsetRetainedThroughWrapToggle verifies that w → w (wrap off →
// on → off) retains the offset when neither the width nor the visible
// set changed enough to require clamping.
func TestOffsetRetainedThroughWrapToggle(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(50)
	if v.HOffset() != 50 {
		t.Fatalf("HOffset = %d, want 50", v.HOffset())
	}
	// Toggle to wrap mode: offset retained but not used.
	wrapM := wrapModel(lines, 10)
	v.SetLayout(10, viewport.WrapOn)
	v.SetRows(wrapM)
	if v.HOffset() != 50 {
		t.Fatalf("HOffset = %d after wrap on, want 50 (retained)", v.HOffset())
	}
	// Toggle back to run-off-edge: offset re-clamped to the current
	// max. The same 300-cell line is visible, so max is still 299.
	offM := runOffModel(lines, 10)
	v.SetLayout(10, viewport.WrapOff)
	v.SetRows(offM)
	if v.HOffset() != 50 {
		t.Fatalf("HOffset = %d after wrap off, want 50 (retained, max 299)", v.HOffset())
	}
}

// TestReentryClampOnEveryReturnToRunOffEdge verifies that every return
// to run-off-edge mode recomputes the maximum and clamps the offset,
// not only after a width change.
func TestReentryClampOnEveryReturnToRunOffEdge(t *testing.T) {
	// A file with one 300-cell line and several 10-cell lines.
	lines := make([]filebuffer.Line, 21)
	lines[0] = makeNArrowLine(1, 300)
	for i := 1; i < 21; i++ {
		lines[i] = makeNArrowLine(i+1, 10)
	}
	v := newRunOffViewport(lines, 10, 10)
	// Line 0 (300 cells) is visible. Pan to 299.
	v.Pan(299)
	if v.HOffset() != 299 {
		t.Fatalf("HOffset = %d, want 299", v.HOffset())
	}
	// Toggle to wrap mode and scroll down so the visible set changes
	// to short lines.
	wrapM := wrapModel(lines, 10)
	v.SetLayout(10, viewport.WrapOn)
	v.SetRows(wrapM)
	// Scroll down past the long line's wrapped rows. The 300-cell line
	// at width 10 wraps to 30 rows. Content height is 9. Scroll down
	// 35 rows to reach the short lines.
	for i := 0; i < 35; i++ {
		v.ScrollDown()
	}
	// Toggle back to run-off-edge. The anchor now points to a short
	// line, so the visible rows are short lines with max 9.
	offM := runOffModel(lines, 10)
	v.SetLayout(10, viewport.WrapOff)
	v.SetRows(offM)
	if v.HOffset() > 9 {
		t.Fatalf("HOffset = %d after re-entry, want <= 9 (clamped to short-line max)", v.HOffset())
	}
}

// --- File-change reset (Issue #18 AC7) ---

// TestResetHorizontalOnFileChange verifies that ResetHorizontal sets
// the offset to zero.
func TestResetHorizontalOnFileChange(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(50)
	if v.HOffset() != 50 {
		t.Fatalf("HOffset = %d, want 50", v.HOffset())
	}
	v.ResetHorizontal()
	if v.HOffset() != 0 {
		t.Fatalf("HOffset = %d after ResetHorizontal, want 0", v.HOffset())
	}
}

// --- Extent / max-offset tests (Issue #18 AC2, AC3, AC8) ---

// TestMaxOffsetShortLinesOnly verifies that a view of only short lines
// (all shorter than the text width) has max offset E-1 of the widest
// line, not 0. Single-cell content yields max = E-1.
func TestMaxOffsetShortLinesOnly(t *testing.T) {
	lines := []filebuffer.Line{
		makeNArrowLine(1, 5),
		makeNArrowLine(2, 3),
		makeNArrowLine(3, 7), // widest: E=7, max=6
	}
	v := newRunOffViewport(lines, 10, 10) // textWidth 10
	if max := v.MaxHOffset(); max != 6 {
		t.Fatalf("MaxHOffset = %d, want 6 (E-1 of widest 7-cell line)", max)
	}
}

// TestMaxOffsetMixedWidthWhileVisible verifies that a 300-cell line
// among 10-cell lines yields max 299 while the long line is visible.
func TestMaxOffsetMixedWidthWhileVisible(t *testing.T) {
	lines := make([]filebuffer.Line, 11)
	lines[0] = makeNArrowLine(1, 300)
	for i := 1; i < 11; i++ {
		lines[i] = makeNArrowLine(i+1, 10)
	}
	v := newRunOffViewport(lines, 10, 10) // line 0 visible, max = 299
	if max := v.MaxHOffset(); max != 299 {
		t.Fatalf("MaxHOffset = %d, want 299 (300-cell line visible)", max)
	}
}

// TestMaxOffsetMixedWidthAfterScrollOut verifies that once the 300-cell
// line scrolls out of view, the max drops to 9 (the 10-cell lines' max).
func TestMaxOffsetMixedWidthAfterScrollOut(t *testing.T) {
	lines := make([]filebuffer.Line, 20)
	lines[0] = makeNArrowLine(1, 300)
	for i := 1; i < 20; i++ {
		lines[i] = makeNArrowLine(i+1, 10)
	}
	v := newRunOffViewport(lines, 10, 10) // contentHeight 9
	// Scroll down so line 0 is no longer visible.
	for i := 0; i < 9; i++ {
		v.ScrollDown()
	}
	// Now lines 9-17 are visible, all 10-cell lines. Max = 9.
	if max := v.MaxHOffset(); max != 9 {
		t.Fatalf("MaxHOffset = %d, want 9 (long line scrolled out)", max)
	}
}

// TestMaxOffsetEmptyBuffer verifies that an empty buffer has max 0.
func TestMaxOffsetEmptyBuffer(t *testing.T) {
	v := newRunOffViewport(nil, 10, 10)
	if max := v.MaxHOffset(); max != 0 {
		t.Fatalf("MaxHOffset = %d, want 0 (empty buffer)", max)
	}
}

// TestMaxOffsetAllEmptyLines verifies that a view of only empty lines
// has max 0.
func TestMaxOffsetAllEmptyLines(t *testing.T) {
	lines := []filebuffer.Line{
		{Number: 1, Display: "", Clusters: nil},
		{Number: 2, Display: "", Clusters: nil},
	}
	v := newRunOffViewport(lines, 10, 10)
	if max := v.MaxHOffset(); max != 0 {
		t.Fatalf("MaxHOffset = %d, want 0 (all empty lines)", max)
	}
}

// TestMaxOffsetTwoCellClusterEnd verifies that a line ending in a
// two-cell cluster has its maximum at the cluster's start (E - 2),
// not at E - 1.
func TestMaxOffsetTwoCellClusterEnd(t *testing.T) {
	// 9 single-cell + 1 two-cell = 11 cells. E = 11.
	// Last cluster starts at cell 9, width 2. Max = 9 (E - 2).
	lines := []filebuffer.Line{makeWideEndLine(1, 9, 2)}
	v := newRunOffViewport(lines, 10, 10)
	if max := v.MaxHOffset(); max != 9 {
		t.Fatalf("MaxHOffset = %d, want 9 (two-cell cluster start, E-2)", max)
	}
}

// TestTwoCellClusterFullyPaintedAtMax verifies that at the maximum
// offset, the two-cell cluster is fully painted (no blanks, no partial
// glyph). This is the render-level assertion from the issue.
func TestTwoCellClusterFullyPaintedAtMax(t *testing.T) {
	// 9 single-cell + 1 two-cell = 11 cells. Max = 9.
	lines := []filebuffer.Line{makeWideEndLine(1, 9, 2)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(9)
	if v.HOffset() != 9 {
		t.Fatalf("HOffset = %d, want 9 (max)", v.HOffset())
	}
	visible := v.Visible()
	if len(visible) == 0 {
		t.Fatal("no visible rows")
	}
	clipped := v.ClipLine(visible[0])
	// At offset 9, textWidth 10: the two-cell cluster at cells [9, 11)
	// is fully visible. The clipped display should contain the wide
	// cluster's text "W", not a blank.
	if !strings.Contains(clipped.Display, "W") {
		t.Fatalf("clipped Display = %q, want it to contain W (two-cell cluster fully painted at max)", clipped.Display)
	}
	// Verify no blank-only text area: the clipped display should have
	// at least one non-space character.
	if strings.TrimSpace(clipped.Display) == "" {
		t.Fatalf("clipped Display = %q is all blanks (text area blank at max)", clipped.Display)
	}
}

// TestMaxOffsetUnfittableFinalCluster verifies that a line whose final
// cluster is wider than the text width falls back to the last fitting
// cluster's start.
func TestMaxOffsetUnfittableFinalCluster(t *testing.T) {
	// 3 single-cell + 1 six-cell cluster. textWidth = 5.
	// E = 9. Last cluster starts at cell 3, width 6 > 5, doesn't fit.
	// Last fitting cluster: cell 2, width 1. Max = 2.
	lines := []filebuffer.Line{makeUnfittableWideLine(1, 3, 6)}
	v := newRunOffViewport(lines, 10, 5)
	if max := v.MaxHOffset(); max != 2 {
		t.Fatalf("MaxHOffset = %d, want 2 (last fitting cluster start)", max)
	}
}

// TestMaxOffsetNoClusterFits verifies that when no cluster of the widest
// line fits the text width at all, the maximum is 0.
func TestMaxOffsetNoClusterFits(t *testing.T) {
	// One cluster of width 3, textWidth = 2. The cluster doesn't fit.
	lines := []filebuffer.Line{makeWideEndLine(1, 0, 3)}
	v := newRunOffViewport(lines, 10, 2)
	if max := v.MaxHOffset(); max != 0 {
		t.Fatalf("MaxHOffset = %d, want 0 (no cluster fits textWidth)", max)
	}
}

// --- Re-clamping on visible-set change (Issue #18 AC4) ---

// TestReclampOnVerticalScroll verifies that scrolling vertically from a
// long line into short lines re-clamps the stored offset leftwards.
func TestReclampOnVerticalScroll(t *testing.T) {
	lines := make([]filebuffer.Line, 20)
	lines[0] = makeNArrowLine(1, 300)
	for i := 1; i < 20; i++ {
		lines[i] = makeNArrowLine(i+1, 10)
	}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(299)
	if v.HOffset() != 299 {
		t.Fatalf("HOffset = %d, want 299", v.HOffset())
	}
	// Scroll down past the long line into short lines.
	for i := 0; i < 9; i++ {
		v.ScrollDown()
	}
	if v.HOffset() > 9 {
		t.Fatalf("HOffset = %d after scroll, want <= 9 (re-clamped to short lines)", v.HOffset())
	}
}

// TestNoRestoreOnScrollBack verifies that scrolling back to the long
// line does not restore the old offset.
func TestNoRestoreOnScrollBack(t *testing.T) {
	lines := make([]filebuffer.Line, 20)
	lines[0] = makeNArrowLine(1, 300)
	for i := 1; i < 20; i++ {
		lines[i] = makeNArrowLine(i+1, 10)
	}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(299)
	// Scroll down into short lines (re-clamp to 9).
	for i := 0; i < 9; i++ {
		v.ScrollDown()
	}
	clamped := v.HOffset()
	// Scroll back up to the long line.
	for i := 0; i < 9; i++ {
		v.ScrollUp()
	}
	// The offset should be the clamped value (9), not restored to 299.
	if v.HOffset() == 299 {
		t.Fatalf("HOffset = 299, want not restored (should stay at clamped value)")
	}
	if v.HOffset() != clamped {
		t.Fatalf("HOffset = %d after scroll back, want %d (clamped value retained)", v.HOffset(), clamped)
	}
}

// TestReclampOnReveal verifies that a reveal that changes the visible
// set re-clamps the horizontal offset.
func TestReclampOnReveal(t *testing.T) {
	lines := make([]filebuffer.Line, 20)
	lines[0] = makeNArrowLine(1, 300)
	for i := 1; i < 20; i++ {
		lines[i] = makeNArrowLine(i+1, 10)
	}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(299)
	// Reveal a row in the short-lines region (row 15).
	v.Reveal(15)
	if v.HOffset() > 9 {
		t.Fatalf("HOffset = %d after reveal, want <= 9 (re-clamped)", v.HOffset())
	}
}

// TestReclampOnResize verifies that a resize that changes the visible
// set re-clamps the horizontal offset.
func TestReclampOnResize(t *testing.T) {
	lines := make([]filebuffer.Line, 20)
	lines[0] = makeNArrowLine(1, 300)
	for i := 1; i < 20; i++ {
		lines[i] = makeNArrowLine(i+1, 10)
	}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(299)
	// Shrink the panel so the long line scrolls out of view.
	v.SetPanelHeight(5) // contentHeight 4
	// After shrinking, the visible rows may change. The offset should
	// be re-clamped to the new max.
	max := v.MaxHOffset()
	if v.HOffset() > max {
		t.Fatalf("HOffset = %d after resize, want <= %d (re-clamped)", v.HOffset(), max)
	}
}

// TestReclampOnTextWidthChange verifies that a text-width change (from
// list hide/show or gutter growth) re-clamps the offset.
func TestReclampOnTextWidthChange(t *testing.T) {
	lines := []filebuffer.Line{makeNArrowLine(1, 300)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(299)
	// Widen the text area: max stays 299, offset still valid.
	v.SetLayout(20, viewport.WrapOff)
	if v.HOffset() != 299 {
		t.Fatalf("HOffset = %d after widen, want 299 (still valid)", v.HOffset())
	}
	// Narrow the text area to 5: max becomes 299 (still 300-cell line),
	// offset 299 still valid.
	v.SetLayout(5, viewport.WrapOff)
	if v.HOffset() != 299 {
		t.Fatalf("HOffset = %d after narrow, want 299 (still valid for 300-cell line)", v.HOffset())
	}
}

// TestReclampOnTextWidthChangeShortLines verifies that narrowing the
// text width re-clamps the offset when the max decreases. A line with
// a 6-cell final cluster: at textWidth 10 the cluster fits (max 3), at
// textWidth 5 it doesn't fit (max 2).
func TestReclampOnTextWidthChangeShortLines(t *testing.T) {
	// 3 single-cell + 1 six-cell cluster = 9 cells.
	lines := []filebuffer.Line{makeUnfittableWideLine(1, 3, 6)}
	v := newRunOffViewport(lines, 10, 10) // textWidth 10, max = 3
	v.Pan(3)
	if v.HOffset() != 3 {
		t.Fatalf("HOffset = %d, want 3", v.HOffset())
	}
	// Narrow to textWidth 5: the 6-cell cluster doesn't fit (6 > 5),
	// so max falls back to cell 2 (last fitting single-cell cluster).
	v.SetLayout(5, viewport.WrapOff)
	if v.HOffset() != 2 {
		t.Fatalf("HOffset = %d after narrow to 5, want 2 (re-clamped to new max)", v.HOffset())
	}
}

// TestReclampOnEveryPan verifies that every pan re-evaluates the
// maximum from the current visible rows, not a cached value.
func TestReclampOnEveryPan(t *testing.T) {
	lines := make([]filebuffer.Line, 20)
	lines[0] = makeNArrowLine(1, 300)
	for i := 1; i < 20; i++ {
		lines[i] = makeNArrowLine(i+1, 10)
	}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(299)
	// Scroll down into short lines.
	for i := 0; i < 9; i++ {
		v.ScrollDown()
	}
	// Now pan right: the max should be 9, so panning right clamps to 9.
	v.Pan(100)
	if v.HOffset() != 9 {
		t.Fatalf("HOffset = %d after pan in short-lines region, want 9 (re-evaluated max)", v.HOffset())
	}
}

// TestPanAfterExtentsChanged verifies that a pan issued after the
// visible rows' extents changed clamps against the newly computed max.
func TestPanAfterExtentsChanged(t *testing.T) {
	lines := make([]filebuffer.Line, 20)
	lines[0] = makeNArrowLine(1, 300)
	for i := 1; i < 20; i++ {
		lines[i] = makeNArrowLine(i+1, 10)
	}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(50)
	// Scroll down into short lines (max drops to 9).
	for i := 0; i < 9; i++ {
		v.ScrollDown()
	}
	// Pan right by 1: should clamp to 9, not go to 51.
	v.Pan(1)
	if v.HOffset() != 9 {
		t.Fatalf("HOffset = %d after pan after extent change, want 9", v.HOffset())
	}
}

// --- Uniform-lines fixture (Issue #18 AC4, Issue #20 geometry) ---

// TestUniformLinesAllHiddenLeft verifies that a uniform-lines fixture
// where every visible line has hidden-left text at a nonzero offset
// while the widest line still paints a fitting cluster is legal. No
// extent or clamp assertion may forbid it.
func TestUniformLinesAllHiddenLeft(t *testing.T) {
	// All lines are 50 cells, textWidth 10. Max = 49.
	lines := make([]filebuffer.Line, 10)
	for i := range lines {
		lines[i] = makeNArrowLine(i+1, 50)
	}
	v := newRunOffViewport(lines, 10, 10)
	// Pan to 5: every visible line has hidden-left text (cells 0-4
	// hidden), and the widest line still paints fitting clusters
	// (cells 5-14 visible, all single-cell clusters).
	v.Pan(5)
	if v.HOffset() != 5 {
		t.Fatalf("HOffset = %d, want 5", v.HOffset())
	}
	// Verify the max is 49 (every line is 50 cells).
	if max := v.MaxHOffset(); max != 49 {
		t.Fatalf("MaxHOffset = %d, want 49 (uniform 50-cell lines)", max)
	}
	// Verify every visible line has fully painted clusters at offset 5.
	for _, line := range v.Visible() {
		clipped := v.ClipLine(line)
		if strings.TrimSpace(clipped.Display) == "" {
			t.Fatalf("clipped Display = %q is all blanks (line has no painted cluster at offset 5)", clipped.Display)
		}
	}
}

// --- Split-cluster blank rendering (Issue #18 AC8) ---

// TestSplitClusterLeftClipBlank verifies that a wide cluster split by
// the left clip edge renders blank cells for the clipped portion
// rather than a broken glyph.
func TestSplitClusterLeftClipBlank(t *testing.T) {
	// Line: wide cluster (2 cells) at cell 0, then 9 single-cell.
	// At hOffset 1, textWidth 10: the wide cluster [0, 2) is split.
	// The visible portion (cell 1) should be a blank, not half a glyph.
	lines := []filebuffer.Line{makeWideStartLine(1, 2, 9)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(1)
	if v.HOffset() != 1 {
		t.Fatalf("HOffset = %d, want 1", v.HOffset())
	}
	visible := v.Visible()
	clipped := v.ClipLine(visible[0])
	// The wide cluster's display text "W" must NOT appear in the
	// clipped output (it was split and replaced with blanks).
	if strings.Contains(clipped.Display, "W") {
		t.Fatalf("clipped Display = %q contains W (split cluster should be blank, not broken glyph)", clipped.Display)
	}
	// The first character should be a space (the blank for the split
	// cluster's visible portion).
	if clipped.Display[0] != ' ' {
		t.Fatalf("clipped Display[0] = %q, want ' ' (blank for split cluster)", clipped.Display[0])
	}
}

// TestSplitClusterRightClipBlank verifies that a wide cluster split by
// the right clip edge renders blank cells for the clipped portion.
func TestSplitClusterRightClipBlank(t *testing.T) {
	// Line: 9 single-cell + wide cluster (2 cells) at cell 9.
	// E = 11. At hOffset 0, textWidth 10: visible [0, 10).
	// The wide cluster [9, 11) is split at the right edge (cell 10
	// clipped). The visible portion (cell 9) should be a blank.
	lines := []filebuffer.Line{makeWideEndLine(1, 9, 2)}
	v := newRunOffViewport(lines, 10, 10)
	// hOffset is 0, textWidth 10. The wide cluster at [9, 11) is split.
	visible := v.Visible()
	clipped := v.ClipLine(visible[0])
	// The wide cluster's display text "W" must NOT appear (split).
	if strings.Contains(clipped.Display, "W") {
		t.Fatalf("clipped Display = %q contains W (right-split cluster should be blank)", clipped.Display)
	}
	// The last character should be a space (blank for the split).
	if clipped.Display[len(clipped.Display)-1] != ' ' {
		t.Fatalf("clipped Display last = %q, want ' ' (blank for right-split cluster)", clipped.Display[len(clipped.Display)-1])
	}
}

// TestNoSplitWhenFullyVisible verifies that a wide cluster fully within
// the visible window is not blanked.
func TestNoSplitWhenFullyVisible(t *testing.T) {
	// Line: 9 single-cell + wide cluster (2 cells) at cell 9.
	// At hOffset 9, textWidth 10: visible [9, 19). The wide cluster
	// [9, 11) is fully visible.
	lines := []filebuffer.Line{makeWideEndLine(1, 9, 2)}
	v := newRunOffViewport(lines, 10, 10)
	v.Pan(9)
	visible := v.Visible()
	clipped := v.ClipLine(visible[0])
	if !strings.Contains(clipped.Display, "W") {
		t.Fatalf("clipped Display = %q, want it to contain W (fully visible cluster)", clipped.Display)
	}
}

// --- Render-cost guard for extent evaluation (Issue #18 AC9) ---

// TestExtentEvaluationTouchesOnlyVisibleRows verifies that computing
// the max horizontal offset queries the row provider only for the
// visible range, not the full buffer.
func TestExtentEvaluationTouchesOnlyVisibleRows(t *testing.T) {
	lines := make([]filebuffer.Line, 100)
	for i := range lines {
		lines[i] = makeNArrowLine(i+1, 50)
	}
	model := runOffModel(lines, 10)
	counter := &countingRowModel{model: model}
	v := viewport.New(counter, 10) // contentHeight 9
	v.SetLayout(10, viewport.WrapOff)
	v.SetOffset(50)
	counter.queries = nil
	// Computing MaxHOffset should query only the visible range.
	max := v.MaxHOffset()
	if len(counter.queries) != 1 {
		t.Fatalf("queries = %d, want 1 (extent evaluation)", len(counter.queries))
	}
	start, end := counter.queries[0][0], counter.queries[0][1]
	if start != 50 || end != 59 {
		t.Fatalf("query = [%d, %d), want [50, 59) (visible range only)", start, end)
	}
	// The max should be 49 (all lines are 50 cells).
	if max != 49 {
		t.Fatalf("MaxHOffset = %d, want 49", max)
	}
}

// TestClampOnPanTouchesOnlyVisibleRows verifies that a pan operation
// (which re-clamps) queries only the visible range.
func TestClampOnPanTouchesOnlyVisibleRows(t *testing.T) {
	lines := make([]filebuffer.Line, 100)
	for i := range lines {
		lines[i] = makeNArrowLine(i+1, 50)
	}
	model := runOffModel(lines, 10)
	counter := &countingRowModel{model: model}
	v := viewport.New(counter, 10)
	v.SetLayout(10, viewport.WrapOff)
	v.SetOffset(50)
	counter.queries = nil
	v.Pan(1)
	if len(counter.queries) != 1 {
		t.Fatalf("queries = %d, want 1 (pan re-clamp)", len(counter.queries))
	}
	start, end := counter.queries[0][0], counter.queries[0][1]
	if start != 50 || end != 59 {
		t.Fatalf("query = [%d, %d), want [50, 59) (visible range only)", start, end)
	}
}
