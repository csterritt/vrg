package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
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

// wantTextW recomputes the text width the layout chain implies for the
// model's current state — the terminal width minus the file list's
// allocated width, the buffer gutter, and the mode's reserved
// right-indicator column, the same value the layout key carries. It
// derives from the terminal width and the list-width function rather
// than the cached layout fields, so a regression installing the
// viewport at a terminal-derived width — omitting the list, the
// gutter, or the reserved indicator — fails every expectation built
// on it.
func (m Model) wantTextW() int {
	res := m.reservedW()
	gutterW := m.gutterDigits() + 2
	return max(0, m.width-m.listWidth(gutterW, res)-gutterW-res)
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
	want := 295 + 1 - m.wantTextW()
	if m.vp.Offset() != want {
		t.Fatalf("offset after n = %d, want %d = 295 + 1 − %d",
			m.vp.Offset(), want, m.wantTextW())
	}
	if got := visText(t, m, 1); got != strings.Repeat("x", m.wantTextW()-1)+"h" {
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
		t.Fatalf("the line-2 reveal left offset 0 — fixture broken at width %d", m.wantTextW())
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
	want := 296 - m.wantTextW()
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
	want := 296 - m.wantTextW()
	if m.vp.Offset() != want {
		t.Fatalf("offset after crossing n = %d, want %d — reset then reveal",
			m.vp.Offset(), want)
	}
	if got := visText(t, m, 0); got != strings.Repeat("x", m.wantTextW()-1)+"h" {
		t.Fatalf("b.txt row = %q, want the match start on the last text column", got)
	}
}

// Hiding the file list widens the panel: the reveal re-measures the
// text area from the current layout state, not the raw terminal width
// — with the list gone the same far-right match lands on the last
// text column of a wider window.
func TestHRevealListHiddenRemeasuresPanelWidth(t *testing.T) {
	dir := t.TempDir()
	m, cmd := browseModel(t, dir, 80, 24, hline3(t, dir)...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))
	shown := m.wantTextW()

	m = pump(t, m, codePress(tea.KeyLeft))
	if m.listShow || m.listW != 0 {
		t.Fatalf("left did not hide the list: show=%v listW=%d", m.listShow, m.listW)
	}
	if m.wantTextW() <= shown {
		t.Fatalf("hiding the list did not widen the text area: %d → %d", shown, m.wantTextW())
	}

	m, _ = update(t, m, keyPress("n")) // → a.txt:2, match at cell 295
	want := 296 - m.wantTextW()
	if m.vp.Offset() != want {
		t.Fatalf("offset after n = %d, want %d = 296 − %d",
			m.vp.Offset(), want, m.wantTextW())
	}
	if got := visText(t, m, 1); got != strings.Repeat("x", m.wantTextW()-1)+"h" {
		t.Fatalf("line-2 row = %q, want the match start 'h' on the last text column", got)
	}
}

// Wrap mode reserves no indicator column: the text area is the panel
// width minus the gutter alone, the installed layout's key carries
// exactly that width, and wrapped rows fill to the panel's last cell.
func TestHRevealWrapModeReservesNoIndicatorColumn(t *testing.T) {
	dir := t.TempDir()
	m, cmd := browseModel(t, dir, 80, 24, hline3(t, dir)...)
	m = settle(t, m, cmd) // wrap mode stays on — no w press
	if !m.wrap {
		t.Fatal("model unexpectedly left wrap mode")
	}
	if m.reservedW() != 0 {
		t.Fatalf("reservedW in wrap mode = %d, want 0", m.reservedW())
	}
	if inst, ok := m.rows["a.txt"]; !ok || inst.key.Width != m.wantTextW() {
		t.Fatalf("installed layout key = %+v, want width %d", inst.key, m.wantTextW())
	}

	// Line 2's 298 cells wrap into rows of the full text width; the
	// first of them paints to the frame's last cell — no reserved
	// column sits between the text and the panel edge.
	r, ok := visRow(m, 1)
	if !ok {
		t.Fatal("line 2 has no visible row")
	}
	if len(r.Cells) != m.wantTextW() {
		t.Fatalf("first wrap row carries %d cells, want the text width %d",
			len(r.Cells), m.wantTextW())
	}
	if got := cellAt(frameLines(m)[2], m.width-1); got != "x" {
		t.Fatalf("frame's last cell = %q, want \"x\" — text reaches the panel edge", got)
	}
}

