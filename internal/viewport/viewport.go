package viewport

import "vrg/internal/present"

// Row is one rendered row's prepared data: the source line the row
// leads with or continues plus the display cells and highlight spans a
// frame paints, translated into row-local cells. In run-off-edge mode
// rendered row i is source line i; wrap mode makes the mapping
// many-to-one.
type Row struct {
	Line int
	// Cont marks a continuation row: it paints the next cells of a
	// line already started above, behind a blank gutter rather than a
	// line number.
	Cont  bool
	Cells []present.Cell
	Spans []present.Span
}

// Target is a display location a reveal must show: the zero-based
// source line and the zero-based display cell within it — the start
// cell of the destination line's first submatch (the marker cell for
// a zero-width match). It is a display location from the outset, not
// a source-line ordinal: once wrap mode lands, the cell selects which
// of the line's rendered rows contains it.
type Target struct {
	Line, Cell int
}

// Rows is the prepared rendered-row model of one loaded buffer at the
// current content width and wrap mode, built when a load completes or
// the layout changes (Issue #17 owns the asynchronous contract). Frame
// rendering queries it only for the visible range.
type Rows interface {
	// Len returns the total rendered-row count.
	Len() int
	// Row returns the prepared data of rendered row i.
	Row(i int) Row
	// RowOf returns the rendered row containing the display target.
	// Run-off-edge, a target's row is its source line; wrap mode's
	// many-to-one mapping lands the row covering the target cell.
	RowOf(t Target) int
}

// Viewport owns the current file's vertical reading position: the
// prepared rows, the content dimensions, and the top rendered row. The
// top is clamped to valid content — never below 0 and never past the
// last full page, so no avoidable blank rows appear below EOF; content
// shorter than the viewport pins the top to 0 and leaves its unused
// rows naturally. With no prepared rows (the loading and unreadable
// placeholders) every scroll is a no-op, reveal is inert, and queries
// stay empty. Horizontal panning and logical anchors arrive with
// Issues 17–19.
type Viewport struct {
	width  int // text columns available to content
	height int // content rows
	rows   Rows
	top    int // first visible rendered row
}

// Resize sets the content-area dimensions in cells and re-clamps the
// top row: growth that would leave avoidable blank rows below EOF
// pulls the top upward, the documented lossy clamp.
func (v *Viewport) Resize(width, height int) {
	v.width, v.height = width, height
	v.clamp()
}

// Height returns the content height in rows.
func (v *Viewport) Height() int { return v.height }

// Top returns the first visible rendered row.
func (v *Viewport) Top() int { return v.top }

// SetTop moves the top row to a previously saved position, clamped to
// the current content — the per-file revisit seam.
func (v *Viewport) SetTop(top int) {
	v.top = top
	v.clamp()
}

// SetRows installs the current file's prepared rows — on load
// completion and when a layout change rebuilds them — preserving the
// top row clamped to the new content. nil marks unavailable content.
func (v *Viewport) SetRows(rows Rows) {
	v.rows = rows
	v.clamp()
}

// Down scrolls one rendered row down.
func (v *Viewport) Down() { v.scroll(1) }

// Up scrolls one rendered row up.
func (v *Viewport) Up() { v.scroll(-1) }

// HalfDown scrolls max(1, floor(height/2)) rendered rows down.
func (v *Viewport) HalfDown() { v.scroll(v.half()) }

// HalfUp scrolls max(1, floor(height/2)) rendered rows up.
func (v *Viewport) HalfUp() { v.scroll(-v.half()) }

// PageDown scrolls a full page — the content height — down.
func (v *Viewport) PageDown() { v.scroll(v.height) }

// PageUp scrolls a full page — the content height — up.
func (v *Viewport) PageUp() { v.scroll(-v.height) }

// Reveal makes the rendered row containing the target visible and
// reports whether the viewport moved — the caller replaces the file's
// saved vertical state only on a move. A target row already inside the
// window leaves the top unchanged; otherwise the top moves so the row
// sits at zero-based row floor(height/3), clamped to valid tops — at
// BOF/EOF the available content takes precedence over the one-third
// placement. With no prepared rows the reveal is a no-op.
func (v *Viewport) Reveal(t Target) bool {
	if v.count() == 0 {
		return false
	}
	row := min(max(v.rows.RowOf(t), 0), v.count()-1)
	if row >= v.top && row < v.top+v.height {
		return false
	}
	before := v.top
	v.top = row - v.height/3
	v.clamp()
	return v.top != before
}

func (v *Viewport) half() int { return max(1, v.height/2) }

func (v *Viewport) scroll(delta int) {
	v.top += delta
	v.clamp()
}

// clamp keeps the top row within valid content: at least 0, and no
// further than the last full page. Content shorter than the viewport
// has maxTop 0, pinning the top and leaving unused rows naturally.
func (v *Viewport) clamp() {
	v.top = min(max(v.top, 0), v.maxTop())
}

func (v *Viewport) maxTop() int { return max(0, v.count()-v.height) }

func (v *Viewport) count() int {
	if v.rows == nil {
		return 0
	}
	return v.rows.Len()
}

// Visible returns the prepared rows a frame paints — the visible range
// only, so the provider is queried once per shown row, never O(N) over
// the buffer.
func (v *Viewport) Visible() []Row {
	n := min(v.count()-v.top, v.height)
	if n <= 0 {
		return nil
	}
	out := make([]Row, n)
	for i := range out {
		out[i] = v.rows.Row(v.top + i)
	}
	return out
}
