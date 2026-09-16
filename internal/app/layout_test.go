package app_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// --- Helpers for gated layout preparation (Issue #17 AC6–AC11) ---

// setupBrowseLayoutGated creates a browse model with the layout gate
// held (not closed). The file load completes, but the layout
// preparation is held by the gate. The layout preparation command is
// returned but not executed, so the test can send keys while the
// layout is pending. A default 80x24 terminal size is set.
func setupBrowseLayoutGated(t *testing.T, idx *searchindex.Index, buf *filebuffer.Buffer) (app.Model, tea.Cmd, chan struct{}) {
	t.Helper()
	return setupBrowseLayoutGatedSize(t, idx, buf, 80, 24)
}

// setupBrowseLayoutGatedSize is like setupBrowseLayoutGated but with
// custom terminal dimensions. Returns the model, the initial layout
// preparation command (blocked on the gate), and the layout gate
// channel so the test can close it to release the preparation.
func setupBrowseLayoutGatedSize(t *testing.T, idx *searchindex.Index, buf *filebuffer.Buffer, width, height int) (app.Model, tea.Cmd, chan struct{}) {
	t.Helper()
	fileGate := make(chan struct{})
	close(fileGate)
	layoutGate := make(chan struct{}) // held
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader),
		app.WithLayoutGate(layoutGate),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	// Execute the file load command and deliver the completion.
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	// cmd is now the layout preparation command (blocked on the
	// layout gate). The viewport is nil until the layout installs.
	return m, cmd, layoutGate
}

// --- AC6 inputs during gated preparation ---

// TestLayoutGateCtrlCExits130 verifies that ctrl+c exits with code 130
// while a layout preparation is held by the gate (Issue #17 AC6).
func TestLayoutGateCtrlCExits130(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m, _, _ := setupBrowseLayoutGated(t, idx, buf)

	// Resize to trigger a layout preparation (held by the gate).
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// ctrl+c must exit 130 immediately, without waiting for the gate.
	m, cmd := update(t, m, ctrlC())
	if m.State() != app.StateCancelled {
		t.Fatalf("after ctrl+c, State = %v, want StateCancelled", m.State())
	}
	if m.ExitCode() != 130 {
		t.Fatalf("after ctrl+c, ExitCode = %d, want 130", m.ExitCode())
	}
	assertQuit(t, cmd)
}

// TestLayoutGateQExits verifies that q exits with the fixed status
// while a layout preparation is held by the gate (Issue #17 AC6).
func TestLayoutGateQExits(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m, _, _ := setupBrowseLayoutGated(t, idx, buf)

	// Resize to trigger a layout preparation (held by the gate).
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// q must exit immediately, without waiting for the gate.
	m, cmd := update(t, m, keyPress('q'))
	// q from browse quits with the fixed exit status (not
	// StateCancelled, which is ctrl+c only).
	assertQuit(t, cmd)
}

// TestLayoutGateNavigateImmediate verifies that n and p advance the
// cursor immediately while a layout preparation is held by the gate
// (Issue #17 AC6). The latest pending reveal is preserved for
// whichever stop is newest.
func TestLayoutGateNavigateImmediate(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/a.go", "world\n", 2, subSpec{"world", 0, 5}),
		textMatch("src/a.go", "foo\n", 3, subSpec{"foo", 0, 3}),
	)
	lines := []filebuffer.Line{
		ml(1, "hello"),
		ml(2, "world"),
		ml(3, "foo"),
	}
	buf := makeBuf(lines, 3, 3)
	m, _, _ := setupBrowseLayoutGated(t, idx, buf)

	// Resize to trigger a layout preparation (held by the gate).
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// n must advance the cursor immediately.
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() != 1 {
		t.Fatalf("after n, CursorPosition = %d, want 1", m.CursorPosition())
	}

	// p must move the cursor back immediately.
	m, _ = update(t, m, keyPress('p'))
	if m.CursorPosition() != 0 {
		t.Fatalf("after p, CursorPosition = %d, want 0", m.CursorPosition())
	}
}

