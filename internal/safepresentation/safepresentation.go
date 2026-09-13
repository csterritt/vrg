package safepresentation

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// PathDisplay is the escaped display form of a raw path and its
// byte→cell mapping. ByteCells[i] is the [start, end) display cell
// range occupied by original byte i.
type PathDisplay struct {
	Text      string
	ByteCells [][2]int
}

// Cluster is one grapheme cluster within a display string: its byte
// range and terminal cell width. The shared grapheme segmentation and
// cell-width policy (Issue #16) produces clusters that FileBuffer
// exposes and Viewport consumes for wrapping without re-deriving.
type Cluster struct {
	// StartByte is the byte offset of the cluster start in the display
	// text.
	StartByte int
	// EndByte is the exclusive byte offset of the cluster end.
	EndByte int
	// Width is the terminal cell width of the cluster: 1 for narrow
	// glyphs, 2 for wide (East Asian Wide/Fullwidth) glyphs, 0 for
	// clusters that add no advance (combining marks attached to a base
	// are part of the base cluster, so standalone zero-width clusters are
	// rare).
	Width int
}

// GraphemeClusters segments a display string into grapheme clusters and
// computes each cluster's terminal cell width. This is the one shared
// grapheme segmentation and cell-width policy (Issue #16): FileBuffer
// calls it to populate Line.Clusters, and Viewport consumes those
// clusters for wrapping without re-deriving. The display string is
// already escaped through EscapeContent (or EscapePath); clusters
// operate on the escaped form.
func GraphemeClusters(display string) []Cluster {
	clusters := make([]Cluster, 0, len(display))
	gr := uniseg.NewGraphemes(display)
	offset := 0
	for gr.Next() {
		cluster := gr.Str()
		startByte := offset
		endByte := offset + len(cluster)
		offset = endByte
		width := uniseg.StringWidth(cluster)
		clusters = append(clusters, Cluster{
			StartByte: startByte,
			EndByte:   endByte,
			Width:     width,
		})
	}
	return clusters
}

// ContentDisplay is the escaped display form of raw content bytes and
// its byte→cell mapping. ByteCells[i] is the [start, end) display cell
// range occupied by original byte i. ByteOffsets[i] is the display byte
// offset where original byte i starts in Text. Line-terminator bytes
// (LF, CRLF) produce no display cells and map to the end-of-line
// position; their ByteOffsets point at the end of the display text.
type ContentDisplay struct {
	Text      string
	ByteCells [][2]int
	// ByteOffsets[i] is the display byte offset where original byte i
	// starts in Text (Issue #21). Used to map raw bytes to grapheme
	// clusters by display byte range.
	ByteOffsets []int
}

// EscapePath escapes raw path bytes for safe single-line display.
// Newline, carriage return, and tab become \n, \r, \t; literal backslash
// becomes \\; invalid UTF-8 bytes become \xNN; other C0 controls and DEL
// use caret notation; C1 controls use \u00XX escapes. Valid printable
// Unicode is preserved. The byte→cell map records the display cell range
// for each original byte so highlight rendering can map byte ranges to
// cells. Displayed strings never become filesystem keys; callers retain
// original bytes for identity, ordering, and file access.
func EscapePath(raw []byte) PathDisplay {
	var b strings.Builder
	b.Grow(len(raw))
	cells := make([][2]int, 0, len(raw))
	cell := 0
	for i := 0; i < len(raw); {
		c := raw[i]
		if c < utf8.RuneSelf {
			start := cell
			switch {
			case c == '\\':
				b.WriteString(`\\`)
				cell += 2
			case c == '\n':
				b.WriteString(`\n`)
				cell += 2
			case c == '\r':
				b.WriteString(`\r`)
				cell += 2
			case c == '\t':
				b.WriteString(`\t`)
				cell += 2
			case c < 0x20:
				b.WriteByte('^')
				b.WriteByte(c + '@')
				cell += 2
			case c == 0x7f:
				b.WriteString("^?")
				cell += 2
			default:
				b.WriteByte(c)
				cell++
			}
			cells = append(cells, [2]int{start, cell})
			i++
			continue
		}
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			start := cell
			fmt.Fprintf(&b, `\x%02x`, c)
			cell += 4
			cells = append(cells, [2]int{start, cell})
			i++
			continue
		}
		if r >= 0x80 && r < 0xa0 {
			start := cell
			fmt.Fprintf(&b, `\u%04x`, r)
			cell += 6
			for j := 0; j < size; j++ {
				cells = append(cells, [2]int{start, cell})
			}
			i += size
			continue
		}
		start := cell
		b.Write(raw[i : i+size])
		cell++
		for j := 0; j < size; j++ {
			cells = append(cells, [2]int{start, cell})
		}
		i += size
	}
	return PathDisplay{Text: b.String(), ByteCells: cells}
}

