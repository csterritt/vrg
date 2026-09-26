package filebuffer

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// EscapePath renders raw path bytes safe for single-line display.
// Newline, carriage return, and tab become \n, \r, \t; backslash
// doubles; invalid UTF-8 bytes become \xNN; other C0 controls and DEL
// take caret notation; C1 controls take \uXXXX; valid printable Unicode
// passes through. The raw bytes — never this display form — remain the
// key for identity, ordering, and file access. Issue #6 unifies this
// with the Issue #1 cli.Escape escaper as the shared all-sink utility.
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

// Cell is one terminal display cell of a presented source line.
type Cell struct {
	// Text is the bytes to emit for this cell: a printable cluster, an
	// escape form, or "" for the trailing cell of a wide cluster, whose
	// first cell's glyph already occupies it.
	Text string
	// Cont marks the trailing cell of a two-cell cluster.
	Cont bool
}

// Span is a display-cell range [Start, End) on one presented line. A
// span with Start == End is a zero-width marker position rather than a
// coverage range: it renders as one marked cell at Start without
// shifting following text, extending the line by a cell when Start
// equals the line width.
type Span struct {
	Start, End int
}

// presented is one source line's display-ready form: the escaped text,
// its cells, and the byte→cell map highlight rendering consumes.
type presented struct {
	raw   []byte // original line bytes including any terminator
	text  string // joined cell texts
	cells []Cell
	// lo[i] is the first display cell of the unit producing raw byte i;
	// hi[i] is the cell after its last. Bytes producing no display —
	// terminator bytes — map to the end-of-line position.
	lo, hi []int
}

// Text returns the escaped display text of the line.
func (p presented) Text() string { return p.text }

// Width returns the line's display width in terminal cells.
func (p presented) Width() int { return len(p.cells) }

// Raw returns the line's original bytes including any terminator.
func (p presented) Raw() []byte { return p.raw }

// Span maps a raw byte range within the line to its display cell span.
// Ranges are clamped to the line; interior bytes expand the span to the
// whole unit they belong to (a rune's bytes share its cells, an escape's
// source byte covers every cell of the escape). A range covering no
// display cells — a zero-width position or a match solely on removed
// terminator bytes — yields a marker position with Start == End: a
// zero-width match at byte 4 of "hit\r\n" marks display column 3.
func (p presented) Span(start, end int) Span {
	pos := func(off int) int {
		if off < 0 {
			return 0
		}
		if off >= len(p.raw) {
			return len(p.cells)
		}
		return p.lo[off]
	}
	if start < 0 {
		start = 0
	}
	if end > len(p.raw) {
		end = len(p.raw)
	}
	if end < start {
		end = start
	}
	if start == end {
		return Span{pos(start), pos(start)}
	}
	return Span{pos(start), p.hi[end-1]}
}

// presentLine escapes one raw source line — including any trailing LF
// or CRLF terminator — into display cells with a byte→cell map. Invalid
// UTF-8 becomes U+FFFD while retaining its raw-byte mapping; C0
// controls and DEL take caret notation except that LF and CRLF are
// never displayed (terminators map to the end-of-line position) and a
// standalone CR becomes ^M; C1 controls take \uXXXX; tab renders as the
// provisional single-cell → placeholder pending Issue 16's stop
// expansion.
func presentLine(raw []byte) presented {
	p := presented{raw: bytes.Clone(raw), lo: make([]int, len(raw)), hi: make([]int, len(raw))}
	var b strings.Builder

	// emit records one unit covering raw bytes [start,end) as width
	// display cells carrying text. A zero-width unit joins the previous
	// cell's text — a combining mark extends its base — or takes a
	// provisional cell of its own at line start.
	emit := func(start, end int, text string, width int) {
		b.WriteString(text)
		if width <= 0 {
			c := len(p.cells) - 1
			if c < 0 {
				p.cells = append(p.cells, Cell{Text: text})
				c = 0
			} else {
				p.cells[c].Text += text
			}
			for j := start; j < end; j++ {
				p.lo[j], p.hi[j] = c, c+1
			}
			return
		}
		c := len(p.cells)
		p.cells = append(p.cells, Cell{Text: text})
		for k := 1; k < width; k++ {
			p.cells = append(p.cells, Cell{Cont: true})
		}
		for j := start; j < end; j++ {
			p.lo[j], p.hi[j] = c, c+width
		}
	}

	i := 0
	for i < len(raw) {
		c := raw[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '\n':
				// Line terminator: no display, maps to end of line.
			case c == '\r' && i+1 < len(raw) && raw[i+1] == '\n':
				i++ // CRLF terminator: both bytes map to end of line.
			case c == '\r':
				emit(i, i+1, "^M", 2)
			case c == '\t':
				emit(i, i+1, "→", 1)
			case c < 0x20:
				emit(i, i+1, "^"+string(c+'@'), 2)
			case c == 0x7f:
				emit(i, i+1, "^?", 2)
			default:
				emit(i, i+1, string(c), 1)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			emit(i, i+1, "\ufffd", 1)
			i++
			continue
		}
		cl, w := ansi.FirstGraphemeCluster(raw[i:], ansi.GraphemeWidth)
		if len(cl) == size && r >= 0x80 && r < 0xa0 {
			emit(i, i+size, fmt.Sprintf(`\u%04x`, r), 6)
		} else if printableCluster(cl) {
			emit(i, i+len(cl), string(cl), w)
		} else {
			// A cluster mixing printable and dangerous forms falls back
			// to per-rune rules so no control byte survives verbatim.
			j := 0
			for j < len(cl) {
				r, size := utf8.DecodeRune(cl[j:])
				switch {
				case r == utf8.RuneError && size == 1:
					emit(i+j, i+j+1, "\ufffd", 1)
				case r == '\n':
					// Terminator byte inside a cluster: no display.
				case r == '\r':
					emit(i+j, i+j+size, "^M", 2)
				case r < 0x20 || r == 0x7f:
					emit(i+j, i+j+size, "^"+string(r+'@'), 2)
				case r >= 0x80 && r < 0xa0:
					emit(i+j, i+j+size, fmt.Sprintf(`\u%04x`, r), 6)
				default:
					emit(i+j, i+j+size, string(cl[j:j+size]), ansi.GraphemeWidth.StringWidth(string(cl[j:j+size])))
				}
				j += size
			}
		}
		i += len(cl)
	}

	// Unmapped bytes are removed terminator bytes; they map to the
	// end-of-line position.
	eol := len(p.cells)
	for j := range p.lo {
		if p.hi[j] == 0 {
			p.lo[j], p.hi[j] = eol, eol
		}
	}
	p.text = b.String()
	return p
}

// printableCluster reports whether every rune in cl is valid printable
// UTF-8 — no controls, no invalid bytes — so the cluster can be emitted
// as one unit at its grapheme width.
func printableCluster(cl []byte) bool {
	for i := 0; i < len(cl); {
		r, size := utf8.DecodeRune(cl[i:])
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return false
		}
		i += size
	}
	return true
}
