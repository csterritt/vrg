package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// numberedContent returns n lines line-01, line-02, …, each one row
// tall while the panel is unwrapped.
func numberedContent(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line-%02d\n", i)
	}
	return b.String()
}

// browseLoaded enters the browse state with one file loaded and
// displayed at the given terminal size.
func browseLoaded(t *testing.T, name, content string, w, h int) Model {
	t.Helper()
	dir := t.TempDir()
	writeMatchFile(t, dir, name, content)
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec(name, strings.SplitN(content, "\n", 2)[0]+"\n", 1, 0, 4, "line"))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	m, _ = update(t, m, cmd())
	// The no-style theme keeps assertions free of the ANSI codes the
	// match highlight inserts inside the fixture's "line-NN" text.
	m.theme = theme.Plain()
	return m
}

// contentRow returns the panel's first content row — screen row 1,
// below the filename rule — from the composed view.
func contentRow(t *testing.T, m Model) string {
	t.Helper()
	rows := strings.SplitN(m.View().Content, "\n", 3)
	if len(rows) < 2 {
		t.Fatalf("view has no content row:\n%s", m.View().Content)
	}
	return rows[1]
}

// The scroll keys move the panel by their rendered-row units: one row
// for up/down, half a page for u/d, and a full content-height page for
// page up/page down.
func TestScrollKeysMoveContent(t *testing.T) {
	m := browseLoaded(t, "a.txt", numberedContent(100), 80, 24) // content height 23

	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if row := contentRow(t, m); !strings.Contains(row, "line-02") {
		t.Fatalf("after down, first content row = %q, want line-02", row)
	}
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if row := contentRow(t, m); !strings.Contains(row, "line-01") {
		t.Fatalf("after up, first content row = %q, want line-01", row)
	}

	m, _ = update(t, m, keyMsg("d")) // half page: max(1, 23/2) = 11
	if row := contentRow(t, m); !strings.Contains(row, "line-12") {
		t.Fatalf("after d, first content row = %q, want line-12", row)
	}
	m, _ = update(t, m, keyMsg("u"))
	if row := contentRow(t, m); !strings.Contains(row, "line-01") {
		t.Fatalf("after u, first content row = %q, want line-01", row)
	}

	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown}) // full page: 23
	if row := contentRow(t, m); !strings.Contains(row, "line-24") {
		t.Fatalf("after pgdn, first content row = %q, want line-24", row)
	}
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgUp})
	if row := contentRow(t, m); !strings.Contains(row, "line-01") {
		t.Fatalf("after pgup, first content row = %q, want line-01", row)
	}
}

// Scrolling stops with the last rendered row at the bottom of the
// panel: repeated page downs cannot push avoidable blank rows below
// EOF, and up at the top is a no-op.
func TestScrollKeysClampAtEOF(t *testing.T) {
	m := browseLoaded(t, "a.txt", numberedContent(30), 80, 24) // max top 30-23 = 7

	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	if row := contentRow(t, m); !strings.Contains(row, "line-08") {
		t.Fatalf("after pgdn, first content row = %q, want line-08", row)
	}
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	if row := contentRow(t, m); !strings.Contains(row, "line-08") {
		t.Fatalf("second pgdn scrolled past EOF: first content row = %q", row)
	}
	v := m.View().Content
	if !strings.Contains(v, "line-30") {
		t.Fatalf("clamped view lacks the last line at the bottom:\n%s", v)
	}
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-01") {
		t.Fatalf("up past BOF scrolled: first content row = %q, want line-01", row)
	}
}

// Scroll keys on a "Loading…" placeholder — and equally on a failed
// "(unreadable)" placeholder — are no-ops: nothing moves and no command
// is returned.
func TestScrollKeysOnPlaceholderAreNoop(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(50))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-01\n", 1, 0, 4, "line"))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchResult{index: idx, integrity: completeStream})
	// The load command is deliberately never delivered.
	before := m.View().Content

	for _, key := range []tea.KeyPressMsg{
		{Code: tea.KeyUp},
		{Code: tea.KeyDown},
		{Code: tea.KeyPgUp},
		{Code: tea.KeyPgDown},
		keyMsg("u"),
		keyMsg("d"),
	} {
		var cmd tea.Cmd
		m, cmd = update(t, m, key)
		if cmd != nil {
			t.Fatalf("scroll key %q on the placeholder returned a command: %v", key.String(), cmd)
		}
	}
	if got := m.View().Content; got != before {
		t.Fatalf("scrolling the placeholder changed the view:\nbefore:\n%s\nafter:\n%s", before, got)
	}
	if !strings.Contains(before, "Loading…") {
		t.Fatalf("fixture lost its placeholder:\n%s", before)
	}
}

