package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"vrg/internal/filebuffer"
	"vrg/internal/present"
	"vrg/internal/viewport"
)

// shiftTabPress is the message a shift+tab (backtab) keypress delivers.
func shiftTabPress() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
}

// The visible file-list width is the nonnegative minimum of the
// longest sanitized path plus two cells, floor(0.40 × terminal
// width), and the terminal width minus the file panel's minimum —
// gutter + ten text cells + the reserved indicator column. Each term
// can win; the result is never negative.
func TestListWidthFormula(t *testing.T) {
	for _, tc := range []struct {
		name         string
		longest, W   int
		gutterW, res int
		want         int
	}{
		{"longest plus two wins", 5, 80, 3, 0, 7},
		{"longest plus two wins at cap boundary", 30, 80, 3, 0, 32},
		{"forty percent cap wins", 100, 80, 3, 0, 32},
		{"forty percent floor rounds down", 100, 81, 3, 0, 32},
		{"forty percent floor rounds down odd", 100, 83, 3, 0, 33},
		{"forty percent exact", 100, 85, 3, 0, 34},
		{"panel minimum wins wrap", 100, 30, 9, 0, 11},
		{"panel minimum wins run-off-edge", 100, 30, 9, 1, 10},
		{"panel minimum leaves ten text cells", 100, 30, 7, 1, 12},
		{"gutter growth narrows", 100, 25, 3, 0, 10},
		{"gutter grown narrows further", 100, 25, 7, 0, 8},
		{"reserved column counts", 100, 25, 7, 1, 7},
		{"zero width allocation", 8, 20, 9, 1, 0},
		{"pathological stays nonnegative", 50, 19, 9, 1, 0},
		{"empty list", 0, 80, 3, 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				width:    tc.W,
				files:    [][]byte{[]byte("f")},
				listShow: true,
				listEntry: func([]byte) string {
					return strings.Repeat("p", tc.longest)
				},
			}
			if got := m.listWidth(tc.gutterW, tc.res); got != tc.want {
				t.Fatalf("listWidth(longest=%d, W=%d, gutter=%d, res=%d) = %d, want %d",
					tc.longest, tc.W, tc.gutterW, tc.res, got, tc.want)
			}
		})
	}
}

// A path wider than its allotted cells is left-truncated with a
// leading "…", never splitting a grapheme cluster and never exceeding
// the budget — a wide cluster straddling the cut is dropped whole.
func TestTruncateLeftGraphemeSafe(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"fits", "a.txt", 10, "a.txt"},
		{"exact fit", "abc", 3, "abc"},
		{"one cell over", "abcd", 3, "…cd"},
		{"basename kept", "dir/sub/file.txt", 9, "…file.txt"},
		{"one cell budget", "abcdef", 1, "…"},
		{"zero budget", "abc", 0, ""},
		{"negative budget", "abc", -2, ""},
		{"wide cluster straddles cut", "xx世界", 4, "…界"},
		{"combining cluster survives", "aab" + "é" + "cd", 4, "…" + "é" + "cd"},
		{"combining cluster dropped whole", "x" + "é" + "yz", 3, "…yz"},
		{"zwj emoji never split", "a👨‍👩‍👧b", 3, "…b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateLeft(tc.s, tc.n)
			if got != tc.want {
				t.Fatalf("truncateLeft(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
			}
			if w := ansi.StringWidth(got); w > max(0, tc.n) {
				t.Fatalf("truncateLeft(%q, %d) = %q is %d cells — over budget", tc.s, tc.n, got, w)
			}
		})
	}
}

