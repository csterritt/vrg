package viewport

// Horizontal panning (Issue #18): in run-off-edge mode the file panel
// shows a window into each line's display cells starting at the pan
// offset. The offset lives on the per-file Viewport so it is saved and
// restored with the rest of the reading position, and it is separate
// state from the anchor — retained through wrap toggles, reset to zero
// on every file change ahead of Issue #19's horizontal reveal.

// Off is the horizontal pan offset in display cells — the first source
// cell the run-off-edge window paints. It is meaningful only while a
// flat (run-off-edge) model is installed; wrap mode renders from cell
// zero regardless of the stored value.
func (v Viewport) Off() int { return v.off }

// ResetOff clears the horizontal offset. Navigation into a file resets
// it before the destination reveal runs, so a revisited file starts at
// its left edge rather than the offset it had when left.
func (v *Viewport) ResetOff() { v.off = 0 }

// Pan moves the horizontal offset by d cells — negative toward the left
// edge — then clamps to [0, MaxOff] computed from the rows visible now.
// The bound is re-derived on every pan, never cached, so a pan after
// the visible set changed applies the new extents. Under a wrap model
// Pan is a strict no-op: the offset is not mutated.
func (v *Viewport) Pan(d int, e Extent, height int) {
	if e.Key().Wrap {
		return
	}
	v.off += d
	v.clampOff(e, height)
}

// HalfText is the [ / ] pan unit for a text width:
// max(1, floor(width / 2)) — a degenerate width still pans one cell.
func HalfText(width int) int {
	if h := width / 2; h > 1 {
		return h
	}
	return 1
}

// MaxOff is the largest valid horizontal offset under the model e at
// effective top row top and content height n: the paintable boundary
// of the widest currently rendered source line. A line's boundary is
// the largest cell index where one of its grapheme clusters begins and
// fits entirely within the text width — at that offset the cluster
// still paints whole, so the maximum can never land inside a cluster
// or leave the window all blank while something remains paintable.
// Lines scrolled out of the window do not count: the extent policy is
// the visible set only, never the whole file. Wrap mode, an empty
// model, and an all-empty visible set all report zero.
func MaxOff(e Extent, top, n int) int {
	k := e.Key()
	if k.Wrap || k.TextWidth < 1 {
		return 0
	}
	lo := top
	if lo < 0 {
		lo = 0
	}
	hi := top + n
	if hi > e.Len() {
		hi = e.Len()
	}
	max := 0
	for i := lo; i < hi; i++ {
		if s := e.At(i).Line.MaxStart(k.TextWidth); s > max {
			max = s
		}
	}
	return max
}
