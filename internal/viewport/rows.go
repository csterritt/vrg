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
	// extent and fit hold the per-line paintable-boundary data the
	// horizontal clamp is computed from: the line's content extent in
	// source-display columns and the largest column at which one of
	// its clusters starts while the cluster's width still fits the
	// text width. End-of-line marker cells (Issue 23) arrive as
	// ordinary width-one clusters, so they extend both like any cell.
	extent []int
	fit    []int
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
		// One pass over the line's clusters yields its cell-slice end,
		// its content extent in columns, and the paintable boundary:
		// the last cluster start whose width still fits the text width.
		end, col, fit := 0, 0, 0
		for _, c := range clusters {
			if c.Width <= key.TextWidth {
				fit = col
			}
			col += c.Width
			end = c.End
		}
		m.extent = append(m.extent, col)
		m.fit = append(m.fit, fit)
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

// maxOffset is the maximum valid horizontal offset for the rendered
// rows [top, top+height) — the visible-lines extent policy's
// paintable-boundary maximum. The widest visible source line sets the
// clamp: the offset may reach the largest column at which one of that
// line's clusters starts while its width still fits the text width, so
// at the maximum the line keeps one fully painted cluster. Lines tied
// for widest contribute their own fitting start, so the maximum is the
// largest qualifying boundary among them. A widest line with no
// fitting cluster — or an empty or all-blank view — clamps to 0.
// Wrap-mode models report 0: panning is a no-op in wrap mode.
func (m *RowModel) maxOffset(top, height int) int {
	if m.key.Wrap {
		return 0
	}
	widest, s := 0, 0
	for i := top; i < top+height && i < len(m.rows); i++ {
		if i < 0 {
			continue
		}
		line := m.rows[i].line
		if e := m.extent[line]; e > widest {
			widest, s = e, m.fit[line]
		} else if e == widest && m.fit[line] > s {
			s = m.fit[line]
		}
	}
	return s
}

