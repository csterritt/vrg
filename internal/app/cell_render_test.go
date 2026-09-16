package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// --- Issue #39: final rendering from the shared grapheme/cell model ---
//
// These tests assert on the composed View() output: the emitted cell
// layout itself, not private helper return values. A match overlapping
// a two-cell CJK character must style exactly that cluster's cells; a
// base-plus-combining sequence and an emoji ZWJ sequence must be styled,
// clipped, and truncated only as one cluster; list padding, indicator
// sizing, filename rows, and the file-change pop-up must all measure
// terminal cells under the shared grapheme policy.

// TestRenderCJKHighlightCoversExactlyClusterCells verifies that a match
// overlapping a two-cell CJK character highlights exactly that
// cluster's cells and never swallows the following character (Issue
// #39). "a中b": 中 occupies display cells [1, 3); the highlight covers
// [1, 3); the styled span must be exactly "中" followed by an unstyled
// "b".
func TestRenderCJKHighlightCoversExactlyClusterCells(t *testing.T) {
	raw := "a中b\n"
	// Submatch on the first two bytes of 中 expands to the whole
	// cluster: display cells [1, 3).
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"\xe4\xb8", 1, 3}),
	)
	line := makeExpandedBufLine(1, raw, [2]int{1, 3})
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	// Line 1 is the current line, so the match is styled with the
	// current-match sequence and restored to base.
	want := "\x1b[30;47;4m中\x1b[37;40mb"
	if !strings.Contains(view, want) {
		t.Fatalf("view does not contain %q (highlight must cover exactly the 中 cluster, never the following b): %q", want, view)
	}
}

// TestRenderCombiningClusterStyledAsOne verifies that a
// base-plus-combining sequence is styled only as one cluster (Issue
// #39). "xéy": the é cluster (e + combining acute) occupies
// cell [1, 2); the styled span must be the whole cluster "é",
// never just the base "e" with the mark left unstyled.
func TestRenderCombiningClusterStyledAsOne(t *testing.T) {
	raw := "xéy\n"
	// Submatch on the base "e" only (byte 1) expands to the whole é
	// cluster: display cells [1, 2).
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"e", 1, 2}),
	)
	line := makeExpandedBufLine(1, raw, [2]int{1, 2})
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	want := "\x1b[30;47;4mé\x1b[37;40my"
	if !strings.Contains(view, want) {
		t.Fatalf("view does not contain %q (styled span must cover the whole é cluster, not split the combining mark): %q", want, view)
	}
}

// TestRenderEmojiZWJHighlightNeverSplit verifies that an emoji ZWJ
// sequence occupies its measured cell width and is never split by a
// highlight boundary (Issue #39). "a👩‍💻b": the emoji cluster occupies
// cells [1, 3); the styled span must be the entire sequence, never a
// partial run of it.
func TestRenderEmojiZWJHighlightNeverSplit(t *testing.T) {
	raw := "a👩‍💻b\n"
	// Submatch on the ZWJ character between the two emoji expands to
	// the whole cluster: display cells [1, 3).
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"‍", 5, 8}),
	)
	line := makeExpandedBufLine(1, raw, [2]int{1, 3})
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	want := "\x1b[30;47;4m👩‍💻\x1b[37;40mb"
	if !strings.Contains(view, want) {
		t.Fatalf("view does not contain %q (ZWJ sequence must be styled as one cluster): %q", want, view)
	}
	// The ZWJ sequence must never appear partially styled: a styled
	// span ending mid-sequence would leave "💻" unstyled after the
	// match restore.
	if strings.Contains(view, "\x1b[37;40m💻") {
		t.Fatalf("view contains a partial ZWJ sequence styled span: %q", view)
	}
}

// TestRenderEmojiZWJClipBoundaryNeverSplits verifies that a clip
// boundary landing inside an emoji ZWJ sequence blanks the cluster's
// visible cells rather than emitting part of the sequence (Issue #39,
// run-off-edge mode).
func TestRenderEmojiZWJClipBoundaryNeverSplits(t *testing.T) {
	prefix := strings.Repeat("a", 50)
	suffix := strings.Repeat("a", 100)
	raw := prefix + "👩‍💻" + suffix + "\n"
	// The emoji cluster occupies cells [50, 52). The ZWJ character is
	// at bytes [54, 57). Pan right by 51 so the clip boundary lands
	// inside the cluster.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"‍", 54, 57}),
	)
	line := makeExpandedBufLine(1, raw, [2]int{50, 52})
	buf := makeBuf([]filebuffer.Line{line}, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 51)
	view := viewContent(m)
	// No part of the ZWJ sequence may be emitted: the split cluster's
	// visible cells render as blanks.
	if strings.Contains(view, "👩") || strings.Contains(view, "💻") {
		t.Fatalf("view contains part of a clipped ZWJ sequence (clip boundary split the cluster): %q", view)
	}
}

