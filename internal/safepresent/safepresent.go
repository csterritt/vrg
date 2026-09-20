package safepresent

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Path escapes raw path bytes into a single safe display line. Newline,
// carriage return, and tab become \n, \r, \t; a literal backslash
// doubles; invalid UTF-8 bytes become \xNN; other C0 controls and DEL
// use caret notation; C1 controls use \uXXXX escapes. Valid printable
// text — including multi-byte Unicode — passes through. The result is
// presentation only: raw bytes remain the identity and filesystem key.
func Path(raw []byte) string {
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

// Cell is one terminal display cell of escaped file content. Text is the
// cell's display string; Start and End are the half-open source-line byte
// range the cell presents, so a byte-range highlight can cover every cell
// of an escaped form. The mapping is provisional for the tab placeholder
// until Issue 16 lands structural tab expansion.
type Cell struct {
	Text       string
	Start, End int
}

// Content escapes one source line's raw bytes — its line terminator
// already removed — into display cells. C0 controls and DEL use caret
// notation (ESC renders as ^[), so a standalone carriage return renders
// as ^M; a tab renders as the provisional single-cell arrow placeholder;
// C1 controls use \uXXXX escapes; invalid UTF-8 bytes each become one
// U+FFFD cell retaining their raw-byte mapping; printable text passes
// through.
func Content(raw []byte) []Cell {
	var cells []Cell
	emit := func(text string, start, end int) {
		for _, r := range text {
			cells = append(cells, Cell{Text: string(r), Start: start, End: end})
		}
	}
	for i := 0; i < len(raw); {
		c := raw[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '\t':
				emit("→", i, i+1)
			case c < 0x20:
				emit(caret(c), i, i+1)
			case c == 0x7f:
				emit("^?", i, i+1)
			default:
				emit(string(c), i, i+1)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			emit("\uFFFD", i, i+1)
			i++
			continue
		}
		if r >= 0x80 && r < 0xa0 {
			emit(fmt.Sprintf(`\u%04x`, r), i, i+size)
		} else {
			emit(string(raw[i:i+size]), i, i+size)
		}
		i += size
	}
	return cells
}

// caret returns the caret-notation form of a C0 control byte.
func caret(c byte) string {
	return string([]byte{'^', c + '@'})
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