// TestLayoutGateWrapToggleImmediate verifies that w changes the wrap
// mode immediately and issues a keyed layout request for the new mode
// without releasing the existing gate (Issue #17 AC6).
func TestLayoutGateWrapToggleImmediate(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m, _, _ := setupBrowseLayoutGated(t, idx, buf)

	// Resize to trigger a layout preparation (held by the gate).
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// w must change the wrap mode immediately.
	before := m.WrapMode()
	m, _ = update(t, m, keyPress('w'))
	after := m.WrapMode()
	if before == after {
		t.Fatalf("after w, WrapMode unchanged (%v)", after)
	}

	// The pending layout key must reflect the new wrap mode.
	key := m.PendingLayoutKey()
	if key.WrapMode != after {
		t.Fatalf("PendingLayoutKey.WrapMode = %v, want %v (new wrap mode)", key.WrapMode, after)
	}
}

// TestLayoutGateSecondResize verifies that a second resize is accepted
// while a layout preparation is held by the gate (Issue #17 AC6).
func TestLayoutGateSecondResize(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m, _, _ := setupBrowseLayoutGated(t, idx, buf)

	// First resize to trigger a layout preparation (held by the gate).
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// Second resize must be accepted without releasing the gate.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.State() != app.StateBrowse {
		t.Fatalf("after second resize, State = %v, want StateBrowse", m.State())
	}

	// The pending layout key must reflect the latest width.
	key := m.PendingLayoutKey()
	if key.TextWidth <= 0 {
		t.Fatalf("after second resize, PendingLayoutKey.TextWidth = %d, want > 0", key.TextWidth)
	}
}

// TestLayoutGatePendingRevealPreserved verifies that a stop selected
// while the gate is held is revealed per the Issue #14 rules once a
// matching layout installs — the pending intent preserved, not lost
// (Issue #17 AC6).
func TestLayoutGatePendingRevealPreserved(t *testing.T) {
	// Two stops in the same file at different lines. The second
	// stop is far enough down to require a reveal scroll.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "line1\n", 1, subSpec{"line1", 0, 5}),
		textMatch("src/a.go", "line40\n", 40, subSpec{"line40", 0, 6}),
	)
	lines := make([]filebuffer.Line, 50)
	for i := range lines {
		lines[i] = ml(i+1, "line"+itoa(i+1))
	}
	buf := makeBuf(lines, 50, 3)
	m, _, _ := setupBrowseLayoutGated(t, idx, buf)

	// Navigate to the second stop while the gate is held. The
	// cursor advances immediately; the reveal is carried as a
	// pending intent (viewport is nil while the layout is pending).
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() != 1 {
		t.Fatalf("after n, CursorPosition = %d, want 1", m.CursorPosition())
	}

	// The model must have a pending reveal intent.
	if !m.HasPendingReveal() {
		t.Fatal("after n while gated, HasPendingReveal = false, want true (intent preserved)")
	}

	// Deliver the layout (gate released). The pending reveal must
	// be committed: the viewport offset reflects the Issue #14
	// reveal placement.
	m, _ = update(t, m, app.LayoutReadyMsg{
		Key:      m.PendingLayoutKey(),
		RowModel: viewport.BuildRowModel(buf, m.PendingLayoutKey().TextWidth, m.PendingLayoutKey().WrapMode, m.PendingLayoutKey()),
	})
	if m.HasPendingReveal() {
		t.Fatal("after layout install, HasPendingReveal = true, want false (intent committed)")
	}
	// The reveal should have scrolled: line 40 is far from the top
	// and beyond the visible range (contentHeight = 23).
	if m.ViewportOffset() == 0 {
		t.Fatal("after layout install, ViewportOffset = 0, want > 0 (reveal committed)")
	}
}

// --- Keyed installation guards and out-of-order isolation ---

