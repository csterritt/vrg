package safepresentation

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/clipperhouse/displaywidth"
)

// EscapePath escapes raw path bytes into a single safe display line.
// Newline, carriage return, and tab become \n, \r, \t; a literal
// backslash doubles; invalid UTF-8 bytes become \xNN; other C0 controls
// and DEL use caret notation; C1 controls use \uXXXX escapes. Valid
// printable text — including multi-byte Unicode — passes through. The
// result is presentation only: raw bytes remain the identity and
// filesystem key. The same single-line form renders any external text
// embedded in a diagnostic or other one-line substitution.
func EscapePath(raw []byte) string {
	var b strings.Builder
	b.Grow(len(raw))
	for i := 0; i < len(raw); {
		c := raw[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '\\':
				b.WriteString(`\\`)
			case c == '\n':
				b.WriteString(`\n`)
			case c == '\r':
				b.WriteString(`\r`)
			case c == '\t':
				b.WriteString(`\t`)
			case c < 0x20:
				writeCaret(&b, c)
			case c == 0x7f:
				b.WriteString(`^?`)
			default:
				b.WriteByte(c)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&b, `\x%02x`, c)
			i++
			continue
		}
		if r >= 0x80 && r < 0xa0 {
			fmt.Fprintf(&b, `\u%04x`, r)
		} else {
			b.Write(raw[i : i+size])
		}
		i += size
	}
	return b.String()
}

// EscapeDiagnostic renders diagnostic text terminal-safe while keeping
// its structure: LF line boundaries are preserved and a CR immediately
// before LF is part of that boundary, a standalone carriage return
// renders as ^M, and tabs expand to the next multiple of eight cells.
// Every other control escapes with the path policy — caret notation for
// C0 and DEL, \uXXXX for C1, \xNN for invalid UTF-8 — while literal
// backslashes pass through so embedded EscapePath forms are not
// corrupted. Any filename vrg embeds must be single-lined with
// EscapePath first, so its newlines cannot forge paragraph breaks.
func EscapeDiagnostic(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	col := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '\n':
				b.WriteByte('\n')
				col = 0
			case c == '\r':
				if i+1 < len(s) && s[i+1] == '\n' {
					b.WriteByte('\n')
					col = 0
					i++
				} else {
					b.WriteString("^M")
					col += 2
				}
			case c == '\t':
				n := 8 - col%8
				b.WriteString(strings.Repeat(" ", n))
				col += n
			case c < 0x20:
				writeCaret(&b, c)
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
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&b, `\x%02x`, c)
			col += 4
			i++
			continue
		}
		if r >= 0x80 && r < 0xa0 {
			fmt.Fprintf(&b, `\u%04x`, r)
			col += 6
		} else {
			b.WriteString(s[i : i+size])
			col += displaywidth.Rune(r)
		}
		i += size
	}
	return b.String()
}

// Cell is one terminal display cell of escaped file content. Text is
// the cell's display string; Start and End are the half-open
// source-line byte range of the cell's grapheme cluster, so a
// byte-range highlight covers every cell of the cluster it touches.
type Cell struct {
	Text       string
	Start, End int
}

// Cluster is one grapheme cluster of an escaped source line: the
// half-open range of display cells it occupies and its width in
// terminal cells. Wrapping and clipping never split a cluster — a
// cluster that cannot fit a row's remaining cells moves to the next
// row. A tab is a single cluster spanning its expansion cells.
type Cluster struct {
	Start, End int
	Width      int
}

// EscapeContent escapes one source line's raw bytes — its line
// terminator already removed — into display cells and the line's
// grapheme clusters. C0 controls and DEL use caret notation (ESC
// renders as ^[), so a standalone carriage return renders as ^M; a tab
// expands to spaces up to the next multiple of eight source-display
// columns; C1 controls use \uXXXX escapes; invalid UTF-8 bytes each
// become one U+FFFD cell retaining their raw-byte mapping; printable
// text passes through.
func EscapeContent(raw []byte) ([]Cell, []Cluster) {
	var cells []Cell
	var clusters []Cluster
	col := 0
	g := displaywidth.BytesGraphemes(raw)
	for pos := 0; g.Next(); {
		cluster := g.Value()
		from := pos
		pos += len(cluster)
		start := len(cells)
		var width int
		if len(cluster) == 1 && cluster[0] == '\t' {
			width = 8 - col%8
			for i := 0; i < width; i++ {
				cells = append(cells, Cell{Text: " ", Start: from, End: pos})
			}
		} else {
			var text strings.Builder
			escapeCluster(&text, cluster)
			s := text.String()
			for _, r := range s {
				cells = append(cells, Cell{Text: string(r), Start: from, End: pos})
			}
			width = displaywidth.String(s)
		}
		clusters = append(clusters, Cluster{Start: start, End: len(cells), Width: width})
		col += width
	}
	return cells, clusters
}

// escapeCluster appends the escaped display text of one grapheme
// cluster's raw bytes: caret notation for C0 controls and DEL, \uXXXX
// for C1 controls, U+FFFD for each invalid UTF-8 byte, and printable
// text verbatim.
func escapeCluster(b *strings.Builder, cluster []byte) {
	for i := 0; i < len(cluster); {
		c := cluster[i]
		if c < utf8.RuneSelf {
			switch {
			case c < 0x20:
				writeCaret(b, c)
			case c == 0x7f:
				b.WriteString("^?")
			default:
				b.WriteByte(c)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(cluster[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteString("\uFFFD")
			i++
			continue
		}
		if r >= 0x80 && r < 0xa0 {
			fmt.Fprintf(b, `\u%04x`, r)
		} else {
			b.Write(cluster[i : i+size])
		}
		i += size
	}
}

// writeCaret appends the caret-notation form of a C0 control byte.
func writeCaret(b *strings.Builder, c byte) {
	b.WriteByte('^')
	b.WriteByte(c + '@')
}

// Span maps a half-open source-byte range to the half-open cell range it
// covers: every cell whose byte range intersects [start, end) is inside
// the result, so a match covering one byte of an escaped form covers all
// of its cells. A range intersecting nothing maps to the insertion point
// — the index of the first cell starting at or after start, or len(cells).
func Span(cells []Cell, start, end int) (cs, ce int) {
	cs, ce = len(cells), len(cells)
	for i, c := range cells {
		if c.Start < end && start < c.End {
			if i < cs {
				cs = i
			}
			ce = i + 1
		}
	}
	if cs == len(cells) {
		for i, c := range cells {
			if c.Start >= start {
				return i, i
			}
		}
	}
	return cs, ce
}
