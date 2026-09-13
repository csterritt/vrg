// Package viewport owns logical reading position and derives
// rendered-row visibility in wrap and run-off-edge modes. Issue #12
// adds manual vertical scrolling with clamping and a RowProvider
// interface so frame rendering queries only the visible row range.
// Issue #16 adds the wrap row model, grapheme-boundary wrapping, tab
// stops, the reserved indicator width, and the swappable row model
// keyed by (path, content revision, text width, wrap mode).
package viewport

import "vrg/internal/filebuffer"

// WrapMode selects wrap or run-off-edge row-model construction.
type WrapMode int

const (
	// WrapOn wraps long lines at grapheme-cluster boundaries to fit
	// the text width. This is the initial mode (Issue #16: wrapping
	// on by default).
	WrapOn WrapMode = iota
	// WrapOff is run-off-edge mode: each source line is one rendered
	// row, clipped by the text width, with a reserved right-indicator
	// column.
	WrapOff
)

// ReservedWidth returns the reserved right-indicator width for the
// given wrap mode: 0 in wrap mode, 1 in run-off-edge mode (Issue #16).
// The column is reserved now and populated by Issue #20.
func ReservedWidth(mode WrapMode) int {
	if mode == WrapOff {
		return 1
	}
	return 0
}

// Toggle returns the opposite wrap mode (Issue #16).
func (m WrapMode) Toggle() WrapMode {
	if m == WrapOn {
		return WrapOff
	}
	return WrapOn
}

// TextWidth returns the number of display cells available for text in
// the content panel: panelWidth minus gutterWidth minus the reserved
// indicator width (Issue #16). The gutter is the right-justified line-
// number gutter plus two spaces; the reserved indicator width is 0 in
// wrap mode and 1 in run-off-edge mode.
func TextWidth(panelWidth, gutterWidth int, mode WrapMode) int {
	tw := panelWidth - gutterWidth - ReservedWidth(mode)
	if tw < 1 {
		tw = 1
	}
	return tw
}

// RowModel is the prepared, swappable row model for a file at a given
// text width and wrap mode (Issue #16). It is built when a load
// completes, the wrap mode is toggled, or the layout changes. The
// model is keyed by (path, content revision, text width, wrap mode)
// so Issue #17 can move preparation off the UI update path without
// restructuring it. The Viewport queries only the visible row range;
// the model never invokes the wrapper for lines outside the visible
// rows.
type RowModel struct {
	rows    []filebuffer.Line
	sources []rowSource
	key     RowModelKey
}

// RowModelKey identifies a row model by path, content revision, text
// width, and wrap mode (Issue #16). Two models with the same key
// produce the same rows.
type RowModelKey struct {
	Path      string
	Revision  int
	TextWidth int
	WrapMode  WrapMode
}

// rowSource tracks the source line, byte range, and cell start for each
// rendered row, used by RowFromByte for the Issue #14 target reveal and
// by RowFromCell/RowAnchor for the Issue #17 logical anchor.
type rowSource struct {
	lineIndex int
	startByte int
	endByte   int
	startCell int // cell offset within the source line (Issue #17)
}

// Key returns the RowModelKey identifying this model.
func (m *RowModel) Key() RowModelKey { return m.key }

// RowCount returns the total number of rendered rows.
func (m *RowModel) RowCount() int { return len(m.rows) }

// Rows returns the rendered rows for the half-open [start, end) range.
func (m *RowModel) Rows(start, end int) []filebuffer.Line {
	if start < 0 {
		start = 0
	}
	if end > len(m.rows) {
		end = len(m.rows)
	}
	if start > end {
		return nil
	}
	return m.rows[start:end]
}

// RowFromByte returns the 0-based rendered row index containing the
// given byte offset in the given 0-based source line, or -1 if not
// found. Used by the Issue #14 target reveal to find the wrapped row
// containing the match start.
func (m *RowModel) RowFromByte(lineIndex, byteOffset int) int {
	for i, src := range m.sources {
		if src.lineIndex != lineIndex {
			continue
		}
		if byteOffset >= src.startByte && byteOffset < src.endByte {
			return i
		}
		// A zero-width row (empty line) maps to its startByte.
		if src.startByte == src.endByte && byteOffset == src.startByte {
			return i
		}
	}
	// Fall back: if byteOffset is at or past the last row's end for the
	// line, return the last row for that line.
	last := -1
	for i, src := range m.sources {
		if src.lineIndex == lineIndex {
			last = i
		}
	}
	if last >= 0 && byteOffset >= m.sources[last].endByte {
		return last
	}
	return -1
}

// Anchor is a width-independent logical reading position: the 0-based
// source line index and the display-column offset within that line
// (Issue #17). After a rewrap, wrap toggle, or resize, the effective
// top row is the row containing the anchor's text location, not the
// former row ordinal.
type Anchor struct {
	LineIndex int
	Column    int
}