// EscapeContent escapes raw content bytes for safe display. LF and CRLF
// are line terminators and are never displayed; their bytes map to the
// end-of-line position. A standalone CR (not followed by LF) is escaped
// as ^M. Tabs expand to the next multiple of 8 source-display columns
// (Issue #16), replacing Issue #5's provisional → placeholder; tab
// stops count from the start of the line content (column 0), independent
// of the gutter and horizontal pan. Other C0 controls and DEL use caret
// notation. C1 controls use \u00XX escapes. Invalid UTF-8 renders as
// U+FFFD while the byte→cell map retains the raw-byte mapping. Valid
// printable Unicode is preserved. The byte→cell map records the display
// cell range for each original byte so highlight rendering can cover all
// cells of an escaped form.
func EscapeContent(raw []byte) ContentDisplay {
	var b strings.Builder
	b.Grow(len(raw))
	cells := make([][2]int, 0, len(raw))
	offsets := make([]int, 0, len(raw))
	cell := 0
	for i := 0; i < len(raw); {
		c := raw[i]
		if c == '\r' && i+1 < len(raw) && raw[i+1] == '\n' {
			cells = append(cells, [2]int{cell, cell})
			cells = append(cells, [2]int{cell, cell})
			offsets = append(offsets, b.Len())
			offsets = append(offsets, b.Len())
			i += 2
			continue
		}
		if c == '\n' {
			cells = append(cells, [2]int{cell, cell})
			offsets = append(offsets, b.Len())
			i++
			continue
		}
		if c < utf8.RuneSelf {
			start := cell
			off := b.Len()
			switch {
			case c == '\r':
				b.WriteString("^M")
				cell += 2
			case c == '\t':
				// Issue #16: expand tabs to the next multiple of 8
				// source-display columns. Tab stops count from column
				// 0 (start of line content), independent of the gutter
				// and horizontal pan.
				spaces := 8 - (cell % 8)
				for j := 0; j < spaces; j++ {
					b.WriteByte(' ')
				}
				cell += spaces
			case c < 0x20:
				b.WriteByte('^')
				b.WriteByte(c + '@')
				cell += 2
			case c == 0x7f:
				b.WriteString("^?")
				cell += 2
			default:
				b.WriteByte(c)
				cell++
			}
			cells = append(cells, [2]int{start, cell})
			offsets = append(offsets, off)
			i++
			continue
		}
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			off := b.Len()
			b.WriteRune('\ufffd')
			cells = append(cells, [2]int{cell, cell + 1})
			offsets = append(offsets, off)
			cell++
			i++
			continue
		}
		if r >= 0x80 && r < 0xa0 {
			start := cell
			off := b.Len()
			fmt.Fprintf(&b, `\u%04x`, r)
			cell += 6
			for j := 0; j < size; j++ {
				cells = append(cells, [2]int{start, cell})
				offsets = append(offsets, off)
			}
			i += size
			continue
		}
		start := cell
		off := b.Len()
		b.Write(raw[i : i+size])
		cell++
		for j := 0; j < size; j++ {
			cells = append(cells, [2]int{start, cell})
			offsets = append(offsets, off)
		}
		i += size
	}
	return ContentDisplay{Text: b.String(), ByteCells: cells, ByteOffsets: offsets}
}

// EscapeDiagnostic escapes raw diagnostic bytes for safe display while
// preserving real line boundaries. LF is preserved as a line boundary;
// CRLF is normalized to LF (the CR is consumed, the LF is retained) so
// no raw CR control byte survives. Tabs are expanded to eight-column
// stops with the column counter resetting at each newline. Other C0
// controls and DEL use caret notation. C1 controls use \u00XX escapes.
// Invalid UTF-8 bytes use \xNN escapes. Backslashes are not escaped so
// that filenames already escaped through EscapePath can be embedded
// without double-escaping. Valid printable Unicode is preserved.
// Callers that embed filenames in diagnostics must first escape the
// filename through EscapePath so filename newlines cannot become
// diagnostic paragraph breaks.
func EscapeDiagnostic(raw []byte) string {
	var b strings.Builder
	b.Grow(len(raw))
	col := 0
	for i := 0; i < len(raw); {
		c := raw[i]
		if c == '\r' && i+1 < len(raw) && raw[i+1] == '\n' {
			b.WriteByte('\n')
			col = 0
			i += 2
			continue
		}
		if c == '\n' {
			b.WriteByte('\n')
			col = 0
			i++
			continue
		}
		if c == '\t' {
			spaces := 8 - (col % 8)
			for j := 0; j < spaces; j++ {
				b.WriteByte(' ')
			}
			col += spaces
			i++
			continue
		}
		if c < utf8.RuneSelf {
			switch {
			case c < 0x20:
				b.WriteByte('^')
				b.WriteByte(c + '@')
				col += 2
			case c == 0x7f:
				b.WriteString("^?")
				col += 2
			default:
				b.WriteByte(c)
				col++
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&b, `\x%02x`, c)
			col += 4
			i++
			continue
		}
		if r >= 0x80 && r < 0xa0 {
			fmt.Fprintf(&b, `\u%04x`, r)
			col += 6
			i += size
			continue
		}
		b.Write(raw[i : i+size])
		col++
		i += size
	}
	return b.String()
}
