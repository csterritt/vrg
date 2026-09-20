package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// indicatorCells extracts a plain-theme content row's two indicator
// positions: the first trailing gutter space — the hidden-left slot at
// cell lw + gw − 2 — and the reserved rightmost column, always the
// row's last cell. List padding and gutter digits are ASCII, so rune
// indexes coincide with cell indexes in the gutter region.
func indicatorCells(t *testing.T, row string, lw, gw int) (left, right rune) {
	t.Helper()
	r := []rune(row)
	if len(r) <= lw+gw {
		t.Fatalf("row %q too short for a gutter at lw=%d gw=%d", row, lw, gw)
	}
	return r[lw+gw-2], r[len(r)-1]
}

// In run-off-edge mode every visible source line's first trailing
// gutter space is the hidden-left indicator: an inverse _ when text is
// hidden left — on every line of a uniform fixture — upgraded to an
// inverse * when a match on that line is entirely hidden left, and
// blank when the line has nothing hidden. The gutter scope is every
// visible line, not only the current matched line.
func TestGutterHiddenLeftIndicators(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 100)
	var b strings.Builder
	for i := 1; i <= 30; i++ {
		if i == 10 { // an empty line: nothing of it can be hidden
			b.WriteString("\n")
			continue
		}
		b.WriteString(long + "\n")
	}
	writeMatchFile(t, dir, "a.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 0, 4, "xxxx"))
	addRec(t, idx, matchRec("a.txt", long+"\n", 3, 0, 4, "xxxx"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m, _ = update(t, m, keyMsg(">")) // offset 10

	// The styled frame paints each indicator in the theme's inverse.
	if got := viewRow(t, m, 1); !strings.Contains(got, "\x1b[30;47m*\x1b[37;40m") {
		t.Fatalf("line 1 gutter lacks the inverse * indicator: %q", got)
	}
	if got := viewRow(t, m, 2); !strings.Contains(got, "\x1b[30;47m_\x1b[37;40m") {
		t.Fatalf("line 2 gutter lacks the inverse _ indicator: %q", got)
	}

	// List width 7, gutter 4 (30 lines) → indicator cell 9; content
	// height 23 shows lines 1–23. The matched lines 1 and 3 hide their
	// whole [0,4) match left of the offset → *; every other nonempty
	// line hides text → _; the empty line 10 hides nothing → blank.
	m.theme = theme.Plain()
	for r := 1; r <= 23; r++ {
		left, right := indicatorCells(t, viewRow(t, m, r), 7, 4)
		want := '_'
		if r == 1 || r == 3 {
			want = '*'
		}
		if r == 10 {
			want = ' '
		}
		if left != want {
			t.Fatalf("row %d gutter indicator = %q, want %q", r, left, want)
		}
		if right != ' ' {
			t.Fatalf("row %d reserved column = %q, want blank", r, right)
		}
	}
}

// The reserved rightmost column shows the hidden-right indicator only
// on the current matched line's row: a non-current line whose match is
// entirely hidden right stays blank. The star is inverse-styled.
func TestRightIndicatorCurrentLineOnly(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 100)
	content := strings.Repeat(long+"\n", 9)
	writeMatchFile(t, dir, "a.txt", content)
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 0, 4, "xxxx"))    // visible
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 90, 95, "xxxxx")) // hidden right
	addRec(t, idx, matchRec("a.txt", long+"\n", 5, 90, 95, "xxxxx")) // hidden right, non-current
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)

	if row := viewRow(t, m, 1); !strings.Contains(row, "\x1b[30;47m*\x1b[37;40m") {
		t.Fatalf("current line's reserved column lacks the inverse *: %q", row)
	}

	m.theme = theme.Plain()
	// 9 lines → gutter 3, text width 69, reserved column last.
	for r := 1; r <= 9; r++ {
		left, right := indicatorCells(t, viewRow(t, m, r), 7, 3)
		want := ' '
		if r == 1 {
			want = '*'
		}
		if right != want {
			t.Fatalf("row %d reserved column = %q, want %q", r, right, want)
		}
		if left != ' ' {
			t.Fatalf("row %d gutter indicator = %q, want blank at offset 0", r, left)
		}
	}
}

// When the current matched line is vertically off-screen its right
// indicator is absent, while other lines' gutter indicators remain.
func TestRightIndicatorAbsentWhenCurrentLineOffScreen(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 100)
	content := strings.Repeat(long+"\n", 30)
	writeMatchFile(t, dir, "a.txt", content)
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 0, 4, "xxxx"))
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 90, 95, "xxxxx"))
	addRec(t, idx, matchRec("a.txt", long+"\n", 15, 90, 95, "xxxxx")) // non-current hidden right
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m, _ = update(t, m, keyMsg(">")) // offset 10

	// Page down to top 7 — lines 8–30 visible, the current line 1 gone.
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	m.theme = theme.Plain()
	for r := 1; r <= 23; r++ {
		left, right := indicatorCells(t, viewRow(t, m, r), 7, 4)
		if right != ' ' {
			t.Fatalf("row %d reserved column = %q, want blank with the current line off-screen", r, right)
		}
		if left != '_' {
			t.Fatalf("row %d gutter indicator = %q, want _", r, left)
		}
	}
}