// RowFromCell returns the 0-based rendered row index containing the
// given display-column offset in the given 0-based source line, or -1
// if not found (Issue #17). In run-off-edge mode every column of a line
// maps to the same row (one row per line).
func (m *RowModel) RowFromCell(lineIndex, column int) int {
	for i, src := range m.sources {
		if src.lineIndex != lineIndex {
			continue
		}
		// Compute the row's cell width from its clusters.
		rowWidth := 0
		for _, c := range m.rows[i].Clusters {
			rowWidth += c.Width
		}
		if column >= src.startCell && column < src.startCell+rowWidth {
			return i
		}
		// A zero-width row (empty line) maps to its startCell.
		if rowWidth == 0 && column == src.startCell {
			return i
		}
	}
	// Fall back: if column is at or past the last row's end for the
	// line, return the last row for that line.
	last := -1
	for i, src := range m.sources {
		if src.lineIndex == lineIndex {
			last = i
		}
	}
	if last >= 0 && column >= m.sources[last].startCell {
		return last
	}
	return -1
}

// RowAnchor returns the source line and display-column offset for the
// start of the given 0-based rendered row (Issue #17). In run-off-edge
// mode every row starts at column 0 (each row is a full source line).
func (m *RowModel) RowAnchor(row int) Anchor {
	if row < 0 || row >= len(m.sources) {
		return Anchor{}
	}
	src := m.sources[row]
	return Anchor{LineIndex: src.lineIndex, Column: src.startCell}
}

// BuildRowModel constructs a swappable row model from a loaded buffer
// at the given text width and wrap mode (Issue #16). In wrap mode,
// long lines are broken at grapheme-cluster boundaries to fit the
// text width; a two-cell cluster that cannot fit moves to the next
// row, leaving a blank. In run-off-edge mode, each source line is one
// rendered row. Continuation rows carry a blank gutter aligned to the
// first row's text.
func BuildRowModel(buf *filebuffer.Buffer, textWidth int, mode WrapMode, key RowModelKey) *RowModel {
	if textWidth < 1 {
		textWidth = 1
	}
	var rows []filebuffer.Line
	var sources []rowSource
	for i, line := range buf.Lines {
		if mode == WrapOff {
			rows = append(rows, line)
			sources = append(sources, rowSource{
				lineIndex: i,
				startByte: 0,
				endByte:   len(line.Display),
				startCell: 0,
			})
			continue
		}
		// Wrap mode: break at grapheme-cluster boundaries.
		wrapped := wrapLine(line, textWidth)
		for _, wr := range wrapped {
			rows = append(rows, wr.row)
			sources = append(sources, rowSource{
				lineIndex: i,
				startByte: wr.startByte,
				endByte:   wr.endByte,
				startCell: wr.startCell,
			})
		}
	}
	return &RowModel{rows: rows, sources: sources, key: key}
}

// wrappedRow is one rendered row from wrapping a source line, with the
// byte range and cell start it covers in the source line's display text.
type wrappedRow struct {
	row       filebuffer.Line
	startByte int
	endByte   int
	startCell int
}

// wrapLine breaks a source line into wrapped rows at grapheme-cluster
// boundaries to fit the text width. A two-cell cluster that cannot fit
// in the remaining row cells moves to the next row, leaving the
// remaining cells blank. Continuation rows have Continuation=true and
// a blank gutter (Number stays the same; the renderer checks
// Continuation). Highlights are adjusted to each row's local cell
// coordinates.
func wrapLine(line filebuffer.Line, textWidth int) []wrappedRow {
	clusters := line.Clusters
	if len(clusters) == 0 {
		// No clusters (empty line or no policy applied): one row.
		return []wrappedRow{{
			row:       line,
			startByte: 0,
			endByte:   len(line.Display),
			startCell: 0,
		}}
	}
	var result []wrappedRow
	startCluster := 0
	cellPos := 0
	rowStartCell := 0
	for ci := 0; ci < len(clusters); ci++ {
		c := clusters[ci]
		// If the cluster doesn't fit in the remaining cells, start a
		// new row (unless we're at the start of a row).
		if cellPos+c.Width > textWidth && cellPos > 0 {
			result = append(result, makeWrappedRow(line, startCluster, ci, len(result) > 0, rowStartCell))
			startCluster = ci
			rowStartCell += cellPos
			cellPos = 0
		}
		cellPos += c.Width
	}
	// Flush the final row.
	isCont := len(result) > 0
	result = append(result, makeWrappedRow(line, startCluster, len(clusters), isCont, rowStartCell))
	return result
}

