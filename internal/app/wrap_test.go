package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

// viewRow returns row r of the composed view.
func viewRow(t *testing.T, m Model, r int) string {
	t.Helper()
	rows := strings.Split(m.View().Content, "\n")
	if r >= len(rows) {
		t.Fatalf("view has %d rows, want row %d:\n%s", len(rows), r, m.View().Content)
	}
	return rows[r]
}

// Wrapping is on initially: a line longer than the text area occupies
// continuation rows whose blank gutter keeps their text aligned under
// the first row's text.
func TestWrapOnInitiallyWithContinuationGutter(t *testing.T) {
	// 80-column terminal: file list 7, panel 73, gutter 3, wrap text
	// width 70 — so the 100-cell line paints 70 + 30.
	m := browseLoaded(t, "a.txt", strings.Repeat("x", 100)+"\n", 80, 24)

	first := viewRow(t, m, 1)
	if want := "       " + "1  " + strings.Repeat("x", 70); first != want {
		t.Fatalf("first content row = %q, want %q", first, want)
	}
	cont := viewRow(t, m, 2)
	if cont[7:10] != "   " {
		t.Fatalf("continuation row gutter = %q, want blank", cont[7:10])
	}
	if i, j := strings.Index(first, "x"), strings.Index(cont, "x"); i != j {
		t.Fatalf("continuation text starts at column %d, want alignment with first row at %d", j, i)
	}
	if got := cont[10:40]; got != strings.Repeat("x", 30) {
		t.Fatalf("continuation text = %q, want 30 x's", got)
	}
}

// Scrolling moves in rendered rows: stepping down into a wrapped file
// lands on the continuation row, not on the next source line.
func TestScrollStepsByRenderedRow(t *testing.T) {
	// 30 short lines after the wrapped one make the file taller than the
	// content area so scrolling is not clamped away.
	var b strings.Builder
	b.WriteString(strings.Repeat("x", 100) + "\nsecond\n")
	for i := 0; i < 30; i++ {
		b.WriteString("tail\n")
	}
	m := browseLoaded(t, "a.txt", b.String(), 80, 24)
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	row := viewRow(t, m, 1)
	if row[7:10] != "   " || !strings.Contains(row, "x") {
		t.Fatalf("after one down, first content row = %q, want the continuation row", row)
	}
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	row = viewRow(t, m, 1)
	if !strings.Contains(row, "2  second") {
		t.Fatalf("after two downs, first content row = %q, want source line 2", row)
	}
}

// w toggles between wrap and run-off-edge mode: the continuation rows
// collapse to one clipped row with a blank reserved indicator cell at
// the panel's right edge, and w again restores the wrap.
func TestWTogglesWrapMode(t *testing.T) {
	m := browseLoaded(t, "a.txt", strings.Repeat("x", 100)+"\n", 80, 24)
	if got := strings.Count(viewRow(t, m, 2), "x"); got != 30 {
		t.Fatalf("wrapped continuation row shows %d x's, want 30", got)
	}

	m, cmd := update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	row := viewRow(t, m, 1)
	// Run-off-edge text width = 73 - 3 - 1 reserved = 69 cells, then
	// the still-blank indicator column.
	if got := strings.Count(row, "x"); got != 69 {
		t.Fatalf("run-off-edge row shows %d x's, want 69", got)
	}
	if row != "       "+"1  "+strings.Repeat("x", 69)+" " {
		t.Fatalf("run-off-edge row = %q, want clipped text plus reserved cell", row)
	}
	if got := strings.Count(viewRow(t, m, 2), "x"); got != 0 {
		t.Fatalf("run-off-edge mode still shows continuation rows: %d x's", got)
	}

	m, cmd = update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, cmd)
	if got := strings.Count(viewRow(t, m, 2), "x"); got != 30 {
		t.Fatalf("second w did not restore wrapping: continuation shows %d x's, want 30", got)
	}
}

// The Issue 14 reveal finds the rendered row of a wrapped line holding
// the match's start cell: a match near the end of a long line lands
// one third down the content area.
func TestRevealFindsRowInsideWrappedLine(t *testing.T) {
	dir := t.TempDir()
	var content strings.Builder
	for i := 1; i <= 31; i++ {
		fmt.Fprintf(&content, "line-%02d\n", i)
	}
	content.WriteString(strings.Repeat("x", 500) + "\n")
	for i := 33; i <= 62; i++ {
		fmt.Fprintf(&content, "line-%02d\n", i)
	}
	writeMatchFile(t, dir, "a.txt", content.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-01\n", 1, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", strings.Repeat("x", 500)+"\n", 32, 480, 490, "x"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m, _ = update(t, m, keyMsg("n"))

	// Text width 70: the 500-cell line 32 covers rendered rows 31..38
	// and its byte-480 match sits in sub-row 6 — rendered row 37. The
	// reveal lands it at floor(23/3) = 7, so the top is row 30.
	if got := m.vps["a.txt"].Top(); got != 30 {
		t.Fatalf("top = %d, want 30", got)
	}
}
