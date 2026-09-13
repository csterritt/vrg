package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// tabKey constructs a KeyPressMsg for the tab key.
func tabKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyTab}
}

// shiftTabKey constructs a KeyPressMsg for shift+tab.
func shiftTabKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
}

// leftKey constructs a KeyPressMsg for the left arrow key.
func leftKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyLeft}
}

// rightKey constructs a KeyPressMsg for the right arrow key.
func rightKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyRight}
}

// --- Pure layout function tests (Issue #24 AC1) ---

// TestComputeListWidthLongestPathWins verifies that the longest-path
// term (longest+2) wins when it is the smallest of the three terms
// (Issue #24 AC1).
func TestComputeListWidthLongestPathWins(t *testing.T) {
	// Short paths: longest+2 = 10. 40% of 80 = 32. 80-(3+10+0) = 67.
	// min(10, 32, 67) = 10.
	got := app.ComputeListWidth(80, 8, 3, 0, true)
	if got != 10 {
		t.Fatalf("ComputeListWidth(80, 8, 3, 0, true) = %d, want 10", got)
	}
}

// TestComputeListWidthFortyPercentCapWins verifies that the 40% cap
// term (floor(0.40 × W)) wins when it is the smallest (Issue #24 AC1).
func TestComputeListWidthFortyPercentCapWins(t *testing.T) {
	// Long paths: longest+2 = 100. 40% of 80 = 32. 80-(3+10+0) = 67.
	// min(100, 32, 67) = 32.
	got := app.ComputeListWidth(80, 98, 3, 0, true)
	if got != 32 {
		t.Fatalf("ComputeListWidth(80, 98, 3, 0, true) = %d, want 32", got)
	}
}

// TestComputeListWidthTenCellMinimumWins verifies that the ten-cell
// minimum term (W − (gutter + 10 + indicator)) wins when it is the
// smallest, leaving at least 10 text cells plus the reserved indicator
// (Issue #24 AC1).
func TestComputeListWidthTenCellMinimumWins(t *testing.T) {
	// Narrow terminal: longest+2 = 100. 40% of 40 = 16. 40-(3+10+1) = 26.
	// min(100, 16, 26) = 16. But let's make the third term win:
	// W=40, longest+2=100, 40% of 40=16, 40-(20+10+1)=9.
	// min(100, 16, 9) = 9.
	got := app.ComputeListWidth(40, 98, 20, 1, true)
	if got != 9 {
		t.Fatalf("ComputeListWidth(40, 98, 20, 1, true) = %d, want 9", got)
	}
}

// TestComputeListWidthFortyPercentFloorRounding verifies that the 40%
// cap uses floor rounding (Issue #24 AC1).
func TestComputeListWidthFortyPercentFloorRounding(t *testing.T) {
	cases := []struct {
		termWidth int
		want      int
	}{
		{80, 32},  // 0.4 * 80 = 32.0 → 32
		{81, 32},  // 0.4 * 81 = 32.4 → 32
		{82, 32},  // 0.4 * 82 = 32.8 → 32
		{85, 34},  // 0.4 * 85 = 34.0 → 34
		{100, 40}, // 0.4 * 100 = 40.0 → 40
		{77, 30},  // 0.4 * 77 = 30.8 → 30
	}
	for _, tc := range cases {
		// Make longest+2 and the ten-cell term large so the 40% cap wins.
		got := app.ComputeListWidth(tc.termWidth, 1000, 3, 0, true)
		if got != tc.want {
			t.Fatalf("ComputeListWidth(%d, 1000, 3, 0, true) = %d, want %d (40%% floor)", tc.termWidth, got, tc.want)
		}
	}
}

