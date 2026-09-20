package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/clipperhouse/displaywidth"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// textAreaW is the expected text width under the model's current
// layout state: the terminal width minus the actual file-list column,
// minus the buffer's line-number gutter, minus the run-off-edge
// reserved right-indicator column — zero in wrap mode, one otherwise.
// This layout has no separator cell: the panel begins at the list
// column's right edge. Computing every expected reveal, clip, and
// indicator position through this chain means a regression that omits
// the list width, the gutter, or the indicator reservation fails.
func textAreaW(t *testing.T, m Model) int {
	t.Helper()
	w, _ := m.termSize()
	tw := w - m.listWidth(w) - m.gutterWidth()
	if !m.wrap {
		tw--
	}
	return tw
}

// The installed layout's text width is the panel width minus the
// gutter and the mode's indicator reservation — never the raw terminal
// width — and every layout-affecting change re-measures it: wrap
// toggle, file-list hide/show, and resize.
func TestInstalledTextWidthTracksPanelGeometry(t *testing.T) {
	m := browseLoaded(t, "a.txt", numberedContent(20)+strings.Repeat("x", 200)+"\n", 80, 24)
	check := func(what string) {
		t.Helper()
		key := m.buffers["a.txt"].Key()
		if want := textAreaW(t, m); key.TextWidth != want {
			t.Fatalf("%s: installed text width = %d, want %d — terminal minus list, gutter, and indicator reservation",
				what, key.TextWidth, want)
		}
	}
	check("wrap mode, list shown")

	m, cmd := update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	check("run-off-edge mode, list shown")

	m, cmd = update(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	m = deliverCmd(t, m, cmd)
	check("run-off-edge mode, list hidden")

	m, cmd = update(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	m = deliverCmd(t, m, cmd)
	check("run-off-edge mode, list shown again")

	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	check("wrap mode, list shown again")

	m, cmd = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 24})
	m = deliverCmd(t, m, cmd)
	check("resized to 60")
}

// The minimal horizontal reveal is measured against the text width: a
// target hidden right of the text area moves the offset by exactly the
// columns that paint its start cell at the text area's right edge —
// the last cell before the reserved indicator column — never a
// terminal-derived width.
func TestHorizontalRevealMeasuresTextArea(t *testing.T) {
	dir := t.TempDir()
	line := strings.Repeat("x", 100) + "hit"
	writeMatchFile(t, dir, "a.txt", line+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", line+"\n", 1, 100, 103, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	// Enter run-off-edge mode before the load lands so the pending
	// startup reveal commits against the unwrapped layout.
	m, _ = update(t, m, keyMsg("w"))
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	tw := textAreaW(t, m)
	want := 100 + 1 - tw
	if got := m.vps["a.txt"].Offset(); got != want {
		t.Fatalf("startup reveal offset = %d, want %d (target column 100 + width 1 − text width %d)",
			got, want, tw)
	}
	// The match's start cell paints at the text area's right edge:
	// cell w−2, followed only by the blank reserved column at w−1.
	w, _ := m.termSize()
	row := []rune(contentRow(t, m))
	if len(row) != w {
		t.Fatalf("composed row width = %d, want the terminal width %d: %q", len(row), w, row)
	}
	if row[w-2] != 'h' || row[w-1] != ' ' {
		t.Fatalf("row's last two cells = %q, want the target cell at the text edge then the blank reserved column",
			string(row[w-2:]))
	}
}

// With the file list hidden the panel is the whole terminal width: the
// reveal re-measures against the larger text area, and the reserved
// indicator column stays at the panel's — now the screen's — right
// edge.
func TestHiddenListRevealMeasuresFullPanel(t *testing.T) {
	dir := t.TempDir()
	line := strings.Repeat("x", 100) + "hit"
	writeMatchFile(t, dir, "a.txt", line+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", line+"\n", 1, 100, 103, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	// Hide the list and enter run-off-edge mode before the load lands
	// so the startup reveal commits against the full-width panel.
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	m, _ = update(t, m, keyMsg("w"))
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	if m.listVisible {
		t.Fatal("the file list is still visible")
	}
	tw := textAreaW(t, m) // 80 − 0 − gutter 3 − reserved 1 = 76
	want := 100 + 1 - tw
	if got := m.vps["a.txt"].Offset(); got != want {
		t.Fatalf("hidden-list reveal offset = %d, want %d (text width %d)", got, want, tw)
	}
	w, _ := m.termSize()
	row := []rune(contentRow(t, m))
	if len(row) != w {
		t.Fatalf("composed row width = %d, want the terminal width %d: %q", len(row), w, row)
	}
	if row[w-2] != 'h' || row[w-1] != ' ' {
		t.Fatalf("row's last two cells = %q, want the target cell at the text edge then the blank reserved column",
			string(row[w-2:]))
	}
}

// Wrap mode reserves no indicator column: the text area runs to the
// panel's right edge, so a long line's first row paints text width
// cells — panel minus gutter exactly — and the row's last cell is
// text, not a reserved blank.
func TestWrapModeReservesNoIndicatorColumn(t *testing.T) {
	m := browseLoaded(t, "a.txt", strings.Repeat("x", 200)+"\n", 80, 24)
	tw := textAreaW(t, m) // 80 − list 7 − gutter 3 = 70, no reservation
	if want := m.panelWidth() - m.gutterWidth(); tw != want {
		t.Fatalf("wrap text width = %d, want %d (panel %d − gutter, zero reservation)",
			tw, want, m.panelWidth())
	}
	if got := m.buffers["a.txt"].Key().TextWidth; got != tw {
		t.Fatalf("installed wrap-mode text width = %d, want %d", got, tw)
	}
	row := viewRow(t, m, 1)
	if n := strings.Count(row, "x"); n != tw {
		t.Fatalf("first wrapped row shows %d x's, want the text width %d", n, tw)
	}
	if last := []rune(row)[len([]rune(row))-1]; last != 'x' {
		t.Fatalf("wrapped row's last cell = %q, want text — wrap mode has no reserved column", last)
	}
}

// A width-changing resize re-measures the text area: the replacement
// layout's key carries the new text width and the next reveal pans
// against it — the viewport never retains a pre-resize measurement.
func TestResizeRemeasuresTextWidth(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 100) + "hit"
	writeMatchFile(t, dir, "a.txt", "ok\n"+long+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "ok\n", 1, 0, 2, "ok"))
	addRec(t, idx, matchRec("a.txt", long+"\n", 2, 100, 103, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m.theme = theme.Plain()

	m, _ = update(t, m, keyMsg("n")) // → line 2's column-100 target
	if got, want := m.vps["a.txt"].Offset(), 101-textAreaW(t, m); got != want {
		t.Fatalf("pre-resize reveal offset = %d, want %d", got, want)
	}

	m, cmd = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 24})
	m = deliverCmd(t, m, cmd)
	tw := textAreaW(t, m) // 60 − list 7 − gutter 3 − reserved 1 = 49
	if got := m.buffers["a.txt"].Key().TextWidth; got != tw {
		t.Fatalf("installed text width after resize = %d, want %d", got, tw)
	}

	// The next reveal measures against the resized text area: n wraps
	// to line 1's column-0 target (dropping the offset to 0), then n
	// again re-reveals line 2's target at the new width.
	m, _ = update(t, m, keyMsg("n"))
	if got := m.vps["a.txt"].Offset(); got != 0 {
		t.Fatalf("n to the left-edge target: offset = %d, want 0", got)
	}
	m, _ = update(t, m, keyMsg("n"))
	if got, want := m.vps["a.txt"].Offset(), 101-tw; got != want {
		t.Fatalf("post-resize reveal offset = %d, want %d (text width %d)", got, want, tw)
	}
}

