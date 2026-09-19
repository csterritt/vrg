package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

var (
	keyComma = tea.KeyPressMsg{Text: ",", Code: ','}
	keyDot   = tea.KeyPressMsg{Text: ".", Code: '.'}
	keyLt    = tea.KeyPressMsg{Text: "<", Code: '<'}
	keyGt    = tea.KeyPressMsg{Text: ">", Code: '>'}
	keyLBrk  = tea.KeyPressMsg{Text: "[", Code: '['}
	keyRBrk  = tea.KeyPressMsg{Text: "]", Code: ']'}
)

// flatTextW asserts the fixture's run-off-edge text width: the widened
// frame leaves exactly 40 text cells.
func flatTextW(t *testing.T, m *model, gutter int) int {
	t.Helper()
	if tw := m.textWidth(gutter); tw != 40 {
		t.Fatalf("run-off-edge text width = %d, want the fixture's 40", tw)
	}
	return 40
}

// widenFrame resizes the fixture's frame so path's flat text width is
// exactly 40 cells regardless of the temporary path's length, and
// delivers the resulting layout. Call it after the file's load — and
// before the w toggle into run-off-edge mode.
func widenFrame(t *testing.T, m *model, key string) {
	t.Helper()
	// 41 while wrapping (no reserved column) leaves 40 in
	// run-off-edge mode (one reserved indicator cell).
	_, lc := m.Update(tea.WindowSizeMsg{Width: frameWidthFor(m, key, 41), Height: 24})
	deliverLayout(t, m, lc)
}

// In run-off-edge mode the pan keys move the horizontal offset by
// their units — one column, ten columns, half the text width — and the
// rendered text shifts left by the same amount.
func TestPanKeysShiftText(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	var b strings.Builder
	for i := 0; i < 300; i++ {
		b.WriteByte(byte('a' + i%26))
	}
	line := b.String()
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: line + "\nsecond\n", line: 1, start: 0, end: 1},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)
	widenFrame(t, m, key)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)

	listW := m.listWidth()
	textW := flatTextW(t, m, 3)
	// Issue #20's gutter mark rides the first trailing space: '*' once
	// the cell-0 match is entirely hidden left, blank at the edge.
	want := func(off int) string {
		mark := "*"
		if off == 0 {
			mark = " "
		}
		return "1" + mark + " " + line[off:off+textW] + " "
	}
	half := viewport.HalfText(textW)
	for _, step := range []struct {
		name string
		key  tea.KeyPressMsg
		off  int
	}{
		{"dot pans one column", keyDot, 1},
		{"gt pans ten columns", keyGt, 11},
		{"rbrk pans half the text width", keyRBrk, 11 + half},
		{"lt pans ten back", keyLt, 1 + half},
		{"comma pans one back", keyComma, half},
		{"lbrk pans half back to the edge", keyLBrk, 0},
	} {
		m.Update(step.key)
		if got := m.vps[key].Off(); got != step.off {
			t.Fatalf("%s: off = %d, want %d", step.name, got, step.off)
		}
		if row := frameRow(t, m, 1)[listW:]; row != want(step.off) {
			t.Fatalf("%s: row = %q, want %q", step.name, row, want(step.off))
		}
	}
}

// The pan keys are a strict no-op in wrap mode: no offset state is
// created and the frame is untouched.
func TestPanKeysNoOpInWrapMode(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: strings.Repeat("x", 300) + "\nsecond\n", line: 1, start: 0, end: 1},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)

	before := viewText(m)
	for _, k := range []tea.KeyPressMsg{keyDot, keyComma, keyGt, keyLt, keyRBrk, keyLBrk} {
		m.Update(k)
	}
	if got := m.vps[key].Off(); got != 0 {
		t.Fatalf("panning in wrap mode set off = %d, want 0", got)
	}
	if v := viewText(m); v != before {
		t.Fatalf("the frame changed under pan keys in wrap mode: %q", v)
	}
}

// At the left edge the leftward keys do nothing — the offset clamps
// at zero.
func TestPanKeysClampAtLeftEdge(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: strings.Repeat("x", 300) + "\nsecond\n", line: 1, start: 0, end: 1},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)

	before := viewText(m)
	for _, k := range []tea.KeyPressMsg{keyComma, keyLt, keyLBrk} {
		m.Update(k)
		if got := m.vps[key].Off(); got != 0 {
			t.Fatalf("leftward pan at the edge gave off = %d, want 0", got)
		}
	}
	if v := viewText(m); v != before {
		t.Fatalf("the frame changed under leftward pans at the edge: %q", v)
	}
}

