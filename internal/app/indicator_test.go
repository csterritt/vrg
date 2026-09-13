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

// setupBrowseIndicatorMode creates a browse model with the no-style theme
// (so indicator characters appear without ANSI sequences) in the given wrap
// mode. The terminal is 80x24; in run-off-edge mode the text width is
// 80 - gutterWidth - 1 (reserved indicator). The file load completes
// immediately.
func setupBrowseIndicatorMode(t *testing.T, idx *searchindex.Index, buf *filebuffer.Buffer, mode viewport.WrapMode) app.Model {
	t.Helper()
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithWrapMode(mode),
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	return m
}

// panRight pans the model right by the given number of columns using the
// '>' (10 columns) and '.' (1 column) pan keys.
func panRight(t *testing.T, m app.Model, columns int) app.Model {
	t.Helper()
	for columns >= 10 {
		m, _ = update(t, m, keyPress('>'))
		columns -= 10
	}
	for columns > 0 {
		m, _ = update(t, m, keyPress('.'))
		columns--
	}
	return m
}

// firstContentRow returns the first content row from the view, skipping the
// filename rule. With the no-style theme, the row includes the list-entry
// prefix (20 chars + 1 space) followed by the panel line.
func firstContentRow(view string) string {
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		return ""
	}
	return lines[1]
}

// anyContentRowEndsWith reports whether any content row (skipping the
// filename rule) ends with the given suffix.
func anyContentRowEndsWith(view, suffix string) bool {
	for _, row := range contentRows(view) {
		if strings.HasSuffix(row, suffix) {
			return true
		}
	}
	return false
}

// contentRows returns the content rows from the view, skipping the filename
// rule.
func contentRows(view string) []string {
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		return nil
	}
	return lines[1:]
}

// --- Left gutter indicator: `_` vs `*` (Issue #20) ---

// TestIndicatorLeftUnderscoreHiddenText verifies that the first trailing
// gutter space shows `_` when text is hidden left but no match is entirely
// hidden left.
func TestIndicatorLeftUnderscoreHiddenText(t *testing.T) {
	lineText := makeLongASCII(300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"h", 0, 1}),
	)
	lines := []filebuffer.Line{
		ml(1, lineText, [2]int{60, 61}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 50)
	view := viewContent(m)
	if !strings.Contains(view, "1_ ") {
		t.Fatalf("view does not contain left '_' indicator '1_ ': %q", view)
	}
	if strings.Contains(view, "1* ") {
		t.Fatalf("view contains left '*' (should be '_' for hidden text without hidden match): %q", view)
	}
}

// TestIndicatorLeftStarHiddenMatch verifies that the first trailing gutter
// space shows `*` when a match is entirely hidden left.
func TestIndicatorLeftStarHiddenMatch(t *testing.T) {
	lineText := makeLongASCII(300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"h", 0, 1}),
	)
	lines := []filebuffer.Line{
		ml(1, lineText, [2]int{5, 6}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 50)
	view := viewContent(m)
	if !strings.Contains(view, "1* ") {
		t.Fatalf("view does not contain left '*' indicator '1* ' (match entirely hidden left): %q", view)
	}
}

// --- Current-line-only right marker (Issue #20) ---

// TestIndicatorRightStarCurrentLine verifies that the reserved rightmost
// column shows `*` only on the current matched line's visible row when a
// match is entirely hidden right.
func TestIndicatorRightStarCurrentLine(t *testing.T) {
	lineText := makeLongASCII(300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"h", 0, 1}),
	)
	lines := []filebuffer.Line{
		ml(1, lineText, [2]int{200, 201}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 50)
	view := viewContent(m)
	row := firstContentRow(view)
	if !strings.HasSuffix(row, "*") {
		t.Fatalf("first content row does not end with '*' (right indicator on current line): %q", row)
	}
}

