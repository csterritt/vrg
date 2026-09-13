package viewport_test

import (
	"os"
	"path/filepath"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// writeFile creates a file in dir with the given content and returns its
// full path. (Local copy for the viewport_test package.)
func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// --- Zero-width marker viewport tests (Issue #23) ---
//
// These tests cover the Issue #23 viewport contracts: markers
// participate in wrap (a marker after a completely full wrap row
// occupies another row), horizontal extent (a marker-only line has
// extent 1), pan clamping (a marker-only line has maximum offset 0),
// reveal targeting (the marker cell is a navigable target), and
// clipping (a marker hidden left or right is excluded from the
// clipped output, driving Issue #20's indicators).
//
// All tests use filebuffer.Load to produce the buffer so the full
// FileBuffer → Viewport path is exercised.

// loadMarkerBuf loads a file with the given content and zero-width
// submatch stops, returning the buffer. The gutter is set to 3 (one
// digit + two spaces) for consistent text-width arithmetic.
func loadMarkerBuf(t *testing.T, content string, stops []searchindex.Stop) *filebuffer.Buffer {
	t.Helper()
	dir := t.TempDir()
	p := writeFile(t, dir, "test.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	return buf
}

// mkEOLStop creates a stop with a zero-width submatch at the given
// byte position (EOL terminator).
func mkEOLStop(path string, lineNo int, line string, bytePos int) searchindex.Stop {
	return searchindex.Stop{
		RawPath:    []byte(path),
		LineNumber: lineNo,
		Line:       []byte(line),
		Submatches: []searchindex.Submatch{{Match: []byte(""), Start: bytePos, End: bytePos}},
	}
}

// TestMarkerAfterFullWrapRow verifies that a marker after a completely
// full wrap row occupies another row. A line that exactly fills the
// text width has its EOL marker on the next wrap row.
func TestMarkerAfterFullWrapRow(t *testing.T) {
	// 10 ASCII chars at text width 10: row 0 is exactly full. The EOL
	// marker (cell 10) wraps to row 1.
	content := "0123456789\n"
	stops := []searchindex.Stop{
		mkEOLStop("test.txt", 1, content, 10),
	}
	buf := loadMarkerBuf(t, content, stops)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	if model.RowCount() != 2 {
		t.Fatalf("RowCount = %d, want 2 (10 chars fill row 0, EOL marker on row 1)", model.RowCount())
	}
	rows := model.Rows(0, 2)
	// Row 1 should contain the marker cell. The marker is a 1-cell
	// cluster at the start of row 1.
	row1 := rows[1]
	rowWidth := 0
	for _, c := range row1.Clusters {
		rowWidth += c.Width
	}
	if rowWidth != 1 {
		t.Fatalf("Row 1 width = %d, want 1 (EOL marker cell)", rowWidth)
	}
}

// TestMarkerOnlyLineExtentOne verifies that a marker-only line (an
// empty line with an EOL marker) has content extent 1. The viewport's
// widestLine computation includes the marker cell.
func TestMarkerOnlyLineExtentOne(t *testing.T) {
	// Empty line with a zero-width match at byte 0 (the \n).
	content := "\n"
	stops := []searchindex.Stop{
		mkEOLStop("test.txt", 1, content, 0),
	}
	buf := loadMarkerBuf(t, content, stops)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOff, key)
	v := viewport.New(model, 24)
	v.SetLayout(10, viewport.WrapOff)
	if v.MaxHOffset() != 0 {
		t.Fatalf("MaxHOffset = %d, want 0 (marker-only line has extent 1, max offset 0)", v.MaxHOffset())
	}
	// The visible row should have cluster width 1 (the marker cell).
	rows := model.Rows(0, 1)
	if len(rows) != 1 {
		t.Fatalf("Rows len = %d, want 1", len(rows))
	}
	rowWidth := 0
	for _, c := range rows[0].Clusters {
		rowWidth += c.Width
	}
	if rowWidth != 1 {
		t.Fatalf("Row width = %d, want 1 (marker-only line has extent 1)", rowWidth)
	}
}

// TestMarkerOnlyLineMaxPanOffsetZero verifies that a marker-only line
// has maximum pan offset 0 under Issue #18's paintable-boundary
// maximum. The marker cell starts at cell 0 and fits the text width,
// so the maximum offset is 0.
func TestMarkerOnlyLineMaxPanOffsetZero(t *testing.T) {
	content := "\n"
	stops := []searchindex.Stop{
		mkEOLStop("test.txt", 1, content, 0),
	}
	buf := loadMarkerBuf(t, content, stops)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOff, key)
	v := viewport.New(model, 24)
	v.SetLayout(10, viewport.WrapOff)
	// Panning right should clamp to 0.
	v.Pan(5)
	if v.HOffset() != 0 {
		t.Fatalf("HOffset = %d, want 0 (marker-only line clamps to max offset 0)", v.HOffset())
	}
}

