package app_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// --- Startup reveal after load (Issue #14) ---

// TestStartupRevealAfterLoad verifies that when the startup file loads,
// the first match's rendered row is revealed at the one-third position
// when it is hidden from the top of the file. A first visit (including
// the startup file) starts from the top of the file before applying the
// reveal.
func TestStartupRevealAfterLoad(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	// Height 24 → contentHeight 23, floor(23/3) = 7.
	// Target row = 199 (0-based line 200). Offset = 199 - 7 = 192.
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	if m.ViewportOffset() != 192 {
		t.Fatalf("ViewportOffset = %d, want 192 (startup reveal at one-third)", m.ViewportOffset())
	}
}

// TestStartupRevealVisibleNoScroll verifies that when the startup
// file's first match is visible from the top of the file, the reveal
// does not scroll.
func TestStartupRevealVisibleNoScroll(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight 23
	// Line 5 (row 4) is visible from offset 0, so no scroll.
	if m.ViewportOffset() != 0 {
		t.Fatalf("ViewportOffset = %d, want 0 (startup reveal visible no-scroll)", m.ViewportOffset())
	}
}

// TestStartupRevealBOFClamp verifies that a startup first match near
// the top of the file clamps to offset 0 rather than placing it at the
// one-third row. BOF content takes precedence over one-third placement.
func TestStartupRevealBOFClamp(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	// Line 1 (row 0) is visible from offset 0, so no scroll.
	if m.ViewportOffset() != 0 {
		t.Fatalf("ViewportOffset = %d, want 0 (startup BOF clamp)", m.ViewportOffset())
	}
}

// TestStartupRevealEOFClamp verifies that a startup first match near
// the bottom of a short file clamps to maxOffset rather than placing
// it at the one-third row. EOF content takes precedence over one-third
// placement.
func TestStartupRevealEOFClamp(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 18, subSpec{"x", 0, 1}),
	)
	// 20 lines, height 10 → contentHeight 9, maxOffset 11.
	// Target row 17 (line 18). floor(9/3) = 3. Offset = 17 - 3 = 14,
	// but maxOffset = 11, so clamped to 11.
	lines := makeScrollLines(20)
	buf := makeBuf(lines, 20, 2)
	m := setupBrowseWithSize(t, idx, buf, 80, 10)
	if m.ViewportOffset() != 11 {
		t.Fatalf("ViewportOffset = %d, want 11 (startup EOF clamp)", m.ViewportOffset())
	}
}

// --- Same-file navigation reveal (Issue #14) ---

// TestSameFileNavigationReveal verifies that n within the same file
// reveals the new match's rendered row at the one-third position when
// it is hidden.
func TestSameFileNavigationReveal(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight 23, floor(23/3) = 7
	// Startup reveals line 1 (row 0), visible from offset 0, no scroll.
	if m.ViewportOffset() != 0 {
		t.Fatalf("startup ViewportOffset = %d, want 0", m.ViewportOffset())
	}
	// n to line 200 (row 199). Reveal: offset = 199 - 7 = 192.
	m, _ = update(t, m, keyPress('n'))
	if m.ViewportOffset() != 192 {
		t.Fatalf("after n ViewportOffset = %d, want 192 (same-file reveal)", m.ViewportOffset())
	}
}

// TestSameFileNavigationRevealBack verifies that p within the same file
// reveals the previous match's rendered row.
func TestSameFileNavigationRevealBack(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight 23, floor(23/3) = 7
	// Startup at line 1 (row 0), offset 0.
	// n to line 200 (row 199), offset 192.
	m, _ = update(t, m, keyPress('n'))
	if m.ViewportOffset() != 192 {
		t.Fatalf("after n ViewportOffset = %d, want 192", m.ViewportOffset())
	}
	// p back to line 1 (row 0). Reveal: offset = 0 - 7 = -7, clamped to 0.
	m, _ = update(t, m, keyPress('p'))
	if m.ViewportOffset() != 0 {
		t.Fatalf("after p ViewportOffset = %d, want 0 (same-file reveal back, BOF clamp)", m.ViewportOffset())
	}
}

