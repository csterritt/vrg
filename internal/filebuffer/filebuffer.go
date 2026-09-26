package filebuffer

import (
	"bytes"
	"os"
	"strconv"

	"vrg/internal/present"
	"vrg/internal/searchindex"
)

// Buffer is one file's prepared, display-ready content: escaped source
// lines with byte→cell maps and the validated highlight spans of its
// matched lines. Callers own scheduling, caching, and notification; a
// Buffer is immutable after Load returns.
type Buffer struct {
	lines  []present.Line
	spans  map[int][]present.Span // 0-based source line → display-cell spans
	digits int                    // gutter digit width, minimum 1
}

// utf8BOM is the UTF-8 byte order mark: invisible at the start of a
// file, where rg removes its three bytes from first-line data.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Load reads path and prepares its display-ready content and validated
// highlights. The whole read, split, escape, and map happens here — the
// caller delivers the finished Buffer as its completion message so no
// full-file work lands on the UI update path.
//
// Each stop's submatches are checked against the line's raw bytes:
// out-of-bounds ranges and text mismatches are dropped. A leading
// UTF-8 BOM splits the first line's coordinate views: its raw bytes
// retain the BOM while rg's first-line data omits it, so rg offsets on
// that line shift by its length into the raw view before validating
// and mapping. Stale marking is Issue #29's; UTF-16/32 classification
// is Issue #30's.
func Load(path []byte, stops []searchindex.Stop) (*Buffer, error) {
	data, err := os.ReadFile(string(path))
	if err != nil {
		return nil, err
	}
	b := &Buffer{spans: make(map[int][]present.Span)}
	bom := 0
	if bytes.HasPrefix(data, utf8BOM) {
		bom = len(utf8BOM)
	}
	for rest := data; len(rest) > 0; {
		// Only the file's first line can carry the BOM: LineOfBOM
		// keeps its bytes in the raw view while painting nothing.
		escape := present.LineOf
		if bom > 0 && len(b.lines) == 0 {
			escape = present.LineOfBOM
		}
		i := bytes.IndexByte(rest, '\n')
		if i < 0 {
			b.lines = append(b.lines, escape(rest))
			break
		}
		b.lines = append(b.lines, escape(rest[:i+1]))
		rest = rest[i+1:]
	}
	b.digits = len(strconv.Itoa(max(len(b.lines), 1)))
	for _, st := range stops {
		li := int(st.Line) - 1
		if li < 0 || li >= len(b.lines) {
			continue
		}
		ln := b.lines[li]
		adj := 0
		if li == 0 {
			adj = bom
		}
		for _, sm := range st.Submatches {
			start, end := sm.Start+adj, sm.End+adj
			if start < 0 || end > len(ln.Raw()) || start > end {
				continue
			}
			if !bytes.Equal(ln.Raw()[start:end], sm.Bytes) {
				continue
			}
			b.spans[li] = append(b.spans[li],
				clusterSpan(ln.Cells(), ln.Span(start, end)))
		}
	}
	return b, nil
}

// LineCount returns the number of source lines; an empty file has zero.
func (b *Buffer) LineCount() int { return len(b.lines) }

// GutterWidth returns the line-number gutter width in cells: the digit
// width of the largest line number plus two trailing spaces, with a
// minimum one-digit slot.
func (b *Buffer) GutterWidth() int { return b.digits + 2 }

// GutterDigits returns the digit width alone, for layout arithmetic.
func (b *Buffer) GutterDigits() int { return b.digits }

// Text returns the escaped display text of 0-based source line i.
func (b *Buffer) Text(i int) string { return b.lines[i].Text() }

// Cells returns the display cells of 0-based source line i.
func (b *Buffer) Cells(i int) []present.Cell { return b.lines[i].Cells() }

// Spans returns the validated highlight spans — display-cell ranges or
// marker positions — of 0-based source line i. Coverage spans are
// cluster-expanded: they are the single span source for highlighting,
// reveal, and the hidden-match indicators downstream.
func (b *Buffer) Spans(i int) []present.Span { return b.spans[i] }

// clusterSpan expands a nonempty span's endpoints outward to the
// grapheme-cluster boundaries the cells' Lead marks carry, so a
// recorded submatch landing inside a cluster highlights the whole
// cluster — a combining mark's bytes alone highlight the base glyph,
// and a wide pair or multi-cell escape is never split by a highlight
// boundary. Marker positions (Start == End) are already cell-precise
// and pass through.
func clusterSpan(cells []present.Cell, s present.Span) present.Span {
	if s.Start == s.End {
		return s
	}
	for s.Start > 0 && s.Start < len(cells) && !cells[s.Start].Lead {
		s.Start--
	}
	for s.End < len(cells) && !cells[s.End].Lead {
		s.End++
	}
	return s
}