// Both stars appear together on the current matched line: a match
// entirely hidden left upgrades the gutter to * while a second match
// entirely hidden right paints the reserved column.
func TestBothSidesHiddenShowBothStars(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 200)
	writeMatchFile(t, dir, "a.txt", long+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 0, 4, "xxxx"))
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 190, 195, "xxxxx"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, keyMsg(">"))
	} // offset 50; text window [50, 119)

	m.theme = theme.Plain()
	left, right := indicatorCells(t, viewRow(t, m, 1), 7, 3)
	if left != '*' || right != '*' {
		t.Fatalf("indicators = %q/%q, want */* — both sides hidden", left, right)
	}
}

// A partially visible match produces no hidden-match indicator for
// that side: a match straddling the left window edge keeps the gutter
// at _ — text is hidden left but no whole match is — and a match
// straddling the right edge leaves the reserved column blank.
func TestPartiallyVisibleMatchShowsNoStar(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 100)
	writeMatchFile(t, dir, "a.txt", long+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 5, 15, "xxxxxxxxxx"))  // straddles [10,79) left
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 75, 85, "xxxxxxxxxx")) // straddles right
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m, _ = update(t, m, keyMsg(">")) // offset 10; window [10, 79)

	m.theme = theme.Plain()
	left, right := indicatorCells(t, viewRow(t, m, 1), 7, 3)
	if left != '_' {
		t.Fatalf("gutter indicator = %q, want _ — the match is only partially hidden left", left)
	}
	if right != ' ' {
		t.Fatalf("reserved column = %q, want blank — the match is partially visible right", right)
	}
}

// A match painted in the last text cell does not block the reserved
// column: a second match entirely farther right still earns the
// inverse *, and the star never overwrites the last text cell.
func TestLastCellMatchStillShowsRightStar(t *testing.T) {
	dir := t.TempDir()
	// 100 cells: "AAA" ends at the last text cell (column 68 of width
	// 69); "BBBBB" sits at columns 85–89.
	line := strings.Repeat("x", 66) + "AAA" + strings.Repeat("x", 16) + "BBBBB" + strings.Repeat("x", 10)
	writeMatchFile(t, dir, "a.txt", line+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", line+"\n", 1, 66, 69, "AAA"))
	addRec(t, idx, matchRec("a.txt", line+"\n", 1, 85, 90, "BBBBB"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)

	m.theme = theme.Plain()
	r := []rune(viewRow(t, m, 1))
	if got := string(r[76:79]); got != "AAA" {
		t.Fatalf("last text cells = %q, want AAA — the star must not overwrite text", got)
	}
	if r[len(r)-1] != '*' {
		t.Fatalf("reserved column = %q, want *", r[len(r)-1])
	}
}

// A match on a wide cluster split by the window edge renders as
// blanks, which are not visible: entirely blank-rendered, it counts as
// a hidden match and upgrades the gutter to * rather than _.
func TestSplitGlyphMatchCountsAsHidden(t *testing.T) {
	dir := t.TempDir()
	line1 := "世" + strings.Repeat("x", 99)
	writeMatchFile(t, dir, "a.txt", line1+"\n"+strings.Repeat("x", 100)+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", line1+"\n", 1, 0, 3, "世"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m, _ = update(t, m, keyMsg(".")) // offset 1 splits 世's cells [0,2)

	m.theme = theme.Plain()
	if left, _ := indicatorCells(t, viewRow(t, m, 1), 7, 3); left != '*' {
		t.Fatalf("line 1 gutter indicator = %q, want * — the blanked 世 match is entirely hidden left", left)
	}
	if left, _ := indicatorCells(t, viewRow(t, m, 2), 7, 3); left != '_' {
		t.Fatalf("line 2 gutter indicator = %q, want _", left)
	}
}

// Wrap mode draws neither hidden-content indicators nor the reserved
// column: text paints through the panel's last cell and no gutter
// position ever shows _ or *.
func TestWrapModeHasNoIndicators(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 200)
	writeMatchFile(t, dir, "a.txt", long+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 0, 4, "xxxx"))
	addRec(t, idx, matchRec("a.txt", long+"\n", 1, 190, 195, "xxxxx"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// Wrap text width 70 wraps the 200-cell line over three rows; the
	// last panel cell is text, not a reserved blank.
	row := viewRow(t, m, 1)
	if last := []rune(row)[len([]rune(row))-1]; last != 'x' {
		t.Fatalf("wrapped row's last cell = %q, want text — no reserved column", last)
	}
	for r := 1; r <= 3; r++ {
		if left, _ := indicatorCells(t, viewRow(t, m, r), 7, 3); left != ' ' {
			t.Fatalf("wrapped row %d shows gutter indicator %q", r, left)
		}
	}
}
