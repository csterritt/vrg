package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/viewport"
)

// matchLineRec returns one rg match record for path: the matched
// line's text without its terminator, its 1-based number, and the
// submatch's byte range and recorded bytes.
func matchLineRec(path, text string, num int, match string, start, end int) string {
	return fmt.Sprintf(`{"type":"match","data":{"path":{"text":"%s"},"lines":{"text":"%s\n"},"line_number":%d,"submatches":[{"match":{"text":"%s"},"start":%d,"end":%d}]}}`,
		path, text, num, match, start, end)
}

// fileRecs wraps match records in their begin/end lifecycle records.
func fileRecs(path string, matches ...string) []string {
	return append([]string{`{"type":"begin","data":{"path":{"text":"` + path + `"}}}`},
		append(matches, `{"type":"end","data":{"path":{"text":"`+path+`"},"binary_offset":null}}`)...)
}

// hline3 writes the shared three-stop fixture: a.txt line 1 "xxxxxhit"
// (match at cell 5), line 2 of 295 x's + "hit" (cell 295), and line 3
// of 250 x's + "hit" (cell 250).
func hline3(t *testing.T, dir string) []string {
	t.Helper()
	writeWorkFile(t, dir, "a.txt",
		"xxxxxhit\n"+strings.Repeat("x", 295)+"hit\n"+strings.Repeat("x", 250)+"hit\n")
	return append(fileRecs("a.txt",
		matchLineRec("a.txt", "xxxxxhit", 1, "hit", 5, 8),
		matchLineRec("a.txt", strings.Repeat("x", 295)+"hit", 2, "hit", 295, 298),
		matchLineRec("a.txt", strings.Repeat("x", 250)+"hit", 3, "hit", 250, 253),
	), `{"type":"summary","data":{}}`)
}

// visRow returns the currently visible row for 0-based source line i.
func visRow(m Model, line int) (viewport.Row, bool) {
	for _, r := range m.vp.Visible() {
		if r.Line == line {
			return r, true
		}
	}
	return viewport.Row{}, false
}

// visText joins the painted cell texts of the visible row for line i.
func visText(t *testing.T, m Model, line int) string {
	t.Helper()
	r, ok := visRow(m, line)
	if !ok {
		t.Fatalf("line %d has no visible row", line)
	}
	var b strings.Builder
	for _, c := range r.Cells {
		b.WriteString(c.Text)
	}
	return b.String()
}

// In run-off-edge mode a same-file n to a match right of the text area
// moves the horizontal offset by the minimum columns that paint its
// start cell — T − (text width − 1) — landing it on the last text
// column.
func TestHRevealSameFileNScrollsRightMinimal(t *testing.T) {
	dir := t.TempDir()
	m, cmd := browseModel(t, dir, 80, 24, hline3(t, dir)...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))

	m, ncmd := update(t, m, keyPress("n"))
	if ncmd != nil {
		t.Fatalf("same-file n returned a command %T", ncmd)
	}
	if s, _ := m.currentStop(); s.Line != 2 {
		t.Fatalf("stop after n = %+v, want a.txt:2", s)
	}
	want := 295 + 1 - m.textW
	if m.vp.Offset() != want {
		t.Fatalf("offset after n = %d, want %d = 295 + 1 − %d",
			m.vp.Offset(), want, m.textW)
	}
	if got := visText(t, m, 1); got != strings.Repeat("x", m.textW-1)+"h" {
		t.Fatalf("line-2 row = %q, want the match start 'h' on the last text column", got)
	}
}

// p back to a match left of the window moves the offset to exactly the
// target column.
func TestHRevealPBackScrollsLeftToTarget(t *testing.T) {
	dir := t.TempDir()
	m, cmd := browseModel(t, dir, 80, 24, hline3(t, dir)...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))
	m, _ = update(t, m, keyPress("n")) // → a.txt:2, offset 296 − textW

	m, _ = update(t, m, keyPress("p")) // → a.txt:1, match at cell 5
	if s, _ := m.currentStop(); s.Line != 1 {
		t.Fatalf("stop after p = %+v, want a.txt:1", s)
	}
	if m.vp.Offset() != 5 {
		t.Fatalf("offset after p = %d, want 5 — the target column", m.vp.Offset())
	}
	if got := visText(t, m, 0); got != "hit" {
		t.Fatalf("line-1 row = %q, want %q", got, "hit")
	}
}

