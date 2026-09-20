package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// panLine is the 210-cell fixture line: a digit prefix then repeated
// alphabet blocks, so a shifted window shows a distinct text run.
func panLine() string {
	return "0123456789" + strings.Repeat("abcdefghij", 20)
}

// runOffEdge loads one file and toggles into run-off-edge mode.
func runOffEdge(t *testing.T, name, content string, w, h int) Model {
	t.Helper()
	m := browseLoaded(t, name, content, w, h)
	m, cmd := update(t, m, keyMsg("w"))
	return deliverCmd(t, m, cmd)
}

// The pan keys shift the run-off-edge text area by their units: one
// column for ./,, ten columns for >/<, and half the text width for
// ]/[ — clamped at the paintable-boundary maximum of the visible rows.
func TestPanKeysShiftContent(t *testing.T) {
	line := panLine()
	// 80-column terminal: list 7, panel 73, gutter 3, reserved 1 →
	// text width 69, so the half-screen unit is 34.
	m := runOffEdge(t, "a.txt", line+"\n", 80, 24)

	want := func(off int) string {
		return "       " + "1  " + line[off:off+69] + " "
	}
	if got := contentRow(t, m); got != want(0) {
		t.Fatalf("initial row = %q, want %q", got, want(0))
	}
	m, _ = update(t, m, keyMsg("."))
	if got := m.vps["a.txt"].Offset(); got != 1 {
		t.Fatalf("after .: offset = %d, want 1", got)
	}
	if got := contentRow(t, m); got != want(1) {
		t.Fatalf("after .: row = %q, want %q", got, want(1))
	}
	m, _ = update(t, m, keyMsg(","))
	if got := m.vps["a.txt"].Offset(); got != 0 {
		t.Fatalf("after ,: offset = %d, want 0", got)
	}
	m, _ = update(t, m, keyMsg(">"))
	if got := m.vps["a.txt"].Offset(); got != 10 {
		t.Fatalf("after >: offset = %d, want 10", got)
	}
	if got := contentRow(t, m); got != want(10) {
		t.Fatalf("after >: row = %q, want %q", got, want(10))
	}
	m, _ = update(t, m, keyMsg("]"))
	if got := m.vps["a.txt"].Offset(); got != 44 {
		t.Fatalf("after ]: offset = %d, want 44 (10 + half width 34)", got)
	}
	m, _ = update(t, m, keyMsg("["))
	if got := m.vps["a.txt"].Offset(); got != 10 {
		t.Fatalf("after [: offset = %d, want 10", got)
	}
	// Panning past the paintable boundary clamps: the 210-cell line's
	// last cluster starts at 209.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, keyMsg("]"))
	}
	if got := m.vps["a.txt"].Offset(); got != 209 {
		t.Fatalf("panning past the end: offset = %d, want the maximum 209", got)
	}
}

// In wrap mode the pan keys are strict no-ops: the offset stays zero
// and the rendered frame does not change.
func TestPanKeysNoopInWrapMode(t *testing.T) {
	m := browseLoaded(t, "a.txt", panLine()+"\n", 80, 24)
	before := m.View().Content
	for _, k := range []string{".", ",", ">", "<", "]", "["} {
		m, _ = update(t, m, keyMsg(k))
	}
	if got := m.vps["a.txt"].Offset(); got != 0 {
		t.Fatalf("pan keys in wrap mode set offset %d, want 0", got)
	}
	if got := m.View().Content; got != before {
		t.Fatalf("pan keys in wrap mode changed the view")
	}
}