// TestMarkerCellRevealTarget verifies that the marker cell is a
// navigable reveal target. ClusterWidthAtCell returns 1 for the marker
// cell, so RevealHorizontal can target it.
func TestMarkerCellRevealTarget(t *testing.T) {
	content := "hit\n"
	stops := []searchindex.Stop{
		mkEOLStop("test.txt", 1, content, 3),
	}
	buf := loadMarkerBuf(t, content, stops)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOff, key)
	v := viewport.New(model, 24)
	v.SetLayout(10, viewport.WrapOff)
	// The marker cell is at column 3 (EOL of "hit").
	rows := model.Rows(0, 1)
	if len(rows) != 1 {
		t.Fatalf("Rows len = %d, want 1", len(rows))
	}
	line := rows[0]
	// ClusterWidthAtCell at the marker cell (3) should return 1.
	w := viewport.ClusterWidthAtCell(line.Clusters, 3)
	if w != 1 {
		t.Fatalf("ClusterWidthAtCell(3) = %d, want 1 (marker cell width)", w)
	}
	// RevealHorizontal to the marker cell should make it visible.
	v.SetHOffset(0)
	v.RevealHorizontal(line, 3)
	// The marker cell [3, 4) should be within the window.
	if v.HOffset() > 3 {
		t.Fatalf("HOffset = %d, want <= 3 (marker cell revealed)", v.HOffset())
	}
	windowEnd := v.HOffset() + v.TextWidth()
	if 3 < v.HOffset() || 4 > windowEnd {
		t.Fatalf("Marker cell [3, 4) not fully visible in window [%d, %d)", v.HOffset(), windowEnd)
	}
}

// TestMarkerHiddenLeftNotInClip verifies that a marker hidden left of
// the clip window is not in the clipped output. When the marker cell
// is entirely left of the window, the clipped line's highlights do not
// include the marker. This drives the Issue #20 left `*` indicator.
func TestMarkerHiddenLeftNotInClip(t *testing.T) {
	// "hit" + EOL marker at cell 3. Pan right by 4: the marker at cell 3
	// is hidden left.
	content := "hit\n"
	stops := []searchindex.Stop{
		mkEOLStop("test.txt", 1, content, 3),
	}
	buf := loadMarkerBuf(t, content, stops)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOff, key)
	v := viewport.New(model, 24)
	v.SetLayout(10, viewport.WrapOff)
	v.SetHOffset(4) // window [4, 14): marker at cell 3 is hidden left.
	rows := model.Rows(0, 1)
	clipped := v.ClipLine(rows[0])
	for _, hl := range clipped.Highlights {
		// The marker highlight [3, 4) should not appear in the clipped
		// output (it's entirely left of the window). In local coords,
		// it would be [-1, 0) which is invalid, so it must be absent.
		if hl[1] <= 0 {
			t.Fatalf("Clipped highlights contain marker %v (should be hidden left)", hl)
		}
	}
}

// TestMarkerVisibleInClip verifies that a marker within the clip
// window is included in the clipped output. The marker cell is
// rendered as a space and the highlight is shifted to local
// coordinates.
func TestMarkerVisibleInClip(t *testing.T) {
	content := "hit\n"
	stops := []searchindex.Stop{
		mkEOLStop("test.txt", 1, content, 3),
	}
	buf := loadMarkerBuf(t, content, stops)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOff, key)
	v := viewport.New(model, 24)
	v.SetLayout(10, viewport.WrapOff)
	// Window [0, 10): marker at cell 3 is visible.
	rows := model.Rows(0, 1)
	clipped := v.ClipLine(rows[0])
	// The clipped display should contain the marker space at position 3.
	if len(clipped.Display) < 4 {
		t.Fatalf("Clipped display = %q, want at least 4 chars (hit + marker space)", clipped.Display)
	}
	if clipped.Display[3] != ' ' {
		t.Fatalf("Clipped display[3] = %q, want ' ' (marker cell)", string(clipped.Display[3]))
	}
	// The marker highlight should be at local [3, 4).
	found := false
	for _, hl := range clipped.Highlights {
		if hl[0] == 3 && hl[1] == 4 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Clipped highlights = %v, want a [3, 4) marker (visible)", clipped.Highlights)
	}
}