// TestSameFileNavigationVisibleNoScroll verifies that n to an
// already-visible target row in the same file does not scroll.
func TestSameFileNavigationVisibleNoScroll(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 10, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight 23
	// Startup at line 5 (row 4), visible from offset 0, no scroll.
	if m.ViewportOffset() != 0 {
		t.Fatalf("startup ViewportOffset = %d, want 0", m.ViewportOffset())
	}
	// n to line 10 (row 9), still visible from offset 0 (range [0, 23)).
	m, _ = update(t, m, keyPress('n'))
	if m.ViewportOffset() != 0 {
		t.Fatalf("after n ViewportOffset = %d, want 0 (visible target no-scroll)", m.ViewportOffset())
	}
}

// --- Cross-file navigation reveal (Issue #14) ---

// TestCrossFileNavigationRevealCached verifies that n to a cached file
// reveals the destination match at the one-third position when hidden.
// A cached destination is shown immediately with its saved viewport
// restored, then destination reveal is applied.
func TestCrossFileNavigationRevealCached(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(300)
	linesB := makeScrollLines(300)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(linesA, 300, 4),
		"src/b.go": makeBuf(linesB, 300, 4),
	}
	m := setupBrowseMulti(t, idx, bufs)
	// Startup at a.go line 1 (row 0), offset 0.
	if m.ViewportOffset() != 0 {
		t.Fatalf("startup ViewportOffset = %d, want 0", m.ViewportOffset())
	}
	// n to b.go line 200 (row 199). First visit: offset 0, then reveal.
	// Reveal: offset = 199 - 7 = 192.
	m, cmd := update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)
	if m.ViewportOffset() != 192 {
		t.Fatalf("after n to b.go ViewportOffset = %d, want 192 (cross-file reveal)", m.ViewportOffset())
	}
}

// TestCrossFileNavigationRevealUncached verifies that n to an uncached
// file reveals the destination match after the load completes.
func TestCrossFileNavigationRevealUncached(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	// Use a per-file blocking channel in the loader (not a file gate,
	// which would block the startup a.go load too).
	bGate := make(chan struct{})
	linesB := makeScrollLines(300)
	bufB := makeBuf(linesB, 300, 4)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if string(path) == "src/b.go" {
			<-bGate
			return bufB, nil
		}
		return makeBuf(makeScrollLines(300), 300, 4), nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoader(loader),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	// Deliver startup load (a.go) — completes immediately.
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}
	// n to b.go (uncached). Load command is returned but not yet executed.
	m, cmd = update(t, m, keyPress('n'))
	// Release the per-file gate, then deliver the load completion.
	close(bGate)
	m = deliverLoad(t, m, cmd)
	// b.go line 200 (row 199). First visit, reveal: offset = 199 - 7 = 192.
	if m.ViewportOffset() != 192 {
		t.Fatalf("after b.go load ViewportOffset = %d, want 192 (uncached reveal)", m.ViewportOffset())
	}
}

// --- Reveal effect on saved state (Issue #14) ---

// TestRevealMovesReplacesSavedState verifies that a reveal that moves
// the viewport replaces the saved per-file vertical state. A match
// reveal that moves the vertical viewport replaces the logical
// anchor; the new offset is saved for revisits.
func TestRevealMovesReplacesSavedState(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(300)
	linesB := makeScrollLines(300)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(linesA, 300, 4),
		"src/b.go": makeBuf(linesB, 300, 4),
	}
	m := setupBrowseMulti(t, idx, bufs)
	// n to b.go line 200 (row 199). First visit, reveal: offset = 192.
	m, cmd := update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)
	if m.ViewportOffset() != 192 {
		t.Fatalf("after n to b.go ViewportOffset = %d, want 192", m.ViewportOffset())
	}
	// The reveal moved the viewport, so the new offset should be saved.
	if m.SavedOffset([]byte("src/b.go")) != 192 {
		t.Fatalf("SavedOffset(b.go) = %d, want 192 (reveal moved, saved state replaced)", m.SavedOffset([]byte("src/b.go")))
	}
}

