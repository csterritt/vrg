package safepresentation

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// EscapeDiagnostic renders diagnostic text safe for output. Real
// diagnostic line boundaries are preserved — LF stays and CRLF
// normalizes to LF — tabs expand to the next multiple of eight display
// columns, other C0 controls and DEL use caret notation (a standalone
// CR is ^M), C1 controls and non-printable runes use \u escapes, and
// invalid UTF-8 bytes use \xNN. A backslash is literal: a diagnostic is
// not a path.
//
// Any external string embedded in a diagnostic — a filename, an error
// message — must be escaped first with EscapePath so its bytes can
// never forge a diagnostic line boundary.
func EscapeDiagnostic(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	col := 0 // display cells emitted on the current line
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
					i++ // consume the LF half of CRLF
					col = 0
				} else {
					b.WriteString(caret(c)) // a standalone CR is ^M
					col += 2
				}
			case c == '\t':
				n := 8 - col%8
				b.WriteString(strings.Repeat(" ", n))
				col += n
			case c < 0x20 || c == 0x7f:
				esc := caret(c)
				b.WriteString(esc)
				col += len(esc)
			default:
				b.WriteByte(c)
				col++
			}
			i++
			continue
		}
		r, size := decodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&b, `\x%02x`, c)
			col += 4
			i++
			continue
		}
		if isC1(r) || !unicode.IsPrint(r) {
			esc := fmt.Sprintf(`\u%04x`, r)
			b.WriteString(esc)
			col += len(esc)
		} else {
			b.WriteString(s[i : i+size])
			col += CellWidth(s[i : i+size])
		}
		i += size
	}
	return b.String()
}
