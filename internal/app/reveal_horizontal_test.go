package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// makeRevealBufLine creates a filebuffer.Line with ByteCells populated
// from the shared safe-presentation policy, so the app can map submatch
// byte offsets to display cells (Issue #19). The raw content is escaped
// through EscapeContent; for ASCII content the display text equals the
// raw text and each byte maps to one display cell.
func makeRevealBufLine(num int, raw string) filebuffer.Line {
	d := safepresentation.EscapeContent([]byte(raw))
	return filebuffer.Line{
		Number:    num,
		Display:   d.Text,
		ByteCells: d.ByteCells,
		Clusters:  safepresentation.GraphemeClusters(d.Text),
	}
}

// makeRevealLines creates n lines of the given ASCII content, each with
// ByteCells populated (Issue #19).
func makeRevealLines(n int, content string) []filebuffer.Line {
	lines := make([]filebuffer.Line, n)
	for i := 0; i < n; i++ {
		lines[i] = makeRevealBufLine(i+1, content)
	}
	return lines
}

// setupBrowseRunOffEdge creates a browse model in run-off-edge mode
// (Issue #19). The terminal is 80x24; the text width in run-off-edge
// mode is m.width - gutterWidth - 1 (reserved indicator). The file load
// completes immediately.
func setupBrowseRunOffEdge(t *testing.T, idx *searchindex.Index, buf *filebuffer.Buffer) app.Model {
	t.Helper()
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithWrapMode(viewport.WrapOff),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	return m
}

// textWidthAt80 returns the text width in run-off-edge mode at 80
// columns with the given gutter width: 80 - gutterWidth - 1 (reserved
// indicator). This matches the app's buildViewport calculation which
// uses m.width (terminal width) minus gutter and reserved indicator.
func textWidthAt80(gutterWidth int) int {
	return 80 - gutterWidth - 1
}

// --- Startup trigger (Issue #19) ---

// TestStartupHorizontalRevealRunOffEdge verifies that the startup
// reveal includes horizontal reveal in run-off-edge mode. A match at
// a far column is horizontally revealed by the minimum movement
// (right-edge arithmetic) after the file loads.
func TestStartupHorizontalRevealRunOffEdge(t *testing.T) {
	// Line 1: 300 ASCII chars. Match at byte 250 (display cell 250).
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 250, 251}),
	)
	line := makeRevealBufLine(1, lineText)
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	m := setupBrowseRunOffEdge(t, idx, buf)
	tw := textWidthAt80(3) // 76
	want := 250 + 1 - tw   // right-edge arithmetic
	if m.ViewportHOffset() != want {
		t.Fatalf("ViewportHOffset = %d, want %d (startup horizontal reveal in run-off-edge mode)", m.ViewportHOffset(), want)
	}
}

// TestStartupHorizontalRevealWrapModeNoOp verifies that the startup
// horizontal reveal is a no-op in wrap mode (the default). The
// horizontal offset stays at zero.
func TestStartupHorizontalRevealWrapModeNoOp(t *testing.T) {
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 250, 251}),
	)
	line := makeRevealBufLine(1, lineText)
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	// Use the default wrap mode (WrapOn) via setupBrowseWithSize.
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	if m.ViewportHOffset() != 0 {
		t.Fatalf("ViewportHOffset = %d, want 0 (startup horizontal reveal no-op in wrap mode)", m.ViewportHOffset())
	}
}

// --- Same-file navigation trigger (Issue #19) ---

// TestSameFileNavigationHorizontalReveal verifies that n within the
// same file triggers a horizontal reveal in run-off-edge mode. A
// visible first match does not move the offset; navigating to a far
// match on another line reveals it at the right edge.
func TestSameFileNavigationHorizontalReveal(t *testing.T) {
	// Line 1: match at byte 5 (cell 5). Line 2: match at byte 250 (cell 250).
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 5, 6}),
		textMatch("src/a.go", lineText+"\n", 2, subSpec{"a", 250, 251}),
	)
	lines := makeRevealLines(2, lineText)
	buf := makeBuf(lines, 2, 3)
	m := setupBrowseRunOffEdge(t, idx, buf)
	tw := textWidthAt80(3) // 76
	// Startup: first match at cell 5, within [0, 76) → no movement.
	if m.ViewportHOffset() != 0 {
		t.Fatalf("startup ViewportHOffset = %d, want 0 (first match visible)", m.ViewportHOffset())
	}
	// n to second match at cell 250. Right-edge: offset = 250 + 1 - 76 = 175.
	m, _ = update(t, m, keyPress('n'))
	want := 250 + 1 - tw
	if m.ViewportHOffset() != want {
		t.Fatalf("after n ViewportHOffset = %d, want %d (same-file horizontal reveal)", m.ViewportHOffset(), want)
	}
}