// The filename row embeds the safe path in a horizontal rule and
// carries a buffer-status note slot: the note wins cells over the
// path, which left-truncates — down to nothing — to keep the note
// whole. A note that cannot fit even with an empty path is dropped.
func TestFilenameRuleStatusSlot(t *testing.T) {
	for _, tc := range []struct {
		name string
		path []byte
		note string
		w    int
		want string
	}{
		{"no note", []byte("a.txt"), "", 20, "── a.txt ───────────"},
		{"note joins the rule", []byte("a.txt"), "NOTE", 20, "── a.txt NOTE ──────"},
		{"path truncates for the note", []byte("dir/sub/file.txt"), "NOTE", 16, "── …le.txt NOTE "},
		{"note wins the whole row", []byte("averylongname.txt"), "NOTE", 8, "── NOTE "},
		{"note too wide is dropped", []byte("a.txt"), "TOOLONGNOTE", 14, "── a.txt ─────"},
		{"tiny width is all dashes", []byte("a.txt"), "NOTE", 4, "────"},
		{"zero width", []byte("a.txt"), "NOTE", 0, ""},
		{"no path is all dashes", nil, "NOTE", 10, "──────────"},
		{"wide grapheme never split", []byte("dir/世界.txt"), "N", 12, "── ….txt N ─"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := filenameRule(tc.path, tc.note, tc.w)
			if got != tc.want {
				t.Fatalf("filenameRule(%q, %q, %d) = %q, want %q", tc.path, tc.note, tc.w, got, tc.want)
			}
			if w := ansi.StringWidth(got); w > tc.w {
				t.Fatalf("filenameRule(%q, %q, %d) = %q is %d cells — over width", tc.path, tc.note, tc.w, got, w)
			}
		})
	}
}

// The list starts shown; tab/left hide it and shift+tab/right show it.
// Every hide or show is a text-width change routed through the
// prepared-layout path: the press returns the current file's relayout
// request keyed with the new width. A repeated hide or show is a
// strict no-op — no command, no state change.
func TestListToggleKeys(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		writeWorkFile(t, dir, n, "hit\n")
		recs = append(recs,
			fmt.Sprintf(`{"type":"begin","data":{"path":{"text":"%s"}}}`, n),
			fmt.Sprintf(`{"type":"match","data":{"path":{"text":"%s"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`, n),
			fmt.Sprintf(`{"type":"end","data":{"path":{"text":"%s"},"binary_offset":null}}`, n),
		)
	}
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	if !m.listShow || m.listW != 7 {
		t.Fatalf("initial list state: show=%v w=%d, want shown at 7", m.listShow, m.listW)
	}
	view := ansi.Strip(m.View().Content)
	rows := strings.Split(view, "\n")
	for i, n := range []string{"a.txt", "b.txt", "c.txt"} {
		if !strings.HasPrefix(rows[i], n) {
			t.Fatalf("row %d = %q, want it to lead with list entry %q", i, rows[i], n)
		}
	}

	// tab hides: no list cells, the panel takes the freed width, and a
	// keyed relayout request is issued for the new text width.
	m, cmd = update(t, m, codePress(tea.KeyTab))
	if m.listShow || m.listW != 0 {
		t.Fatalf("tab did not hide the list: show=%v w=%d", m.listShow, m.listW)
	}
	if m.textW != 77 {
		t.Fatalf("textW after tab = %d, want 77 — the freed list cells", m.textW)
	}
	if cmd == nil {
		t.Fatal("tab issued no relayout request")
	}
	msgs := cmdMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("tab's command produced %d messages, want 1", len(msgs))
	}
	lm, ok := msgs[0].(layoutDoneMsg)
	if !ok || lm.key.Width != 77 {
		t.Fatalf("tab request = %T %+v, want layoutDoneMsg at width 77", msgs[0], lm.key)
	}
	m = pump(t, m, lm)
	rows = strings.Split(ansi.Strip(m.View().Content), "\n")
	if !strings.HasPrefix(rows[0], "──") {
		t.Fatalf("hidden-list row 0 = %q, want the filename rule from column 0", rows[0])
	}
	for i, n := range []string{"b.txt", "c.txt"} {
		if strings.Contains(rows[i+1], n) {
			t.Fatalf("hidden list still drew %q in row %d: %q", n, i+1, rows[i+1])
		}
	}

	// tab while hidden is a strict no-op.
	m, cmd = update(t, m, codePress(tea.KeyTab))
	if cmd != nil {
		t.Fatalf("repeated tab produced a command %T, want none", cmd)
	}
	if m.listShow || m.listW != 0 {
		t.Fatal("repeated tab changed the hidden list")
	}

	// shift+tab shows the list again, again through a keyed relayout.
	m, cmd = update(t, m, shiftTabPress())
	if !m.listShow || m.listW != 7 || m.textW != 70 {
		t.Fatalf("shift+tab did not restore the list: show=%v listW=%d textW=%d",
			m.listShow, m.listW, m.textW)
	}
	m = settle(t, m, cmd)
	rows = strings.Split(ansi.Strip(m.View().Content), "\n")
	if !strings.HasPrefix(rows[1], "b.txt") {
		t.Fatalf("restored row 1 = %q, want the b.txt entry", rows[1])
	}

	// left hides and right shows — the same toggle on the arrow keys.
	m, cmd = update(t, m, codePress(tea.KeyLeft))
	if m.listShow || m.listW != 0 {
		t.Fatalf("left did not hide the list: show=%v w=%d", m.listShow, m.listW)
	}
	m = settle(t, m, cmd)
	m, cmd = update(t, m, codePress(tea.KeyRight))
	if !m.listShow || m.listW != 7 {
		t.Fatalf("right did not show the list: show=%v w=%d", m.listShow, m.listW)
	}
	m = settle(t, m, cmd)
	if !strings.HasPrefix(strings.Split(ansi.Strip(m.View().Content), "\n")[2], "c.txt") {
		t.Fatal("restored list lost the c.txt entry")
	}
}

