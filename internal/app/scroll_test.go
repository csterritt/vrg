package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/filebuffer"
	"vrg/internal/viewport"
)

// codePress is the message a non-text key delivers: arrows and page
// keys report their special code rather than printable text.
func codePress(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

// loadedModel returns a browse model whose a.txt load has completed:
// lines a001..aNNN with a match stop on line 1.
func loadedModel(t *testing.T, w, h, lines int) Model {
	t.Helper()
	dir := t.TempDir()
	var sb strings.Builder
	for i := 1; i <= lines; i++ {
		fmt.Fprintf(&sb, "a%03d\n", i)
	}
	writeWorkFile(t, dir, "a.txt", sb.String())
	m, cmd := browseModel(t, dir, w, h,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"a001\n"},"line_number":1,"submatches":[{"match":{"text":"a001"},"start":0,"end":4}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m, _ = update(t, m, cmd())
	return m
}

// countingRows is the prepared-rows fake: every Row query is recorded
// so a frame render can be proven to touch only the visible range.
type countingRows struct {
	n     int
	calls []int
}

func (r *countingRows) Len() int { return r.n }

func (r *countingRows) Row(i int) viewport.Row {
	r.calls = append(r.calls, i)
	return viewport.Row{Line: i}
}

// down and up move the viewport's top row by one rendered row; the
// rendered frame tracks it and the matched-line cursor does not move.
func TestDownUpMoveOneRenderedRow(t *testing.T) {
	m := loadedModel(t, 80, 24, 60)
	m, _ = update(t, m, codePress(tea.KeyDown))
	if m.vp.Top() != 1 {
		t.Fatalf("top after down = %d, want 1", m.vp.Top())
	}
	v := m.View().Content
	if !strings.Contains(v, "a024") || strings.Contains(v, "a025") || strings.Contains(v, "a001") {
		t.Fatalf("view after down does not show rows 2..24: %q", v)
	}
	m, _ = update(t, m, codePress(tea.KeyDown))
	if m.vp.Top() != 2 {
		t.Fatalf("top after second down = %d, want 2", m.vp.Top())
	}
	m, _ = update(t, m, codePress(tea.KeyUp))
	if m.vp.Top() != 1 {
		t.Fatalf("top after up = %d, want 1", m.vp.Top())
	}
	if s, ok := m.currentStop(); !ok || s.Line != 1 {
		t.Fatalf("manual scrolling moved the cursor to %+v", s)
	}
}

// d/u move by max(1, floor(content height/2)) where the content height
// is the panel height minus the filename row.
func TestHalfPageScrollUsesContentHeight(t *testing.T) {
	for _, tc := range []struct {
		height int
		want   int
	}{
		{24, 11}, // content 23 → half 11
		{22, 10}, // content 21 → half 10
		{2, 1},   // content 1 → max(1, 0)
	} {
		t.Run(fmt.Sprintf("h%d", tc.height), func(t *testing.T) {
			m := loadedModel(t, 80, tc.height, 60)
			m, _ = update(t, m, keyPress("d"))
			if m.vp.Top() != tc.want {
				t.Fatalf("top after d = %d, want %d", m.vp.Top(), tc.want)
			}
			m, _ = update(t, m, keyPress("u"))
			if m.vp.Top() != 0 {
				t.Fatalf("top after u = %d, want 0", m.vp.Top())
			}
		})
	}
}

// page down/up move by the full content height — panel height minus
// the filename row.
func TestPageScrollUsesContentHeight(t *testing.T) {
	m := loadedModel(t, 80, 24, 120)
	m, _ = update(t, m, codePress(tea.KeyPgDown))
	if m.vp.Top() != 23 {
		t.Fatalf("top after pgdown = %d, want 23", m.vp.Top())
	}
	m, _ = update(t, m, codePress(tea.KeyPgDown))
	if m.vp.Top() != 46 {
		t.Fatalf("top after second pgdown = %d, want 46", m.vp.Top())
	}
	m, _ = update(t, m, codePress(tea.KeyPgUp))
	if m.vp.Top() != 23 {
		t.Fatalf("top after pgup = %d, want 23", m.vp.Top())
	}
}

// Scrolling past EOF stops with the file's last row on the bottom row;
// scrolling up at BOF does nothing.
func TestScrollClampsAtEOFAndBOF(t *testing.T) {
	m := loadedModel(t, 80, 24, 30) // max top = 30-23 = 7
	before := m.View().Content
	m, _ = update(t, m, codePress(tea.KeyUp))
	if m.vp.Top() != 0 || m.View().Content != before {
		t.Fatalf("up at BOF changed the view: top=%d", m.vp.Top())
	}
	m, _ = update(t, m, codePress(tea.KeyPgDown))
	if m.vp.Top() != 7 {
		t.Fatalf("pgdown clamped top = %d, want 7", m.vp.Top())
	}
	m, _ = update(t, m, codePress(tea.KeyDown))
	if m.vp.Top() != 7 {
		t.Fatalf("down past EOF moved top to %d, want 7", m.vp.Top())
	}
	if v := m.View().Content; !strings.Contains(v, "a030") {
		t.Fatalf("EOF view lacks the last row at the bottom: %q", v)
	}
}

// A file shorter than the viewport leaves its unused rows blank and
// ignores every scroll key.
func TestScrollOnShortFileIsNoOp(t *testing.T) {
	m := loadedModel(t, 80, 24, 5)
	before := m.View().Content
	for _, msg := range []tea.KeyPressMsg{
		codePress(tea.KeyDown), keyPress("d"), codePress(tea.KeyPgDown),
	} {
		var cmd tea.Cmd
		m, cmd = update(t, m, msg)
		if cmd != nil {
			t.Fatalf("scroll key produced a command %T", cmd)
		}
	}
	if m.vp.Top() != 0 || m.View().Content != before {
		t.Fatalf("short file scrolled: top=%d", m.vp.Top())
	}
}

// Scroll keys on a "Loading…" placeholder are no-ops: no top movement,
// no saved state, no command.
func TestScrollOnPlaceholderIsNoOp(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "x\n")
	m, _ := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	for _, msg := range []tea.KeyPressMsg{
		codePress(tea.KeyUp), codePress(tea.KeyDown), keyPress("u"), keyPress("d"),
		codePress(tea.KeyPgUp), codePress(tea.KeyPgDown),
	} {
		var cmd tea.Cmd
		m, cmd = update(t, m, msg)
		if cmd != nil {
			t.Fatalf("scroll on placeholder produced a command %T", cmd)
		}
	}
	if m.vp.Top() != 0 {
		t.Fatalf("placeholder scrolled: top=%d", m.vp.Top())
	}
	if len(m.saved) != 0 {
		t.Fatalf("placeholder scroll wrote saved state: %v", m.saved)
	}
	if v := m.View().Content; !strings.Contains(v, "Loading…") {
		t.Fatalf("placeholder lost: %q", v)
	}
}

// The current file's top row is saved per path when scrolling; a file
// whose load completes starts from its saved vertical state — the
// revisit seam Issue #13 will drive.
func TestPerFileSavedViewportState(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&sb, "a%03d\n", i)
	}
	writeWorkFile(t, dir, "a.txt", sb.String())
	sb.Reset()
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&sb, "b%03d\n", i)
	}
	writeWorkFile(t, dir, "b.txt", sb.String())
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"a001\n"},"line_number":1,"submatches":[{"match":{"text":"a001"},"start":0,"end":4}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"begin","data":{"path":{"text":"b.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"b.txt"},"lines":{"text":"b001\n"},"line_number":1,"submatches":[{"match":{"text":"b001"},"start":0,"end":4}]}}`,
		`{"type":"end","data":{"path":{"text":"b.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m, _ = update(t, m, cmd()) // a.txt loaded at top
	m, _ = update(t, m, codePress(tea.KeyDown))
	m, _ = update(t, m, codePress(tea.KeyDown))
	m, _ = update(t, m, codePress(tea.KeyDown))
	if m.saved["a.txt"] != 3 {
		t.Fatalf("saved a.txt top = %d, want 3", m.saved["a.txt"])
	}

	// n moves the cursor to b.txt's stop — Issue #13's real
	// mechanism — and returns its load command; seed b.txt's saved
	// state before the load completes: the panel starts at the saved
	// top while a.txt's state is untouched.
	m, load := update(t, m, keyPress("n"))
	if load == nil {
		t.Fatal("no load command for b.txt")
	}
	m.saved["b.txt"] = 7
	m, _ = update(t, m, load())
	if m.vp.Top() != 7 {
		t.Fatalf("b.txt top after load = %d, want 7", m.vp.Top())
	}
	v := m.View().Content
	if !strings.Contains(v, "b008") || strings.Contains(v, "b007") {
		t.Fatalf("b.txt view does not start at the saved row: %q", v)
	}
	if m.saved["a.txt"] != 3 {
		t.Fatalf("a.txt saved state changed to %d", m.saved["a.txt"])
	}
}

// A frame render queries the prepared row provider only for the
// visible range — not O(N) over the buffer.
func TestRenderQueriesOnlyVisibleRows(t *testing.T) {
	m := loadedModel(t, 80, 24, 60)
	fake := &countingRows{n: 60}
	m.rows["a.txt"] = fake
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	fake.calls = nil
	_ = m.View()

	want := make([]int, 23)
	for i := range want {
		want[i] = i
	}
	if len(fake.calls) != len(want) {
		t.Fatalf("provider calls = %v, want %d calls for the visible rows", fake.calls, len(want))
	}
	for i, c := range fake.calls {
		if c != want[i] {
			t.Fatalf("provider calls = %v, want rows %v", fake.calls, want)
		}
	}

	m, _ = update(t, m, codePress(tea.KeyDown))
	fake.calls = nil
	_ = m.View()
	for i, c := range fake.calls {
		if c != i+1 {
			t.Fatalf("after down provider calls = %v, want rows 1..23", fake.calls)
		}
	}
	if len(fake.calls) != 23 {
		t.Fatalf("after down provider calls = %v, want 23", fake.calls)
	}
}

// The prepared-row adapter maps rendered row i to source line i in
// unwrapped mode, carrying the buffer's cells and spans.
func TestBufferRowsAdaptsBuffer(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var rows viewport.Rows = bufferRows{buf: buf}
	if rows.Len() != 2 {
		t.Fatalf("Len = %d, want 2", rows.Len())
	}
	if r := rows.Row(1); r.Line != 1 || len(r.Cells) != 3 {
		t.Fatalf("Row(1) = %+v, want line 1 with 3 cells", r)
	}
}
