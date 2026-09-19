package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

var (
	keyU    = tea.KeyPressMsg{Text: "u", Code: 'u'}
	keyD    = tea.KeyPressMsg{Text: "d", Code: 'd'}
	keyPgUp = tea.KeyPressMsg{Code: tea.KeyPgUp}
	keyPgDn = tea.KeyPressMsg{Code: tea.KeyPgDown}
)

// contentRows24 is the file-panel content height at 24 frame rows: the
// filename rule occupies row 0.
const contentRows24 = 23

// numberedFile is a fixture file of n lines "name line i" with the
// match on the first line.
func numberedFile(name string, n int) fixtureFile {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%s line %d\n", name, i)
	}
	return fixtureFile{name: name, content: b.String(), line: 1, start: 0, end: 1}
}

// frameRow returns the view's 0-based row.
func frameRow(t *testing.T, m *model, i int) string {
	t.Helper()
	rows := strings.Split(viewText(m), "\n")
	if i >= len(rows) {
		t.Fatalf("view has %d rows, no row %d", len(rows), i)
	}
	return rows[i]
}

// up/down, u/d, and pgup/pgdn move the current file's viewport by one
// rendered row, a half page — max(1, floor(h/2)) — and a full page of
// the content height (frame height minus the filename row).
func TestScrollKeysMoveRenderedRows(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := browseIndex(t, []fixtureFile{numberedFile("a.txt", 60)})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)

	steps := []struct {
		name string
		key  tea.KeyPressMsg
		want int
	}{
		{"down one row", keyDown, 1},
		{"up one row", keyUp, 0},
		{"d half page", keyD, contentRows24 / 2},
		{"u half page", keyU, 0},
		{"pgdn full page", keyPgDn, contentRows24},
		{"pgup full page", keyPgUp, 0},
	}
	for _, s := range steps {
		if _, cmd := m.Update(s.key); cmd != nil {
			t.Fatalf("%s returned a command, want none", s.name)
		}
		if got := m.vps[key].Top(); got != s.want {
			t.Fatalf("%s: top = %d, want %d", s.name, got, s.want)
		}
	}

	m.Update(keyPgDn)
	if row := frameRow(t, m, 1); !strings.Contains(row, "a.txt line 24") {
		t.Fatalf("first content row = %q, want %q at top %d", row, "a.txt line 24", contentRows24)
	}
}

// Scrolling past EOF stops at the last top leaving no avoidable blank
// rows: the file's last line sits at the panel's bottom row and further
// down keys change nothing.
func TestScrollStopsAtEOF(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := browseIndex(t, []fixtureFile{numberedFile("a.txt", 30)})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)

	const maxTop = 30 - contentRows24
	for _, k := range []tea.KeyPressMsg{keyPgDn, keyDown, keyD, keyPgDn} {
		m.Update(k)
		if got := m.vps[key].Top(); got != maxTop {
			t.Fatalf("top = %d, want clamped to %d", got, maxTop)
		}
	}
	if row := frameRow(t, m, 23); !strings.Contains(row, "a.txt line 30") {
		t.Fatalf("bottom content row = %q, want the file's last line there", row)
	}
}

// up at the top of the file does nothing.
func TestScrollStopsAtBOF(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain() // unstyled: matched text stays contiguous
	idx := browseIndex(t, []fixtureFile{numberedFile("a.txt", 60)})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)

	for _, k := range []tea.KeyPressMsg{keyUp, keyU, keyPgUp} {
		m.Update(k)
		if got := m.vps[key].Top(); got != 0 {
			t.Fatalf("top = %d at BOF, want 0", got)
		}
	}
	if row := frameRow(t, m, 1); !strings.Contains(row, "a.txt line 1") {
		t.Fatalf("first content row = %q, want the file's first line", row)
	}
}

// A file shorter than the viewport cannot scroll; the unused rows below
// its content are left naturally blank.
func TestScrollShortFileLeavesUnusedRows(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := browseIndex(t, []fixtureFile{numberedFile("a.txt", 5)})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)

	for _, k := range []tea.KeyPressMsg{keyDown, keyD, keyPgDn} {
		m.Update(k)
	}
	if got := m.vps[key].Top(); got != 0 {
		t.Fatalf("top = %d for a 5-line file in a 23-row panel, want 0", got)
	}
	if row := frameRow(t, m, 23); strings.TrimSpace(row) != "" {
		t.Fatalf("bottom row = %q, want blank — the short file leaves unused rows", row)
	}
}

// Scroll keys on the "Loading…" placeholder — and on the "(unreadable)"
// placeholder — are no-ops: no viewport state is created and the frame
// does not change.
func TestScrollKeysNoOpOnPlaceholders(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := browseIndex(t, []fixtureFile{numberedFile("a.txt", 60)})
	startBrowse(t, m, idx) // the load stays in flight

	loading := viewText(m)
	for _, k := range []tea.KeyPressMsg{keyDown, keyUp, keyD, keyU, keyPgDn, keyPgUp} {
		m.Update(k)
	}
	if len(m.vps) != 0 {
		t.Fatalf("scrolling Loading… created viewport state: %v", m.vps)
	}
	if v := viewText(m); v != loading || !strings.Contains(v, "Loading…") {
		t.Fatalf("view changed under scroll keys on Loading…: %q", v)
	}

	m.Update(fileLoadedMsg{path: idx.Files[0].Path, err: errors.New("denied")})
	unreadable := viewText(m)
	for _, k := range []tea.KeyPressMsg{keyDown, keyD, keyPgDn} {
		m.Update(k)
	}
	if len(m.vps) != 0 {
		t.Fatalf("scrolling (unreadable) created viewport state: %v", m.vps)
	}
	if v := viewText(m); v != unreadable || !strings.Contains(v, "(unreadable)") {
		t.Fatalf("view changed under scroll keys on (unreadable): %q", v)
	}
}