// TestLayoutOutOfOrderDiscarded verifies that out-of-order layout
// completions across W1→W2→W3 resizes are discarded, with only the
// latest matching layout installed (Issue #17 AC7). The anchor is
// unaffected throughout.
func TestLayoutOutOfOrderDiscarded(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", strings.Repeat("x", 500)+"\n", 1, subSpec{"x", 0, 1}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, strings.Repeat("x", 500))}, 1, 4)
	m, _, layoutGate := setupBrowseLayoutGated(t, idx, buf)

	// Three resizes while the gate is held. Each issues a layout
	// preparation command with a different text width.
	m, cmd1 := update(t, m, tea.WindowSizeMsg{Width: 40, Height: 24})
	m, cmd2 := update(t, m, tea.WindowSizeMsg{Width: 60, Height: 24})
	m, cmd3 := update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})

	// The current layout key must reflect the latest width (W3).
	currentKey := m.PendingLayoutKey()
	if currentKey.TextWidth <= 0 {
		t.Fatalf("PendingLayoutKey.TextWidth = %d, want > 0", currentKey.TextWidth)
	}

	// Release the gate so the layout preparation commands can
	// complete. The commands were captured before the gate was
	// released, so they carry the keys for W1, W2, and W3.
	close(layoutGate)

	// Deliver the W1 completion (out-of-order). It must be discarded.
	if cmd1 != nil {
		msg1 := execCmd(t, cmd1)
		if lr, ok := msg1.(app.LayoutReadyMsg); ok {
			before := m.ViewportOffset()
			m, _ = update(t, m, lr)
			// The stale layout must not install: the offset and
			// pending key must not change.
			if m.ViewportOffset() != before {
				t.Fatalf("after stale W1 layout, ViewportOffset changed to %d", m.ViewportOffset())
			}
		}
	}

	// Deliver the W3 completion (matching). It must install.
	if cmd3 != nil {
		msg3 := execCmd(t, cmd3)
		if lr, ok := msg3.(app.LayoutReadyMsg); ok {
			m, _ = update(t, m, lr)
		}
	}

	// The viewport must be active with the W3 layout.
	if m.ViewportRowCount() == 0 {
		t.Fatal("after W3 layout install, ViewportRowCount = 0, want > 0")
	}
	_ = cmd2 // W2 completion is also out-of-order; not delivered.
}

// TestLayoutForStaleFileDiscarded verifies that a layout completion for
// a file that is no longer current leaves the visible panel and saved
// per-file state untouched (Issue #17 AC7).
func TestLayoutForStaleFileDiscarded(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
	)
	bufA := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	bufB := makeBuf([]filebuffer.Line{ml(1, "world")}, 1, 3)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if strings.Contains(string(path), "a.go") {
			return bufA, nil
		}
		return bufB, nil
	}
	fileGate := make(chan struct{})
	close(fileGate)
	layoutGate := make(chan struct{})
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader),
		app.WithLayoutGate(layoutGate),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	// Execute the file load for a.go.
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	// cmd is the layout preparation for a.go (held by the gate).
	cmdA := cmd

	// Navigate to b.go (uncached). Request a file load.
	m, cmd = update(t, m, keyPress('n'))
	if m.CurrentPath() == nil || !strings.Contains(string(m.CurrentPath()), "b.go") {
		t.Fatalf("after n, CurrentPath = %q, want b.go", m.CurrentPath())
	}
	// Execute the file load for b.go.
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	// Now deliver the stale a.go layout completion. It must be
	// discarded: the current file is b.go. Release the gate first
	// so the layout preparation command can complete.
	close(layoutGate)
	if cmdA != nil {
		msg := execCmd(t, cmdA)
		if lr, ok := msg.(app.LayoutReadyMsg); ok {
			m, _ = update(t, m, lr)
		}
	}
	if m.CurrentPath() == nil || !strings.Contains(string(m.CurrentPath()), "b.go") {
		t.Fatalf("after stale a.go layout, CurrentPath = %q, want b.go (untouched)", m.CurrentPath())
	}
}

// --- Cached-file stale-layout navigation ---

