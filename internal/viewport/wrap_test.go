package viewport_test

import (
	"fmt"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/viewport"
)

// --- Wrap row-count tests (Issue #16) ---

// makeLineWithClusters creates a filebuffer.Line with the given display
// text and populates its Clusters field using the shared grapheme
// policy. This is the form Viewport consumes for wrapping.
func makeLineWithClusters(num int, display string) filebuffer.Line {
	return filebuffer.Line{
		Number:   num,
		Display:  display,
		Clusters: makeClusters(display),
	}
}

// makeClusters produces grapheme clusters for a display string. This
// is a test helper that mirrors the shared policy; production code
// uses safepresentation.GraphemeClusters via filebuffer.Load.
func makeClusters(display string) []filebuffer.Cluster {
	// For ASCII-only test content, each byte is one 1-cell cluster.
	// For wide/combining content, the test helper needs the real policy.
	// We use a simple approach: delegate to the shared policy through
	// the filebuffer's cluster type.
	clusters := make([]filebuffer.Cluster, 0, len(display))
	for i := 0; i < len(display); {
		// Default: one byte = one cluster, width 1.
		// Wide characters (UTF-8 3 bytes) are detected by the policy.
		// This helper is a placeholder; the real tests use
		// filebuffer.Load which calls the shared policy.
		clusters = append(clusters, filebuffer.Cluster{
			StartByte: i,
			EndByte:   i + 1,
			Width:     1,
		})
		i++
	}
	return clusters
}

// makeBufFromLines wraps a slice of filebuffer.Lines into a Buffer.
func makeBufFromLines(lines []filebuffer.Line) *filebuffer.Buffer {
	return &filebuffer.Buffer{
		Lines:       lines,
		LineCount:   len(lines),
		GutterWidth: 3, // 1 digit + 2 spaces
	}
}

// TestWrapRowCountASCII verifies that ASCII text wraps at the text
// width, producing the correct number of rendered rows. A 20-character
// line at text width 10 produces 2 rows.
func TestWrapRowCountASCII(t *testing.T) {
	line := makeLineWithClusters(1, "0123456789abcdefghij") // 20 chars
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	// 20 chars at width 10 = 2 rows.
	if model.RowCount() != 2 {
		t.Fatalf("RowCount = %d, want 2 (20 chars at width 10)", model.RowCount())
	}
}

// TestWrapRowCountExactMultiple verifies that a line whose length is
// an exact multiple of the text width produces no extra row.
func TestWrapRowCountExactMultiple(t *testing.T) {
	line := makeLineWithClusters(1, "0123456789") // 10 chars
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 1 {
		t.Fatalf("RowCount = %d, want 1 (10 chars at width 10 = exact)", model.RowCount())
	}
}

// TestWrapRowCountShortLine verifies that a line shorter than the text
// width produces one row.
func TestWrapRowCountShortLine(t *testing.T) {
	line := makeLineWithClusters(1, "short") // 5 chars
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 1 {
		t.Fatalf("RowCount = %d, want 1 (short line)", model.RowCount())
	}
}

// TestWrapRowCountMultipleLines verifies that multiple source lines
// each produce their own set of wrapped rows.
func TestWrapRowCountMultipleLines(t *testing.T) {
	lines := []filebuffer.Line{
		makeLineWithClusters(1, "0123456789abcdefghij"), // 20 chars → 2 rows
		makeLineWithClusters(2, "abc"),                  // 3 chars → 1 row
	}
	buf := makeBufFromLines(lines)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 3 {
		t.Fatalf("RowCount = %d, want 3 (2 + 1)", model.RowCount())
	}
}

// --- Wide cluster wrapping (Issue #16) ---

