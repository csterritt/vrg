package app

import (
	"fmt"
	"strings"
	"testing"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// flatIndicatorModel loads the index, sizes the frame for a 40-cell
// flat text width, toggles into run-off-edge mode with the layout
// installed, and returns the current file's key — the shared setup
// for the Issue #20 indicator rendering tests.
func flatIndicatorModel(t *testing.T, m *model, idx *searchindex.Index, gutter int) string {
	t.Helper()
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)
	widenFrame(t, m, key)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	flatTextW(t, m, gutter)
	return key
}

// panTo moves the file's horizontal offset exactly to off through the
// real Pan path; the fixtures stay inside the paintable boundary so
// the stored offset lands where asked.
func panTo(t *testing.T, m *model, key string, off int) {
	t.Helper()
	vp := m.vps[key]
	vp.Pan(off, m.rows[key], contentRows24)
	m.vps[key] = vp
	if got := m.vps[key].Off(); got != off {
		t.Fatalf("off = %d, want %d", got, off)
	}
}

// The gutter's first trailing space marks every visible source line:
// '_' when text is hidden left, '*' when a match on the line is
// entirely hidden left, blank when nothing is hidden — including an
// empty line, which has no text to hide. The marks apply to every
// visible line, not just the current one.
func TestGutterMarkPerVisibleLine(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{{
		name: "a.txt",
		content: strings.Repeat("x", 5) + "one" + strings.Repeat("x", 60) + "\n" +
			strings.Repeat("x", 20) + "two" + strings.Repeat("x", 60) + "\n" +
			strings.Repeat("y", 50) + "tre" + "\n" +
			"\n" +
			"pad\n",
		stops: []navStop{
			{line: 1, start: 5, end: 8},
			{line: 2, start: 20, end: 23},
		},
	}})
	key := flatIndicatorModel(t, m, idx, 3)
	listW := m.listWidth()
	panTo(t, m, key, 10)

	// At offset 10 the window shows cells 10–49 of each line.
	cases := []struct {
		name string
		row  int
		want string
	}{
		// Line 1's match at cells 5–7 is entirely hidden left: the
		// gutter upgrades '_' to '*'.
		{"match entirely hidden left", 1, "1* " + strings.Repeat("x", 40) + " "},
		// Line 2's match at cells 20–22 is painted; only text hides
		// left, so '_'.
		{"match visible, text hidden", 2, "2_ " + strings.Repeat("x", 10) + "two" + strings.Repeat("x", 27) + " "},
		// Line 3 has no match at all: plain '_'.
		{"no match, text hidden", 3, "3_ " + strings.Repeat("y", 40) + " "},
		// The empty line has no text — nothing is hidden, so the
		// gutter stays blank.
		{"empty line stays blank", 4, "4  " + strings.Repeat(" ", 41)},
		// Every cell of the short line hides left — '_' still shows.
		{"whole line hidden left", 5, "5_ " + strings.Repeat(" ", 41)},
	}
	for _, c := range cases {
		if row := dropCells(frameRow(t, m, c.row), listW+1); row != c.want {
			t.Fatalf("%s: row = %q, want %q", c.name, row, c.want)
		}
	}
}

// The reserved right column's '*' belongs to the current matched line
// alone: it shows on that line's row when a match is entirely hidden
// right and moves with the cursor — a non-current line with a
// hidden-right match shows nothing.
func TestRightStarFollowsCurrentLine(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	line := func(needle string) string {
		return needle + strings.Repeat("x", 40) + "far" + "\n"
	}
	// Each line matches at cells 0–2 (visible at offset 0) and again
	// at cells 43–45 — "far" is entirely hidden right of the 40-cell
	// window.
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: line("one") + line("two") + line("tre"),
		stops: []navStop{
			{line: 1, start: 0, end: 3}, {line: 1, start: 43, end: 46},
			{line: 2, start: 0, end: 3}, {line: 2, start: 43, end: 46},
			{line: 3, start: 0, end: 3}, {line: 3, start: 43, end: 46},
		},
	}})
	flatIndicatorModel(t, m, idx, 3)
	listW := m.listWidth()

	// Line 1 is the current matched line: its row carries the right
	// star; lines 2 and 3 hide the same "far" match but are not
	// current, so their reserved columns stay blank.
	want := func(n int, needle, right string) string {
		return fmt.Sprintf("%d  %s%s%s", n, needle, strings.Repeat("x", 37), right)
	}
	for _, c := range []struct {
		row  int
		want string
	}{
		{1, want(1, "one", "*")},
		{2, want(2, "two", " ")},
		{3, want(3, "tre", " ")},
	} {
		if row := dropCells(frameRow(t, m, c.row), listW+1); row != c.want {
			t.Fatalf("current line 1: row = %q, want %q", row, c.want)
		}
	}

	// n makes line 2 current: the star moves to its row. Its first
	// submatch at cell 0 is already painted, so the offset does not
	// move.
	m.Update(keyN)
	for _, c := range []struct {
		row  int
		want string
	}{
		{1, want(1, "one", " ")},
		{2, want(2, "two", "*")},
		{3, want(3, "tre", " ")},
	} {
		if row := dropCells(frameRow(t, m, c.row), listW+1); row != c.want {
			t.Fatalf("current line 2: row = %q, want %q", row, c.want)
		}
	}
}

