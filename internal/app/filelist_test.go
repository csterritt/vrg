package app

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rivo/uniseg"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

var (
	keyLeft     = tea.KeyPressMsg{Code: tea.KeyLeft}
	keyRight    = tea.KeyPressMsg{Code: tea.KeyRight}
	keyTab      = tea.KeyPressMsg{Code: tea.KeyTab}
	keyShiftTab = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
)

// dropCells returns row without its first n display cells — the
// cell-accurate way to slice the file list off an unstyled frame
// row: a byte index would land inside the multi-byte … the Issue
// #24 left-truncation prepends to over-wide paths.
func dropCells(row string, n int) string {
	col := 0
	rest := row
	state := -1
	for len(rest) > 0 {
		if col >= n {
			return rest
		}
		var cw int
		_, rest, cw, state = uniseg.FirstGraphemeClusterInString(rest, state)
		col += cw
	}
	return ""
}

// The file list's rendered width is the nonnegative minimum of the
// longest displayed path width plus two, floor(0.40 × terminal width),
// and terminal width minus (gutter width + 10 + reserved indicator
// width) — Issue #24's three-term formula with the ten-cell text
// minimum taking precedence over the 40% cap.
func TestFileListWidthFormula(t *testing.T) {
	for _, c := range []struct {
		name                           string
		longest, width, gutter, resInd int
		want                           int
	}{
		// Each term wins in turn.
		{"longest path + 2 wins", 8, 80, 3, 0, 10},
		{"40%% cap wins", 60, 80, 3, 0, 32},
		{"minimum content width wins", 60, 25, 7, 1, 7}, // leaves 7 gutter + 10 text + 1 indicator
		{"equal terms", 30, 80, 3, 0, 32},
		// floor(0.40 × W) rounding.
		{"40%% floors", 60, 82, 3, 0, 32}, // floor(32.8) = 32
		{"40%% next integer", 60, 85, 3, 0, 34},
		// A larger gutter shrinks the third term.
		{"gutter growth narrows third term", 60, 26, 7, 0, 9}, // 26-17 < floor(10.4)
		// Zero-width allocation and the nonnegative clamp.
		{"zero allocation", 8, 20, 9, 1, 0}, // 20-(9+10+1) = 0
		{"negative clamps to zero", 8, 20, 12, 1, 0},
	} {
		if got := fileListWidth(c.longest, c.width, c.gutter, c.resInd); got != c.want {
			t.Errorf("%s: fileListWidth(%d, %d, %d, %d) = %d, want %d",
				c.name, c.longest, c.width, c.gutter, c.resInd, got, c.want)
		}
	}
}

// left and tab hide the file list; right and shift+tab show it; the
// list is shown at startup. Each toggle changes the text width, so the
// current file's layout is re-keyed through the Issue #17 prepared-
// layout path and reinstalled on delivery.
func TestListHideShowToggles(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)

	if !m.listVisible {
		t.Fatal("the file list is not shown at startup")
	}
	if !strings.Contains(frameRow(t, m, 1), "a.txt") {
		t.Fatal("the file list entry is not visible at startup")
	}
	textW0 := m.rows[key].(*viewport.Rows).Key().TextWidth

	assertHidden := func(k tea.KeyPressMsg) {
		t.Helper()
		_, cmd := m.Update(k)
		if m.listVisible {
			t.Fatalf("%s did not hide the file list", k.Keystroke())
		}
		if cmd == nil {
			t.Fatalf("%s hiding the list issued no layout request", k.Keystroke())
		}
		deliverLayout(t, m, cmd)
		for r := 1; r < m.height; r++ {
			if row := frameRow(t, m, r); strings.Contains(row, "a.txt") || strings.Contains(row, "b.txt") {
				t.Fatalf("%s: a list entry is still drawn at row %d: %q", k.Keystroke(), r, row)
			}
		}
		if got := m.rows[key].(*viewport.Rows).Key().TextWidth; got <= textW0 {
			t.Fatalf("%s: text width = %d, want the widened panel > %d", k.Keystroke(), got, textW0)
		}
	}
	assertShown := func(k tea.KeyPressMsg) {
		t.Helper()
		_, cmd := m.Update(k)
		if !m.listVisible {
			t.Fatalf("%s did not show the file list", k.Keystroke())
		}
		deliverLayout(t, m, cmd)
		if !strings.Contains(frameRow(t, m, 1), "a.txt") {
			t.Fatalf("%s: the list entry was not redrawn", k.Keystroke())
		}
		if got := m.rows[key].(*viewport.Rows).Key().TextWidth; got != textW0 {
			t.Fatalf("%s: text width = %d, want the restored %d", k.Keystroke(), got, textW0)
		}
	}

	assertHidden(keyTab)
	assertShown(keyShiftTab)
	assertHidden(keyLeft)
	assertShown(keyRight)
}