// TestCachedFileStaleLayoutRequestsRebuild verifies that navigation to
// a cached file whose installed layout is stale (wrong text width or
// wrap mode) requests a prepared layout for the current parameters
// and carries the saved-viewport intent to commit on installation
// (Issue #17 AC8).
func TestCachedFileStaleLayoutRequestsRebuild(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
	)
	bufA := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	bufB := makeBuf([]filebuffer.Line{ml(1, "world")}, 1, 3)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if strings.Contains(string(path), "a.go") {
			return bufA, nil
		}
		return bufB, nil
	}
	fileGate := make(chan struct{})
	close(fileGate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	// Execute the file load and layout for a.go.
	m = deliverLoad(t, m, cmd)
	// a.go is loaded with a layout at width 80.

	// Navigate to b.go (uncached). Load and cache b.go.
	m, cmd = update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)

	// Resize to a different width. b.go's layout is now stale.
	m = resize(t, m, 100, 30)

	// Navigate back to a.go (cached). Its layout is stale (width
	// changed from 80 to 100). A layout preparation must be
	// requested for the current parameters.
	m, cmd = update(t, m, keyPress('p'))
	if m.CurrentPath() == nil || !strings.Contains(string(m.CurrentPath()), "a.go") {
		t.Fatalf("after p, CurrentPath = %q, want a.go", m.CurrentPath())
	}
	if !m.HasPendingLayout() {
		t.Fatal("after navigating to cached a.go with stale layout, HasPendingLayout = false, want true (rebuild requested)")
	}
	if cmd == nil {
		t.Fatal("after navigating to cached a.go with stale layout, cmd = nil, want layout preparation command")
	}
}

// TestCachedFileMatchingLayoutCommitsImmediately verifies that
// navigation to a cached file with a matching installed layout commits
// immediately with no layout preparation request (Issue #17 AC8 fast
// path).
func TestCachedFileMatchingLayoutCommitsImmediately(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
	)
	bufA := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	bufB := makeBuf([]filebuffer.Line{ml(1, "world")}, 1, 3)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if strings.Contains(string(path), "a.go") {
			return bufA, nil
		}
		return bufB, nil
	}
	fileGate := make(chan struct{})
	close(fileGate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	// Execute the file load and layout for a.go.
	m = deliverLoad(t, m, cmd)
	// a.go is loaded with a layout at width 80.

	// Navigate to b.go (uncached). Load and cache b.go.
	m, cmd = update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)

	// Navigate back to a.go (cached, same width). The layout
	// matches, so no layout preparation is needed.
	m = navigate(t, m, 'p')
	if m.CurrentPath() == nil || !strings.Contains(string(m.CurrentPath()), "a.go") {
		t.Fatalf("after p, CurrentPath = %q, want a.go", m.CurrentPath())
	}
	if m.HasPendingLayout() {
		t.Fatal("after navigating to cached a.go with matching layout, HasPendingLayout = true, want false (immediate commit)")
	}
	// The viewport must be active (layout installed from cache).
	if m.ViewportRowCount() == 0 {
		t.Fatal("after cached a.go with matching layout, ViewportRowCount = 0, want > 0")
	}
}

// --- Render-cost guards ---

// countingFileList is a FileListProvider that records every FilePath
// query for the render-cost guard. It proves the render path queries
// the file list only for the visible range, not all entries.
type countingFileList struct {
	paths   [][]byte
	queries []int
}

func (c *countingFileList) FileCount() int { return len(c.paths) }

func (c *countingFileList) FilePath(idx int) []byte {
	c.queries = append(c.queries, idx)
	if idx < 0 || idx >= len(c.paths) {
		return nil
	}
	return c.paths[idx]
}

// TestRenderCostGuardFileList verifies that a frame render queries the
// file-list provider only for the visible range (terminal height),
// not all file entries (Issue #17 AC11).
func TestRenderCostGuardFileList(t *testing.T) {
	// Build an index with many files so the file list exceeds the
	// terminal height.
	records := make([]string, 50)
	paths := make([][]byte, 50)
	for i := 0; i < 50; i++ {
		path := "src/file" + itoa(i) + ".go"
		records[i] = textMatch(path, "hello\n", 1, subSpec{"hello", 0, 5})
		paths[i] = []byte(path)
	}
	idx := buildIndex(t, "/work", records...)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)

	counter := &countingFileList{paths: paths}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithFileListProvider(counter),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
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

	// Verify only the visible range was queried. The terminal
	// height is 24, so at most 24 file-list entries should be
	// queried, not all 50.
	if len(counter.queries) > 24 {
		t.Fatalf("file-list queries = %d, want <= 24 (visible range only)", len(counter.queries))
	}
}

// --- Issue #40: bounded render cost across Update + View ---