// TestMarkerHiddenRightNotInClip verifies that a marker hidden right
// of the clip window is not in the clipped output. When the marker
// cell is entirely right of the window, the clipped line's highlights
// do not include the marker. This drives the Issue #20 right `*`
// indicator.
func TestMarkerHiddenRightNotInClip(t *testing.T) {
	// 10 ASCII chars + EOL marker at cell 10. Text width 5: window
	// [0, 5). The marker at cell 10 is hidden right.
	content := "0123456789\n"
	stops := []searchindex.Stop{
		mkEOLStop("test.txt", 1, content, 10),
	}
	buf := loadMarkerBuf(t, content, stops)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 5, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 5, viewport.WrapOff, key)
	v := viewport.New(model, 24)
	v.SetLayout(5, viewport.WrapOff)
	rows := model.Rows(0, 1)
	clipped := v.ClipLine(rows[0])
	// The clipped display should be 5 chars (the first 5 of
	// "0123456789 "), not including the marker.
	if len(clipped.Display) > 5 {
		t.Fatalf("Clipped display = %q, want at most 5 chars (marker hidden right)", clipped.Display)
	}
	// The marker highlight [10, 11) should not appear in local coords.
	for _, hl := range clipped.Highlights {
		if hl[0] >= 0 && hl[1] > 5 {
			t.Fatalf("Clipped highlights contain marker %v (should be hidden right)", hl)
		}
	}
}

// TestMarkerParticipatesInPanClamping verifies that a line with an EOL
// marker has its extent include the marker cell, so the pan clamp
// allows scrolling to see the marker. A line "hit" + EOL marker has
// extent 4; at text width 3, the max offset is 1 (so the marker cell
// at position 3 is reachable).
func TestMarkerParticipatesInPanClamping(t *testing.T) {
	content := "hit\n"
	stops := []searchindex.Stop{
		mkEOLStop("test.txt", 1, content, 3),
	}
	buf := loadMarkerBuf(t, content, stops)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 3, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 3, viewport.WrapOff, key)
	v := viewport.New(model, 24)
	v.SetLayout(3, viewport.WrapOff)
	// Extent is 4 (hit=3 + marker=1). At text width 3, the marker
	// cluster starts at cell 3 and its width (1) fits the text width,
	// so the max offset includes the marker cell at position 3.
	maxOff := v.MaxHOffset()
	if maxOff < 3 {
		t.Fatalf("MaxHOffset = %d, want >= 3 (marker at cell 3 participates in pan clamping)", maxOff)
	}
}

// TestMarkerEOLWrapRowCount verifies that an EOL marker on a line that
// does not fill the wrap row stays on the same row. A 5-char line at
// text width 10: the EOL marker (cell 5) fits on the same row.
func TestMarkerEOLWrapRowCount(t *testing.T) {
	content := "hello\n"
	stops := []searchindex.Stop{
		mkEOLStop("test.txt", 1, content, 5),
	}
	buf := loadMarkerBuf(t, content, stops)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 10, WrapMode: viewport.WrapOn}
	model := viewport.BuildRowModel(buf, 10, viewport.WrapOn, key)
	// 5 chars + 1 marker = 6 cells, fits in width 10 → 1 row.
	if model.RowCount() != 1 {
		t.Fatalf("RowCount = %d, want 1 (5 chars + marker fits in width 10)", model.RowCount())
	}
}

// TestMarkerEOLRevealHorizontal verifies that RevealHorizontal can
// target the EOL marker cell and make it visible. The marker cell
// has cluster width 1, so the reveal arithmetic works like any
// single-cell target.
func TestMarkerEOLRevealHorizontal(t *testing.T) {
	content := "hit\n"
	stops := []searchindex.Stop{
		mkEOLStop("test.txt", 1, content, 3),
	}
	buf := loadMarkerBuf(t, content, stops)
	key := viewport.RowModelKey{Path: "test.txt", Revision: 1, TextWidth: 3, WrapMode: viewport.WrapOff}
	model := viewport.BuildRowModel(buf, 3, viewport.WrapOff, key)
	v := viewport.New(model, 24)
	v.SetLayout(3, viewport.WrapOff)
	// Pan right so the marker is hidden right.
	v.SetHOffset(0)
	// The marker is at cell 3, window [0, 3). The marker starts at 3,
	// which is at the right edge. RevealHorizontal should make it
	// visible.
	rows := model.Rows(0, 1)
	line := rows[0]
	v.RevealHorizontal(line, 3)
	// After reveal, the marker cell [3, 4) should be within the window.
	windowEnd := v.HOffset() + v.TextWidth()
	if 3 < v.HOffset() || 4 > windowEnd {
		t.Fatalf("Marker cell [3, 4) not visible after reveal: window [%d, %d)", v.HOffset(), windowEnd)
	}
}
