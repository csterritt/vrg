package filebuffer

import (
	"os"
	"sort"

	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// Span is a half-open display-cell range on one source line to render in
// inverse video.
type Span struct {
	Start, End int
}

// Buffer is a prepared file: display-ready source lines plus highlight
// spans. Load performs the whole read, decode, and byte→cell mapping so
// the caller's update path does no full-file work — the prepared Buffer
// travels inside the load-completion message.
type Buffer struct {
	lines [][]safepresentation.Cell
	spans map[int][]Span
}

// Load reads path — the raw resolved path bytes, never a display string
// — and prepares it for display: source lines split on LF and CRLF, each
// line's content escaped into display cells, and every stop's coverage
// ranges mapped onto those cells. stops are the file's matched-line
// entries; stops whose line is out of range contribute nothing (stale
// match validation is Issue 29's).
func Load(path []byte, stops []searchindex.Stop) (*Buffer, error) {
	raw, err := os.ReadFile(string(path))
	if err != nil {
		return nil, err
	}
	b := &Buffer{spans: make(map[int][]Span)}
	for _, line := range splitLines(raw) {
		b.lines = append(b.lines, safepresentation.EscapeContent(line))
	}
	for _, s := range stops {
		i := int(s.Line) - 1
		if i < 0 || i >= len(b.lines) {
			continue
		}
		for _, r := range s.Coverage {
			cs, ce := safepresentation.Span(b.lines[i], r.Start, r.End)
			if cs < ce {
				b.spans[i] = append(b.spans[i], Span{Start: cs, End: ce})
			}
		}
	}
	for i, spans := range b.spans {
		sort.Slice(spans, func(a, c int) bool { return spans[a].Start < spans[c].Start })
		b.spans[i] = mergeSpans(spans)
	}
	return b, nil
}

// splitLines divides raw bytes into source lines: LF terminates a line,
// a CR immediately before LF is part of the terminator, a standalone CR
// stays in the line's bytes, an unterminated final line counts, a
// trailing newline adds no empty line, and empty input yields no lines.
func splitLines(raw []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\n' {
			continue
		}
		line := raw[start:i]
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
		lines = append(lines, line)
		start = i + 1
	}
	if start < len(raw) {
		lines = append(lines, raw[start:])
	}
	return lines
}

// mergeSpans coalesces sorted overlapping or abutting spans.
func mergeSpans(spans []Span) []Span {
	out := spans[:0]
	for _, s := range spans {
		if n := len(out); n > 0 && s.Start <= out[n-1].End {
			if s.End > out[n-1].End {
				out[n-1].End = s.End
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

// LineCount is the number of source lines in the loaded file.
func (b *Buffer) LineCount() int { return len(b.lines) }

// GutterWidth is the file-panel gutter width: the decimal digit width of
// the largest line number plus two spaces, with at least one digit slot.
func (b *Buffer) GutterWidth() int {
	d := 1
	for n := len(b.lines); n >= 10; n /= 10 {
		d++
	}
	return d + 2
}

// Cells returns the display cells of 0-based source line i, or nil when
// i is out of range. The slice is owned by the buffer; do not mutate.
func (b *Buffer) Cells(i int) []safepresentation.Cell {
	if i < 0 || i >= len(b.lines) {
		return nil
	}
	return b.lines[i]
}

// Highlights returns the sorted inverse-video cell spans of 0-based
// source line i, or nil when the line has none.
func (b *Buffer) Highlights(i int) []Span { return b.spans[i] }