// TestSameFileNavigationHorizontalRevealBack verifies that p within
// the same file triggers a horizontal reveal in run-off-edge mode.
// Navigating back to a left-of-view match reveals it at the left edge.
func TestSameFileNavigationHorizontalRevealBack(t *testing.T) {
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 5, 6}),
		textMatch("src/a.go", lineText+"\n", 2, subSpec{"a", 250, 251}),
	)
	lines := makeRevealLines(2, lineText)
	buf := makeBuf(lines, 2, 3)
	m := setupBrowseRunOffEdge(t, idx, buf)
	// Startup: first match at cell 5, offset 0.
	// n to second match at cell 250, offset 175.
	m, _ = update(t, m, keyPress('n'))
	// p back to first match at cell 5. Cell 5 is left of view (5 < 175).
	// Left-side reveal: offset = 5.
	m, _ = update(t, m, keyPress('p'))
	if m.ViewportHOffset() != 5 {
		t.Fatalf("after p ViewportHOffset = %d, want 5 (same-file horizontal reveal back, left edge)", m.ViewportHOffset())
	}
}

// TestSameFileNavigationHorizontalVisibleNoMove verifies that n to an
// already-painted target in the same file does not move the horizontal
// offset.
func TestSameFileNavigationHorizontalVisibleNoMove(t *testing.T) {
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 5, 6}),
		textMatch("src/a.go", lineText+"\n", 2, subSpec{"a", 10, 11}),
	)
	lines := makeRevealLines(2, lineText)
	buf := makeBuf(lines, 2, 3)
	m := setupBrowseRunOffEdge(t, idx, buf)
	// Startup: first match at cell 5, offset 0.
	// n to second match at cell 10. Cell 10 is within [0, 76) → painted → no move.
	m, _ = update(t, m, keyPress('n'))
	if m.ViewportHOffset() != 0 {
		t.Fatalf("after n ViewportHOffset = %d, want 0 (visible target no horizontal move)", m.ViewportHOffset())
	}
}

// --- Every navigation action (Issue #19) ---

// TestEveryNavigationTriggersHorizontalReveal verifies that every n/p
// navigation action triggers a horizontal reveal in run-off-edge mode.
// Three matches at different columns on different lines exercise n, n,
// p, p.
func TestEveryNavigationTriggersHorizontalReveal(t *testing.T) {
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 5, 6}),
		textMatch("src/a.go", lineText+"\n", 2, subSpec{"a", 250, 251}),
		textMatch("src/a.go", lineText+"\n", 3, subSpec{"a", 100, 101}),
	)
	lines := makeRevealLines(3, lineText)
	buf := makeBuf(lines, 3, 3)
	m := setupBrowseRunOffEdge(t, idx, buf)
	tw := textWidthAt80(3) // 76
	// Startup: match 1 at cell 5, offset 0 (visible).
	if m.ViewportHOffset() != 0 {
		t.Fatalf("startup ViewportHOffset = %d, want 0", m.ViewportHOffset())
	}
	// n to match 2 at cell 250. Right-edge: offset = 250 + 1 - 76 = 175.
	m, _ = update(t, m, keyPress('n'))
	if m.ViewportHOffset() != 250+1-tw {
		t.Fatalf("after n to match 2 ViewportHOffset = %d, want %d", m.ViewportHOffset(), 250+1-tw)
	}
	// n to match 3 at cell 100. Cell 100 is left of view (100 < 175).
	// Left-side reveal: offset = 100.
	m, _ = update(t, m, keyPress('n'))
	if m.ViewportHOffset() != 100 {
		t.Fatalf("after n to match 3 ViewportHOffset = %d, want 100 (left-side reveal)", m.ViewportHOffset())
	}
	// p to match 2 at cell 250. Cell 250 is right of view (250 > 100 + 76 = 176).
	// Right-edge: offset = 250 + 1 - 76 = 175.
	m, _ = update(t, m, keyPress('p'))
	if m.ViewportHOffset() != 250+1-tw {
		t.Fatalf("after p to match 2 ViewportHOffset = %d, want %d (right-side reveal)", m.ViewportHOffset(), 250+1-tw)
	}
	// p to match 1 at cell 5. Cell 5 is left of view (5 < 175).
	// Left-side reveal: offset = 5.
	m, _ = update(t, m, keyPress('p'))
	if m.ViewportHOffset() != 5 {
		t.Fatalf("after p to match 1 ViewportHOffset = %d, want 5 (left-side reveal)", m.ViewportHOffset())
	}
}