// A match whose start cell is already painted causes no horizontal
// movement on n.
func TestHRevealVisibleMatchKeepsOffset(t *testing.T) {
	dir := t.TempDir()
	m, cmd := browseModel(t, dir, 80, 24, hline3(t, dir)...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))
	m, _ = update(t, m, keyPress("n")) // → a.txt:2, offset 296 − textW
	before := m.vp.Offset()
	if before == 0 {
		t.Fatalf("the line-2 reveal left offset 0 — fixture broken at width %d", m.textW)
	}

	// Line 3's match at cell 250 sits inside the revealed window, so
	// n to it must not move the offset.
	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); s.Line != 3 {
		t.Fatalf("stop after n = %+v, want a.txt:3", s)
	}
	if m.vp.Offset() != before {
		t.Fatalf("offset = %d, want the unchanged %d — the target was visible",
			m.vp.Offset(), before)
	}
}

// The startup reveal applies horizontal movement too: with run-off-edge
// active when the first file's layout installs, a first-stop match
// right of the text area moves the offset to show its start cell.
func TestHRevealAppliesAtStartup(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", strings.Repeat("x", 295)+"hit\n")
	m := newModel(nil, nil)
	m.popupTimer = func(int) tea.Cmd { return nil }
	m.wrap = false // run-off-edge from startup
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	recs := append(fileRecs("a.txt",
		matchLineRec("a.txt", strings.Repeat("x", 295)+"hit", 1, "hit", 295, 298)),
		`{"type":"summary","data":{}}`)
	m, cmd := update(t, m, searchDoneMsg{index: fixtureIndex(t, dir, recs...)})
	m = settle(t, m, cmd)
	want := 296 - m.textW
	if m.vp.Offset() != want {
		t.Fatalf("offset after startup reveal = %d, want %d", m.vp.Offset(), want)
	}
}

// A file-changing n applies the Issue #18 reset first, then reveals:
// the destination's far-right match still lands at the right edge —
// the offset is derived from zero, not added to the old pan.
func TestHRevealAfterFileChangeReset(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "hit a\n")
	writeWorkFile(t, dir, "b.txt", strings.Repeat("x", 295)+"hit\n")
	recs := append(fileRecs("a.txt", matchLineRec("a.txt", "hit a", 1, "hit", 0, 3)),
		fileRecs("b.txt", matchLineRec("b.txt", strings.Repeat("x", 295)+"hit", 1, "hit", 295, 298))...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))
	m, _ = update(t, m, keyPress(">")) // pan a.txt right 10 columns

	m = pump(t, m, keyPress("n")) // → b.txt:1
	if string(m.currentPath()) != "b.txt" {
		t.Fatalf("path after n = %s, want b.txt", m.currentPath())
	}
	want := 296 - m.textW
	if m.vp.Offset() != want {
		t.Fatalf("offset after crossing n = %d, want %d — reset then reveal",
			m.vp.Offset(), want)
	}
	if got := visText(t, m, 0); got != strings.Repeat("x", m.textW-1)+"h" {
		t.Fatalf("b.txt row = %q, want the match start on the last text column", got)
	}
}

// A match whose first submatch starts on a two-cell cluster reveals by
// the cluster-width rule: both cells of the CJK glyph paint at the
// right edge, not a clipping blank.
func TestHRevealWideClusterMatchPaintsBothCells(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "hit\n"+strings.Repeat("x", 300)+"世\n")
	recs := append(fileRecs("a.txt",
		matchLineRec("a.txt", "hit", 1, "hit", 0, 3),
		matchLineRec("a.txt", strings.Repeat("x", 300)+"世", 2, "世", 300, 303)),
		`{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))

	m, _ = update(t, m, keyPress("n")) // → a.txt:2, 世 at cells 300–301
	want := 300 + 2 - m.textW
	if m.vp.Offset() != want {
		t.Fatalf("offset = %d, want %d = 300 + 2 − %d", m.vp.Offset(), want, m.textW)
	}
	r, ok := visRow(m, 1)
	if !ok {
		t.Fatal("line 2 has no visible row")
	}
	if len(r.Cells) != m.textW || r.Cells[m.textW-2].Text != "世" || !r.Cells[m.textW-1].Cont {
		t.Fatalf("line-2 row's last cells = %+v, want 世 painted whole at the right edge",
			r.Cells[max(0, len(r.Cells)-3):])
	}
}
