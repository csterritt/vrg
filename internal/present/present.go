package present

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// Path renders raw path bytes safe for single-line display — the
// canonical filename contract for every sink. Newline, carriage return,
// and tab become \n, \r, \t; backslash doubles; invalid UTF-8 bytes
// become \xNN; other C0 controls and DEL take caret notation; C1
// controls take \uXXXX; valid printable Unicode passes through. The raw
// bytes — never this display form — remain the key for identity,
// ordering, and file access.
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
				b.WriteByte('^')
				b.WriteByte(c + '@')
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

// Diagnostic renders diagnostic text safe for terminal output while
// preserving the message's own line structure: LF is a real line
// boundary and CRLF counts as one boundary; a standalone CR takes ^M;
// tabs expand to the next multiple of eight display columns; other C0
// controls and DEL take caret notation; C1 controls take \uXXXX;
// invalid UTF-8 bytes take \xNN. Printable text — including backslash —
// passes through, so a filename embedded via Path keeps its single-line
// escaped form intact and can never become a diagnostic paragraph
// break.
func Diagnostic(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	col := 0 // display column within the current diagnostic line
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '\n':
				b.WriteByte('\n')
				col = 0
			case c == '\r' && i+1 < len(s) && s[i+1] == '\n':
				b.WriteByte('\n')
				col = 0
				i++
			case c == '\r':
				b.WriteString("^M")
				col += 2
			case c == '\t':
				n := 8 - col%8
				b.WriteString(strings.Repeat(" ", n))
				col += n
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
			col += ansi.StringWidth(s[i : i+size])
		}
		i += size
	}
	return b.String()
}
