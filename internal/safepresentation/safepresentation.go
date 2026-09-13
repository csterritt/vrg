package safepresentation

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// PathDisplay is the escaped display form of a raw path and its
// byte→cell mapping. ByteCells[i] is the [start, end) display cell
// range occupied by original byte i.
type PathDisplay struct {
	Text      string
	ByteCells [][2]int
}

// ContentDisplay is the escaped display form of raw content bytes and
// its byte→cell mapping. ByteCells[i] is the [start, end) display cell
// range occupied by original byte i. Line-terminator bytes (LF, CRLF)
// produce no display cells and map to the end-of-line position.
type ContentDisplay struct {
	Text      string
	ByteCells [][2]int
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
// as ^M. Tab renders as a single → placeholder cell (provisional until
// Issue #16's eight-column-stop expansion). Other C0 controls and DEL
// use caret notation. C1 controls use \u00XX escapes. Invalid UTF-8
// renders as U+FFFD while the byte→cell map retains the raw-byte
// mapping. Valid printable Unicode is preserved. The byte→cell map
// records the display cell range for each original byte so highlight
// rendering can cover all cells of an escaped form.
func EscapeContent(raw []byte) ContentDisplay {
	var b strings.Builder
	b.Grow(len(raw))
	cells := make([][2]int, 0, len(raw))
	cell := 0
	for i := 0; i < len(raw); {
		c := raw[i]
		if c == '\r' && i+1 < len(raw) && raw[i+1] == '\n' {
			cells = append(cells, [2]int{cell, cell})
			cells = append(cells, [2]int{cell, cell})
			i += 2
			continue
		}
		if c == '\n' {
			cells = append(cells, [2]int{cell, cell})
			i++
			continue
		}
		if c < utf8.RuneSelf {
			start := cell
			switch {
			case c == '\r':
				b.WriteString("^M")
				cell += 2
			case c == '\t':
				b.WriteString("→")
				cell++
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
			b.WriteRune('\ufffd')
			cells = append(cells, [2]int{cell, cell + 1})
			cell++
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
	return ContentDisplay{Text: b.String(), ByteCells: cells}
}