// A hide key on an already-hidden list and a show key on an
// already-shown list are no-ops: no command, no preference change.
func TestListToggleIdempotent(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx))

	m.Update(keyTab) // hide; the layout request stays in flight
	for _, k := range []tea.KeyPressMsg{keyTab, keyLeft} {
		if _, cmd := m.Update(k); cmd != nil {
			t.Fatalf("%s on a hidden list returned a command", k.Keystroke())
		}
	}
	if m.listVisible {
		t.Fatal("a repeated hide key changed the visibility preference")
	}

	// shift+tab restores the shown preference; the still-installed
	// pre-hide layout already matches, so no request is needed.
	if _, cmd := m.Update(keyShiftTab); cmd != nil {
		t.Fatal("shift+tab issued a redundant layout request")
	}
	for _, k := range []tea.KeyPressMsg{keyShiftTab, keyRight} {
		if _, cmd := m.Update(k); cmd != nil {
			t.Fatalf("%s on a shown list returned a command", k.Keystroke())
		}
	}
	if !m.listVisible {
		t.Fatal("a repeated show key changed the visibility preference")
	}
}

// synthRows adapts countingRows for synthetic layout cases: a chosen
// gutter width so tests can drive gutters no real file could produce.
type synthRows struct {
	countingRows
	gutter int
}

func (s *synthRows) GutterWidth() int { return s.gutter }

// A terminal too narrow to satisfy the third term allocates zero list
// cells: nothing is drawn, but the user's visibility preference is
// untouched — no automatic toggling — and the list returns on growth.
func TestZeroWidthListRetainsPreference(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)

	// Synthetic dimensions: at 20 columns a nine-cell gutter, the
	// ten-cell minimum, and the run-off-edge indicator column consume
	// the whole frame — W - (9+10+1) = 0.
	m.Update(keyW) // run-off-edge: the indicator column is reserved
	m.rows[key] = &synthRows{countingRows: countingRows{n: 50}, gutter: 9}
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 24})

	if got := m.listWidth(); got != 0 {
		t.Fatalf("list width = %d, want the zero-width allocation", got)
	}
	if !m.listVisible {
		t.Fatal("zero-width allocation toggled the visibility preference")
	}
	v := viewText(m)
	if n := strings.Count(v, "b.txt"); n != 0 {
		t.Fatalf("b.txt appears %d times, want 0 — no list cells may be drawn", n)
	}
	if n := strings.Count(v, "a.txt"); n != 1 {
		t.Fatalf("a.txt appears %d times, want 1 — the filename rule only", n)
	}

	// Growth restores the list: the preference survived untouched.
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.listWidth() == 0 {
		t.Fatal("the list did not return when the terminal widened")
	}
	if v := viewText(m); !strings.Contains(v, "b.txt") {
		t.Fatalf("view = %q, want the file list redrawn", v)
	}
}

