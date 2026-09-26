package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// frameLines splits the rendered frame into its pane lines with all
// SGR sequences removed, so display cells line up with string cells.
func frameLines(m Model) []string {
	return strings.Split(ansi.Strip(m.View().Content), "\n")
}

// cellAt returns the grapheme cluster covering display cell c of a
// pane line — " " for a cell the row never painted.
func cellAt(s string, c int) string {
	rest := ansi.Strip(s)
	x := 0
	for len(rest) > 0 {
		cl, w := ansi.FirstGraphemeCluster([]byte(rest), ansi.GraphemeWidth)
		if c < x+w {
			return string(cl)
		}
		x += w
		rest = rest[len(cl):]
	}
	return " "
}

// indCol is the frame column of the first trailing gutter space — the
// hidden-left indicator position.
func indCol(m Model) int { return m.listW + m.gutterDigits() }

// subJSON renders one submatch member for a match record.
func subJSON(match string, start, end int) string {
	return fmt.Sprintf(`{"match":{"text":%q},"start":%d,"end":%d}`, match, start, end)
}

// matchRec renders one rg match record for path: the matched line's
// text without its terminator, its 1-based number, and the submatches.
func matchRec(path, text string, num int, subs ...string) string {
	return fmt.Sprintf(`{"type":"match","data":{"path":{"text":"%s"},"lines":{"text":"%s\n"},"line_number":%d,"submatches":[%s]}}`,
		path, text, num, strings.Join(subs, ","))
}

// indModel returns a settled run-off-edge browse model over a.txt at
// 80x24 — a 7-cell list, the digit-width gutter, the text area, and
// the reserved indicator column.
func indModel(t *testing.T, dir string, recs ...string) Model {
	t.Helper()
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m = pump(t, m, keyPress("w"))
	if m.wrap {
		t.Fatal("model did not enter run-off-edge mode")
	}
	return m
}

// The first trailing gutter space of every visible source line shows
// "_" when any of its text is hidden left — upgraded to "*" when a
// match or marker on that line is entirely hidden left — and stays
// blank when nothing is hidden left, including on an empty line.
func TestGutterUnderscoreStarAndBlank(t *testing.T) {
	dir := t.TempDir()
	l1 := "hit" + strings.Repeat("x", 150) // match [0,3)
	l4 := strings.Repeat("x", 150) + "hit" // match [150,153)
	writeWorkFile(t, dir, "a.txt",
		l1+"\n"+strings.Repeat("x", 150)+"\n\n"+l4+"\n")
	recs := append(fileRecs("a.txt",
		matchRec("a.txt", l1, 1, subJSON("hit", 0, 3)),
		matchRec("a.txt", l4, 4, subJSON("hit", 150, 153)),
	), `{"type":"summary","data":{}}`)
	m := indModel(t, dir, recs...)

	m.vp.SetOffset(10)
	lines := frameLines(m)
	ind := indCol(m)
	for _, tc := range []struct {
		line int // 0-based source line; pane row is line+1
		want string
	}{
		{0, "*"}, // its match is entirely hidden left
		{1, "_"}, // text hidden left only
		{2, " "}, // the empty line has no hidden text
		{3, "_"}, // its match is hidden right, not left
	} {
		if got := cellAt(lines[tc.line+1], ind); got != tc.want {
			t.Fatalf("line %d gutter indicator = %q, want %q (row %q)",
				tc.line, got, tc.want, lines[tc.line+1])
		}
	}
	// No right indicator anywhere: line 4's hidden-right match is not
	// on the current matched line, and line 1's match hides left.
	for i := 1; i <= 4; i++ {
		if got := cellAt(lines[i], m.width-1); got != " " {
			t.Fatalf("row %d reserved column = %q, want blank", i, got)
		}
	}
}

// The indicators paint in the theme's inverse style.
func TestIndicatorsPaintInverse(t *testing.T) {
	dir := t.TempDir()
	l1 := "hit" + strings.Repeat("x", 150) + "far"
	writeWorkFile(t, dir, "a.txt", l1+"\n")
	recs := append(fileRecs("a.txt",
		matchRec("a.txt", l1, 1, subJSON("hit", 0, 3), subJSON("far", 153, 156)),
	), `{"type":"summary","data":{}}`)
	m := indModel(t, dir, recs...)
	m.vp.SetOffset(10) // left match hidden left, right match hidden right

	v := m.View().Content
	if !strings.Contains(v, "\x1b[30;47m*\x1b[37;40m") {
		t.Fatalf("view lacks an inverse-\"*\" indicator: %q", v)
	}
	// Offset 2 leaves the last cell of "hit" painted — partially
	// visible — so the gutter shows "_" for the hidden-left text.
	m.vp.SetOffset(2)
	v = m.View().Content
	if !strings.Contains(v, "\x1b[30;47m_\x1b[37;40m") {
		t.Fatalf("view lacks an inverse-\"_\" indicator: %q", v)
	}
}