// TestFileListWidePathPaddingCells verifies that file-list entry
// padding uses grapheme/cell widths: a two-cell CJK path segment must
// pad to the same list column as ASCII entries, so the content panel
// begins at the same cell offset on every row (Issue #39).
func TestFileListWidePathPaddingCells(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/b.go", "bbb\n", 1, subSpec{"bbb", 0, 3}),
		textMatch("src/中.go", "zzz\n", 1, subSpec{"zzz", 0, 3}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "bbb")}, 1, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	listW := m.ListWidth()
	// "src/中.go" measures 9 cells under the grapheme policy, so the
	// list column is min(9+2, 32, 80-13) = 11.
	if listW != 11 {
		t.Fatalf("ListWidth = %d, want 11 (longest path 9 cells + 2)", listW)
	}
	lines := strings.Split(stripANSI(view), "\n")
	// Row 0 pairs the "src/b.go" entry with the filename rule row;
	// row 1 pairs the "src/中.go" entry with the first content row.
	anchors := []string{"── src/b.go", "1  bbb"}
	for row, anchor := range anchors {
		i := strings.Index(lines[row], anchor)
		if i < 0 {
			t.Fatalf("row %d does not contain panel anchor %q: %q", row, anchor, lines[row])
		}
		if w := graphemeCellWidth(lines[row][:i]); w != listW+1 {
			t.Fatalf("row %d panel starts at cell %d, want %d (list entry padding must use cell widths): %q", row, w, listW+1, lines[row])
		}
	}
}

// TestFilenameRowWidePathFitsPanel verifies that the filename row fits
// a wide path to the panel width on grapheme boundaries: the row never
// exceeds the panel and truncation keeps whole clusters (Issue #39).
func TestFilenameRowWidePathFitsPanel(t *testing.T) {
	path := strings.Repeat("中", 8) + ".go" // 19 cells
	idx := buildIndex(t, "/work",
		textMatch(path, "x\n", 1, subSpec{"x", 0, 1}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "x")}, 1, 3)
	m := setupBrowseWithSize(t, idx, buf, 30, 24)
	view := viewContent(m)
	listW := m.ListWidth()
	panelW := 30 - listW - 1
	lines := strings.Split(stripANSI(view), "\n")
	i := strings.Index(lines[0], "──")
	if i < 0 {
		t.Fatalf("row 0 has no filename rule: %q", lines[0])
	}
	row := lines[0][i:]
	if w := graphemeCellWidth(row); w > panelW {
		t.Fatalf("filename row width = %d, want <= %d (panel width): %q", w, panelW, row)
	}
	// The path is longer than the panel allows, so it is
	// left-truncated with … on a cluster boundary.
	if !strings.Contains(row, "…") || !strings.Contains(row, ".go") {
		t.Fatalf("filename row not left-truncated on a cluster boundary: %q", row)
	}
}

