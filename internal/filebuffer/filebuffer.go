package filebuffer

import (
	"bytes"
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

// utf8BOM is the encoding signature ripgrep removes from the searched
// view of a file's first line under its default detection.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Buffer is a prepared file: display-ready source lines with their
// grapheme-cluster layout plus highlight spans. Three coordinate views
// stay separate throughout: the retained raw line bytes — terminators
// and a leading UTF-8 BOM included — that LineBytes exposes; the
// rg-line byte offsets stop coverage carries, which omit the BOM's
// three bytes on the first line; and the display cells both map onto.
// Load performs the whole read, decode, and byte→cell mapping so the
// caller's update path does no full-file work — the prepared Buffer
// travels inside the load-completion message.
type Buffer struct {
	lines    [][]safepresentation.Cell
	clusters [][]safepresentation.Cluster
	raw      [][]byte
	spans    map[int][]Span
	bom      int
}

// Load reads path — the raw resolved path bytes, never a display string
// — and prepares it for display: source lines split on LF and CRLF with
// their original bytes retained, a leading UTF-8 BOM removed from the
// first line's display view, each line's content escaped into display
// cells carrying raw-file byte offsets, and every stop's rg-line
// coverage ranges shifted into raw offsets and mapped onto those cells.
// stops are the file's matched-line entries; stops whose line is out of
// range contribute nothing (stale match validation is Issue 29's).
func Load(path []byte, stops []searchindex.Stop) (*Buffer, error) {
	raw, err := os.ReadFile(string(path))
	if err != nil {
		return nil, err
	}
	b := &Buffer{spans: make(map[int][]Span)}
	if bytes.HasPrefix(raw, utf8BOM) {
		b.bom = len(utf8BOM)
	}
	for i, line := range splitLines(raw) {
		b.raw = append(b.raw, line)
		content, base := lineContent(line, i == 0 && b.bom > 0)
		cells, clusters := safepresentation.EscapeContent(content)
		for j := range cells {
			cells[j].Start += base
			cells[j].End += base
		}
		b.lines = append(b.lines, cells)
		b.clusters = append(b.clusters, clusters)
	}
	for _, s := range stops {
		i := int(s.Line) - 1
		if i < 0 || i >= len(b.lines) {
			continue
		}
		for _, r := range s.Coverage {
			cs, ce := safepresentation.Span(b.lines[i],
				b.rawOffset(i, r.Start), b.rawOffset(i, r.End))
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

// splitLines divides raw bytes into source lines, each retaining its
// original bytes including the terminator: LF ends a line, a CR
// immediately before LF is part of the terminator, a standalone CR
// stays ordinary content, an unterminated final line counts, a
// trailing newline adds no empty line, and empty input yields no lines.
// The returned slices share the input's backing array.
func splitLines(raw []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\n' {
			continue
		}
		lines = append(lines, raw[start:i+1])
		start = i + 1
	}
	if start < len(raw) {
		lines = append(lines, raw[start:])
	}
	return lines
}

// lineContent returns the line's displayable bytes and their base
// offset within the retained raw line. A trailing LF — and the CR of a
// CRLF — is part of the terminator and leaves the display view; when
// bom is set the line opens with a UTF-8 BOM, which is removed from
// the display view but keeps its three bytes in the raw view, so the
// content's first byte sits at raw offset 3.
func lineContent(line []byte, bom bool) (content []byte, base int) {
	content = line
	if n := len(content); n > 0 && content[n-1] == '\n' {
		content = content[:n-1]
		if n := len(content); n > 0 && content[n-1] == '\r' {
			content = content[:n-1]
		}
	}
	if bom {
		base, content = len(utf8BOM), content[len(utf8BOM):]
	}
	return content, base
}

// rawOffset maps a byte offset in rg's view of a line to the same
// position in the line's retained raw bytes. The views differ only on
// the first line of a UTF-8-BOM file, where rg's searched line data
// omits the BOM's three bytes.
func (b *Buffer) rawOffset(line, off int) int {
	if line == 0 {
		return off + b.bom
	}
	return off
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

// TargetCell returns the stop's display target: its 0-based source
// line clamped to the buffer's lines and the start cell of its first
// coverage range — the cell whose rendered row a reveal must find.
// Taking the whole stop rather than a bare line ordinal is what lets
// wrap mode move the target onto a later row of a tall line. A stop
// without coverage targets the line's first cell; coverage reaching
// past the line's cells maps to the insertion point at the line end.
func (b *Buffer) TargetCell(stop searchindex.Stop) (line, cell int) {
	line = int(stop.Line) - 1
	if last := len(b.lines) - 1; line > last {
		line = last
	}
	if line < 0 {
		line = 0
	}
	if len(b.lines) == 0 || len(stop.Coverage) == 0 {
		return line, 0
	}
	cs, _ := safepresentation.Span(b.lines[line],
		b.rawOffset(line, stop.Coverage[0].Start), b.rawOffset(line, stop.Coverage[0].End))
	return line, cs
}

// LineCount is the number of source lines in the loaded file.
func (b *Buffer) LineCount() int { return len(b.lines) }

// LineBytes returns the retained raw file bytes of 0-based source line
// i — content plus line terminator, plus the leading UTF-8 BOM on line
// 0 — or nil when i is out of range. This raw-file coordinate view is
// the byte space the display cells' Start/End offsets map into and the
// comparison view Issue 29's stale-match validation checks recorded
// submatch bytes against; rg coverage offsets reach it through
// rawOffset. The slice is owned by the buffer; do not mutate.
func (b *Buffer) LineBytes(i int) []byte {
	if i < 0 || i >= len(b.raw) {
		return nil
	}
	return b.raw[i]
}

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

// Clusters returns the grapheme clusters of 0-based source line i —
// each cluster's half-open cell range and terminal cell width — or nil
// when i is out of range. This is the one grapheme segmentation and
// cell-width policy the viewport wraps and clips by; it never
// re-derives boundaries. The slice is owned by the buffer; do not
// mutate.
func (b *Buffer) Clusters(i int) []safepresentation.Cluster {
	if i < 0 || i >= len(b.clusters) {
		return nil
	}
	return b.clusters[i]
}

// Highlights returns the sorted inverse-video cell spans of 0-based
// source line i, or nil when the line has none.
func (b *Buffer) Highlights(i int) []Span { return b.spans[i] }