// makeWrappedRow builds a wrapped row from a slice of clusters
// [startCluster, endCluster). The row's Display is the substring of the
// source line's Display from the first cluster's StartByte to the last
// cluster's EndByte. StartByte is the byte offset in the source line.
// Continuation is set for rows after the first. Highlights are adjusted
// to local cell coordinates. rowStartCell is the cell offset of the row
// start in the source line (Issue #17).
func makeWrappedRow(line filebuffer.Line, startCluster, endCluster int, continuation bool, rowStartCell int) wrappedRow {
	clusters := line.Clusters
	startByte := clusters[startCluster].StartByte
	var endByte int
	if endCluster > 0 && endCluster <= len(clusters) {
		endByte = clusters[endCluster-1].EndByte
	} else {
		endByte = len(line.Display)
	}
	display := line.Display[startByte:endByte]
	row := filebuffer.Line{
		Number:       line.Number,
		Display:      display,
		Highlights:   adjustHighlights(line.Highlights, rowStartCell, rowStartCell+cellWidth(clusters, startCluster, endCluster)),
		Clusters:     clusters[startCluster:endCluster],
		StartByte:    startByte,
		Continuation: continuation,
	}
	return wrappedRow{
		row:       row,
		startByte: startByte,
		endByte:   endByte,
		startCell: rowStartCell,
	}
}

// cellWidth returns the total cell width of clusters [start, end).
func cellWidth(clusters []filebuffer.Cluster, start, end int) int {
	w := 0
	for i := start; i < end && i < len(clusters); i++ {
		w += clusters[i].Width
	}
	return w
}

// adjustHighlights shifts the source line's highlight cell ranges to
// the wrapped row's local cell coordinates [rowStartCell, rowEndCell).
// Only highlights that overlap the row's cell range are included,
// clamped to the row's extent and offset to start at 0.
func adjustHighlights(highlights [][2]int, rowStartCell, rowEndCell int) [][2]int {
	if len(highlights) == 0 {
		return nil
	}
	var result [][2]int
	for _, hl := range highlights {
		hlStart := hl[0]
		hlEnd := hl[1]
		if hlEnd <= rowStartCell || hlStart >= rowEndCell {
			continue
		}
		if hlStart < rowStartCell {
			hlStart = rowStartCell
		}
		if hlEnd > rowEndCell {
			hlEnd = rowEndCell
		}
		if hlStart < hlEnd {
			result = append(result, [2]int{hlStart - rowStartCell, hlEnd - rowStartCell})
		}
	}
	return result
}

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
//
// The logical anchor (Issue #17) is a width-independent (source line,
// display-column) position. After a rewrap, wrap toggle, or resize,
// the effective top row is recomputed from the anchor so the same text
// location remains at the top. User vertical scrolling and moving
// reveals replace the anchor with the resulting top row's location.
// EOF clamping that pulls the top upward updates the anchor to the new
// top; this clamp is intentionally lossy — a later shrink need not
// restore the old top.
type Viewport struct {
	rows        RowProvider
	panelHeight int
	offset      int
	anchor      Anchor
}

