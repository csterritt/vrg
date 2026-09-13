package filebuffer

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// Buffer is a loaded, decoded, and mapped file ready for display. The
// completion message carries a fully prepared buffer so Update does no
// full-file work.
type Buffer struct {
	Lines       []Line
	LineCount   int
	GutterWidth int
	// BOMOffset is the number of leading UTF-8 BOM bytes stripped from
	// the raw file before line splitting (Issue #22). Ripgrep 15.x
	// removes a leading UTF-8 BOM from searched line data, so
	// submatch offsets for line 1 are relative to the line without the
	// BOM. Load strips the BOM so ByteCells and submatch offsets align
	// in rg-line coordinates; BOMOffset is retained for converting
	// back to raw-file coordinates (e.g. Issue #29 stale validation).
	// Zero when no leading BOM is present; 3 (EF BB BF) otherwise.
	BOMOffset int
	// Stale is true when at least one submatch was dropped by stale
	// validation on this load (Issue #29). Stale entries remain
	// navigation stops; surviving submatches keep their highlights.
	// The filename row shows "file changed since search" while Stale
	// is true. Reload recomputes Stale: it clears only when the newly
	// loaded content passes validation for every retained submatch.
	Stale bool
}

// Cluster is one grapheme cluster within a display string: its byte
// range and terminal cell width. It is an alias for
// safepresentation.Cluster so callers can use filebuffer.Cluster
// without importing safepresentation directly.
type Cluster = safepresentation.Cluster

// Line is one display-ready source line.
type Line struct {
	// Number is the 1-based source line number.
	Number int
	// Display is the escaped display text (no line terminator).
	Display string
	// ByteCells maps each original byte index to its [start, end) display
	// cell range.
	ByteCells [][2]int
	// Highlights are the display cell ranges to render in inverse video.
	Highlights [][2]int
	// Clusters are the grapheme clusters of the display text, produced
	// by the shared grapheme segmentation policy (Issue #16). Viewport
	// consumes these for wrapping without re-deriving.
	Clusters []safepresentation.Cluster
	// StartByte is the byte offset in the source line's display text
	// where a wrapped row begins. Zero for source lines and the first
	// row of a source line; non-zero for continuation rows. Set by the
	// viewport row model.
	StartByte int
	// Continuation is true for wrapped rows that are not the first row
	// of their source line. The renderer shows a blank gutter for
	// continuation rows. Set by the viewport row model.
	Continuation bool
	// RawBytes are the original line bytes including terminators, in
	// rg-line coordinates (BOM stripped for line 1). Retained for
	// stale-match validation (Issue #29): each submatch's range and
	// byte equality are checked against these bytes, not the stripped
	// display text.
	RawBytes []byte
	// ContentWidth is the display content width before any end-of-line
	// marker extension (Issue #29). The end-of-line position for a
	// clamped-start fallback reveal target; when there is no marker
	// cell, the fallback clamps to the last rendered cell
	// (ContentWidth-1).
	ContentWidth int
	// HasMarker is true when an end-of-line zero-width marker cell was
	// appended to the display (Issue #29). Used by the end-of-line
	// fallback to distinguish the marker cell from a past-content
	// position.
	HasMarker bool
	// ValidStarts are the Start byte offsets of submatches that
	// passed stale validation on this line, in recorded order (Issue
	// #29). The first entry is the reveal target when survivors exist.
	// Empty when no submatches survived.
	ValidStarts []int
	// FirstRecordedStart is the first recorded submatch's Start on
	// this line, used as the fallback reveal target when no submatches
	// survived (Issue #29). RevealTarget clamps it to the available
	// line bytes.
	FirstRecordedStart int
}

