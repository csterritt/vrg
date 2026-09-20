package viewport

// Location is a width-independent logical text position: a 0-based
// source line and a display-column offset within that line. It is the
// viewport's anchor form, surviving rewraps, wrap toggles, and resizes
// that change how many rendered rows a line occupies.
type Location struct {
	Line, Col int
}

// Viewport owns the logical reading position of one file panel: a
// width-independent logical anchor plus the installed row model that
// maps the anchor to the effective top rendered row of the visible
// window. Scroll units are rendered rows — one row, a half page, or a
// full page of the content height.
type Viewport struct {
	model  *RowModel
	height int
	top    int
	anchor Location
}

// SetLayout installs a prepared row model and the content height — the
// file-panel height minus the filename row — and re-derives the
// effective top: the rendered row containing the anchor's text
// location, not the row with the same former ordinal. The retained
// logical column only narrows the anchor to a containing row, so a wrap
// off/on round trip restores the original row. When the anchor's row
// would leave avoidable blank rows below the end of the file the EOF
// clamp pulls the top up and moves the anchor to the resulting top —
// deliberately lossy, so a later shrink need not restore the old
// position. A file shorter than the viewport has no avoidable blank
// rows: the effective top sits at zero while the anchor is retained.
func (v *Viewport) SetLayout(m *RowModel, height int) {
	v.model = m
	v.height = height
	row := v.rowOf(v.anchor)
	if max := v.extent() - height; max >= 0 && row > max {
		v.anchor = v.location(max)
		row = max
	}
	v.top = row
	v.clamp()
}

// Top returns the first rendered row of the visible window.
func (v *Viewport) Top() int { return v.top }

// Anchor returns the logical anchor: the source line and display-column
// offset the effective top derives from.
func (v *Viewport) Anchor() Location { return v.anchor }

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

// Reveal applies the destination-reveal contract to a target rendered
// row: a target already inside the window leaves the top unchanged and
// retains the anchor's logical column, and a hidden target lands at
// zero-based row floor(height/3) of the content area by moving the top,
// clamped to the valid positions — at BOF and EOF the available content
// takes precedence over one-third placement. The reveal starts from
// whatever top the viewport holds, so a caller seeds it with the saved
// per-file state on a revisit or the top of the file on a first visit;
// a reveal that moves the top replaces the anchor with the resulting
// top row's location.
func (v *Viewport) Reveal(target int) {
	if target >= v.top && target < v.top+v.height {
		return
	}
	v.top = target - v.height/3
	v.clamp()
	v.anchor = v.location(v.top)
}

// halfPage is the half-page scroll unit: max(1, floor(height/2)).
func (v *Viewport) halfPage() int {
	if h := v.height / 2; h > 1 {
		return h
	}
	return 1
}

// scroll moves the top row by n rendered rows, clamped to the extent,
// and replaces the anchor with the resulting top row's location: a user
// scroll commits to wherever the top lands.
func (v *Viewport) scroll(n int) {
	v.top += n
	v.clamp()
	v.anchor = v.location(v.top)
}

// clamp keeps the top row in [0, max(0, extent-height)]: never above
// the first row, and never so far down that avoidable blank rows show
// below the last rendered row.
func (v *Viewport) clamp() {
	if max := v.extent() - v.height; v.top > max {
		v.top = max
	}
	if v.top < 0 {
		v.top = 0
	}
}

// extent is the installed layout's rendered row count — zero before the
// first layout installs.
func (v *Viewport) extent() int {
	if v.model == nil {
		return 0
	}
	return v.model.LineCount()
}

// rowOf maps a logical location to the rendered row containing it in
// the installed layout — zero before the first layout installs.
func (v *Viewport) rowOf(loc Location) int {
	if v.model == nil {
		return 0
	}
	return v.model.rowOf(loc.Line, loc.Col)
}

// location maps a rendered row back to its logical location — the zero
// location before the first layout installs.
func (v *Viewport) location(row int) Location {
	if v.model == nil {
		return Location{}
	}
	return v.model.location(row)
}
