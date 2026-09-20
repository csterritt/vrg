package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// stubSource is a viewport.Source fake: each line is a run of n
// one-cell "x" cells with a configurable gutter width — the minimum
// geometry the wrap, anchor, and list-width arithmetic needs without a
// real file behind it.
type stubSource struct {
	gutter int
	widths []int
}

func (s *stubSource) LineCount() int   { return len(s.widths) }
func (s *stubSource) GutterWidth() int { return s.gutter }
func (s *stubSource) Cells(i int) []safepresentation.Cell {
	cells := make([]safepresentation.Cell, s.widths[i])
	for j := range cells {
		cells[j] = safepresentation.Cell{Text: "x"}
	}
	return cells
}
func (s *stubSource) Clusters(i int) []safepresentation.Cluster {
	clusters := make([]safepresentation.Cluster, s.widths[i])
	for j := range clusters {
		clusters[j] = safepresentation.Cluster{Start: j, End: j + 1, Width: 1}
	}
	return clusters
}
func (s *stubSource) Highlights(i int) []filebuffer.Span { return nil }
func (s *stubSource) TargetCell(stop searchindex.Stop) (int, int) {
	line := int(stop.Line) - 1
	if last := len(s.widths) - 1; line > last {
		line = last
	}
	if line < 0 {
		line = 0
	}
	return line, 0
}

// The Issue 24 list width is the nonnegative minimum of the longest
// sanitized path plus two, floor(0.40 × terminal width), and terminal
// width minus (gutter + 10 + reserved indicator) — each term can win,
// the 40% cap floors, a hidden or empty list draws nothing, and
// pathological dimensions never go negative.
func TestListColumnWidthTerms(t *testing.T) {
	for _, tc := range []struct {
		name     string
		visible  bool
		widest   int // longest sanitized path + 2
		w        int
		gutter   int
		reserved int
		want     int
	}{
		{"longest-path term wins", true, 12, 80, 3, 0, 12},
		{"forty-percent cap wins", true, 60, 80, 3, 0, 32},
		{"forty-percent cap floors", true, 60, 81, 3, 0, 32},
		{"content minimum wins", true, 60, 30, 9, 1, 10},
		{"gutter growth narrows", true, 60, 30, 9, 0, 11},
		{"zero-width allocation", true, 12, 20, 9, 1, 0},
		{"hidden draws no cells", false, 12, 80, 3, 0, 0},
		{"never negative", true, 60, 10, 9, 1, 0},
		{"no entries no column", true, 0, 80, 3, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := listColumn(tc.visible, tc.widest, tc.w, tc.gutter, tc.reserved)
			if got != tc.want {
				t.Fatalf("listColumn(%v, %d, %d, %d, %d) = %d, want %d",
					tc.visible, tc.widest, tc.w, tc.gutter, tc.reserved, got, tc.want)
			}
		})
	}
}

