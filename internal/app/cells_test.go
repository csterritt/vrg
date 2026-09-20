package app

import (
	"strings"
	"testing"

	"github.com/clipperhouse/displaywidth"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// cellsFixture enters the browse state over one real file whose line-1
// match covers bytes [start,end), keeping the styled theme so the
// emitted SGR layout is inspectable at cell level.
func cellsFixture(t *testing.T, name, content string, start, end int, match string, w, h int) Model {
	t.Helper()
	dir := t.TempDir()
	writeMatchFile(t, dir, name, content)
	idx := searchindex.New(dir)
	line := strings.SplitN(content, "\n", 2)[0]
	addRec(t, idx, matchRec(name, line+"\n", 1, start, end, match))
	idx.Finish()
	m, cmd := startBrowse(t, dir, idx, w, h)
	m = applyLoad(t, m, cmd())
	return m
}

// A match overlapping a two-cell CJK cluster paints the inverse match
// style over exactly that cluster's cells: the neighbouring characters
// stay in base colours — the styled run neither swallows the following
// character nor leaves the cluster half-painted.
func TestMatchOverlappingCJKCoversExactlyItsCluster(t *testing.T) {
	// Bytes: a=0, 世=E4 B8 96 at [1,4), b=4.
	m := cellsFixture(t, "a.txt", "a世b\n", 1, 4, "世", 80, 24)
	row := viewRow(t, m, 1)
	want := "a\x1b[4m\x1b[30;47m世\x1b[24m\x1b[37;40mb"
	if !strings.Contains(row, want) {
		t.Fatalf("content row = %q, want the CJK cluster styled alone inside %q", row, want)
	}
}

// A base-plus-combining sequence styles as one cluster — the inverse
// run holds base and mark together — and in run-off-edge mode the
// cluster leaves the window whole rather than painting a bare
// combining mark or a lone base.
func TestCombiningClusterStyledAndClippedWhole(t *testing.T) {
	// Bytes: x=0, e=1, ◌́=CC 81 at [2,4) — the match [1,4) is "é".
	m := cellsFixture(t, "a.txt", "xéy\n", 1, 4, "é", 80, 24)
	row := viewRow(t, m, 1)
	want := "x\x1b[4m\x1b[30;47mé\x1b[24m\x1b[37;40my"
	if !strings.Contains(row, want) {
		t.Fatalf("content row = %q, want the combining cluster styled as one inside %q", row, want)
	}

	// Pan the text window past the width-one cluster: it drops whole,
	// its hidden match upgrades the gutter to *, and the row stays a
	// complete 80 cells.
	m, cmd := update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m, _ = update(t, m, keyMsg(">")) // offset clamps to 2
	m.theme = theme.Plain()
	want = "       1* y" + strings.Repeat(" ", 68) + " "
	if row = viewRow(t, m, 1); row != want {
		t.Fatalf("panned row = %q, want %q — the cluster left the window whole", row, want)
	}
}

// An emoji ZWJ sequence is one cluster of measured width two: a match
// covering only part of its bytes paints the whole sequence as one
// styled run, and a window edge inside the cluster blanks it rather
// than drawing a fragment.
func TestEmojiZWJClusterNeverSplits(t *testing.T) {
	// Bytes: a=0, 👨 at [1,5), then ZWJ 👩 ZWJ 👧 through byte 19, b=19.
	// The recorded match covers only the first emoji's bytes.
	m := cellsFixture(t, "a.txt", "a👨‍👩‍👧b\n", 1, 5, "👨", 80, 24)
	row := viewRow(t, m, 1)
	want := "a\x1b[4m\x1b[30;47m👨‍👩‍👧\x1b[24m\x1b[37;40mb"
	if !strings.Contains(row, want) {
		t.Fatalf("content row = %q, want the whole ZWJ cluster styled inside %q", row, want)
	}

	m, cmd := update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m, _ = update(t, m, keyMsg(".")) // offset 1: cluster [1,3) still paints whole
	m.theme = theme.Plain()
	want = "       1_ 👨‍👩‍👧b" + strings.Repeat(" ", 66) + " "
	if row = viewRow(t, m, 1); row != want {
		t.Fatalf("offset-1 row = %q, want %q", row, want)
	}

	m, _ = update(t, m, keyMsg(".")) // offset 2 splits the cluster: one blank, then b
	row = viewRow(t, m, 1)
	want = "       1*  b" + strings.Repeat(" ", 67) + " "
	if row != want {
		t.Fatalf("offset-2 row = %q, want %q — no partial emoji may render", row, want)
	}
	if strings.Contains(row, "👨") || strings.Contains(row, "‍") {
		t.Fatalf("offset-2 row contains a ZWJ fragment: %q", row)
	}
}

// wrapCells' too-wide-cluster fallback emits the whole cluster: a
// multi-rune sequence — an emoji ZWJ cluster or a base-plus-combining
// sequence — is never parted even when it cannot fit the wrap width.
func TestWrapCellsNeverSplitsCluster(t *testing.T) {
	if got := wrapCells("👨‍👩‍👧x", 1); len(got) != 2 || got[0] != "👨‍👩‍👧" || got[1] != "x" {
		t.Fatalf(`wrapCells("👨‍👩‍👧x", 1) = %q, want the whole cluster then x`, got)
	}
	if got := wrapCells("éx", 1); len(got) != 2 || got[0] != "é" || got[1] != "x" {
		t.Fatalf(`wrapCells("éx", 1) = %q, want the combining cluster then x`, got)
	}
}

// File-list entries pad by measured cells: a wide rune counts as two
// cells, so the padded column ends exactly at the gutter.
func TestFileListPadsWideNamesByCells(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	writeMatchFile(t, dir, "日.txt", "hit c\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("日.txt", "hit c\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// 日.txt is 6 cells → the column is 8; the current entry pads to it
	// before the filename rule, the other before the gutter.
	if got := viewRow(t, m, 0); !strings.HasPrefix(got, "b.txt   ──") {
		t.Fatalf("current entry row = %q, want padding to the 8-cell column", got)
	}
	if got := viewRow(t, m, 1); !strings.HasPrefix(got, "日.txt  1  ") {
		t.Fatalf("wide entry row = %q, want two pad cells before the gutter", got)
	}
}

// The filename rule fits wide and combining paths by measured cells,
// truncating only at cluster boundaries.
func TestFilenameRuleFitsWideCombiningPaths(t *testing.T) {
	for _, tc := range []struct {
		name, file, note string
		w                int
		want             string
	}{
		{"wide path fits", "日.txt", "", 12, "── 日.txt ──"},
		{"combining cluster kept at the seam", "dddéf.txt", "", 12, "── …éf.txt ─"},
		{"wide path truncates whole", "日本語.txt", "n", 14, "── ….txt ── n "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := filenameRule(tc.file, tc.note, tc.w); got != tc.want {
				t.Fatalf("filenameRule(%q, %q, %d) = %q, want %q",
					tc.file, tc.note, tc.w, got, tc.want)
			}
		})
	}
}

// A wide/combining destination path is measured in cells inside the
// file-change pop-up: the box width and centring follow the cluster
// model.
func TestPopupWideCombiningPathCentred(t *testing.T) {
	dir := t.TempDir()
	name := "日本語é.txt" // 6 + 1 + 4 = 11 cells
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, name, "hit w\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec(name, "hit w\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 40, 12)
	m = applyLoad(t, m, cmd())
	m, _ = update(t, m, keyMsg("n"))
	m.theme = theme.Plain()

	// Box width 13 → left edge (40-13)/2 = 13; interior is row 5.
	row, col := popupBox(t, m.View().Content, name)
	if row != 5 || col != 13 {
		t.Fatalf("pop-up interior at (%d,%d), want (5,13) — centred by measured cells", row, col)
	}
}

