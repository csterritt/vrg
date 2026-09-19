// Package filebuffer loads and classifies one file and turns content plus
// original match data into safe, display-ready source lines and validated
// highlights.
//
// Issue #5 lands the first path: byte loading, line splitting on LF/CRLF
// with the unterminated-final-line and empty-file rules, display mapping
// through the safe-presentation core, and highlight spans mapped through
// each line's byte→cell map. Issue #16 makes the buffer the source of
// the shared grapheme policy: every line's Clusters segment its display
// cells at grapheme-cluster boundaries (tabs already expanded to their
// eight-column stops) so Viewport wraps without re-deriving. Stale-match
// validation is Issue #29's and unsupported encodings are Issue #30's.
package filebuffer

import (
	"os"

	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// Line is one source line prepared for display: the escaped Text with
// its byte→cell map and grapheme-cluster segmentation (embedded Mapped),
// the original Raw bytes — terminator included — retained for identity
// and later validation, and the matched spans as display-cell ranges in
// Highlights — each expanded outward to whole grapheme clusters, the
// single span source Viewport and App consume (Issue #21).
type Line struct {
	safepresentation.Mapped
	Number     int64
	Raw        []byte
	Highlights []searchindex.Span
}

// Buffer is one loaded file's display-ready content.
type Buffer struct {
	lines []Line
}

// Load reads path — the raw bytes are the filesystem key, never the
// escaped display form — splits the content into source lines, maps each
// line's content through the safe-presentation core, and maps each
// stop's recorded byte-range highlights onto display cells. All reading,
// decoding, and mapping happens here so the caller's completion message
// carries a prepared buffer and its update path does no full-file work.
func Load(path []byte, stops []searchindex.Stop) (*Buffer, error) {
	raw, err := os.ReadFile(string(path))
	if err != nil {
		return nil, err
	}
	byLine := make(map[int64][]searchindex.Span)
	for _, st := range stops {
		byLine[st.Number] = append(byLine[st.Number], st.Highlights...)
	}
	b := &Buffer{}
	for start := 0; start < len(raw); {
		end := start
		for end < len(raw) && raw[end] != '\n' {
			end++
		}
		if end < len(raw) {
			end++ // the line keeps its \n terminator
		}
		b.lines = append(b.lines, makeLine(raw[start:end], int64(len(b.lines))+1, byLine))
		start = end
	}
	return b, nil
}

// LineCount is the number of source lines: an empty file has zero, a
// missing final newline still yields the last line, and a trailing
// newline does not invent an empty line.
func (b *Buffer) LineCount() int { return len(b.lines) }

// Lines returns the prepared source lines in file order.
func (b *Buffer) Lines() []Line { return b.lines }

// GutterWidth is the decimal digit width of the largest line number plus
// the two trailing spaces, with a one-digit-slot minimum for empty or
// placeholder views.
func (b *Buffer) GutterWidth() int {
	digits := 1
	for n := len(b.lines); n >= 10; n /= 10 {
		digits++
	}
	return digits + 2
}

// MaxStart is the largest display-cell index where one of the line's
// grapheme clusters begins and fits entirely within width cells — the
// paintable boundary (Issue #18) a horizontal offset may reach so at
// least one whole cluster still paints. A trailing cluster wider than
// width is skipped, falling back to the last fitting cluster's start,
// and a line with no fitting cluster reports 0. Issue #23's
// end-of-line marker will join the candidate set once it exists.
func (l Line) MaxStart(width int) int {
	max := 0
	for _, cl := range l.Clusters {
		if w := cl.End - cl.Start; w >= 1 && w <= width && cl.Start > max {
			max = cl.Start
		}
	}
	return max
}

// makeLine prepares one raw line — terminator included — for display:
// the content bytes are mapped through the safe-presentation core and
// the line's recorded highlight byte ranges are mapped onto cells.
func makeLine(raw []byte, number int64, byLine map[int64][]searchindex.Span) Line {
	content := raw
	if n := len(content); n > 0 && content[n-1] == '\n' {
		content = content[:n-1]
		// A \r immediately before the \n is half of the CRLF terminator;
		// a standalone CR stays content and escapes as ^M.
		if n := len(content); n > 0 && content[n-1] == '\r' {
			content = content[:n-1]
		}
	}
	l := Line{Mapped: safepresentation.MapContent(content), Number: number, Raw: raw}
	for _, sp := range byLine[number] {
		if lo, hi, ok := l.CellsCovering(sp.Start, sp.End); ok {
			lo, hi = l.expandToClusters(lo, hi)
			l.Highlights = append(l.Highlights, searchindex.Span{Start: lo, End: hi})
		}
	}
	return l
}

// expandToClusters snaps a nonempty mapped cell range outward to
// whole grapheme clusters: a span touching any cell of a cluster
// covers that cluster entirely (Issue #21), so a partial-cluster
// match — a combining-only match or a span starting or ending inside
// a wide glyph — highlights the whole cluster and never splits it.
// The recorded spans are the single highlight source Viewport and App
// consume for painting, reveal targets, and hidden-content
// indicators.
func (l Line) expandToClusters(lo, hi int) (int, int) {
	for _, cl := range l.Clusters {
		if lo > cl.Start && lo < cl.End {
			lo = cl.Start
		}
		if hi > cl.Start && hi < cl.End {
			hi = cl.End
		}
	}
	return lo, hi
}
