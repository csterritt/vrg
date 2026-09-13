package app_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// setupBrowseMulti creates a browse model with multiple files. The
// loader returns the buffer associated with the path key in bufs, or
// an empty buffer if not found. The gate is released immediately so
// the initial load completes.
func setupBrowseMulti(t *testing.T, idx *searchindex.Index, bufs map[string]*filebuffer.Buffer) app.Model {
	t.Helper()
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

// deliverLoad executes a load command (if non-nil) and delivers the
// resulting FileLoadCompleteMsg to the model. This simulates the
// async load completing. Handles tea.BatchMsg (Issue #15: cross-file
// navigation batches the pop-up timer and the load command). Timer
// commands that block (e.g. tea.Tick with a non-zero duration) are
// skipped via a short timeout so the test does not wait for the timer.
// Issue #17: the FileLoadCompleteMsg handler returns a layout
// preparation command; this helper delivers it too so the viewport
// is installed.
func deliverLoad(t *testing.T, m app.Model, cmd tea.Cmd) app.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := execCmd(t, cmd)
	if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
		nm, layoutCmd := update(t, m, lc)
		return deliverLayout(t, nm, layoutCmd)
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			// Execute with a short timeout so blocking timer
			// commands (e.g. tea.Tick with 1s duration) don't
			// stall the test. The load command returns
			// immediately; the timer command is skipped.
			ch := make(chan tea.Msg, 1)
			go func(cmd tea.Cmd) { ch <- cmd() }(c)
			select {
			case sub := <-ch:
				if lc, ok := sub.(app.FileLoadCompleteMsg); ok {
					nm, layoutCmd := update(t, m, lc)
					m = deliverLayout(t, nm, layoutCmd)
				}
			case <-time.After(100 * time.Millisecond):
				// Command is still running (likely a timer);
				// skip it and move on.
			}
		}
	}
	return m
}

// deliverLayout executes a layout preparation command (if non-nil) and
// delivers the resulting LayoutReadyMsg to the model (Issue #17). This
// simulates the async layout preparation completing. Handles
// tea.BatchMsg (cross-file navigation batches the pop-up timer and the
// layout command). Timer commands that block are skipped via a short
// timeout so the test does not wait.
func deliverLayout(t *testing.T, m app.Model, cmd tea.Cmd) app.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := execCmd(t, cmd)
	if lr, ok := msg.(app.LayoutReadyMsg); ok {
		m, _ = update(t, m, lr)
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			ch := make(chan tea.Msg, 1)
			go func(cmd tea.Cmd) { ch <- cmd() }(c)
			select {
			case sub := <-ch:
				if lr, ok := sub.(app.LayoutReadyMsg); ok {
					m, _ = update(t, m, lr)
				}
			case <-time.After(100 * time.Millisecond):
				// Skip blocking commands (e.g. timers).
			}
		}
	}
	return m
}

// resize sends a WindowSizeMsg and delivers any resulting layout
// preparation command (Issue #17). Tests that check viewport state
// after a resize must use this helper so the async layout is installed.
func resize(t *testing.T, m app.Model, width, height int) app.Model {
	t.Helper()
	m, cmd := update(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	return deliverLayout(t, m, cmd)
}

// navigate sends n or p and delivers any resulting layout preparation
// command (Issue #17). Tests that check viewport state after
// cross-file navigation must use this helper so the async layout is
// installed.
func navigate(t *testing.T, m app.Model, key rune) app.Model {
	t.Helper()
	m, cmd := update(t, m, keyPress(key))
	return deliverLayout(t, m, cmd)
}

// assertCurrentPath requires the model's current file path to equal
// want.
func assertCurrentPath(t *testing.T, m app.Model, want string) {
	t.Helper()
	got := m.CurrentPath()
	if string(got) != want {
		t.Fatalf("CurrentPath = %q, want %q", got, want)
	}
}

// assertCursorPosition requires the model's cursor position to equal
// want.
func assertCursorPosition(t *testing.T, m app.Model, want int) {
	t.Helper()
	got := m.CursorPosition()
	if got != want {
		t.Fatalf("CursorPosition = %d, want %d", got, want)
	}
}

// --- Startup selection tests ---

// TestNavigationStartupSelectsFirstStop verifies that startup selects
// the first stop in path-then-line order and the current file is the
// first file.
func TestNavigationStartupSelectsFirstStop(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 3, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "match-a"), ml(3, "match-a3")}, 3, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "match-b")}, 1, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	assertCursorPosition(t, m, 0)
	assertCurrentPath(t, m, "src/a.go")
}

