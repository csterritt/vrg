// Package viewport owns logical reading position and derives
// rendered-row visibility in wrap and run-off-edge modes. Issue #12
// adds manual vertical scrolling with clamping and a RowProvider
// interface so frame rendering queries only the visible row range.
package viewport

import "vrg/internal/filebuffer"

// RowProvider supplies rendered rows on demand. The Viewport queries
// only the visible range so a counting fake can prove the render cost
// is proportional to the visible rows, not the full buffer. Prepared
// row data is built when a load completes or the layout changes; the
// Viewport never scans the full buffer per frame.
type RowProvider interface {
	// RowCount returns the total number of rendered rows available.
	RowCount() int
	// Rows returns the rendered rows for the half-open [start, end)
	// range. Callers must ensure 0 ≤ start ≤ end ≤ RowCount().
	Rows(start, end int) []filebuffer.Line
}

// Viewport is the scrollable content view for the browse panel. The
// panel height includes the filename row; content height is
// panelHeight - 1. The offset is the 0-based top row, clamped to
// [0, maxOffset] where maxOffset = max(0, rowCount - contentHeight).
// This ensures no avoidable blank rows below EOF; files shorter than
// the viewport naturally leave unused rows.
type Viewport struct {
	rows        RowProvider
	panelHeight int
	offset      int
}

// New creates a viewport with the given row provider and panel height.
// The panel height includes the filename row; content height is
// panelHeight - 1. The offset starts at 0 (top of file).
func New(rows RowProvider, panelHeight int) *Viewport {
	v := &Viewport{rows: rows, panelHeight: panelHeight}
	v.clampOffset()
	return v
}

// ContentHeight returns the number of content rows available for
// scrolling: panelHeight - 1 (the filename row occupies one row).
// Returns 0 when panelHeight ≤ 1.
func (v *Viewport) ContentHeight() int {
	h := v.panelHeight - 1
	if h < 0 {
		h = 0
	}
	return h
}

// Offset returns the current top row (0-based).
func (v *Viewport) Offset() int { return v.offset }

// SetOffset sets the top row, clamped to [0, maxOffset].
func (v *Viewport) SetOffset(offset int) {
	v.offset = offset
	v.clampOffset()
}

// SetPanelHeight updates the panel height and clamps the offset to the
// new maxOffset. This is used on resize to recompute layout from current
// dimensions without losing the reading position.
func (v *Viewport) SetPanelHeight(panelHeight int) {
	v.panelHeight = panelHeight
	v.clampOffset()
}

// SetRows updates the row provider and clamps the offset to the new
// maxOffset. This is used when prepared row data is rebuilt after a
// load completes or the layout changes.
func (v *Viewport) SetRows(rows RowProvider) {
	v.rows = rows
	v.clampOffset()
}

// Visible returns the visible rows from the row provider. Only the
// [offset, offset+contentHeight) range is queried, not the full buffer.
// If the file is shorter than the viewport, the returned slice is
// shorter than contentHeight; unused rows are left naturally.
func (v *Viewport) Visible() []filebuffer.Line {
	if v.rows == nil {
		return nil
	}
	contentHeight := v.ContentHeight()
	start := v.offset
	end := v.offset + contentHeight
	return v.rows.Rows(start, end)
}

// ScrollDown scrolls one rendered row down, clamped to maxOffset.
func (v *Viewport) ScrollDown() {
	v.offset++
	v.clampOffset()
}

// ScrollUp scrolls one rendered row up, clamped to 0.
func (v *Viewport) ScrollUp() {
	v.offset--
	v.clampOffset()
}

// ScrollHalfDown scrolls half a page down: max(1, floor(contentHeight/2)).
func (v *Viewport) ScrollHalfDown() {
	v.offset += halfPage(v.ContentHeight())
	v.clampOffset()
}

// ScrollHalfUp scrolls half a page up: max(1, floor(contentHeight/2)).
func (v *Viewport) ScrollHalfUp() {
	v.offset -= halfPage(v.ContentHeight())
	v.clampOffset()
}

// ScrollPageDown scrolls a full page down: contentHeight rows.
func (v *Viewport) ScrollPageDown() {
	v.offset += v.ContentHeight()
	v.clampOffset()
}

// ScrollPageUp scrolls a full page up: contentHeight rows.
func (v *Viewport) ScrollPageUp() {
	v.offset -= v.ContentHeight()
	v.clampOffset()
}

// maxOffset returns the maximum valid top row: max(0, rowCount -
// contentHeight). This ensures the last row is at the bottom with no
// avoidable blank rows below EOF.
func (v *Viewport) maxOffset() int {
	if v.rows == nil {
		return 0
	}
	rowCount := v.rows.RowCount()
	contentHeight := v.ContentHeight()
	max := rowCount - contentHeight
	if max < 0 {
		max = 0
	}
	return max
}

// clampOffset clamps the offset to [0, maxOffset].
func (v *Viewport) clampOffset() {
	max := v.maxOffset()
	if v.offset < 0 {
		v.offset = 0
	}
	if v.offset > max {
		v.offset = max
	}
}

// halfPage returns the half-page scroll amount: max(1, floor(h/2)).
func halfPage(h int) int {
	half := h / 2
	if half < 1 {
		half = 1
	}
	return half
}

// BufferRows adapts a filebuffer.Buffer to the RowProvider interface.
// The buffer's Lines slice is the prepared row data built when the load
// completes; the Viewport slices it for the visible range only.
func BufferRows(buf *filebuffer.Buffer) RowProvider {
	return bufferRows{lines: buf.Lines}
}

// bufferRows is a RowProvider backed by a slice of prepared lines.
type bufferRows struct {
	lines []filebuffer.Line
}

func (b bufferRows) RowCount() int { return len(b.lines) }

func (b bufferRows) Rows(start, end int) []filebuffer.Line {
	if start < 0 {
		start = 0
	}
	if end > len(b.lines) {
		end = len(b.lines)
	}
	if start > end {
		return nil
	}
	return b.lines[start:end]
}
