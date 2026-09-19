// Package viewport owns logical reading position and derives rendered-row
// visibility in wrap and run-off-edge modes.
//
// Issue #5 lands only the seam's first responsibility: a top-of-file
// vertical window over one loaded filebuffer. Scrolling, destination
// reveal, wrap, logical anchors, and horizontal state arrive in later
// issues.
package viewport

import "vrg/internal/filebuffer"

// Viewport is the file panel's vertical window over one loaded buffer.
// The zero value shows the top of the file; Issue #12 owns scrolling.
type Viewport struct{ top int }

// Top is the index of the first visible source line.
func (v Viewport) Top() int { return v.top }

// Visible returns the source lines shown within height rows: the window
// [top, top+height) intersected with the buffer's lines.
func (v Viewport) Visible(lines []filebuffer.Line, height int) []filebuffer.Line {
	if v.top >= len(lines) || height <= 0 {
		return nil
	}
	end := v.top + height
	if end > len(lines) {
		end = len(lines)
	}
	return lines[v.top:end]
}