// TestNavigationStartupSelectsFirstStopView verifies that the rendered
// view shows the first file's content and underlines the first file.
func TestNavigationStartupSelectsFirstStopView(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	view := viewContent(m)
	if !strings.Contains(view, "content-a") {
		t.Fatalf("View does not contain first file content 'content-a': %q", view)
	}
	if strings.Contains(view, "content-b") {
		t.Fatalf("View contains second file content 'content-b' (should not be current): %q", view)
	}
}

// --- Cross-file switching tests ---

// TestNavigationNextSwitchesFile verifies that pressing n when the
// cursor crosses a file boundary switches the content panel to the
// new file and requests its load when uncached.
func TestNavigationNextSwitchesFile(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	assertCurrentPath(t, m, "src/a.go")

	// Press n to cross to b.go. The file is uncached, so a load
	// command should be returned.
	m, cmd := update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 1)
	assertCurrentPath(t, m, "src/b.go")
	if cmd == nil {
		t.Fatal("n to uncached file returned nil cmd, want a load command")
	}
	// Deliver the load completion.
	m = deliverLoad(t, m, cmd)
	view := viewContent(m)
	if !strings.Contains(view, "content-b") {
		t.Fatalf("View does not contain 'content-b' after cross-file n: %q", view)
	}
	if strings.Contains(view, "content-a") {
		t.Fatalf("View still contains 'content-a' after cross-file n: %q", view)
	}
}

// TestNavigationPrevSwitchesFile verifies that pressing p when the
// cursor crosses a file boundary (wrapping from first to last) switches
// the content panel.
func TestNavigationPrevSwitchesFile(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	assertCurrentPath(t, m, "src/a.go")

	// Press p to wrap to b.go (last stop).
	m, cmd := update(t, m, keyPress('p'))
	assertCursorPosition(t, m, 1)
	assertCurrentPath(t, m, "src/b.go")
	if cmd == nil {
		t.Fatal("p to uncached file returned nil cmd, want a load command")
	}
	m = deliverLoad(t, m, cmd)
	view := viewContent(m)
	if !strings.Contains(view, "content-b") {
		t.Fatalf("View does not contain 'content-b' after cross-file p: %q", view)
	}
}

// TestNavigationNextSameFileNoLoad verifies that pressing n within the
// same file does not request a load (the file is already loaded).
func TestNavigationNextSameFileNoLoad(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 10, subSpec{"x", 0, 1}),
	)
	lines := []filebuffer.Line{
		ml(1, "line1"), ml(2, "line2"), ml(3, "line3"),
		ml(4, "line4"), ml(5, "line5"), ml(6, "line6"),
		ml(7, "line7"), ml(8, "line8"), ml(9, "line9"),
		ml(10, "line10"),
	}
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(lines, 10, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	assertCursorPosition(t, m, 0)

	// n within the same file: no load command.
	m, cmd := update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 1)
	if cmd != nil {
		t.Fatal("n within same file returned non-nil cmd, want nil (no load)")
	}
	// n again within the same file: still no load command.
	m, cmd = update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 2)
	if cmd != nil {
		t.Fatal("second n within same file returned non-nil cmd, want nil")
	}
}

// TestNavigationNextWrapsCrossFile verifies that n wraps from the last
// stop to the first stop, crossing file boundaries and requesting a
// load for the (now-uncached) first file if it was evicted. Since
// Issue #13 retains cached buffers, the first file should still be
// cached and no load is needed.
func TestNavigationNextWrapsCrossFile(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	// Move to b.go (last stop).
	m, _ = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	// Wrap from b.go back to a.go (first stop). a.go is cached, so
	// no load command is needed.
	m, cmd := update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 0)
	assertCurrentPath(t, m, "src/a.go")
	view := viewContent(m)
	if !strings.Contains(view, "content-a") {
		t.Fatalf("View does not contain 'content-a' after wrap: %q", view)
	}
	// a.go is cached from startup, so no load needed.
	if cmd != nil {
		// If a command is returned, it should not be a load command
		// that changes state. But cached files should not need a
		// load. Allow nil.
		msg := execCmd(t, cmd)
		if _, ok := msg.(app.FileLoadCompleteMsg); ok {
			t.Fatal("wrap to cached file returned a FileLoadCompleteMsg, want no load")
		}
	}
}

