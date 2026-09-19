package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// deliverLayout runs a returned layout-preparation command and feeds
// its message back through Update; a nil command — no request issued —
// is a no-op.
func deliverLayout(t *testing.T, m *model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	m.Update(cmd())
}

// A resize preserves the cursor selection and the logical reading
// position: the anchor survives a narrower and a wider frame, landing
// the effective top on the row containing the anchor's text location —
// not the row with the same former ordinal.
func TestResizePreservesCursorAndAnchor(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("x", 500) + "\n" + numberedContent("pad", 20),
		stops:   []navStop{{line: 1, start: 0, end: 1}, {line: 15, start: 0, end: 3}},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)
	m.Update(keyN) // select the second stop
	cur0, _ := m.idx.Cursor()

	// Scroll partway back into the wrapped 500-cell line: the anchor
	// lands mid-line at the top row's first cell.
	for i := 0; i < 3; i++ {
		m.Update(keyUp)
	}
	top0 := m.vps[key].Top()
	if top0 == 0 {
		t.Fatal("scrolling up from the revealed position did not move the top")
	}
	anchor0 := m.vps[key].Anchor()
	if anchor0.Line != 1 || anchor0.Cell == 0 {
		t.Fatalf("anchor = %+v, want a mid-line anchor on line 1", anchor0)
	}

	// Narrow the frame: the same text location stays at the top even
	// though the row ordinals all shifted.
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	deliverLayout(t, m, cmd)
	if got := m.vps[key].Anchor(); got != anchor0 {
		t.Fatalf("anchor after narrowing = %+v, want %+v", got, anchor0)
	}
	rows, ok := m.rows[key].(*viewport.Rows)
	if !ok {
		t.Fatalf("installed layout is %T, want *viewport.Rows", m.rows[key])
	}
	if got, want := m.vps[key].Top(), rows.RowOf(anchor0); got != want {
		t.Fatalf("top after narrowing = %d, want %d — the row containing the anchor", got, want)
	}
	if cur, _ := m.idx.Cursor(); cur != cur0 {
		t.Fatalf("cursor moved on resize: %v, want %v", cur, cur0)
	}

	// Widen back: the anchor still holds, so the original top returns.
	_, cmd = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	deliverLayout(t, m, cmd)
	if got := m.vps[key].Anchor(); got != anchor0 {
		t.Fatalf("anchor after widening = %+v, want %+v", got, anchor0)
	}
	if got := m.vps[key].Top(); got != top0 {
		t.Fatalf("top after widening back = %d, want the original %d", got, top0)
	}
	if cur, _ := m.idx.Cursor(); cur != cur0 {
		t.Fatalf("cursor moved on resize: %v, want %v", cur, cur0)
	}
}