// TestIndicatorRightStarAbsentOffScreen verifies that the right indicator is
// absent when the current matched line is vertically off-screen. Other
// lines' left gutter indicators remain.
func TestIndicatorRightStarAbsentOffScreen(t *testing.T) {
	lineText := makeLongASCII(300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 50, subSpec{"h", 0, 1}),
	)
	lines := make([]filebuffer.Line, 50)
	for i := range lines {
		lines[i] = ml(i+1, lineText)
	}
	// Line 50 carries the match highlight (entirely hidden right when
	// panned), but line 50 is scrolled off-screen.
	lines[49] = ml(50, lineText, [2]int{200, 201})
	buf := makeBuf(lines, 50, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	// Startup reveal places line 50 on-screen. Scroll up so line 50 is
	// off-screen (show lines 1-23).
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgUp})
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyPgUp})
	// Pan right so the match on line 50 would be hidden right.
	m = panRight(t, m, 50)
	view := viewContent(m)
	if anyContentRowEndsWith(view, "*") {
		t.Fatalf("no content row should end with '*' (current line off-screen): %q", view)
	}
	// Visible lines still show left `_` for hidden text.
	if !strings.Contains(view, "_ ") {
		t.Fatalf("view does not contain left '_' indicator for hidden text: %q", view)
	}
}

// --- Both sides hidden (Issue #20) ---

// TestIndicatorBothStarsTogether verifies that both left `*` and right `*`
// can appear together on the same line.
func TestIndicatorBothStarsTogether(t *testing.T) {
	lineText := makeLongASCII(300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"h", 0, 1}),
	)
	lines := []filebuffer.Line{
		ml(1, lineText, [2]int{5, 6}, [2]int{200, 201}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 50)
	view := viewContent(m)
	if !strings.Contains(view, "1* ") {
		t.Fatalf("view does not contain left '*' indicator '1* ': %q", view)
	}
	row := firstContentRow(view)
	if !strings.HasSuffix(row, "*") {
		t.Fatalf("first content row does not end with '*' (right indicator): %q", row)
	}
}

// --- Partial visibility (Issue #20) ---

// TestIndicatorPartialVisibilityLeftNoStar verifies that a match partially
// visible on the left (straddling the left clip edge with a non-blank
// in-window cell) produces no left hidden-match indicator.
func TestIndicatorPartialVisibilityLeftNoStar(t *testing.T) {
	lineText := makeLongASCII(300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"h", 0, 1}),
	)
	lines := []filebuffer.Line{
		ml(1, lineText, [2]int{49, 51}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 50)
	view := viewContent(m)
	if strings.Contains(view, "1* ") {
		t.Fatalf("view contains left '*' (partially visible match should not produce hidden-match indicator): %q", view)
	}
	if !strings.Contains(view, "1_ ") {
		t.Fatalf("view does not contain left '_' (text hidden left, match partially visible): %q", view)
	}
}

// TestIndicatorPartialVisibilityRightNoStar verifies that a match partially
// visible on the right (straddling the right clip edge with a non-blank
// in-window cell) produces no right hidden-match indicator.
func TestIndicatorPartialVisibilityRightNoStar(t *testing.T) {
	lineText := makeLongASCII(300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"h", 0, 1}),
	)
	lines := []filebuffer.Line{
		ml(1, lineText, [2]int{125, 127}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 50)
	view := viewContent(m)
	row := firstContentRow(view)
	if strings.HasSuffix(row, "*") {
		t.Fatalf("first content row ends with '*' (partially visible match should not produce right hidden-match indicator): %q", row)
	}
}

// --- Last-cell match with another farther right (Issue #20) ---

// TestIndicatorLastCellMatchFarRightStar verifies that a match at the last
// visible cell (visible) with another match farther right (entirely hidden
// right) produces a right `*` on the current matched line.
func TestIndicatorLastCellMatchFarRightStar(t *testing.T) {
	lineText := makeLongASCII(300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"h", 0, 1}),
	)
	// Match 1 at cell 125 (last visible cell in [50, 126)). Match 2 at
	// cell 200 (entirely hidden right).
	lines := []filebuffer.Line{
		ml(1, lineText, [2]int{125, 126}, [2]int{200, 201}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 50)
	view := viewContent(m)
	row := firstContentRow(view)
	if !strings.HasSuffix(row, "*") {
		t.Fatalf("first content row does not end with '*' (far-right match hidden): %q", row)
	}
	if strings.Contains(view, "1* ") {
		t.Fatalf("view contains left '*' (last-cell match is visible, no left hidden match): %q", view)
	}
	if !strings.Contains(view, "1_ ") {
		t.Fatalf("view does not contain left '_' (text hidden left): %q", view)
	}
}

