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
	// bom is the leading UTF-8 BOM's byte length — 3 when present —
	// the shift rg-line offsets on the first line take into the raw
	// view for validation and the stale fallback's recorded start.
	bom int
	// stale marks a buffer whose recorded submatches did not all
	// validate — the "file changed since search" state (Issue #29).
	stale bool
	// enc names the encoding a leading UTF-16/UTF-32 byte-order mark
	// declared — "UTF-16 LE", "UTF-16 BE", "UTF-32 LE", or
	// "UTF-32 BE" — when the buffer is the "(unsupported encoding)"
	// placeholder: no lines, no spans, no stale verdict (Issue #30).
	enc string
}

// utf8BOM is the UTF-8 byte order mark: invisible at the start of a
// file, where rg removes its three bytes from first-line data.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// unsupportedBOMs are the byte-order marks vrg detects but does not
// decode, paired with the encoding names their diagnostics carry.
// Longer marks precede the shorter ones they overlap: UTF-32 LE's
// FF FE 00 00 opens with UTF-16 LE's own bytes, so checking UTF-16
// first would misclassify it.
var unsupportedBOMs = []struct {
	mark []byte
	name string
}{
	{[]byte{0xFF, 0xFE, 0x00, 0x00}, "UTF-32 LE"},
	{[]byte{0x00, 0x00, 0xFE, 0xFF}, "UTF-32 BE"},
	{[]byte{0xFF, 0xFE}, "UTF-16 LE"},
	{[]byte{0xFE, 0xFF}, "UTF-16 BE"},
}

// unsupportedEncoding returns the encoding name a leading UTF-16 or
// UTF-32 byte-order mark declares, "" when the bytes open with no
// unsupported mark.
func unsupportedEncoding(data []byte) string {
	for _, m := range unsupportedBOMs {
		if bytes.HasPrefix(data, m.mark) {
			return m.name
		}
	}
	return ""
}

// ReadFile is the read phase of a load: the file's raw bytes. It is
// separate from Prepare — the decode/map phase — so the caller can
// schedule, gate, or cancel the phases independently (Issue #25).
func ReadFile(path []byte) ([]byte, error) {
	return os.ReadFile(string(path))
}

// Load reads path and prepares its display-ready content and validated
// highlights — ReadFile followed by Prepare. The caller delivers the
// finished Buffer as its completion message so no full-file work lands
// on the UI update path.
func Load(path []byte, stops []searchindex.Stop) (*Buffer, error) {
	data, err := ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Prepare(data, stops), nil
}

// Prepare turns previously read file bytes into a display-ready
// Buffer: the split, escape, and map work of a load.
//
// Each stop's submatches are checked against the line's raw bytes —
// never the escaped display text — for line existence, range validity,
// and byte equality with the recorded match; every failure drops that
// submatch and marks the buffer stale while the line's valid
// submatches keep their highlights (Issue #29). A leading UTF-8 BOM
// splits the first line's coordinate views: its raw bytes retain the
// BOM while rg's first-line data omits it, so rg offsets on that line
// shift by its length into the raw view before validating and mapping.
// A leading UTF-16/UTF-32 BOM classifies the file as an unsupported
// encoding instead (Issue #30): its bytes are not displayable UTF-8
// text, so the buffer carries no lines or spans and skips the
// raw-byte validation entirely — rg's recorded submatches describe the
// transcoded text and can never equal these bytes.
func Prepare(data []byte, stops []searchindex.Stop) *Buffer {
	b := &Buffer{spans: make(map[int][]present.Span)}
	if enc := unsupportedEncoding(data); enc != "" {
		b.enc = enc
		b.digits = 1 // the placeholder's minimal one-digit gutter
		return b
	}
	if bytes.HasPrefix(data, utf8BOM) {
		b.bom = len(utf8BOM)
	}
	for rest := data; len(rest) > 0; {
		// Only the file's first line can carry the BOM: LineOfBOM
		// keeps its bytes in the raw view while painting nothing.
		escape := present.LineOf
		if b.bom > 0 && len(b.lines) == 0 {
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
			// The stop's line is gone: every recorded submatch fails
			// its line-existence check, so the stop drops whole and
			// the buffer is stale.
			if len(st.Submatches) > 0 {
				b.stale = true
			}
			continue
		}
		ln := b.lines[li]
		adj := 0
		if li == 0 {
			adj = b.bom
		}
		for _, sm := range st.Submatches {
			start, end := sm.Start+adj, sm.End+adj
			if start < 0 || end > len(ln.Raw()) || start > end ||
				!bytes.Equal(ln.Raw()[start:end], sm.Bytes) {
				b.stale = true
				continue
			}
			b.spans[li] = append(b.spans[li],
				clusterSpan(ln.Cells(), ln.Span(start, end)))
		}
	}
	return b
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

// Stale reports whether any recorded submatch failed validation —
// the "file changed since search" state. It is computed at Prepare
// time, so a reload's fresh buffer carries its own verdict: the mark
// clears only when the newly loaded content validates fully. The
// check is best-effort correspondence, not a snapshot — same-text
// moves and edits outside matched spans go undetected.
func (b *Buffer) Stale() bool { return b.stale }

// Unsupported reports the encoding name a leading UTF-16/UTF-32
// byte-order mark declared when the buffer is the "(unsupported
// encoding)" placeholder — a buffer whose file the terminal cannot
// display as text — or "" for ordinary displayable content (Issue
// #30).
func (b *Buffer) Unsupported() string { return b.enc }

// RevealTarget returns the display location — the zero-based source
// line and display cell — that stop s's navigation reveal must show
// (Issue #29). Ordinarily it is the start cell of the line's first
// surviving validated span, the marker cell for a zero-width match
// included. When the stop validated stale the fallbacks keep the entry
// landable while inventing no highlights or markers: with no survivors
// but the line still present, the earliest recorded submatch start —
// shifted past the leading BOM's bytes on the first line — clamped to
// the line's raw bytes and mapped to a display cell, an end-of-line
// mapping falling back to the last rendered cell since no marker
// paints there; with the line gone, the last source line's start. An
// empty file yields the zero value: a zero-line panel has nowhere to
// land, and the viewport's empty reveal is a no-op.
func (b *Buffer) RevealTarget(s searchindex.Stop) (line, cell int) {
	li := int(s.Line) - 1
	if li < 0 || li >= len(b.lines) {
		return max(0, len(b.lines)-1), 0
	}
	if sp := b.spans[li]; len(sp) > 0 {
		cell = sp[0].Start
		for _, x := range sp[1:] {
			cell = min(cell, x.Start)
		}
		return li, cell
	}
	start := 0
	for i, sm := range s.Submatches {
		if i == 0 || sm.Start < start {
			start = sm.Start
		}
	}
	if li == 0 {
		start += b.bom
	}
	ln := b.lines[li]
	start = min(max(start, 0), len(ln.Raw()))
	cell = ln.Span(start, start).Start
	if n := len(ln.Cells()); cell >= n {
		cell = max(0, n-1)
	}
	return li, cell
}

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