// TestComputeListWidthGutterGrowthReduces verifies that a larger
// gutter width reduces the list width via the ten-cell minimum term
// (Issue #24 AC1).
func TestComputeListWidthGutterGrowthReduces(t *testing.T) {
	// W=60, longest+2=100, indicator=0. 40% of 60 = 24.
	// gutter=3:  60-(3+10+0) = 47. min(100, 24, 47) = 24.
	// gutter=20: 60-(20+10+0) = 30. min(100, 24, 30) = 24.
	// gutter=26: 60-(26+10+0) = 24. min(100, 24, 24) = 24.
	// gutter=30: 60-(30+10+0) = 20. min(100, 24, 20) = 20.
	got := app.ComputeListWidth(60, 98, 3, 0, true)
	if got != 24 {
		t.Fatalf("gutter=3: ComputeListWidth = %d, want 24", got)
	}
	got = app.ComputeListWidth(60, 98, 30, 0, true)
	if got != 20 {
		t.Fatalf("gutter=30: ComputeListWidth = %d, want 20", got)
	}
}

// TestComputeListWidthZeroAllocation verifies that a computed zero
// width is returned (not negative) when the ten-cell minimum term
// yields zero (Issue #24 AC1, AC4). Example: W=20, gutter=9, indicator=1
// → W − (9+10+1) = 0.
func TestComputeListWidthZeroAllocation(t *testing.T) {
	got := app.ComputeListWidth(20, 100, 9, 1, true)
	if got != 0 {
		t.Fatalf("ComputeListWidth(20, 100, 9, 1, true) = %d, want 0", got)
	}
}

// TestComputeListWidthNeverNegative verifies that pathological
// dimensions never produce a negative width (Issue #24 AC1).
func TestComputeListWidthNeverNegative(t *testing.T) {
	cases := []struct {
		name              string
		termWidth         int
		longestPathWidth  int
		gutterWidth       int
		reservedIndicator int
	}{
		{"zero terminal", 0, 100, 9, 1},
		{"negative terminal", -5, 100, 9, 1},
		{"huge gutter", 20, 100, 100, 1},
		{"huge indicator", 20, 100, 9, 100},
		{"all huge", 10, 1000, 1000, 1000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := app.ComputeListWidth(tc.termWidth, tc.longestPathWidth, tc.gutterWidth, tc.reservedIndicator, true)
			if got < 0 {
				t.Fatalf("ComputeListWidth(%d, %d, %d, %d, true) = %d, want >= 0",
					tc.termWidth, tc.longestPathWidth, tc.gutterWidth, tc.reservedIndicator, got)
			}
		})
	}
}

// TestComputeListWidthHiddenReturnsZero verifies that when visible is
// false, the list width is zero regardless of other inputs (Issue #24
// AC3).
func TestComputeListWidthHiddenReturnsZero(t *testing.T) {
	got := app.ComputeListWidth(80, 50, 3, 0, false)
	if got != 0 {
		t.Fatalf("ComputeListWidth(80, 50, 3, 0, false) = %d, want 0", got)
	}
}

// TestComputeListWidthIndicatorAffectsThirdTerm verifies that the
// reserved indicator width participates in the ten-cell minimum term
// (Issue #24 AC1). In run-off-edge mode (indicator=1) the third term is
// one cell narrower than in wrap mode (indicator=0).
func TestComputeListWidthIndicatorAffectsThirdTerm(t *testing.T) {
	// W=40, longest+2=100, gutter=3.
	// indicator=0: 40-(3+10+0) = 27. 40% of 40 = 16. min(100, 16, 27) = 16.
	// indicator=1: 40-(3+10+1) = 26. 40% of 40 = 16. min(100, 16, 26) = 16.
	// Make the third term win: W=30, longest+2=100, gutter=3.
	// indicator=0: 30-(3+10+0) = 17. 40% of 30 = 12. min(100, 12, 17) = 12.
	// indicator=1: 30-(3+10+1) = 16. 40% of 30 = 12. min(100, 12, 16) = 12.
	// Still 12. Let's make it clearer: W=24, longest+2=100, gutter=3.
	// indicator=0: 24-(3+10+0) = 11. 40% of 24 = 9. min(100, 9, 11) = 9.
	// indicator=1: 24-(3+10+1) = 10. 40% of 24 = 9. min(100, 9, 10) = 9.
	// Still 9. Let's make the third term win:
	// W=22, longest+2=100, gutter=3.
	// indicator=0: 22-(3+10+0) = 9. 40% of 22 = 8. min(100, 8, 9) = 8.
	// indicator=1: 22-(3+10+1) = 8. 40% of 22 = 8. min(100, 8, 8) = 8.
	// Equal. Let's try: W=21, longest+2=100, gutter=3.
	// indicator=0: 21-(3+10+0) = 8. 40% of 21 = 8. min(100, 8, 8) = 8.
	// indicator=1: 21-(3+10+1) = 7. 40% of 21 = 8. min(100, 8, 7) = 7.
	got0 := app.ComputeListWidth(21, 98, 3, 0, true)
	got1 := app.ComputeListWidth(21, 98, 3, 1, true)
	if got0 != 8 {
		t.Fatalf("indicator=0: ComputeListWidth(21, 98, 3, 0, true) = %d, want 8", got0)
	}
	if got1 != 7 {
		t.Fatalf("indicator=1: ComputeListWidth(21, 98, 3, 1, true) = %d, want 7", got1)
	}
}

