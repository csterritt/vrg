package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
	"vrg/internal/theme"
)

var keyW = tea.KeyPressMsg{Text: "w", Code: 'w'}

// Wrapping is on at startup: a line longer than the text width spans
// several rendered rows whose continuation gutters are blank and whose
// text aligns with the first row's.
func TestWrapOnByDefaultBlankContinuationGutters(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	long := strings.Repeat("x", 200)
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: long + "\nsecond\n", line: 1, start: 0, end: 1},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)

	listW := safepresentation.CellWidth(escapedPath(idx.Files[0])) + 1
	textW := 80 - listW - 3 // panel minus the one-digit gutter
	if textW >= 200 {
		t.Fatalf("fixture line must exceed the text width %d", textW)
	}
	key := string(idx.Files[0].Path)
	wrapped := (200 + textW - 1) / textW // ceil
	if got := m.rows[key].Len(); got != wrapped+1 {
		t.Fatalf("rows = %d, want %d wrapped rows + the second line", got, wrapped+1)
	}
	// Row 1 is the line's first rendered row: numbered gutter, then a
	// full text-width band of x's.
	if row := frameRow(t, m, 1); !strings.Contains(row, "1  "+strings.Repeat("x", textW)) {
		t.Fatalf("first row = %q, want gutter then %d x's", row, textW)
	}
	// Row 2 continues the same line: a blank gutter the gutter's width,
	// then the next band aligned with the first row's text column.
	if row := frameRow(t, m, 2); row[:listW+3+textW] != strings.Repeat(" ", listW+3)+strings.Repeat("x", textW) {
		t.Fatalf("continuation row = %q, want a %d-cell blank gutter then text", row, 3)
	}
}

// w toggles wrap mode for the session: the same long line goes from
// many wrapped rows to a single clipped row — the rightmost panel
// column reserved blank — and back again.
func TestWTogglesWrapMode(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	long := strings.Repeat("x", 200)
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: long + "\nsecond\n", line: 1, start: 0, end: 1},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)
	listW := safepresentation.CellWidth(escapedPath(idx.Files[0])) + 1
	wrapped := m.rows[key].Len()
	if wrapped <= 2 {
		t.Fatalf("wrapped rows = %d, want the long line spanning several", wrapped)
	}

	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	if m.wrap {
		t.Fatal("w did not toggle wrap off")
	}
	if got := m.rows[key].Len(); got != 2 {
		t.Fatalf("run-off-edge rows = %d, want one per source line", got)
	}
	// The clipped row shows the first text-width cells and leaves the
	// reserved rightmost indicator column blank.
	textW := 80 - listW - 3 - 1
	if row := frameRow(t, m, 1); row[listW:] != "1  "+strings.Repeat("x", textW)+" " {
		t.Fatalf("run-off-edge row = %q, want %d clipped cells then a blank reserved column", row, textW)
	}
	// The next row is the second line immediately — no spillover.
	if row := frameRow(t, m, 2); !strings.Contains(row, "2  second") {
		t.Fatalf("row after the clipped line = %q, want line 2", row)
	}

	_, wc = m.Update(keyW)
	deliverLayout(t, m, wc)
	if !m.wrap {
		t.Fatal("second w did not toggle wrap back on")
	}
	if got := m.rows[key].Len(); got != wrapped {
		t.Fatalf("rows after toggling back = %d, want the wrapped %d", got, wrapped)
	}
}

// A match deep in a wrapped line reveals its own rendered row: after n
// the row holding the match start lands at floor(content height / 3)
// with a blank continuation gutter.
func TestRevealMatchDeepInWrappedLine(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	long := strings.Repeat("x", 1000) + "needle tail"
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: "match first\n" + strings.Repeat("pad\n", 5), line: 1, start: 0, end: 5},
		{name: "b.txt", content: long + "\n" + strings.Repeat("pad line\n", 30), line: 1, start: 1000, end: 1006},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)

	// n crosses into b.txt; the reveal runs when its load completes.
	_, nav := m.Update(keyN)
	finishLoad(t, m, nav)

	key := string(idx.Files[1].Path)
	listW := safepresentation.CellWidth(escapedPath(idx.Files[1])) + 1
	textW := 80 - listW - 4 // b.txt has 31 lines → two-digit gutter + 2
	targetRow := 1000 / textW
	wantTop := targetRow - contentRows24/3
	if wantTop < 0 {
		t.Fatalf("fixture text width %d puts the target too high to test", textW)
	}
	if got := m.vps[key].Top(); got != wantTop {
		t.Fatalf("top = %d, want %d — target row %d at index floor(23/3) = 7", got, wantTop, targetRow)
	}
	// The match sits on screen row 1 + 7 = 8, inverse and underlined,
	// behind a blank continuation gutter. The needle may straddle the
	// row boundary — the match start cell is what the row guarantees —
	// so the styled run is a non-empty prefix of "needle".
	row := frameRow(t, m, 8)
	i := strings.Index(row, "\x1b[30;47;4m")
	if i < 0 {
		t.Fatalf("row 8 = %q, want the current match inverse+underlined there", row)
	}
	rest := row[i+len("\x1b[30;47;4m"):]
	end := strings.Index(rest, "\x1b[")
	got := rest
	if end >= 0 {
		got = rest[:end]
	}
	if got == "" || !strings.HasPrefix("needle", got) {
		t.Fatalf("row 8 styled run = %q, want a prefix of %q", got, "needle")
	}
	if !strings.Contains(row, "\x1b[37;40m    \x1b[37;40;24m") {
		t.Fatalf("row 8 = %q, want a blank four-cell continuation gutter", row)
	}
}

// Resizing the terminal rebuilds the prepared row model at the new
// text width: a narrow frame wraps the same line into more rows, and
// the saved viewport re-clamps to the new row count.
func TestResizeRebuildsRowModel(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	long := strings.Repeat("x", 200)
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: long + "\nsecond\n", line: 2, start: 0, end: 6},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)
	before := m.rows[key].Len()

	_, rc := m.Update(tea.WindowSizeMsg{Width: 70, Height: 24})
	deliverLayout(t, m, rc)
	listW := safepresentation.CellWidth(escapedPath(idx.Files[0])) + 1
	if listW > 70 {
		listW = 70 // the list caps at the terminal width
	}
	textW := 70 - listW - 3
	if textW < 1 || textW >= 200 {
		t.Skipf("fixture path leaves a degenerate text width %d", textW)
	}
	want := (200+textW-1)/textW + 1
	if got := m.rows[key].Len(); got != want || got == before {
		t.Fatalf("rows after resize = %d, want %d wrapped rows at text width %d", got, want, textW)
	}
}