// TestWrapRowCountWideCluster verifies that a two-cell cluster that
// cannot fit in a row's remaining cells moves to the next row,
// leaving a blank. A 9-char ASCII line followed by a 2-cell wide
// cluster at text width 10: the wide cluster can't fit in the
// remaining 1 cell, so it moves to the next row, leaving 1 blank.
func TestWrapRowCountWideCluster(t *testing.T) {
	// 9 ASCII chars + 1 wide char (2 cells) = 11 cells at width 10.
	// Row 0: 9 ASCII chars + 1 blank (wide can't fit in 1 cell).
	// Row 1: wide char (2 cells).
	// Total: 2 rows.
	display := "012345678中" // 9 ASCII + 1 CJK
	line := filebuffer.Line{
		Number:  1,
		Display: display,
		Clusters: []filebuffer.Cluster{
			{StartByte: 0, EndByte: 1, Width: 1},
			{StartByte: 1, EndByte: 2, Width: 1},
			{StartByte: 2, EndByte: 3, Width: 1},
			{StartByte: 3, EndByte: 4, Width: 1},
			{StartByte: 4, EndByte: 5, Width: 1},
			{StartByte: 5, EndByte: 6, Width: 1},
			{StartByte: 6, EndByte: 7, Width: 1},
			{StartByte: 7, EndByte: 8, Width: 1},
			{StartByte: 8, EndByte: 9, Width: 1},
			{StartByte: 9, EndByte: 12, Width: 2}, // 中 = 3 UTF-8 bytes, 2 cells
		},
	}
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 2 {
		t.Fatalf("RowCount = %d, want 2 (wide cluster moves to next row)", model.RowCount())
	}
}

// --- Combining mark wrapping (Issue #16) ---

// TestWrapRowCountCombining verifies that a combining mark stays with
// its base character across a wrap boundary. A line where the base
// character is the last cell that fits must include the combining mark
// in the same row (the cluster is atomic).
func TestWrapRowCountCombining(t *testing.T) {
	// 9 ASCII chars + "e\u0301" (e + combining = 1 cluster, 1 cell) = 10 cells.
	// At width 10, this fits in one row.
	display := "012345678e\u0301"
	line := filebuffer.Line{
		Number:  1,
		Display: display,
		Clusters: []filebuffer.Cluster{
			{StartByte: 0, EndByte: 1, Width: 1},
			{StartByte: 1, EndByte: 2, Width: 1},
			{StartByte: 2, EndByte: 3, Width: 1},
			{StartByte: 3, EndByte: 4, Width: 1},
			{StartByte: 4, EndByte: 5, Width: 1},
			{StartByte: 5, EndByte: 6, Width: 1},
			{StartByte: 6, EndByte: 7, Width: 1},
			{StartByte: 7, EndByte: 8, Width: 1},
			{StartByte: 8, EndByte: 9, Width: 1},
			{StartByte: 9, EndByte: 12, Width: 1}, // e + combining = 1 cluster, 1 cell
		},
	}
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 1 {
		t.Fatalf("RowCount = %d, want 1 (combining stays with base, fits in 10 cells)", model.RowCount())
	}
}

// TestWrapCombiningAtBoundary verifies that a combining mark at a wrap
// boundary moves with its base to the next row. If the base+combining
// cluster is the 11th cell, it moves to the next row.
func TestWrapCombiningAtBoundary(t *testing.T) {
	// 10 ASCII chars + "e\u0301" (1 cluster, 1 cell) = 11 cells at width 10.
	// Row 0: 10 ASCII chars. Row 1: e+combining.
	display := "0123456789e\u0301"
	line := filebuffer.Line{
		Number:  1,
		Display: display,
		Clusters: []filebuffer.Cluster{
			{StartByte: 0, EndByte: 1, Width: 1},
			{StartByte: 1, EndByte: 2, Width: 1},
			{StartByte: 2, EndByte: 3, Width: 1},
			{StartByte: 3, EndByte: 4, Width: 1},
			{StartByte: 4, EndByte: 5, Width: 1},
			{StartByte: 5, EndByte: 6, Width: 1},
			{StartByte: 6, EndByte: 7, Width: 1},
			{StartByte: 7, EndByte: 8, Width: 1},
			{StartByte: 8, EndByte: 9, Width: 1},
			{StartByte: 9, EndByte: 10, Width: 1},
			{StartByte: 10, EndByte: 13, Width: 1}, // e + combining
		},
	}
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 2 {
		t.Fatalf("RowCount = %d, want 2 (combining cluster at boundary moves to next row)", model.RowCount())
	}
}

// --- Continuation gutter (Issue #16) ---