// Paths wider than their cells left-truncate with a leading …,
// keeping the basename end visible and never splitting a grapheme
// cluster: a wide or combining cluster straddling the boundary drops
// or joins whole.
func TestLeftTruncateGraphemeSafe(t *testing.T) {
	for _, c := range []struct {
		name, in string
		w        int
		want     string
	}{
		{"fits", "abc", 5, "abc"},
		{"exact fit", "abc", 3, "abc"},
		{"truncates", "abcdef", 4, "…def"},
		{"wide cluster dropped whole", "ab文cd", 4, "…cd"},
		{"wide cluster kept whole", "ab文cd", 5, "…文cd"},
		{"combining cluster kept whole", "xbe\u0301", 2, "…e\u0301"},
		{"one cell", "abcdef", 1, "…"},
		{"zero width", "abc", 0, ""},
		{"empty", "", 4, ""},
	} {
		if got := leftTruncate(c.in, c.w); got != c.want {
			t.Errorf("%s: leftTruncate(%q, %d) = %q, want %q", c.name, c.in, c.w, got, c.want)
		}
	}
}

// A list entry whose path exceeds the list's cells renders the
// left-truncated form: a leading … and the basename tail.
func TestListEntriesLeftTruncate(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: "hit\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
		{name: "b.txt", content: "hit\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
	})
	startBrowse(t, m, idx)

	listW := m.listWidth()
	path := escapedPath(idx.Files[0])
	if safepresentation.CellWidth(path) <= listW {
		t.Skipf("fixture path %q (%d cells) fits the %d-cell list", path, safepresentation.CellWidth(path), listW)
	}
	entry := clipCells(frameRow(t, m, 1), listW) // the entry's cells, not bytes — … is multi-byte
	if !strings.HasPrefix(entry, "…") || !strings.HasSuffix(strings.TrimRight(entry, " "), "a.txt") {
		t.Fatalf("list entry = %q, want a …-led truncation ending in the basename", entry)
	}
}