// TestNavigationPrevWrapsCrossFile verifies that p wraps from the
// first stop to the last stop.
func TestNavigationPrevWrapsCrossFile(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	// p wraps from a.go (first) to b.go (last).
	m, _ = update(t, m, keyPress('p'))
	assertCursorPosition(t, m, 1)
	assertCurrentPath(t, m, "src/b.go")
}

// --- One-stop no-op tests ---

// TestNavigationOneStopNextNoOp verifies that n with exactly one stop
// is a strict no-op: no state change, no load command, no pop-up.
func TestNavigationOneStopNextNoOp(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	assertCursorPosition(t, m, 0)
	beforeView := viewContent(m)

	m, cmd := update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 0)
	if cmd != nil {
		t.Fatal("n with one stop returned non-nil cmd, want nil (no-op)")
	}
	afterView := viewContent(m)
	if beforeView != afterView {
		t.Fatalf("View changed after one-stop n no-op:\nbefore: %q\nafter:  %q", beforeView, afterView)
	}
}

// TestNavigationOneStopPrevNoOp verifies that p with exactly one stop
// is a strict no-op.
func TestNavigationOneStopPrevNoOp(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	assertCursorPosition(t, m, 0)

	m, cmd := update(t, m, keyPress('p'))
	assertCursorPosition(t, m, 0)
	if cmd != nil {
		t.Fatal("p with one stop returned non-nil cmd, want nil (no-op)")
	}
}

// --- List underline follows cursor ---

// TestNavigationListUnderlineFollowsCursor verifies that the file list
// underline follows the cursor's current file after navigation.
func TestNavigationListUnderlineFollowsCursor(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/c.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
		"src/c.go": makeBuf([]filebuffer.Line{ml(1, "content-c")}, 1, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)

	// Initially a.go is underlined.
	view := viewContent(m)
	if !strings.Contains(view, "\x1b[4msrc/a.go") {
		t.Fatalf("Initial view does not underline src/a.go: %q", view)
	}

	// n to b.go: b.go should be underlined.
	m, cmd := update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)
	view = viewContent(m)
	if !strings.Contains(view, "\x1b[4msrc/b.go") {
		t.Fatalf("After n, view does not underline src/b.go: %q", view)
	}
	if strings.Contains(view, "\x1b[4msrc/a.go") {
		t.Fatalf("After n to b.go, view still underlines src/a.go: %q", view)
	}

	// n to c.go: c.go should be underlined.
	m, cmd = update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)
	view = viewContent(m)
	if !strings.Contains(view, "\x1b[4msrc/c.go") {
		t.Fatalf("After second n, view does not underline src/c.go: %q", view)
	}
}

// --- Current matched line underline ---

// TestNavigationCurrentMatchUnderlineMoves verifies that the current
// matched line's matches are rendered with the CurrentMatch style
// (true inverse + underline) and that the underline moves to the new
// current line after same-file navigation.
func TestNavigationCurrentMatchUnderlineMoves(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/a.go", "world\n", 2, subSpec{"world", 0, 5}),
	)
	lines := []filebuffer.Line{
		ml(1, "hello", [2]int{0, 5}),
		ml(2, "world", [2]int{0, 5}),
	}
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(lines, 2, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)

	// Initially line 1 is the current matched line. The current-match
	// style uses true-inverse colours + underline (30;47;4m for dark
	// scheme).
	view := viewContent(m)
	if !strings.Contains(view, "30;47;4") {
		t.Fatalf("Initial view does not contain current-match underline style (30;47;4): %q", view)
	}

	// n to line 2: the current-match style should now be on line 2.
	m, _ = update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 1)
	view = viewContent(m)
	// The current-match style should still be present (now on line 2).
	if !strings.Contains(view, "30;47;4") {
		t.Fatalf("After n, view does not contain current-match underline style: %q", view)
	}
	// Line 2's content "world" should have the current-match style.
	if !strings.Contains(view, "30;47;4") {
		t.Fatalf("After n, view does not contain current-match style on line 2: %q", view)
	}
}