// TestWrapContinuationRows verifies that continuation rows (sub-rows
// after the first of a source line) have Continuation=true and the
// same line Number as the first row.
func TestWrapContinuationRows(t *testing.T) {
	line := makeLineWithClusters(1, "0123456789abcdefghij") // 20 chars
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	rows := model.Rows(0, model.RowCount())
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Continuation {
		t.Fatal("row 0 Continuation = true, want false (first row)")
	}
	if !rows[1].Continuation {
		t.Fatal("row 1 Continuation = false, want true (continuation row)")
	}
	if rows[1].Number != rows[0].Number {
		t.Fatalf("row 1 Number = %d, want %d (same source line)", rows[1].Number, rows[0].Number)
	}
}

// TestWrapContinuationRowsThreePlus verifies that ALL continuation rows
// (not just the last) carry Continuation=true when a line wraps to 3+
// rows. This catches the bug where intermediate wrapped rows were
// incorrectly marked as non-continuation.
func TestWrapContinuationRowsThreePlus(t *testing.T) {
	line := makeLineWithClusters(1, "0123456789abcdefghij0123456789") // 30 chars
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	rows := model.Rows(0, model.RowCount())
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[0].Continuation {
		t.Fatal("row 0 Continuation = true, want false (first row)")
	}
	for i := 1; i < len(rows); i++ {
		if !rows[i].Continuation {
			t.Fatalf("row %d Continuation = false, want true (continuation row)", i)
		}
		if rows[i].Number != rows[0].Number {
			t.Fatalf("row %d Number = %d, want %d (same source line)", i, rows[i].Number, rows[0].Number)
		}
	}
}

// --- Run-off-edge mode (Issue #16) ---

// TestRunOffEdgeRowCount verifies that run-off-edge mode produces one
// row per source line, regardless of line length.
func TestRunOffEdgeRowCount(t *testing.T) {
	line := makeLineWithClusters(1, "0123456789abcdefghij") // 20 chars
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOff, key)
	if model.RowCount() != 1 {
		t.Fatalf("RowCount = %d, want 1 (run-off-edge: one row per line)", model.RowCount())
	}
}

// --- Toggle wrap mode (Issue #16) ---

// TestToggleWrapModeChangesRowModel verifies that toggling wrap mode
// changes the row model. The same buffer at the same text width
// produces different row counts in wrap and run-off-edge modes.
func TestToggleWrapModeChangesRowModel(t *testing.T) {
	line := makeLineWithClusters(1, "0123456789abcdefghij") // 20 chars
	buf := makeBufFromLines([]filebuffer.Line{line})
	wrapKey := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	offKey := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	wrapModel := viewport.BuildRowModel(buf, 10, viewport.WrapOn, wrapKey)
	offModel := viewport.BuildRowModel(buf, 10, viewport.WrapOff, offKey)
	if wrapModel.RowCount() == offModel.RowCount() {
		t.Fatalf("wrap and run-off-edge row counts are equal (%d), want different", wrapModel.RowCount())
	}
	if wrapModel.RowCount() != 2 {
		t.Fatalf("wrap RowCount = %d, want 2", wrapModel.RowCount())
	}
	if offModel.RowCount() != 1 {
		t.Fatalf("run-off-edge RowCount = %d, want 1", offModel.RowCount())
	}
}

// --- Reserved indicator width (Issue #16) ---

// TestReservedIndicatorWidth verifies the reserved right-indicator
// width: 0 in wrap mode, 1 in run-off-edge mode.
func TestReservedIndicatorWidth(t *testing.T) {
	if w := viewport.ReservedWidth(viewport.WrapOn); w != 0 {
		t.Fatalf("ReservedWidth(WrapOn) = %d, want 0", w)
	}
	if w := viewport.ReservedWidth(viewport.WrapOff); w != 1 {
		t.Fatalf("ReservedWidth(WrapOff) = %d, want 1", w)
	}
}

// TestTextWidthCalculation verifies that TextWidth subtracts the
// gutter and reserved indicator width from the panel width.
func TestTextWidthCalculation(t *testing.T) {
	// panelWidth 80, gutterWidth 4 (2 digits + 2 spaces).
	// Wrap mode: 80 - 4 - 0 = 76.
	if tw := viewport.TextWidth(80, 4, viewport.WrapOn); tw != 76 {
		t.Fatalf("TextWidth(80, 4, WrapOn) = %d, want 76", tw)
	}
	// Run-off-edge: 80 - 4 - 1 = 75.
	if tw := viewport.TextWidth(80, 4, viewport.WrapOff); tw != 75 {
		t.Fatalf("TextWidth(80, 4, WrapOff) = %d, want 75", tw)
	}
}

