package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// startBrowse drives a model into the browse state at the given
// terminal size and returns the model plus the startup file's load
// command.
func startBrowse(t *testing.T, dir string, idx *searchindex.Index, w, h int) (Model, tea.Cmd) {
	t.Helper()
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, dir)
	m.popupTimer = instantPopupTimer
	m, _ = update(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	return update(t, m, searchResult{index: idx, integrity: completeStream})
}

// rowWith returns the screen row containing sub, failing when none does.
func rowWith(t *testing.T, view, sub string) string {
	t.Helper()
	for _, r := range strings.Split(view, "\n") {
		if strings.Contains(r, sub) {
			return r
		}
	}
	t.Fatalf("no screen row contains %q:\n%s", sub, view)
	return ""
}

// Startup selects the first stop in path-then-line order: the first
// file's load is requested, its name rides the filename rule, its
// file-list entry is underlined, and the first matched line renders the
// underlined current-match style.
func TestStartupSelectsFirstStop(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "zero\nhit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("a.txt", "hit a\n", 2, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	if cmd == nil {
		t.Fatal("startup requested no file load")
	}
	msg := cmd()
	if lr, ok := msg.(loadResult); !ok || string(lr.path) != "a.txt" {
		t.Fatalf("startup load = %#v, want the a.txt load", msg)
	}
	m, _ = update(t, m, msg)

	v := m.View().Content
	if !strings.Contains(v, "── a.txt ") {
		t.Fatalf("filename rule does not name a.txt:\n%s", v)
	}
	if !strings.Contains(v, "\x1b[4ma.txt\x1b[24m") {
		t.Fatalf("a.txt list entry not underlined:\n%s", v)
	}
	if !strings.Contains(v, "\x1b[4m\x1b[30;47mhit\x1b[24m") {
		t.Fatalf("first matched line lacks the underlined current match:\n%s", v)
	}
}

