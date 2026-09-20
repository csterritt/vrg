package viewport

import (
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// fakeSource is a counting Source: every line's cells and clusters come
// from the real grapheme policy, while Cells and Clusters calls are
// recorded so tests can prove which lines a render touched.
type fakeSource struct {
	cells    [][]safepresentation.Cell
	clusters [][]safepresentation.Cluster
	spans    map[int][]filebuffer.Span

	cellsQueried    []int
	clustersQueried []int
}

// sourceOf builds a fakeSource whose lines are escaped by the real
// content policy, so cluster boundaries and widths in tests match
// production.
func sourceOf(t *testing.T, lines ...string) *fakeSource {
	t.Helper()
	src := &fakeSource{spans: make(map[int][]filebuffer.Span)}
	for _, line := range lines {
		cells, clusters := safepresentation.EscapeContent([]byte(line))
		src.cells = append(src.cells, cells)
		src.clusters = append(src.clusters, clusters)
	}
	return src
}

func (f *fakeSource) LineCount() int   { return len(f.cells) }
func (f *fakeSource) GutterWidth() int { return 3 }
func (f *fakeSource) Cells(i int) []safepresentation.Cell {
	f.cellsQueried = append(f.cellsQueried, i)
	return f.cells[i]
}
func (f *fakeSource) Clusters(i int) []safepresentation.Cluster {
	f.clustersQueried = append(f.clustersQueried, i)
	return f.clusters[i]
}
func (f *fakeSource) Highlights(i int) []filebuffer.Span { return f.spans[i] }

// TargetCell mirrors the Buffer contract: the stop's clamped 0-based
// source line and the start cell of its first coverage range.
func (f *fakeSource) TargetCell(stop searchindex.Stop) (line, cell int) {
	line = int(stop.Line) - 1
	if last := len(f.cells) - 1; line > last {
		line = last
	}
	if line < 0 || len(stop.Coverage) == 0 {
		return line, 0
	}
	cs, _ := safepresentation.Span(f.cells[line], stop.Coverage[0].Start, stop.Coverage[0].End)
	return line, cs
}

// rowText joins the display text of rendered row i.
func rowText(m *RowModel, i int) string {
	var b strings.Builder
	for _, c := range m.Cells(i) {
		b.WriteString(c.Text)
	}
	return b.String()
}

func stopAt(line int64, start, end int) searchindex.Stop {
	return searchindex.Stop{
		Line:     line,
		Coverage: []searchindex.Range{{Start: start, End: end}},
	}
}

// In wrap mode a source line occupies enough rendered rows to hold its
// grapheme clusters at the text width; run-off-edge mode gives each
// line exactly one row.
func TestWrapRowCounts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		line  string
		width int
		want  []string
	}{
		{"fits", "abc", 10, []string{"abc"}},
		{"exact fit", "abcdefghij", 10, []string{"abcdefghij"}},
		{"one over", "abcdefghijk", 10, []string{"abcdefghij", "k"}},
		{"twenty-five at ten", "abcdefghijklmnopqrstuvwxy", 10,
			[]string{"abcdefghij", "klmnopqrst", "uvwxy"}},
		{"empty line", "", 10, []string{""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewRowModel(RowModelKey{TextWidth: tc.width, Wrap: true}, sourceOf(t, tc.line))
			if m.LineCount() != len(tc.want) {
				t.Fatalf("wrap width %d of %q: LineCount = %d, want %d",
					tc.width, tc.line, m.LineCount(), len(tc.want))
			}
			for i, want := range tc.want {
				if got := rowText(m, i); got != want {
					t.Fatalf("wrap width %d of %q: row %d = %q, want %q",
						tc.width, tc.line, i, got, want)
				}
			}
		})
	}
}

// A two-cell cluster that cannot fit in a row's remaining cells moves
// to the next row, leaving the previous row short — the blank cell is
// the padding the row's narrower painted width implies.
func TestWrapMovesWideClusterToNextRow(t *testing.T) {
	m := NewRowModel(RowModelKey{TextWidth: 4, Wrap: true}, sourceOf(t, "a世界b"))
	if m.LineCount() != 2 {
		t.Fatalf("LineCount = %d, want 2", m.LineCount())
	}
	if got := rowText(m, 0); got != "a世" {
		t.Fatalf("row 0 = %q, want %q", got, "a世")
	}
	if got := rowText(m, 1); got != "界b" {
		t.Fatalf("row 1 = %q, want %q", got, "界b")
	}

	// A wide cluster that exactly fills the remaining cells stays.
	m = NewRowModel(RowModelKey{TextWidth: 4, Wrap: true}, sourceOf(t, "ab世c"))
	if m.LineCount() != 2 {
		t.Fatalf("exact-fit LineCount = %d, want 2", m.LineCount())
	}
	if got := rowText(m, 0); got != "ab世" {
		t.Fatalf("row 0 = %q, want %q", got, "ab世")
	}
}

