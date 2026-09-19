// Package viewport owns logical reading position and derives rendered-row
// visibility in wrap and run-off-edge modes.
//
// Issue #12 lands the manual-scroll responsibilities: the scroll units
// (one rendered row, half page, full page), clamping to valid content
// with no avoidable blank rows below EOF, and the prepared rendered-row
// model a frame render slices instead of rescanning the buffer.
// Destination reveal, wrap, logical anchors, and horizontal state
// arrive in later issues.
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

// Rows is the prepared rendered-row model for one loaded buffer at the
// current layout — the data a frame render slices rather than rescanning
// the buffer. The still-unwrapped panel maps each source line to
// exactly one rendered row, so the model is the buffer's lines plus its
// gutter; Issues #16/#17 own wrap-aware rows keyed by layout and
// preparation off the update path.
type Rows struct {
	lines  []filebuffer.Line
	gutter int
}

// Prepare builds the rendered-row model for a loaded buffer; a nil
// buffer prepares an empty model whose queries stay safe.
func Prepare(buf *filebuffer.Buffer) *Rows {
	r := &Rows{}
	if buf != nil {
		r.lines = buf.Lines()
		r.gutter = buf.GutterWidth()
	}
	return r
}

// Len is the number of rendered rows.
func (r *Rows) Len() int { return len(r.lines) }

// At returns the source line rendered as row i; it panics outside
// [0, Len), as a slice index does.
func (r *Rows) At(i int) filebuffer.Line { return r.lines[i] }

// GutterWidth is the line-number gutter width the rows' panel renders
// with.
func (r *Rows) GutterWidth() int { return r.gutter }