// File-list entry padding is measured in cells (Issue #39): paths of
// wide and combining characters pad to exactly the list's cell width —
// a rune count would leave wide entries short and combining entries
// over-padded.
func TestListEntryWidePathPadsInCells(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{
		{name: "文文.txt", content: "hit\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
		{name: "é.txt", content: "hit\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
		{name: "b.txt", content: "hit\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
	})
	startBrowse(t, m, idx)
	listW := m.listWidth()
	for i := range idx.Files {
		// The styled entry measures its content cells — SGR runs
		// around the text are not cells.
		if w := safepresentation.CellWidth(m.listCell(i, listW)); w != listW {
			t.Fatalf("entry %d = %d cells, want the %d-cell list width", i, w, listW)
		}
	}
}

// The filename row fits by cells (Issue #39): a wide path embeds in a
// rule measuring exactly the frame width — a rune count would leave
// the rule short of the right edge.
func TestFilenameRuleWidePath(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{
		{name: "文文文.txt", content: "hit\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	row := frameRow(t, m, 0)
	if w := safepresentation.CellWidth(row); w != 80 {
		t.Fatalf("filename row = %d cells, want the 80-cell frame: %q", w, row)
	}
	if !strings.Contains(row, "文文文.txt") {
		t.Fatalf("filename row = %q, want the wide path's tail embedded", row)
	}
}

// The filename row provides a buffer-status note slot at its right
// edge: the note renders inside the rule and the embedded path
// truncates — leading …, grapheme-safe — to make room for it where
// possible. The slot exists now for a synthetic status string; the
// real notes arrive with Issues 26, 29, and 30.
func TestFilenameRuleStatusSlot(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{
		{name: "deep-" + strings.Repeat("n", 70) + "-leaf.txt", content: "hit\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	m.notes[string(idx.Files[0].Path)] = "SYNTH-NOTE"

	row := frameRow(t, m, 0)
	for _, want := range []string{"SYNTH-NOTE", "…", "leaf.txt"} {
		if !strings.Contains(row, want) {
			t.Fatalf("filename row = %q, want %q — the note in its slot with the path truncated", row, want)
		}
	}
	if w := safepresentation.CellWidth(row); w > 80 {
		t.Fatalf("filename row = %d cells at an 80-cell frame: %q", w, row)
	}

	// Constrained width: the path yields entirely and the note itself
	// clips to the slot — nothing overflows.
	m.Update(tea.WindowSizeMsg{Width: 24, Height: 24})
	row = frameRow(t, m, 0)
	if w := safepresentation.CellWidth(row); w > 24 {
		t.Fatalf("filename row = %d cells at a 24-cell frame: %q", w, row)
	}
	if !strings.Contains(row, "SYNTH") {
		t.Fatalf("filename row = %q, want the note kept (clipped) at constrained width", row)
	}
}

// With no note recorded for the current file the filename row is the
// plain embedded-path rule.
func TestFilenameRuleNoNote(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx))

	row := frameRow(t, m, 0)
	if !strings.HasPrefix(row, "─ "+escapedPath(idx.Files[0])+" ") {
		t.Fatalf("filename row = %q, want the plain %q rule", row, "─ path ─…")
	}
	if strings.Contains(row, "NOTE") {
		t.Fatalf("filename row = %q, want no status note", row)
	}
}

// The file list scrolls to keep the active entry visible: navigating
// deep into a long list keeps the current file's underlined entry on
// screen — in both directions.
func TestListAutoScrollsToActiveEntry(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	files := make([]navFile, 50)
	for i := range files {
		files[i] = navFile{
			name:    fmt.Sprintf("f%02d.txt", i),
			content: "hit\n",
			stops:   []navStop{{line: 1, start: 0, end: 3}},
		}
	}
	idx := navIndex(t, files)
	startBrowse(t, m, idx)

	for i := 0; i < 30; i++ {
		m.Update(keyN)
	}
	// The crossing's pop-up would overlay the frame; expire it.
	m.Update(popupExpireMsg{id: m.popupID})

	v := viewText(m)
	if !strings.Contains(v, "f30.txt") {
		t.Fatalf("view = %q, want the active f30.txt entry scrolled into view", v)
	}
	if strings.Contains(v, "f00.txt") {
		t.Fatalf("view = %q, f00.txt scrolled off but still renders", v)
	}
	// The active entry is the underlined one — a …-led truncation
	// ending in the basename.
	if !strings.Contains(v, "\x1b[37;40;4m…") || !strings.Contains(v, "f30.txt\x1b[37;40;24m") {
		t.Fatalf("view = %q, want f30.txt's truncated entry underlined", v)
	}

	for i := 0; i < 10; i++ {
		m.Update(keyP)
	}
	m.Update(popupExpireMsg{id: m.popupID})
	v = viewText(m)
	if !strings.Contains(v, "f20.txt") {
		t.Fatalf("view = %q, want f20.txt kept visible scrolling back", v)
	}
	if strings.Contains(v, "f30.txt") {
		t.Fatalf("view = %q, f30.txt scrolled off but still renders", v)
	}
}

// Scrolled partway into a wrapped line, tab then shift+tab: the
// logical anchor carries the reading position through each rewrap, so
// the top row holds the same text location before, between, and after
// the toggles.
func TestAnchorSurvivesListToggle(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("x", 500) + "\n" + numberedContent("pad", 20),
		stops:   []navStop{{line: 1, start: 0, end: 1}, {line: 15, start: 0, end: 3}},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)

	// Reveal the second stop, then scroll back partway into the
	// wrapped 500-cell line: the anchor lands mid-line.
	m.Update(keyN)
	for i := 0; i < 3; i++ {
		m.Update(keyUp)
	}
	anchor0 := m.vps[key].Anchor()
	if anchor0.Line != 1 || anchor0.Cell == 0 {
		t.Fatalf("anchor = %+v, want a mid-line anchor on line 1", anchor0)
	}
	top0 := m.vps[key].Top()

	// tab hides the list: the panel widens, the layout re-keys through
	// the prepared-layout path, and the anchor's row holds the top.
	_, cmd := m.Update(keyTab)
	if cmd == nil {
		t.Fatal("tab hiding the list issued no layout request")
	}
	deliverLayout(t, m, cmd)
	rows := m.rows[key]
	if got, want := m.vps[key].Top(), rows.RowOf(anchor0); got != want {
		t.Fatalf("top after tab = %d, want the anchor's row %d", got, want)
	}
	if got := m.vps[key].Anchor(); got != anchor0 {
		t.Fatalf("anchor after tab = %+v, want %+v", got, anchor0)
	}

	// shift+tab restores the list; the original top returns.
	_, cmd = m.Update(keyShiftTab)
	deliverLayout(t, m, cmd)
	if got := m.vps[key].Anchor(); got != anchor0 {
		t.Fatalf("anchor after shift+tab = %+v, want %+v", got, anchor0)
	}
	if got := m.vps[key].Top(); got != top0 {
		t.Fatalf("top after shift+tab = %d, want the original %d", got, top0)
	}
}

// A load completing with a larger gutter narrows the list and the text
// width: the rewrap goes through the prepared-layout path and the
// current file's anchor location stays at the top.
func TestAnchorSurvivesGutterGrowth(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	// a.txt grows from 60 lines (four-cell gutter) to 20,000 (seven)
	// between loads; line 1 is long enough to wrap at both widths.
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("x", 200) + "\n" + numberedContent("pad", 59),
		stops:   []navStop{{line: 10, start: 0, end: 3}},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)

	// At 26 columns the list sits at its 40% cap; scroll partway into
	// the wrapped first line for a mid-line anchor.
	_, rc := m.Update(tea.WindowSizeMsg{Width: 26, Height: 24})
	deliverLayout(t, m, rc)
	if got := m.listWidth(); got != 10 {
		t.Fatalf("list width = %d at 26 columns, want the 40%% cap 10", got)
	}
	for i := 0; i < 5; i++ {
		m.Update(keyDown)
	}
	anchor0 := m.vps[key].Anchor()
	if anchor0.Line != 1 || anchor0.Cell == 0 {
		t.Fatalf("anchor = %+v, want a mid-line anchor on line 1", anchor0)
	}

	// The file grows to five-digit line numbers on disk; the reload's
	// completion arrives and its freshly keyed layout installs. The
	// request is minted the way Issue #27's reload will mint it.
	big := strings.Repeat("x", 200) + "\n" + numberedContent("pad", 19999)
	if err := os.WriteFile(string(idx.Files[0].Path), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load(idx.Files[0].Path, idx.Files[0].Stops)
	if err != nil {
		t.Fatal(err)
	}
	if buf.GutterWidth() != 7 {
		t.Fatalf("gutter = %d, want 7 for a five-digit file", buf.GutterWidth())
	}
	mintRequest(m, idx.Files[0].Path)
	injectLoad(t, m, fileLoadedMsg{path: idx.Files[0].Path, buf: buf, reload: true})

	if got := m.listWidth(); got != 9 {
		t.Fatalf("list width = %d after the gutter grew, want 9", got)
	}
	rows := m.rows[key]
	if got, want := m.vps[key].Top(), rows.RowOf(anchor0); got != want {
		t.Fatalf("top after the larger-gutter load = %d, want the anchor's row %d", got, want)
	}
	if got := m.vps[key].Anchor(); got != anchor0 {
		t.Fatalf("anchor after the larger-gutter load = %+v, want %+v", got, anchor0)
	}
}

// A hidden list formats nothing — and neither does a shown one:
// display metadata is prepared once at search completion (Issue #40),
// so a frame queries the escaper for no paths at all, not even the
// filename rule's.
func TestHiddenListEscapesNoEntries(t *testing.T) {
	var escapes int
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		escapePath: func(p []byte) string {
			escapes++
			return safepresentation.EscapePath(p)
		},
	})
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx))

	m.Update(keyTab)
	escapes = 0
	viewText(m)
	if escapes != 0 {
		t.Fatalf("hidden list frame escaped %d paths, want 0 — path metadata is prepared at search completion", escapes)
	}
}
