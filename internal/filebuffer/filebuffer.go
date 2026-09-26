package filebuffer

import (
	"bytes"
	"os"
	"strconv"

	"vrg/internal/searchindex"
)

// Buffer is one file's prepared, display-ready content: escaped source
// lines with byte→cell maps and the validated highlight spans of its
// matched lines. Callers own scheduling, caching, and notification; a
// Buffer is immutable after Load returns.
type Buffer struct {
	lines  []presented
	spans  map[int][]Span // 0-based source line → display-cell spans
	digits int            // gutter digit width, minimum 1
}

// Load reads path and prepares its display-ready content and validated
// highlights. The whole read, split, escape, and map happens here — the
// caller delivers the finished Buffer as its completion message so no
// full-file work lands on the UI update path.
//
// Each stop's submatches are checked against the line's raw bytes:
// out-of-bounds ranges and text mismatches are dropped. Stale marking
// is Issue #29's; UTF-16/32 classification is Issue #30's; UTF-8 BOM
// adjustment is Issue #22's.
func Load(path []byte, stops []searchindex.Stop) (*Buffer, error) {
	data, err := os.ReadFile(string(path))
	if err != nil {
		return nil, err
	}
	b := &Buffer{spans: make(map[int][]Span)}
	for rest := data; len(rest) > 0; {
		i := bytes.IndexByte(rest, '\n')
		if i < 0 {
			b.lines = append(b.lines, presentLine(rest))
			break
		}
		b.lines = append(b.lines, presentLine(rest[:i+1]))
		rest = rest[i+1:]
	}
	b.digits = len(strconv.Itoa(max(len(b.lines), 1)))
	for _, st := range stops {
		li := int(st.Line) - 1
		if li < 0 || li >= len(b.lines) {
			continue
		}
		ln := b.lines[li]
		for _, sm := range st.Submatches {
			if sm.Start < 0 || sm.End > len(ln.raw) || sm.Start > sm.End {
				continue
			}
			if !bytes.Equal(ln.raw[sm.Start:sm.End], sm.Bytes) {
				continue
			}
			b.spans[li] = append(b.spans[li], ln.Span(sm.Start, sm.End))
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
func (b *Buffer) Cells(i int) []Cell { return b.lines[i].cells }

// Spans returns the validated highlight spans — display-cell ranges or
// marker positions — of 0-based source line i.
func (b *Buffer) Spans(i int) []Span { return b.spans[i] }