// --- File-change reset ordering (Issue #19) ---

// TestFileChangeResetThenHorizontalReveal verifies that cross-file
// navigation resets the horizontal offset to zero and then applies the
// horizontal reveal. The reset happens before the reveal, so the new
// file's match is revealed from offset zero.
func TestFileChangeResetThenHorizontalReveal(t *testing.T) {
	// File A: match at cell 250. File B: match at cell 250.
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 250, 251}),
		textMatch("src/b.go", lineText+"\n", 1, subSpec{"a", 250, 251}),
	)
	line := makeRevealBufLine(1, lineText)
	bufA := makeBuf([]filebuffer.Line{line}, 1, 3)
	bufB := makeBuf([]filebuffer.Line{line}, 1, 3)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": bufA,
		"src/b.go": bufB,
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if buf, ok := bufs[string(path)]; ok {
			return buf, nil
		}
		return &filebuffer.Buffer{}, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithWrapMode(viewport.WrapOff),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	tw := textWidthAt80(3) // 76
	// Startup: a.go match at cell 250. Right-edge: offset = 175.
	want := 250 + 1 - tw
	if m.ViewportHOffset() != want {
		t.Fatalf("startup ViewportHOffset = %d, want %d (a.go reveal)", m.ViewportHOffset(), want)
	}
	// n to b.go. Reset to 0, then reveal cell 250: offset = 175.
	m, cmd = update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)
	if m.ViewportHOffset() != want {
		t.Fatalf("after n to b.go ViewportHOffset = %d, want %d (reset then reveal)", m.ViewportHOffset(), want)
	}
}

// TestFileChangeResetOrdering verifies that the horizontal offset is
// reset to zero before the reveal is applied on file change. A first
// file with a far match sets a non-zero offset; the second file's
// near match is visible from offset zero, so the reveal is a no-op
// and the offset stays at zero (not the carried-over value).
func TestFileChangeResetOrdering(t *testing.T) {
	// File A: match at cell 250 (far). File B: match at cell 5 (near).
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 250, 251}),
		textMatch("src/b.go", lineText+"\n", 1, subSpec{"a", 5, 6}),
	)
	line := makeRevealBufLine(1, lineText)
	bufA := makeBuf([]filebuffer.Line{line}, 1, 3)
	bufB := makeBuf([]filebuffer.Line{line}, 1, 3)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": bufA,
		"src/b.go": bufB,
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if buf, ok := bufs[string(path)]; ok {
			return buf, nil
		}
		return &filebuffer.Buffer{}, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithWrapMode(viewport.WrapOff),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	tw := textWidthAt80(3) // 76
	// Startup: a.go match at cell 250. offset = 175.
	if m.ViewportHOffset() != 250+1-tw {
		t.Fatalf("startup ViewportHOffset = %d, want %d (a.go far match)", m.ViewportHOffset(), 250+1-tw)
	}
	// n to b.go. Reset to 0, then reveal cell 5 (within [0, 76) → no move).
	// Final offset: 0 (reset + no-op reveal). Without reset: 175 (carried
	// over), then reveal cell 5 (left of view) → offset 5.
	m, cmd = update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)
	if m.ViewportHOffset() != 0 {
		t.Fatalf("after n to b.go ViewportHOffset = %d, want 0 (reset then no-op reveal)", m.ViewportHOffset())
	}
}