// Vertical viewport state is per file: scrolling file A is recorded
// under A's raw path and survives a leave-and-revisit, while file B
// keeps its own state. A's stop sits at line 5 so the revisit's
// destination reveal finds its target row visible from the saved top
// and leaves it (Issue #14).
func TestViewportStateSavedPerFile(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain() // unstyled: matched text stays contiguous
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: numberedContent("a.txt", 60), line: 5, start: 0, end: 1},
		numberedFile("b.txt", 60),
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	keyA := string(idx.Files[0].Path)

	m.Update(keyDown)
	m.Update(keyDown)
	m.Update(keyDown)
	if got := m.vps[keyA].Top(); got != 3 {
		t.Fatalf("file A top = %d, want 3", got)
	}

	// Issue #13 drives file changes through n/p: n crosses to file B's
	// stop and requests its load; the saved state is per raw path.
	buf, err := filebuffer.Load(idx.Files[1].Path, idx.Files[1].Stops)
	if err != nil {
		t.Fatal(err)
	}
	m.Update(keyN)
	injectLoad(t, m, fileLoadedMsg{path: idx.Files[1].Path, buf: buf})
	if got := m.vps[string(idx.Files[1].Path)].Top(); got != 0 {
		t.Fatalf("file B top = %d on its first visit, want 0", got)
	}
	if row := frameRow(t, m, 1); !strings.Contains(row, "b.txt line 1") {
		t.Fatalf("file B's first content row = %q, want its first line", row)
	}

	m.Update(keyP) // revisit A
	if got := m.vps[keyA].Top(); got != 3 {
		t.Fatalf("revisited file A top = %d, want the saved 3", got)
	}
	if row := frameRow(t, m, 1); !strings.Contains(row, "a.txt line 4") {
		t.Fatalf("revisited file A's first content row = %q, want line 4", row)
	}
}

// countingRows is the render-cost guard's fake row provider: it records
// every row index the frame render asks for.
type countingRows struct {
	n       int
	queries []int
}

func (c *countingRows) Len() int         { return c.n }
func (c *countingRows) GutterWidth() int { return 3 }

// Key reports the fake as a wrap-mode model so horizontal panning and
// its extent evaluation stay inert — the render-cost guard counts only
// the frame's own queries.
func (c *countingRows) Key() viewport.Key { return viewport.Key{Wrap: true} }

func (c *countingRows) At(i int) viewport.Row {
	c.queries = append(c.queries, i)
	return viewport.Row{Line: filebuffer.Line{Number: int64(i + 1)}}
}

func (c *countingRows) AnchorAt(row int) viewport.Anchor {
	return viewport.Anchor{Line: int64(row + 1)}
}

func (c *countingRows) RowOf(a viewport.Anchor) int {
	row := int(a.Line) - 1
	if row < 0 {
		row = 0
	}
	if row >= c.n {
		row = c.n - 1
	}
	return row
}

func (c *countingRows) TargetRow(st searchindex.Stop) int {
	row := int(st.Number) - 1
	if row < 0 {
		row = 0
	}
	if row >= c.n {
		row = c.n - 1
	}
	return row
}

// A frame render queries the row provider only for the visible row
// range — the prepared-row window [top, top + content height) — never
// scanning the whole buffer.
func TestRenderQueriesOnlyVisibleRows(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := browseIndex(t, []fixtureFile{numberedFile("a.txt", 60)})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)

	fake := &countingRows{n: 200}
	m.rows[string(idx.Files[0].Path)] = fake
	m.Update(keyPgDn) // top = 23, the content height
	viewText(m)

	if len(fake.queries) != contentRows24 {
		t.Fatalf("render queried %d rows of 200, want the %d visible ones",
			len(fake.queries), contentRows24)
	}
	for i, q := range fake.queries {
		if want := contentRows24 + i; q != want {
			t.Fatalf("query %d asked for row %d, want visible row %d", i, q, want)
		}
	}
}

// A resize that would leave avoidable blank rows below EOF pulls the
// saved top up to the new last valid position.
func TestResizeReclampsViewport(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := browseIndex(t, []fixtureFile{numberedFile("a.txt", 60)})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)

	m.Update(keyPgDn) // top = 23
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 50})
	const want = 60 - 49 // the new content height is 49
	if got := m.vps[key].Top(); got != want {
		t.Fatalf("after growing the frame top = %d, want clamped to %d", got, want)
	}
	if row := frameRow(t, m, 1); !strings.Contains(row, "a.txt line 12") {
		t.Fatalf("first content row = %q, want line 12", row)
	}
}