// --- Grapheme-safe left truncation tests (Issue #24 AC2) ---

// TestTruncateLeftGraphemeFits verifies that a string that fits within
// the width is returned unchanged (Issue #24 AC2).
func TestTruncateLeftGraphemeFits(t *testing.T) {
	got := app.TruncateLeftGrapheme("hello", 10)
	if got != "hello" {
		t.Fatalf("TruncateLeftGrapheme(\"hello\", 10) = %q, want \"hello\"", got)
	}
	got = app.TruncateLeftGrapheme("hello", 5)
	if got != "hello" {
		t.Fatalf("TruncateLeftGrapheme(\"hello\", 5) = %q, want \"hello\"", got)
	}
}

// TestTruncateLeftGraphemeTruncates verifies that a string wider than
// the width is left-truncated with a leading … (Issue #24 AC2).
func TestTruncateLeftGraphemeTruncates(t *testing.T) {
	// "hello world" is 11 cells. Truncate to 8: … + 7 trailing cells.
	got := app.TruncateLeftGrapheme("hello world", 8)
	if !strings.HasPrefix(got, "…") {
		t.Fatalf("TruncateLeftGrapheme(\"hello world\", 8) = %q, want leading …", got)
	}
	// The result should be 8 cells: 1 for … + 7 for the trailing text.
	if w := graphemeCellWidth(got); w != 8 {
		t.Fatalf("TruncateLeftGrapheme(\"hello world\", 8) width = %d, want 8", w)
	}
	// The trailing portion should be "o world" (7 cells).
	if !strings.HasSuffix(got, "o world") {
		t.Fatalf("TruncateLeftGrapheme(\"hello world\", 8) = %q, want suffix \"o world\"", got)
	}
}

// TestTruncateLeftGraphemeNarrowWidth verifies that width 1 yields just
// the … character (Issue #24 AC2).
func TestTruncateLeftGraphemeNarrowWidth(t *testing.T) {
	got := app.TruncateLeftGrapheme("hello", 1)
	if got != "…" {
		t.Fatalf("TruncateLeftGrapheme(\"hello\", 1) = %q, want \"…\"", got)
	}
}