// New creates a viewport with the given row provider and panel height.
// The panel height includes the filename row; content height is
// panelHeight - 1. The offset starts at 0 (top of file) and the anchor
// defaults to (0, 0).
func New(rows RowProvider, panelHeight int) *Viewport {
	v := &Viewport{rows: rows, panelHeight: panelHeight, anchor: Anchor{0, 0}}
	v.recomputeOffsetFromAnchor()
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

// RowCount returns the total number of rendered rows in the viewport's
// row provider.
func (v *Viewport) RowCount() int {
	if v.rows == nil {
		return 0
	}
	return v.rows.RowCount()
}

// Offset returns the current top row (0-based).
func (v *Viewport) Offset() int { return v.offset }

// Anchor returns the current logical anchor (Issue #17).
func (v *Viewport) Anchor() Anchor { return v.anchor }

// SetAnchor sets the logical anchor and recomputes the offset from it
// (Issue #17). The offset becomes the row containing the anchor's text
// location, clamped to [0, maxOffset]. If the clamp pulls the offset
// below the anchor's row, the anchor is updated to the new top row's
// location (lossy EOF clamp).
func (v *Viewport) SetAnchor(a Anchor) {
	v.anchor = a
	v.recomputeOffsetFromAnchor()
}

// SetOffset sets the top row, clamped to [0, maxOffset], and replaces
// the anchor with the new top row's location (Issue #17).
func (v *Viewport) SetOffset(offset int) {
	v.offset = offset
	v.clampOffset()
	v.syncAnchorToOffset()
}

// SetPanelHeight updates the panel height and recomputes the offset
// from the anchor (Issue #17). If the new maxOffset pulls the offset
// below the anchor's row (EOF clamp), the anchor is updated to the new
// top row's location.
func (v *Viewport) SetPanelHeight(panelHeight int) {
	v.panelHeight = panelHeight
	v.recomputeOffsetFromAnchor()
}

// SetRows updates the row provider and recomputes the offset from the
// anchor (Issue #17). This is used when prepared row data is rebuilt
// after a load completes, the wrap mode toggles, or the layout
// changes. The anchor is preserved; the offset becomes the row
// containing the anchor's text location in the new row model.
func (v *Viewport) SetRows(rows RowProvider) {
	v.rows = rows
	v.recomputeOffsetFromAnchor()
}

// Reveal adjusts the viewport offset so that the target row is
// visible. If the target row is already within the visible range, the
// offset is unchanged (visible-target no-scroll) and the anchor is
// retained (Issue #17). Otherwise the viewport is moved so the target
// lands at zero-based row floor(contentHeight / 3), clamped to valid
// top positions. A moving reveal replaces the anchor with the new top
// row's location (Issue #17). At BOF and EOF, available content takes
// precedence over one-third placement: the computed offset is clamped
// to [0, maxOffset], so a target near the top stays near the top and a
// target near the bottom stays near the bottom rather than forcing the
// one-third row.
func (v *Viewport) Reveal(targetRow int) {
	if v.rows == nil {
		return
	}
	contentHeight := v.ContentHeight()
	// If the target is already visible, do not scroll and retain the
	// anchor (Issue #17).
	if targetRow >= v.offset && targetRow < v.offset+contentHeight {
		return
	}
	// Place the target at zero-based row floor(contentHeight / 3) by
	// moving the viewport. Clamping to [0, maxOffset] gives BOF and
	// EOF precedence over one-third placement.
	third := contentHeight / 3
	v.offset = targetRow - third
	v.clampOffset()
	v.syncAnchorToOffset()
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

// ScrollDown scrolls one rendered row down, clamped to maxOffset, and
// replaces the anchor with the new top row's location (Issue #17).
func (v *Viewport) ScrollDown() {
	v.offset++
	v.clampOffset()
	v.syncAnchorToOffset()
}

// ScrollUp scrolls one rendered row up, clamped to 0, and replaces the
// anchor with the new top row's location (Issue #17).
func (v *Viewport) ScrollUp() {
	v.offset--
	v.clampOffset()
	v.syncAnchorToOffset()
}

// ScrollHalfDown scrolls half a page down: max(1, floor(contentHeight/2)),
// and replaces the anchor with the new top row's location (Issue #17).
func (v *Viewport) ScrollHalfDown() {
	v.offset += halfPage(v.ContentHeight())
	v.clampOffset()
	v.syncAnchorToOffset()
}

// ScrollHalfUp scrolls half a page up: max(1, floor(contentHeight/2)),
// and replaces the anchor with the new top row's location (Issue #17).
func (v *Viewport) ScrollHalfUp() {
	v.offset -= halfPage(v.ContentHeight())
	v.clampOffset()
	v.syncAnchorToOffset()
}

// ScrollPageDown scrolls a full page down: contentHeight rows, and
// replaces the anchor with the new top row's location (Issue #17).
func (v *Viewport) ScrollPageDown() {
	v.offset += v.ContentHeight()
	v.clampOffset()
	v.syncAnchorToOffset()
}

// ScrollPageUp scrolls a full page up: contentHeight rows, and replaces
// the anchor with the new top row's location (Issue #17).
func (v *Viewport) ScrollPageUp() {
	v.offset -= v.ContentHeight()
	v.clampOffset()
	v.syncAnchorToOffset()
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

// recomputeOffsetFromAnchor sets the offset to the row containing the
// anchor's text location in the current row model, clamped to
// [0, maxOffset] (Issue #17). If the clamp pulls the offset below the
// anchor's row (EOF clamp), the anchor is updated to the new top row's
// location. When the row provider is not a *RowModel (e.g. a test fake
// or BufferRows), the offset is just clamped.
func (v *Viewport) recomputeOffsetFromAnchor() {
	if v.rows == nil {
		v.offset = 0
		return
	}
	rm, ok := v.rows.(*RowModel)
	if !ok {
		v.clampOffset()
		return
	}
	row := rm.RowFromCell(v.anchor.LineIndex, v.anchor.Column)
	if row < 0 {
		row = 0
		v.anchor = Anchor{0, 0}
	}
	v.offset = row
	v.clampOffset()
	// If clamping pulled the offset below the anchor's row, update the
	// anchor to the new top row's location (lossy EOF clamp).
	if v.offset != row {
		v.anchor = rm.RowAnchor(v.offset)
	}
}

// syncAnchorToOffset replaces the anchor with the current top row's
// location (Issue #17). Called after scroll and reveal operations that
// move the viewport.
func (v *Viewport) syncAnchorToOffset() {
	rm, ok := v.rows.(*RowModel)
	if !ok || v.offset < 0 || v.offset >= rm.RowCount() {
		return
	}
	v.anchor = rm.RowAnchor(v.offset)
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