// The list is shown at startup; tab and left hide it, shift+tab and
// right show it again. Each transition is a text-width change routed
// through the Issue 17 prepared-layout path, and a hidden list never
// queries the item provider.
func TestListHideShowToggles(t *testing.T) {
	m := browseLoaded(t, "a.txt", numberedContent(50), 80, 24)
	if !m.listVisible {
		t.Fatal("the file list is not shown at startup")
	}
	if got := viewRow(t, m, 0); !strings.HasPrefix(got, "a.txt") {
		t.Fatalf("startup view lacks the list column: %q", got)
	}

	queries := 0
	m.itemName = func(i int) string {
		queries++
		return "a.txt"
	}

	hide := []tea.KeyPressMsg{
		{Code: tea.KeyTab},
		{Code: tea.KeyLeft},
	}
	show := []tea.KeyPressMsg{
		{Code: tea.KeyTab, Mod: tea.ModShift},
		{Code: tea.KeyRight},
	}
	for i := range hide {
		m, cmd := update(t, m, hide[i])
		if cmd == nil {
			t.Fatalf("%s issued no prepared-layout request", hide[i].String())
		}
		m = deliverCmd(t, m, cmd)
		if m.listVisible {
			t.Fatalf("%s did not hide the list", hide[i].String())
		}
		if got := viewRow(t, m, 0); !strings.HasPrefix(got, "──") {
			t.Fatalf("hidden list still draws cells: %q", got)
		}
		queries = 0
		_ = m.View()
		if queries != 0 {
			t.Fatalf("hidden list queried the item provider %d times", queries)
		}

		m, cmd = update(t, m, show[i])
		if cmd == nil {
			t.Fatalf("%s issued no prepared-layout request", show[i].String())
		}
		m = deliverCmd(t, m, cmd)
		if !m.listVisible {
			t.Fatalf("%s did not show the list", show[i].String())
		}
		if got := viewRow(t, m, 0); !strings.HasPrefix(got, "a.txt") {
			t.Fatalf("shown list draws no cells: %q", got)
		}
	}

	// A redundant press is a no-op: no preference change, no layout
	// churn.
	m, cmd := update(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if cmd != nil {
		t.Fatalf("right while already visible returned a command: %v", cmd)
	}
	if !m.listVisible {
		t.Fatal("right while visible hid the list")
	}
}

// A computed zero-width list draws no cells but keeps the user's
// visibility preference — no automatic toggling — and a later resize
// restores the column. W=20, gutter=9, indicator=1 → 20 − (9+10+1) = 0.
func TestZeroWidthListRetainsPreference(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "x\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "x\n", 1, 0, 1, "x"))
	idx.Finish()

	m, _ := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, loadResult{
		path: []byte("a.txt"),
		req:  m.loading["a.txt"],
		src:  &stubSource{gutter: 9, widths: []int{40, 1, 1}},
	})
	m.theme = theme.Plain()

	// Run-off-edge mode reserves the rightmost indicator column.
	m, w := update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, w)

	m, resize := update(t, m, tea.WindowSizeMsg{Width: 20, Height: 24})
	m = deliverCmd(t, m, resize)

	if !m.listVisible {
		t.Fatal("zero-width allocation lost the visibility preference")
	}
	if got := m.listWidth(20); got != 0 {
		t.Fatalf("list width at W=20 = %d, want 0", got)
	}
	if got := viewRow(t, m, 0); !strings.HasPrefix(got, "──") {
		t.Fatalf("zero-width list still draws cells: %q", got)
	}

	// Growing the terminal restores the column without any toggle.
	m, resize = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = deliverCmd(t, m, resize)
	if got := viewRow(t, m, 0); !strings.HasPrefix(got, "a.txt") {
		t.Fatalf("resized view did not restore the list: %q", got)
	}
}

// Match navigation keeps the active entry inside the list's visible
// window: stepping twelve files down in a ten-row terminal scrolls the
// list so f12.txt is rendered and f00.txt has scrolled out.
func TestListScrollsToKeepActiveEntry(t *testing.T) {
	idx := searchindex.New("/w")
	for i := 0; i < 30; i++ {
		addRec(t, idx, matchRec(fmt.Sprintf("f%02d.txt", i), "hit\n", 1, 0, 3, "hit"))
	}
	idx.Finish()

	m, _ := startBrowse(t, "/w", idx, 80, 10)
	m.theme = theme.Plain()
	for i := 0; i < 12; i++ {
		m, _ = update(t, m, keyMsg("n"))
	}
	// Any key dismisses the file-change pop-up so the assertion reads
	// the unobscured list.
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})

	lw := m.listWidth(80)
	if lw <= 0 {
		t.Fatalf("list width = %d, want a visible column", lw)
	}
	found := false
	for _, r := range strings.Split(m.View().Content, "\n") {
		if len(r) < lw {
			continue
		}
		entry := r[:lw]
		if strings.Contains(entry, "f12.txt") {
			found = true
		}
		if strings.Contains(entry, "f00.txt") {
			t.Fatalf("scrolled-out entry f00.txt still visible: %q", entry)
		}
	}
	if !found {
		t.Fatalf("active entry f12.txt not in the visible window:\n%s", m.View().Content)
	}
}