// The vertical viewport is saved per file: scrolling in one file is
// remembered and is the revisit's starting point, while a first visit
// starts at the top. The destination reveal leaves each saved top alone
// here because every stop's target row is visible inside it. Navigation
// between files goes through the n/p keys.
func TestVerticalViewportSavedPerFile(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(50))
	var b strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&b, "row-%02d\n", i)
	}
	writeMatchFile(t, dir, "b.txt", b.String())

	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-06\n", 6, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "row-04\n", 4, 0, 3, "row"))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m.popupTimer = instantPopupTimer
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	m, _ = update(t, m, cmd()) // a.txt loaded; target row 5 visible at top 0
	m.theme = theme.Plain()    // no ANSI inside the fixture's match text

	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-06") {
		t.Fatalf("a.txt scrolled to %q, want line-06", row)
	}

	// n switches to b.txt and requests its load; a first visit starts
	// at the top, and its target row 3 is visible there.
	m, cmd = update(t, m, keyMsg("n"))
	m, _ = update(t, m, deliverNavLoad(t, cmd))
	if row := contentRow(t, m); !strings.Contains(row, "row-01") {
		t.Fatalf("first visit to b.txt shows %q, want row-01 at the top", row)
	}
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if row := contentRow(t, m); !strings.Contains(row, "row-04") {
		t.Fatalf("b.txt scrolled to %q, want row-04", row)
	}

	// Revisiting a.txt starts from its saved top rather than restarting
	// at the top or inheriting b.txt's position; the target row 5 is
	// visible inside it, so the reveal does not scroll.
	m, cmd = update(t, m, keyMsg("p"))
	if msg := navLoadMsg(t, cmd); msg != nil {
		t.Fatalf("p to cached a.txt requested a load: %v", msg)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-06") {
		t.Fatalf("revisit to a.txt shows %q, want the saved top line-06", row)
	}
	m, _ = update(t, m, keyMsg("n"))
	if row := contentRow(t, m); !strings.Contains(row, "row-04") {
		t.Fatalf("revisit to b.txt shows %q, want the saved top row-04", row)
	}
}

// countingSource is a prepared-row provider that records every row a
// frame render queries, so the test can prove the render path reads
// only the visible range instead of scanning the whole buffer.
type countingSource struct {
	rows    int
	queried []int
}

func (c *countingSource) LineCount() int   { return c.rows }
func (c *countingSource) GutterWidth() int { return 6 }
func (c *countingSource) Cells(i int) []safepresentation.Cell {
	c.queried = append(c.queried, i)
	return []safepresentation.Cell{{Text: "x"}}
}
func (c *countingSource) Highlights(i int) []filebuffer.Span { return nil }
func (c *countingSource) TargetRow(stop searchindex.Stop) int {
	row := int(stop.Line) - 1
	if last := c.rows - 1; row > last {
		row = last
	}
	if row < 0 {
		row = 0
	}
	return row
}

// Rendering a frame for a huge buffer queries the row provider for the
// visible row range only — never O(file size) per frame.
func TestRenderQueriesOnlyVisibleRows(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "line\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line\n", 1, 0, 4, "line"))
	idx.Finish()

	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchResult{index: idx, integrity: completeStream})

	src := &countingSource{rows: 10000}
	m, _ = update(t, m, loadResult{path: []byte("a.txt"), buf: src})
	_ = m.View()

	contentH := 23 // height 24 minus the filename row
	if len(src.queried) != contentH {
		t.Fatalf("frame queried %d rows of a %d-row buffer, want the %d visible rows",
			len(src.queried), src.rows, contentH)
	}
	for i, q := range src.queried {
		if q != i {
			t.Fatalf("row query %d was rendered row %d, want the visible range 0..%d",
				i, q, contentH)
		}
	}
}
