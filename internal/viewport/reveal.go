package viewport

import "vrg/internal/searchindex"

// Target is the display location a destination reveal brings on
// screen: the start cell of the first submatch on a matched source
// line — the marker cell for a zero-width match (Issue #23) — subject
// to Issue #29's stale-entry fallback.
type Target struct {
	// Line is the destination line's source line number.
	Line int64
	// Cell is the first submatch's start cell on that line.
	Cell int
}

// StopTarget resolves a navigation stop to its display target: the
// destination line's number plus the display cell where its first
// submatch starts, mapped through the line's byte→cell map — an
// escaped byte widens into several cells, so the cell is not the byte
// offset. A submatch whose bytes produced no cell — a zero-width
// position or a terminator-only span (Issues #22/#23) — targets the
// marker cell one past the line's last. A stop whose line the prepared
// rows do not hold (the file changed on disk; Issue #29 owns stale
// handling) keeps the bare line number.
func (r *Rows) StopTarget(st searchindex.Stop) Target {
	t := Target{Line: st.Number}
	row := int(st.Number) - 1
	if len(st.Submatches) == 0 || row < 0 || row >= len(r.lines) {
		return t
	}
	l := r.lines[row]
	s := st.Submatches[0]
	// A zero-width submatch maps to the cell holding its position; a
	// nonempty one to the first cell its bytes produced.
	end := s.End
	if end <= s.Start {
		end = s.Start + 1
	}
	if lo, _, ok := l.CellsCovering(s.Start, end); ok {
		t.Cell = lo
	} else {
		t.Cell = len(l.Cells)
	}
	return t
}

// TargetRow is the rendered row containing the stop's display target —
// the row a reveal must show. In the still-unwrapped panel a source
// line is exactly one row, so the target's cell selects nothing yet;
// Issue #16's wrap-aware rows map it into the line's wrapped rows. A
// line number outside the prepared rows clamps to the nearest real
// row.
func (r *Rows) TargetRow(st searchindex.Stop) int {
	if len(r.lines) == 0 {
		return 0
	}
	row := int(r.StopTarget(st).Line) - 1
	if row < 0 {
		row = 0
	}
	if row >= len(r.lines) {
		row = len(r.lines) - 1
	}
	return row
}

// Reveal applies the vertical destination-reveal rules for the
// rendered row holding the display target: an already-visible target
// row leaves the viewport unchanged; otherwise the viewport moves so
// the row lands at zero-based position floor(height / 3) — the top
// becomes row − height/3 — clamped to valid tops, so BOF and EOF
// content take precedence over the one-third placement. Reveal reports
// whether the viewport moved, distinguishing a saved-state replacement
// from a no-scroll reveal.
func (v *Viewport) Reveal(row, rows, height int) bool {
	if height > 0 && row >= v.top && row < v.top+height {
		return false
	}
	top := v.top
	v.top = row - height/3
	v.Clamp(rows, height)
	return v.top != top
}
