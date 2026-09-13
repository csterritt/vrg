package app_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// countingRows is a RowProvider that records every Rows query for the
// render-cost guard. It proves the App's render path queries the row
// provider only for the visible range, not the full buffer.
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

// makeScrollLines returns n filebuffer.Lines with sequential line
// numbers and distinct display text for scroll testing.
func makeScrollLines(n int) []filebuffer.Line {
	lines := make([]filebuffer.Line, n)
	for i := range lines {
		lines[i] = ml(i+1, fmt.Sprintf("line-%d", i+1))
	}
	return lines
}

// setupBrowseWithSize creates a browse model with the given terminal
// dimensions. The WindowSizeMsg is delivered before the search
// completes so the viewport is created with the correct panel height.
func setupBrowseWithSize(t *testing.T, idx *searchindex.Index, buf *filebuffer.Buffer, width, height int) app.Model {
	t.Helper()
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	// Set terminal size before search completes so the viewport is
	// created with the correct panel height.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	return m
}

// setupBrowseWithSizeLoading creates a browse model with the gate held
// (not closed) and the given terminal dimensions. The file load command
// is returned but not executed.
func setupBrowseWithSizeLoading(t *testing.T, idx *searchindex.Index, width, height int) (app.Model, tea.Cmd) {
	t.Helper()
	gate := make(chan struct{})
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		<-gate
		return &filebuffer.Buffer{}, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	return m, cmd
}

// --- Placeholder no-op tests ---

// TestScrollKeysOnLoadingPlaceholder verifies that scroll keys on a
// "Loading…" placeholder are no-ops: the state stays browse, no error
// occurs, and the view still shows "Loading…".
func TestScrollKeysOnLoadingPlaceholder(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	m, _ := setupBrowseWithSizeLoading(t, idx, 80, 24)
	keys := []tea.KeyPressMsg{
		{Code: tea.KeyDown},
		{Code: tea.KeyUp},
		keyPress('d'),
		keyPress('u'),
		{Code: tea.KeyPgDown},
		{Code: tea.KeyPgUp},
	}
	for _, key := range keys {
		m, _ = update(t, m, key)
		if m.State() != app.StateBrowse {
			t.Fatalf("after scroll key %v, State = %v, want StateBrowse", key, m.State())
		}
	}
	view := viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("View does not contain 'Loading' after scroll keys: %q", view)
	}
}

// --- Per-file viewport state tests ---

// TestPerFileViewportStateSaved verifies that scrolling saves the
// vertical offset as per-file state in the model. The saved offset is
// retrievable via SavedOffset for the current file's path.
func TestPerFileViewportStateSaved(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(50)
	buf := makeBuf(lines, 50, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight = 23

	// Scroll down 3 rows.
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}

	saved := m.SavedOffset([]byte("src/a.go"))
	if saved != 3 {
		t.Fatalf("SavedOffset = %d, want 3", saved)
	}
}

