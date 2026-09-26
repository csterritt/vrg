package viewport

import (
	"slices"
	"strings"
	"testing"

	"vrg/internal/present"
)

// lineSource is a prepared-line Source fake backed by real
// present.Line segmentation, so cluster boundaries and cell widths are
// genuine. It records every per-line Cells and Spans query so tests
// can prove frame rendering touches only the lines behind visible
// rows — the render-cost guard.
type lineSource struct {
	lines  []present.Line
	spans  map[int][]present.Span
	cellsQ []int
	spansQ []int
}

func (s *lineSource) LineCount() int { return len(s.lines) }

func (s *lineSource) Cells(i int) []present.Cell {
	s.cellsQ = append(s.cellsQ, i)
	return s.lines[i].Cells()
}

func (s *lineSource) Spans(i int) []present.Span {
	s.spansQ = append(s.spansQ, i)
	return s.spans[i]
}

// source builds a lineSource over text split on LF, keeping
// terminators; spans keys are 0-based source lines.
func source(t *testing.T, text string, spans map[int][]present.Span) *lineSource {
	t.Helper()
	var lines []present.Line
	for _, l := range strings.SplitAfter(text, "\n") {
		if l == "" {
			continue
		}
		lines = append(lines, present.LineOf([]byte(l)))
	}
	return &lineSource{lines: lines, spans: spans}
}

// rowText joins a rendered row's cell texts — the content a frame
// paints in the text area for that row.
func rowText(r Row) string {
	var b strings.Builder
	for _, c := range r.Cells {
		b.WriteString(c.Text)
	}
	return b.String()
}

// Wrapping is a prepared row model built once for a source and text
// width: clusters pack greedily, a two-cell cluster that cannot fit in
// a row's remaining cells moves to the next row leaving a blank, and a
// cluster wider than the whole row splits as a last resort.
func TestWrapRowModel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		text  string
		width int
		rows  []string
	}{
		{"short line is one row", "abc\n", 10, []string{"abc"}},
		{"ascii packs full rows", strings.Repeat("x", 25) + "\n", 10,
			[]string{strings.Repeat("x", 10), strings.Repeat("x", 10), strings.Repeat("x", 5)}},
		{"exact multiple adds no empty row", "1234567890abcdefghij\n", 10,
			[]string{"1234567890", "abcdefghij"}},
		{"empty line still occupies a row", "\n", 10, []string{""}},
		// 世 needs both remaining cells; it moves whole to the next
		// row and the first row ends a cell short.
		{"wide cluster at the boundary leaves a blank", "abc世x\n", 4,
			[]string{"abc", "世x"}},
		{"combining cluster stays whole", "abécd\n", 4,
			[]string{"abéc", "d"}},
		// A cluster wider than the text width cannot fit any row: it
		// fills whole rows as a last-resort split, its clipped portion
		// rendering blank.
		{"oversized cluster splits as a last resort", "a\xc2\x85b\n", 2,
			[]string{"a", " ", "", "", "b"}},
		// The tab expansion is one cluster: narrower than the row it
		// moves whole; wider, it splits like any oversized cluster.
		{"tab expansion moves whole then splits", "a\tb\n", 5,
			[]string{"a", "     ", "  b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Prepare(source(t, tc.text, nil), Key{Width: tc.width, Wrap: true})
			if m.Len() != len(tc.rows) {
				t.Fatalf("Len = %d, want %d rows", m.Len(), len(tc.rows))
			}
			for i, w := range tc.rows {
				if got := rowText(m.Row(i)); got != w {
					t.Fatalf("row %d text = %q, want %q", i, got, w)
				}
			}
		})
	}
}

// Every rendered row knows its source line and whether it continues a
// line already started above — the blank continuation gutter the frame
// paints — while scroll units remain rendered rows.
func TestWrapRowLineAndContinuation(t *testing.T) {
	src := source(t, strings.Repeat("x", 25)+"\nshort\n", nil)
	m := Prepare(src, Key{Width: 10, Wrap: true})
	want := []struct {
		line int
		cont bool
	}{
		{0, false}, {0, true}, {0, true}, {1, false},
	}
	if m.Len() != len(want) {
		t.Fatalf("Len = %d, want %d", m.Len(), len(want))
	}
	for i, w := range want {
		r := m.Row(i)
		if r.Line != w.line || r.Cont != w.cont {
			t.Fatalf("Row(%d) = line %d cont %v, want line %d cont %v",
				i, r.Line, r.Cont, w.line, w.cont)
		}
	}
}

// Run-off-edge mode is the one-row-per-line model: each row carries
// its line's full cells for the frame to clip, and a display target's
// row is its source line regardless of the cell.
func TestRunOffEdgeRowModel(t *testing.T) {
	src := source(t, strings.Repeat("x", 25)+"\nshort\n", nil)
	m := Prepare(src, Key{Width: 10, Wrap: false})
	if m.Len() != 2 {
		t.Fatalf("Len = %d, want 2", m.Len())
	}
	if got := rowText(m.Row(0)); got != strings.Repeat("x", 25) {
		t.Fatalf("row 0 text = %q, want the full line", got)
	}
	for _, cell := range []int{0, 9, 24} {
		if got := m.RowOf(Target{Line: 0, Cell: cell}); got != 0 {
			t.Fatalf("RowOf cell %d = %d, want the source line 0", cell, got)
		}
	}
}

