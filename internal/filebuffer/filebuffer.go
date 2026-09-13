package filebuffer

import (
	"fmt"
	"os"

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
		highlights := expandedHighlights(d.ByteOffsets, d.ByteCells, clusters, stopsByLine[lineNum])

		lines = append(lines, Line{
			Number:     lineNum,
			Display:    d.Text,
			ByteCells:  byteCells,
			Highlights: highlights,
			Clusters:   clusters,
		})
	}

	return &Buffer{
		Lines:       lines,
		LineCount:   lineCount,
		GutterWidth: gw,
		BOMOffset:   bomOffset,
	}, nil
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