// TestPerFileViewportStateRestoredOnReload verifies that the saved
// per-file offset is restored when the same file is loaded again.
func TestPerFileViewportStateRestoredOnReload(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(50)
	buf := makeBuf(lines, 50, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight = 23

	// Scroll down 5 rows.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}

	// Verify the offset was saved.
	if m.SavedOffset([]byte("src/a.go")) != 5 {
		t.Fatalf("SavedOffset = %d, want 5", m.SavedOffset([]byte("src/a.go")))
	}

	// Simulate a reload of the same file by delivering a new
	// FileLoadCompleteMsg with the same path.
	m, _ = update(t, m, app.FileLoadCompleteMsg{
		Path:   []byte("src/a.go"),
		Buffer: buf,
	})

	// The saved offset should be restored.
	if m.ViewportOffset() != 5 {
		t.Fatalf("ViewportOffset after reload = %d, want 5 (restored)", m.ViewportOffset())
	}
}

// TestPerFileViewportStateFirstVisitStartsAtTop verifies that a first
// visit to a file (no saved state) starts at the top of the file.
func TestPerFileViewportStateFirstVisitStartsAtTop(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(50)
	buf := makeBuf(lines, 50, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	if m.ViewportOffset() != 0 {
		t.Fatalf("ViewportOffset = %d, want 0 (first visit starts at top)", m.ViewportOffset())
	}
}

// --- Scroll key behavior tests ---

// TestScrollDownOneRow verifies that pressing down in the browse state
// scrolls the viewport by one rendered row.
func TestScrollDownOneRow(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(50)
	buf := makeBuf(lines, 50, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.ViewportOffset() != 1 {
		t.Fatalf("ViewportOffset = %d, want 1", m.ViewportOffset())
	}
}

// TestScrollUpOneRow verifies that pressing up in the browse state
// scrolls the viewport up by one rendered row.
func TestScrollUpOneRow(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(50)
	buf := makeBuf(lines, 50, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	// Scroll down first.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	// Scroll up.
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.ViewportOffset() != 4 {
		t.Fatalf("ViewportOffset = %d, want 4", m.ViewportOffset())
	}
}

// TestScrollHalfDown verifies that pressing d scrolls by
// max(1, floor(contentHeight / 2)).
func TestScrollHalfDown(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(100)
	buf := makeBuf(lines, 100, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight = 23, half = 11
	m, _ = update(t, m, keyPress('d'))
	if m.ViewportOffset() != 11 {
		t.Fatalf("ViewportOffset = %d, want 11", m.ViewportOffset())
	}
}

// TestScrollHalfUp verifies that pressing u scrolls up by
// max(1, floor(contentHeight / 2)).
func TestScrollHalfUp(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(100)
	buf := makeBuf(lines, 100, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight = 23, half = 11
	// Scroll down first.
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, keyPress('d'))
	}
	// Scroll up by half.
	m, _ = update(t, m, keyPress('u'))
	// 3 * 11 = 33, then 33 - 11 = 22
	if m.ViewportOffset() != 22 {
		t.Fatalf("ViewportOffset = %d, want 22", m.ViewportOffset())
	}
}

// TestScrollPageDown verifies that pressing page down scrolls by the
// full content height.
func TestScrollPageDown(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(100)
	buf := makeBuf(lines, 100, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight = 23
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	if m.ViewportOffset() != 23 {
		t.Fatalf("ViewportOffset = %d, want 23", m.ViewportOffset())
	}
}

// TestScrollPageUp verifies that pressing page up scrolls up by the
// full content height.
func TestScrollPageUp(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(100)
	buf := makeBuf(lines, 100, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight = 23
	// Scroll down first.
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	// Scroll up by a page.
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgUp})
	// 3 * 23 = 69, then 69 - 23 = 46
	if m.ViewportOffset() != 46 {
		t.Fatalf("ViewportOffset = %d, want 46", m.ViewportOffset())
	}
}

// TestScrollClampBOF verifies that pressing up at the top of the file
// does nothing.
func TestScrollClampBOF(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(50)
	buf := makeBuf(lines, 50, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.ViewportOffset() != 0 {
		t.Fatalf("ViewportOffset = %d, want 0 (BOF clamp)", m.ViewportOffset())
	}
}

// TestScrollClampEOF verifies that scrolling past EOF stops with the
// last row at the bottom.
func TestScrollClampEOF(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(50)
	buf := makeBuf(lines, 50, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight = 23, maxOffset = 27
	// Scroll to the bottom by pressing page down many times.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if m.ViewportOffset() != 27 {
		t.Fatalf("ViewportOffset = %d, want 27 (EOF clamp)", m.ViewportOffset())
	}
}

// --- Render-cost guard (App level) ---

// TestRenderCostGuard verifies that a frame render queries the row
// provider only for the visible row range, not the full buffer. A
// counting fake tracks every Rows call; after rendering, only the
// visible [0, contentHeight) range should have been queried.
func TestRenderCostGuard(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(100)
	buf := makeBuf(lines, 100, 4)

	counter := &countingRows{lines: lines}
	factory := func(buf *filebuffer.Buffer) viewport.RowProvider {
		return counter
	}

	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithRowProviderFactory(factory),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24}) // contentHeight = 23
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	// Reset the counter and render the view.
	counter.queries = nil
	_ = viewContent(m)

	// Verify only the visible range was queried.
	if len(counter.queries) != 1 {
		t.Fatalf("queries = %d, want 1", len(counter.queries))
	}
	start, end := counter.queries[0][0], counter.queries[0][1]
	if start != 0 || end != 23 {
		t.Fatalf("query = [%d, %d), want [0, 23) (visible range only)", start, end)
	}
}

// TestRenderCostGuardAfterScroll verifies the render-cost guard after
// scrolling: only the new visible range is queried.
func TestRenderCostGuardAfterScroll(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(100)
	buf := makeBuf(lines, 100, 4)

	counter := &countingRows{lines: lines}
	factory := func(buf *filebuffer.Buffer) viewport.RowProvider {
		return counter
	}

	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithRowProviderFactory(factory),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24}) // contentHeight = 23
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	// Scroll down 10 rows.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}

	// Reset and render.
	counter.queries = nil
	_ = viewContent(m)

	if len(counter.queries) != 1 {
		t.Fatalf("queries = %d, want 1", len(counter.queries))
	}
	start, end := counter.queries[0][0], counter.queries[0][1]
	if start != 10 || end != 33 {
		t.Fatalf("query = [%d, %d), want [10, 33) (visible range after scroll)", start, end)
	}
}

// TestRenderShowsOnlyVisibleRows verifies that the rendered view
// contains only the visible rows, not the full buffer.
func TestRenderShowsOnlyVisibleRows(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	lines := makeScrollLines(100)
	buf := makeBuf(lines, 100, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24) // contentHeight = 23

	view := viewContent(m)
	// The first visible row should be "line-1".
	if !strings.Contains(view, "line-1") {
		t.Fatalf("View does not contain 'line-1': %q", view)
	}
	// Line 24 should not be visible (contentHeight = 23, so lines 1-23
	// are visible, line 24 is not).
	if strings.Contains(view, "line-24") {
		t.Fatalf("View contains 'line-24' (should not be visible): %q", view)
	}

	// Scroll down 10 rows.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	view = viewContent(m)
	// Line 1 should no longer be visible.
	if strings.Contains(view, "line-1\n") {
		t.Fatalf("View contains 'line-1' after scroll (should not be visible): %q", view)
	}
	// Line 11 should be visible.
	if !strings.Contains(view, "line-11") {
		t.Fatalf("View does not contain 'line-11' after scroll: %q", view)
	}
}