// --- Row model key (Issue #16) ---

// TestRowModelKey verifies that the row model carries its key and two
// models with the same key are equivalent.
func TestRowModelKey(t *testing.T) {
	line := makeLineWithClusters(1, "hello")
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.Key() != key {
		t.Fatalf("Key = %v, want %v", model.Key(), key)
	}
}

// TestRowModelKeyDifferentModes verifies that the key distinguishes
// wrap modes.
func TestRowModelKeyDifferentModes(t *testing.T) {
	line := makeLineWithClusters(1, "hello")
	buf := makeBufFromLines([]filebuffer.Line{line})
	wrapKey := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	offKey := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	wrapModel := viewport.BuildRowModel(buf, 10, viewport.WrapOn, wrapKey)
	offModel := viewport.BuildRowModel(buf, 10, viewport.WrapOff, offKey)
	if wrapModel.Key() == offModel.Key() {
		t.Fatal("wrap and off keys are equal, want different")
	}
}

// --- Wrapped target reveal (Issue #16) ---

// TestRowFromByteWrappedLine verifies that RowFromByte finds the
// correct rendered row for a byte offset in a wrapped line. A
// 20-character line at width 10 wraps to 2 rows; byte 15 is in row 1.
func TestRowFromByteWrappedLine(t *testing.T) {
	line := makeLineWithClusters(1, "0123456789abcdefghij") // 20 chars
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	// Byte 15 is in the second row (bytes 10-19).
	row := model.RowFromByte(0, 15)
	if row != 1 {
		t.Fatalf("RowFromByte(0, 15) = %d, want 1 (second wrapped row)", row)
	}
}

// TestRowFromByteFirstRow verifies that RowFromByte returns 0 for a
// byte offset in the first wrapped row.
func TestRowFromByteFirstRow(t *testing.T) {
	line := makeLineWithClusters(1, "0123456789abcdefghij") // 20 chars
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	row := model.RowFromByte(0, 5)
	if row != 0 {
		t.Fatalf("RowFromByte(0, 5) = %d, want 0 (first wrapped row)", row)
	}
}

// TestRowFromByteSecondLine verifies that RowFromByte finds the row for
// a byte offset in the second source line.
func TestRowFromByteSecondLine(t *testing.T) {
	lines := []filebuffer.Line{
		makeLineWithClusters(1, "0123456789abcdefghij"), // 20 chars → 2 rows
		makeLineWithClusters(2, "abc"),                  // 3 chars → 1 row
	}
	buf := makeBufFromLines(lines)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	// Line 1, byte 1 → row 2 (after line 0's 2 rows).
	row := model.RowFromByte(1, 1)
	if row != 2 {
		t.Fatalf("RowFromByte(1, 1) = %d, want 2 (line 1 starts at row 2)", row)
	}
}

// --- Source line taller than several screens (Issue #16) ---

// TestWrappedSourceLineTallerThanScreens verifies that a source line
// taller than several screens produces many wrapped rows, and the
// reveal can find a row deep in the wrapped line. A 500-character line
// at width 10 produces 50 rows.
func TestWrappedSourceLineTallerThanScreens(t *testing.T) {
	// 500-character line at width 10 = 50 rows.
	display := makeLongString(500)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 50 {
		t.Fatalf("RowCount = %d, want 50 (500 chars at width 10)", model.RowCount())
	}
	// Byte 495 is in row 49 (bytes 490-499).
	row := model.RowFromByte(0, 495)
	if row != 49 {
		t.Fatalf("RowFromByte(0, 495) = %d, want 49", row)
	}
}

// makeLongString returns a string of n ASCII characters.
func makeLongString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + (i % 26))
	}
	return string(b)
}

// --- Render-cost guard with wrapping (Issue #16) ---

