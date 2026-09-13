package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
)

// --- Cursor preservation on resize (Issue #17) ---

// TestCursorPreservedOnResize verifies that any resize preserves the
// cursor selection (the matched-line navigation cursor). After
// navigating to the second stop, a resize must not move the cursor.
func TestCursorPreservedOnResize(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello world")}, 1, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)

	// Navigate to the second stop (file b.go).
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() != 1 {
		t.Fatalf("after n, CursorPosition = %d, want 1", m.CursorPosition())
	}

	// Resize: cursor must be preserved.
	m = resize(t, m, 100, 30)
	if m.CursorPosition() != 1 {
		t.Fatalf("after resize, CursorPosition = %d, want 1 (preserved)", m.CursorPosition())
	}
	if p := m.CurrentPath(); p == nil || string(p) != "src/b.go" {
		t.Fatalf("after resize, CurrentPath = %q, want src/b.go (preserved)", p)
	}
}

// TestCursorPreservedOnShrinkResize verifies that shrinking and
// re-widening the terminal preserves the cursor selection.
func TestCursorPreservedOnShrinkResize(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
		textMatch("src/c.go", "foo\n", 1, subSpec{"foo", 0, 3}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello world")}, 1, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)

	// Navigate to the third stop.
	m, _ = update(t, m, keyPress('n'))
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() != 2 {
		t.Fatalf("after 2x n, CursorPosition = %d, want 2", m.CursorPosition())
	}

	// Shrink to minimum.
	m = resize(t, m, 20, 3)
	if m.CursorPosition() != 2 {
		t.Fatalf("after shrink, CursorPosition = %d, want 2 (preserved)", m.CursorPosition())
	}

	// Widen back.
	m = resize(t, m, 120, 40)
	if m.CursorPosition() != 2 {
		t.Fatalf("after widen, CursorPosition = %d, want 2 (preserved)", m.CursorPosition())
	}
	if p := m.CurrentPath(); p == nil || string(p) != "src/c.go" {
		t.Fatalf("after widen, CurrentPath = %q, want src/c.go (preserved)", p)
	}
}

// TestViewportAnchorOnResize verifies that a resize which changes the
// text width recomputes the viewport top from the logical anchor, not
// the old row ordinal. A 500-char line at a narrow width produces many
// rows; scrolling partway in and then widening must land the top on the
// anchor's text location in the new row model, not the old ordinal.
func TestViewportAnchorOnResize(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", strings.Repeat("x", 500)+"\n", 1, subSpec{"x", 0, 1}),
	)
	// 500-char line. At terminal width 30, fileListWidth(30)=20, panel
	// width = 30-20-1 = 9, gutter 4 → text width 5 (wrap mode).
	// 500 chars at width 5 = 100 rows. contentHeight = 9, maxOffset = 91.
	display := strings.Repeat("x", 500)
	line := ml(1, display)
	buf := makeBuf([]filebuffer.Line{line}, 1, 4)
	m := setupBrowseWithSize(t, idx, buf, 30, 10)

	// Scroll down 5 rows. At width 5, row 5 covers cells 25-29 of line 0.
	// The anchor should be (0, 25).
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, keyPress(tea.KeyDown))
	}
	if m.ViewportOffset() != 5 {
		t.Fatalf("after 5 down, ViewportOffset = %d, want 5", m.ViewportOffset())
	}

	// Widen to terminal width 50. fileListWidth(50)=20, panel width = 29,
	// gutter 4 → text width 25 (wrap mode). 500 chars at width 25 = 20 rows.
	// contentHeight = 9, maxOffset = 11. Old ordinal 5 is valid (5 <= 11).
	// But the anchor (0, 25) maps to row 1 (cells 25-49) at width 25.
	// The offset should be 1, not 5 (the old ordinal).
	m = resize(t, m, 50, 10)
	if m.ViewportOffset() != 1 {
		t.Fatalf("after widen, ViewportOffset = %d, want 1 (anchor (0, 25) → row 1 at width 25, not old ordinal 5)", m.ViewportOffset())
	}
}
