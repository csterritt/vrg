package viewport

import "vrg/internal/present"

// Source is the prepared per-line data a row model lays out: display
// cells carrying the shared grapheme policy — Lead marks each
// cluster's first cell, the only legal wrap boundary, and Cont the
// trailing cells of a multi-cell unit — plus the line's validated
// highlight spans. *filebuffer.Buffer satisfies it; the row model
// consumes cluster boundaries without re-deriving segmentation.
type Source interface {
	LineCount() int
	Cells(i int) []present.Cell
	Spans(i int) []present.Span
}

// Key identifies what a Model was prepared for: the file's raw path,
// its content revision, the text width in cells, and the wrap mode.
// A prepared model whose key no longer matches the current path,
// revision, or layout is obsolete — the contract Issue #17's
// asynchronous preparation relies on.
type Key struct {
	Path  string
	Rev   int
	Width int
	Wrap  bool
}

// rowSpan locates one rendered row inside its source line: the
// half-open display-cell range the row paints.
type rowSpan struct {
	line, start, end int
}

// Model is the prepared rendered-row model of one buffer at one text
// width and wrap mode — a value swapped in when a prepared-layout job
// completes for the current parameters (Issue #17 keeps preparation
// off the update path). Layout is computed once in Prepare; Row
// materializes a queried row's cells and spans on demand, so a frame
// touches only the lines behind its visible rows.
type Model struct {
	key   Key
	src   Source
	rows  []rowSpan
	first []int // first rendered row of each source line
}

// Prepare lays out src for the key's text width and wrap mode. In
// run-off-edge mode every source line is one row carrying its full
// cells for the frame to clip. In wrap mode each line packs its
// grapheme clusters into rows of at most Width cells, breaking only at
// cluster boundaries: a cluster that cannot fit a row's remaining
// cells moves to the next row leaving a blank, and a cluster wider
// than the whole row splits across rows as a last resort. An
// end-of-line marker after a completely full final row occupies
// another row.
func Prepare(src Source, k Key) Model {
	m := Model{key: k, src: src, first: make([]int, src.LineCount())}
	for li := 0; li < src.LineCount(); li++ {
		m.first[li] = len(m.rows)
		cells := src.Cells(li)
		if !k.Wrap {
			m.rows = append(m.rows, rowSpan{li, 0, len(cells)})
			continue
		}
		m.wrapLine(li, cells)
	}
	return m
}

// Key returns what the model was prepared for.
func (m Model) Key() Key { return m.key }

// Wrap reports the model's layout mode — the key's wrap flag.
func (m Model) Wrap() bool { return m.key.Wrap }

// Len returns the total rendered-row count.
func (m Model) Len() int { return len(m.rows) }

// Row materializes rendered row i: the source line it leads with or
// continues, its slice of the line's cells, and the line's spans
// translated into row-local cells — coverage spans clipped to the
// row's range, markers painted on their owning row. A row ending
// inside a multi-cell unit blanks the clipped portion rather than
// letting the lead cell's glyph spill onto the next row.
func (m Model) Row(i int) Row {
	s := m.rows[i]
	lc := m.src.Cells(s.line)
	cells := lc[s.start:s.end]
	if s.end < len(lc) && lc[s.end].Cont {
		// The row ends inside a multi-cell unit: blank the unit's
		// lead cell so the clipped portion renders blank. A row
		// entirely inside the unit holds no lead — its Cont cells
		// already carry no text.
		j := s.end - 1
		for j > s.start && lc[j].Cont {
			j--
		}
		if !lc[j].Cont {
			cp := make([]present.Cell, len(cells))
			copy(cp, cells)
			cp[j-s.start] = present.Cell{Text: " ", Blank: true}
			cells = cp
		}
	}
	var spans []present.Span
	for _, x := range m.src.Spans(s.line) {
		if x.Start == x.End {
			if m.rowOf(s.line, x.Start) == i {
				spans = append(spans, present.Span{Start: x.Start - s.start, End: x.Start - s.start})
			}
			continue
		}
		if lo, hi := max(x.Start, s.start), min(x.End, s.end); lo < hi {
			spans = append(spans, present.Span{Start: lo - s.start, End: hi - s.start})
		}
	}
	return Row{Line: s.line, Start: s.start, Cont: i != m.first[s.line], Cells: cells, Spans: spans}
}

// RowOf returns the rendered row containing the display target: in
// run-off-edge mode the target's source line, in wrap mode the line's
// row covering the target cell — including the extra row an
// end-of-line marker occupies past a completely full wrap row.
func (m Model) RowOf(t Target) int {
	if len(m.first) == 0 {
		return 0
	}
	return m.rowOf(min(max(t.Line, 0), len(m.first)-1), t.Cell)
}

// rowOf returns the rendered row of the source line containing cell —
// the last of the line's rows whose first cell does not pass it. A
// position on a row boundary belongs to the next row, and a position
// at or past the line's end lands on the line's last row.
func (m Model) rowOf(line, cell int) int {
	r := m.first[line]
	end := len(m.rows)
	if line+1 < len(m.first) {
		end = m.first[line+1]
	}
	for r+1 < end && m.rows[r+1].start <= cell {
		r++
	}
	return r
}

// wrapLine appends the line's rendered rows: clusters pack greedily
// into rows of at most Width cells, breaking at cluster boundaries —
// a cluster that cannot fit the row's remaining cells starts the next
// row, leaving the remainder blank — and a cluster wider than the
// whole row splits across rows as a last resort.
func (m *Model) wrapLine(li int, cells []present.Cell) {
	w := m.key.Width
	if w < 1 {
		// No text columns: the line occupies one empty row.
		m.rows = append(m.rows, rowSpan{li, 0, 0})
		return
	}
	start, used := 0, 0
	for i := 0; i < len(cells); {
		e := i + 1
		for e < len(cells) && !cells[e].Lead {
			e++
		}
		if cw := e - i; cw > w {
			// The cluster cannot fit any row: close the open row and
			// fill whole rows from the cluster.
			if used > 0 {
				m.rows = append(m.rows, rowSpan{li, start, i})
			}
			for e-i > w {
				m.rows = append(m.rows, rowSpan{li, i, i + w})
				i += w
			}
			start, used = i, e-i
		} else {
			if used+cw > w {
				m.rows = append(m.rows, rowSpan{li, start, i})
				start, used = i, 0
			}
			used += cw
		}
		i = e
	}
	if used > 0 || len(m.rows) == m.first[li] {
		m.rows = append(m.rows, rowSpan{li, start, len(cells)})
	}
	// An end-of-line marker extends the line by one cell: after a
	// completely full final row it occupies another row.
	last := m.rows[len(m.rows)-1]
	if last.end-last.start == w {
		for _, sp := range m.src.Spans(li) {
			if sp.Start == sp.End && sp.Start == len(cells) {
				m.rows = append(m.rows, rowSpan{li, len(cells), len(cells)})
				break
			}
		}
	}
}
