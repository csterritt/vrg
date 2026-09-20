package viewport

import "vrg/internal/searchindex"

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
// full page of the content height. The horizontal offset is separate
// state in source-display columns: retained through wrap toggles
// subject to the re-entry clamp, reset by the caller on file change.
type Viewport struct {
	model  *RowModel
	height int
	top    int
	anchor Location
	hoff   int
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
	v.clampOffset()
}

// Top returns the first rendered row of the visible window.
func (v *Viewport) Top() int { return v.top }

// Offset returns the horizontal pan offset in source-display columns:
// the left edge of the text window in run-off-edge mode.
func (v *Viewport) Offset() int { return v.hoff }

// ResetOffset zeroes the horizontal offset — the file-change reset
// applied before the destination file's reveal.
func (v *Viewport) ResetOffset() { v.hoff = 0 }

// Pan shifts the horizontal offset n source-display columns — positive
// pans right, negative left — clamped to [0, the paintable-boundary
// maximum] recomputed from the current visible rows. It is a strict
// no-op in wrap mode and without an installed layout: the offset is
// retained state, never re-derived against a wrapped layout.
func (v *Viewport) Pan(n int) {
	if v.model == nil || v.model.Key().Wrap {
		return
	}
	v.hoff += n
	v.clampOffset()
}

// HalfPan is the half-screen pan unit: max(1, floor(text width / 2)).
func (v *Viewport) HalfPan() int {
	tw := 0
	if v.model != nil {
		tw = v.model.Key().TextWidth
	}
	if h := tw / 2; h > 1 {
		return h
	}
	return 1
}

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

// Reveal applies the destination-reveal contract to a stop: a target
// row — the rendered row holding the first submatch's start cell —
// already inside the window leaves the top unchanged and retains the
// anchor's logical column, and a hidden target lands at zero-based row
// floor(height/3) of the content area by moving the top, clamped to the
// valid positions — at BOF and EOF the available content takes
// precedence over one-third placement. The same call then performs the
// run-off-edge minimal horizontal reveal of the target cell's cluster,
// so every destination reveal covers both axes. The reveal starts from
// whatever top the viewport holds, so a caller seeds it with the saved
// per-file state on a revisit or the top of the file on a first visit;
// a reveal that moves the top replaces the anchor with the resulting
// top row's location.
func (v *Viewport) Reveal(stop searchindex.Stop) {
	if v.model == nil {
		return
	}
	if target := v.model.TargetRow(stop); target < v.top || target >= v.top+v.height {
		v.top = target - v.height/3
		v.clamp()
		v.anchor = v.location(v.top)
		v.clampOffset()
	}
	v.revealHorizontal(stop)
}

// revealHorizontal is the reveal's run-off-edge half: when the cluster
// holding the target cell is not fully painted inside the text window —
// outside it, or split by an edge into clipping blanks — the offset
// moves by the minimum columns that paint it whole: to the cluster's
// start column from the left, to start + width − text width from the
// right. A painted start cell leaves the offset untouched. A cluster
// wider than the text area can never paint at any offset: the offset is
// set to its start column — the closest achievable position, treated as
// geometrically revealed — which is stable, so repeated navigation
// never enters a panning loop. Wrap mode leaves the offset untouched:
// it is retained run-off-edge state.
func (v *Viewport) revealHorizontal(stop searchindex.Stop) {
	if v.model.Key().Wrap {
		return
	}
	tw := v.model.Key().TextWidth
	col, width := v.model.TargetColumn(stop)
	switch {
	case width > tw:
		v.hoff = col
	case col < v.hoff:
		v.hoff = col
	case col+width > v.hoff+tw:
		v.hoff = col + width - tw
	default:
		return
	}
	v.clampOffset()
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
	v.clampOffset()
}

// clampOffset keeps the horizontal offset in [0, the paintable-boundary
// maximum of the visible rows], recomputing the maximum on every call —
// every visible-set change and every pan re-derives it rather than
// trusting a cached value. Wrap-mode layouts and the no-layout state
// leave the stored offset untouched: it is retained through wrap
// toggles and re-clamped on each re-entry into run-off-edge mode.
func (v *Viewport) clampOffset() {
	if v.model == nil || v.model.Key().Wrap {
		return
	}
	if max := v.model.maxOffset(v.top, v.height); v.hoff > max {
		v.hoff = max
	}
	if v.hoff < 0 {
		v.hoff = 0
	}
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
