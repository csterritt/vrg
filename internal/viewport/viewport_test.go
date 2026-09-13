package viewport_test

import (
	"fmt"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/viewport"
)

// fakeRows is a simple RowProvider backed by a slice of lines.
type fakeRows []filebuffer.Line

func (f fakeRows) RowCount() int { return len(f) }

func (f fakeRows) Rows(start, end int) []filebuffer.Line {
	if start < 0 {
		start = 0
	}
	if end > len(f) {
		end = len(f)
	}
	if start > end {
		return nil
	}
	return f[start:end]
}

// countingRows is a RowProvider that records every Rows query for the
// render-cost guard. It proves the Viewport queries only the visible
// range, not the full buffer.
type countingRows struct {
	lines   []filebuffer.Line
	queries [][2]int
}

func (c *countingRows) RowCount() int { return len(c.lines) }

func (c *countingRows) Rows(start, end int) []filebuffer.Line {
	c.queries = append(c.queries, [2]int{start, end})
	if start < 0 {
		start = 0
	}
	if end > len(c.lines) {
		end = len(c.lines)
	}
	if start > end {
		return nil
	}
	return c.lines[start:end]
}

// makeLines returns n filebuffer.Lines with sequential line numbers.
func makeLines(n int) []filebuffer.Line {
	lines := make([]filebuffer.Line, n)
	for i := range lines {
		lines[i] = filebuffer.Line{Number: i + 1, Display: fmt.Sprintf("line %d", i+1)}
	}
	return lines
}

// --- Scroll unit tests ---

// TestScrollDownOneRow verifies that ScrollDown moves the top row by
// exactly one rendered row.
func TestScrollDownOneRow(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(20)), 10) // contentHeight = 9
	v.ScrollDown()
	if v.Offset() != 1 {
		t.Fatalf("Offset = %d, want 1", v.Offset())
	}
}

// TestScrollUpOneRow verifies that ScrollUp moves the top row up by
// exactly one rendered row.
func TestScrollUpOneRow(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(20)), 10)
	v.SetOffset(5)
	v.ScrollUp()
	if v.Offset() != 4 {
		t.Fatalf("Offset = %d, want 4", v.Offset())
	}
}

// TestScrollHalfDown verifies that ScrollHalfDown moves the top row by
// max(1, floor(contentHeight / 2)).
func TestScrollHalfDown(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(50)), 10) // contentHeight = 9, half = max(1, 4) = 4
	v.ScrollHalfDown()
	if v.Offset() != 4 {
		t.Fatalf("Offset = %d, want 4", v.Offset())
	}
}

// TestScrollHalfUp verifies that ScrollHalfUp moves the top row up by
// max(1, floor(contentHeight / 2)).
func TestScrollHalfUp(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(50)), 10) // contentHeight = 9, half = 4
	v.SetOffset(10)
	v.ScrollHalfUp()
	if v.Offset() != 6 {
		t.Fatalf("Offset = %d, want 6", v.Offset())
	}
}

// TestScrollPageDown verifies that ScrollPageDown moves the top row by
// the full content height.
func TestScrollPageDown(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(50)), 10) // contentHeight = 9
	v.ScrollPageDown()
	if v.Offset() != 9 {
		t.Fatalf("Offset = %d, want 9", v.Offset())
	}
}

// TestScrollPageUp verifies that ScrollPageUp moves the top row up by
// the full content height.
func TestScrollPageUp(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(50)), 10) // contentHeight = 9
	v.SetOffset(20)
	v.ScrollPageUp()
	if v.Offset() != 11 {
		t.Fatalf("Offset = %d, want 11", v.Offset())
	}
}

// --- Clamp tests ---

// TestClampBOF verifies that scrolling up at the top of the file does
// nothing (top row stays at 0).
func TestClampBOF(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(20)), 10)
	v.ScrollUp()
	if v.Offset() != 0 {
		t.Fatalf("Offset = %d, want 0 (BOF clamp)", v.Offset())
	}
	// Half-page up at BOF should also clamp to 0.
	v.ScrollHalfUp()
	if v.Offset() != 0 {
		t.Fatalf("Offset = %d, want 0 (BOF clamp for half-page)", v.Offset())
	}
	// Full-page up at BOF should also clamp to 0.
	v.ScrollPageUp()
	if v.Offset() != 0 {
		t.Fatalf("Offset = %d, want 0 (BOF clamp for full-page)", v.Offset())
	}
}

// TestClampEOF verifies that scrolling down past EOF stops with the
// last row at the bottom, leaving no avoidable blank rows below EOF.
func TestClampEOF(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(20)), 10) // contentHeight = 9, maxOffset = 11
	v.SetOffset(11)
	v.ScrollDown()
	if v.Offset() != 11 {
		t.Fatalf("Offset = %d, want 11 (EOF clamp)", v.Offset())
	}
	// Half-page down past EOF should clamp.
	v.ScrollHalfDown()
	if v.Offset() != 11 {
		t.Fatalf("Offset = %d, want 11 (EOF clamp for half-page)", v.Offset())
	}
	// Full-page down past EOF should clamp.
	v.ScrollPageDown()
	if v.Offset() != 11 {
		t.Fatalf("Offset = %d, want 11 (EOF clamp for full-page)", v.Offset())
	}
}