// The toggle keys are browse keys: while the search runs the
// preference stays untouched and no command issues.
func TestListToggleKeysInertWhileSearching(t *testing.T) {
	m := newModel(make(chan searchDoneMsg), nil)
	for _, k := range []tea.KeyPressMsg{
		codePress(tea.KeyTab), shiftTabPress(),
		codePress(tea.KeyLeft), codePress(tea.KeyRight),
	} {
		m2, cmd := update(t, m, k)
		if cmd != nil {
			t.Fatalf("%q while searching produced a command %T", k.String(), cmd)
		}
		if !m2.listShow {
			t.Fatalf("%q while searching cleared the visibility preference", k.String())
		}
	}
}

// Hide and show are text-width changes: with the top mid-way through a
// wrapped line, the anchor's text location stays at the top through
// the tab / shift+tab round trip — each transition rewraps through the
// prepared-layout path keyed by the new width.
func TestListHideShowPreservesAnchorText(t *testing.T) {
	dir := t.TempDir()
	// A 300-cell first line with NEEDLE at cells 70–75: at textW 70,
	// scrolling one row down puts NEEDLE at the top row's start.
	var sb strings.Builder
	sb.WriteString(strings.Repeat("x", 70) + "NEEDLE" + strings.Repeat("x", 224) + "\n")
	for i := 2; i <= 9; i++ {
		fmt.Fprintf(&sb, "x%05d\n", i)
	}
	writeWorkFile(t, dir, "a.txt", sb.String())
	m, cmd := browseModel(t, dir, 80, 8,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"x00002\n"},"line_number":2,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m = settle(t, m, cmd)
	m, _ = update(t, m, codePress(tea.KeyDown))
	anchor := viewport.Target{Line: 0, Cell: 70}
	if m.vp.Anchor() != anchor {
		t.Fatalf("anchor after one down = %v, want %v", m.vp.Anchor(), anchor)
	}
	topRow := func() string {
		return strings.Split(ansi.Strip(m.View().Content), "\n")[1]
	}
	if !strings.Contains(topRow(), "NEEDLE") {
		t.Fatalf("top row before tab lacks the anchor text: %q", topRow())
	}

	m, cmd = update(t, m, codePress(tea.KeyTab))
	if cmd == nil {
		t.Fatal("tab issued no relayout request")
	}
	m = settle(t, m, cmd)
	if m.vp.Anchor() != anchor {
		t.Fatalf("hide moved the anchor to %v, want %v", m.vp.Anchor(), anchor)
	}
	if !strings.Contains(topRow(), "NEEDLE") {
		t.Fatalf("top row after tab lacks the anchor text: %q", topRow())
	}

	m, cmd = update(t, m, shiftTabPress())
	m = settle(t, m, cmd)
	if m.vp.Anchor() != anchor {
		t.Fatalf("show moved the anchor to %v, want %v", m.vp.Anchor(), anchor)
	}
	if !strings.Contains(topRow(), "NEEDLE") {
		t.Fatalf("top row after shift+tab lacks the anchor text: %q", topRow())
	}
}

