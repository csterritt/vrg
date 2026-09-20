package viewport

import (
	"sort"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// Source is the prepared per-line content a row model lays out:
// display cells, grapheme-cluster boundaries and cell widths, highlight
// spans, and the stop→cell target mapping. *filebuffer.Buffer
// implements it; tests may substitute a fake. The row model consumes
// the source's cluster policy rather than re-deriving segmentation or
// widths itself.
type Source interface {
	// LineCount is the number of source lines.
	LineCount() int
	// GutterWidth is the line-number gutter width in cells.
	GutterWidth() int
	// Cells returns the display cells of 0-based source line i.
	Cells(i int) []safepresentation.Cell
	// Clusters returns the grapheme clusters of source line i: each
	// cluster's half-open cell range and terminal cell width.
	Clusters(i int) []safepresentation.Cluster
	// Highlights returns the inverse-video cell spans of source line i.
	Highlights(i int) []filebuffer.Span
	// TargetCell returns the stop's clamped 0-based source line and the
	// index of its display-target cell on that line.
	TargetCell(stop searchindex.Stop) (line, cell int)
}

// RowModelKey identifies a prepared row model: the file's raw path, the
// content revision the rows were built from, the text width, and the
// wrap mode. A model is a swappable value — when the current parameters
// differ from a prepared model's key the model is stale and must be
// rebuilt; Issue 17 prepares replacements off the update path and
// discards obsolete completions on this key.
type RowModelKey struct {
	Path      string
	Revision  int
	TextWidth int
	Wrap      bool
}

// row is one rendered row: the half-open display-cell range of one
// source line it presents. In run-off-edge mode the range is the whole
// line; in wrap mode it is the clusters that fit the row.
type row struct {
	line       int
	start, end int
}

// RowModel is the prepared source-line → rendered-row mapping for one
// RowModelKey. Building it scans each line's clusters once — at load,
// toggle, or resize time — so a frame render reads row boundaries
// directly and queries the source only for the lines it paints.
type RowModel struct {
	key   RowModelKey
	src   Source
	rows  []row
	first []int // each source line's first row, plus len(rows) at the end
}

// NewRowModel builds the row model for key over src. In wrap mode each
// source line yields enough rows to hold its clusters at the key's text
// width, breaking only at cluster boundaries — a cluster wider than a
// row's remaining cells moves to the next row leaving the rest blank,
// and a cluster wider than the whole text width overflows on a row of
// its own for the renderer to clip. In run-off-edge mode each source
// line is exactly one row.
func NewRowModel(key RowModelKey, src Source) *RowModel {
	m := &RowModel{key: key, src: src}
	lines := src.LineCount()
	m.first = make([]int, 0, lines+1)
	for i := 0; i < lines; i++ {
		m.first = append(m.first, len(m.rows))
		clusters := src.Clusters(i)
		end := 0
		if n := len(clusters); n > 0 {
			end = clusters[n-1].End
		}
		if !key.Wrap || key.TextWidth <= 0 {
			m.rows = append(m.rows, row{line: i, end: end})
			continue
		}
		start, width := 0, 0
		for _, c := range clusters {
			if width > 0 && width+c.Width > key.TextWidth {
				m.rows = append(m.rows, row{line: i, start: start, end: c.Start})
				start, width = c.Start, 0
			}
			width += c.Width
		}
		m.rows = append(m.rows, row{line: i, start: start, end: end})
	}
	m.first = append(m.first, len(m.rows))
	return m
}

// Key returns the parameters the model was built for.
func (m *RowModel) Key() RowModelKey { return m.key }

// LineCount is the number of rendered rows — the scrollable extent.
func (m *RowModel) LineCount() int { return len(m.rows) }

// GutterWidth is the line-number gutter width in cells.
func (m *RowModel) GutterWidth() int { return m.src.GutterWidth() }

// RowLine returns the 0-based source line rendered row i presents and
// whether i is the line's first row; continuation rows report false so
// their gutter stays blank and their text aligns with the first row's.
func (m *RowModel) RowLine(i int) (line int, first bool) {
	r := m.rows[i]
	return r.line, r.start == 0
}

// Cells returns the display cells of rendered row i: the subslice of
// its source line's cells the row presents, or nil out of range.
func (m *RowModel) Cells(i int) []safepresentation.Cell {
	if i < 0 || i >= len(m.rows) {
		return nil
	}
	r := m.rows[i]
	return m.src.Cells(r.line)[r.start:r.end]
}

// Highlights returns rendered row i's inverse-video spans in row-local
// cell indexes: the source line's spans clipped to the row's cell range
// and rebased to its first cell.
func (m *RowModel) Highlights(i int) []filebuffer.Span {
	if i < 0 || i >= len(m.rows) {
		return nil
	}
	r := m.rows[i]
	var out []filebuffer.Span
	for _, s := range m.src.Highlights(r.line) {
		lo, hi := s.Start, s.End
		if lo < r.start {
			lo = r.start
		}
		if hi > r.end {
			hi = r.end
		}
		if lo < hi {
			out = append(out, filebuffer.Span{Start: lo - r.start, End: hi - r.start})
		}
	}
	return out
}

// TargetRow returns the rendered row holding the stop's display target:
// the row of its source line that contains the first coverage range's
// start cell, so a match far down a wrapped line lands on its own row.
func (m *RowModel) TargetRow(stop searchindex.Stop) int {
	line, cell := m.src.TargetCell(stop)
	return m.rowOf(line, cell)
}

// rowOf returns the rendered row of source line that contains display
// cell — the last of the line's rows starting at or before the cell.
// The line is clamped into the model's extent so a location past the
// end of the file lands on the last row.
func (m *RowModel) rowOf(line, cell int) int {
	if len(m.rows) == 0 {
		return 0
	}
	if last := len(m.first) - 2; line > last {
		line = last
	}
	if line < 0 {
		line = 0
	}
	lo, hi := m.first[line], m.first[line+1]
	// The last of the line's rows starting at or before the cell: the
	// row preceding the first one that starts after it.
	return lo + sort.Search(hi-lo, func(i int) bool {
		return m.rows[lo+i].start > cell
	}) - 1
}

// location returns the logical location where rendered row i begins —
// its source line and the display column of the row's first cell — or
// the zero location for a row outside the model.
func (m *RowModel) location(i int) Location {
	if i < 0 || i >= len(m.rows) {
		return Location{}
	}
	r := m.rows[i]
	return Location{Line: r.line, Col: r.start}
}

// TextWidth is the file panel's text width in cells: the panel width
// minus the gutter minus the reserved right-indicator column — zero in
// wrap mode, one in run-off-edge mode. The column is reserved now and
// populated by Issue 20's hidden-content indicators.
func TextWidth(panelWidth, gutterWidth int, wrap bool) int {
	w := panelWidth - gutterWidth
	if !wrap {
		w--
	}
	if w < 0 {
		w = 0
	}
	return w
}