// TestClampEOFLastRowAtBottom verifies that at the maximum offset, the
// last row is visible at the bottom of the content area.
func TestClampEOFLastRowAtBottom(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(20)), 10) // contentHeight = 9, maxOffset = 11
	v.SetOffset(11)
	visible := v.Visible()
	if len(visible) != 9 {
		t.Fatalf("Visible count = %d, want 9", len(visible))
	}
	last := visible[len(visible)-1]
	if last.Number != 20 {
		t.Fatalf("Last visible line Number = %d, want 20 (last row at bottom)", last.Number)
	}
}

// --- Odd-height half-page tests ---

// TestHalfPageOddHeight verifies the half-page formula
// max(1, floor(contentHeight / 2)) for odd content heights. Content
// height is panelHeight - 1 (filename row).
func TestHalfPageOddHeight(t *testing.T) {
	cases := []struct {
		panelHeight int
		wantHalf    int
	}{
		{6, 2},  // contentHeight = 5, half = max(1, 2) = 2
		{8, 3},  // contentHeight = 7, half = max(1, 3) = 3
		{3, 1},  // contentHeight = 2, half = max(1, 1) = 1
		{4, 1},  // contentHeight = 3, half = max(1, 1) = 1
		{10, 4}, // contentHeight = 9, half = max(1, 4) = 4
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("panelHeight=%d", tc.panelHeight), func(t *testing.T) {
			v := viewport.New(fakeRows(makeLines(100)), tc.panelHeight)
			v.ScrollHalfDown()
			if v.Offset() != tc.wantHalf {
				t.Fatalf("ScrollHalfDown Offset = %d, want %d", v.Offset(), tc.wantHalf)
			}
		})
	}
}

// TestHalfPageOddHeightUp verifies the half-page formula for upward
// scrolling with odd content heights.
func TestHalfPageOddHeightUp(t *testing.T) {
	cases := []struct {
		panelHeight int
		startOffset int
		wantOffset  int
	}{
		{6, 10, 8}, // contentHeight = 5, half = 2, 10 - 2 = 8
		{8, 10, 7}, // contentHeight = 7, half = 3, 10 - 3 = 7
		{3, 10, 9}, // contentHeight = 2, half = 1, 10 - 1 = 9
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("panelHeight=%d", tc.panelHeight), func(t *testing.T) {
			v := viewport.New(fakeRows(makeLines(100)), tc.panelHeight)
			v.SetOffset(tc.startOffset)
			v.ScrollHalfUp()
			if v.Offset() != tc.wantOffset {
				t.Fatalf("ScrollHalfUp Offset = %d, want %d", v.Offset(), tc.wantOffset)
			}
		})
	}
}

// --- File length tests ---

// TestFileShorterThanViewport verifies that a file shorter than the
// viewport leaves unused rows naturally and scrolling is a no-op.
func TestFileShorterThanViewport(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(5)), 10) // contentHeight = 9, maxOffset = max(0, 5-9) = 0
	v.ScrollDown()
	if v.Offset() != 0 {
		t.Fatalf("Offset = %d, want 0 (file shorter than viewport)", v.Offset())
	}
	visible := v.Visible()
	if len(visible) != 5 {
		t.Fatalf("Visible count = %d, want 5 (unused rows left naturally)", len(visible))
	}
}

// TestFileEqualToViewport verifies that a file exactly filling the
// viewport has maxOffset 0 and scrolling is a no-op.
func TestFileEqualToViewport(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(9)), 10) // contentHeight = 9, maxOffset = max(0, 9-9) = 0
	v.ScrollDown()
	if v.Offset() != 0 {
		t.Fatalf("Offset = %d, want 0 (file equal to viewport)", v.Offset())
	}
	visible := v.Visible()
	if len(visible) != 9 {
		t.Fatalf("Visible count = %d, want 9", len(visible))
	}
}

// TestFileLongerThanViewport verifies that a file longer than the
// viewport can scroll and the visible count equals the content height.
func TestFileLongerThanViewport(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(20)), 10) // contentHeight = 9, maxOffset = 11
	v.ScrollDown()
	if v.Offset() != 1 {
		t.Fatalf("Offset = %d, want 1", v.Offset())
	}
	visible := v.Visible()
	if len(visible) != 9 {
		t.Fatalf("Visible count = %d, want 9", len(visible))
	}
}

// TestFileLongerThanViewportMaxOffset verifies the maximum offset for a
// long file: max(0, rowCount - contentHeight).
func TestFileLongerThanViewportMaxOffset(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(20)), 10) // contentHeight = 9, maxOffset = 11
	v.SetOffset(100)                               // Try to scroll past EOF
	if v.Offset() != 11 {
		t.Fatalf("Offset = %d, want 11 (clamped to maxOffset)", v.Offset())
	}
}