// TestTruncateLeftGraphemeGraphemeSafe verifies that truncation does
// not split grapheme clusters: wide characters and combining marks
// are kept intact (Issue #24 AC2).
func TestTruncateLeftGraphemeGraphemeSafe(t *testing.T) {
	// "中" is a 2-cell-wide grapheme. Truncating to 3 cells should
	// give "…中" (1 + 2 = 3), not "…" + half of "中".
	got := app.TruncateLeftGrapheme("ab中", 3)
	if w := graphemeCellWidth(got); w != 3 {
		t.Fatalf("TruncateLeftGrapheme(\"ab中\", 3) width = %d, want 3", w)
	}
	if !strings.HasSuffix(got, "中") {
		t.Fatalf("TruncateLeftGrapheme(\"ab中\", 3) = %q, want suffix \"中\"", got)
	}

	// Truncating to 2 cells: "…中" would be 3 cells, too wide.
	// So the result should be "…" (1 cell) since "中" (2 cells)
	// doesn't fit in the remaining 1 cell.
	got = app.TruncateLeftGrapheme("ab中", 2)
	if w := graphemeCellWidth(got); w > 2 {
		t.Fatalf("TruncateLeftGrapheme(\"ab中\", 2) width = %d, want <= 2", w)
	}
	if !strings.HasPrefix(got, "…") {
		t.Fatalf("TruncateLeftGrapheme(\"ab中\", 2) = %q, want leading …", got)
	}

	// Combining mark: "e\u0301" (é) is one grapheme cluster of 1 cell.
	// Truncating "abc e\u0301" to 5 should keep the combining cluster
	// intact.
	combined := "abc e\u0301"
	got = app.TruncateLeftGrapheme(combined, 5)
	if w := graphemeCellWidth(got); w != 5 {
		t.Fatalf("TruncateLeftGrapheme(%q, 5) width = %d, want 5", combined, w)
	}
	// The trailing portion should end with the combining cluster.
	if !strings.HasSuffix(got, "e\u0301") {
		t.Fatalf("TruncateLeftGrapheme(%q, 5) = %q, want suffix \"e\\u0301\"", combined, got)
	}
}

// TestTruncateLeftGraphemeEmpty verifies that an empty string is
// returned unchanged (Issue #24 AC2).
func TestTruncateLeftGraphemeEmpty(t *testing.T) {
	got := app.TruncateLeftGrapheme("", 5)
	if got != "" {
		t.Fatalf("TruncateLeftGrapheme(\"\", 5) = %q, want \"\"", got)
	}
}

// graphemeCellWidth computes the terminal cell width of s using the
// shared grapheme policy.
func graphemeCellWidth(s string) int {
	clusters := app.GraphemeClustersForTest(s)
	w := 0
	for _, c := range clusters {
		w += c.Width
	}
	return w
}

// --- App toggle tests (Issue #24 AC3) ---

// TestListInitiallyShown verifies that the file list is visible at
// startup (Issue #24 AC3).
func TestListInitiallyShown(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m := setupBrowse(t, idx, buf)
	if !m.ListVisible() {
		t.Fatal("ListVisible = false at startup, want true")
	}
	if m.ListWidth() <= 0 {
		t.Fatalf("ListWidth = %d at startup, want > 0", m.ListWidth())
	}
}

// TestTabHidesList verifies that tab hides the file list (Issue #24
// AC3).
func TestTabHidesList(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m := setupBrowse(t, idx, buf)
	m = deliverLayout(t, m, nil) // ensure layout is settled
	m, _ = update(t, m, tabKey())
	if m.ListVisible() {
		t.Fatal("after tab, ListVisible = true, want false")
	}
	if m.ListWidth() != 0 {
		t.Fatalf("after tab, ListWidth = %d, want 0", m.ListWidth())
	}
}

// TestLeftHidesList verifies that the left arrow key hides the file
// list (Issue #24 AC3).
func TestLeftHidesList(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m := setupBrowse(t, idx, buf)
	m, _ = update(t, m, leftKey())
	if m.ListVisible() {
		t.Fatal("after left, ListVisible = true, want false")
	}
}

// TestShiftTabShowsList verifies that shift+tab shows the file list
// after it has been hidden (Issue #24 AC3).
func TestShiftTabShowsList(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m := setupBrowse(t, idx, buf)
	// Hide the list first.
	m, _ = update(t, m, tabKey())
	if m.ListVisible() {
		t.Fatal("after tab, ListVisible = true, want false (precondition)")
	}
	// Show it with shift+tab.
	m = deliverLayout(t, m, nil)
	m, _ = update(t, m, shiftTabKey())
	if !m.ListVisible() {
		t.Fatal("after shift+tab, ListVisible = false, want true")
	}
	if m.ListWidth() <= 0 {
		t.Fatalf("after shift+tab, ListWidth = %d, want > 0", m.ListWidth())
	}
}

