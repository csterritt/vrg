package present

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// Cell is one terminal display cell of a presented source line.
type Cell struct {
	// Text is the bytes to emit for this cell: a printable cluster, an
	// escape form, or "" for the trailing cell of a wide cluster, whose
	// first cell's glyph already occupies it.
	Text string
	// Lead marks the first display cell of a grapheme cluster — the
	// only legal wrap boundary. A cluster producing several units (the
	// fallback per-rune escapes of a cluster mixing printable and
	// dangerous forms) leads only on its first unit.
	Lead bool
	// Cont marks a trailing cell of a multi-cell unit — a wide cluster
	// or a multi-cell escape — whose lead cell's text covers it.
	Cont bool
	// Blank marks a substituted filler cell — a wrap boundary's or clip
	// edge's stand-in for a split cluster's unpaintable cells. It is
	// never produced by LineOf; the row and clip layers mark the cells
	// they blank so a covering highlight never styles them as match
	// cells.
	Blank bool
}

// Span is a display-cell range [Start, End) on one presented line. A
// span with Start == End is a zero-width marker position rather than a
// coverage range: it renders as one marked cell at Start without
// shifting following text, extending the line by a cell when Start
// equals the line width.
type Span struct {
	Start, End int
}

// Line is one source line's display-ready form: the escaped text, its
// cells, and the byte→cell map highlight rendering consumes.
type Line struct {
	raw   []byte // original line bytes including any terminator
	text  string // joined cell texts
	cells []Cell
	// lo[i] is the first display cell of the unit producing raw byte i;
	// hi[i] is the cell after its last. Bytes producing no display —
	// terminator bytes — map to the end-of-line position.
	lo, hi []int
}

// Text returns the escaped display text of the line.
func (l Line) Text() string { return l.text }

// Width returns the line's display width in terminal cells.
func (l Line) Width() int { return len(l.cells) }

// Cells returns the line's display cells, sharing the line's storage —
// callers must not mutate them.
func (l Line) Cells() []Cell { return l.cells }

// Raw returns the line's original bytes including any terminator.
func (l Line) Raw() []byte { return l.raw }

// Span maps a raw byte range within the line to its display cell span.
// Ranges are clamped to the line; interior bytes expand the span to the
// whole unit they belong to (a rune's bytes share its cells, an escape's
// source byte covers every cell of the escape). A range covering no
// display cells — a zero-width position or a match solely on removed
// terminator bytes — yields a marker position with Start == End: a
// zero-width match at byte 4 of "hit\r\n" marks display column 3.
func (l Line) Span(start, end int) Span {
	pos := func(off int) int {
		if off < 0 {
			return 0
		}
		if off >= len(l.raw) {
			return len(l.cells)
		}
		return l.lo[off]
	}
	if start < 0 {
		start = 0
	}
	if end > len(l.raw) {
		end = len(l.raw)
	}
	if end < start {
		end = start
	}
	if start == end {
		return Span{pos(start), pos(start)}
	}
	return Span{pos(start), l.hi[end-1]}
}

// LineOf escapes one raw source line — including any trailing LF or
// CRLF terminator — into display cells with a byte→cell map. Invalid
// UTF-8 becomes U+FFFD while retaining its raw-byte mapping; C0
// controls and DEL take caret notation except that LF and CRLF are
// never displayed (terminators map to the end-of-line position) and a
// standalone CR becomes ^M; C1 controls take \uXXXX; tab expands with
// space cells to the next multiple of eight source-display columns as
// one cluster.
func LineOf(raw []byte) Line {
	l := Line{raw: bytes.Clone(raw), lo: make([]int, len(raw)), hi: make([]int, len(raw))}
	var b strings.Builder

	// emit records one unit covering raw bytes [start,end) as width
	// display cells carrying text; lead marks whether the unit begins
	// a new grapheme cluster. A zero-width unit joins the previous
	// cell's text — a combining mark extends its base — or takes a
	// provisional cell of its own at line start on a dotted-circle
	// base, so a standalone invisible cluster is a real painted cell
	// rather than a bare mark the terminal would merge into the cell
	// before it.
	emit := func(start, end int, text string, width int, lead bool) {
		if width <= 0 {
			c := len(l.cells) - 1
			if c < 0 {
				text = "◌" + text
				l.cells = append(l.cells, Cell{Text: text, Lead: lead})
				c = 0
			} else {
				l.cells[c].Text += text
			}
			b.WriteString(text)
			for j := start; j < end; j++ {
				l.lo[j], l.hi[j] = c, c+1
			}
			return
		}
		b.WriteString(text)
		c := len(l.cells)
		l.cells = append(l.cells, Cell{Text: text, Lead: lead})
		for k := 1; k < width; k++ {
			l.cells = append(l.cells, Cell{Cont: true})
		}
		for j := start; j < end; j++ {
			l.lo[j], l.hi[j] = c, c+width
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
				emit(i, i+1, "^M", 2, true)
			case c == '\t':
				// Tab expands to the next multiple of eight
				// source-display columns: one cluster of space cells.
				// The source column is len(l.cells), independent of
				// gutter and horizontal pan.
				n := 8 - len(l.cells)%8
				l.lo[i], l.hi[i] = len(l.cells), len(l.cells)+n
				for k := 0; k < n; k++ {
					l.cells = append(l.cells, Cell{Text: " ", Lead: k == 0})
				}
				b.WriteString(strings.Repeat(" ", n))
			case c < 0x20:
				emit(i, i+1, "^"+string(c+'@'), 2, true)
			case c == 0x7f:
				emit(i, i+1, "^?", 2, true)
			default:
				emit(i, i+1, string(c), 1, true)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			emit(i, i+1, "\ufffd", 1, true)
			i++
			continue
		}
		cl, w := ansi.FirstGraphemeCluster(raw[i:], ansi.GraphemeWidth)
		if len(cl) == size && r >= 0x80 && r < 0xa0 {
			emit(i, i+size, fmt.Sprintf(`\u%04x`, r), 6, true)
		} else if printableCluster(cl) {
			emit(i, i+len(cl), string(cl), w, true)
		} else {
			// A cluster mixing printable and dangerous forms falls back
			// to per-rune rules so no control byte survives verbatim.
			// Only the first emitted unit leads: the whole cluster is
			// still one wrap unit.
			j := 0
			lead := true
			for j < len(cl) {
				r, size := utf8.DecodeRune(cl[j:])
				switch {
				case r == utf8.RuneError && size == 1:
					emit(i+j, i+j+1, "\ufffd", 1, lead)
				case r == '\n':
					// Terminator byte inside a cluster: no display.
				case r == '\r':
					emit(i+j, i+j+size, "^M", 2, lead)
				case r < 0x20 || r == 0x7f:
					emit(i+j, i+j+size, "^"+string(r+'@'), 2, lead)
				case r >= 0x80 && r < 0xa0:
					emit(i+j, i+j+size, fmt.Sprintf(`\u%04x`, r), 6, lead)
				default:
					emit(i+j, i+j+size, string(cl[j:j+size]), ansi.GraphemeWidth.StringWidth(string(cl[j:j+size])), lead)
				}
				lead = false
				j += size
			}
		}
		i += len(cl)
	}

	// Unmapped bytes are removed terminator bytes; they map to the
	// end-of-line position.
	eol := len(l.cells)
	for j := range l.lo {
		if l.hi[j] == 0 {
			l.lo[j], l.hi[j] = eol, eol
		}
	}
	l.text = b.String()
	return l
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