// Clip returns rendered row i's display cells and highlight spans seen
// through the horizontal window [off, off+w) of source-display
// columns. Cells of clusters wholly outside the window are dropped; a
// cluster either window edge splits contributes one blank cell per
// in-window column instead, so a wide glyph never renders half-drawn.
// The returned spans are rebased onto the window's cell indexes and
// never mark a blank. Wrap-mode rows return the row's plain cells and
// spans: panning is a no-op in wrap mode and each row already holds
// only what fits.
func (m *RowModel) Clip(i, off, w int) ([]safepresentation.Cell, []filebuffer.Span) {
	if m.key.Wrap || off <= 0 {
		return m.Cells(i), m.Highlights(i)
	}
	if i < 0 || i >= len(m.rows) {
		return nil, nil
	}
	r := m.rows[i]
	cells := m.src.Cells(r.line)
	var out []safepresentation.Cell
	var from []int // each output cell's line cell index; -1 for blanks
	col, end := 0, off+w
	for _, c := range m.src.Clusters(r.line) {
		next := col + c.Width
		if col < end && next > off {
			if col >= off && next <= end {
				for k := c.Start; k < c.End; k++ {
					out = append(out, cells[k])
					from = append(from, k)
				}
			} else {
				lo, hi := col, next
				if lo < off {
					lo = off
				}
				if hi > end {
					hi = end
				}
				for n := hi - lo; n > 0; n-- {
					out = append(out, safepresentation.Cell{Text: " "})
					from = append(from, -1)
				}
			}
		}
		if col = next; col >= end {
			break
		}
	}
	mark := make([]bool, len(out))
	for _, s := range m.Highlights(i) {
		for j, k := range from {
			if k >= s.Start && k < s.End {
				mark[j] = true
			}
		}
	}
	var spans []filebuffer.Span
	for j := 0; j < len(mark); {
		if !mark[j] {
			j++
			continue
		}
		k := j
		for k < len(mark) && mark[k] {
			k++
		}
		spans = append(spans, filebuffer.Span{Start: j, End: k})
		j = k
	}
	return out, spans
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

// Hidden is a rendered row's hidden-content report for the
// run-off-edge text window [off, off+w): cells concealed left of the
// window and match/marker spans entirely concealed on each side.
type Hidden struct {
	// LeftText reports cells of the line hidden left of the window.
	LeftText bool
	// LeftMatch reports a match/marker with no painted cell whose
	// columns reach left of the window.
	LeftMatch bool
	// RightMatch reports a match/marker with no painted cell whose
	// columns reach past the window's right edge.
	RightMatch bool
}

// Hidden computes rendered row i's hidden-content report for the
// window [off, off+w). Visibility is the rendered-cell kind Clip
// produces: only clusters wholly inside the window paint — edge-split
// clusters render as blanks and outside clusters drop — so a span
// covered by blanks alone counts as hidden, while a span with one
// painted cell is visible and earns no hidden-match indicator for that
// side. Wrap-mode models and out-of-range rows report nothing hidden:
// wrap draws no indicators.
func (m *RowModel) Hidden(i, off, w int) Hidden {
	var h Hidden
	if m.key.Wrap || w <= 0 || i < 0 || i >= len(m.rows) {
		return h
	}
	r := m.rows[i]
	clusters := m.src.Clusters(r.line)
	if len(clusters) == 0 {
		return h
	}
	end := off + w
	// One pass over the line's clusters finds the cells hidden left of
	// the window and the painted run [pLo, pHi] — the consecutive
	// cluster indexes wholly inside it.
	pLo, pHi := len(clusters), -1
	col := 0
	for k, c := range clusters {
		next := col + c.Width
		h.LeftText = h.LeftText || col < off
		if col >= off && next <= end {
			if k < pLo {
				pLo = k
			}
			pHi = k
		}
		col = next
	}
	for _, s := range m.src.Highlights(r.line) {
		if s.Start >= s.End {
			continue
		}
		// The clusters holding the span's first and last cells — its
		// whole cluster range, since clusters partition the cells.
		lo := sort.Search(len(clusters), func(k int) bool { return clusters[k].End > s.Start })
		if lo >= len(clusters) {
			continue
		}
		hi := sort.Search(len(clusters), func(k int) bool { return clusters[k].End > s.End-1 })
		if hi >= len(clusters) {
			hi = len(clusters) - 1
		}
		if lo <= pHi && hi >= pLo {
			continue // a painted cell makes the span visible
		}
		if clusterCol(clusters, lo) < off {
			h.LeftMatch = true
		}
		if clusterCol(clusters, hi)+clusters[hi].Width > end {
			h.RightMatch = true
		}
	}
	return h
}

// clusterCol returns the source-display column at which cluster k
// starts — the sum of the preceding clusters' widths.
func clusterCol(clusters []safepresentation.Cluster, k int) int {
	col := 0
	for _, c := range clusters[:k] {
		col += c.Width
	}
	return col
}

// TargetRow returns the rendered row holding the stop's display target:
// the row of its source line that contains the first coverage range's
// start cell, so a match far down a wrapped line lands on its own row.
func (m *RowModel) TargetRow(stop searchindex.Stop) int {
	line, cell := m.src.TargetCell(stop)
	return m.rowOf(line, cell)
}

// TargetColumn returns the horizontal reveal's aim point for a stop:
// the source-display column at which the cluster holding the stop's
// target cell starts, and that cluster's terminal cell width. A target
// cell past the line's cells — an insertion point at the line's end —
// aims at the line's extent as a single end-of-line cell.
func (m *RowModel) TargetColumn(stop searchindex.Stop) (col, width int) {
	line, cell := m.src.TargetCell(stop)
	for _, c := range m.src.Clusters(line) {
		if cell < c.End {
			return col, c.Width
		}
		col += c.Width
	}
	return col, 1
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
// wrap mode, one in run-off-edge mode — the column the hidden-content
// indicators of Hidden populate.
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
