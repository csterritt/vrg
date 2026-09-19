package viewport

import (
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// Hidden-content indicators (Issue #20): in run-off-edge mode the file
// panel signposts what the horizontal window cannot show — a per-line
// gutter mark for the left edge and a reserved right-column star the
// renderer draws only on the current matched line's row. Every
// judgment derives from painted-cell visibility over the window
// [off, off + width) — the cells the text renderer actually paints,
// excluding the reserved column — so a geometrically in-window cell
// blanked by a straddling grapheme cluster counts as hidden, never as
// partially visible.

// LeftMark is the row's gutter indicator under the window starting at
// off: '*' when a match or marker on the line is entirely hidden
// left, '_' when text is hidden left but no match is entirely hidden
// there, and ' ' when nothing is hidden left. A cell is hidden left
// when its grapheme cluster starts before the window — sitting left
// of it or straddling the edge and rendering blank inside it.
func LeftMark(r Row, off int) byte {
	for _, s := range r.Line.Highlights {
		if spanHiddenLeft(r.Line, s, off) {
			return '*'
		}
	}
	// Cluster starts are non-decreasing across the row, so the first
	// cell's cluster starting left of the window means the row has
	// text hidden there; an empty row hides nothing.
	if r.Start < r.End {
		if start, _ := targetCluster(r.Line, r.Start); start < off {
			return '_'
		}
	}
	return ' '
}

// RightMark is the row's reserved-column mark under the window
// [off, off + width): '*' when a match or marker on the line is
// entirely hidden right, ' ' otherwise. The mark's scope — the
// current matched line's visible row alone — is the renderer's gate;
// the column is reserved out of the text width, so it never
// overwrites text.
func RightMark(r Row, off, width int) byte {
	for _, s := range r.Line.Highlights {
		if spanHiddenRight(r.Line, s, off, width) {
			return '*'
		}
	}
	return ' '
}

// spanHiddenLeft reports whether every cell of the span is hidden
// left of the window: its grapheme cluster starts before off. An
// empty span is the one-cell position of a zero-width or end-of-line
// marker (Issue #23), which follows the same rules.
func spanHiddenLeft(l filebuffer.Line, s searchindex.Span, off int) bool {
	lo, hi := s.Start, s.End
	if hi <= lo {
		hi = lo + 1
	}
	for c := lo; c < hi; c++ {
		if start, _ := targetCluster(l, c); start >= off {
			return false
		}
	}
	return true
}

// spanHiddenRight reports whether every cell of the span is hidden
// right of the window [off, off + width): its grapheme cluster ends
// beyond it. One painted cell makes the span partially visible, and a
// partially visible match earns no star.
func spanHiddenRight(l filebuffer.Line, s searchindex.Span, off, width int) bool {
	lo, hi := s.Start, s.End
	if hi <= lo {
		hi = lo + 1
	}
	for c := lo; c < hi; c++ {
		if start, w := targetCluster(l, c); start+w <= off+width {
			return false
		}
	}
	return true
}
