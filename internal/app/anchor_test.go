package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Any resize preserves the cursor selection: the matched-line cursor
// stays on its stop while the viewport rewraps around the logical
// anchor. Checked across both directions of a width round trip.
func TestResizePreservesCursorSelection(t *testing.T) {
	m := twoFileModel(t, t.TempDir(), 80, 24, 60, 30)
	m, _ = update(t, m, keyPress("n")) // a.txt:1 → a.txt:3
	before, ok := m.currentStop()
	if !ok {
		t.Fatal("no current stop before resize")
	}
	for _, w := range []int{120, 40, 80} {
		var cmd tea.Cmd
		m, cmd = update(t, m, tea.WindowSizeMsg{Width: w, Height: 24})
		m = settle(t, m, cmd)
		after, ok := m.currentStop()
		if !ok || after.Line != before.Line || string(after.Path) != string(before.Path) {
			t.Fatalf("resize to %d moved the cursor to %+v, want %+v", w, after, before)
		}
	}
}

// A resize keeps the same text at the top of the panel: the logical
// anchor is width-independent, so after the rewrap lands the first
// content row is the row containing the anchor's text location, not
// the row with the same former ordinal. Scrolled two rows into the
// 300-cell first line at 80 columns — text width 69, so the anchor is
// cell 138 — narrowing to 50 columns (text width 39) lands the top on
// rendered row 3, cells 117..155, which shows the NEEDLE marker.
func TestResizeKeepsAnchorTextAtTop(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 120) + "NEEDLE0123" + strings.Repeat("x", 170)
	var sb strings.Builder
	sb.WriteString(long + "\n")
	for i := 2; i <= 41; i++ {
		fmt.Fprintf(&sb, "x%05d\n", i)
	}
	writeWorkFile(t, dir, "a.txt", sb.String())
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"x00002\n"},"line_number":2,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m = settle(t, m, cmd)

	m, _ = update(t, m, codePress(tea.KeyDown))
	m, _ = update(t, m, codePress(tea.KeyDown))
	if m.vp.Top() != 2 {
		t.Fatalf("top after two downs = %d, want 2", m.vp.Top())
	}

	m, cmd = update(t, m, tea.WindowSizeMsg{Width: 50, Height: 24})
	m = settle(t, m, cmd)
	if m.vp.Top() != 3 {
		t.Fatalf("top after narrowing = %d, want 3 — the row containing cell 138", m.vp.Top())
	}
	row := strings.Split(ansi.Strip(m.View().Content), "\n")[1]
	if !strings.Contains(row, "NEEDLE") {
		t.Fatalf("top row after rewrap lacks the anchor's text: %q", row)
	}
}