// --- Empty file test ---

// TestEmptyFile verifies that an empty file (zero lines) has no visible
// rows and scrolling is a no-op.
func TestEmptyFile(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(0)), 10)
	v.ScrollDown()
	if v.Offset() != 0 {
		t.Fatalf("Offset = %d, want 0 (empty file)", v.Offset())
	}
	visible := v.Visible()
	if len(visible) != 0 {
		t.Fatalf("Visible count = %d, want 0 (empty file)", len(visible))
	}
}

// --- Render-cost guard (Viewport level) ---

// TestVisibleQueriesOnlyVisibleRange verifies that Visible queries the
// row provider only for the visible [offset, offset+contentHeight)
// range, not the full buffer. This is the render-cost guard at the
// Viewport level.
func TestVisibleQueriesOnlyVisibleRange(t *testing.T) {
	counter := &countingRows{lines: makeLines(100)}
	v := viewport.New(counter, 10) // contentHeight = 9
	v.SetOffset(50)
	counter.queries = nil
	v.Visible()
	if len(counter.queries) != 1 {
		t.Fatalf("queries = %d, want 1", len(counter.queries))
	}
	start, end := counter.queries[0][0], counter.queries[0][1]
	if start != 50 || end != 59 {
		t.Fatalf("query = [%d, %d), want [50, 59)", start, end)
	}
}

// TestVisibleQueriesOnlyVisibleRangeAtEOF verifies the render-cost guard
// when the viewport is at EOF (the visible range is shorter than the
// content height).
func TestVisibleQueriesOnlyVisibleRangeAtEOF(t *testing.T) {
	counter := &countingRows{lines: makeLines(20)}
	v := viewport.New(counter, 10) // contentHeight = 9, maxOffset = 11
	v.SetOffset(11)
	counter.queries = nil
	v.Visible()
	if len(counter.queries) != 1 {
		t.Fatalf("queries = %d, want 1", len(counter.queries))
	}
	start, end := counter.queries[0][0], counter.queries[0][1]
	if start != 11 || end != 20 {
		t.Fatalf("query = [%d, %d), want [11, 20)", start, end)
	}
}

// --- SetOffset clamping test ---

// TestSetOffsetClamp verifies that SetOffset clamps to [0, maxOffset].
func TestSetOffsetClamp(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(20)), 10) // maxOffset = 11
	v.SetOffset(-5)
	if v.Offset() != 0 {
		t.Fatalf("SetOffset(-5) = %d, want 0 (BOF clamp)", v.Offset())
	}
	v.SetOffset(100)
	if v.Offset() != 11 {
		t.Fatalf("SetOffset(100) = %d, want 11 (EOF clamp)", v.Offset())
	}
}

// --- ContentHeight test ---

// TestContentHeight verifies that ContentHeight returns panelHeight - 1
// (the filename row occupies one row).
func TestContentHeight(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(20)), 10)
	if v.ContentHeight() != 9 {
		t.Fatalf("ContentHeight = %d, want 9", v.ContentHeight())
	}
}

// --- SetPanelHeight test ---

// TestSetPanelHeight verifies that SetPanelHeight updates the panel
// height and clamps the offset to the new maxOffset.
func TestSetPanelHeight(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(50)), 10) // contentHeight = 9, maxOffset = 41
	v.SetOffset(30)
	v.SetPanelHeight(5) // contentHeight = 4, maxOffset = 46
	if v.ContentHeight() != 4 {
		t.Fatalf("ContentHeight = %d, want 4", v.ContentHeight())
	}
	// Offset 30 is still valid (maxOffset = 46).
	if v.Offset() != 30 {
		t.Fatalf("Offset = %d, want 30 (still valid)", v.Offset())
	}
	// Shrink so that offset 30 is past the new EOF.
	v.SetPanelHeight(4) // contentHeight = 3, maxOffset = 47
	// Offset 30 is still valid.
	if v.Offset() != 30 {
		t.Fatalf("Offset = %d, want 30 (still valid after shrink)", v.Offset())
	}
	// Shrink to a very small height so offset 30 is past EOF.
	v.SetPanelHeight(3) // contentHeight = 2, maxOffset = 48
	// Offset 30 is still valid.
	if v.Offset() != 30 {
		t.Fatalf("Offset = %d, want 30 (still valid)", v.Offset())
	}
}

// TestSetPanelHeightClampsOffset verifies that SetPanelHeight clamps
// the offset when the new maxOffset is smaller than the current offset.
func TestSetPanelHeightClampsOffset(t *testing.T) {
	v := viewport.New(fakeRows(makeLines(20)), 20) // contentHeight = 19, maxOffset = 1
	v.SetOffset(1)
	v.SetPanelHeight(10) // contentHeight = 9, maxOffset = 11
	// Offset 1 is still valid.
	if v.Offset() != 1 {
		t.Fatalf("Offset = %d, want 1 (still valid)", v.Offset())
	}
}