// n within one file moves the current matched line — the underlined
// match style — without a file switch, a load request, or a scroll.
func TestNSameFileMovesCurrentLine(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "xa one\nplain\nxa two\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "xa one\n", 1, 0, 2, "xa"))
	addRec(t, idx, matchRec("a.txt", "xa two\n", 3, 0, 2, "xa"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m, _ = update(t, m, cmd())

	v := m.View().Content
	if !strings.Contains(rowWith(t, v, " one"), "\x1b[4m") {
		t.Fatalf("startup: line 1's match is not the underlined current match:\n%s", v)
	}
	if strings.Contains(rowWith(t, v, " two"), "\x1b[4m") {
		t.Fatalf("startup: line 3's match is already underlined:\n%s", v)
	}

	m, cmd = update(t, m, keyMsg("n"))
	if cmd != nil {
		t.Fatalf("same-file n returned a command: %v", cmd)
	}
	v = m.View().Content
	if strings.Contains(rowWith(t, v, " one"), "\x1b[4m") {
		t.Fatalf("after n: line 1's match is still underlined:\n%s", v)
	}
	if !strings.Contains(rowWith(t, v, " two"), "\x1b[4m") {
		t.Fatalf("after n: line 3's match is not the underlined current match:\n%s", v)
	}
	if !strings.Contains(v, "── a.txt ") {
		t.Fatalf("same-file n switched the panel:\n%s", v)
	}
}

// n onto a stop in another file switches the panel immediately: the
// filename rule and the list underline move to the new file and its
// load is requested when uncached; delivering the load renders its
// content under the new current matched line.
func TestNCrossesFileBoundary(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m, _ = update(t, m, cmd()) // a.txt loaded

	m, cmd = update(t, m, keyMsg("n"))
	if cmd == nil {
		t.Fatal("n onto an uncached file returned no load command")
	}
	v := m.View().Content
	if !strings.Contains(v, "── b.txt ") || !strings.Contains(v, "Loading…") {
		t.Fatalf("cross-file n did not switch the panel to b.txt:\n%s", v)
	}
	if !strings.Contains(v, "\x1b[4mb.txt\x1b[24m") || strings.Contains(v, "\x1b[4ma.txt") {
		t.Fatalf("cross-file n did not move the list underline to b.txt:\n%s", v)
	}

	msg := deliverNavLoad(t, cmd)
	if lr, ok := msg.(loadResult); !ok || string(lr.path) != "b.txt" {
		t.Fatalf("cross-file load = %#v, want the b.txt load", msg)
	}
	m, _ = update(t, m, msg)
	v = m.View().Content
	if !strings.Contains(v, "\x1b[4m\x1b[30;47mhit\x1b[24m\x1b[37;40m b") {
		t.Fatalf("b.txt content did not render under the current matched line:\n%s", v)
	}
}

// n past the last stop wraps to the first and p past the first wraps to
// the last; when the destination file is already cached neither step
// requests a load.
func TestNavigationWrapsCircular(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m, _ = update(t, m, cmd()) // a.txt loaded
	m, cmd = update(t, m, keyMsg("n"))
	m, _ = update(t, m, deliverNavLoad(t, cmd)) // b.txt loaded; cursor on the last stop

	m, cmd = update(t, m, keyMsg("n")) // wraps to a.txt
	// A file change still opens the pop-up — the command is the new
	// instance's expiry timer, not a load for the cached file.
	if em, ok := cmd().(popupExpiredMsg); !ok || em.id != m.popup.id {
		t.Fatalf("n onto a cached file returned %T, want the pop-up's expiry", cmd())
	}
	if navLoadMsg(t, cmd) != nil || len(m.loading) != 0 {
		t.Fatal("n onto a cached file requested a load")
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[4ma.txt\x1b[24m") {
		t.Fatalf("wrapping n did not return to a.txt:\n%s", v)
	}

	m, cmd = update(t, m, keyMsg("p")) // wraps back to b.txt
	if em, ok := cmd().(popupExpiredMsg); !ok || em.id != m.popup.id {
		t.Fatalf("p onto a cached file returned %T, want the pop-up's expiry", cmd())
	}
	if navLoadMsg(t, cmd) != nil || len(m.loading) != 0 {
		t.Fatal("p onto a cached file requested a load")
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[4mb.txt\x1b[24m") {
		t.Fatalf("wrapping p did not return to b.txt:\n%s", v)
	}
}

// The departing file's viewport stays saved and a visited file resumes
// from it, while a first visit starts from the top of the file. The
// destination reveal leaves the saved top alone here because the
// target row is visible inside it.
func TestCrossFileRestoresSavedViewport(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(50))
	var b strings.Builder
	for i := 1; i <= 50; i++ {
		b.WriteString("row\n")
	}
	writeMatchFile(t, dir, "b.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-06\n", 6, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "row\n", 1, 0, 3, "row"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m, _ = update(t, m, cmd()) // a.txt loaded; target row 5 visible at top 0
	// The no-style theme keeps the fixture's matched text contiguous.
	m.theme = theme.Plain()

	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-06") {
		t.Fatalf("a.txt scrolled to %q, want line-06", row)
	}

	// n to b.txt: a first visit starts at the top of the file.
	m, cmd = update(t, m, keyMsg("n"))
	m, _ = update(t, m, deliverNavLoad(t, cmd))
	if row := contentRow(t, m); !strings.Contains(row, "row") {
		t.Fatalf("first visit to b.txt shows %q, want the top of the file", row)
	}
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}

	// p back to a.txt resumes its saved top: the target row 5 is still
	// inside the saved window [5, 28), so the reveal does not scroll.
	m, _ = update(t, m, keyMsg("p"))
	if row := contentRow(t, m); !strings.Contains(row, "line-06") {
		t.Fatalf("revisit to a.txt shows %q, want the saved top line-06", row)
	}
}

// Manual scrolling does not move the matched-line cursor: after
// scrolling, n continues from the previously selected stop.
func TestManualScrollLeavesCursor(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(50))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-01\n", 1, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-40\n", 40, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m, _ = update(t, m, cmd())
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-11") {
		t.Fatalf("scrolled to %q, want line-11", row)
	}

	m, cmd = update(t, m, keyMsg("n"))
	if cmd != nil {
		t.Fatalf("same-file n after scrolling returned a command: %v", cmd)
	}
	stop, ok := m.index.Current()
	if !ok || stop.Line != 40 {
		t.Fatalf("n after scrolling selected line %d, want 40: the cursor must continue "+
			"from the selected stop, not the scrolled position", stop.Line)
	}
	// The destination reveal then moves the viewport to the target row:
	// 39 - floor(23/3) = 32, clamped to 50 - 23 = 27.
	if row := contentRow(t, m); !strings.Contains(row, "line-28") {
		t.Fatalf("same-file n did not reveal the target: first content row = %q, want line-28", row)
	}
}

// With exactly one stop, n and p are strict no-ops: no command, no view
// change, no cursor movement — and no load or pop-up is triggered.
func TestSingleStopIgnoresNP(t *testing.T) {
	m, _ := startBrowse(t, "/w", fixtureIndex(t, 1, 1), 80, 24)
	before := m.View().Content

	for _, key := range []tea.KeyPressMsg{keyMsg("n"), keyMsg("p")} {
		var cmd tea.Cmd
		m, cmd = update(t, m, key)
		if cmd != nil {
			t.Fatalf("single-stop %q returned a command: %v", key.String(), cmd)
		}
		if got := m.View().Content; got != before {
			t.Fatalf("single-stop %q changed the view:\nbefore:\n%s\nafter:\n%s",
				key.String(), before, got)
		}
		stop, ok := m.index.Current()
		if !ok || string(stop.Path) != "a.txt" || stop.Line != 1 {
			t.Fatalf("single-stop %q moved the cursor to %+v", key.String(), stop)
		}
	}
}

// The file list is a passive overview: no key selects a file directly.
// Keys that could plausibly operate a list are all ignored — the cursor
// is the only selection route.
func TestFileListHasNoDirectSelection(t *testing.T) {
	m, _ := startBrowse(t, "/w", fixtureIndex(t, 2, 1), 80, 24)

	for _, key := range []tea.KeyPressMsg{
		{Code: tea.KeyEnter},
		{Code: tea.KeyTab},
		{Code: tea.KeySpace},
		{Code: tea.KeyLeft},
		{Code: tea.KeyRight},
		keyMsg("j"),
		keyMsg("k"),
	} {
		var cmd tea.Cmd
		m, cmd = update(t, m, key)
		if cmd != nil {
			t.Fatalf("list-selection key %q returned a command: %v", key.String(), cmd)
		}
		stop, ok := m.index.Current()
		if !ok || string(stop.Path) != "a.txt" || stop.Line != 1 {
			t.Fatalf("list-selection key %q moved the cursor to %+v", key.String(), stop)
		}
	}
}