// The offset is retained through a wrap toggle: w w returns to
// run-off-edge with the same offset when the visible set's extent
// still admits it.
func TestPanOffsetSurvivesWrapToggle(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	line := strings.Repeat("x", 300)
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: line + "\nsecond\n", line: 1, start: 0, end: 1},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)
	widenFrame(t, m, key)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)

	m.Update(keyGt)
	m.Update(keyGt)
	if got := m.vps[key].Off(); got != 20 {
		t.Fatalf("off = %d, want 20", got)
	}
	_, wc = m.Update(keyW)
	deliverLayout(t, m, wc)
	_, wc = m.Update(keyW)
	deliverLayout(t, m, wc)
	if got := m.vps[key].Off(); got != 20 {
		t.Fatalf("off after w w = %d, want the retained 20", got)
	}
	listW := m.listWidth()
	textW := flatTextW(t, m, 3)
	if row := frameRow(t, m, 1)[listW:]; row != "1* "+line[20:20+textW]+" " {
		t.Fatalf("row after w w = %q, want the text from cell 20", row)
	}
}

// Navigation to another file resets the horizontal offset to zero
// before the reveal — and a revisited file starts from its left edge
// again rather than its saved offset.
func TestPanResetsOnFileChange(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	m.theme = theme.Plain()
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: "A-head " + strings.Repeat("x", 300) + "\n", line: 1, start: 0, end: 6},
		{name: "b.txt", content: "B-head " + strings.Repeat("y", 300) + "\n", line: 1, start: 0, end: 6},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	keyA := string(idx.Files[0].Path)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	m.Update(keyGt)
	if got := m.vps[keyA].Off(); got != 10 {
		t.Fatalf("file A off = %d, want 10", got)
	}

	// n crosses to B: B starts at its left edge.
	m.Update(keyN)
	buf, err := filebuffer.Load(idx.Files[1].Path, idx.Files[1].Stops)
	if err != nil {
		t.Fatal(err)
	}
	injectLoad(t, m, fileLoadedMsg{path: idx.Files[1].Path, buf: buf})
	keyB := string(idx.Files[1].Path)
	if got := m.vps[keyB].Off(); got != 0 {
		t.Fatalf("file B off = %d on arrival, want 0", got)
	}
	if row := frameRow(t, m, 1)[m.listWidth():]; !strings.HasPrefix(row, "1  B-head ") {
		t.Fatalf("file B row = %q, want it from the left edge", row)
	}

	// p returns to A: the file change reset A's offset too — not the
	// 20 it had when left.
	m.Update(keyP)
	if got := m.vps[keyA].Off(); got != 0 {
		t.Fatalf("revisited file A off = %d, want the reset 0", got)
	}
	if row := frameRow(t, m, 1)[m.listWidth():]; !strings.HasPrefix(row, "1  A-head ") {
		t.Fatalf("file A row = %q, want it from the left edge", row)
	}
}

// A grapheme cluster split by the left clip edge renders blank cells
// for the clipped portion — never half a glyph.
func TestPanClippedClusterRendersBlank(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	// 文 occupies cells 2–3; at offset 3 the window starts inside the
	// cluster, so its one visible cell renders blank.
	line := "ab文" + strings.Repeat("c", 200)
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: line + "\nsecond\n", line: 1, start: 0, end: 1},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)
	widenFrame(t, m, key)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	textW := flatTextW(t, m, 3)

	for i := 0; i < 3; i++ {
		m.Update(keyDot)
	}
	if got := m.vps[key].Off(); got != 3 {
		t.Fatalf("off = %d, want 3", got)
	}
	want := "1*  " + strings.Repeat("c", textW-1) + " "
	if row := frameRow(t, m, 1)[m.listWidth():]; row != want {
		t.Fatalf("row = %q, want a blank cell where 文's second half was clipped, then c's: %q", row, want)
	}
	if row := frameRow(t, m, 1); strings.Contains(row, "文") {
		t.Fatalf("row = %q shows a clipped glyph — want blanks for the clipped portion", row)
	}
}

// At the maximum offset the widest line's final cluster is painted
// whole — the paintable boundary never lands inside a cluster — and
// further pans do nothing.
func TestPanMaximumPaintsFinalCluster(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	line := strings.Repeat("x", 40) + "文" // E = 42 cells; 文 spans [40,42)
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: line + "\nsecond\n", line: 1, start: 0, end: 1},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)
	widenFrame(t, m, key)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	textW := flatTextW(t, m, 3)

	for i := 0; i < 4; i++ {
		m.Update(keyGt)
	}
	if got := m.vps[key].Off(); got != 40 {
		t.Fatalf("off at the maximum = %d, want 40 — the final cluster's start", got)
	}
	// The final cluster paints both its cells: the row starts with the
	// whole 文, then blanks — no split glyph, no blank-only text area.
	want := "1* " + "文" + strings.Repeat(" ", textW-2) + " "
	if row := frameRow(t, m, 1)[m.listWidth():]; row != want {
		t.Fatalf("row at the maximum = %q, want the fully painted 文 then blanks: %q", row, want)
	}
	before := viewText(m)
	for _, k := range []tea.KeyPressMsg{keyDot, keyGt, keyRBrk} {
		m.Update(k)
		if got := m.vps[key].Off(); got != 40 {
			t.Fatalf("panning past the maximum gave off = %d, want 40", got)
		}
	}
	if v := viewText(m); v != before {
		t.Fatalf("the frame changed under pans at the maximum: %q", v)
	}
}

