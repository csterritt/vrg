package app_test

import (
	"strings"
	"testing"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
)

// makeLongLine creates a filebuffer.Line with a display string of the
// given number of ASCII characters, using the shared grapheme policy.
func makeLongLine(num int, n int) filebuffer.Line {
	display := makeLongASCII(n)
	return filebuffer.Line{
		Number:   num,
		Display:  display,
		Clusters: safepresentation.GraphemeClusters(display),
	}
}

// makeLongASCII returns a string of n ASCII characters.
func makeLongASCII(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + (i % 26))
	}
	return string(b)
}

// makeCJKLine creates a line starting with a CJK character (2 cells)
// followed by ASCII characters, using the shared grapheme policy.
func makeCJKLine(num int) filebuffer.Line {
	display := "中" + makeLongASCII(20)
	return filebuffer.Line{
		Number:   num,
		Display:  display,
		Clusters: safepresentation.GraphemeClusters(display),
	}
}

// setupBrowsePan creates a browse model in run-off-edge mode with long
// lines, ready for pan testing. The terminal is 80x24; the text width
// in run-off-edge mode is 80 - gutter - 1 (reserved indicator).
func setupBrowsePan(t *testing.T, lines []filebuffer.Line, lineCount, gutterWidth int) app.Model {
	t.Helper()
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf(lines, lineCount, gutterWidth)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	// Toggle to run-off-edge mode and deliver the layout preparation.
	m, cmd := update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	return m
}

// TestPanRightOneColumn verifies that `.` pans right by one column in
// run-off-edge mode.
func TestPanRightOneColumn(t *testing.T) {
	lines := []filebuffer.Line{makeLongLine(1, 300)}
	m := setupBrowsePan(t, lines, 1, 3)
	m, _ = update(t, m, keyPress('.'))
	if m.ViewportHOffset() != 1 {
		t.Fatalf("ViewportHOffset = %d, want 1 after '.'", m.ViewportHOffset())
	}
}

// TestPanLeftOneColumn verifies that `,` pans left by one column.
func TestPanLeftOneColumn(t *testing.T) {
	lines := []filebuffer.Line{makeLongLine(1, 300)}
	m := setupBrowsePan(t, lines, 1, 3)
	// Pan right a few times first.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, keyPress('.'))
	}
	m, _ = update(t, m, keyPress(','))
	if m.ViewportHOffset() != 4 {
		t.Fatalf("ViewportHOffset = %d, want 4 after ',' from 5", m.ViewportHOffset())
	}
}

// TestPanRightTenColumns verifies that `>` pans right by ten columns.
func TestPanRightTenColumns(t *testing.T) {
	lines := []filebuffer.Line{makeLongLine(1, 300)}
	m := setupBrowsePan(t, lines, 1, 3)
	m, _ = update(t, m, keyPress('>'))
	if m.ViewportHOffset() != 10 {
		t.Fatalf("ViewportHOffset = %d, want 10 after '>'", m.ViewportHOffset())
	}
}

// TestPanLeftTenColumns verifies that `<` pans left by ten columns.
func TestPanLeftTenColumns(t *testing.T) {
	lines := []filebuffer.Line{makeLongLine(1, 300)}
	m := setupBrowsePan(t, lines, 1, 3)
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, keyPress('>'))
	}
	m, _ = update(t, m, keyPress('<'))
	if m.ViewportHOffset() != 20 {
		t.Fatalf("ViewportHOffset = %d, want 20 after '<' from 30", m.ViewportHOffset())
	}
}

// TestPanRightHalfWidth verifies that `]` pans right by half the text
// width.
func TestPanRightHalfWidth(t *testing.T) {
	lines := []filebuffer.Line{makeLongLine(1, 300)}
	m := setupBrowsePan(t, lines, 1, 3)
	m, _ = update(t, m, keyPress(']'))
	// textWidth = 80 - 3 - 1 = 76, half = 38.
	if m.ViewportHOffset() != 38 {
		t.Fatalf("ViewportHOffset = %d, want 38 after ']' (half of 76)", m.ViewportHOffset())
	}
}

// TestPanLeftHalfWidth verifies that `[` pans left by half the text
// width.
func TestPanLeftHalfWidth(t *testing.T) {
	lines := []filebuffer.Line{makeLongLine(1, 300)}
	m := setupBrowsePan(t, lines, 1, 3)
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, keyPress(']'))
	}
	m, _ = update(t, m, keyPress('['))
	if m.ViewportHOffset() != 76 {
		t.Fatalf("ViewportHOffset = %d, want 76 after '[' from 114", m.ViewportHOffset())
	}
}