// When the current matched line scrolls out of the window its right
// star is absent — the reserved column is blank on every visible row
// even though the line's hidden-right match is unchanged.
func TestRightStarAbsentWhenCurrentLineOffScreen(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	var content strings.Builder
	content.WriteString("one" + strings.Repeat("x", 40) + "far\n")
	for i := 2; i <= 30; i++ {
		content.WriteString("pad\n")
	}
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: content.String(),
		stops:   []navStop{{line: 1, start: 0, end: 3}, {line: 1, start: 43, end: 46}},
	}})
	key := flatIndicatorModel(t, m, idx, 4)
	listW := m.listWidth()

	// Line 1 is current and visible with "far" hidden right.
	if row := dropCells(frameRow(t, m, 1), listW+1); !strings.HasSuffix(row, "*") {
		t.Fatalf("current visible row = %q, want the right star", row)
	}

	// A full page down puts line 1 off-screen; no row may carry the
	// right marker while the current matched line is not visible.
	m.Update(keyPgDn)
	if got := m.vps[key].Top(); got == 0 {
		t.Fatal("pgdn did not scroll line 1 off-screen")
	}
	for i := 1; i < 24; i++ {
		if row := dropCells(frameRow(t, m, i), listW+1); strings.HasSuffix(row, "*") {
			t.Fatalf("row %d = %q carries the right star with the current line off-screen", i, row)
		}
	}
}

// A left gutter star and the reserved right star coexist: the current
// line has one match entirely hidden left and another entirely
// hidden right.
func TestBothStarsAppearTogether(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	// "one" at cells 0–2, "far" at cells 103–105.
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: "one" + strings.Repeat("x", 100) + "far\n",
		stops:   []navStop{{line: 1, start: 0, end: 3}, {line: 1, start: 103, end: 106}},
	}})
	key := flatIndicatorModel(t, m, idx, 3)
	listW := m.listWidth()
	panTo(t, m, key, 20)

	if row := dropCells(frameRow(t, m, 1), listW+1); row != "1* "+strings.Repeat("x", 40)+"*" {
		t.Fatalf("row = %q, want the gutter star and the right star together", row)
	}
}

// A match that is only partially hidden earns no hidden-match
// indicator for that side: straddling the right edge keeps the
// reserved column blank, and straddling the left edge leaves '_' —
// the star requires a match entirely hidden.
func TestPartiallyVisibleMatchShowsNoStar(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{{
		name: "a.txt",
		content: strings.Repeat("x", 38) + "needle" + strings.Repeat("x", 30) + "\n" +
			strings.Repeat("x", 8) + "needle" + strings.Repeat("x", 60) + "\n",
		stops: []navStop{
			{line: 1, start: 38, end: 44},
			{line: 2, start: 8, end: 14},
		},
	}})
	key := flatIndicatorModel(t, m, idx, 3)
	listW := m.listWidth()

	// At offset 0 the line-1 match at cells 38–43 paints cells 38–39:
	// partially visible, so no right star even on the current line.
	if row := dropCells(frameRow(t, m, 1), listW+1); row != "1  "+strings.Repeat("x", 38)+"ne"+" " {
		t.Fatalf("right-edge partial match: row = %q, want a blank reserved column", row)
	}

	// At offset 10 the line-2 match at cells 8–13 paints cells 10–13:
	// partially visible, so its gutter is '_', never '*'. Line 1's
	// match is fully painted at this offset — still no right star.
	panTo(t, m, key, 10)
	if row := dropCells(frameRow(t, m, 2), listW+1); row != "2_ "+"edle"+strings.Repeat("x", 36)+" " {
		t.Fatalf("left-edge partial match: row = %q, want '_' not '*'", row)
	}
	if row := dropCells(frameRow(t, m, 1), listW+1); row != "1_ "+strings.Repeat("x", 28)+"needle"+strings.Repeat("x", 6)+" " {
		t.Fatalf("now-visible match: row = %q, want a blank reserved column", row)
	}
}

