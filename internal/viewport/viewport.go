package viewport

// Viewport owns the logical reading position of one file panel. The
// Issue 5 browse tracer needs only the top rendered row, clamped to the
// content extent; wrap, panning, anchors, and reveal arrive in later
// issues.
type Viewport struct {
	top int
}

// SetExtent clamps the top row into [0, max(0, rows-height)] for a
// document of rows rendered rows shown in a panel height rows tall.
func (v *Viewport) SetExtent(rows, height int) {
	max := rows - height
	if max < 0 {
		max = 0
	}
	if v.top > max {
		v.top = max
	}
	if v.top < 0 {
		v.top = 0
	}
}

// Top returns the first rendered row of the visible window.
func (v *Viewport) Top() int { return v.top }
