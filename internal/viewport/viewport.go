package viewport

import "vrg/internal/present"

// Row is one rendered row's prepared data: the source line the row
// leads with plus the display cells and highlight spans a frame paints.
// In unwrapped mode rendered row i is source line i; Issue #16's wrap
// mode makes the mapping many-to-one.
type Row struct {
	Line  int
	Cells []present.Cell
	Spans []present.Span
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
}

// Viewport owns the current file's vertical reading position: the
// prepared rows, the content dimensions, and the top rendered row. The
// top is clamped to valid content — never below 0 and never past the
// last full page, so no avoidable blank rows appear below EOF; content
// shorter than the viewport pins the top to 0 and leaves its unused
// rows naturally. With no prepared rows (the loading and unreadable
// placeholders) every scroll is a no-op and queries stay empty. Wrap
// toggling, horizontal panning, logical anchors, and destination
// reveal arrive with Issues 13–19.
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