// TestPopupWidePathTruncatedToCells verifies that the file-change
// pop-up truncates a wide path to the terminal width in cells: a
// two-cell path must be measured by cells, not runes, so the pop-up
// never overflows the terminal (Issue #39).
func TestPopupWidePathTruncatedToCells(t *testing.T) {
	longPath := strings.Repeat("中", 15) + ".go" // 33 cells
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch(longPath, "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		longPath:   makeBuf([]filebuffer.Line{ml(1, "content-long")}, 1, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufs[string(path)], nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 20, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}
	view := viewContent(m)
	lines := strings.Split(view, "\n")
	if len(lines) <= 11 {
		t.Fatalf("view has only %d lines, need at least 12", len(lines))
	}
	popupLine := stripANSI(lines[11])
	if !strings.HasPrefix(strings.TrimLeft(popupLine, " "), "…") {
		t.Fatalf("popup line not left-truncated with …: %q", popupLine)
	}
	if w := graphemeCellWidth(popupLine); w > 20 {
		t.Fatalf("popup line width = %d cells, want <= 20 (terminal width): %q", w, popupLine)
	}
	if !strings.HasSuffix(strings.TrimRight(popupLine, " "), ".go") {
		t.Fatalf("popup line does not keep the .go suffix: %q", popupLine)
	}
}

// TestPopupWidePathCentredInCells verifies that the file-change pop-up
// centres a wide path by measured cell width, not rune count (Issue
// #39). "中.go" is 5 cells; at width 80 the left pad must be
// (80-5)/2 = 37 spaces.
func TestPopupWidePathCentredInCells(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("中.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		"中.go":     makeBuf([]filebuffer.Line{ml(1, "content-cjk")}, 1, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufs[string(path)], nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}
	view := viewContent(m)
	lines := strings.Split(view, "\n")
	popupLine := stripANSI(lines[11])
	i := strings.Index(popupLine, "中.go")
	if i < 0 {
		t.Fatalf("popup line does not contain the wide path: %q", popupLine)
	}
	if got := len(popupLine[:i]); got != 37 {
		t.Fatalf("popup leading pad = %d cells, want 37 = (80-5)/2 (centering must use cell width): %q", got, popupLine)
	}
}

// TestPopupCombiningPathNeverSplitsCluster verifies that pop-up
// left-truncation never starts the kept text mid-cluster: an escaped
// path may contain combining marks (EscapePath preserves printable
// runes), and truncation must keep the whole base-plus-combining
// cluster or drop it entirely (Issue #39).
func TestPopupCombiningPathNeverSplitsCluster(t *testing.T) {
	// "src/ddé" + "x"*15 + ".go": 26 runes but only 25 cells (the
	// é cluster is one cell). At width 20 the pop-up keeps the
	// trailing 19 cells; a rune-count cut would start the kept text on
	// the bare combining mark.
	longPath := "src/ddé" + strings.Repeat("x", 15) + ".go"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch(longPath, "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3),
		longPath:   makeBuf([]filebuffer.Line{ml(1, "content-long")}, 1, 3),
	}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufs[string(path)], nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 20, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatal("pop-up not open after cross-file n")
	}
	view := viewContent(m)
	lines := strings.Split(view, "\n")
	popupLine := stripANSI(lines[11])
	if !strings.Contains(popupLine, "́") {
		t.Fatalf("popup line dropped the combining path entirely: %q", popupLine)
	}
	// Every combining mark in the pop-up line must still be attached
	// to its base character.
	if got, want := strings.Count(popupLine, "́"), strings.Count(popupLine, "é"); got != want {
		t.Fatalf("popup line contains a bare combining mark (truncation split the é cluster): %q", popupLine)
	}
	if w := graphemeCellWidth(popupLine); w > 20 {
		t.Fatalf("popup line width = %d cells, want <= 20: %q", w, popupLine)
	}
}

// TestOverlayWideTextWrapsAndPadsToCells verifies that overlay text
// containing two-cell characters is wrapped and padded by measured
// cells so the border rows and content rows have equal cell widths
// (Issue #39). Under rune-count geometry a CJK row renders wider than
// the border.
func TestOverlayWideTextWrapsAndPadsToCells(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return makeBuf([]filebuffer.Line{ml(1, "content")}, 1, 3), nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	// A fatal exit with wide diagnostic text opens the error overlay.
	m, _ = update(t, m, app.SearchCompleteMsg{
		Index:   idx,
		Files:   idx.Files(),
		Lines:   idx.Len(),
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  strings.Repeat("中", 70),
	})
	view := viewContent(m)
	if !strings.Contains(view, "中") {
		t.Fatalf("overlay does not contain the wide diagnostic text: %q", view)
	}
	// Every emitted overlay row must have the same cell width as the
	// top border row. The overlay is centred, so rows are identified
	// by their border characters.
	var widths []int
	for _, l := range strings.Split(stripANSI(view), "\n") {
		if !strings.Contains(l, "│") && !strings.Contains(l, "─") {
			continue
		}
		w := graphemeCellWidth(strings.TrimSpace(l))
		widths = append(widths, w)
	}
	if len(widths) < 3 {
		t.Fatalf("view does not contain a bordered overlay: %q", view)
	}
	for i, w := range widths {
		if w != widths[0] {
			t.Fatalf("overlay row %d width = %d cells, want %d (border row); rows must align under cell geometry: %q", i, w, widths[0], view)
		}
	}
}