// The composed frame never exceeds the terminal width and the
// run-off-edge reserved indicator column is the panel's last cell —
// outside the text area — in both list states.
func TestComposedRowsFitTerminalAndReserveRightEdge(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 100)
	writeMatchFile(t, dir, "a.txt", strings.Repeat(long+"\n", 30))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 0, 4, "xxxx"))    // hidden left once panned
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 90, 95, "xxxxx")) // hidden right
	idx.Finish()

	checkWidths := func(m Model, what string) {
		t.Helper()
		w, _ := m.termSize()
		for r, row := range strings.Split(m.View().Content, "\n") {
			if d := displaywidth.String(row); d > w {
				t.Fatalf("%s: row %d is %d cells, exceeding the terminal width %d: %q", what, r, d, w, row)
			}
		}
	}

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m, _ = update(t, m, keyMsg(">")) // offset 10: both indicators live on line 1
	m.theme = theme.Plain()

	checkWidths(m, "list shown, run-off-edge")
	w, _ := m.termSize()
	lw, gw, tw := m.listWidth(w), m.gutterWidth(), textAreaW(t, m)
	if lw+gw+tw+1 != w {
		t.Fatalf("list %d + gutter %d + text %d + reserved 1 = %d, want the terminal width %d",
			lw, gw, tw, lw+gw+tw+1, w)
	}
	row := []rune(viewRow(t, m, 1))
	// The current line's gutter indicator and reserved column both
	// carry *: the reserved cell is the panel's last cell, one column
	// right of the text area's last cell.
	if row[lw+gw-2] != '*' {
		t.Fatalf("gutter indicator cell %d = %q, want *", lw+gw-2, row[lw+gw-2])
	}
	if row[lw+gw+tw-1] != 'x' {
		t.Fatalf("last text cell %d = %q, want text — the reserved column must not overwrite it",
			lw+gw+tw-1, row[lw+gw+tw-1])
	}
	if row[w-1] != '*' {
		t.Fatalf("reserved column cell %d = %q, want *", w-1, row[w-1])
	}
	// A non-current line hides only text: gutter _, reserved blank.
	row = []rune(viewRow(t, m, 2))
	if row[lw+gw-2] != '_' || row[w-1] != ' ' {
		t.Fatalf("non-current line indicators = %q/%q, want _/blank", row[lw+gw-2], row[w-1])
	}

	m, cmd = update(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	m = deliverCmd(t, m, cmd)
	checkWidths(m, "list hidden, run-off-edge")
	lw, gw, tw = m.listWidth(w), m.gutterWidth(), textAreaW(t, m)
	if lw != 0 || gw+tw+1 != w {
		t.Fatalf("hidden-list geometry: list %d, gutter %d + text %d + reserved 1 = %d, want %d",
			lw, gw, tw, gw+tw+1, w)
	}
	row = []rune(viewRow(t, m, 1))
	if row[w-1] != '*' {
		t.Fatalf("hidden-list reserved column = %q, want * at the panel's right edge", row[w-1])
	}
}
