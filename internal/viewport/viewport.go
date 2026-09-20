package viewport

// Viewport owns the logical reading position of one file panel: the top
// rendered row of the visible window, clamped to the content extent.
// Scroll units are rendered rows — one row, a half page, or a full page
// of the content height. Wrap, panning, anchors, and reveal arrive in
// later issues.
type Viewport struct {
	top    int
	rows   int
	height int
}

// SetExtent records the rendered row count and the content height — the
// file-panel height minus the filename row — and clamps the top row
// into [0, max(0, rows-height)]. Shrinking the extent pulls the top up
// so no avoidable blank rows remain below the end of the file; a file
// shorter than the viewport leaves its unused rows naturally.
func (v *Viewport) SetExtent(rows, height int) {
	v.rows = rows
	v.height = height
	v.clamp()
}

// Top returns the first rendered row of the visible window.
func (v *Viewport) Top() int { return v.top }

// Up scrolls up one rendered row.
func (v *Viewport) Up() { v.scroll(-1) }

// Down scrolls down one rendered row.
func (v *Viewport) Down() { v.scroll(1) }

// HalfUp scrolls up half a page of the content height.
func (v *Viewport) HalfUp() { v.scroll(-v.halfPage()) }

// HalfDown scrolls down half a page of the content height.
func (v *Viewport) HalfDown() { v.scroll(v.halfPage()) }

// PageUp scrolls up one full page of the content height.
func (v *Viewport) PageUp() { v.scroll(-v.height) }

// PageDown scrolls down one full page of the content height.
func (v *Viewport) PageDown() { v.scroll(v.height) }

// halfPage is the half-page scroll unit: max(1, floor(height/2)).
func (v *Viewport) halfPage() int {
	if h := v.height / 2; h > 1 {
		return h
	}
	return 1
}

// scroll moves the top row by n rendered rows, clamped to the extent.
func (v *Viewport) scroll(n int) {
	v.top += n
	v.clamp()
}

// clamp keeps the top row in [0, max(0, rows-height)]: never above the
// first row, and never so far down that avoidable blank rows show below
// the last rendered row.
func (v *Viewport) clamp() {
	if max := v.rows - v.height; v.top > max {
		v.top = max
	}
	if v.top < 0 {
		v.top = 0
	}
}
