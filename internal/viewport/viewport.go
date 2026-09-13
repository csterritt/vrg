// Package viewport owns the scrollable content view for the browse
// panel. Issue #5 shows the top of the file; later issues add scrolling,
// cursor tracking, and reveal-on-match.
package viewport

import "vrg/internal/filebuffer"

// Viewport is a minimal scrollable view of file content. Issue #5
// renders from the top; Offset is always 0 until later issues add
// scrolling.
type Viewport struct {
	Lines  []filebuffer.Line
	Height int
	Offset int
}

// New creates a viewport showing the top of the file.
func New(lines []filebuffer.Line, height int) *Viewport {
	return &Viewport{Lines: lines, Height: height, Offset: 0}
}

// Visible returns the lines visible at the current offset. For Issue #5
// the offset is always 0 (top of file).
func (v *Viewport) Visible() []filebuffer.Line {
	if v.Offset < 0 {
		v.Offset = 0
	}
	end := v.Offset + v.Height
	if end > len(v.Lines) {
		end = len(v.Lines)
	}
	if v.Offset > end {
		return nil
	}
	return v.Lines[v.Offset:end]
}