// TestRevealNoScrollLeavesSavedState verifies that a no-scroll reveal
// (target already visible) leaves the saved per-file vertical state
// unchanged. A no-scroll reveal does not discard the retained state.
func TestRevealNoScrollLeavesSavedState(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 10, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(300)
	linesB := makeScrollLines(300)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(linesA, 300, 4),
		"src/b.go": makeBuf(linesB, 300, 4),
	}
	m := setupBrowseMulti(t, idx, bufs)
	// Manually scroll a.go down 5 rows and save.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.ViewportOffset() != 5 {
		t.Fatalf("after scroll ViewportOffset = %d, want 5", m.ViewportOffset())
	}
	if m.SavedOffset([]byte("src/a.go")) != 5 {
		t.Fatalf("SavedOffset(a.go) = %d, want 5", m.SavedOffset([]byte("src/a.go")))
	}
	// n to a.go line 10 (row 9). From offset 5, visible range is [5, 28).
	// Row 9 is visible, so no-scroll reveal. Saved state (5) is preserved.
	m, _ = update(t, m, keyPress('n'))
	if m.ViewportOffset() != 5 {
		t.Fatalf("after n (no-scroll) ViewportOffset = %d, want 5 (no-scroll)", m.ViewportOffset())
	}
	if m.SavedOffset([]byte("src/a.go")) != 5 {
		t.Fatalf("SavedOffset(a.go) = %d, want 5 (no-scroll reveal preserves saved state)", m.SavedOffset([]byte("src/a.go")))
	}
}

// TestRevealSavedViewportStartingPoint verifies that a revisit starts
// from the saved per-file viewport before applying the reveal. If the
// target is visible from the saved offset, no scroll occurs.
func TestRevealSavedViewportStartingPoint(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(300)
	linesB := makeScrollLines(300)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(linesA, 300, 4),
		"src/b.go": makeBuf(linesB, 300, 4),
	}
	m := setupBrowseMulti(t, idx, bufs)
	// Scroll a.go down 10 rows and save.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.ViewportOffset() != 10 {
		t.Fatalf("after scroll ViewportOffset = %d, want 10", m.ViewportOffset())
	}
	// Navigate to b.go (saves a.go offset = 10).
	m, cmd := update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)
	// b.go is a first visit, starts at offset 0.
	if m.ViewportOffset() != 0 {
		t.Fatalf("b.go ViewportOffset = %d, want 0 (first visit)", m.ViewportOffset())
	}
	// Navigate back to a.go (p wraps). a.go is cached, saved offset 10.
	// Line 1 (row 0) is hidden from offset 10. Reveal: offset = 0 - 7 = -7,
	// clamped to 0. The reveal moves, so saved state is replaced.
	m, _ = update(t, m, keyPress('p'))
	if m.ViewportOffset() != 0 {
		t.Fatalf("a.go revisit ViewportOffset = %d, want 0 (reveal from saved offset)", m.ViewportOffset())
	}
}

// TestRevealFirstVisitStartsAtTop verifies that a first visit to a
// file starts from the top of the file (offset 0) before applying the
// reveal.
func TestRevealFirstVisitStartsAtTop(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 5, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(300)
	linesB := makeScrollLines(300)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(linesA, 300, 4),
		"src/b.go": makeBuf(linesB, 300, 4),
	}
	m := setupBrowseMulti(t, idx, bufs)
	// n to b.go (first visit). Line 5 (row 4) is visible from offset 0.
	// No scroll needed.
	m, cmd := update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)
	if m.ViewportOffset() != 0 {
		t.Fatalf("b.go first visit ViewportOffset = %d, want 0 (first visit starts at top, visible no-scroll)", m.ViewportOffset())
	}
}

// TestRevealIdentifiesFirstSubmatch verifies that the reveal targets
// the rendered row containing the first submatch's start cell, not
// merely a source-line ordinal. With multiple submatches on the same
// line, the first (by byte start) identifies the target row. Without
// wrapping (Issue #16 pending), the rendered row is the source line's
// 0-based index, so this test confirms the reveal uses the line of
// the first submatch.
func TestRevealIdentifiesFirstSubmatch(t *testing.T) {
	// Two submatches on line 200: first at byte 0, second at byte 10.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x and x again\n", 200,
			subSpec{"x", 0, 1},
			subSpec{"x", 10, 11},
		),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight 23, floor(23/3) = 7
	// The display target is the start cell of the first submatch (byte 0)
	// on line 200. The rendered row is 199 (0-based). Reveal: offset = 192.
	if m.ViewportOffset() != 192 {
		t.Fatalf("ViewportOffset = %d, want 192 (first submatch row reveal)", m.ViewportOffset())
	}
}
