package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/theme"
)

// hrevealFile is the three-stop same-file fixture: matches at cells 5,
// 300, and 270 on consecutive lines, so n steps through a right reveal,
// a no-move reveal, and a wrap-around left reveal.
func hrevealFile() navFile {
	return navFile{
		name: "a.txt",
		content: strings.Repeat("a", 5) + "one\n" +
			strings.Repeat("b", 300) + "two\n" +
			strings.Repeat("c", 270) + "tre\n",
		stops: []navStop{
			{line: 1, start: 5, end: 8},
			{line: 2, start: 300, end: 303},
			{line: 3, start: 270, end: 273},
		},
	}
}

// Same-file n applies the minimal horizontal reveal: the far match at
// cell 300 moves the offset just enough to paint its start at the
// right edge, the next match already inside the window moves nothing,
// and wrapping back to the near match scrolls left to its column.
func TestSameFileNavTriggersHorizontalReveal(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{hrevealFile()})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)
	widenFrame(t, m, key)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	textW := flatTextW(t, m, 3)
	listW := m.listWidth()

	// n to the cell-300 match: off = 300 + 1 − 40 = 261 — the match
	// start paints as the last text cell.
	m.Update(keyN)
	if got := m.vps[key].Off(); got != 261 {
		t.Fatalf("n to the far match: off = %d, want 261", got)
	}
	want := "2_ " + strings.Repeat("b", textW-1) + "t" + " "
	if row := dropCells(frameRow(t, m, 2), listW+1); row != want {
		t.Fatalf("far-match row = %q, want the match start at the right edge: %q", row, want)
	}

	// n to the cell-270 match: already painted at offset 261, so the
	// offset stays put — minimal reveal is no movement.
	m.Update(keyN)
	if got := m.vps[key].Off(); got != 261 {
		t.Fatalf("n to a visible match moved off to %d, want 261", got)
	}
	want = "3_ " + strings.Repeat("c", 9) + "tre" + strings.Repeat(" ", textW-12) + " "
	if row := dropCells(frameRow(t, m, 3), listW+1); row != want {
		t.Fatalf("visible-match row = %q, want %q", row, want)
	}

	// n wraps to the cell-5 match, hidden left: off = 5, the target
	// column itself.
	m.Update(keyN)
	if got := m.vps[key].Off(); got != 5 {
		t.Fatalf("n wrapping to the near match: off = %d, want 5", got)
	}
	want = "1_ " + "one" + strings.Repeat(" ", textW-3) + " "
	if row := dropCells(frameRow(t, m, 1), listW+1); row != want {
		t.Fatalf("near-match row = %q, want %q", row, want)
	}
}

// The startup reveal applies the horizontal rule too: toggling
// run-off-edge before the first load lands means the pending reveal
// commits against the flat layout — the match at cell 300 scrolls to
// the right edge without a single pan key.
func TestStartupRevealAppliesHorizontalReveal(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("b", 300) + "two\n",
		stops:   []navStop{{line: 1, start: 300, end: 303}},
	}})
	cmd := startBrowse(t, m, idx)
	// Resize so the flat text width is exactly 40 (gutter 3 + reserved
	// 1 + text 40 past the file list), then drop wrap before the load
	// completes: the startup reveal pends until the flat layout
	// installs.
	m.Update(tea.WindowSizeMsg{Width: frameWidthForG(m, 3, 41), Height: 24})
	m.Update(keyW)
	finishLoad(t, m, cmd)

	key := string(idx.Files[0].Path)
	textW := flatTextW(t, m, 3)
	if got := m.vps[key].Off(); got != 261 {
		t.Fatalf("startup reveal: off = %d, want 261 = 300 + 1 − 40", got)
	}
	want := "1_ " + strings.Repeat("b", textW-1) + "t" + " "
	if row := dropCells(frameRow(t, m, 1), m.listWidth()+1); row != want {
		t.Fatalf("startup row = %q, want the match start at the right edge: %q", row, want)
	}
}

// The file-change reset runs before the horizontal reveal: revisited
// with a saved offset of 50, the cell-10 target would be hidden left
// and reveal to 10 — because the reset zeroes the offset first, the
// target is already visible and the offset stays 0.
func TestFileChangeResetsOffsetBeforeHorizontalReveal(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: strings.Repeat("a", 10) + "one" + strings.Repeat("a", 200) + "\n",
			stops: []navStop{{line: 1, start: 10, end: 13}}},
		{name: "b.txt", content: strings.Repeat("b", 5) + "two\n",
			stops: []navStop{{line: 1, start: 5, end: 8}}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	keyA := string(idx.Files[0].Path)
	widenFrame(t, m, keyA)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	flatTextW(t, m, 3)

	vp := m.vps[keyA]
	vp.Pan(50, m.rows[keyA], contentRows24)
	m.vps[keyA] = vp
	if got := m.vps[keyA].Off(); got != 50 {
		t.Fatalf("a.txt off = %d, want the panned 50", got)
	}

	// n to b.txt: the reset zeroes the fresh viewport's offset, then
	// the reveal sees cell 5 painted at the left edge — off stays 0.
	m.Update(keyN)
	buf, err := filebuffer.Load(idx.Files[1].Path, idx.Files[1].Stops)
	if err != nil {
		t.Fatal(err)
	}
	injectLoad(t, m, fileLoadedMsg{path: idx.Files[1].Path, buf: buf})
	keyB := string(idx.Files[1].Path)
	if got := m.vps[keyB].Off(); got != 0 {
		t.Fatalf("b.txt off = %d, want the reset-then-visible 0", got)
	}

	// p back to a.txt: saved offset 50 is reset before the reveal, so
	// the cell-10 target is visible at offset 0 and nothing moves.
	m.Update(keyP)
	if got := m.vps[keyA].Off(); got != 0 {
		t.Fatalf("revisited a.txt off = %d, want 0 — reset precedes reveal", got)
	}
}