// --- Split-glyph blanks (Issue #20) ---

// TestIndicatorSplitGlyphBlankHiddenLeft verifies that a split wide glyph
// rendered as blanks does not count as visible, so a match on that glyph is
// considered entirely hidden left and produces a left `*`.
func TestIndicatorSplitGlyphBlankHiddenLeft(t *testing.T) {
	// CJK character (2 cells) at the start followed by ASCII. The match
	// highlight covers the CJK glyph (cells [0, 2)).
	display := "中" + makeLongASCII(298)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"h", 0, 1}),
	)
	lines := []filebuffer.Line{
		ml(1, display, [2]int{0, 2}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	// Pan right by 1: the CJK glyph (cells [0, 2)) straddles the left
	// clip edge. Cell 0 is out, cell 1 is in but rendered as a blank
	// (split glyph). The match has no non-blank visible cells.
	m = panRight(t, m, 1)
	view := viewContent(m)
	if !strings.Contains(view, "1* ") {
		t.Fatalf("view does not contain left '*' (split-glyph blank should not count as visible): %q", view)
	}
}

// --- Wrap-mode absence (Issue #20) ---

// TestIndicatorWrapModeNoIndicators verifies that wrap mode draws neither
// hidden-content indicators nor a reserved right column.
func TestIndicatorWrapModeNoIndicators(t *testing.T) {
	lineText := makeLongASCII(300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"h", 0, 1}),
	)
	lines := []filebuffer.Line{
		ml(1, lineText, [2]int{200, 201}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOn)
	view := viewContent(m)
	if strings.Contains(view, "1_ ") {
		t.Fatalf("wrap-mode view contains left '_' indicator (should be absent): %q", view)
	}
	if strings.Contains(view, "1* ") {
		t.Fatalf("wrap-mode view contains left '*' indicator (should be absent): %q", view)
	}
	if anyContentRowEndsWith(view, "*") {
		t.Fatalf("wrap-mode view contains a right '*' indicator (should be absent): %q", view)
	}
}

// --- Indicator styling (Issue #20) ---

// TestIndicatorStyling verifies that indicators use the theme's inverse
// indicator style. With the default dark theme, the indicator character is
// wrapped in the true-inverse match colour pair (black on white) and
// restored to the base colours.
func TestIndicatorStyling(t *testing.T) {
	lineText := makeLongASCII(300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"h", 0, 1}),
	)
	// Left `*` (match at cell 5 hidden left) and right `*` (match at cell
	// 200 hidden right) with the default dark theme.
	lines := []filebuffer.Line{
		ml(1, lineText, [2]int{5, 6}, [2]int{200, 201}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseRunOffEdge(t, idx, buf)
	m = panRight(t, m, 50)
	view := viewContent(m)
	// The dark-theme indicator style is matchSeq + char + baseSeq:
	// black on white, then restore to white on black.
	const indicatorSeq = "\x1b[30;47m*\x1b[37;40m"
	if !strings.Contains(view, indicatorSeq) {
		t.Fatalf("view does not contain styled '*' indicator %q: %q", indicatorSeq, view)
	}
}

// TestIndicatorStylingUnderscore verifies that the `_` indicator uses the
// theme's inverse indicator style.
func TestIndicatorStylingUnderscore(t *testing.T) {
	lineText := makeLongASCII(300)
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"h", 0, 1}),
	)
	// Left `_` (text hidden left, match at cell 60 visible) with the
	// default dark theme.
	lines := []filebuffer.Line{
		ml(1, lineText, [2]int{60, 61}),
	}
	buf := makeBuf(lines, 1, 3)
	m := setupBrowseRunOffEdge(t, idx, buf)
	m = panRight(t, m, 50)
	view := viewContent(m)
	const indicatorSeq = "\x1b[30;47m_\x1b[37;40m"
	if !strings.Contains(view, indicatorSeq) {
		t.Fatalf("view does not contain styled '_' indicator %q: %q", indicatorSeq, view)
	}
}