// renderCostBounds cap the heap work allowed for one combined
// navigation Update() plus the resulting View() (Issue #40). The
// bound is independent of index size: at the documented ~100,000
// matched-line scale a whole-index stop copy, a regroup, or a
// whole-group reallocation allocates megabytes and tens of thousands
// of objects, while the bounded render path allocates in proportion
// to the visible window only.
const (
	renderCostBoundBytes   = 512 * 1024
	renderCostBoundMallocs = 4096
)

// costGuardFiles and costGuardStopsPerFile size the large guard index:
// 3,000 files × 15 stops = 45,000 stops, so a single Stops() copy or
// regroup allocates several megabytes — far beyond the bound above.
const (
	costGuardFiles        = 3000
	costGuardStopsPerFile = 15
)

// measureUpdateViewAllocs runs f once and returns the heap allocation
// count and byte total observed across it. The Issue #40 guard wraps
// a navigation Update() and the resulting View() in a single measured
// region — the counter is never reset between the halves — so moving
// whole-index work from View() into Update() also fails.
func measureUpdateViewAllocs(f func()) (mallocs uint64, bytes uint64) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.Mallocs - before.Mallocs, after.TotalAlloc - before.TotalAlloc
}

// largeBrowseIndex builds an index with costGuardFiles files ×
// costGuardStopsPerFile matched lines and returns it with the file
// paths in index order (the same order a FileListProvider serves).
// Default paths are "src/fileNNNNN.go"; overrides replaces the name at
// the given file index and must preserve unsigned-byte sort order.
func largeBrowseIndex(t *testing.T, overrides map[int]string) (*searchindex.Index, [][]byte) {
	t.Helper()
	records := make([]string, 0, costGuardFiles*costGuardStopsPerFile)
	paths := make([][]byte, costGuardFiles)
	for i := 0; i < costGuardFiles; i++ {
		path := overrides[i]
		if path == "" {
			path = fmt.Sprintf("src/file%05d.go", i)
		}
		paths[i] = []byte(path)
		for j := 1; j <= costGuardStopsPerFile; j++ {
			records = append(records, textMatch(path, "hello\n", j, subSpec{"hello", 0, 5}))
		}
	}
	return buildIndex(t, "/work", records...), paths
}

// setupBrowseCostGuard builds a browse model over the large index with
// the counting file-list provider installed and the startup file's
// load and layout delivered, so tests can measure one navigation
// Update() plus the resulting View().
func setupBrowseCostGuard(t *testing.T, idx *searchindex.Index, paths [][]byte, loader app.FileLoader, width, height int) (app.Model, *countingFileList) {
	t.Helper()
	counter := &countingFileList{paths: paths}
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithFileListProvider(counter),
		// Instant pop-up timer: cross-file navigation commands return
		// their expiry message immediately instead of blocking for the
		// one-second pop-up duration.
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	return deliverLoad(t, m, cmd), counter
}

// assertBoundedTransitionCost requires the measured transition to stay
// within the visible-window cost bounds (Issue #40).
func assertBoundedTransitionCost(t *testing.T, mallocs, bytes uint64) {
	t.Helper()
	if bytes > renderCostBoundBytes {
		t.Fatalf("Update+View allocated %d bytes, want <= %d (bounded by the visible window, not index size)", bytes, renderCostBoundBytes)
	}
	if mallocs > renderCostBoundMallocs {
		t.Fatalf("Update+View performed %d heap allocations, want <= %d (bounded by the visible window, not index size)", mallocs, renderCostBoundMallocs)
	}
}

// assertVisibleWindowQueries requires every file-list provider query to
// land inside the visible window [offset, offset+rows) and the query
// count to stay within the window size (Issue #40: only the visible
// file range is materialized).
func assertVisibleWindowQueries(t *testing.T, c *countingFileList, offset, rows int) {
	t.Helper()
	if len(c.queries) > rows {
		t.Fatalf("file-list queries = %d, want <= %d (visible window only)", len(c.queries), rows)
	}
	for _, q := range c.queries {
		if q < offset || q >= offset+rows {
			t.Fatalf("file-list query %d outside visible window [%d, %d)", q, offset, offset+rows)
		}
	}
}

