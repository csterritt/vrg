package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
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
// mode is the panel width minus gutter minus one reserved indicator
// cell, where the panel width is the terminal width minus the list
// width minus the one-cell separator (Issue #38). The file load
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

// textWidthFor returns the expected text width for the model's current
// layout state (Issue #38): the terminal width minus the actual file
// list width, minus the one-cell separator, minus the buffer gutter,
// minus the reserved right-indicator width for the current wrap mode
// (one cell in run-off-edge, zero in wrap). This mirrors the LayoutKey
// TextWidth chain — terminal width, panel width, text width — so a
// regression that installs the viewport from the raw terminal width,
// or that omits the list width, separator, gutter, or indicator
// reservation, fails the dependent assertions.
func textWidthFor(t *testing.T, m app.Model, termWidth, gutterWidth int) int {
	t.Helper()
	tw := termWidth - m.ListWidth() - 1 - gutterWidth - viewport.ReservedWidth(m.WrapMode())
	if tw < 1 {
		tw = 1
	}
	return tw
}

// --- Startup trigger (Issue #19) ---

// TestStartupHorizontalRevealRunOffEdge verifies that the startup
// reveal includes horizontal reveal in run-off-edge mode. A match at
// a far column is horizontally revealed by the minimum movement
// (right-edge arithmetic) after the file loads, measured against the
// text width (Issue #38).
func TestStartupHorizontalRevealRunOffEdge(t *testing.T) {
	// Line 1: 300 ASCII chars. Match at byte 250 (display cell 250).
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 250, 251}),
	)
	line := makeRevealBufLine(1, lineText)
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	m := setupBrowseRunOffEdge(t, idx, buf)
	tw := textWidthFor(t, m, 80, 3)
	want := 250 + 1 - tw // right-edge arithmetic
	if m.ViewportHOffset() != want {
		t.Fatalf("ViewportHOffset = %d, want %d (startup horizontal reveal in run-off-edge mode, text width %d)", m.ViewportHOffset(), want, tw)
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
// match on another line reveals it at the right edge of the text
// width (Issue #38).
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
	tw := textWidthFor(t, m, 80, 3)
	// Startup: first match at cell 5, within [0, tw) → no movement.
	if m.ViewportHOffset() != 0 {
		t.Fatalf("startup ViewportHOffset = %d, want 0 (first match visible)", m.ViewportHOffset())
	}
	// n to second match at cell 250. Right-edge: offset = 250 + 1 - tw.
	m, _ = update(t, m, keyPress('n'))
	want := 250 + 1 - tw
	if m.ViewportHOffset() != want {
		t.Fatalf("after n ViewportHOffset = %d, want %d (same-file horizontal reveal, text width %d)", m.ViewportHOffset(), want, tw)
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
	// n to second match at cell 250, offset 250 + 1 - text width.
	m, _ = update(t, m, keyPress('n'))
	// p back to first match at cell 5. Cell 5 is left of view.
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
	// n to second match at cell 10. Cell 10 is within [0, tw) → painted → no move.
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
	tw := textWidthFor(t, m, 80, 3)
	// Startup: match 1 at cell 5, offset 0 (visible).
	if m.ViewportHOffset() != 0 {
		t.Fatalf("startup ViewportHOffset = %d, want 0", m.ViewportHOffset())
	}
	// n to match 2 at cell 250. Right-edge: offset = 250 + 1 - tw.
	m, _ = update(t, m, keyPress('n'))
	if m.ViewportHOffset() != 250+1-tw {
		t.Fatalf("after n to match 2 ViewportHOffset = %d, want %d", m.ViewportHOffset(), 250+1-tw)
	}
	// n to match 3 at cell 100. Cell 100 is left of view (100 < offset).
	// Left-side reveal: offset = 100.
	m, _ = update(t, m, keyPress('n'))
	if m.ViewportHOffset() != 100 {
		t.Fatalf("after n to match 3 ViewportHOffset = %d, want 100 (left-side reveal)", m.ViewportHOffset())
	}
	// p to match 2 at cell 250. Cell 250 is right of view
	// (250 > 100 + tw). Right-edge: offset = 250 + 1 - tw.
	m, _ = update(t, m, keyPress('p'))
	if m.ViewportHOffset() != 250+1-tw {
		t.Fatalf("after p to match 2 ViewportHOffset = %d, want %d (right-side reveal)", m.ViewportHOffset(), 250+1-tw)
	}
	// p to match 1 at cell 5. Cell 5 is left of view (5 < offset).
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
	tw := textWidthFor(t, m, 80, 3)
	// Startup: a.go match at cell 250. Right-edge: offset = 250 + 1 - tw.
	want := 250 + 1 - tw
	if m.ViewportHOffset() != want {
		t.Fatalf("startup ViewportHOffset = %d, want %d (a.go reveal)", m.ViewportHOffset(), want)
	}
	// n to b.go. Reset to 0, then reveal cell 250: offset = 250 + 1 - tw.
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
	tw := textWidthFor(t, m, 80, 3)
	// Startup: a.go match at cell 250. offset = 250 + 1 - tw.
	if m.ViewportHOffset() != 250+1-tw {
		t.Fatalf("startup ViewportHOffset = %d, want %d (a.go far match)", m.ViewportHOffset(), 250+1-tw)
	}
	// n to b.go. Reset to 0, then reveal cell 5 (within [0, tw) → no move).
	// Final offset: 0 (reset + no-op reveal). Without reset: the carried
	// over offset, then reveal cell 5 (left of view) → offset 5.
	m, cmd = update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)
	if m.ViewportHOffset() != 0 {
		t.Fatalf("after n to b.go ViewportHOffset = %d, want 0 (reset then no-op reveal)", m.ViewportHOffset())
	}
}