// TestRightShowsList verifies that the right arrow key shows the file
// list after it has been hidden (Issue #24 AC3).
func TestRightShowsList(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m := setupBrowse(t, idx, buf)
	// Hide the list first.
	m, _ = update(t, m, tabKey())
	if m.ListVisible() {
		t.Fatal("after tab, ListVisible = true, want false (precondition)")
	}
	// Show it with right.
	m = deliverLayout(t, m, nil)
	m, _ = update(t, m, rightKey())
	if !m.ListVisible() {
		t.Fatal("after right, ListVisible = false, want true")
	}
}

// TestListToggleTriggersRelayout verifies that hiding/showing the list
// triggers a layout preparation (the text width changes) through the
// Issue #17 prepared-layout path (Issue #24 AC6).
func TestListToggleTriggersRelayout(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m := setupBrowse(t, idx, buf)
	// The initial layout key text width.
	initialKey := m.LayoutKey()
	if initialKey.TextWidth <= 0 {
		t.Fatalf("initial LayoutKey.TextWidth = %d, want > 0", initialKey.TextWidth)
	}
	// Hide the list: should trigger a relayout (text width grows).
	m, cmd := update(t, m, tabKey())
	if cmd == nil {
		t.Fatal("after tab, cmd = nil, want layout preparation command")
	}
	// Deliver the layout.
	m = deliverLayout(t, m, cmd)
	// The new text width should be wider (list is hidden, panel is wider).
	newKey := m.LayoutKey()
	if newKey.TextWidth <= initialKey.TextWidth {
		t.Fatalf("after tab, LayoutKey.TextWidth = %d, want > %d (wider)", newKey.TextWidth, initialKey.TextWidth)
	}
}

// --- Zero-width preference retention (Issue #24 AC4) ---

// TestZeroWidthListRetainsPreference verifies that a computed zero
// list width draws no cells but does not change the user's visibility
// preference (Issue #24 AC4). With W=20, gutter=9, indicator=1, the
// formula gives 0, but the preference stays "visible".
func TestZeroWidthListRetainsPreference(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	// Gutter width 9 (e.g. 7-digit line numbers + 2 spaces).
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 9)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	// 20-column terminal, 3 rows. Run-off-edge mode (indicator=1).
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 20, Height: 3})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	// Toggle to run-off-edge mode so the reserved indicator is 1.
	m, cmd = update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)

	// The list preference should still be "visible" (initially shown).
	if !m.ListVisible() {
		t.Fatal("ListVisible = false, want true (preference retained through zero-width)")
	}
	// The computed list width should be 0.
	if m.ListWidth() != 0 {
		t.Fatalf("ListWidth = %d, want 0 (zero-width allocation)", m.ListWidth())
	}
}

// --- Auto-scroll: list keeps active entry visible (Issue #24 AC5) ---

// TestListAutoScrollActiveEntry verifies that the file list scrolls to
// keep the active (current) entry visible when navigation moves to a
// file outside the visible list rows (Issue #24 AC5).
func TestListAutoScrollActiveEntry(t *testing.T) {
	// Build an index with many files so the list exceeds the terminal
	// height. Navigate to a file near the bottom and verify the list
	// offset includes it. Use zero-padded names so they sort
	// lexicographically in the same order as numeric order.
	records := make([]string, 30)
	for i := 0; i < 30; i++ {
		path := "src/file" + zeroPad2(i) + ".go"
		records[i] = textMatch(path, "hello\n", 1, subSpec{"hello", 0, 5})
	}
	idx := buildIndex(t, "/work", records...)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m := setupBrowse(t, idx, buf)

	// The list should start at offset 0 (first file visible).
	if m.ListOffset() != 0 {
		t.Fatalf("initial ListOffset = %d, want 0", m.ListOffset())
	}

	// Navigate forward to file 25 (0-based). The list should scroll
	// to include it.
	var cmd tea.Cmd
	for i := 0; i < 25; i++ {
		m, cmd = update(t, m, keyPress('n'))
		m = deliverLayout(t, m, cmd)
	}
	// The current file should be file 25.
	currentFile := m.CurrentPath()
	if !strings.Contains(string(currentFile), "file25") {
		t.Fatalf("after 25 n presses, CurrentPath = %q, want file25", currentFile)
	}
	// The list offset should be > 0 (scrolled to include file 25).
	if m.ListOffset() <= 0 {
		t.Fatalf("after navigating to file 25, ListOffset = %d, want > 0 (scrolled)", m.ListOffset())
	}
	// The current file index (25) should be within the visible window.
	offset := m.ListOffset()
	visibleRows := 24 // terminal height
	if 25 < offset || 25 >= offset+visibleRows {
		t.Fatalf("file 25 not visible: offset=%d, visibleRows=%d", offset, visibleRows)
	}
}

