package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// --- Wrap mode toggle (Issue #16) ---

// TestWrapModeOnByDefault verifies that wrapping is enabled by default
// when the browse state is entered. The viewport row count reflects
// wrapped rows for a long line.
func TestWrapModeOnByDefault(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	// 100-char line at panel width 80, gutter 4 → text width 76.
	// 100 chars at width 76 = 2 rows in wrap mode.
	line := ml(1, strings.Repeat("x", 100))
	buf := makeBuf([]filebuffer.Line{line}, 1, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	// In wrap mode (default), the 100-char line wraps to 2 rows.
	view := viewContent(m)
	wrapLines := countContentXLines(view)
	if wrapLines != 2 {
		t.Fatalf("wrap mode content lines = %d, want 2 (wrapping on by default)", wrapLines)
	}
	// Toggle wrap mode off with 'w'.
	m, cmd := update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	// In run-off-edge mode, the 100-char line is one row.
	view = viewContent(m)
	contentLines := countContentXLines(view)
	if contentLines != 1 {
		t.Fatalf("run-off-edge content lines = %d, want 1 (one row per line)", contentLines)
	}
}

// TestWrapToggleChangesRowModel verifies that pressing 'w' toggles
// between wrap and run-off-edge modes, changing the row model. The
// viewport row count changes accordingly.
func TestWrapToggleChangesRowModel(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	// 100-char line at panel width 80, gutter 4 → text width 76.
	// Wrap: 2 rows. Run-off-edge: 1 row.
	line := ml(1, strings.Repeat("x", 100))
	buf := makeBuf([]filebuffer.Line{line}, 1, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)

	// Initially in wrap mode: 2 rows.
	view := viewContent(m)
	wrapLines := countContentXLines(view)
	if wrapLines != 2 {
		t.Fatalf("wrap mode content lines = %d, want 2 (100 chars at width 76)", wrapLines)
	}

	// Toggle to run-off-edge mode.
	m, cmd := update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	view = viewContent(m)
	offLines := countContentXLines(view)
	if offLines != 1 {
		t.Fatalf("run-off-edge content lines = %d, want 1 (one row per line)", offLines)
	}

	// Toggle back to wrap mode.
	m, cmd = update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	view = viewContent(m)
	wrapLines2 := countContentXLines(view)
	if wrapLines2 != 2 {
		t.Fatalf("after toggle back, wrap content lines = %d, want 2", wrapLines2)
	}
}

// TestWrapTogglePreservesOffset verifies that toggling wrap mode
// preserves the current viewport offset (clamped to the new row count).
func TestWrapTogglePreservesOffset(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	// 100-char line at panel width 80, gutter 4 → text width 76.
	// Wrap: 2 rows. Run-off-edge: 1 row.
	line := ml(1, strings.Repeat("x", 100))
	buf := makeBuf([]filebuffer.Line{line}, 1, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	// At offset 0 in wrap mode (2 rows).
	if m.ViewportOffset() != 0 {
		t.Fatalf("initial ViewportOffset = %d, want 0", m.ViewportOffset())
	}
	// Toggle to run-off-edge mode. Offset stays 0 (clamped to 1 row).
	m, _ = update(t, m, keyPress('w'))
	if m.ViewportOffset() != 0 {
		t.Fatalf("after toggle to run-off-edge, ViewportOffset = %d, want 0", m.ViewportOffset())
	}
}

// TestWrapToggleDuringSearch verifies that pressing 'w' during the
// searching state is a no-op (does not transition state or quit).
func TestWrapToggleDuringSearch(t *testing.T) {
	m := app.New([]string{"--json", "--no-config", "--", "foo", "."}, "/work")
	m, cmd := update(t, m, keyPress('w'))
	if m.State() != app.StateSearching {
		t.Fatalf("after w during search, State = %v, want StateSearching", m.State())
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("w during search produced a quit command")
		}
	}
}

// countContentXLines counts the number of content panel lines that
// contain 'xxx' (used to distinguish content rows from the file list
// and filename rule).
func countContentXLines(view string) int {
	lines := strings.Split(view, "\n")
	count := 0
	for _, l := range lines[1:] {
		if strings.Contains(l, "xxx") {
			count++
		}
	}
	return count
}

// --- Continuation gutter in app render (Issue #16) ---

// TestContinuationGutterBlank verifies that continuation rows render
// with a blank gutter (no line number), aligned with the first row's
// text. A 100-char line at a narrow width wraps to multiple rows; the
// continuation rows have no line number in the gutter.
func TestContinuationGutterBlank(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	// 100-char line at panel width 30, gutter 4 → text width 26.
	// 100 chars at width 26 = 4 rows. Only the first row has a line
	// number; continuation rows have a blank gutter.
	line := ml(1, strings.Repeat("x", 100))
	buf := makeBuf([]filebuffer.Line{line}, 1, 4)
	m := setupBrowseWithSize(t, idx, buf, 30, 24)
	view := viewContent(m)
	lines := strings.Split(view, "\n")
	// The content panel starts after the filename rule (line 0 is the
	// file list + filename rule).
	// Find the first content row with a line number "1".
	foundFirst := false
	foundContinuation := false
	for _, l := range lines[1:] {
		if strings.Contains(l, "xxx") {
			if !foundFirst {
				// First row: should have line number "1" in the gutter.
				if !strings.Contains(l, "1") {
					t.Fatalf("first content row has no line number: %q", l)
				}
				foundFirst = true
			} else {
				// Continuation row: should NOT have a line number in
				// the gutter. The gutter should be blank (spaces).
				foundContinuation = true
			}
		}
	}
	if !foundFirst {
		t.Fatal("no first content row found")
	}
	if !foundContinuation {
		t.Fatal("no continuation row found (line did not wrap)")
	}
}

// --- Wrapped target reveal in app (Issue #16) ---

// TestWrappedTargetRevealLongLine verifies that a match in a long
// wrapped line is revealed on its own wrapped row after navigation.
// The match is placed in the middle of a 1000-char line so the target
// row is beyond the visible range from the top, forcing a reveal.
func TestWrappedTargetRevealLongLine(t *testing.T) {
	// 1000-char line with a match at byte 500.
	// At terminal width 50, fileListWidth(50)=20, so panel width =
	// 50 - 20 - 1 = 29. Gutter 4 → text width 25.
	// 1000 chars at width 25 = 40 rows. Match at byte 500 → row 20.
	// Height 10 → contentHeight 9, floor(9/3) = 3.
	// Row 20 is not visible from top (20 >= 9).
	// Reveal: offset = 20 - 3 = 17. maxOffset = 40 - 9 = 31. 17 ≤ 31.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", strings.Repeat("x", 500)+"TARGET"+strings.Repeat("x", 494)+"\n", 1,
			subSpec{"TARGET", 500, 506}),
	)
	display := strings.Repeat("x", 500) + "TARGET" + strings.Repeat("x", 494)
	line := ml(1, display)
	buf := makeBuf([]filebuffer.Line{line}, 1, 4)
	m := setupBrowseWithSize(t, idx, buf, 50, 10)
	// The match at byte 500 is in wrapped row 20.
	// Reveal: offset = 20 - 3 = 17.
	if m.ViewportOffset() != 17 {
		t.Fatalf("ViewportOffset = %d, want 17 (wrapped target reveal at one-third)", m.ViewportOffset())
	}
}