// --- Issue #38: panel-width-derived text width at every install site ---

// TestListHiddenHorizontalReveal verifies that with the file list
// hidden, the horizontal reveal is measured against the wider panel
// width — terminal width minus zero list width minus the separator,
// gutter, and reserved indicator — not the raw terminal width (Issue
// #38). The panel width follows the current layout state.
func TestListHiddenHorizontalReveal(t *testing.T) {
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 5, 6}),
		textMatch("src/a.go", lineText+"\n", 2, subSpec{"a", 250, 251}),
	)
	lines := makeRevealLines(2, lineText)
	buf := makeBuf(lines, 2, 3)
	m := setupBrowseRunOffEdge(t, idx, buf)
	// Startup: first match at cell 5 visible, offset 0.
	if m.ViewportHOffset() != 0 {
		t.Fatalf("startup ViewportHOffset = %d, want 0", m.ViewportHOffset())
	}
	// Hide the file list; the layout is rebuilt at the wider panel.
	m, cmd := update(t, m, leftKey())
	m = deliverLayout(t, m, cmd)
	if m.ListWidth() != 0 {
		t.Fatalf("after left, ListWidth = %d, want 0 (list hidden)", m.ListWidth())
	}
	tw := textWidthFor(t, m, 80, 3)
	if m.LayoutKey().TextWidth != tw {
		t.Fatalf("after list hide, LayoutKey.TextWidth = %d, want %d (terminal − separator − gutter − indicator)", m.LayoutKey().TextWidth, tw)
	}
	// n to the far match at cell 250. Right-edge: offset = 250 + 1 - tw.
	m, _ = update(t, m, keyPress('n'))
	want := 250 + 1 - tw
	if m.ViewportHOffset() != want {
		t.Fatalf("after n with list hidden ViewportHOffset = %d, want %d (reveal at hidden-list text width %d)", m.ViewportHOffset(), want, tw)
	}
}

// TestWrapModeZeroReservedIndicator verifies that wrap mode reserves
// no right-indicator column: the layout key's text width is the panel
// width minus the gutter only (Issue #38). Toggling to run-off-edge
// re-measures with the one-cell reservation, and horizontal behaviour
// follows the run-off-edge text width.
func TestWrapModeZeroReservedIndicator(t *testing.T) {
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 5, 6}),
		textMatch("src/a.go", lineText+"\n", 2, subSpec{"a", 250, 251}),
	)
	lines := makeRevealLines(2, lineText)
	buf := makeBuf(lines, 2, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	// Wrap mode: zero reserved indicator width.
	wrapTW := textWidthFor(t, m, 80, 3)
	if got := viewport.ReservedWidth(m.WrapMode()); got != 0 {
		t.Fatalf("ReservedWidth(wrap) = %d, want 0", got)
	}
	if m.LayoutKey().TextWidth != wrapTW {
		t.Fatalf("wrap mode LayoutKey.TextWidth = %d, want %d (terminal − list − separator − gutter, no indicator)", m.LayoutKey().TextWidth, wrapTW)
	}
	// Toggle to run-off-edge: the reserved column appears and the
	// horizontal reveal is measured against the narrower text width.
	m, cmd := update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	tw := textWidthFor(t, m, 80, 3)
	if tw != wrapTW-1 {
		t.Fatalf("run-off-edge text width = %d, want %d (wrap width minus reserved indicator)", tw, wrapTW-1)
	}
	// Startup match at cell 5 stays visible. n to cell 250:
	// right-edge reveal offset = 250 + 1 - tw.
	m, _ = update(t, m, keyPress('n'))
	want := 250 + 1 - tw
	if m.ViewportHOffset() != want {
		t.Fatalf("after w then n ViewportHOffset = %d, want %d (reveal at run-off-edge text width %d)", m.ViewportHOffset(), want, tw)
	}
}