// A cluster of a base rune plus combining marks is never split: a wrap
// boundary lands before or after the whole cluster, never between the
// base and its marks.
func TestWrapNeverSplitsCombiningCluster(t *testing.T) {
	m := NewRowModel(RowModelKey{TextWidth: 2, Wrap: true}, sourceOf(t, "xéyz"))
	want := []string{"xé", "yz"}
	if m.LineCount() != len(want) {
		t.Fatalf("LineCount = %d, want %d", m.LineCount(), len(want))
	}
	for i, w := range want {
		if got := rowText(m, i); got != w {
			t.Fatalf("row %d = %q, want %q", i, got, w)
		}
	}
}

// Tabs are structural: a tab expands to the next multiple of eight
// source-display columns as one indivisible cluster of space cells, so
// positions derive from the line's own display columns — independent of
// gutter and horizontal pan, which the row model never sees.
func TestWrapExpandsTabStops(t *testing.T) {
	m := NewRowModel(RowModelKey{TextWidth: 20, Wrap: true}, sourceOf(t, "a\tb"))
	if m.LineCount() != 1 {
		t.Fatalf("LineCount = %d, want 1", m.LineCount())
	}
	if got := rowText(m, 0); got != "a       b" {
		t.Fatalf("row 0 = %q, want %q — tab at column 1 expands to 7 spaces", got, "a       b")
	}

	// A tab cluster wider than the whole text width sits alone on its
	// row rather than splitting; the next cluster starts a fresh row.
	m = NewRowModel(RowModelKey{TextWidth: 5, Wrap: true}, sourceOf(t, "a\tb"))
	if m.LineCount() != 3 {
		t.Fatalf("narrow LineCount = %d, want 3", m.LineCount())
	}
	if got := rowText(m, 1); got != "       " {
		t.Fatalf("row 1 = %q, want the 7-cell tab cluster alone", got)
	}
	if got := rowText(m, 2); got != "b" {
		t.Fatalf("row 2 = %q, want %q", got, "b")
	}
}

// Run-off-edge mode renders each source line as exactly one row holding
// all of its cells; the clip to the text width is the renderer's, not
// the model's.
func TestRunOffEdgeRowsAreWholeLines(t *testing.T) {
	src := sourceOf(t, "short", "a very much longer line", "x")
	m := NewRowModel(RowModelKey{TextWidth: 6, Wrap: false}, src)
	if m.LineCount() != 3 {
		t.Fatalf("LineCount = %d, want 3 source lines", m.LineCount())
	}
	for i, want := range []string{"short", "a very much longer line", "x"} {
		if got := rowText(m, i); got != want {
			t.Fatalf("row %d = %q, want whole line %q", i, got, want)
		}
	}
}

// RowLine maps each rendered row back to its source line and reports
// whether the row is the line's first, so continuation rows can keep
// their gutter blank.
func TestRowLineReportsContinuation(t *testing.T) {
	src := sourceOf(t, strings.Repeat("x", 25), "done")
	m := NewRowModel(RowModelKey{TextWidth: 10, Wrap: true}, src)
	want := []struct {
		line  int
		first bool
	}{{0, true}, {0, false}, {0, false}, {1, true}}
	if m.LineCount() != len(want) {
		t.Fatalf("LineCount = %d, want %d", m.LineCount(), len(want))
	}
	for i, w := range want {
		line, first := m.RowLine(i)
		if line != w.line || first != w.first {
			t.Fatalf("RowLine(%d) = (%d, %v), want (%d, %v)", i, line, first, w.line, w.first)
		}
	}
}

// Toggling the wrap mode produces a different row model over the same
// source: the swappable value keyed by (path, revision, text width,
// wrap mode) is what changes.
func TestWrapToggleSwapsRowModel(t *testing.T) {
	src := sourceOf(t, strings.Repeat("x", 25), "short")
	key := RowModelKey{Path: "p", Revision: 1, TextWidth: 10, Wrap: true}
	wrapped := NewRowModel(key, src)
	if wrapped.Key() != key {
		t.Fatalf("Key() = %+v, want %+v", wrapped.Key(), key)
	}
	key.Wrap = false
	nowrap := NewRowModel(key, src)
	if nowrap.Key() != key {
		t.Fatalf("Key() = %+v, want %+v", nowrap.Key(), key)
	}
	if wrapped.LineCount() != 4 || nowrap.LineCount() != 2 {
		t.Fatalf("row counts wrap/nowrap = %d/%d, want 4/2",
			wrapped.LineCount(), nowrap.LineCount())
	}
}