// A top row mid-way through a wrapped line survives a hide/show round
// trip: each toggle is a rewrap through the Issue 17 prepared-layout
// path, and the logical anchor keeps the same text location at the top.
func TestAnchorSurvivesListHideShow(t *testing.T) {
	// The 100-cell first line wraps; one down lands the top on its
	// continuation row — a mid-line logical column anchor.
	m := browseLoaded(t, "a.txt", strings.Repeat("x", 100)+"\n"+numberedContent(50), 80, 24)
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	anchor := m.vps["a.txt"].Anchor()
	if anchor.Line != 0 || anchor.Col == 0 {
		t.Fatalf("anchor = %+v, want a mid-line column on line 0", anchor)
	}
	before := contentRow(t, m)

	m, cmd := update(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if cmd == nil {
		t.Fatal("tab issued no prepared-layout request")
	}
	msg := cmd()
	lr, ok := msg.(layoutResult)
	if !ok {
		t.Fatalf("tab's command produced %T, want a layoutResult", msg)
	}
	// Hidden list: panel 80, gutter 4, wrap mode → text width 76.
	if lr.key.TextWidth != 76 {
		t.Fatalf("hidden-list layout text width = %d, want 76", lr.key.TextWidth)
	}
	m, _ = update(t, m, lr)
	if got := m.vps["a.txt"].Anchor(); got != anchor {
		t.Fatalf("hide moved the anchor to %+v, want %+v", got, anchor)
	}
	if row := contentRow(t, m); !strings.Contains(row, "x") || strings.Contains(row, "line-") {
		t.Fatalf("hidden-list top row = %q, want line 0's text at the top", row)
	}

	m, cmd = update(t, m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if cmd == nil {
		t.Fatal("shift+tab issued no prepared-layout request")
	}
	m = deliverCmd(t, m, cmd)
	if !m.listVisible {
		t.Fatal("shift+tab did not show the list")
	}
	if got := m.vps["a.txt"].Anchor(); got != anchor {
		t.Fatalf("show moved the anchor to %+v, want %+v", got, anchor)
	}
	if got := contentRow(t, m); got != before {
		t.Fatalf("round trip restored top row %q, want %q", got, before)
	}
}

// A load that enlarges the gutter narrows the list and the text width;
// the change routes through the keyed prepared-layout path so the top
// mid-way through a wrapped line keeps its text location — and back.
func TestGutterGrowthRelayoutPreservesAnchor(t *testing.T) {
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("aaaaaaaaaaaa.txt", "x\n", 1, 0, 1, "x"))
	idx.Finish()

	widths := make([]int, 51)
	widths[0] = 100 // one wrapping line; the rest are single cells
	for i := 1; i < len(widths); i++ {
		widths[i] = 1
	}

	m, _ := startBrowse(t, dir, idx, 30, 24)
	m = applyLoad(t, m, loadResult{
		path: []byte("aaaaaaaaaaaa.txt"),
		req:  m.loading["aaaaaaaaaaaa.txt"],
		src:  &stubSource{gutter: 3, widths: widths},
	})
	m.theme = theme.Plain()

	// W=30: min(18, floor(0.4·30)=12, 30−(3+10)=17) = 12; text width 15.
	if got := m.listWidth(30); got != 12 {
		t.Fatalf("list width = %d, want 12", got)
	}
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	anchor := m.vps["aaaaaaaaaaaa.txt"].Anchor()
	if anchor.Line != 0 || anchor.Col != 45 {
		t.Fatalf("anchor = %+v, want line 0 column 45", anchor)
	}

	// The completion's gutter 9 narrows the list to 11 and the text
	// width to 10; the relayout preserves the anchor's location.
	var req int
	m, req = beginLoad(m, "aaaaaaaaaaaa.txt")
	m, cmd := update(t, m, loadResult{
		path: []byte("aaaaaaaaaaaa.txt"),
		req:  req,
		src:  &stubSource{gutter: 9, widths: widths},
	})
	m = deliverCmd(t, m, cmd)

	if got := m.listWidth(30); got != 11 {
		t.Fatalf("list width after gutter growth = %d, want 11", got)
	}
	if got := m.vps["aaaaaaaaaaaa.txt"].Anchor(); got != anchor {
		t.Fatalf("gutter growth moved the anchor to %+v, want %+v", got, anchor)
	}
	if got := m.vps["aaaaaaaaaaaa.txt"].Top(); got != 4 {
		t.Fatalf("top = %d, want the anchor's row 4", got)
	}
	if row := contentRow(t, m); !strings.HasSuffix(row, strings.Repeat("x", 10)) {
		t.Fatalf("top row = %q, want 10 cells of line 0's text", row)
	}

	// Shrinking back restores the earlier geometry and the same top
	// row — the round trip is lossless.
	m, req = beginLoad(m, "aaaaaaaaaaaa.txt")
	m, cmd = update(t, m, loadResult{
		path: []byte("aaaaaaaaaaaa.txt"),
		req:  req,
		src:  &stubSource{gutter: 3, widths: widths},
	})
	m = deliverCmd(t, m, cmd)
	if got := m.listWidth(30); got != 12 {
		t.Fatalf("list width after gutter shrink = %d, want 12", got)
	}
	if got := m.vps["aaaaaaaaaaaa.txt"].Anchor(); got != anchor {
		t.Fatalf("gutter shrink moved the anchor to %+v, want %+v", got, anchor)
	}
	if got := m.vps["aaaaaaaaaaaa.txt"].Top(); got != 3 {
		t.Fatalf("top after the round trip = %d, want 3", got)
	}
}

// Paths wider than their cells are left-truncated with a leading … at
// a grapheme-cluster boundary: a kept suffix never begins with a bare
// combining mark, and a wide glyph is never halved.
func TestTruncateLeftGraphemeSafe(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		w    int
		want string
	}{
		{"fits unchanged", "a/b/c.txt", 20, "a/b/c.txt"},
		{"exact fit unchanged", "abc", 3, "abc"},
		{"ascii suffix", "dir/name.txt", 8, "…ame.txt"},
		{"zero width", "abc", 0, ""},
		{"ellipsis only", "abc", 1, "…"},
		{"combining cluster not split", "abécd", 3, "…cd"},
		{"combining cluster kept whole", "abécd", 4, "…écd"},
		{"wide glyph never halved", "ab日cd", 4, "…cd"},
		{"wide glyph fits", "ab日cd", 5, "…日cd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresentation.TruncateLeftGrapheme(tc.in, tc.w); got != tc.want {
				t.Fatalf("TruncateLeftGrapheme(%q, %d) = %q, want %q", tc.in, tc.w, got, tc.want)
			}
		})
	}
}

