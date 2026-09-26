package viewport

// Viewport is the minimal Issue #5 seam for logical reading position:
// it holds the content dimensions and the top source line, clamped to
// the loaded line count. Scrolling, wrapping, horizontal panning, and
// destination reveal arrive with Issues 12–19; until then the view
// stays at the top of the file.
type Viewport struct {
	width  int // text columns available to content
	height int // content rows
}

// Resize sets the content-area dimensions in cells.
func (v *Viewport) Resize(width, height int) {
	v.width, v.height = width, height
}

// Height returns the content height in rows.
func (v *Viewport) Height() int { return v.height }

// Range returns the visible window over count source lines: the first
// visible line index and how many rows follow. An empty buffer yields
// an empty range, never fictitious lines.
func (v *Viewport) Range(count int) (first, n int) {
	if count <= 0 || v.height <= 0 {
		return 0, 0
	}
	return 0, min(count, v.height)
}