// --- Anchor preservation through relayout (Issue #24 AC6) ---

// TestAnchorSurvivesTabShiftTab verifies that the logical reading
// anchor is preserved when the list is hidden and then shown again
// (Issue #24 AC6). The top of the viewport, mid-way through a wrapped
// line, should contain the same text location after a tab/shift+tab
// round trip.
func TestAnchorSurvivesTabShiftTab(t *testing.T) {
	// A file with a long line that wraps into many rows. Scroll
	// partway into the wrapped line, then hide and show the list.
	// The anchor (source line, column) should be preserved.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", strings.Repeat("x", 1500)+"\n", 1, subSpec{"x", 0, 1}),
	)
	lines := []filebuffer.Line{
		ml(1, strings.Repeat("x", 1500)),
		ml(2, "short"),
	}
	buf := makeBuf(lines, 2, 3)
	m := setupBrowse(t, idx, buf)

	// Scroll down a few rows into the wrapped line.
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	// Record the anchor.
	anchor := m.ViewportAnchor()
	if anchor.LineIndex != 0 {
		t.Fatalf("after scrolling, anchor.LineIndex = %d, want 0 (still in first line)", anchor.LineIndex)
	}
	if anchor.Column <= 0 {
		t.Fatalf("after scrolling, anchor.Column = %d, want > 0 (mid-way through wrapped line)", anchor.Column)
	}
	offsetBefore := m.ViewportOffset()

	// Hide the list (tab). The text width grows, so the line rewraps
	// with fewer rows. The anchor should be preserved.
	m, cmd := update(t, m, tabKey())
	m = deliverLayout(t, m, cmd)
	anchorAfterHide := m.ViewportAnchor()
	if anchorAfterHide.LineIndex != anchor.LineIndex || anchorAfterHide.Column != anchor.Column {
		t.Fatalf("after tab, anchor = (%d, %d), want (%d, %d) (preserved)",
			anchorAfterHide.LineIndex, anchorAfterHide.Column, anchor.LineIndex, anchor.Column)
	}

	// Show the list (shift+tab). The text width shrinks back. The
	// anchor should still be preserved.
	m, cmd = update(t, m, shiftTabKey())
	m = deliverLayout(t, m, cmd)
	anchorAfterShow := m.ViewportAnchor()
	if anchorAfterShow.LineIndex != anchor.LineIndex || anchorAfterShow.Column != anchor.Column {
		t.Fatalf("after shift+tab, anchor = (%d, %d), want (%d, %d) (preserved)",
			anchorAfterShow.LineIndex, anchorAfterShow.Column, anchor.LineIndex, anchor.Column)
	}
	// The viewport offset may differ (different wrapping), but the
	// top row should contain the same text location.
	_ = offsetBefore
}

