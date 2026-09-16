package safepresentation

import "strings"

// Shared ANSI-aware grapheme/cell helpers (Issue #39).
//
// This file is the single display-geometry policy for the final render
// path: terminal cells are measured on grapheme clusters under the
// shared rivo/uniseg policy, and ANSI escape sequences are recognized
// as zero-width units that never contribute cells and never split
// segmentation. Every display-width, truncation, and wrap consumer in
// internal/app and internal/theme routes through these helpers; no
// other production file performs its own rune decoding for display
// geometry.

// ansiEscapeLen returns the byte length of the ANSI escape sequence
// beginning at s[i] (s[i] == '\x1b'), or 0 when the bytes at i do not
// form a recognized escape sequence. Only CSI sequences are
// recognized: ESC '[', parameter and intermediate bytes (0x20-0x3F),
// and a final byte (0x40-0x7E). An ESC that does not introduce a CSI
// sequence is left to grapheme segmentation so malformed input
// degrades to visible text instead of swallowing display cells.
func ansiEscapeLen(s string, i int) int {
	if s[i] != '\x1b' || i+1 >= len(s) || s[i+1] != '[' {
		return 0
	}
	j := i + 2
	for j < len(s) && s[j] >= 0x20 && s[j] <= 0x3f {
		j++
	}
	if j < len(s) && s[j] >= 0x40 && s[j] <= 0x7e {
		return j + 1 - i
	}
	return 0
}

// GraphemeClustersANSI segments s into display units for cell
// accounting: recognized ANSI escape sequences become zero-width
// clusters at their byte positions and the text between them is
// segmented by the shared grapheme policy of GraphemeClusters. This is
// the segmentation used for display geometry on styled strings.
func GraphemeClustersANSI(s string) []Cluster {
	if strings.IndexByte(s, '\x1b') < 0 {
		return GraphemeClusters(s)
	}
	var clusters []Cluster
	i := 0
	for i < len(s) {
		if n := ansiEscapeLen(s, i); n > 0 {
			clusters = append(clusters, Cluster{StartByte: i, EndByte: i + n, Width: 0})
			i += n
			continue
		}
		// Segment the text gap up to the next recognized escape
		// sequence. An unrecognized ESC stays inside the gap so the
		// scan always makes progress.
		j := i + 1
		for j < len(s) && !(s[j] == '\x1b' && ansiEscapeLen(s, j) > 0) {
			j++
		}
		for _, c := range GraphemeClusters(s[i:j]) {
			clusters = append(clusters, Cluster{StartByte: c.StartByte + i, EndByte: c.EndByte + i, Width: c.Width})
		}
		i = j
	}
	return clusters
}

// CellWidth returns the number of terminal display cells in s under
// the shared grapheme/cell policy: grapheme clusters are measured by
// rivo/uniseg and ANSI escape sequences contribute zero cells. This is
// the single width measure for all display geometry in the final
// render path (Issue #39).
func CellWidth(s string) int {
	w := 0
	for _, c := range GraphemeClustersANSI(s) {
		w += c.Width
	}
	return w
}

// TruncateLeftCells returns the trailing keep display cells of s.
// Grapheme clusters are never split: when the cut lands inside a
// cluster the whole cluster is dropped. ANSI escape sequences and
// other zero-width clusters do not consume the cell budget; zero-width
// units directly adjacent to kept text are preserved so styling
// applied to the kept portion survives (Issue #39).
func TruncateLeftCells(s string, keep int) string {
	if keep <= 0 {
		return ""
	}
	clusters := GraphemeClustersANSI(s)
	total := 0
	for _, c := range clusters {
		total += c.Width
	}
	if total <= keep {
		return s
	}
	used := 0
	start := len(s)
	for i := len(clusters) - 1; i >= 0; i-- {
		c := clusters[i]
		if used+c.Width > keep {
			break
		}
		used += c.Width
		start = c.StartByte
	}
	return s[start:]
}