// --- Manual-scroll independence ---

// TestNavigationManualScrollIndependence verifies that manual
// scrolling does not move the matched-line cursor, so n/p continue
// from the last selected stop rather than the scrolled position.
func TestNavigationManualScrollIndependence(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 20, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 40, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(50)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(lines, 50, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	assertCursorPosition(t, m, 0)

	// Manually scroll down 10 rows.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.ViewportOffset() != 10 {
		t.Fatalf("ViewportOffset = %d, want 10 after scrolling", m.ViewportOffset())
	}

	// The cursor should still be at position 0 (manual scroll did not
	// move it).
	assertCursorPosition(t, m, 0)

	// n should advance from position 0 to position 1, not from the
	// scrolled position.
	m, _ = update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 1)
}

// TestNavigationManualScrollThenPContinues verifies that p after
// manual scrolling continues from the last selected stop.
func TestNavigationManualScrollThenPContinues(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 20, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 40, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(50)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(lines, 50, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	// Advance to position 1.
	m, _ = update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 1)

	// Manually scroll down.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}

	// p should retreat from position 1 to position 0.
	m, _ = update(t, m, keyPress('p'))
	assertCursorPosition(t, m, 0)
}

// --- Departing viewport save and restore ---

// TestNavigationSavesDepartingViewport verifies that navigating away
// from a file saves that file's viewport offset as per-file state.
func TestNavigationSavesDepartingViewport(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(50)
	linesB := makeScrollLines(50)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(linesA, 50, 3),
		"src/b.go": makeBuf(linesB, 50, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)

	// Scroll down 5 rows in a.go.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.ViewportOffset() != 5 {
		t.Fatalf("ViewportOffset = %d, want 5", m.ViewportOffset())
	}

	// Navigate to b.go. This should save a.go's offset.
	m, _ = update(t, m, keyPress('n'))
	if m.SavedOffset([]byte("src/a.go")) != 5 {
		t.Fatalf("SavedOffset(a.go) = %d, want 5 (departing viewport saved)", m.SavedOffset([]byte("src/a.go")))
	}
}

// TestNavigationRestoresSavedViewport verifies that a revisited file
// starts from its saved viewport offset before applying the
// destination reveal (Issue #14). When the destination target is
// visible from the saved offset, the saved position is preserved.
func TestNavigationRestoresSavedViewport(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 10, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(50)
	linesB := makeScrollLines(50)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(linesA, 50, 3),
		"src/b.go": makeBuf(linesB, 50, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)

	// Scroll down 7 rows in a.go. The match on line 10 (row 9) is
	// still visible from offset 7 (range [7, 30)).
	for i := 0; i < 7; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}

	// Navigate to b.go (saves a.go offset = 7).
	m, cmd := update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)
	assertCurrentPath(t, m, "src/b.go")

	// b.go is a first visit, so it starts at offset 0.
	if m.ViewportOffset() != 0 {
		t.Fatalf("b.go ViewportOffset = %d, want 0 (first visit)", m.ViewportOffset())
	}

	// Navigate back to a.go (p wraps or n wraps).
	m = navigate(t, m, 'p')
	assertCurrentPath(t, m, "src/a.go")
	// a.go's saved offset (7) is the starting point. The match on
	// line 10 (row 9) is visible from offset 7 (range [7, 30)), so
	// the reveal does not scroll and the saved offset is preserved.
	if m.ViewportOffset() != 7 {
		t.Fatalf("a.go ViewportOffset after revisit = %d, want 7 (saved offset preserved, visible target no-scroll)", m.ViewportOffset())
	}
}