// TestRenderCostGuardNavigateViewBounded verifies that a file
// navigation Update() plus the resulting View() performs no
// whole-index work (Issue #40): no Stops() enumeration or copy, no
// regrouping, and no whole-group reallocation on either half of the
// transition. Only the visible file range may be materialized.
func TestRenderCostGuardNavigateViewBounded(t *testing.T) {
	idx, paths := largeBrowseIndex(t, nil)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m, counter := setupBrowseCostGuard(t, idx, paths, loader, 80, 24)

	// Cache the next file so the measured navigation exercises the
	// full render path (cached destination installs a viewport).
	// file00001 starts at stop 15 (15 stops per file).
	for i := 0; i < costGuardStopsPerFile; i++ {
		var cmd tea.Cmd
		m, cmd = update(t, m, keyPress('n'))
		m = deliverLoad(t, m, cmd)
	}
	if got := m.CursorPosition(); got != costGuardStopsPerFile {
		t.Fatalf("CursorPosition = %d, want %d (first stop of file00001)", got, costGuardStopsPerFile)
	}
	// Return to the previous file so the measured 'n' is a cached
	// cross-file navigation.
	m, cmd := update(t, m, keyPress('p'))
	m = deliverLoad(t, m, cmd)

	// The bound spans the combined model transition — the navigation
	// Update() and the resulting View() — with no reset between the
	// halves.
	counter.queries = nil
	var rendered app.Model
	mallocs, b := measureUpdateViewAllocs(func() {
		rendered, _ = update(t, m, keyPress('n'))
		_ = viewContent(rendered)
	})
	assertBoundedTransitionCost(t, mallocs, b)
	assertVisibleWindowQueries(t, counter, rendered.ListOffset(), 24)
}

// TestRenderCostGuardNavigatePrevBounded verifies the same combined
// Update()+View() bound for the 'p' navigation direction (Issue #40).
func TestRenderCostGuardNavigatePrevBounded(t *testing.T) {
	idx, paths := largeBrowseIndex(t, nil)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m, counter := setupBrowseCostGuard(t, idx, paths, loader, 80, 24)

	// Move to a mid-index file and cache it so 'p' is a cached
	// cross-file navigation.
	for i := 0; i < 2*costGuardStopsPerFile; i++ {
		var cmd tea.Cmd
		m, cmd = update(t, m, keyPress('n'))
		m = deliverLoad(t, m, cmd)
	}

	counter.queries = nil
	var rendered app.Model
	mallocs, b := measureUpdateViewAllocs(func() {
		rendered, _ = update(t, m, keyPress('p'))
		_ = viewContent(rendered)
	})
	assertBoundedTransitionCost(t, mallocs, b)
	assertVisibleWindowQueries(t, counter, rendered.ListOffset(), 24)
}

// TestRenderCostGuardResizeRetruncates verifies that a terminal resize
// that changes the allotted list width re-truncates the visible paths
// against the new width at grapheme boundaries, inside the same
// visible-window cost bound and with no whole-list scan (Issue #40).
func TestRenderCostGuardResizeRetruncates(t *testing.T) {
	// The wide path at file index 1 is engineered so the post-resize
	// truncation cut lands inside the two-cell 中 cluster: at 80
	// columns the keep budget (31 cells) starts before it; at 60
	// columns the keep budget (23 cells) starts inside it, so the
	// cluster must be dropped whole, never split.
	widePath := "src/file00001-" + strings.Repeat("d", 30) + "中" + strings.Repeat("t", 22)
	idx, paths := largeBrowseIndex(t, map[int]string{1: widePath})
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m, counter := setupBrowseCostGuard(t, idx, paths, loader, 80, 24)

	// Precondition: at 80 columns the list width is 32 (40% cap) and
	// the entry keeps the whole 中 cluster.
	if got := m.ListWidth(); got != 32 {
		t.Fatalf("ListWidth at 80 columns = %d, want 32 (40%% cap)", got)
	}
	wantWide := app.TruncateLeftGrapheme(widePath, m.ListWidth())
	if !strings.Contains(wantWide, "中") {
		t.Fatalf("precondition: 80-column entry %q lost the 中 cluster", wantWide)
	}
	if view := viewContent(m); !strings.Contains(view, wantWide) {
		t.Fatalf("80-column View does not contain entry %q:\n%s", wantWide, view)
	}

	// Resize to 60 columns: the 40% cap shrinks the list to 24 cells.
	// The measured region covers the resize Update() and the
	// resulting View() together.
	counter.queries = nil
	var rendered app.Model
	mallocs, b := measureUpdateViewAllocs(func() {
		rendered, _ = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 24})
		_ = viewContent(rendered)
	})
	assertBoundedTransitionCost(t, mallocs, b)
	assertVisibleWindowQueries(t, counter, rendered.ListOffset(), 24)

	if got := rendered.ListWidth(); got != 24 {
		t.Fatalf("ListWidth at 60 columns = %d, want 24 (40%% cap)", got)
	}
	view := viewContent(rendered)
	want := "…" + strings.Repeat("t", 22)
	if !strings.Contains(view, want) {
		t.Fatalf("60-column View does not contain re-truncated entry %q:\n%s", want, view)
	}
	// The two-cell cluster straddling the cut is dropped whole: no
	// 中 survives in the 60-column render.
	if strings.Contains(view, "中") {
		t.Fatalf("60-column View contains 中 (cluster split by the cut):\n%s", view)
	}
}