// The reserved rightmost column shows "*" only on the current matched
// line's visible row when a match or marker there is entirely hidden
// right; other lines stay blank even with hidden-right matches of
// their own, and a non-matched line's hidden-right text draws nothing.
func TestRightStarCurrentMatchedLineOnly(t *testing.T) {
	dir := t.TempDir()
	l := "hit" + strings.Repeat("x", 150) + "far" // matches [0,3), [153,156)
	writeWorkFile(t, dir, "a.txt",
		l+"\n"+strings.Repeat("y", 160)+"\n"+l+"\n")
	recs := append(fileRecs("a.txt",
		matchRec("a.txt", l, 1, subJSON("hit", 0, 3), subJSON("far", 153, 156)),
		matchRec("a.txt", l, 3, subJSON("hit", 0, 3), subJSON("far", 153, 156)),
	), `{"type":"summary","data":{}}`)
	m := indModel(t, dir, recs...)
	// offset 0: the "far" matches are entirely hidden right on both
	// matched lines; only line 1 is current.

	lines := frameLines(m)
	right := m.width - 1
	if got := cellAt(lines[1], right); got != "*" {
		t.Fatalf("current line's reserved column = %q, want \"*\"", got)
	}
	if got := cellAt(lines[2], right); got != " " {
		t.Fatalf("unmatched line's reserved column = %q, want blank", got)
	}
	if got := cellAt(lines[3], right); got != " " {
		t.Fatalf("non-current matched line's reserved column = %q, want blank", got)
	}
	// Nothing is hidden left at offset 0: the gutters stay blank.
	ind := indCol(m)
	for i := 1; i <= 3; i++ {
		if got := cellAt(lines[i], ind); got != " " {
			t.Fatalf("row %d gutter indicator = %q, want blank", i, got)
		}
	}
}

// When the current matched line scrolls vertically off-screen its
// right indicator is absent, while other lines' gutter indicators keep
// signposting hidden-left text.
func TestRightStarAbsentWhenCurrentLineOffScreen(t *testing.T) {
	dir := t.TempDir()
	l1 := "hit" + strings.Repeat("x", 150) + "far" // matches [0,3), [153,156)
	var sb strings.Builder
	sb.WriteString(l1 + "\n")
	for i := 0; i < 29; i++ {
		sb.WriteString(strings.Repeat("y", 60) + "\n")
	}
	writeWorkFile(t, dir, "a.txt", sb.String())
	recs := append(fileRecs("a.txt",
		matchRec("a.txt", l1, 1, subJSON("hit", 0, 3), subJSON("far", 153, 156)),
	), `{"type":"summary","data":{}}`)
	m := indModel(t, dir, recs...)

	m.vp.SetOffset(10)
	m = pump(t, m, codePress(tea.KeyDown))
	m = pump(t, m, codePress(tea.KeyDown))
	if m.vp.Top() == 0 {
		t.Fatal("scrolling did not move the current line off-screen")
	}
	lines := frameLines(m)
	right := m.width - 1
	ind := indCol(m)
	for i := 1; i < len(lines); i++ {
		if got := cellAt(lines[i], right); got != " " {
			t.Fatalf("row %d reserved column = %q with the current line off-screen, want blank", i, got)
		}
	}
	// Other lines still signpost their hidden-left text.
	if got := cellAt(lines[2], ind); got != "_" {
		t.Fatalf("scrolled line's gutter = %q, want \"_\"", got)
	}
}

// Both stars appear together on the current matched line: a match
// hidden left upgrades the gutter to "*" while a match hidden right
// paints the reserved column's "*".
func TestBothSidesHiddenStarsTogether(t *testing.T) {
	dir := t.TempDir()
	l1 := "hit" + strings.Repeat("x", 150) + "far" // matches [0,3), [153,156)
	writeWorkFile(t, dir, "a.txt", l1+"\n"+strings.Repeat("y", 60)+"\n")
	recs := append(fileRecs("a.txt",
		matchRec("a.txt", l1, 1, subJSON("hit", 0, 3), subJSON("far", 153, 156)),
	), `{"type":"summary","data":{}}`)
	m := indModel(t, dir, recs...)

	m.vp.SetOffset(10) // "hit" entirely hidden left; "far" still hidden right
	lines := frameLines(m)
	if got := cellAt(lines[1], indCol(m)); got != "*" {
		t.Fatalf("gutter = %q, want \"*\" for the hidden-left match", got)
	}
	if got := cellAt(lines[1], m.width-1); got != "*" {
		t.Fatalf("reserved column = %q, want \"*\" for the hidden-right match", got)
	}
}