// The filename rule reserves a buffer-status slot at the row's right
// end: the embedded path truncates to make room for the note where
// possible, and the composed row never exceeds its width.
func TestFilenameRuleStatusSlot(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
		note string
		w    int
		want string
	}{
		{"no note keeps the plain rule", "a.txt", "", 12, "── a.txt ───"},
		{"note occupies the slot", "a.txt", "note", 16, "── a.txt ─ note "},
		{"path truncates for the note", "averylongname.txt", "note", 20, "── …name.txt ─ note "},
		{"tiny width never overflows", "a.txt", "note", 4, "── a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := filenameRule(tc.file, tc.note, tc.w)
			if got != tc.want {
				t.Fatalf("filenameRule(%q, %q, %d) = %q, want %q",
					tc.file, tc.note, tc.w, got, tc.want)
			}
		})
	}
	// Beyond the envelope the note itself cannot fit; the row still
	// clips to its width rather than overflowing.
	if got := filenameRule("averylongname.txt", "synthetic status", 12); len([]rune(got)) != 12 {
		t.Fatalf("pathological filename rule = %q, want a 12-cell row", got)
	}
}

// In the composed view the buffer-status note occupies the filename
// row's status slot and the safe path truncates to make room for it.
func TestStatusNoteInFilenameRow(t *testing.T) {
	long := strings.Repeat("n", 60) + ".txt"
	m := browseLoaded(t, long, "hit\n", 80, 24)
	m.notes[long] = "synthetic status"

	row := viewRow(t, m, 0)
	if !strings.Contains(row, "synthetic status") {
		t.Fatalf("filename row lacks the status note: %q", row)
	}
	if !strings.Contains(row, "…") {
		t.Fatalf("filename row lacks the truncated path: %q", row)
	}
}