// A path too wide for the terminal left-truncates on a cluster
// boundary: when the cut lands on a base-plus-combining cluster the
// kept suffix opens with the whole cluster, never a bare combining
// mark — EscapePath passes printable combining text through and the
// pop-up may not assume otherwise.
func TestPopupTruncateLeftKeepsCombiningCluster(t *testing.T) {
	dir := t.TempDir()
	name := strings.Repeat("d", 11) + "é" + strings.Repeat("g", 17)
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec(name, "hit z\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 21, 5)
	m = applyLoad(t, m, cmd())
	m, _ = update(t, m, keyMsg("n"))
	m.theme = theme.Plain()

	// Interior width 19: the kept suffix is the whole é cluster plus
	// the 17 trailing g's behind a leading ellipsis.
	want := "…é" + strings.Repeat("g", 17)
	v := m.View().Content
	row, col := popupBox(t, v, want)
	if row != 2 || col != 0 {
		t.Fatalf("pop-up interior at (%d,%d), want (2,0) at 21x5:\n%s", row, col, v)
	}
	if strings.Contains(v, "…́") {
		t.Fatalf("pop-up opens the suffix with a bare combining mark:\n%s", v)
	}
	for i, r := range strings.Split(v, "\n") {
		if w := displaywidth.String(r); w > 21 {
			t.Fatalf("row %d is %d cells wide at width 21:\n%q", i, w, r)
		}
	}
}

// A wide cluster straddling the run-off-edge window's right edge
// renders blank rather than half-drawn, and the row stays a complete
// 80 cells including the reserved indicator column.
func TestWideClusterSplitAtRightEdgeRendersBlank(t *testing.T) {
	// x*68 occupies columns 0–67; 世 spans columns 68–69, split by the
	// 69-cell window's right edge; z at column 70 is out entirely.
	line := strings.Repeat("x", 68) + "世z"
	m := cellsFixture(t, "a.txt", line+"\n", 0, 4, "xxxx", 80, 24)
	m, cmd := update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m.theme = theme.Plain()

	row := viewRow(t, m, 1)
	want := "       1  " + strings.Repeat("x", 68) + "  "
	if row != want {
		t.Fatalf("row = %q, want %q — the split cluster paints blank", row, want)
	}
	if strings.Contains(row, "世") || strings.Contains(row, "z") {
		t.Fatalf("row contains a split or out-of-window cluster: %q", row)
	}
	if got := displaywidth.String(row); got != 80 {
		t.Fatalf("row is %d cells, want 80", got)
	}
}

// The reserved right-edge indicator column composes correctly when the
// hidden-right match sits behind wide text: the star paints in the
// row's last cell and the row keeps its full width.
func TestRightIndicatorWithWideText(t *testing.T) {
	line := strings.Repeat("x", 80) + "世hit" // hit at bytes [83,86)
	m := cellsFixture(t, "a.txt", line+"\n", 83, 86, "hit", 80, 24)
	// The startup reveal ran in wrap mode, leaving the offset at zero;
	// unwrapping shows the window [0,69) with the match hidden right.
	m, cmd := update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	m.theme = theme.Plain()

	row := viewRow(t, m, 1)
	if !strings.HasSuffix(row, "*") {
		t.Fatalf("row = %q, want the reserved column's * for the hidden-right match", row)
	}
	if strings.Contains(row, "hit") || strings.Contains(row, "世") {
		t.Fatalf("row paints hidden-right content: %q", row)
	}
	if got := displaywidth.String(row); got != 80 {
		t.Fatalf("row is %d cells, want 80", got)
	}
}