// A partially visible match counts as visible: it produces no
// hidden-match indicator for that side — on the left the gutter stays
// at "_", on the right the reserved column stays blank.
func TestPartialMatchVisibilityDrawsNoStar(t *testing.T) {
	dir := t.TempDir()
	// "hit" covers cells 5–7; "far" covers 70–72 against a 69-cell
	// text area (7-cell list, 3-cell gutter, 1 reserved at 80x24).
	l1 := strings.Repeat("x", 5) + "hit" + strings.Repeat("x", 62) + "far"
	writeWorkFile(t, dir, "a.txt", l1+"\n")
	recs := append(fileRecs("a.txt",
		matchRec("a.txt", l1, 1, subJSON("hit", 5, 8), subJSON("far", 70, 73)),
	), `{"type":"summary","data":{}}`)
	m := indModel(t, dir, recs...)
	if m.textW != 69 {
		t.Fatalf("textW = %d, want 69 for this fixture", m.textW)
	}
	lines := frameLines(m)
	ind, right := indCol(m), m.width-1

	// Offset 0: "far" is entirely hidden right → "*"; nothing hides
	// left → blank gutter.
	if got := cellAt(lines[1], ind); got != " " {
		t.Fatalf("gutter at offset 0 = %q, want blank", got)
	}
	if got := cellAt(lines[1], right); got != "*" {
		t.Fatalf("reserved column at offset 0 = %q, want \"*\"", got)
	}

	// Offset 2: "far" pokes one cell into the window — partially
	// visible → no right star; "hit" fully painted → "_" for the
	// hidden-left text only.
	m.vp.SetOffset(2)
	lines = frameLines(m)
	if got := cellAt(lines[1], ind); got != "_" {
		t.Fatalf("gutter at offset 2 = %q, want \"_\"", got)
	}
	if got := cellAt(lines[1], right); got != " " {
		t.Fatalf("reserved column at offset 2 = %q, want blank — the match is partially visible", got)
	}

	// Offset 7: one cell of "hit" still paints — partially visible →
	// still "_" not "*".
	m.vp.SetOffset(7)
	lines = frameLines(m)
	if got := cellAt(lines[1], ind); got != "_" {
		t.Fatalf("gutter at offset 7 = %q, want \"_\" — the match is partially visible", got)
	}

	// Offset 8: the last cell of "hit" leaves the window → entirely
	// hidden left → "*".
	m.vp.SetOffset(8)
	lines = frameLines(m)
	if got := cellAt(lines[1], ind); got != "*" {
		t.Fatalf("gutter at offset 8 = %q, want \"*\"", got)
	}
}

// A match ending exactly on the last text cell stays fully painted —
// visible — while a second match farther right is entirely hidden:
// the reserved column's "*" appears beside it, never overwriting the
// painted match.
func TestLastCellMatchAndFarMatchRightStar(t *testing.T) {
	dir := t.TempDir()
	// 66 x's + "hit" covers cells 66–68, the last text cell; "far"
	// covers cells 120–122, entirely hidden right at offset 0.
	l1 := strings.Repeat("x", 66) + "hit" + strings.Repeat("x", 51) + "far"
	writeWorkFile(t, dir, "a.txt", l1+"\n")
	recs := append(fileRecs("a.txt",
		matchRec("a.txt", l1, 1, subJSON("hit", 66, 69), subJSON("far", 120, 123)),
	), `{"type":"summary","data":{}}`)
	m := indModel(t, dir, recs...)
	if m.textW != 69 {
		t.Fatalf("textW = %d, want 69 for this fixture", m.textW)
	}

	lines := frameLines(m)
	// The last text column (frame cell 78) paints the match's final
	// 't'; the reserved cell 79 carries the right star.
	if got := cellAt(lines[1], m.listW+m.gutterDigits()+2+m.textW-1); got != "t" {
		t.Fatalf("last text cell = %q, want \"t\" — the visible match is not overwritten", got)
	}
	if got := cellAt(lines[1], m.width-1); got != "*" {
		t.Fatalf("reserved column = %q, want \"*\"", got)
	}
}