// The row model is a swappable value keyed by what it was prepared
// for — path, content revision, text width, and wrap mode — so a
// stale model is detectable by key alone (Issue #17's contract).
func TestRowModelKey(t *testing.T) {
	src := source(t, "abc\n", nil)
	k := Key{Path: "f.txt", Rev: 2, Width: 40, Wrap: true}
	m := Prepare(src, k)
	if m.Key() != k {
		t.Fatalf("Key() = %+v, want %+v", m.Key(), k)
	}
	other := Prepare(src, Key{Path: "f.txt", Rev: 2, Width: 40, Wrap: false})
	if other.Key() == m.Key() {
		t.Fatalf("distinct preparations share a key: %+v", m.Key())
	}
}

// Highlight spans translate into each wrapped row's local cells: a
// coverage span crossing a wrap boundary splits, and a marker paints
// on the row owning its position — the row after a completely full
// wrap row for a boundary position.
func TestWrapTranslatesSpansPerRow(t *testing.T) {
	src := source(t, "abcdefgh\n", map[int][]present.Span{
		0: {{Start: 1, End: 7}},
	})
	m := Prepare(src, Key{Width: 4, Wrap: true})
	if got := m.Row(0).Spans; !slices.Equal(got, []present.Span{{Start: 1, End: 4}}) {
		t.Fatalf("row 0 spans = %+v, want [{1 4}]", got)
	}
	if got := m.Row(1).Spans; !slices.Equal(got, []present.Span{{Start: 0, End: 3}}) {
		t.Fatalf("row 1 spans = %+v, want [{0 3}]", got)
	}

	// A marker at a full row's end belongs to the next row.
	src = source(t, "abcdefgh\n", map[int][]present.Span{
		0: {{Start: 4, End: 4}},
	})
	m = Prepare(src, Key{Width: 4, Wrap: true})
	if got := m.Row(0).Spans; len(got) != 0 {
		t.Fatalf("row 0 spans = %+v, want none — the marker is on row 1", got)
	}
	if got := m.Row(1).Spans; !slices.Equal(got, []present.Span{{Start: 0, End: 0}}) {
		t.Fatalf("row 1 spans = %+v, want the marker at [{0 0}]", got)
	}
}

// An end-of-line marker extends the line by one cell: after a
// completely full final wrap row it occupies another row — a
// continuation row with no cells, painting one marker cell at column
// zero. On a non-full last row it stays on that row.
func TestEndOfLineMarkerRows(t *testing.T) {
	src := source(t, "abcd\n", map[int][]present.Span{
		0: {{Start: 4, End: 4}},
	})
	m := Prepare(src, Key{Width: 4, Wrap: true})
	if m.Len() != 2 {
		t.Fatalf("Len = %d, want 2 — the marker overflows to its own row", m.Len())
	}
	r := m.Row(1)
	if len(r.Cells) != 0 || !r.Cont || r.Line != 0 {
		t.Fatalf("marker row = %+v, want an empty continuation of line 0", r)
	}
	if !slices.Equal(r.Spans, []present.Span{{Start: 0, End: 0}}) {
		t.Fatalf("marker row spans = %+v, want [{0 0}]", r.Spans)
	}

	// Non-full last row: the marker stays at the line's end cell.
	src = source(t, "ab\n", map[int][]present.Span{
		0: {{Start: 2, End: 2}},
	})
	m = Prepare(src, Key{Width: 4, Wrap: true})
	if m.Len() != 1 {
		t.Fatalf("Len = %d, want 1", m.Len())
	}
	if got := m.Row(0).Spans; !slices.Equal(got, []present.Span{{Start: 2, End: 2}}) {
		t.Fatalf("row 0 spans = %+v, want the marker at [{2 2}]", got)
	}
}

// The Issue #14 reveal resolves a display target through the wrapped
// model: a match far down a source line taller than several screens
// lands its containing rendered row at floor(height/3).
func TestRevealFindsRowOfWrappedLine(t *testing.T) {
	src := source(t, strings.Repeat("x", 200)+"\ntail\n", nil)
	var v Viewport
	v.Resize(10, 6)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	if !v.Reveal(Target{Line: 0, Cell: 150}) {
		t.Fatal("reveal of a hidden wrapped target reported no move")
	}
	// Cell 150 sits in the line's rendered row [150,160) — row 15 —
	// and floor(6/3) = 2, so the top moves to 13.
	if v.Top() != 13 {
		t.Fatalf("top = %d, want 13 — row 15 placed at row 2", v.Top())
	}
	// A target in the tail line resolves past the wrapped rows; its
	// one-third placement wants 18 but the EOF clamp keeps 15.
	v.SetTop(0)
	if !v.Reveal(Target{Line: 1, Cell: 0}) {
		t.Fatal("reveal of the tail line reported no move")
	}
	if v.Top() != 15 {
		t.Fatalf("top = %d, want the last full page 15", v.Top())
	}
}

// A frame's row queries touch only the lines behind the visible rows:
// building the model reads every line once, but rendering queries the
// source per visible row and never re-wraps the buffer per frame.
func TestVisibleRowsNeverWrapsOffscreenLines(t *testing.T) {
	src := source(t, strings.Repeat("x\n", 1000), nil)
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	v.SetTop(8)
	src.cellsQ, src.spansQ = nil, nil

	vis := v.Visible()
	if len(vis) != 4 {
		t.Fatalf("visible = %d rows, want 4", len(vis))
	}
	want := []int{8, 9, 10, 11}
	if !slices.Equal(src.cellsQ, want) || !slices.Equal(src.spansQ, want) {
		t.Fatalf("source queries cells=%v spans=%v, want lines %v only",
			src.cellsQ, src.spansQ, want)
	}
}