// A load completing with a larger gutter narrows the text width: the
// rewrap goes through the prepared-layout path and the current file's
// anchor location stays at the top — in both directions of a
// gutter-growth round trip.
func TestGutterGrowthRelayoutKeepsAnchorText(t *testing.T) {
	dir := t.TempDir()
	writeA := func(lines int) {
		var sb strings.Builder
		sb.WriteString(strings.Repeat("x", 70) + "NEEDLE" + strings.Repeat("x", 224) + "\n")
		for i := 2; i <= lines; i++ {
			fmt.Fprintf(&sb, "x%06d\n", i)
		}
		writeWorkFile(t, dir, "a.txt", sb.String())
	}
	writeA(9)
	m, cmd := browseModel(t, dir, 80, 8,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"x000002\n"},"line_number":2,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m = settle(t, m, cmd)
	m, _ = update(t, m, codePress(tea.KeyDown))
	anchor := viewport.Target{Line: 0, Cell: 70}
	if m.vp.Anchor() != anchor {
		t.Fatalf("anchor after one down = %v, want %v", m.vp.Anchor(), anchor)
	}
	topRow := func(mm Model) string {
		return strings.Split(ansi.Strip(mm.View().Content), "\n")[1]
	}
	if !strings.Contains(topRow(m), "NEEDLE") {
		t.Fatalf("top row before growth lacks NEEDLE: %q", topRow(m))
	}
	if m.textW != 70 {
		t.Fatalf("textW = %d, want 70 at gutter 3", m.textW)
	}

	reload := func(mm Model) Model {
		t.Helper()
		stops := stopsForFile(mm.index, []byte("a.txt"))
		buf, err := filebuffer.Load(stops[0].ResolvedPath, stops)
		if err != nil {
			t.Fatalf("reload load: %v", err)
		}
		mm, cmd = update(t, mm, loadDoneMsg{path: []byte("a.txt"), buf: buf})
		if cmd == nil {
			t.Fatal("the gutter-changing load issued no relayout request")
		}
		return settle(t, mm, cmd)
	}

	// Growth to five-digit line numbers: gutter 3→7 narrows the text
	// width to 66; the anchor's text stays at the top.
	writeA(12000)
	m = reload(m)
	if m.textW != 66 {
		t.Fatalf("textW after gutter growth = %d, want 66", m.textW)
	}
	if m.vp.Anchor() != anchor {
		t.Fatalf("gutter growth moved the anchor to %v, want %v", m.vp.Anchor(), anchor)
	}
	if !strings.Contains(topRow(m), "NEEDLE") {
		t.Fatalf("top row after gutter growth lacks NEEDLE: %q", topRow(m))
	}

	// Shrinking back rewraps around the same anchor.
	writeA(9)
	m = reload(m)
	if m.textW != 70 {
		t.Fatalf("textW after gutter shrink = %d, want 70", m.textW)
	}
	if m.vp.Anchor() != anchor {
		t.Fatalf("gutter shrink moved the anchor to %v, want %v", m.vp.Anchor(), anchor)
	}
	if !strings.Contains(topRow(m), "NEEDLE") {
		t.Fatalf("top row after gutter shrink lacks NEEDLE: %q", topRow(m))
	}
}