// TestAnchorSurvivesResize verifies that a terminal resize that
// changes the list width (and thus the text width) preserves the
// logical reading anchor through the relayout (Issue #24 AC6). The
// anchor is a width-independent (source line, column) position.
func TestAnchorSurvivesResize(t *testing.T) {
	// A file with a long line that wraps into many rows. Scroll
	// partway into the wrapped line, then resize the terminal. The
	// list width changes (40% cap changes), so the text width
	// changes, triggering a relayout. The anchor should be preserved.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", strings.Repeat("x", 1500)+"\n", 1, subSpec{"x", 0, 1}),
	)
	lines := []filebuffer.Line{
		ml(1, strings.Repeat("x", 1500)),
		ml(2, "short"),
	}
	buf := makeBuf(lines, 2, 3)
	m := setupBrowse(t, idx, buf)

	// Scroll down a few rows into the wrapped line.
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	anchor := m.ViewportAnchor()
	if anchor.LineIndex != 0 {
		t.Fatalf("after scrolling, anchor.LineIndex = %d, want 0 (still in first line)", anchor.LineIndex)
	}
	if anchor.Column <= 0 {
		t.Fatalf("after scrolling, anchor.Column = %d, want > 0 (mid-way through wrapped line)", anchor.Column)
	}

	// Resize the terminal from 80 to 100 columns. The list width
	// changes (40% cap goes from 32 to 40), so the text width changes,
	// triggering a relayout. The anchor should be preserved.
	m, cmd := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m = deliverLayout(t, m, cmd)
	anchorAfterResize := m.ViewportAnchor()
	if anchorAfterResize.LineIndex != anchor.LineIndex || anchorAfterResize.Column != anchor.Column {
		t.Fatalf("after resize, anchor = (%d, %d), want (%d, %d) (preserved)",
			anchorAfterResize.LineIndex, anchorAfterResize.Column, anchor.LineIndex, anchor.Column)
	}

	// Resize back to 80 columns. The anchor should still be preserved.
	m, cmd = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = deliverLayout(t, m, cmd)
	anchorAfterRestore := m.ViewportAnchor()
	if anchorAfterRestore.LineIndex != anchor.LineIndex || anchorAfterRestore.Column != anchor.Column {
		t.Fatalf("after restore, anchor = (%d, %d), want (%d, %d) (preserved)",
			anchorAfterRestore.LineIndex, anchorAfterRestore.Column, anchor.LineIndex, anchor.Column)
	}
}

// --- Render-cost guard: visible entries only (Issue #24 AC7) ---

// TestRenderCostGuardFileListVisibleEntries verifies that a frame render
// queries the file-list provider only for the visible range (terminal
// height), not all file entries (Issue #24 AC7). This re-verifies the
// Issue #17 guard with the Issue #24 layout in place.
func TestRenderCostGuardFileListVisibleEntries(t *testing.T) {
	records := make([]string, 50)
	paths := make([][]byte, 50)
	for i := 0; i < 50; i++ {
		path := "src/file" + itoa(i) + ".go"
		records[i] = textMatch(path, "hello\n", 1, subSpec{"hello", 0, 5})
		paths[i] = []byte(path)
	}
	idx := buildIndex(t, "/work", records...)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)

	counter := &countingFileList{paths: paths}
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithFileListProvider(counter),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, _ = update(t, m, lc)
		}
	}

	// Reset the counter and render the view.
	counter.queries = nil
	_ = viewContent(m)

	// Verify only the visible range was queried. The terminal
	// height is 24, so at most 24 file-list entries should be
	// queried, not all 50.
	if len(counter.queries) > 24 {
		t.Fatalf("file-list queries = %d, want <= 24 (visible range only)", len(counter.queries))
	}
}

// --- Filename-row status slot (Issue #24 AC8) ---

// TestFilenameRowStatusNote verifies that a synthetic status string
// appears in the filename row when a status note function is set
// (Issue #24 AC8).
func TestFilenameRowStatusNote(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithStatusNote(func() string { return "[test status]" }),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	view := viewContent(m)
	if !strings.Contains(view, "[test status]") {
		t.Fatalf("View does not contain status note '[test status]':\n%s", view)
	}
}