// TestResizeRemeasuresTextWidth verifies that a terminal resize
// re-measures the viewport against the new panel width: the reveal
// after the resize is computed from the new text width, not the old
// one and not the raw terminal width (Issue #38).
func TestResizeRemeasuresTextWidth(t *testing.T) {
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 5, 6}),
		textMatch("src/a.go", lineText+"\n", 2, subSpec{"a", 250, 251}),
	)
	lines := makeRevealLines(2, lineText)
	buf := makeBuf(lines, 2, 3)
	m := setupBrowseRunOffEdge(t, idx, buf)
	// Resize to 100 columns and deliver the re-layout.
	m = resize(t, m, 100, 24)
	tw := textWidthFor(t, m, 100, 3)
	if m.LayoutKey().TextWidth != tw {
		t.Fatalf("after resize, LayoutKey.TextWidth = %d, want %d (new panel width chain)", m.LayoutKey().TextWidth, tw)
	}
	// n to the far match at cell 250. Right-edge: offset = 250 + 1 - tw.
	m, _ = update(t, m, keyPress('n'))
	want := 250 + 1 - tw
	if m.ViewportHOffset() != want {
		t.Fatalf("after resize + n ViewportHOffset = %d, want %d (reveal at resized text width %d)", m.ViewportHOffset(), want, tw)
	}
}

// TestFactorySeamInstallsLayoutTextWidth verifies that the synchronous
// row-provider-factory install in buildViewport also installs the
// layout key's text width — the panel width minus gutter minus
// reserved indicator — not a terminal-derived width (Issue #38).
func TestFactorySeamInstallsLayoutTextWidth(t *testing.T) {
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 250, 251}),
	)
	line := makeRevealBufLine(1, lineText)
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	factory := func(buf *filebuffer.Buffer) viewport.RowProvider {
		return viewport.BufferRows(buf)
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithRowProviderFactory(factory),
		app.WithWrapMode(viewport.WrapOff),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	tw := textWidthFor(t, m, 80, 3)
	want := 250 + 1 - tw
	if m.ViewportHOffset() != want {
		t.Fatalf("ViewportHOffset = %d, want %d (factory-seam install at text width %d)", m.ViewportHOffset(), want, tw)
	}
}

// TestComposedViewRowsFitTerminal verifies that in run-off-edge mode
// no composed row exceeds the terminal width, and that the reserved
// right-indicator column sits at the panel's right edge outside the
// text area (Issue #38). The composed row is list width + separator +
// gutter + text width + one reserved indicator cell = terminal width.
func TestComposedViewRowsFitTerminal(t *testing.T) {
	lineText := strings.Repeat("a", 300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 1, subSpec{"a", 250, 251}),
	)
	// Line 1: the current match at cell 250 plus a second match at
	// cell 290 so the right hidden-content indicator is drawn. Line
	// 2 has no far-right match, so its reserved cell is blank.
	line1 := makeRevealBufLine(1, lineText)
	line1.Highlights = [][2]int{{250, 251}, {290, 291}}
	line2 := makeRevealBufLine(2, "bb")
	buf := makeBuf([]filebuffer.Line{line1, line2}, 2, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithWrapMode(viewport.WrapOff),
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	listWidth := m.ListWidth()
	tw := textWidthFor(t, m, 80, 3)
	// The panel starts after the list and the one-cell separator.
	// The reserved indicator cell is the last panel cell.
	indicatorCell := listWidth + 1 + 3 + tw
	if indicatorCell != 80-1 {
		t.Fatalf("indicator cell = %d, want %d (panel right edge at terminal width)", indicatorCell, 80-1)
	}
	rows := strings.Split(viewContent(m), "\n")
	for i, row := range rows {
		if w := graphemeCellWidth(row); w > 80 {
			t.Fatalf("row %d is %d cells, exceeds terminal width 80: %q", i, w, row)
		}
	}
	// The current matched line's content row: text cells fill the
	// text area and the reserved cell at the panel's right edge
	// shows the right hidden-content indicator.
	row1 := []rune(rows[1])
	if len(row1) != 80 {
		t.Fatalf("line 1 content row is %d cells, want 80: %q", len(row1), rows[1])
	}
	if row1[indicatorCell] != '*' {
		t.Fatalf("line 1 cell %d = %q, want '*' (right hidden-content indicator at panel edge)", indicatorCell, row1[indicatorCell])
	}
	if row1[indicatorCell-1] != 'a' {
		t.Fatalf("line 1 cell %d = %q, want 'a' (last text-area cell; indicator sits outside the text area)", indicatorCell-1, row1[indicatorCell-1])
	}
	// Line 2's reserved cell is blank: no match is hidden right.
	row2 := []rune(rows[2])
	if len(row2) != 80 {
		t.Fatalf("line 2 content row is %d cells, want 80: %q", len(row2), rows[2])
	}
	if row2[indicatorCell] != ' ' {
		t.Fatalf("line 2 cell %d = %q, want ' ' (reserved column blank, no hidden-right match)", indicatorCell, row2[indicatorCell])
	}
}