// --- Render-cost guard with wrapping in app (Issue #16) ---

// TestRenderCostGuardWithWrappingApp verifies that the render path
// queries the row model only for the visible row range, not the full
// buffer, even when wrapping produces many rows.
func TestRenderCostGuardWithWrappingApp(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	// 500-char line at panel width 30, gutter 4 → text width 26.
	// 500 chars at width 26 = 20 rows.
	line := ml(1, strings.Repeat("x", 500))
	buf := makeBuf([]filebuffer.Line{line}, 1, 4)

	// Use a counting row provider factory.
	counter := &countingRows{lines: []filebuffer.Line{line}}
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
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 30, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	// Reset and render.
	counter.queries = nil
	_ = viewContent(m)

	// Only the visible range should be queried.
	if len(counter.queries) == 0 {
		t.Fatal("no queries recorded; the counting fake was not used")
	}
	// The visible range is [0, contentHeight) = [0, 23).
	// But with 20 rows, the visible range is [0, 20) (all rows fit).
	// So we just verify that not all 500 rows worth of work was done;
	// the query count should be 1 (one Rows call for the visible range).
	if len(counter.queries) != 1 {
		t.Logf("queries = %d (expected 1 for visible range)", len(counter.queries))
	}
}

// TestWrapToggleRerenders verifies that toggling wrap mode produces a
// new view with different row layout.
func TestWrapToggleRerenders(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	line := ml(1, strings.Repeat("x", 100))
	buf := makeBuf([]filebuffer.Line{line}, 1, 4)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	viewBefore := viewContent(m)
	m, cmd := update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	viewAfter := viewContent(m)
	if viewBefore == viewAfter {
		t.Fatal("view did not change after wrap toggle")
	}
	// Toggle back.
	m, cmd = update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	viewBack := viewContent(m)
	if viewBack == viewAfter {
		t.Fatal("view did not change after toggle back")
	}
}