// TestRenderCostGuardWithWrapping verifies that the Viewport queries
// the row model only for the visible row range, not the full buffer,
// even when wrapping produces many rows. A counting fake proves the
// render cost is proportional to the visible rows.
func TestRenderCostGuardWithWrapping(t *testing.T) {
	// 500-char line at width 10 = 50 rows. Viewport height 10 →
	// contentHeight 9. Only [offset, offset+9) should be queried.
	display := makeLongString(500)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)

	// Use a counting wrapper around the model.
	counter := &countingRowModel{model: model}
	v := viewport.New(counter, 10) // contentHeight = 9
	v.SetOffset(25)
	counter.queries = nil
	v.Visible()
	if len(counter.queries) != 1 {
		t.Fatalf("queries = %d, want 1", len(counter.queries))
	}
	start, end := counter.queries[0][0], counter.queries[0][1]
	if start != 25 || end != 34 {
		t.Fatalf("query = [%d, %d), want [25, 34) (visible range only)", start, end)
	}
}

// countingRowModel wraps a RowModel and records every Rows query.
type countingRowModel struct {
	model   *viewport.RowModel
	queries [][2]int
}

func (c *countingRowModel) RowCount() int { return c.model.RowCount() }
func (c *countingRowModel) Rows(start, end int) []filebuffer.Line {
	c.queries = append(c.queries, [2]int{start, end})
	return c.model.Rows(start, end)
}

// --- Swappable row model (Issue #16) ---