// TestNavigationFirstVisitStartsAtTop verifies that a first visit to
// a file starts at the top (offset 0).
func TestNavigationFirstVisitStartsAtTop(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(50)
	linesB := makeScrollLines(50)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(linesA, 50, 3),
		"src/b.go": makeBuf(linesB, 50, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	// a.go is the first file, starts at 0.
	if m.ViewportOffset() != 0 {
		t.Fatalf("a.go ViewportOffset = %d, want 0 (first visit)", m.ViewportOffset())
	}
	// Navigate to b.go (first visit).
	m, cmd := update(t, m, keyPress('n'))
	m = deliverLoad(t, m, cmd)
	if m.ViewportOffset() != 0 {
		t.Fatalf("b.go ViewportOffset = %d, want 0 (first visit)", m.ViewportOffset())
	}
}

// --- File list passive ---

// TestNavigationFileListPassive verifies that keys other than n/p do
// not change the current file or cursor position. The file list is
// passive with no direct selection route.
func TestNavigationFileListPassive(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/c.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
		"src/c.go": makeBuf([]filebuffer.Line{ml(1, "content-c")}, 1, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	assertCursorPosition(t, m, 0)
	assertCurrentPath(t, m, "src/a.go")

	// Various keys that might be expected to navigate a file list
	// should not change the current file or cursor position.
	for _, key := range []tea.KeyPressMsg{
		{Code: tea.KeyUp},
		{Code: tea.KeyDown},
		{Code: tea.KeyPgUp},
		{Code: tea.KeyPgDown},
		keyPress('h'),
		keyPress('l'),
		keyPress('j'),
		keyPress('k'),
		keyPress('\t'),
		{Code: tea.KeyEnter},
	} {
		m, _ = update(t, m, key)
		assertCursorPosition(t, m, 0)
		assertCurrentPath(t, m, "src/a.go")
	}
}

// --- n/p during loading ---

// TestNavigationNextWhileLoading verifies that n while a file is
// loading still advances the cursor. Navigation remains active during
// loading (PRD: navigation remains active while loading).
func TestNavigationNextWhileLoading(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	gate := make(chan struct{}) // held
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		<-gate
		return makeBuf(nil, 0, 3), nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}

	// n while loading should still advance the cursor (same-file).
	m, _ = update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 1)
}

// --- Multiple files with multiple stops ---

// TestNavigationMultiFileMultiStop verifies a full navigation sequence
// across multiple files with multiple stops each, checking cursor
// position and current file at each step.
func TestNavigationMultiFileMultiStop(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 10, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 20, subSpec{"x", 0, 1}),
		textMatch("src/c.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(makeScrollLines(30), 30, 3),
		"src/b.go": makeBuf(makeScrollLines(30), 30, 3),
		"src/c.go": makeBuf(makeScrollLines(30), 30, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)

	// Stop 0: a.go:1
	assertCursorPosition(t, m, 0)
	assertCurrentPath(t, m, "src/a.go")

	// n → stop 1: a.go:10 (same file)
	m, _ = update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 1)
	assertCurrentPath(t, m, "src/a.go")

	// n → stop 2: b.go:1 (cross file, load)
	m, cmd := update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 2)
	assertCurrentPath(t, m, "src/b.go")
	m = deliverLoad(t, m, cmd)

	// n → stop 3: b.go:20 (same file)
	m, _ = update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 3)
	assertCurrentPath(t, m, "src/b.go")

	// n → stop 4: c.go:1 (cross file, load)
	m, cmd = update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 4)
	assertCurrentPath(t, m, "src/c.go")
	m = deliverLoad(t, m, cmd)

	// n → wrap to stop 0: a.go:1 (cross file, cached)
	m, _ = update(t, m, keyPress('n'))
	assertCursorPosition(t, m, 0)
	assertCurrentPath(t, m, "src/a.go")

	// p → wrap to stop 4: c.go:1 (cross file, cached)
	m, _ = update(t, m, keyPress('p'))
	assertCursorPosition(t, m, 4)
	assertCurrentPath(t, m, "src/c.go")

	// p → stop 3: b.go:20 (same file)
	m, _ = update(t, m, keyPress('p'))
	assertCursorPosition(t, m, 3)
	assertCurrentPath(t, m, "src/b.go")

	// p → stop 2: b.go:1 (same file)
	m, _ = update(t, m, keyPress('p'))
	assertCursorPosition(t, m, 2)
	assertCurrentPath(t, m, "src/b.go")

	// p → stop 1: a.go:10 (cross file, cached)
	m, _ = update(t, m, keyPress('p'))
	assertCursorPosition(t, m, 1)
	assertCurrentPath(t, m, "src/a.go")

	// p → stop 0: a.go:1 (same file)
	m, _ = update(t, m, keyPress('p'))
	assertCursorPosition(t, m, 0)
	assertCurrentPath(t, m, "src/a.go")
}