// A resize re-measures the text width from the new panel before the
// reveal computes its offset: at a wider terminal the same far-right
// match needs a smaller offset, and shrinking back re-measures again.
func TestHRevealResizeRemeasuresTextWidth(t *testing.T) {
	dir := t.TempDir()
	m, cmd := browseModel(t, dir, 80, 24, hline3(t, dir)...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))

	m = pump(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m, _ = update(t, m, keyPress("n")) // → a.txt:2, match at cell 295
	want := 296 - m.wantTextW()
	if m.vp.Offset() != want {
		t.Fatalf("offset after resize+n = %d, want %d = 296 − %d",
			m.vp.Offset(), want, m.wantTextW())
	}
	if got := visText(t, m, 1); got != strings.Repeat("x", m.wantTextW()-1)+"h" {
		t.Fatalf("line-2 row = %q, want the match start 'h' on the last text column", got)
	}

	m = pump(t, m, tea.WindowSizeMsg{Width: 60, Height: 24})
	m, _ = update(t, m, keyPress("p")) // → a.txt:1
	m, _ = update(t, m, keyPress("n")) // → a.txt:2 again
	want = 296 - m.wantTextW()
	if m.vp.Offset() != want {
		t.Fatalf("offset after shrink+n = %d, want %d = 296 − %d",
			m.vp.Offset(), want, m.wantTextW())
	}
	if got := visText(t, m, 1); got != strings.Repeat("x", m.wantTextW()-1)+"h" {
		t.Fatalf("line-2 row after shrink = %q, want the match start on the last text column", got)
	}
}

// The composed run-off-edge frame keeps the panel's width terms
// visible at once: after panning right every row stays within the
// terminal width, the file list keeps its cells, painted text stops
// where the text width ends, and the reserved indicator column is the
// panel's right edge outside the text area.
func TestComposedViewContentStaysInsidePanel(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt",
		"xxxxxhit\n"+strings.Repeat("x", 295)+"hit\n"+strings.Repeat("x", 250)+"hit\n")
	writeWorkFile(t, dir, "b.txt", "hit\n")
	writeWorkFile(t, dir, "c.txt", "hit\n")
	recs := append(fileRecs("a.txt",
		matchLineRec("a.txt", "xxxxxhit", 1, "hit", 5, 8),
		matchLineRec("a.txt", strings.Repeat("x", 295)+"hit", 2, "hit", 295, 298),
	), fileRecs("b.txt",
		matchLineRec("b.txt", "hit", 1, "hit", 0, 3))...)
	recs = append(recs, fileRecs("c.txt",
		matchLineRec("c.txt", "hit", 1, "hit", 0, 3))...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))
	m, _ = update(t, m, keyPress(">"))
	m, _ = update(t, m, keyPress(">")) // pan 20 columns right
	if m.vp.Offset() != 20 {
		t.Fatalf("offset after > > = %d, want 20", m.vp.Offset())
	}

	// The text area ends exactly one reserved indicator column short
	// of the frame's right edge — the chain terminal − list − gutter
	// − reserved leaves no other slack.
	textEnd := m.listW + m.gutterDigits() + 2 + m.wantTextW()
	if textEnd != m.width-m.reservedW() {
		t.Fatalf("text area ends at column %d, want %d — one reserved column short of the edge",
			textEnd, m.width-m.reservedW())
	}
	lines := frameLines(m)
	for i, row := range lines {
		if w := ansi.StringWidth(row); w > m.width {
			t.Fatalf("row %d is %d cells — over the %d-column terminal: %q", i, w, m.width, row)
		}
		// The last file-list cell stays blank padding: panned
		// content never bleeds under the list.
		if got := cellAt(row, m.listW-1); got != " " {
			t.Fatalf("row %d list-boundary cell = %q, want blank — content bled under the list", i, got)
		}
		// On content rows the reserved column paints only blank or
		// the indicator — never text. Row 0 is the filename rule and
		// spans the panel to the edge.
		if i > 0 {
			if got := cellAt(row, m.width-1); got != " " && got != "*" {
				t.Fatalf("row %d reserved column = %q, want blank or \"*\"", i, got)
			}
		}
	}
	// The list entries survived the pan in place.
	for i, n := range []string{"a.txt", "b.txt", "c.txt"} {
		if !strings.HasPrefix(lines[i], n) {
			t.Fatalf("row %d = %q, want it to lead with list entry %q", i, lines[i], n)
		}
	}
	// Hidden-left text signposts at the gutter's first trailing
	// space — the indicator column the list width implies.
	if got := cellAt(lines[2], indCol(m)); got != "_" {
		t.Fatalf("panned row's gutter indicator = %q, want \"_\"", got)
	}
	// And the last text cell still paints content, not the reserved
	// column's space: the panned long line fills the text area.
	if got := cellAt(lines[2], m.width-2); got == " " {
		t.Fatalf("last text cell is blank on a filled row — the text area is mismeasured")
	}
}