// The Issue 14 reveal finds the rendered row of a wrapped line that
// contains the match's start cell — not merely the line's first row.
func TestTargetRowFindsWrappedMatchRow(t *testing.T) {
	src := sourceOf(t, "top", strings.Repeat("x", 60), "bot")
	m := NewRowModel(RowModelKey{TextWidth: 10, Wrap: true}, src)
	// Line 1 occupies rendered rows 1..6; the match at byte 55 lives in
	// the row covering cells [50, 60).
	if got := m.TargetRow(stopAt(2, 55, 58)); got != 6 {
		t.Fatalf("TargetRow = %d, want 6", got)
	}
	if got := m.TargetRow(stopAt(2, 0, 3)); got != 1 {
		t.Fatalf("TargetRow for line start = %d, want 1", got)
	}
	// Run-off-edge mode is one row per line again.
	m = NewRowModel(RowModelKey{TextWidth: 10, Wrap: false}, src)
	if got := m.TargetRow(stopAt(2, 55, 58)); got != 1 {
		t.Fatalf("nowrap TargetRow = %d, want 1", got)
	}
}

// A source line taller than several screens reveals a match near its
// end at zero-based row floor(h/3) of the content area; the lines
// after it keep the reveal clear of the end-of-file clamp.
func TestRevealInsideLineTallerThanScreens(t *testing.T) {
	lines := []string{strings.Repeat("x", 100)}
	for i := 0; i < 20; i++ {
		lines = append(lines, "short")
	}
	src := sourceOf(t, lines...)
	m := NewRowModel(RowModelKey{TextWidth: 10, Wrap: true}, src)
	target := m.TargetRow(stopAt(1, 95, 98))
	if target != 9 {
		t.Fatalf("TargetRow = %d, want 9", target)
	}
	v := &Viewport{}
	v.SetLayout(m, 3)
	v.Reveal(target)
	if got := v.Top(); got != 8 {
		t.Fatalf("top = %d, want 8 so the target sits at row floor(3/3)=1", got)
	}
}

// A render that paints a window of rows queries the source for those
// rows' cells only — the prepared model never re-invokes the grapheme
// wrapper for lines outside the visible range.
func TestRenderTouchesOnlyVisibleRows(t *testing.T) {
	var lines []string
	for i := 0; i < 10000; i++ {
		lines = append(lines, "line")
	}
	src := sourceOf(t, lines...)
	m := NewRowModel(RowModelKey{TextWidth: 10, Wrap: true}, src)

	src.cellsQueried = nil
	src.clustersQueried = nil
	for r := 5; r < 25; r++ {
		_ = m.Cells(r)
		_ = m.Highlights(r)
	}
	if len(src.clustersQueried) != 0 {
		t.Fatalf("render invoked the cluster wrapper for %d lines, want 0",
			len(src.clustersQueried))
	}
	if len(src.cellsQueried) != 20 {
		t.Fatalf("render queried %d rows, want the 20 visible rows", len(src.cellsQueried))
	}
	for i, q := range src.cellsQueried {
		if q != i+5 {
			t.Fatalf("query %d touched line %d, want visible line %d", i, q, i+5)
		}
	}
}

// The text width is the panel width minus the gutter minus the
// reserved right-indicator column — zero cells in wrap mode, one in
// run-off-edge mode — and never negative.
func TestTextWidth(t *testing.T) {
	for _, tc := range []struct {
		name          string
		panel, gutter int
		wrap          bool
		want          int
	}{
		{"wrap reserves nothing", 73, 3, true, 70},
		{"run-off-edge reserves one", 73, 3, false, 69},
		{"gutter fills the panel", 3, 3, false, 0},
		{"cannot go negative", 2, 3, true, 0},
	} {
		if got := TextWidth(tc.panel, tc.gutter, tc.wrap); got != tc.want {
			t.Errorf("%s: TextWidth(%d, %d, %v) = %d, want %d",
				tc.name, tc.panel, tc.gutter, tc.wrap, got, tc.want)
		}
	}
}