// At 30 columns with a five-digit gutter in run-off-edge mode the
// panel's minimum binds: the formula leaves ten text cells plus the
// reserved indicator column — a constrained, nonzero list.
func TestListReducedLeavesTenTextCells(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	for i := 0; i < 10000; i++ {
		sb.WriteString("x\n")
	}
	writeWorkFile(t, dir, "averylongfilename.txt", sb.String())
	m, cmd := browseModel(t, dir, 30, 24,
		`{"type":"begin","data":{"path":{"text":"averylongfilename.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"averylongfilename.txt"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"end","data":{"path":{"text":"averylongfilename.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m = settle(t, m, cmd)
	if m.gutterDigits() != 5 {
		t.Fatalf("gutter digits = %d, want 5", m.gutterDigits())
	}
	// Wrap mode: min(23, 12, 13) = 12 leaves textW 11.
	if m.listW != 12 || m.textW != 11 {
		t.Fatalf("wrap layout: listW=%d textW=%d, want 12/11", m.listW, m.textW)
	}
	// Run-off-edge reserves the indicator column: min(23, 12, 12) = 12
	// leaves exactly ten text cells plus the reserved column.
	m, cmd = update(t, m, keyPress("w"))
	m = settle(t, m, cmd)
	if m.listW != 12 || m.textW != 10 {
		t.Fatalf("run-off-edge layout: listW=%d textW=%d, want 12/10", m.listW, m.textW)
	}
}

// A computed zero-width list draws no cells but the visibility
// preference is untouched — no automatic toggling. The Issue #24
// example is synthetic: W=20, gutter 9, reserved indicator 1 leaves
// exactly zero.
func TestZeroWidthAllocationKeepsPreference(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "hit\n")
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m = settle(t, m, cmd)

	m.width = 20
	if got := m.listWidth(9, 1); got != 0 {
		t.Fatalf("listWidth at W=20 gutter=9 reserved=1 = %d, want 0", got)
	}
	if got := m.listWidth(9, 2); got < 0 {
		t.Fatalf("pathological width produced %d, want nonnegative", got)
	}
	if !m.listShow {
		t.Fatal("a zero-width allocation cleared the visibility preference")
	}
	// The list returns when width allows — the preference never moved.
	m.width = 80
	if got := m.listWidth(9, 1); got <= 0 {
		t.Fatalf("listWidth back at W=80 = %d, want the list's cells", got)
	}

	// The zero-allocation frame draws no list cells: the content row
	// leads with the gutter, not the entry.
	m.width = 20
	m.listW = m.listWidth(9, 1)
	m.textW = max(0, m.width-m.listW-9-1)
	row := strings.Split(ansi.Strip(m.View().Content), "\n")[1]
	if strings.HasPrefix(row, "a.txt") {
		t.Fatalf("zero-width list drew its entry: %q", row)
	}
}

// The file list scrolls just enough to keep the active entry visible:
// navigating past the window's end shifts it down; navigating back up
// leaves later entries in view until the entry crosses the top edge.
func TestListScrollsToKeepActiveVisible(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("f%02d.txt", i)
		writeWorkFile(t, dir, name, "hit\n")
		recs = append(recs,
			fmt.Sprintf(`{"type":"begin","data":{"path":{"text":"%s"}}}`, name),
			fmt.Sprintf(`{"type":"match","data":{"path":{"text":"%s"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`, name),
			fmt.Sprintf(`{"type":"end","data":{"path":{"text":"%s"},"binary_offset":null}}`, name),
		)
	}
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 10, recs...)
	m = settle(t, m, cmd)

	// Twenty-five forward steps land on f25.txt: the ten-row window
	// shows f16..f25 with the active entry on the last row.
	for i := 0; i < 25; i++ {
		var c tea.Cmd
		m, c = update(t, m, keyPress("n"))
		m = settle(t, m, c)
	}
	if m.listTop != 16 {
		t.Fatalf("listTop after 25 forward steps = %d, want 16", m.listTop)
	}
	m, _ = update(t, m, escPress()) // dismiss the file-change pop-up
	v := m.View().Content
	if !strings.Contains(v, "\x1b[4mf25.txt") {
		t.Fatalf("active entry not underlined in the view: %q", v)
	}
	if strings.Contains(ansi.Strip(v), "f00.txt") {
		t.Fatal("scrolled-off f00.txt still rendered")
	}

	// Five steps back land on f20.txt: the window does not move — the
	// entries below the active one stay visible.
	for i := 0; i < 5; i++ {
		var c tea.Cmd
		m, c = update(t, m, keyPress("p"))
		m = settle(t, m, c)
	}
	if m.listTop != 16 {
		t.Fatalf("listTop after retreating to f20 = %d, want the unmoved 16", m.listTop)
	}
	m, _ = update(t, m, escPress())
	v = m.View().Content
	if !strings.Contains(v, "\x1b[4mf20.txt") {
		t.Fatalf("active entry f20.txt not underlined: %q", v)
	}
	if !strings.Contains(ansi.Strip(v), "f25.txt") {
		t.Fatal("minimal scroll dropped the entries below the active one")
	}

	// Retreating past the window's top edge shifts it up to include
	// the active entry.
	for i := 0; i < 5; i++ {
		var c tea.Cmd
		m, c = update(t, m, keyPress("p"))
		m = settle(t, m, c)
	}
	if m.listTop != 15 {
		t.Fatalf("listTop after crossing the top edge = %d, want 15", m.listTop)
	}
	m, _ = update(t, m, escPress())
	v = m.View().Content
	if !strings.Contains(v, "\x1b[4mf15.txt") {
		t.Fatalf("active entry f15.txt not underlined: %q", v)
	}
}

// A frame render formats only the entries inside the scrolled window —
// the item provider is queried once per visible row, for exactly the
// shown slice, even deep into the list.
func TestListRenderQueriesOnlyVisibleWindow(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("f%02d.txt", i)
		writeWorkFile(t, dir, name, "hit\n")
		recs = append(recs,
			fmt.Sprintf(`{"type":"begin","data":{"path":{"text":"%s"}}}`, name),
			fmt.Sprintf(`{"type":"match","data":{"path":{"text":"%s"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`, name),
			fmt.Sprintf(`{"type":"end","data":{"path":{"text":"%s"},"binary_offset":null}}`, name),
		)
	}
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 10, recs...)
	m = settle(t, m, cmd)
	for i := 0; i < 25; i++ {
		var c tea.Cmd
		m, c = update(t, m, keyPress("n"))
		m = settle(t, m, c)
	}

	var queried []string
	m.listEntry = func(b []byte) string {
		queried = append(queried, string(b))
		return present.Path(b)
	}
	m, _ = update(t, m, escPress())
	_ = m.View()
	want := make([]string, 0, m.height)
	for _, f := range m.files[m.listTop : m.listTop+m.height] {
		want = append(want, string(f))
	}
	if fmt.Sprint(queried) != fmt.Sprint(want) {
		t.Fatalf("frame queried %v, want the visible window %v", queried, want)
	}
}

// The filename row's buffer-status slot renders the note the provider
// returns — a synthetic string here; the real notes are owned by
// Issues #26, #29, and #30 — and the path truncates to make room for
// it at constrained widths.
func TestStatusSlotRendersInFilenameRow(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "averyverylongfilename.txt", "hit\n")
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"averyverylongfilename.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"averyverylongfilename.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"averyverylongfilename.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m = settle(t, m, cmd)
	m.statusNote = func([]byte) string { return "(synthetic)" }

	row0 := func(mm Model) string {
		return strings.Split(ansi.Strip(mm.View().Content), "\n")[0]
	}
	if got := row0(m); !strings.Contains(got, "averyverylongfilename.txt (synthetic)") {
		t.Fatalf("filename row lacks the status note: %q", got)
	}

	// At 30 columns the panel is 18 cells: the path truncates to two
	// cells so the whole note still shows.
	m, cmd = update(t, m, tea.WindowSizeMsg{Width: 30, Height: 24})
	m = settle(t, m, cmd)
	got := row0(m)
	if !strings.Contains(got, "(synthetic)") {
		t.Fatalf("status note lost at width 30: %q", got)
	}
	if strings.Contains(got, "averyverylongfilename.txt") {
		t.Fatalf("path did not truncate for the note: %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("truncated path lacks the … marker: %q", got)
	}
}
