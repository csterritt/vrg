// Package viewport owns logical reading position and derives rendered-row
// visibility in wrap and run-off-edge modes.
//
// Issue #12 lands the manual-scroll responsibilities: the scroll units
// (one rendered row, half page, full page), clamping to valid content
// with no avoidable blank rows below EOF, and the prepared rendered-row
// model a frame render slices instead of rescanning the buffer.
// Issue #14 adds vertical destination reveal: the display-target
// identification, visible-target no-scroll, and one-third placement
// with BOF/EOF precedence in reveal.go. Issue #16 lands the two row
// models — wrap and run-off-edge — built from FileBuffer's shared
// grapheme-cluster segmentation, and the keyed swappable row model.
// Logical anchors and horizontal state arrive in later issues.
package viewport

import "vrg/internal/filebuffer"

// Viewport is the file panel's vertical window over one loaded file's
// prepared rows. The zero value shows the top of the file.
type Viewport struct{ top int }

// Top is the index of the first visible rendered row.
func (v Viewport) Top() int { return v.top }

// Scroll moves the top row by d rendered rows — negative toward the top
// of the file — then clamps to valid content: the result is never below
// 0 and never past MaxTop.
func (v *Viewport) Scroll(d, rows, height int) {
	v.top += d
	v.Clamp(rows, height)
}

// Clamp brings the top row back into the valid range after the row
// count or content height changes, dropping positions that would leave
// avoidable blank rows below EOF.
func (v *Viewport) Clamp(rows, height int) {
	if max := MaxTop(rows, height); v.top > max {
		v.top = max
	}
	if v.top < 0 {
		v.top = 0
	}
}

// MaxTop is the largest valid top row for a file of rows rendered rows
// under a content height: the last position whose window leaves no
// avoidable blank rows below EOF. A file no taller than the viewport —
// or no content height at all — clamps to 0.
func MaxTop(rows, height int) int {
	if height < 1 {
		return 0
	}
	if max := rows - height; max > 0 {
		return max
	}
	return 0
}

// HalfPage is the u/d scroll unit for a content height:
// max(1, floor(height / 2)).
func HalfPage(height int) int {
	if d := height / 2; d > 1 {
		return d
	}
	return 1
}

// Key identifies the exact inputs a prepared row model was built for:
// the buffer's raw path, its content revision, the text width, and the
// wrap mode. The model is a swappable value — a change in any key field
// makes the model stale, so Issue #17 can prepare a replacement off the
// update path and swap it in while the key still matches the live
// layout.
type Key struct {
	Path      string
	Revision  int
	TextWidth int
	Wrap      bool
}

// ReservedIndicator is the width the file panel's right edge reserves
// for the hidden-content indicator column under a wrap mode: zero while
// wrapping, one in run-off-edge mode. The column is reserved now and
// populated by Issue #20.
func ReservedIndicator(wrap bool) int {
	if wrap {
		return 0
	}
	return 1
}

// Row is one rendered row of the model: a contiguous range of a source
// line's display cells. A line's rows partition its cells in order.
type Row struct {
	// Line is the source line the row belongs to.
	Line filebuffer.Line
	// Start and End are the half-open display-cell range of Line.Cells
	// the row renders.
	Start, End int
}

// Continuation reports whether the row continues a wrapped source
// line: its gutter is blank and its text aligns with the first row's.
func (r Row) Continuation() bool { return r.Start > 0 }

// span is one rendered row: the line index plus the cell range.
type span struct{ line, start, end int }

// Rows is the prepared rendered-row model for one loaded buffer under
// one layout — the data a frame render slices rather than rescanning
// the buffer. In wrap mode each source line becomes one row per
// text-width band, broken only at the buffer's grapheme-cluster
// boundaries; in run-off-edge mode each line is exactly one row. The
// model is built at load, toggle, and resize time — synchronously in
// this issue, off the update path in Issue #17 — and swapped in whole.
type Rows struct {
	key   Key
	lines []filebuffer.Line
	spans []span
	// firstRow maps a source-line index to the index of its first
	// rendered row, with a final sentinel holding len(spans), so line i
	// occupies rows[firstRow[i] : firstRow[i+1]].
	firstRow []int
	gutter   int
}

// Prepare builds the rendered-row model for a loaded buffer under the
// layout the key describes; a nil buffer prepares an empty model whose
// queries stay safe.
func Prepare(buf *filebuffer.Buffer, key Key) *Rows {
	r := &Rows{key: key}
	if buf == nil {
		return r
	}
	r.lines = buf.Lines()
	r.gutter = buf.GutterWidth()
	for i, l := range r.lines {
		r.firstRow = append(r.firstRow, len(r.spans))
		if key.Wrap {
			r.wrapLine(i, l, key.TextWidth)
		} else {
			r.spans = append(r.spans, span{i, 0, len(l.Cells)})
		}
	}
	r.firstRow = append(r.firstRow, len(r.spans))
	return r
}

// wrapLine appends the line's rendered rows: each row holds as many
// whole clusters as fit in width cells; a cluster that does not fit in
// the row's remaining cells starts the next row, so a row may end with
// blank cells but never with a split cluster. A line — even an empty
// one — always yields at least one row.
func (r *Rows) wrapLine(li int, l filebuffer.Line, width int) {
	if width < 1 {
		r.spans = append(r.spans, span{li, 0, 0})
		return
	}
	start, col := 0, 0
	for _, cl := range l.Clusters {
		w := cl.End - cl.Start
		if col > 0 && col+w > width {
			r.spans = append(r.spans, span{li, start, cl.Start})
			start, col = cl.Start, 0
		}
		col += w
	}
	r.spans = append(r.spans, span{li, start, len(l.Cells)})
}

// Key returns the layout key the model was prepared for.
func (r *Rows) Key() Key { return r.key }

// Len is the number of rendered rows.
func (r *Rows) Len() int { return len(r.spans) }

// At returns rendered row i; it panics outside [0, Len), as a slice
// index does.
func (r *Rows) At(i int) Row {
	s := r.spans[i]
	return Row{Line: r.lines[s.line], Start: s.start, End: s.end}
}

// GutterWidth is the line-number gutter width the rows' panel renders
// with.
func (r *Rows) GutterWidth() int { return r.gutter }