// A w w round trip keeps the offset: it is separate state retained
// through wrap toggles.
func TestPanOffsetSurvivesWrapToggle(t *testing.T) {
	m := runOffEdge(t, "a.txt", panLine()+"\n", 80, 24)
	m, _ = update(t, m, keyMsg(">"))
	if got := m.vps["a.txt"].Offset(); got != 10 {
		t.Fatalf("setup pan: offset = %d, want 10", got)
	}
	m, cmd := update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	if got := m.vps["a.txt"].Offset(); got != 10 {
		t.Fatalf("offset while wrapped = %d, want the retained 10", got)
	}
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	if got := m.vps["a.txt"].Offset(); got != 10 {
		t.Fatalf("offset after the wrap round trip = %d, want 10", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "bcdefghij") {
		t.Fatalf("row after the round trip = %q, want the panned window", row)
	}
}

// Navigation to another file resets the offset to zero, and revisiting
// the panned file resets it again on entry — stored per-file pan does
// not carry across a file change.
func TestPanOffsetResetsOnFileChange(t *testing.T) {
	dir := t.TempDir()
	line := panLine()
	writeMatchFile(t, dir, "a.txt", line+"\n")
	writeMatchFile(t, dir, "b.txt", line+"\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", line+"\n", 1, 0, 4, "0123"))
	addRec(t, idx, matchRec("b.txt", line+"\n", 1, 0, 4, "0123"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()
	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)

	m, _ = update(t, m, keyMsg(">"))
	if got := m.vps["a.txt"].Offset(); got != 10 {
		t.Fatalf("setup pan: offset = %d, want 10", got)
	}

	m, cmd = update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, cmd))
	if got := m.vps["b.txt"].Offset(); got != 0 {
		t.Fatalf("b.txt offset = %d, want the file-change reset 0", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "0123456789") {
		t.Fatalf("b.txt shows %q, want its left edge", row)
	}

	m, _ = update(t, m, keyMsg("p"))
	if got := m.vps["a.txt"].Offset(); got != 0 {
		t.Fatalf("revisited a.txt offset = %d, want the reset 0", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "0123456789") {
		t.Fatalf("revisited a.txt shows %q, want its left edge", row)
	}
}

// A vertical-shrink that drops the widest line out of the window
// re-clamps the stored offset to the remaining lines' maximum.
func TestPanReclampsWhenWidestLineLeaves(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 22; i++ {
		b.WriteString("0123456789\n")
	}
	b.WriteString(strings.Repeat("x", 300) + "\n")
	for i := 0; i < 10; i++ {
		b.WriteString("0123456789\n")
	}
	// Height 24 → 23 content rows: the 300-cell line 23 is visible.
	m := runOffEdge(t, "a.txt", b.String(), 80, 24)
	m, _ = update(t, m, keyMsg("]")) // half width 34
	if got := m.vps["a.txt"].Offset(); got != 34 {
		t.Fatalf("setup pan: offset = %d, want 34", got)
	}

	// Height 10 → 9 content rows: only the ten-cell lines show, so the
	// maximum falls to 9 and the stored offset follows it down.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
	if got := m.vps["a.txt"].Offset(); got != 9 {
		t.Fatalf("offset after the shrink = %d, want the re-clamped 9", got)
	}
}

// A grapheme cluster split by the left clip edge renders blank cells:
// the panned view shows a blank in place of the half-clipped wide
// glyph, never a broken one.
func TestPanSplitClusterRendersBlank(t *testing.T) {
	m := runOffEdge(t, "a.txt", "世"+strings.Repeat("z", 100)+"\n", 80, 24)
	m, _ = update(t, m, keyMsg("."))
	if got := m.vps["a.txt"].Offset(); got != 1 {
		t.Fatalf("setup pan: offset = %d, want 1", got)
	}
	want := "       " + "1  " + " " + strings.Repeat("z", 68) + " "
	if got := contentRow(t, m); got != want {
		t.Fatalf("row = %q, want %q — the split glyph's cell is blank", got, want)
	}
}

// Pan keys on a load placeholder are no-ops, like the scroll keys:
// nothing moves whether the mode is wrap or run-off-edge.
func TestPanKeysOnPlaceholderNoop(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", fmt.Sprintf("%s\n", panLine()))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", panLine()+"\n", 1, 0, 4, "0123"))
	idx.Finish()

	m, _ := startBrowse(t, dir, idx, 80, 24)
	// The load command is deliberately never delivered.
	before := m.View().Content
	for _, k := range []string{".", ",", ">", "<", "]", "["} {
		var cmd tea.Cmd
		m, cmd = update(t, m, keyMsg(k))
		if cmd != nil {
			t.Fatalf("pan key %q on the placeholder returned a command: %v", k, cmd)
		}
	}
	m, _ = update(t, m, keyMsg("w")) // run-off-edge mode, still loading
	for _, k := range []string{".", ">", "]"} {
		var cmd tea.Cmd
		m, cmd = update(t, m, keyMsg(k))
		if cmd != nil {
			t.Fatalf("pan key %q on the placeholder returned a command: %v", k, cmd)
		}
	}
	if got := m.View().Content; got != before {
		t.Fatalf("panning the placeholder changed the view:\nbefore:\n%s\nafter:\n%s", before, got)
	}
}