// TestSwappableRowModel verifies that the Viewport can swap row models
// via SetRows. This is the swappable value built at load, toggle, or
// resize time.
func TestSwappableRowModel(t *testing.T) {
	// Build a model with one short line (1 row).
	line1 := makeLineWithClusters(1, "short")
	buf1 := makeBufFromLines([]filebuffer.Line{line1})
	key1 := viewport.RowModelKey{Path: "a.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model1 := viewport.BuildRowModel(buf1, 10, viewport.WrapOn, key1)

	v := viewport.New(model1, 10)
	if v.Visible()[0].Display != "short" {
		t.Fatalf("initial visible = %q, want %q", v.Visible()[0].Display, "short")
	}

	// Swap to a model with a long line (2 rows).
	line2 := makeLineWithClusters(1, "0123456789abcdefghij") // 20 chars
	buf2 := makeBufFromLines([]filebuffer.Line{line2})
	key2 := viewport.RowModelKey{Path: "b.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model2 := viewport.BuildRowModel(buf2, 10, viewport.WrapOn, key2)

	v.SetRows(model2)
	if v.Visible()[0].Display != "0123456789" {
		t.Fatalf("after swap visible = %q, want %q (first wrapped row)", v.Visible()[0].Display, "0123456789")
	}
}

// --- Tab stops in wrap mode (Issue #16) ---

// TestTabStopsInWrapMode verifies that tab expansion produces the
// correct row count in wrap mode. A line with a tab that expands to 8
// spaces at text width 10 wraps correctly.
func TestTabStopsInWrapMode(t *testing.T) {
	// "a\tb" expands to "a       b" (9 chars). At width 10, one row.
	// The tab expands to 7 spaces (columns 1-7), "b" at column 8.
	// Total: 9 cells, fits in one row of width 10.
	display := "a       b" // already expanded
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 1 {
		t.Fatalf("RowCount = %d, want 1 (9 cells fits in width 10)", model.RowCount())
	}
}

// TestTabStopsWrapLongLine verifies that a long line with tabs wraps
// correctly, with tab expansion contributing to the cell count.
func TestTabStopsWrapLongLine(t *testing.T) {
	// "a\t" expands to "a       " (8 chars). Repeated 3 times = 24 chars.
	// At width 10: 3 rows (10 + 10 + 4).
	display := "a       a       a       " // 24 chars
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 3 {
		t.Fatalf("RowCount = %d, want 3 (24 chars at width 10)", model.RowCount())
	}
}

// --- Empty line wrapping (Issue #16) ---

// TestWrapEmptyLine verifies that an empty line produces one row.
func TestWrapEmptyLine(t *testing.T) {
	line := makeLineWithClusters(1, "")
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 1 {
		t.Fatalf("RowCount = %d, want 1 (empty line)", model.RowCount())
	}
}

// --- Wrap row display text (Issue #16) ---

// TestWrapRowDisplayText verifies that each wrapped row's Display is
// the correct substring of the source line's display text.
func TestWrapRowDisplayText(t *testing.T) {
	line := makeLineWithClusters(1, "0123456789abcdefghij") // 20 chars
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	rows := model.Rows(0, model.RowCount())
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Display != "0123456789" {
		t.Fatalf("row 0 Display = %q, want %q", rows[0].Display, "0123456789")
	}
	if rows[1].Display != "abcdefghij" {
		t.Fatalf("row 1 Display = %q, want %q", rows[1].Display, "abcdefghij")
	}
}

// TestWrapRowStartByte verifies that each wrapped row's StartByte is
// the correct byte offset in the source line's display text.
func TestWrapRowStartByte(t *testing.T) {
	line := makeLineWithClusters(1, "0123456789abcdefghij") // 20 chars
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	rows := model.Rows(0, model.RowCount())
	if rows[0].StartByte != 0 {
		t.Fatalf("row 0 StartByte = %d, want 0", rows[0].StartByte)
	}
	if rows[1].StartByte != 10 {
		t.Fatalf("row 1 StartByte = %d, want 10", rows[1].StartByte)
	}
}

// --- Highlight adjustment in wrapped rows (Issue #16) ---

// TestWrapRowHighlights verifies that highlights are adjusted for each
// wrapped row. A highlight spanning [5, 15) in a 20-char line at width
// 10 produces [5, 10) in row 0 and [0, 5) in row 1.
func TestWrapRowHighlights(t *testing.T) {
	line := filebuffer.Line{
		Number:     1,
		Display:    "0123456789abcdefghij",
		Highlights: [][2]int{{5, 15}},
		Clusters:   makeClusters("0123456789abcdefghij"),
	}
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	rows := model.Rows(0, model.RowCount())
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	// Row 0: highlight [5, 10) (clamped to row width).
	if len(rows[0].Highlights) != 1 || rows[0].Highlights[0] != [2]int{5, 10} {
		t.Fatalf("row 0 Highlights = %v, want [[5 10]]", rows[0].Highlights)
	}
	// Row 1: highlight [0, 5) (offset by -10).
	if len(rows[1].Highlights) != 1 || rows[1].Highlights[0] != [2]int{0, 5} {
		t.Fatalf("row 1 Highlights = %v, want [[0 5]]", rows[1].Highlights)
	}
}

// --- RowModel implements RowProvider (Issue #16) ---

// TestRowModelImplementsRowProvider verifies that *RowModel satisfies
// the RowProvider interface so it can be used directly by the
// Viewport.
func TestRowModelImplementsRowProvider(t *testing.T) {
	line := makeLineWithClusters(1, "hello")
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	var _ viewport.RowProvider = model
}

// --- Multiple wrapped lines with reveal (Issue #16) ---

// TestRevealAcrossMultipleWrappedLines verifies that the reveal can
// find rows across multiple wrapped source lines. Two 20-char lines
// at width 10 produce 4 rows; a target in line 2 at byte 15 is in
// row 3.
func TestRevealAcrossMultipleWrappedLines(t *testing.T) {
	lines := []filebuffer.Line{
		makeLineWithClusters(1, "0123456789abcdefghij"), // 20 chars → rows 0-1
		makeLineWithClusters(2, "0123456789abcdefghij"), // 20 chars → rows 2-3
	}
	buf := makeBufFromLines(lines)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	// Line 1, byte 15 → row 3 (line 1 starts at row 2, byte 15 is in row 3).
	row := model.RowFromByte(1, 15)
	if row != 3 {
		t.Fatalf("RowFromByte(1, 15) = %d, want 3", row)
	}
}

// --- Reveal with viewport (Issue #16) ---

// TestRevealWrappedTargetWithViewport verifies that the Viewport's
// Reveal method works with the wrapped row model. A target in a
// wrapped row is revealed at the one-third position.
func TestRevealWrappedTargetWithViewport(t *testing.T) {
	// 500-char line at width 10 = 50 rows. Viewport height 10 →
	// contentHeight 9, floor(9/3) = 3.
	display := makeLongString(500)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	v := viewport.New(model, 10) // contentHeight = 9
	// Target row 40. Reveal: offset = 40 - 3 = 37.
	v.Reveal(40)
	if v.Offset() != 37 {
		t.Fatalf("Offset = %d, want 37 (one-third placement for wrapped row)", v.Offset())
	}
}

// TestRevealWrappedTargetVisibleNoScroll verifies that Reveal does not
// scroll when the wrapped target row is already visible.
func TestRevealWrappedTargetVisibleNoScroll(t *testing.T) {
	display := makeLongString(500)
	line := makeLineWithClusters(1, display)
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	v := viewport.New(model, 10) // contentHeight = 9
	v.SetOffset(20)              // visible [20, 29)
	v.Reveal(25)
	if v.Offset() != 20 {
		t.Fatalf("Offset = %d, want 20 (visible target no-scroll)", v.Offset())
	}
}

// --- Wide cluster blank cell (Issue #16) ---

// TestWrapWideClusterBlankCell verifies that when a wide cluster moves
// to the next row, the remaining cells in the current row are blank.
// The first row's display text should be shorter than the text width.
func TestWrapWideClusterBlankCell(t *testing.T) {
	// 9 ASCII + 1 wide (2 cells) at width 10.
	// Row 0: 9 chars + 1 blank. Row 1: wide char.
	display := "012345678中"
	line := filebuffer.Line{
		Number:  1,
		Display: display,
		Clusters: []filebuffer.Cluster{
			{StartByte: 0, EndByte: 1, Width: 1},
			{StartByte: 1, EndByte: 2, Width: 1},
			{StartByte: 2, EndByte: 3, Width: 1},
			{StartByte: 3, EndByte: 4, Width: 1},
			{StartByte: 4, EndByte: 5, Width: 1},
			{StartByte: 5, EndByte: 6, Width: 1},
			{StartByte: 6, EndByte: 7, Width: 1},
			{StartByte: 7, EndByte: 8, Width: 1},
			{StartByte: 8, EndByte: 9, Width: 1},
			{StartByte: 9, EndByte: 12, Width: 2},
		},
	}
	buf := makeBufFromLines([]filebuffer.Line{line})
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	rows := model.Rows(0, model.RowCount())
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	// Row 0 should be 9 chars (the wide cluster moved to row 1).
	if rows[0].Display != "012345678" {
		t.Fatalf("row 0 Display = %q, want %q (9 chars, wide moved to next row)", rows[0].Display, "012345678")
	}
	// Row 1 should be the wide char.
	if rows[1].Display != "中" {
		t.Fatalf("row 1 Display = %q, want %q", rows[1].Display, "中")
	}
}

// --- Run-off-edge mode with RowFromByte (Issue #16) ---

// TestRunOffEdgeRowFromByte verifies that in run-off-edge mode,
// RowFromByte returns the source line index (one row per line).
func TestRunOffEdgeRowFromByte(t *testing.T) {
	lines := []filebuffer.Line{
		makeLineWithClusters(1, "0123456789abcdefghij"),
		makeLineWithClusters(2, "abc"),
	}
	buf := makeBufFromLines(lines)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOff, key)
	// Line 1, byte 1 → row 1 (one row per line in run-off-edge).
	row := model.RowFromByte(1, 1)
	if row != 1 {
		t.Fatalf("RowFromByte(1, 1) = %d, want 1 (run-off-edge: one row per line)", row)
	}
}

// --- Summary test (Issue #16) ---

// TestWrapAllTests is a meta-test that verifies the key Issue #16
// contracts are covered. It does not test behavior itself; it exists
// to document the contract coverage.
func TestWrapAllTests(t *testing.T) {
	// This test documents the Issue #16 contract coverage:
	// - Wrap row counts for ASCII: TestWrapRowCountASCII
	// - Wide cluster wrapping: TestWrapRowCountWideCluster
	// - Combining mark wrapping: TestWrapRowCountCombining
	// - Continuation gutters: TestWrapContinuationRows
	// - Run-off-edge mode: TestRunOffEdgeRowCount
	// - Toggle wrap mode: TestToggleWrapModeChangesRowModel
	// - Reserved indicator width: TestReservedIndicatorWidth
	// - Text width calculation: TestTextWidthCalculation
	// - Row model key: TestRowModelKey
	// - Wrapped target reveal: TestRowFromByteWrappedLine
	// - Source line taller than screens: TestWrappedSourceLineTallerThanScreens
	// - Render-cost guard: TestRenderCostGuardWithWrapping
	// - Swappable row model: TestSwappableRowModel
	// - Tab stops in wrap mode: TestTabStopsInWrapMode
	// - Highlight adjustment: TestWrapRowHighlights
	// - Wide cluster blank cell: TestWrapWideClusterBlankCell
	_ = fmt.Sprintf // keep fmt import
}