// A match in the window's last text cell is painted, not hidden: the
// right star comes from a different match entirely beyond the edge.
// Panning until that far match paints removes the star.
func TestLastCellMatchWithFartherMatchHidden(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	// "near" at cells 36–39 (the last text cell), "far" at cells
	// 60–62.
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("x", 36) + "near" + strings.Repeat("x", 20) + "far" + strings.Repeat("x", 30) + "\n",
		stops:   []navStop{{line: 1, start: 36, end: 40}, {line: 1, start: 60, end: 63}},
	}})
	key := flatIndicatorModel(t, m, idx, 3)
	listW := m.listWidth()

	// "near" fills the last text cell and is fully painted; the star
	// reports only the entirely hidden "far".
	if row := dropCells(frameRow(t, m, 1), listW+1); row != "1  "+strings.Repeat("x", 36)+"near"+"*" {
		t.Fatalf("row = %q, want the last-cell match painted then the right star", row)
	}

	// Pan so "far" enters the window: both matches painted, the star
	// is gone — and the left gutter shows only '_'.
	panTo(t, m, key, 30)
	if row := dropCells(frameRow(t, m, 1), listW+1); row != "1_ "+strings.Repeat("x", 6)+"near"+strings.Repeat("x", 20)+"far"+strings.Repeat("x", 7)+" " {
		t.Fatalf("row = %q, want no star once the far match paints", row)
	}
}

// The indicator columns size by cells (Issue #39): a match on a
// two-cell glyph straddling the right edge paints nothing — its
// cluster counts hidden and earns the star — while the same glyph one
// cell inward paints whole and is visible, not hidden.
func TestWideMatchIndicatorColumns(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	// Line 1: "one" at cells 0–2 keeps the reveal at offset 0; 文
	// straddles cells 39–40 of the 40-cell window — a clip blank,
	// entirely hidden right.
	// Line 2: 文 sits at cells 38–39, fully painted inside the window.
	idx := navIndex(t, []navFile{{
		name: "a.txt",
		content: "one" + strings.Repeat("x", 36) + "文" + strings.Repeat("x", 30) + "\n" +
			strings.Repeat("x", 38) + "文" + strings.Repeat("x", 30) + "\n",
		stops: []navStop{
			{line: 1, start: 0, end: 3}, {line: 1, start: 39, end: 42},
			{line: 2, start: 38, end: 41},
		},
	}})
	flatIndicatorModel(t, m, idx, 3)
	listW := m.listWidth()

	// Line 1 is current: the straddling cluster's in-window cell is a
	// clip blank and the reserved column stars.
	if row := dropCells(frameRow(t, m, 1), listW+1); row != "1  one"+strings.Repeat("x", 36)+" *" {
		t.Fatalf("straddling wide match: row = %q, want a clip blank then the right star", row)
	}
	// Line 2's match is painted, so its reserved column stays blank —
	// the two-cell glyph paints at cells 38–39 as one unit.
	if row := dropCells(frameRow(t, m, 2), listW+1); row != "2  "+strings.Repeat("x", 38)+"文 " {
		t.Fatalf("painted wide match: row = %q, want the whole glyph then a blank column", row)
	}
}

// A wide glyph clipped to blanks counts as hidden, not partially
// visible: a match on the clipped 文 upgrades the left gutter to '*'
// and earns the right star — the painted-cell visibility rule the
// indicators share with the horizontal reveal.
func TestSplitGlyphBlanksCountHidden(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	// Line 1: 文 spans cells 39–40 — clipped blank at the right edge of
	// a 40-cell window. Its first submatch at cell 0 keeps the startup
	// reveal at offset 0.
	// Line 2: 文 spans cells 2–3 — clipped blank once the offset is 3.
	idx := navIndex(t, []navFile{{
		name: "a.txt",
		content: strings.Repeat("x", 39) + "文" + strings.Repeat("x", 30) + "\n" +
			"ab文" + strings.Repeat("x", 60) + "\n",
		stops: []navStop{
			{line: 1, start: 0, end: 1}, {line: 1, start: 39, end: 42},
			{line: 2, start: 2, end: 5},
		},
	}})
	key := flatIndicatorModel(t, m, idx, 3)
	listW := m.listWidth()

	// 文 renders its clipped in-window cell as a blank — not a
	// partially visible match — so the current line earns the right
	// star.
	if row := dropCells(frameRow(t, m, 1), listW+1); row != "1  "+strings.Repeat("x", 39)+" "+"*" {
		t.Fatalf("right-edge clipped 文: row = %q, want a blank text cell then the star", row)
	}

	// At offset 3 the line-2 文 is clipped blank at the left edge:
	// entirely hidden left, so the gutter upgrades to '*'. Line 1's
	// 文 now paints whole, and its cell-0 match is hidden left — the
	// left star shows while the right one is gone.
	panTo(t, m, key, 3)
	if row := dropCells(frameRow(t, m, 2), listW+1); row != "2*  "+strings.Repeat("x", 39)+" " {
		t.Fatalf("left-edge clipped 文: row = %q, want '*' then the clipping blank", row)
	}
	if row := dropCells(frameRow(t, m, 1), listW+1); row != "1* "+strings.Repeat("x", 36)+"文"+strings.Repeat("x", 2)+" " {
		t.Fatalf("painted 文: row = %q, want the left star only", row)
	}
}