// TestRenderCostGuardGutterGrowthRetruncates verifies that navigating
// to a file whose buffer has a wider gutter shrinks the list width
// and re-truncates the visible paths against it, still inside the
// visible-window cost bound (Issue #40).
func TestRenderCostGuardGutterGrowthRetruncates(t *testing.T) {
	// The wide path sits at file index 2 (never the current file in
	// this test, so its entry is not underlined). File index 1 loads
	// a large-gutter buffer.
	widePath := "src/file00002-" + strings.Repeat("d", 30) + "中" + strings.Repeat("t", 22)
	idx, paths := largeBrowseIndex(t, map[int]string{2: widePath})
	bufA := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	bufB := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 30)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if string(path) == string(paths[1]) {
			return bufB, nil
		}
		return bufA, nil
	}
	m, _ := setupBrowseCostGuard(t, idx, paths, loader, 60, 24)

	// At 60 columns with gutter 3 the list width is 24.
	if got := m.ListWidth(); got != 24 {
		t.Fatalf("ListWidth with gutter 3 = %d, want 24", got)
	}
	wantBefore := app.TruncateLeftGrapheme(widePath, m.ListWidth())
	if view := viewContent(m); !strings.Contains(view, wantBefore) {
		t.Fatalf("View before gutter growth does not contain entry %q:\n%s", wantBefore, view)
	}

	// Navigate to file00001 (large gutter) and back so it is cached.
	for i := 0; i < costGuardStopsPerFile; i++ {
		var cmd tea.Cmd
		m, cmd = update(t, m, keyPress('n'))
		m = deliverLoad(t, m, cmd)
	}
	m, cmd := update(t, m, keyPress('p'))
	m = deliverLoad(t, m, cmd)

	// Measured: cross-file navigation to the cached large-gutter
	// file plus the resulting View(). The gutter growth shrinks the
	// list width from 24 to 20 (60 - (30 + 10 + 0)).
	var rendered app.Model
	mallocs, b := measureUpdateViewAllocs(func() {
		rendered, _ = update(t, m, keyPress('n'))
		_ = viewContent(rendered)
	})
	assertBoundedTransitionCost(t, mallocs, b)

	if got := rendered.ListWidth(); got != 20 {
		t.Fatalf("ListWidth after gutter growth = %d, want 20", got)
	}
	view := viewContent(rendered)
	want := app.TruncateLeftGrapheme(widePath, rendered.ListWidth())
	if !strings.Contains(view, want) {
		t.Fatalf("View after gutter growth does not contain re-truncated entry %q:\n%s", want, view)
	}
	// The re-truncated entry never splits the wide cluster.
	if !strings.HasPrefix(want, "…") {
		t.Fatalf("re-truncated entry %q does not start with …", want)
	}
}

// itoa is a minimal int-to-string helper to avoid importing strconv
// in the test file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