// Scrolling from a long line into a region of short lines re-clamps
// the stored offset leftwards; scrolling back does not restore it.
func TestScrollReclampsOffsetLeftwards(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	line := strings.Repeat("x", 300)
	content := line + "\n" + strings.Repeat("pad\n", 40)
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: content, line: 1, start: 0, end: 1},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)
	widenFrame(t, m, key)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	textW := flatTextW(t, m, 4)

	vp := m.vps[key]
	vp.Pan(200, m.rows[key], contentRows24)
	m.vps[key] = vp
	if got := m.vps[key].Off(); got != 200 {
		t.Fatalf("off = %d, want 200", got)
	}

	m.Update(keyDown) // the 300-cell line scrolls out: max is the pads' 2
	if got := m.vps[key].Off(); got != 2 {
		t.Fatalf("off after scrolling into short lines = %d, want re-clamped 2", got)
	}
	m.Update(keyUp) // the long line returns — the offset does not
	if got := m.vps[key].Off(); got != 2 {
		t.Fatalf("off after scrolling back = %d, want the clamped 2", got)
	}
	if row := frameRow(t, m, 1)[m.listWidth():]; row != " 1* "+line[2:2+textW]+" " {
		t.Fatalf("row after returning = %q, want the long line from cell 2", row)
	}
}

// A navigation reveal that moves the viewport re-clamps the stored
// offset against the newly visible rows. The destination match starts
// at cell 2 — painted at the clamped offset — so Issue #19's
// horizontal reveal leaves the re-clamped value in place.
func TestRevealReclampsOffset(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	content := strings.Repeat("x", 300) + "\n" + strings.Repeat("pad\n", 40)
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: content,
		stops:   []navStop{{line: 1, start: 0, end: 1}, {line: 35, start: 2, end: 3}},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)

	vp := m.vps[key]
	vp.Pan(200, m.rows[key], contentRows24)
	m.vps[key] = vp

	m.Update(keyN) // same file: the reveal scrolls to the line-35 stop
	if got := m.vps[key].Off(); got != 2 {
		t.Fatalf("off after the reveal = %d, want the pad lines' 2", got)
	}
}

// Re-entry into run-off-edge mode re-clamps against the rows visible
// now: when the visible set changed while in wrap mode, the retained
// offset is not restored.
func TestWrapReentryReclampsOffset(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	content := strings.Repeat("x", 300) + "\n" + strings.Repeat("pad\n", 40)
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: content, line: 1, start: 0, end: 1},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)

	vp := m.vps[key]
	vp.Pan(100, m.rows[key], contentRows24)
	m.vps[key] = vp

	_, wc = m.Update(keyW) // wrap on: the offset is retained meanwhile
	deliverLayout(t, m, wc)
	m.Update(keyPgDn)
	m.Update(keyPgDn) // deep into the pad lines while wrapped

	_, wc = m.Update(keyW) // back to run-off-edge over short lines
	deliverLayout(t, m, wc)
	if got := m.vps[key].Off(); got != 2 {
		t.Fatalf("off after re-entry = %d, want re-clamped 2", got)
	}
}

// A resize shrinking the visible window re-clamps the stored offset
// against the rows still visible.
func TestResizeReclampsOffset(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	var b strings.Builder
	b.WriteString(strings.Repeat("pad\n", 10))
	b.WriteString(strings.Repeat("x", 300) + "\n")
	b.WriteString(strings.Repeat("pad\n", 20))
	idx := browseIndex(t, []fixtureFile{
		{name: "a.txt", content: b.String(), line: 1, start: 0, end: 3},
	})
	cmd := startBrowse(t, m, idx)
	finishLoad(t, m, cmd)
	key := string(idx.Files[0].Path)
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)

	// Height 24 shows rows 0–22, including the long line at row 10.
	vp := m.vps[key]
	vp.Pan(200, m.rows[key], contentRows24)
	m.vps[key] = vp

	// Shrinking to 10 rows leaves only "pad" lines in rows 0–8.
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	if got := m.vps[key].Off(); got != 2 {
		t.Fatalf("off after the resize = %d, want re-clamped 2", got)
	}
}

// The pan keys are no-ops on the Loading… placeholder: no offset state
// is created and the frame does not change.
func TestPanKeysNoOpOnPlaceholder(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := browseIndex(t, []fixtureFile{numberedFile("a.txt", 60)})
	startBrowse(t, m, idx) // the load stays in flight
	m.Update(keyW)         // run-off-edge while the placeholder is up

	loading := viewText(m)
	for _, k := range []tea.KeyPressMsg{keyDot, keyComma, keyGt, keyRBrk} {
		m.Update(k)
	}
	if len(m.vps) != 0 {
		t.Fatalf("panning Loading… created viewport state: %v", m.vps)
	}
	if v := viewText(m); v != loading || !strings.Contains(v, "Loading…") {
		t.Fatalf("view changed under pan keys on Loading…: %q", v)
	}
}
