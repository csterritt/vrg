package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// revealFile is the horizontal-reveal fixture: a near match on line
// 1 and a far match on line 2 of a 300-cell file, so n steps from a
// visible match to one that needs a right-edge reveal. The far match
// also gives the current line a match hidden right — the reserved
// indicator column's '*' — once the cursor lands on it.
func revealFile() navFile {
	return navFile{
		name:    "a.txt",
		content: strings.Repeat("a", 300) + "\n" + strings.Repeat("b", 300) + "\n",
		stops: []navStop{
			{line: 1, start: 5, end: 6},
			{line: 2, start: 250, end: 251},
		},
	}
}

// textWidthFor returns the expected text width for the model's
// current layout state (Issue #38): the terminal width minus the
// actual file list width, minus the one-cell separator, minus the
// buffer gutter, minus the reserved right-indicator width for the
// current wrap mode (one cell in run-off-edge, zero in wrap). This
// mirrors the layout-key TextWidth chain — terminal width, panel
// width, text width — so a regression that sizes the panel from the
// raw terminal width, or that omits the list, separator, gutter, or
// indicator reservation, fails the dependent assertions.
func textWidthFor(t *testing.T, m *model, termWidth, gutterWidth int) int {
	t.Helper()
	tw := termWidth - m.listWidthFor(gutterWidth) - 1 - gutterWidth - viewport.ReservedIndicator(m.wrap)
	if tw < 1 {
		tw = 1
	}
	return tw
}

// The layout key's text width is panel-derived — terminal width
// minus the list width, the one-cell separator, the buffer gutter,
// and the reserved right indicator — never the raw terminal width
// (Issue #38).
func TestPanelDerivedTextWidth(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{revealFile()})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)
	if m.listWidth() == 0 {
		t.Skip("fixture list computed zero width — no list width to subtract")
	}
	gw := m.bufs[key].GutterWidth()
	want := textWidthFor(t, m, m.width, gw)
	if got := m.rows[key].(*viewport.Rows).Key().TextWidth; got != want {
		t.Fatalf("layout key TextWidth = %d, want %d (terminal − list − separator − gutter − indicator)", got, want)
	}
}

// Same-file n applies the horizontal reveal against the text width:
// a match at cell 250 reveals at offset 250 + 1 − text width, where
// the text width is terminal − list − separator − gutter − reserved
// (Issue #38) — not the full terminal width.
func TestSameFileRevealUsesPanelDerivedTextWidth(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{revealFile()})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	if got := m.vps[key].Off(); got != 0 {
		t.Fatalf("startup off = %d, want 0 (first match visible)", got)
	}
	m.Update(keyN)
	tw := textWidthFor(t, m, m.width, 3)
	if got := m.vps[key].Off(); got != 250+1-tw {
		t.Fatalf("after n, off = %d, want %d (reveal at text width %d)", got, 250+1-tw, tw)
	}
}

// With the file list hidden the horizontal reveal measures against
// the wider panel — terminal width minus zero list width minus the
// separator, gutter, and reserved indicator — not the raw terminal
// width (Issue #38). The panel width follows the current layout
// state.
func TestListHiddenHorizontalReveal(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{revealFile()})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	if got := m.vps[key].Off(); got != 0 {
		t.Fatalf("startup off = %d, want 0 (first match visible)", got)
	}
	// Hide the file list; the layout is rebuilt at the wider panel.
	_, lc := m.Update(keyLeft)
	deliverLayout(t, m, lc)
	if m.listWidth() != 0 {
		t.Fatalf("after left, list width = %d, want 0 (list hidden)", m.listWidth())
	}
	tw := textWidthFor(t, m, m.width, 3)
	if got := m.rows[key].(*viewport.Rows).Key().TextWidth; got != tw {
		t.Fatalf("after list hide, layout key TextWidth = %d, want %d (terminal − separator − gutter − indicator)", got, tw)
	}
	m.Update(keyN)
	if got := m.vps[key].Off(); got != 250+1-tw {
		t.Fatalf("after n with list hidden, off = %d, want %d (reveal at hidden-list text width %d)", got, 250+1-tw, tw)
	}
}

// Wrap mode reserves no right-indicator column: the layout key's
// text width is the panel width minus the gutter only (Issue #38).
// Toggling to run-off-edge re-measures with the one-cell
// reservation, and horizontal behaviour follows the run-off-edge
// text width.
func TestWrapModeZeroReservedIndicator(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{revealFile()})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)
	if got := viewport.ReservedIndicator(m.wrap); got != 0 {
		t.Fatalf("ReservedIndicator(wrap) = %d, want 0", got)
	}
	wrapTW := textWidthFor(t, m, m.width, 3)
	if got := m.rows[key].(*viewport.Rows).Key().TextWidth; got != wrapTW {
		t.Fatalf("wrap mode layout key TextWidth = %d, want %d (terminal − list − separator − gutter, no indicator)", got, wrapTW)
	}
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	tw := textWidthFor(t, m, m.width, 3)
	if tw != wrapTW-1 {
		t.Fatalf("run-off-edge text width = %d, want %d (wrap width minus reserved indicator)", tw, wrapTW-1)
	}
	// The line-1 match at cell 5 stays visible; n to cell 250
	// reveals right-edge at offset 250 + 1 − tw.
	m.Update(keyN)
	if got := m.vps[key].Off(); got != 250+1-tw {
		t.Fatalf("after w then n, off = %d, want %d (reveal at run-off-edge text width %d)", got, 250+1-tw, tw)
	}
}