// Load reads, decodes, and maps a file's bytes into a display-ready
// buffer. The stops provide the match data for this file (filtered by
// the caller to the file's raw path). The returned buffer is fully
// prepared so the caller's Update does no full-file work.
func Load(path []byte, stops []searchindex.Stop) (*Buffer, error) {
	data, err := os.ReadFile(string(path))
	if err != nil {
		return nil, err
	}

	// Detect and strip a leading UTF-8 BOM (EF BB BF). Ripgrep 15.x
	// removes it from searched line data under default detection, so
	// submatch offsets for line 1 are relative to the line without the
	// BOM. Stripping it here keeps the raw-line bytes in rg-line
	// coordinates so ByteCells and submatch offsets align; BOMOffset
	// is retained on the Buffer for converting back to raw-file
	// coordinates. Non-leading U+FEFF is not a file BOM and remains
	// as ordinary content.
	bomOffset := 0
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		bomOffset = 3
		data = data[3:]
	}

	rawLines := splitLines(data)
	lineCount := len(rawLines)
	gw := gutterWidth(lineCount)

	stopsByLine := make(map[int][]searchindex.Stop)
	for _, s := range stops {
		stopsByLine[s.LineNumber] = append(stopsByLine[s.LineNumber], s)
	}

	// Issue #29: best-effort stale validation. For each stop, check
	// every submatch against the original line bytes (including
	// terminators, in rg-line coordinates with the BOM stripped for
	// line 1): line existence, range validity, and byte equality with
	// the recorded match bytes. Any failure drops that submatch and
	// marks the buffer stale; surviving submatches keep their
	// highlights. Validation runs on first load and every reload
	// (both call Load), so staleness recomputes each time. This is
	// best-effort correspondence, not a snapshot or regex re-evaluation;
	// same-text moves and changes outside matched spans can remain
	// undetected. UTF-16/32 files are excluded (Issue #30).
	stale := false
	validStopsByLine := make(map[int][]searchindex.Stop, len(stopsByLine))
	validStartsByLine := make(map[int][]int, len(stopsByLine))
	firstStartByLine := make(map[int]int, len(stopsByLine))
	for lineNum, lineStops := range stopsByLine {
		var rawLine []byte
		if lineNum >= 1 && lineNum <= lineCount {
			rawLine = rawLines[lineNum-1]
		}
		for _, s := range lineStops {
			filtered := s
			filtered.Submatches = nil
			var validStarts []int
			for _, sm := range s.Submatches {
				if !submatchValid(rawLine, sm, lineNum, lineCount) {
					stale = true
					continue
				}
				filtered.Submatches = append(filtered.Submatches, sm)
				validStarts = append(validStarts, sm.Start)
			}
			validStopsByLine[lineNum] = append(validStopsByLine[lineNum], filtered)
			validStartsByLine[lineNum] = append(validStartsByLine[lineNum], validStarts...)
			if len(s.Submatches) > 0 {
				firstStartByLine[lineNum] = s.Submatches[0].Start
			}
		}
	}

	lines := make([]Line, 0, lineCount)
	for i, rawLine := range rawLines {
		lineNum := i + 1
		d := safepresentation.EscapeContent(rawLine)
		clusters := safepresentation.GraphemeClusters(d.Text)

		// Issue #21: expand each submatch's byte range to the enclosing
		// grapheme-cluster boundaries so highlights never split a
		// cluster. ByteCells are remapped so every byte in a cluster
		// (including combining marks) maps to the cluster's full cell
		// range; this makes the Issue #19 reveal target the cluster
		// start and keeps indicators consistent with the expanded span.
		// A standalone cluster with width 0 (no base, no visible cell)
		// receives a visible fallback cell so the highlight is never
		// zero cells.
		byteCells := expandedByteCells(d.ByteCells, d.ByteOffsets, clusters)
		// Issue #29: compute highlights from validated stops only so
		// dropped submatches produce no highlight.
		highlights := expandedHighlights(d.ByteOffsets, d.ByteCells, clusters, validStopsByLine[lineNum])

		// Issue #23: zero-width submatches (Start == End) render as
		// one inverse-video cell at their mapped display location.
		// A position inside a cluster maps to the cluster start (via
		// the expanded ByteCells); a terminator or end-of-line
		// position maps to the display end-of-line column. A marker at
		// end of line extends the effective line width by one cell, so
		// a space is appended to the display text and a 1-cell cluster
		// is appended to the Clusters slice. An empty matched line
		// therefore has width one. A marker after a completely full
		// wrap row occupies another row because the extra cluster
		// participates in wrapping. The marker highlight is a one-cell
		// range [cell, cell+1) that participates in clipping, indicators,
		// and reveal like any other highlight. The terminator-only $
		// marker is an ordinary marker with no special cases.
		// Issue #29: markers come from validated stops only.
		display := d.Text
		markerCells := markerCellsForStops(validStopsByLine[lineNum], byteCells, clusters)
		eolCell := clusterContentWidth(clusters)
		eolMarkerAdded := false
		for _, mc := range markerCells {
			if mc == eolCell && !eolMarkerAdded {
				display = display + " "
				clusters = append(clusters, safepresentation.Cluster{
					StartByte: len(d.Text),
					EndByte:   len(d.Text) + 1,
					Width:     1,
				})
				eolMarkerAdded = true
			}
			highlights = append(highlights, [2]int{mc, mc + 1})
		}
		// Sort highlights by start cell so renderLineWithHighlights
		// processes them in cell order without skipping markers that
		// precede non-zero-width highlights.
		sort.SliceStable(highlights, func(i, j int) bool {
			return highlights[i][0] < highlights[j][0]
		})

		lines = append(lines, Line{
			Number:             lineNum,
			Display:            display,
			ByteCells:          byteCells,
			Highlights:         highlights,
			Clusters:           clusters,
			RawBytes:           rawLine,
			ContentWidth:       eolCell,
			HasMarker:          eolMarkerAdded,
			ValidStarts:        validStartsByLine[lineNum],
			FirstRecordedStart: firstStartByLine[lineNum],
		})
	}

	return &Buffer{
		Lines:       lines,
		LineCount:   lineCount,
		GutterWidth: gw,
		BOMOffset:   bomOffset,
		Stale:       stale,
	}, nil
}