// The uniform-lines case: when every visible line has text hidden
// left at a nonzero offset, every visible row carries the gutter
// mark — the pan clamp's painted-cluster guarantee bounds painted
// cells, not indicator counts.
func TestUniformLinesMarkEveryVisibleRow(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat(strings.Repeat("x", 100)+"\n", 30),
		stops:   []navStop{{line: 1, start: 30, end: 33}},
	}})
	key := flatIndicatorModel(t, m, idx, 4)
	listW := m.listWidth()
	panTo(t, m, key, 20)

	// All 23 visible rows show '_': every line has text hidden left
	// and the only match — line 1's at cells 30–32 — is painted.
	for i := 1; i <= 23; i++ {
		row := dropCells(frameRow(t, m, i), listW+1)
		if row[2] != '_' {
			t.Fatalf("row %d = %q, want '_' in the gutter's first trailing space", i, row)
		}
		if !strings.HasSuffix(row, " ") {
			t.Fatalf("row %d = %q, want a blank reserved column", i, row)
		}
	}
}

// Wrap mode draws neither indicator nor the reserved column: the
// gutter keeps both trailing spaces and the text area claims the cell
// the run-off-edge layout reserves.
func TestWrapModeDrawsNoIndicators(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("x", 300) + "needle\nsecond\n",
		stops:   []navStop{{line: 1, start: 295, end: 301}},
	}})
	flatIndicatorModel(t, m, idx, 3) // installs the flat layout
	listW := m.listWidth()

	// In run-off-edge mode at offset 0 the far "needle" match is
	// entirely hidden right: the reserved column proves itself.
	if row := dropCells(frameRow(t, m, 1), listW+1); !strings.HasSuffix(row, "*") {
		t.Fatalf("flat-mode row = %q, want the right star", row)
	}

	// Back in wrap mode the indicators and the reserved column are
	// gone: the first row's gutter keeps both trailing spaces and the
	// text area claims the last column.
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	textW := m.textWidth(3)
	if row := dropCells(frameRow(t, m, 1), listW+1); row != "1  "+strings.Repeat("x", textW) {
		t.Fatalf("wrap-mode row = %q, want a blank gutter and text through the last column", row)
	}
	for i := 1; i <= 8; i++ {
		row := dropCells(frameRow(t, m, i), listW+1)
		if row[1] != ' ' {
			t.Fatalf("wrapped row %d = %q carries a gutter mark", i, row)
		}
		if strings.Contains(row, "*") {
			t.Fatalf("wrapped row %d = %q carries a right star", i, row)
		}
	}
}

// Under a real scheme both marks render in the theme's inverse
// indicator style — never as ordinary gutter or text cells.
func TestIndicatorsRenderInverseStyled(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{{
		name: "a.txt",
		content: "one" + strings.Repeat("x", 100) + "far\n" +
			strings.Repeat("x", 60) + "\n",
		stops: []navStop{{line: 1, start: 0, end: 3}, {line: 1, start: 103, end: 106}},
	}})
	key := flatIndicatorModel(t, m, idx, 3)
	panTo(t, m, key, 20)

	v := viewText(m)
	// The dark scheme's inverse is 30;47: line 1 shows the gutter '*'
	// and the right '*', line 2 shows the gutter '_'. No match text is
	// on screen at this offset, so the inverse SGR belongs to the
	// indicators alone.
	if n := strings.Count(v, "\x1b[30;47m*\x1b[37;40;24m"); n != 2 {
		t.Fatalf("view has %d inverse stars, want 2 (gutter + reserved column)", n)
	}
	if n := strings.Count(v, "\x1b[30;47m_\x1b[37;40;24m"); n != 1 {
		t.Fatalf("view has %d inverse underscores, want 1", n)
	}
}