// A match on a wide glyph split by a clip edge renders as blanks and
// is not visible: a right-edge split produces the current line's right
// "*", and a left-edge split on another line produces its gutter "*".
// Offsets that paint the whole cluster clear both.
func TestSplitGlyphBlanksDrawStars(t *testing.T) {
	dir := t.TempDir()
	l1 := strings.Repeat("x", 68) + "世" + strings.Repeat("x", 40) // 世 at cells 68–69
	l2 := "aaa世" + strings.Repeat("y", 80)                        // 世 at cells 3–4
	writeWorkFile(t, dir, "a.txt", l1+"\n"+l2+"\n")
	recs := append(fileRecs("a.txt",
		matchRec("a.txt", l1, 1, subJSON("世", 68, 71)),
		matchRec("a.txt", l2, 2, subJSON("世", 3, 6)),
	), `{"type":"summary","data":{}}`)
	m := indModel(t, dir, recs...)
	if m.textW != 69 {
		t.Fatalf("textW = %d, want 69 for this fixture", m.textW)
	}
	lines := frameLines(m)
	ind, right := indCol(m), m.width-1

	// Offset 0: line 1's 世 straddles the right edge — its lead cell
	// paints blank, so the match is entirely hidden right → "*".
	if got := cellAt(lines[1], right); got != "*" {
		t.Fatalf("reserved column = %q, want \"*\" for the split-glyph match", got)
	}
	// Line 2's 世 paints whole → its match is visible → blank gutter.
	if got := cellAt(lines[2], ind); got != " " {
		t.Fatalf("line 2 gutter at offset 0 = %q, want blank", got)
	}

	// Offset 4: line 2's 世 is split by the left edge — its in-window
	// cell is blank, so its match is entirely hidden left → "*".
	// Line 1's 世 now paints whole → its right star clears.
	m.vp.SetOffset(4)
	lines = frameLines(m)
	if got := cellAt(lines[2], ind); got != "*" {
		t.Fatalf("line 2 gutter at offset 4 = %q, want \"*\"", got)
	}
	if got := cellAt(lines[1], right); got != " " {
		t.Fatalf("reserved column at offset 4 = %q, want blank — 世 is whole", got)
	}
	if got := cellAt(lines[1], ind); got != "_" {
		t.Fatalf("line 1 gutter at offset 4 = %q, want \"_\"", got)
	}
}

// Wrap mode draws neither indicator nor a reserved column: the text
// area reaches the frame's last cell, and every gutter's first
// trailing space stays blank even on lines that would hide content in
// run-off-edge mode.
func TestWrapModeDrawsNoIndicatorsOrReservedColumn(t *testing.T) {
	dir := t.TempDir()
	l1 := "hit" + strings.Repeat("x", 150) + "far" // matches [0,3), [153,156)
	writeWorkFile(t, dir, "a.txt", l1+"\n"+strings.Repeat("y", 60)+"\n")
	recs := append(fileRecs("a.txt",
		matchRec("a.txt", l1, 1, subJSON("hit", 0, 3), subJSON("far", 153, 156)),
	), `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd) // wrap mode stays on — no w press
	if !m.wrap {
		t.Fatal("model unexpectedly left wrap mode")
	}
	if m.reservedW() != 0 {
		t.Fatalf("reservedW in wrap mode = %d, want 0", m.reservedW())
	}

	lines := frameLines(m)
	ind := indCol(m)
	for i := 1; i < len(lines); i++ {
		if got := cellAt(lines[i], ind); got != " " {
			t.Fatalf("wrap row %d gutter indicator = %q, want blank", i, got)
		}
	}
	// With no reserved column the wrapped text reaches the last cell:
	// the first wrap row ends in an 'x' at the frame's right edge.
	if got := cellAt(lines[1], m.width-1); got != "x" {
		t.Fatalf("wrap row last cell = %q, want \"x\" — no reserved column", got)
	}
}

// A uniform-lines fixture at a nonzero offset shows "_" on every
// visible line — the pan clamp's painted-cluster guarantee constrains
// painted cells, not indicator counts.
func TestUniformLinesEveryGutterUnderscore(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	for i := 0; i < 4; i++ {
		sb.WriteString(strings.Repeat(string(rune('a'+i)), 30) + "\n")
	}
	writeWorkFile(t, dir, "a.txt", sb.String())
	recs := append(fileRecs("a.txt",
		matchRec("a.txt", strings.Repeat("a", 30), 1, subJSON("aaa", 20, 23)),
	), `{"type":"summary","data":{}}`)
	m := indModel(t, dir, recs...)

	m.vp.SetOffset(15)
	lines := frameLines(m)
	ind := indCol(m)
	for i := 0; i < 4; i++ {
		if got := cellAt(lines[i+1], ind); got != "_" {
			t.Fatalf("line %d gutter = %q, want \"_\" — every visible line has hidden-left text", i, got)
		}
	}
	// Line 1's match at cells 20–22 paints inside the window, so no
	// gutter or right star appears.
	if got := cellAt(lines[1], m.width-1); got != " " {
		t.Fatalf("reserved column = %q, want blank", got)
	}
}