// submatchValid checks one submatch against the original line bytes
// (Issue #29). The rawLine is the original line bytes including
// terminators in rg-line coordinates (BOM stripped for line 1), so rg
// offsets align directly. lineNum and lineCount determine line
// existence: a missing line (lineNum outside [1, lineCount]) fails
// every submatch. Range validity requires Start <= End and
// [Start, End) within [0, len(rawLine)]. Byte equality requires
// rawLine[Start:End] to equal the recorded match bytes. Both JSON
// encodings (text and base64) are already decoded by searchindex, so
// the comparison is plain byte equality. A zero-width submatch
// (Start == End) validates when Start is in range and the match bytes
// are empty.
func submatchValid(rawLine []byte, sm searchindex.Submatch, lineNum, lineCount int) bool {
	if lineNum < 1 || lineNum > lineCount {
		return false
	}
	if sm.Start < 0 || sm.End < sm.Start || sm.End > len(rawLine) {
		return false
	}
	return bytes.Equal(rawLine[sm.Start:sm.End], sm.Match)
}

// splitLines splits file bytes into lines, each including its terminator.
// LF and CRLF are line terminators; a standalone CR is not. A trailing
// terminator does not produce an extra empty line.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); {
		if data[i] == '\n' {
			lines = append(lines, data[start:i+1])
			start = i + 1
			i++
		} else if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
			lines = append(lines, data[start:i+2])
			start = i + 2
			i += 2
		} else {
			i++
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// gutterWidth returns the digit count of the largest line number plus
// two spaces, with a minimum of one digit.
func gutterWidth(lineCount int) int {
	digits := 1
	if lineCount > 0 {
		digits = len(fmt.Sprintf("%d", lineCount))
	}
	return digits + 2
}

// expandedByteCells remaps the per-byte display cell ranges from
// EscapeContent so every byte in a grapheme cluster (including combining
// marks and ZWJ joiners) maps to the cluster's full cell range (Issue
// #21). This makes the Issue #19 reveal target the cluster start for a
// combining-only match and keeps ByteCells consistent with the expanded
// highlight spans. A standalone cluster with width 0 (no base, no
// visible cell) receives a visible fallback cell of width 1 so the
// highlight is never zero cells.
//
// byteOffsets[i] is the display byte offset where raw byte i starts;
// it is matched to the grapheme cluster whose [StartByte, EndByte)
// contains it, and the cell range is replaced with that cluster's
// [cellStart, cellEnd). Cluster cell positions are derived by summing
// cluster widths.
func expandedByteCells(rawCells [][2]int, byteOffsets []int, clusters []safepresentation.Cluster) [][2]int {
	if len(rawCells) == 0 {
		return rawCells
	}
	// Compute each cluster's cell range [cellStart, cellEnd).
	clusterCells := make([][2]int, len(clusters))
	cellPos := 0
	for ci, c := range clusters {
		start := cellPos
		width := c.Width
		// Issue #21: a standalone cluster with width 0 receives a
		// visible fallback cell so the highlight is never zero cells.
		if width == 0 {
			width = 1
		}
		clusterCells[ci] = [2]int{start, start + width}
		cellPos += c.Width
	}
	// displayByteEnd[i] is the display byte offset where raw byte i's
	// display text ends. For a multi-byte rune, all its bytes share the
	// same start offset; the end is the next distinct offset or
	// len(byteOffsets) boundary. Used to determine whether a raw
	// byte's display text spans multiple clusters (escaped form like
	// ESC → ^[) or sits within one cluster (combining mark).
	displayByteEnd := computeDisplayByteEnd(byteOffsets)
	remapped := make([][2]int, len(rawCells))
	for i, rc := range rawCells {
		ci := clusterContainingByte(byteOffsets, clusters, i)
		if ci < 0 {
			// No cluster covers this byte (e.g. a newline at end-
			// of-line). Keep the original mapping.
			remapped[i] = rc
			continue
		}
		cc := clusterCells[ci]
		// If the raw byte's display text spans beyond this cluster
		// (e.g. ESC → ^[ produces two clusters), preserve the
		// original multi-cell range. Otherwise remap to the cluster's
		// cell range (e.g. a combining mark remaps to the base
		// cluster's range).
		if displayByteEnd[i] > clusters[ci].EndByte {
			// Spans multiple clusters: keep original range.
			remapped[i] = rc
			continue
		}
		remapped[i] = cc
	}
	return remapped
}

// computeDisplayByteEnd returns, for each raw byte i, the display byte
// offset where its display text ends. For a multi-byte rune, all its
// bytes share the same start offset (byteOffsets[i]); the end is the
// next distinct offset in byteOffsets, or len(byteOffsets) for the last
// distinct offset. Line-terminator bytes (LF, CRLF) produce no display
// text, so their end equals their start.
func computeDisplayByteEnd(byteOffsets []int) []int {
	ends := make([]int, len(byteOffsets))
	if len(byteOffsets) == 0 {
		return ends
	}
	// Find the next distinct offset for each position.
	for i := range byteOffsets {
		off := byteOffsets[i]
		end := off
		for j := i + 1; j < len(byteOffsets); j++ {
			if byteOffsets[j] > off {
				end = byteOffsets[j]
				break
			}
		}
		if end == off {
			// Last distinct offset: end is the display text length.
			// We don't have the text length here; use a sentinel
			// that the caller's cluster check handles. For safety,
			// use a large value so the multi-cluster check passes
			// only when there is a genuinely later offset.
			end = off
		}
		ends[i] = end
	}
	return ends
}

// clusterContainingByte returns the index of the cluster whose display
// byte range [StartByte, EndByte) contains the display byte offset of
// raw byte byteIdx, or -1 if none. byteOffsets[byteIdx] gives the
// display byte offset for raw byte byteIdx.
func clusterContainingByte(byteOffsets []int, clusters []safepresentation.Cluster, byteIdx int) int {
	if byteIdx < 0 || byteIdx >= len(byteOffsets) {
		return -1
	}
	off := byteOffsets[byteIdx]
	for ci, c := range clusters {
		if off >= c.StartByte && off < c.EndByte {
			return ci
		}
	}
	return -1
}

// expandedHighlights converts each submatch's source-byte range to a
// display-cell span expanded to the enclosing grapheme-cluster
// boundaries (Issue #21). The span starts at the cell start of the
// cluster containing the submatch's start byte, and ends at the cell
// end of the cluster containing the submatch's last byte. A standalone
// cluster with width 0 receives a visible fallback cell. The expanded
// span is the sole source of highlight spans for Viewport and App.
//
// The original cell range from EscapeContent's ByteCells is the
// starting point; expansion only grows the range outward to cluster
// boundaries, never shrinks it. This preserves multi-cell escaped forms
// (e.g. ESC → ^[ covers both cells) while expanding partial-cluster
// matches (e.g. a combining-only match expands to the base cluster).
func expandedHighlights(byteOffsets []int, rawCells [][2]int, clusters []safepresentation.Cluster, stops []searchindex.Stop) [][2]int {
	if len(stops) == 0 || len(clusters) == 0 {
		return nil
	}
	// Compute each cluster's cell range.
	clusterCells := make([][2]int, len(clusters))
	cellPos := 0
	for ci, c := range clusters {
		start := cellPos
		width := c.Width
		if width == 0 {
			width = 1
		}
		clusterCells[ci] = [2]int{start, start + width}
		cellPos += c.Width
	}
	displayByteEnd := computeDisplayByteEnd(byteOffsets)
	clusterForByte := func(byteIdx int) int {
		return clusterContainingByte(byteOffsets, clusters, byteIdx)
	}
	// spanMultiCluster reports whether raw byte byteIdx's display text
	// spans multiple grapheme clusters (e.g. ESC → ^[). When true, the
	// original multi-cell range is preserved instead of remapping to a
	// single cluster's range.
	spanMultiCluster := func(byteIdx int) bool {
		ci := clusterForByte(byteIdx)
		if ci < 0 || byteIdx >= len(displayByteEnd) {
			return false
		}
		return displayByteEnd[byteIdx] > clusters[ci].EndByte
	}
	var highlights [][2]int
	for _, s := range stops {
		for _, sm := range s.Submatches {
			if sm.Start >= sm.End {
				continue
			}
			if sm.Start >= len(byteOffsets) {
				continue
			}
			startCi := clusterForByte(sm.Start)
			if startCi < 0 {
				continue
			}
			end := sm.End - 1
			if end < 0 {
				end = 0
			}
			if end >= len(byteOffsets) {
				end = len(byteOffsets) - 1
			}
			endCi := clusterForByte(end)
			endHasCluster := endCi >= 0
			if !endHasCluster {
				endCi = startCi
			}
			// Determine the expanded cell range. When a raw byte's
			// display text sits within one cluster (combining mark,
			// ZWJ joiner), replace its cell range with the cluster's
			// range. When it spans multiple clusters (ESC → ^[),
			// preserve the original multi-cell range. When the end
			// byte has no cluster (a terminator byte mapping to the
			// zero-width end-of-line position), use its original cell
			// end so a span covering visible text plus terminator
			// highlights only the visible text (Issue #22).
			hlStart := rawCells[sm.Start][0]
			hlEnd := rawCells[end][1]
			if spanMultiCluster(sm.Start) {
				// Start byte spans multiple clusters: keep its
				// original cell start (escaped form).
				hlStart = rawCells[sm.Start][0]
			} else {
				hlStart = clusterCells[startCi][0]
			}
			if spanMultiCluster(end) {
				// End byte spans multiple clusters: keep its
				// original cell end (escaped form).
				hlEnd = rawCells[end][1]
			} else if !endHasCluster {
				// End byte is a terminator or has no cluster: use
				// its original cell end (the end-of-line position)
				// so the highlight covers the visible text without
				// the terminator.
				hlEnd = rawCells[end][1]
			} else {
				hlEnd = clusterCells[endCi][1]
			}
			if hlStart < hlEnd {
				highlights = append(highlights, [2]int{hlStart, hlEnd})
			}
		}
	}
	return highlights
}

// markerCellsForStops returns the display cell positions of zero-width
// submatches (Start == End) for the given stops (Issue #23). Each
// position is mapped through the expanded ByteCells: a byte inside a
// cluster maps to the cluster start cell, and a terminator or
// out-of-range byte maps to the display end-of-line column (the sum
// of cluster widths). Duplicate cells are removed so multiple
// zero-width submatches at the same position produce one marker.
func markerCellsForStops(stops []searchindex.Stop, byteCells [][2]int, clusters []safepresentation.Cluster) []int {
	contentWidth := clusterContentWidth(clusters)
	var cells []int
	seen := make(map[int]bool)
	for _, s := range stops {
		for _, sm := range s.Submatches {
			if sm.Start != sm.End || sm.Start < 0 {
				continue
			}
			var cell int
			if sm.Start < len(byteCells) {
				cell = byteCells[sm.Start][0]
			} else {
				cell = contentWidth
			}
			if !seen[cell] {
				seen[cell] = true
				cells = append(cells, cell)
			}
		}
	}
	return cells
}

// clusterContentWidth returns the total terminal cell width of the
// given clusters: the sum of each cluster's Width. This is the display
// content extent before any EOL marker extension (Issue #23).
func clusterContentWidth(clusters []safepresentation.Cluster) int {
	w := 0
	for _, c := range clusters {
		w += c.Width
	}
	return w
}

// RevealTarget returns the validated reveal target for a navigation
// stop (Issue #29). It returns the 0-based source-line index, the byte
// offset within that line, and the display cell of the target.
//
// For a stop whose line exists and has surviving submatches, the
// target is the first survivor's start. For a stop whose line exists
// but has no survivors, the target is the first recorded start clamped
// to the available line bytes, with the display cell clamped to the
// last rendered cell when the byte maps to the end-of-line position
// and there is no marker cell. For a stop whose line is gone, the
// target is the last source line's start. For an empty file, lineIdx
// is -1 indicating no target.
//
// Fallbacks never invent highlights or markers: they only position the
// reveal. The byte offset feeds the vertical row lookup; the display
// cell feeds the horizontal reveal.
func (b *Buffer) RevealTarget(stop searchindex.Stop) (lineIdx, byteStart, cell int) {
	lineIdx = stop.LineNumber - 1
	if lineIdx < 0 || lineIdx >= len(b.Lines) {
		// Missing line: land at the last source line's start.
		if b.LineCount == 0 {
			// Empty file: zero-line panel, no target.
			return -1, 0, 0
		}
		return b.LineCount - 1, 0, 0
	}
	line := b.Lines[lineIdx]
	// When the buffer was not validated (e.g. test buffers built with
	// makeBuf that lack RawBytes), fall back to the first recorded
	// submatch directly so existing behavior is preserved.
	if line.RawBytes == nil {
		if len(stop.Submatches) > 0 {
			byteStart = stop.Submatches[0].Start
			if byteStart >= 0 && byteStart < len(line.ByteCells) {
				cell = line.ByteCells[byteStart][0]
			}
		}
		return lineIdx, byteStart, cell
	}
	if len(line.ValidStarts) > 0 {
		byteStart = line.ValidStarts[0]
	} else {
		// No survivors: clamp the first recorded start to the
		// available line bytes.
		byteStart = line.FirstRecordedStart
		if byteStart < 0 {
			byteStart = 0
		}
		if byteStart > len(line.RawBytes) {
			byteStart = len(line.RawBytes)
		}
	}
	cell = revealCellForByte(line, byteStart)
	return lineIdx, byteStart, cell
}

// revealCellForByte maps a byte offset to a display cell with the
// end-of-line fallback (Issue #29). When the byte maps to the
// end-of-line position (the content width, past the last content
// cell) and there is no marker cell, the cell clamps to the last
// rendered cell. When there is a marker cell, the end-of-line position
// is the marker cell and no clamping is needed.
func revealCellForByte(line Line, byteStart int) int {
	if byteStart >= 0 && byteStart < len(line.ByteCells) {
		cell := line.ByteCells[byteStart][0]
		if cell >= line.ContentWidth && !line.HasMarker {
			// End-of-line fallback: clamp to the last rendered
			// content cell.
			if line.ContentWidth > 0 {
				return line.ContentWidth - 1
			}
			return 0
		}
		return cell
	}
	// Byte at or past the end of the line. Map to the end-of-line
	// position, then apply the marker/fallback rule.
	if line.HasMarker {
		return line.ContentWidth
	}
	if line.ContentWidth > 0 {
		return line.ContentWidth - 1
	}
	return 0
}
