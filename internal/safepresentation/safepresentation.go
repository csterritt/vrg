// Package safepresentation turns external bytes — searched paths, file
// content, and text embedded in diagnostics — into terminal-safe
// display text. It is the single shared utility every output sink
// routes through: Issue #6 generalized the Issue #5 core and replaced
// the minimal Issue #1 cli.Escape escaper.
//
// The terminal-safety contract is narrow and strong: raw control
// sequences from external data never execute. Original bytes remain the
// key for identity, ordering, and file access — displayed strings never
// become filesystem keys.
package safepresentation

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// EscapePath renders raw path bytes safe for single-line display:
// newline, carriage return, and tab become the two-character escapes
// \n, \r, \t; a literal backslash doubles; other C0 controls and DEL use
// caret notation (ESC is ^[); C1 controls and non-printable runes use
// \u escapes; invalid UTF-8 bytes use \xNN. Valid printable Unicode is
// preserved.
func EscapePath(p []byte) string {
	var b strings.Builder
	b.Grow(len(p))
	for i := 0; i < len(p); {
		c := p[i]
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
			case c < 0x20 || c == 0x7f:
				b.WriteString(caret(c))
			default:
				b.WriteByte(c)
			}
			i++
			continue
		}
		r, size := decodeRune(p[i:])
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&b, `\x%02x`, c)
			i++
			continue
		}
		if isC1(r) || !unicode.IsPrint(r) {
			fmt.Fprintf(&b, `\u%04x`, r)
		} else {
			b.Write(p[i : i+size])
		}
		i += size
	}
	return b.String()
}

// Cell is one terminal display cell of escaped content. Text holds the
// cell's display bytes — empty on a wide glyph's continuation cells.
// Start and End give the half-open source byte range that produced the
// cell, so a highlight over those bytes covers every cell of the
// escaped or measured form.
type Cell struct {
	Text       string
	Start, End int
}

// Cluster is one grapheme cluster's half-open cell range within Cells;
// End-Start is its terminal cell width. Each character of an escaped
// form and each invalid-byte U+FFFD replacement is its own single-cell
// cluster, while a wide glyph or a tab expansion is one multi-cell
// cluster, so wrapping and clipping never split a cluster's cells.
type Cluster struct {
	Start, End int
}

// Mapped is escaped display text plus its byte→cell map: Text is what a
// sink renders, Cells[i] records which source bytes produced cell i,
// and Clusters segments the cells into grapheme-cluster boundaries —
// the single segmentation and cell-width policy every consumer shares.
type Mapped struct {
	Text     string
	Cells    []Cell
	Clusters []Cluster
}

// CellsCovering maps a half-open source byte range to the half-open
// cell range covering every cell produced by those bytes, so a match
// covering an ESC byte highlights both ^ and [. It reports false when no
// cell came from the range — including empty ranges; zero-width match
// markers are Issue #23's.
func (m Mapped) CellsCovering(start, end int) (lo, hi int, ok bool) {
	for i, c := range m.Cells {
		if c.Start < end && c.End > start {
			if !ok {
				lo = i
				ok = true
			}
			hi = i + 1
		}
	}
	return lo, hi, ok
}

// MapContent escapes one content line's raw bytes — without its line
// terminator — into display text plus the per-cell byte map and the
// grapheme-cluster segmentation. C0 controls and DEL render in caret
// notation (ESC is ^[), a standalone CR renders as ^M, a tab expands
// with blank cells to the next multiple of eight source-display columns
// (the structural tab-stop rule), C1 controls and non-printable single
// runes use \u escapes, and invalid UTF-8 bytes render as U+FFFD. LF
// and CRLF never reach this function: line splitting owns terminators.
func MapContent(raw []byte) Mapped {
	var m Mapped
	var b strings.Builder
	b.Grow(len(raw))
	// put appends one display cell holding disp, produced by raw[s:e).
	put := func(disp string, s, e int) {
		b.WriteString(disp)
		m.Cells = append(m.Cells, Cell{Text: disp, Start: s, End: e})
	}
	// cell emits a single-cell cluster holding disp.
	cell := func(disp string, s, e int) {
		put(disp, s, e)
		m.Clusters = append(m.Clusters, Cluster{Start: len(m.Cells) - 1, End: len(m.Cells)})
	}
	// escape emits an ASCII escape form, one single-cell cluster per
	// character.
	escape := func(esc string, s, e int) {
		for i := 0; i < len(esc); i++ {
			cell(esc[i:i+1], s, e)
		}
	}
	// unit emits one w-cell cluster: the text sits on the first cell
	// and continuation cells are blank, all mapping to raw[s:e).
	unit := func(disp string, w, s, e int) {
		if w < 1 {
			w = 1
		}
		start := len(m.Cells)
		put(disp, s, e)
		for i := 1; i < w; i++ {
			put("", s, e)
		}
		m.Clusters = append(m.Clusters, Cluster{Start: start, End: start + w})
	}

	rest := raw
	state := -1
	for len(rest) > 0 {
		cl, r, w, ns := uniseg.FirstGraphemeCluster(rest, state)
		s := len(raw) - len(rest)
		e := s + len(cl)
		state, rest = ns, r
		switch {
		case len(cl) == 1 && cl[0] < utf8.RuneSelf:
			c := cl[0]
			switch {
			case c == '\t':
				// Tabs expand to the next multiple of eight
				// source-display columns — the line's own cell
				// position, so stops never shift with gutter width
				// or horizontal pan. The expansion is one cluster:
				// an unbreakable wrap unit of blank cells.
				tab := 8 - len(m.Cells)%8
				start := len(m.Cells)
				for i := 0; i < tab; i++ {
					put(" ", s, e)
				}
				m.Clusters = append(m.Clusters, Cluster{Start: start, End: start + tab})
			case c < 0x20 || c == 0x7f:
				escape(caret(c), s, e)
			default:
				cell(string(c), s, e)
			}
		case len(cl) == 1 || !utf8.Valid(cl):
			// A lone byte ≥ 0x80 is invalid UTF-8; so is any cluster
			// uniseg could not decode. Each invalid byte becomes one
			// U+FFFD cell retaining its own byte mapping.
			for j := 0; j < len(cl); {
				rr, size := decodeRune(cl[j:])
				if rr == utf8.RuneError && size == 1 {
					cell("", s+j, s+j+1)
					j++
					continue
				}
				unit(string(cl[j:j+size]), uniseg.StringWidth(string(rr)), s+j, s+j+size)
				j += size
			}
		default:
			rr, size := decodeRune(cl)
			switch {
			case size == len(cl) && (isC1(rr) || !unicode.IsPrint(rr)):
				escape(fmt.Sprintf(`\u%04x`, rr), s, e)
			case w < 1:
				// A cluster with no visible cell still needs one
				// reachable cell; ◌ plus the cluster's bytes is the
				// recorded Issue #43 fallback.
				unit("◌"+string(cl), 1, s, e)
			default:
				unit(string(cl), w, s, e)
			}
		}
	}
	m.Text = b.String()
	return m
}

// caret renders a C0 control or DEL in caret notation: '^' + '@'
// shifted, with DEL as ^?.
func caret(c byte) string {
	if c == 0x7f {
		return "^?"
	}
	return string([]byte{'^', c + '@'})
}

// isC1 reports whether r is a C1 control (U+0080–U+009F).
func isC1(r rune) bool { return r >= 0x80 && r < 0xa0 }