// TestPanNoOpInWrapMode verifies that pan keys are no-ops in wrap mode.
func TestPanNoOpInWrapMode(t *testing.T) {
	lines := []filebuffer.Line{makeLongLine(1, 300)}
	m := setupBrowsePan(t, lines, 1, 3)
	// Toggle back to wrap mode and deliver the layout.
	m, cmd := update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	m, _ = update(t, m, keyPress('.'))
	if m.ViewportHOffset() != 0 {
		t.Fatalf("ViewportHOffset = %d, want 0 (pan no-op in wrap mode)", m.ViewportHOffset())
	}
}

// TestPanLeftAtZeroNoOp verifies that `,` at offset 0 does nothing.
func TestPanLeftAtZeroNoOp(t *testing.T) {
	lines := []filebuffer.Line{makeLongLine(1, 300)}
	m := setupBrowsePan(t, lines, 1, 3)
	m, _ = update(t, m, keyPress(','))
	if m.ViewportHOffset() != 0 {
		t.Fatalf("ViewportHOffset = %d, want 0 (',' at 0 is no-op)", m.ViewportHOffset())
	}
}

// TestOffsetRetainedThroughWrapToggle verifies that w → w retains the
// horizontal offset.
func TestOffsetRetainedThroughWrapToggle(t *testing.T) {
	lines := []filebuffer.Line{makeLongLine(1, 300)}
	m := setupBrowsePan(t, lines, 1, 3)
	// Pan right.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, keyPress('.'))
	}
	if m.ViewportHOffset() != 10 {
		t.Fatalf("ViewportHOffset = %d, want 10 before toggle", m.ViewportHOffset())
	}
	// Toggle to wrap mode and back, delivering the layout each time.
	m, cmd := update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	m, cmd = update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	if m.ViewportHOffset() != 10 {
		t.Fatalf("ViewportHOffset = %d after w→w, want 10 (retained)", m.ViewportHOffset())
	}
}

// TestHorizontalResetOnFileChange verifies that navigating to a
// different file resets the horizontal offset to zero.
func TestHorizontalResetOnFileChange(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
	)
	lines := []filebuffer.Line{makeLongLine(1, 300)}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	// Toggle to run-off-edge mode and deliver the layout.
	m, cmd := update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	// Pan right.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, keyPress('.'))
	}
	if m.ViewportHOffset() != 10 {
		t.Fatalf("ViewportHOffset = %d, want 10 before navigation", m.ViewportHOffset())
	}
	// Navigate to the next file (n).
	m, _ = update(t, m, keyPress('n'))
	// Deliver the load for the new file.
	bBuf := makeBuf([]filebuffer.Line{makeLongLine(1, 300)}, 1, 3)
	m, _ = update(t, m, app.FileLoadCompleteMsg{
		Path:   []byte("src/b.go"),
		Buffer: bBuf,
	})
	if m.ViewportHOffset() != 0 {
		t.Fatalf("ViewportHOffset = %d after file change, want 0 (reset)", m.ViewportHOffset())
	}
}

// TestSplitClusterBlankRender verifies that a half-clipped CJK glyph
// shows a blank rather than a broken glyph in the rendered output.
func TestSplitClusterBlankRender(t *testing.T) {
	lines := []filebuffer.Line{makeCJKLine(1)}
	m := setupBrowsePan(t, lines, 1, 3)
	// Pan right by 1: the CJK glyph (2 cells at position 0-1) is split.
	m, _ = update(t, m, keyPress('.'))
	view := viewContent(m)
	// The CJK character 中 should NOT appear in the content panel
	// because it's split and replaced with a blank.
	// The view includes the filename row and gutter, so check that
	// the content area doesn't contain 中.
	for _, line := range strings.Split(view, "\n") {
		// Skip the filename row and gutter lines.
		if strings.Contains(line, "──") {
			continue
		}
		if strings.Contains(line, "中") {
			t.Fatalf("view contains 中 (split cluster should be blank): %q", line)
		}
	}
}

// TestPanAtMaxNoOp verifies that panning past the maximum does nothing.
func TestPanAtMaxNoOp(t *testing.T) {
	lines := []filebuffer.Line{makeLongLine(1, 100)}
	m := setupBrowsePan(t, lines, 1, 3)
	// Pan far right past the maximum.
	for i := 0; i < 200; i++ {
		m, _ = update(t, m, keyPress('.'))
	}
	max := m.ViewportHOffset()
	// Further panning should not change the offset.
	m, _ = update(t, m, keyPress('.'))
	if m.ViewportHOffset() != max {
		t.Fatalf("ViewportHOffset = %d after pan at max, want %d (no-op)", m.ViewportHOffset(), max)
	}
}
