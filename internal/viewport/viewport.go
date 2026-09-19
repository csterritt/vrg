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
// Issue #17 lands the logical anchor: the width-independent (source
// line, display-column) reading position that survives rewraps, wrap
// toggles, and resizes, updated by scrolling, moving reveals, and the
// lossy EOF clamp. Issue #18 lands the horizontal pan offset: the
// run-off-edge cell window, its pan units, the visible-lines extent
// policy, and the paintable-boundary clamp — all in pan.go.
// Issue #19 lands the minimal horizontal reveal: painted-cell
// visibility, the cluster-width reveal arithmetic, and the unpaintable
// cluster's geometric fallback — RevealOff and CellVisible in
// reveal.go. Issue #20 lands the hidden-content indicators — the
// per-line left-gutter mark and the current-line right-column star —
// derived from the same painted-cell visibility in indicators.go.
package viewport

import "vrg/internal/filebuffer"

// Anchor is the width-independent logical reading position: the source
// line and the display-column offset within that line of the location
// the effective top row must contain. Unlike a rendered-row ordinal it
// means the same text under any wrap width or mode, so it is what a
// rewrap, wrap toggle, or resize retains.
type Anchor struct {
	// Line is the anchor's 1-based source line number.
	Line int64
	// Cell is the anchor's display-column offset within that line.
	Cell int
}

// Model is the prepared row model a viewport positions itself against:
// the row count plus the two anchor translations. *Rows is the
// production implementation; tests may substitute a fake.
type Model interface {
	// Len is the number of rendered rows.
	Len() int
	// AnchorAt is the logical location a rendered row starts at.
	AnchorAt(row int) Anchor
	// RowOf is the rendered row containing the anchor's location.
	RowOf(a Anchor) int
}

// Extent is a Model that also exposes its rendered rows and layout
// key — the contract the horizontal pan offset consults. Every
// visible-set change re-clamps the offset against MaxOff, so the
// operations that move the effective top all take an Extent.
type Extent interface {
	Model
	// Key is the layout the model was prepared for: TextWidth is the
	// paintable boundary's width, and Wrap suppresses panning and the
	// re-clamp entirely.
	Key() Key
	// At returns rendered row i; it panics outside [0, Len), as a
	// slice index does.
	At(i int) Row
}

// Viewport is the file panel's vertical window over one loaded file's
// prepared rows. The zero value shows the top of the file.
//
// top is the effective top — the first visible rendered row in the
// installed model. anchor is the retained logical position: scrolling
// and moving reveals replace it with the resulting top row's location,
// a no-scroll reveal leaves it — keeping a logical column that is
// inside rather than at the start of the top row — and end-of-file
// clamping rewrites it to the clamped top, the documented lossy case.
// off is the horizontal pan offset in display cells (Issue #18):
// meaningful only under a run-off-edge model, retained untouched while
// a wrap model is installed, and re-clamped to the paintable boundary
// of the visible rows on every visible-set change — pan, scroll,
// reveal, rewrap, restore, or clamp. Issue #19's horizontal reveal may
// also move it, by the minimum columns that paint the target cluster.
type Viewport struct {
	top    int
	anchor Anchor
	off    int
}

// Top is the index of the first visible rendered row.
func (v Viewport) Top() int { return v.top }

// Anchor is the retained logical reading position.
func (v Viewport) Anchor() Anchor { return v.anchor }

// Scroll moves the top row by d rendered rows — negative toward the top
// of the file — then clamps to valid content: the result is never below
// 0 and never past MaxTop. A scroll that moved the effective top
// replaces the logical anchor with the resulting top row's location; a
// clamped no-move scroll leaves a retained column intact. The
// horizontal offset re-clamps against the newly visible rows — a
// stored offset is never restored when a wide line returns.
func (v *Viewport) Scroll(d int, e Extent, height int) {
	top := v.top
	v.top += d
	v.clamp(e.Len(), height)
	v.reanchor(top, e)
	v.clampOff(e, height)
}

// Restore re-derives the effective top under a replacement row model —
// a rewrap, wrap toggle, or resize — as the row containing the retained
// anchor's location rather than the row with the same former ordinal.
// The logical column survives a round trip through run-off-edge mode:
// the anchor's line shows as one row while its cell is kept, and
// wrapping again restores the row containing that cell. EOF clamping
// may pull the effective top upward; when it does, the anchor is
// updated to the resulting top — the documented lossy rule. A
// run-off-edge model re-clamps the horizontal offset against the rows
// visible under the restored top, so re-entry after a wrap toggle
// applies the current visible set's maximum.
func (v *Viewport) Restore(e Extent, height int) {
	v.top = e.RowOf(v.anchor)
	top := v.top
	v.clamp(e.Len(), height)
	v.reanchor(top, e)
	v.clampOff(e, height)
}

// Clamp brings the top row back into the valid range after the row
// count or content height changes, dropping positions that would leave
// avoidable blank rows below EOF — the lossy clamp: a moved top updates
// the anchor to the resulting top row's location. The horizontal
// offset re-clamps against the rows still visible.
func (v *Viewport) Clamp(e Extent, height int) {
	top := v.top
	v.clamp(e.Len(), height)
	v.reanchor(top, e)
	v.clampOff(e, height)
}

// clamp bounds the top row to [0, MaxTop].
func (v *Viewport) clamp(rows, height int) {
	if max := MaxTop(rows, height); v.top > max {
		v.top = max
	}
	if v.top < 0 {
		v.top = 0
	}
}

// reanchor replaces the logical anchor with the effective top row's
// location when the top moved under the model m.
func (v *Viewport) reanchor(old int, m Model) {
	if v.top != old && m.Len() > 0 {
		v.anchor = m.AnchorAt(v.top)
	}
}

// clampOff pulls the horizontal offset back to the paintable boundary
// of the rows visible now — the re-clamp every visible-set change
// applies so a stored offset can never exceed what the current window
// paints. A wrap model keeps the offset untouched: it is retained
// through the toggle and re-clamped on re-entry into run-off-edge
// mode.
func (v *Viewport) clampOff(e Extent, height int) {
	if e.Key().Wrap {
		return
	}
	if max := MaxOff(e, v.top, height); v.off > max {
		v.off = max
	}
	if v.off < 0 {
		v.off = 0
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
// wrapping, one in run-off-edge mode. Issue #20's right-edge star is
// what the column holds.
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
// model is prepared off the update path on load, toggle, and resize
// (Issue #17) and installed whole, only while its key still matches
// the live layout.
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

// AnchorAt is the logical location rendered row i starts at: its source
// line's number and the row's first display cell — the location a
// scroll or moving reveal adopts as the new anchor. It panics outside
// [0, Len), as a slice index does.
func (r *Rows) AnchorAt(i int) Anchor {
	s := r.spans[i]
	return Anchor{Line: r.lines[s.line].Number, Cell: s.start}
}

// RowOf is the rendered row containing the anchor's location — the row
// a retained anchor restores to after a rewrap, so the effective top is
// the row holding the anchor's text, not the row with the same former
// ordinal. A line number outside the prepared rows clamps to the
// nearest real row; a cell past the line's cells lands on its last row.
// The empty model maps every anchor to row 0.
func (r *Rows) RowOf(a Anchor) int {
	if len(r.spans) == 0 {
		return 0
	}
	li := int(a.Line) - 1
	if li < 0 {
		li = 0
	}
	if li >= len(r.lines) {
		li = len(r.lines) - 1
	}
	row := r.firstRow[li]
	for i := row; i < r.firstRow[li+1]; i++ {
		row = i
		if a.Cell < r.spans[i].end {
			break
		}
	}
	return row
}