// A CJK match reveals with its whole first cluster painted: at cell
// 298 the two-cell 文 lands at the right edge — both cells, never a
// clipping blank.
func TestHorizontalRevealPaintsWideClusterAtRightEdge(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("x", 298) + "文\n",
		stops:   []navStop{{line: 1, start: 298, end: 301}},
	}})
	cmd := startBrowse(t, m, idx)
	m.Update(tea.WindowSizeMsg{Width: frameWidthForG(m, 3, 41), Height: 24})
	m.Update(keyW)
	finishLoad(t, m, cmd)

	key := string(idx.Files[0].Path)
	flatTextW(t, m, 3)
	if got := m.vps[key].Off(); got != 260 {
		t.Fatalf("off = %d, want 260 = 298 + 2 − 40", got)
	}
	want := "1_ " + strings.Repeat("x", 38) + "文" + " "
	if row := dropCells(frameRow(t, m, 1), m.listWidth()+1); row != want {
		t.Fatalf("row = %q, want both cells of 文 at the right edge: %q", row, want)
	}
}

// A match whose start cell is geometrically inside the window but
// clipped to a blank by the right edge counts as hidden: the reveal
// moves one column so the two-cell 文 paints whole.
func TestHorizontalRevealClippedBlankCountsHidden(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	// 文 spans cells 39–40 — its start is the last cell inside the
	// 40-wide window, but the cluster does not fit and renders blank.
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("x", 39) + "文" + strings.Repeat("x", 100) + "\n",
		stops:   []navStop{{line: 1, start: 39, end: 42}},
	}})
	cmd := startBrowse(t, m, idx)
	m.Update(tea.WindowSizeMsg{Width: frameWidthForG(m, 3, 41), Height: 24})
	m.Update(keyW)
	finishLoad(t, m, cmd)

	key := string(idx.Files[0].Path)
	flatTextW(t, m, 3)
	if got := m.vps[key].Off(); got != 1 {
		t.Fatalf("off = %d, want 1 = 39 + 2 − 40", got)
	}
	want := "1_ " + strings.Repeat("x", 38) + "文" + " "
	if row := dropCells(frameRow(t, m, 1), m.listWidth()+1); row != want {
		t.Fatalf("row = %q, want the fully painted 文 at the right edge: %q", row, want)
	}
}

// A target cluster wider than the whole text area takes the geometric
// fallback: the offset is its start column, the in-window cells render
// as blanks, and repeated navigation lands on the same offset — no
// panning loop.
func TestHorizontalRevealUnpaintableClusterFallback(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	// The tab expands cells 2–7 as one six-cell cluster — unpaintable
	// at text width 5. The line-2 stop gives n somewhere to go.
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: "ab\t\nq\n",
		stops: []navStop{
			{line: 1, start: 2, end: 3},
			{line: 2, start: 0, end: 1},
		},
	}})
	cmd := startBrowse(t, m, idx)
	// The dimensions are set directly and the model returned to
	// unsized rather than reported: the 9-column frame this fixture
	// needs is below the Issue #33 minimum, where a real
	// WindowSizeMsg raises the too-small gate and the key map
	// collapses to q/ctrl+c. The gate leaves the reveal arithmetic
	// itself live, so the fallback is exercised here on an unsized
	// model — the only model state in which a sub-minimum text area
	// can still exist.
	m.width, m.height, m.sized = frameWidthForG(m, 3, 6), 24, false
	m.Update(keyW)
	finishLoad(t, m, cmd)

	key := string(idx.Files[0].Path)
	if tw := m.textWidth(3); tw != 5 {
		t.Fatalf("flat text width = %d, want the fixture's 5", tw)
	}
	if got := m.vps[key].Off(); got != 2 {
		t.Fatalf("startup reveal off = %d, want the start column 2", got)
	}
	// The window over cells 2–6 is all clipping blanks: the tab's
	// in-window portion never paints. The hidden tab match counts as
	// entirely hidden right — the clipped blank is not a visible
	// cell — so the current line earns the reserved-column '*', and
	// the a–b cells hidden left show '_'.
	want := "1_ " + strings.Repeat(" ", 5) + "*"
	if row := dropCells(frameRow(t, m, 1), m.listWidth()+1); row != want {
		t.Fatalf("row = %q, want the tab window all blank: %q", row, want)
	}

	// n to line 2's cell-0 match reveals left to 0; n wraps back to
	// the tab — the same offset, no further movement.
	m.Update(keyN)
	if got := m.vps[key].Off(); got != 0 {
		t.Fatalf("n to the cell-0 match: off = %d, want 0", got)
	}
	m.Update(keyN)
	if got := m.vps[key].Off(); got != 2 {
		t.Fatalf("n wrapping back to the tab: off = %d, want 2", got)
	}
	m.Update(keyN)
	m.Update(keyN)
	if got := m.vps[key].Off(); got != 2 {
		t.Fatalf("repeated navigation moved off to %d, want the stable 2", got)
	}
}

// The horizontal reveal is inert in wrap mode: startup on a wrapped
// layout leaves the offset unset even with a far-off match.
func TestHorizontalRevealInertInWrapMode(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("b", 300) + "two\n",
		stops:   []navStop{{line: 1, start: 300, end: 303}},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)
	if vp, ok := m.vps[key]; ok && vp.Off() != 0 {
		t.Fatalf("wrap-mode startup reveal set off = %d, want none", vp.Off())
	}
}