// A terminal resize re-measures the layout against the new panel
// width: the reveal after the resize is computed from the new text
// width — not the old one, and not the raw terminal width (Issue
// #38).
func TestResizeRemeasuresTextWidth(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{revealFile()})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	_, rc := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	deliverLayout(t, m, rc)
	tw := textWidthFor(t, m, 100, 3)
	if got := m.rows[key].(*viewport.Rows).Key().TextWidth; got != tw {
		t.Fatalf("after resize, layout key TextWidth = %d, want %d (new panel width chain)", got, tw)
	}
	m.Update(keyN)
	if got := m.vps[key].Off(); got != 250+1-tw {
		t.Fatalf("after resize + n, off = %d, want %d (reveal at resized text width %d)", got, 250+1-tw, tw)
	}
}

// Navigating to a cached file whose installed layout already matches
// the current parameters is the immediate path: no new preparation
// is minted, and the reveal runs against the installed layout key's
// panel-derived text width (Issue #38).
func TestCacheHitRevealUsesInstalledTextWidth(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	m.theme = theme.Plain()
	// One stop per file so n/p cross directly between a.txt and
	// b.txt; b.txt's match at cell 250 needs a right-edge reveal.
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: strings.Repeat("a", 300) + "\n", stops: []navStop{{line: 1, start: 5, end: 6}}},
		{name: "b.txt", content: strings.Repeat("c", 300) + "\n", stops: []navStop{{line: 1, start: 250, end: 251}}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	keyB := string(idx.Files[1].Path)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	// Visit B and return so its layout installs under the current
	// parameters; then the re-crossing is the cache-hit path.
	m.Update(keyN)
	buf, err := filebuffer.Load(idx.Files[1].Path, idx.Files[1].Stops)
	if err != nil {
		t.Fatal(err)
	}
	injectLoad(t, m, fileLoadedMsg{path: idx.Files[1].Path, buf: buf})
	m.Update(keyP)
	// n back to B: its installed layout is fresh — the fast path
	// issues no request — and the far match reveals at the text
	// width.
	m.Update(keyN)
	if _, ok := m.layoutReqs[keyB]; ok {
		t.Fatal("the fresh-layout cache hit issued a redundant layout request")
	}
	tw := textWidthFor(t, m, m.width, 3)
	if got := m.vps[keyB].Off(); got != 250+1-tw {
		t.Fatalf("after cache-hit n to B, off = %d, want %d (reveal at installed text width %d)", got, 250+1-tw, tw)
	}
}

// In run-off-edge mode no composed row exceeds the terminal width,
// the separator column between the list and the panel is blank, and
// the reserved right-indicator cell sits at the panel's right edge
// outside the text area (Issue #38). The composed row is list width
// + separator + gutter + text width + one reserved indicator cell =
// terminal width.
func TestComposedViewRowsFitTerminal(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	// Line 1 carries the cursor's match at cell 5 and a second match
	// at cell 290 — entirely hidden right of the window, so its row
	// draws the reserved right star. Line 2 has no hidden-right
	// match: its reserved cell is blank.
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("a", 300) + "\nbb\n",
		stops: []navStop{
			{line: 1, start: 5, end: 6},
			{line: 1, start: 290, end: 291},
		},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	listW := m.listWidth()
	tw := textWidthFor(t, m, m.width, 3)

	for i := 1; i < m.height; i++ {
		if w := safepresentation.CellWidth(frameRow(t, m, i)); w > m.width {
			t.Fatalf("row %d is %d cells, exceeds terminal width %d: %q", i, w, m.width, frameRow(t, m, i))
		}
	}
	// The separator cell between the list and the panel is blank;
	// the reserved indicator cell is the last panel cell.
	ind := listW + 1 + 3 + tw
	if ind != m.width-1 {
		t.Fatalf("indicator cell = %d, want %d (panel right edge at terminal width)", ind, m.width-1)
	}
	row1 := []rune(frameRow(t, m, 1))
	if len(row1) != m.width {
		t.Fatalf("line 1 content row is %d cells, want %d: %q", len(row1), m.width, row1)
	}
	if row1[listW] != ' ' {
		t.Fatalf("line 1 cell %d = %q, want ' ' (the list–panel separator)", listW, row1[listW])
	}
	if row1[ind] != '*' {
		t.Fatalf("line 1 cell %d = %q, want '*' (right hidden-content indicator at the panel edge)", ind, row1[ind])
	}
	if row1[ind-1] != 'a' {
		t.Fatalf("line 1 cell %d = %q, want 'a' (last text-area cell; the indicator sits outside the text area)", ind-1, row1[ind-1])
	}
	row2 := []rune(frameRow(t, m, 2))
	if len(row2) != m.width {
		t.Fatalf("line 2 content row is %d cells, want %d: %q", len(row2), m.width, row2)
	}
	if row2[ind] != ' ' {
		t.Fatalf("line 2 cell %d = %q, want ' ' (reserved column blank — no hidden-right match)", ind, row2[ind])
	}
}