// TestFilenameRowPathTruncationForStatus verifies that the path is
// truncated to make room for the status note when the panel is narrow
// (Issue #24 AC8).
func TestFilenameRowPathTruncationForStatus(t *testing.T) {
	// Use a long path and a narrow terminal so the path must be
	// truncated to make room for the status note.
	longPath := "src/very/long/path/to/a/file/that/exceeds/the/panel/width.go"
	idx := buildIndex(t, "/work",
		textMatch(longPath, "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithStatusNote(func() string { return "[status]" }),
	)
	// Narrow terminal: 40 columns. The list width is small, so the
	// panel is narrow and the long path must be truncated.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	view := viewContent(m)
	// The status note should be present.
	if !strings.Contains(view, "[status]") {
		t.Fatalf("View does not contain status note '[status]':\n%s", view)
	}
	// The full long path should NOT be present (it was truncated).
	if strings.Contains(view, longPath) {
		t.Fatalf("View contains the full long path (not truncated):\n%s", view)
	}
	// The truncated path should start with … (left-truncated).
	// The filename row is the first line of the content panel.
	lines := strings.Split(view, "\n")
	for _, line := range lines {
		if strings.Contains(line, "[status]") {
			if !strings.Contains(line, "…") {
				t.Fatalf("Filename row with status does not contain … (path not truncated):\n%s", line)
			}
			break
		}
	}
}

// TestFilenameRowNoStatusNote verifies that when no status note is
// set, the filename row shows the path without truncation for a
// status slot (Issue #24 AC8).
func TestFilenameRowNoStatusNote(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	// The path should be present in the filename row.
	if !strings.Contains(view, "a.go") {
		t.Fatalf("View does not contain 'a.go':\n%s", view)
	}
}

// --- List width recomputation after gutter growth (Issue #24 AC1) ---

// TestListWidthRecomputedAfterGutterGrowth verifies that the list width
// is recomputed after loading changes the gutter width (Issue #24
// AC1). A file with 5-digit line numbers has a wider gutter, which
// narrows the list.
func TestListWidthRecomputedAfterGutterGrowth(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	// First load with a small gutter (3).
	bufSmall := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufSmall, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	m = deliverLoad(t, m, cmd)
	widthSmall := m.ListWidth()

	// Now reload with a large gutter (7 = 5 digits + 2 spaces).
	bufLarge := makeBuf([]filebuffer.Line{ml(1, "hello")}, 99999, 7)
	loader2 := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return bufLarge, nil
	}
	m2 := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader2),
	)
	m2, _ = update(t, m2, tea.WindowSizeMsg{Width: 80, Height: 24})
	m2, cmd2 := update(t, m2, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	m2 = deliverLoad(t, m2, cmd2)
	widthLarge := m2.ListWidth()

	// The list width with a larger gutter should be <= the width
	// with a smaller gutter (the ten-cell minimum term is smaller).
	if widthLarge > widthSmall {
		t.Fatalf("ListWidth with large gutter = %d, want <= %d (small gutter)", widthLarge, widthSmall)
	}
}

// --- List width in the rendered view (Issue #24 AC1) ---

// TestListWidthInRender verifies that the rendered file list is at most
// 32 columns in an 80-column terminal with long paths (Issue #24 AC1,
// manual verification basis). The 40% cap gives floor(0.4*80) = 32.
func TestListWidthInRender(t *testing.T) {
	// Long paths so the 40% cap wins.
	longPath := "src/very/long/path/to/a/file/with/many/components.go"
	idx := buildIndex(t, "/work",
		textMatch(longPath, "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m := setupBrowse(t, idx, buf)
	// The list width should be at most 32 (40% of 80).
	if m.ListWidth() > 32 {
		t.Fatalf("ListWidth = %d, want <= 32 (40%% of 80)", m.ListWidth())
	}
}

// --- List entry truncation in render (Issue #24 AC2) ---

// TestListEntryTruncation verifies that a path wider than the list
// width is left-truncated with a leading … in the rendered view
// (Issue #24 AC2).
func TestListEntryTruncation(t *testing.T) {
	// Use a very long path so it exceeds the list width.
	longPath := "src/very/long/path/to/a/file/that/is/much/longer/than/the/list/width/allows.go"
	idx := buildIndex(t, "/work",
		textMatch(longPath, "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello")}, 1, 3)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	// The full path should not be present (it was truncated).
	if strings.Contains(view, longPath) {
		t.Fatalf("View contains the full long path (not truncated):\n%s", view)
	}
	// The truncated path should contain … and the basename.
	if !strings.Contains(view, "…") {
		t.Fatalf("View does not contain … (path not left-truncated):\n%s", view)
	}
	// The basename should be visible (left-truncation keeps the end).
	if !strings.Contains(view, "allows.go") {
		t.Fatalf("View does not contain basename 'allows.go':\n%s", view)
	}
}

// zeroPad2 formats n as a zero-padded two-digit string (00–99).
func zeroPad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}
