package app

import (
	"strings"
	"testing"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// A standalone combining cluster renders the recorded fallback cell —
// U+25CC DOTTED CIRCLE carrying the cluster's marks — and a match on
// the cluster highlights that one visible cell, never a zero-width
// position. Line 1 is the current stop, so the fallback paints in the
// underlined inverse current-match style.
func TestStandaloneCombiningClusterRendersFallbackCell(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "́x\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "́x\n", 1, 0, 2, "́"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())

	row := viewRow(t, m, 1)
	if !strings.Contains(row, "\x1b[4m\x1b[30;47m◌́\x1b[24m\x1b[37;40m") {
		t.Fatalf("styled row = %q, want the ◌́ fallback cell inverse-underlined", row)
	}
	m.theme = theme.Plain()
	if row := viewRow(t, m, 1); !strings.Contains(row, "◌́x") {
		t.Fatalf("plain row = %q, want the ◌́ fallback cell followed by x", row)
	}
}

// Wrap mode counts the fallback cell like any other cell: the
// standalone combining cluster leads the line as one real cell, so the
// row's 70-cell budget paints it plus 69 following cells and the rest
// wraps whole to the continuation row — no overlap and no shared cell.
// Were the fallback only a zero-width annotation, 70 x's would fit the
// first row.
func TestStandaloneCombiningClusterWrapsAsARealCell(t *testing.T) {
	dir := t.TempDir()
	line := "́" + strings.Repeat("x", 70) + "z"
	writeMatchFile(t, dir, "a.txt", line+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", line+"\n", 1, 0, 2, "́"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())

	row := viewRow(t, m, 1)
	if !strings.Contains(row,
		"\x1b[4m\x1b[30;47m◌́\x1b[24m\x1b[37;40m"+strings.Repeat("x", 69)) {
		t.Fatalf("wrap row = %q, want the highlighted fallback cell then 69 x's", row)
	}
	m.theme = theme.Plain()
	if row := viewRow(t, m, 2); row[7:10] != "   " || strings.TrimSpace(row[10:]) != "xz" {
		t.Fatalf("continuation row = %q, want a blank gutter then the wrapped xz", row)
	}
}

// Clipping and horizontal panning count the fallback cell like any
// other cell: one column of pan drops the one-cell fallback cluster
// wholly left of the window — never half-drawn — the following text
// shifts up by that one cell, and the match now entirely hidden left
// earns the gutter's inverse *.
func TestStandaloneCombiningClusterPansAsARealCell(t *testing.T) {
	dir := t.TempDir()
	line := "́" + strings.Repeat("x", 100)
	writeMatchFile(t, dir, "a.txt", line+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", line+"\n", 1, 0, 2, "́"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)

	// Run-off-edge text width 69: the highlighted fallback cell leads
	// the row, followed by 68 x's.
	if row := viewRow(t, m, 1); !strings.Contains(row, "\x1b[4m\x1b[30;47m◌́\x1b[24m\x1b[37;40m") {
		t.Fatalf("row = %q, want the highlighted fallback cell leading", row)
	}
	m, _ = update(t, m, keyMsg("."))
	if got := m.vps["a.txt"].Offset(); got != 1 {
		t.Fatalf("pan offset = %d, want 1", got)
	}
	row := viewRow(t, m, 1)
	if strings.Contains(row, "◌") {
		t.Fatalf("row still shows the panned-out fallback cell: %q", row)
	}
	if !strings.Contains(row, "\x1b[30;47m*\x1b[37;40m") {
		t.Fatalf("row lacks the hidden-left match indicator: %q", row)
	}
	if !strings.Contains(row, strings.Repeat("x", 69)) {
		t.Fatalf("row = %q, want the window's 69 x's starting at cell 1", row)
	}
}

// A highlight at a wrap boundary does not paint the blank filler cell:
// 世 cannot fit row 1's last cell and moves to row 2, so row 1 ends in
// an unpainted blank while row 2 carries the whole highlighted glyph.
func TestWrapBoundaryBlankIsNotPaintedAsMatch(t *testing.T) {
	dir := t.TempDir()
	line := strings.Repeat("x", 69) + "世"
	writeMatchFile(t, dir, "a.txt", line+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", line+"\n", 1, 69, 72, "世"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())

	// Wrap text width 70: 69 x's fill row 1 and 世 (width 2) moves to
	// row 2, leaving one blank filler cell at row 1's right edge. No
	// inverse styling may appear on row 1 — the filler is no match cell.
	if row := viewRow(t, m, 1); strings.Contains(row, "\x1b[30;47m") {
		t.Fatalf("wrap row before the boundary paints a match cell: %q", row)
	}
	if row := viewRow(t, m, 2); !strings.Contains(row, "\x1b[4m\x1b[30;47m世\x1b[24m\x1b[37;40m") {
		t.Fatalf("continuation row = %q, want the whole 世 highlighted", row)
	}
}

// Clipping blanks are never painted as match cells: an offset that
// splits the matched cluster renders its in-window columns as a plain
// blank, while the now-entirely-hidden match still earns the gutter's
// inverse * — the row's only inverse segment.
func TestClipBlankIsNotPaintedAsMatch(t *testing.T) {
	dir := t.TempDir()
	line1 := "世" + strings.Repeat("x", 99)
	writeMatchFile(t, dir, "a.txt", line1+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", line1+"\n", 1, 0, 3, "世"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m, _ = update(t, m, keyMsg(".")) // offset 1 splits 世's columns [0,2)

	row := viewRow(t, m, 1)
	if n := strings.Count(row, "\x1b[30;47m"); n != 1 {
		t.Fatalf("row carries %d inverse segments, want only the hidden-match indicator: %q", n, row)
	}
	if !strings.Contains(row, "\x1b[30;47m*\x1b[37;40m") {
		t.Fatalf("row lacks the inverse * indicator: %q", row)
	}
}
