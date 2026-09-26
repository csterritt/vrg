package viewport

import "vrg/internal/present"

// Row is one rendered row's prepared data: the source line the row
// leads with or continues, the display-cell offset within that line
// where the row begins, plus the display cells and highlight spans a
// frame paints, translated into row-local cells. In run-off-edge mode
// rendered row i is source line i with Start 0; wrap mode makes the
// mapping many-to-one.
type Row struct {
	Line int
	// Start is the display-column offset within the source line where
	// the row begins — a wrapped continuation row starts mid-line, a
	// run-off-edge row at 0. (Line, Start) is the row's logical
	// location: the anchor coordinates a rewrap restores.
	Start int
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
// current content width and wrap mode, delivered by an off-path
// layout-preparation job and installed when its key matches the
// current parameters. Frame rendering queries it only for the visible
// range.
type Rows interface {
	// Len returns the total rendered-row count.
	Len() int
	// Row returns the prepared data of rendered row i.
	Row(i int) Row
	// RowOf returns the rendered row containing the display target.
	// Run-off-edge, a target's row is its source line; wrap mode's
	// many-to-one mapping lands the row covering the target cell.
	RowOf(t Target) int
	// Wrap reports the mode the model was prepared for: true for
	// wrap, false for run-off-edge. Horizontal panning and the extent
	// clamp apply only to a run-off-edge model — under a wrap model
	// the offset is dormant: retained, applied to no rendering, and
	// re-clamped when a run-off-edge model next installs.
	Wrap() bool
}

// Viewport owns the current file's vertical reading position: the
// prepared rows, the content dimensions, the effective top rendered
// row, and the logical anchor. The anchor is a (source line,
// display-column offset) location independent of wrap width; after a
// rewrap, wrap toggle, or row-model swap the effective top is the row
// containing the anchor's location, not the row with the same former
// ordinal. The top is clamped to valid content — never below 0 and
// never past the last full page, so no avoidable blank rows appear
// below EOF; content shorter than the viewport pins the top to 0 and
// leaves its unused rows naturally. With no prepared rows (the loading
// and unreadable placeholders) every scroll is a no-op, reveal is
// inert, and queries stay empty. The horizontal pan offset is separate
// state: under a run-off-edge model it is clamped to the visible-lines
// extent — recomputed from the visible rows on every pan and every
// visible-set change — while under a wrap model it is dormant, kept
// for the next re-entry. Horizontal reveal arrives with Issue 19.
type Viewport struct {
	width  int // text columns available to content
	height int // content rows
	rows   Rows
	top    int    // first visible rendered row
	anchor Target // logical reading position: source line + display column
	off    int    // horizontal pan offset: the first painted column
}

// Resize sets the content-area dimensions in cells and re-resolves the
// top row from the anchor: growth that would leave avoidable blank
// rows below EOF pulls the top upward, the documented lossy clamp.
func (v *Viewport) Resize(width, height int) {
	v.width, v.height = width, height
	v.resolve()
}

// Height returns the content height in rows.
func (v *Viewport) Height() int { return v.height }

// Top returns the first visible rendered row.
func (v *Viewport) Top() int { return v.top }

// Anchor returns the logical reading position — the source line and
// display-column offset the effective top resolves from. It is the
// width-independent form of the reading position: callers persist it
// as per-file viewport state rather than a row ordinal.
func (v *Viewport) Anchor() Target { return v.anchor }

// SetAnchor moves the reading position to a logical location —
// a previously saved per-file anchor — and resolves the effective top
// to the row containing it under the installed rows, clamped to valid
// content.
func (v *Viewport) SetAnchor(t Target) {
	v.anchor = t
	v.resolve()
}

// SetTop moves the top row to a previously saved position, clamped to
// the current content, and replaces the anchor with the resulting top
// row's location.
func (v *Viewport) SetTop(top int) {
	v.top = top
	v.clamp()
	if v.count() > 0 {
		v.anchor = v.loc(v.top)
	}
}

// SetRows installs the current file's prepared rows — when a prepared
// layout for the current parameters arrives — and re-resolves the top
// row from the retained anchor. nil marks unavailable content: the
// viewport empties but the anchor is retained for the next install.
func (v *Viewport) SetRows(rows Rows) {
	v.rows = rows
	v.resolve()
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

// Offset returns the horizontal pan offset in display cells — the
// first column a run-off-edge frame paints. Under a wrap model the
// value is dormant: retained for the next run-off-edge re-entry.
func (v *Viewport) Offset() int { return v.off }

// SetOffset replaces the horizontal pan offset — the file-change reset
// drives it with 0 — clamped to the visible-lines extent when the
// installed model is run-off-edge and stored dormant under wrap.
func (v *Viewport) SetOffset(off int) {
	v.off = max(off, 0)
	v.clampOff()
}

// Left pans one column left; Right pans one column right.
func (v *Viewport) Left()  { v.pan(-1) }
func (v *Viewport) Right() { v.pan(1) }

// TenLeft pans ten columns left; TenRight pans ten columns right.
func (v *Viewport) TenLeft()  { v.pan(-10) }
func (v *Viewport) TenRight() { v.pan(10) }

// HalfLeft pans max(1, floor(text width / 2)) columns left; HalfRight
// pans the same right.
func (v *Viewport) HalfLeft()  { v.pan(-v.halfW()) }
func (v *Viewport) HalfRight() { v.pan(v.halfW()) }

func (v *Viewport) halfW() int { return max(1, v.width/2) }

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
	if v.top != before {
		v.anchor = v.loc(v.top)
	}
	return v.top != before
}

func (v *Viewport) half() int { return max(1, v.height/2) }

// pan moves the horizontal offset by delta columns, clamped to
// [0, max(0, S)] where S is the paintable-boundary maximum of the
// widest currently rendered source line — recomputed from the visible
// rows on every pan, never cached. Under a wrap model or with no
// prepared rows panning is a no-op.
func (v *Viewport) pan(delta int) {
	if v.rows == nil || v.rows.Wrap() {
		return
	}
	v.off = min(max(v.off+delta, 0), v.maxOffset())
}

// clampOff applies the visible-lines extent clamp to the stored
// offset. It runs on every change that can alter the visible row set —
// scroll, reveal, resize, and a row-model swap including wrap-toggle
// re-entry — so the offset is always valid for the rows now visible;
// the loss is permanent, never restored when a wider line returns.
// Under a wrap model the offset is dormant and left alone; with no
// prepared rows the placeholder maximum is 0.
func (v *Viewport) clampOff() {
	if v.rows != nil && v.rows.Wrap() {
		return
	}
	v.off = min(max(v.off, 0), v.maxOffset())
}

// maxOffset returns the maximum valid horizontal offset under the
// visible-lines extent policy: the paintable-boundary maximum of the
// widest currently rendered source line — the largest cell index at
// which one of its grapheme clusters starts that also fits within the
// text width, so at that offset the whole cluster still paints and
// clamping alone can never blank the text area or draw half a glyph.
// Among equally wide lines the smallest boundary wins so every widest
// line keeps a fully painted cluster; a widest line with no cluster
// fitting the text width — and an empty or all-empty view — gives 0.
func (v *Viewport) maxOffset() int {
	s, widest := 0, -1
	for i := v.top; i < v.count() && i < v.top+v.height; i++ {
		e, p := lineExtent(v.rows.Row(i), v.width)
		if e > widest {
			widest, s = e, p
		} else if e == widest {
			s = min(s, p)
		}
	}
	return max(0, s)
}

// lineExtent returns the content extent and the paintable boundary of
// a run-off-edge row's source line at text width w: the extent is the
// line's display width in cells; the boundary is the largest cell
// index where a cluster starts whose own cell width fits within w, so
// it is fully paintable at that offset. A line whose final cluster
// cannot fit starting inside it reports the last fitting cluster's
// start; a line with no cluster fitting w at all reports 0.
// End-of-line marker cells join both values with Issue #23 — a marker
// extends the extent by one cell and is itself a paintable cell.
func lineExtent(r Row, w int) (extent, paintable int) {
	extent = len(r.Cells)
	for i := 0; i < extent; {
		e := i + 1
		for e < extent && !r.Cells[e].Lead {
			e++
		}
		if e-i <= w {
			paintable = i
		}
		i = e
	}
	return extent, paintable
}

// scroll moves the top row by delta rendered rows, clamped to valid
// content. A scroll that moves replaces the logical anchor with the
// resulting top row's location; one clamped to no movement keeps it —
// a retained logical column survives an ineffective scroll.
func (v *Viewport) scroll(delta int) {
	if v.count() == 0 {
		v.top = 0
		return
	}
	before := v.top
	v.top += delta
	v.clamp()
	if v.top != before {
		v.anchor = v.loc(v.top)
	}
}

// loc returns the logical location of rendered row i: its source line
// and the display-column offset where it begins.
func (v *Viewport) loc(i int) Target {
	r := v.rows.Row(i)
	return Target{Line: r.Line, Cell: r.Start}
}

// resolve recomputes the effective top as the row containing the
// anchor under the installed rows, then clamps to valid content. The
// clamp is deliberately lossy: when it moves the top off the anchor's
// row the anchor is rewritten to the clamped top's location, so a
// later shrink or rewrap does not restore the pre-clamp position.
func (v *Viewport) resolve() {
	if v.count() == 0 {
		v.top = 0
	} else {
		want := v.rows.RowOf(v.anchor)
		v.top = min(max(want, 0), v.maxTop())
		if v.top != want {
			v.anchor = v.loc(v.top)
		}
	}
	// The resolve may have changed the visible row set — a resize, a
	// row-model swap, wrap-toggle re-entry — so the horizontal offset
	// re-clamps against it.
	v.clampOff()
}

// clamp keeps the top row within valid content: at least 0, and no
// further than the last full page. Content shorter than the viewport
// has maxTop 0, pinning the top and leaving unused rows naturally.
// Every clamp also re-clamps the horizontal offset: a moved top is a
// changed visible set.
func (v *Viewport) clamp() {
	v.top = min(max(v.top, 0), v.maxTop())
	v.clampOff()
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
// the buffer. Under a run-off-edge model each row is clipped to the
// pan offset and text width with grapheme-safe blanking; under a wrap
// model the offset is dormant and rows pass through unclipped.
func (v *Viewport) Visible() []Row {
	n := min(v.count()-v.top, v.height)
	if n <= 0 {
		return nil
	}
	clip := !v.rows.Wrap()
	out := make([]Row, n)
	for i := range out {
		r := v.rows.Row(v.top + i)
		if clip {
			r = clipRow(r, v.off, v.width)
		}
		out[i] = r
	}
	return out
}

// clipRow returns a run-off-edge row's painted form at horizontal
// offset off in a text area w cells wide: the line's cells
// [off, off+w), its spans translated into window cells and clipped to
// the window, and Start advanced to the first painted column. A
// grapheme cluster split by either clip edge paints its in-window
// cells blank — never a half glyph.
func clipRow(r Row, off, w int) Row {
	off, w = max(off, 0), max(w, 0)
	if off == 0 && len(r.Cells) <= w {
		return r
	}
	lo := min(off, len(r.Cells))
	hi := min(off+w, len(r.Cells))
	cells := r.Cells[lo:hi]

	// A cluster split by the left edge contributes only Cont cells;
	// one split by the right edge runs past hi. Either way its
	// in-window cells paint blank.
	var cp []present.Cell
	blank := func(from, to int) {
		if cp == nil {
			cp = make([]present.Cell, len(cells))
			copy(cp, cells)
		}
		for i := from; i < to; i++ {
			cp[i] = present.Cell{Text: " "}
		}
	}
	if lo < hi && r.Cells[lo].Cont {
		e := lo + 1
		for e < hi && r.Cells[e].Cont {
			e++
		}
		blank(0, e-lo)
	}
	if hi > lo && hi < len(r.Cells) && r.Cells[hi].Cont {
		s := hi - 1
		for s > lo && r.Cells[s].Cont {
			s--
		}
		blank(s-lo, hi-lo)
	}
	if cp != nil {
		cells = cp
	}

	var spans []present.Span
	for _, s := range r.Spans {
		if s.Start == s.End {
			if s.Start >= off && s.Start < off+w {
				spans = append(spans, present.Span{Start: s.Start - off, End: s.Start - off})
			}
			continue
		}
		if a, b := max(s.Start, off), min(s.End, off+w); a < b {
			spans = append(spans, present.Span{Start: a - off, End: b - off})
		}
	}
	r.Start += off
	r.Cells = cells
	r.Spans = spans
	return r
}