// A match on a standalone combining mark reveals and paints the
// Issue #43 fallback cell FileBuffer hands down: the mark's own
// one-cell cluster — U+25CC plus the mark's bytes — stands between the
// ^A escape and the following text, and the highlight covers exactly
// that cell and nothing adjacent.
func TestHRevealStandaloneMatchPaintsFallbackCell(t *testing.T) {
	dir := t.TempDir()
	// Line 2: 60 x's + \x01 + ́ + 30 x's — the ^A escape is the
	// two-cell cluster at cells 60–61 and the mark's standalone
	// cluster takes the one-cell ◌́ fallback at cell 62, so
	// the recorded mark at bytes 61–63 maps to span [62,63).
	l2 := strings.Repeat("x", 60) + "\x01" + "́" + strings.Repeat("x", 30)
	writeWorkFile(t, dir, "a.txt", "hit\n"+l2+"\n")
	recs := append(fileRecs("a.txt",
		matchLineRec("a.txt", "hit", 1, "hit", 0, 3),
		matchRecJSON("a.txt", l2, 2, subJSON("́", 61, 63)),
	), `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))

	m, _ = update(t, m, keyPress("n")) // → a.txt:2
	if s, _ := m.currentStop(); s.Line != 2 {
		t.Fatalf("stop after n = %+v, want a.txt:2", s)
	}
	// The fallback cell sits inside the window, so the offset holds
	// at zero and the painted span covers exactly that one cell.
	if m.vp.Offset() != 0 {
		t.Fatalf("offset = %d, want 0 — the fallback target cell 62 was visible", m.vp.Offset())
	}
	r, ok := visRow(m, 1)
	if !ok {
		t.Fatal("line 2 has no visible row")
	}
	if len(r.Spans) != 1 || r.Spans[0].Start != 62 || r.Spans[0].End != 63 {
		t.Fatalf("line-2 spans = %+v, want the fallback cell's [{62 63}]", r.Spans)
	}
	if got := m.View().Content; !strings.Contains(got, "\x1b[30;47;4m◌́\x1b[24;37;40m") {
		t.Fatalf("view lacks the one-cell fallback highlight: %q", got)
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
	want := 300 + 2 - m.wantTextW()
	if m.vp.Offset() != want {
		t.Fatalf("offset = %d, want %d = 300 + 2 − %d", m.vp.Offset(), want, m.wantTextW())
	}
	r, ok := visRow(m, 1)
	if !ok {
		t.Fatal("line 2 has no visible row")
	}
	if len(r.Cells) != m.wantTextW() || r.Cells[m.wantTextW()-2].Text != "世" || !r.Cells[m.wantTextW()-1].Cont {
		t.Fatalf("line-2 row's last cells = %+v, want 世 painted whole at the right edge",
			r.Cells[max(0, len(r.Cells)-3):])
	}
}
