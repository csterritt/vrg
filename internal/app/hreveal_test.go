package app

import (
	"strings"
	"testing"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// The startup reveal applies the minimal horizontal reveal too: when
// the first layout that can commit the pending reveal is a
// run-off-edge one, a target hidden right of the text window moves the
// offset by exactly enough to paint its start cell at the right edge.
func TestStartupRevealAppliesHorizontalReveal(t *testing.T) {
	dir := t.TempDir()
	line := strings.Repeat("x", 100) + "hit"
	writeMatchFile(t, dir, "a.txt", line+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", line+"\n", 1, 100, 103, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	// Enter run-off-edge mode before the load lands, so the pending
	// startup reveal commits against the unwrapped layout.
	m, _ = update(t, m, keyMsg("w"))
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// Text width 69: the match's start cell at column 100 reveals to
	// 100 + 1 - 69 = 32 — no further.
	if got := m.vps["a.txt"].Offset(); got != 32 {
		t.Fatalf("startup reveal: offset = %d, want 32", got)
	}
	if row := contentRow(t, m); !strings.HasSuffix(row, "h ") {
		t.Fatalf("first content row = %q, want the match's start cell at the right edge", row)
	}
}

// A wide target cluster reveals far enough to paint its start cell,
// not just to touch the window: the CJK match at column 100 needs both
// of its cells, so the offset lands on 100 + 2 − 69 and the rendered
// row shows the glyph whole at the right edge — never a blank.
func TestStartupRevealPaintsWideClusterWhole(t *testing.T) {
	dir := t.TempDir()
	line := strings.Repeat("x", 100) + "世zz"
	writeMatchFile(t, dir, "a.txt", line+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", line+"\n", 1, 100, 103, "世"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m, _ = update(t, m, keyMsg("w"))
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	if got := m.vps["a.txt"].Offset(); got != 33 {
		t.Fatalf("wide-cluster reveal: offset = %d, want 33 (100 + 2 − 69)", got)
	}
	if row := contentRow(t, m); !strings.HasSuffix(row, "世 ") {
		t.Fatalf("first content row = %q, want 世 fully painted at the right edge", row)
	}
}

// Same-file n and p trigger the horizontal reveal too: a target hidden
// right of the window moves the offset by the minimum that paints it,
// and a target left of the window drops the offset to its column —
// even though both target rows were already vertically visible.
func TestSameFileNavigationAppliesHorizontalReveal(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt",
		"xxxxxaa\n"+strings.Repeat("x", 100)+"bb\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "xxxxxaa\n", 1, 5, 7, "aa"))
	addRec(t, idx, matchRec("a.txt", strings.Repeat("x", 100)+"bb\n", 2, 100, 102, "bb"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)

	// The cursor sits on line 1's column-5 target; panning it out of
	// view changes nothing until a navigation action reveals.
	m, _ = update(t, m, keyMsg(">"))
	if got := m.vps["a.txt"].Offset(); got != 10 {
		t.Fatalf("setup pan: offset = %d, want 10", got)
	}

	m, cmd = update(t, m, keyMsg("n")) // → line 2, column 100
	if cmd != nil {
		t.Fatalf("same-file n returned a command: %v", cmd)
	}
	if got := m.vps["a.txt"].Offset(); got != 32 {
		t.Fatalf("n to a right-hidden target: offset = %d, want 32", got)
	}

	m, _ = update(t, m, keyMsg("n")) // wraps to line 1, column 5
	if got := m.vps["a.txt"].Offset(); got != 5 {
		t.Fatalf("n to a left-hidden target: offset = %d, want 5", got)
	}

	m, _ = update(t, m, keyMsg("p")) // back to line 2, column 100
	if got := m.vps["a.txt"].Offset(); got != 32 {
		t.Fatalf("p to a right-hidden target: offset = %d, want 32", got)
	}
	m, _ = update(t, m, keyMsg("p")) // wraps back to line 1
	if got := m.vps["a.txt"].Offset(); got != 5 {
		t.Fatalf("p to a left-hidden target: offset = %d, want 5", got)
	}
}

// On a file change the Issue 18 offset reset lands before the
// horizontal reveal: the destination's stored pan is zeroed first,
// then the reveal moves the offset by the minimum that paints the
// target. The 32 here proves that ordering — reveal-before-reset
// would leave 0, no reset would leave the stored 50.
func TestFileChangeResetsThenRevealsHorizontally(t *testing.T) {
	dir := t.TempDir()
	aline := strings.Repeat("x", 200)
	writeMatchFile(t, dir, "a.txt", aline+"\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", aline+"\n", 1, 100, 102, "xx"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, keyMsg(">"))
	}
	if got := m.vps["a.txt"].Offset(); got != 50 {
		t.Fatalf("setup pan: offset = %d, want 50", got)
	}

	m, cmd = update(t, m, keyMsg("n")) // → b.txt
	m = applyLoad(t, m, deliverNavLoad(t, cmd))
	if got := m.vps["b.txt"].Offset(); got != 0 {
		t.Fatalf("b.txt offset = %d, want the reset 0", got)
	}

	m, _ = update(t, m, keyMsg("p")) // → a.txt, cached: reset then reveal
	if got := m.vps["a.txt"].Offset(); got != 32 {
		t.Fatalf("revisit offset = %d, want 32 — reset then minimal reveal", got)
	}
}
