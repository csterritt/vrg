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
// eight-column stops) so Viewport wraps without re-deriving. Issue #22
// separates the coordinate views — raw-file bytes, the rg-line view
// ripgrep's offsets index, and display cells — so a leading UTF-8 BOM
// stays invisible yet accounted for and removed terminator bytes still
// map to the display end-of-line position. Issue #23 turns the
// zero-width and terminator-only mappings into markers: an empty
// highlight span records the marker's cell — the existing cell it
// marks inside text, or the one-cell unit past the last cluster that
// extends the effective line width. Issue #29 validates every recorded
// submatch against the loaded bytes on each load: a submatch whose
// line vanished, whose range outgrew the line, or whose bytes no
// longer equal the recorded ones is dropped — the buffer reports
// Stale — while survivors keep their highlights and each stale stop
// keeps a landing, the first survivor's cell or the clamped recorded
// start. Unsupported encodings are Issue #30's.
package filebuffer

import (
	"bytes"
	"os"

	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// utf8BOM is the encoding signature a leading UTF-8 BOM occupies at the
// very start of a file.
var utf8BOM = []byte{0xef, 0xbb, 0xbf}

// Line is one source line prepared for display: the escaped Text with
// its byte→cell map and grapheme-cluster segmentation (embedded Mapped),
// the original Raw bytes — leading BOM and terminator included —
// retained for identity and validation, and the matched spans as
// display-cell ranges in Highlights — each expanded outward to whole
// grapheme clusters, the single span source Viewport and App consume
// (Issue #21). An empty span records a zero-width marker's position —
// the display cell it marks (Issue #23). Only submatches that survived
// Issue #29's stale validation contribute spans; a dropped submatch
// paints nothing. Cell byte offsets are raw-file coordinates; the
// rg-line coordinate view ripgrep's offsets index is SearchBytes.
type Line struct {
	safepresentation.Mapped
	Number     int64
	Raw        []byte
	Highlights []searchindex.Span
	// searchOff counts the leading Raw bytes the rg-line coordinate
	// view omits — three for a leading UTF-8 BOM on line 1, zero
	// elsewhere — so content occupies Raw[searchOff:contentEnd] and
	// Raw[contentEnd:] is the undisplayed terminator.
	searchOff  int
	contentEnd int
	// target is the navigation stop's resolved reveal cell (Issue
	// #29): the first surviving submatch's start cell — the marker
	// cell for a zero-width survivor — or, when every recorded
	// submatch dropped, the first recorded start clamped to the
	// line's bytes and mapped to a valid display cell. hasTarget
	// records that the line is a stop whose recorded submatches were
	// validated.
	target    int
	hasTarget bool
}

// SearchBytes is the line's bytes in the rg-line coordinate view —
// exactly the bytes ripgrep reported and indexed its submatch offsets
// against: a leading UTF-8 BOM is stripped on line 1 while the
// terminator stays. Raw keeps the raw-file view; the two differ only
// by the BOM adjustment.
func (l Line) SearchBytes() []byte { return l.Raw[l.searchOff:] }

// CellsCovering maps a half-open byte range in rg-line coordinates —
// the offsets ripgrep's reported lines and submatches use — to the
// half-open display-cell range covering it. The BOM's bytes do not
// exist in rg space, so a first-line range shifts by the search
// adjustment into the raw-file coordinates the byte→cell map records.
// Bytes display removed — the line terminator — and positions past the
// content end all land on the display end-of-line position one past
// the last cell, as does a range lying beyond the line; a zero-width
// position inside content lands on the cell holding its byte. The
// empty results — lo == hi — are the marker positions Issue #23
// paints: MarkerAt reports them.
func (l Line) CellsCovering(start, end int) (lo, hi int, ok bool) {
	s, e := start+l.searchOff, end+l.searchOff
	if e <= s {
		switch {
		case s >= l.contentEnd:
			return len(l.Cells), len(l.Cells), true
		case s <= l.searchOff:
			return 0, 0, true
		default:
			lo, _, _ = l.Mapped.CellsCovering(s, s+1)
			return lo, lo, true
		}
	}
	cs, ce := s, e
	if cs < l.searchOff {
		cs = l.searchOff
	}
	if ce > l.contentEnd {
		ce = l.contentEnd
	}
	if cs < ce {
		lo, hi, _ = l.Mapped.CellsCovering(cs, ce)
		return lo, hi, true
	}
	if e <= l.searchOff {
		return 0, 0, true
	}
	return len(l.Cells), len(l.Cells), true
}

// Buffer is one loaded file's display-ready content.
type Buffer struct {
	lines []Line
	// stale marks that at least one recorded submatch failed
	// validation against the loaded bytes — Issue #29's "file changed
	// since search" state the filename row notes.
	stale bool
}

// Read loads path's raw bytes — the raw bytes are the filesystem key,
// never the escaped display form. It is the disk phase of Load, split
// out so the caller can pace or hold the boundary between the read and
// the decode/map phase.
func Read(path []byte) ([]byte, error) {
	return os.ReadFile(string(path))
}

// Decode splits raw content into source lines, maps each line's
// content through the safe-presentation core, then validates every
// recorded submatch against the loaded bytes — the decode/map phase
// of Load, with no filesystem access. Surviving submatches map onto
// display cells as the lines' highlights; dropped ones mark the
// buffer stale. A file the UTF-16/32 signatures classify keeps the
// unchecked mapping — Issue #30 owns that contract.
func Decode(raw []byte, stops []searchindex.Stop) *Buffer {
	b := &Buffer{}
	for start := 0; start < len(raw); {
		end := start
		for end < len(raw) && raw[end] != '\n' {
			end++
		}
		if end < len(raw) {
			end++ // the line keeps its \n terminator
		}
		b.lines = append(b.lines, makeLine(raw[start:end], int64(len(b.lines))+1))
		start = end
	}
	if utf16Or32(raw) {
		for _, st := range stops {
			b.mapRecorded(st)
		}
		return b
	}
	for _, st := range stops {
		b.validateStop(st)
	}
	return b
}

// utf16Or32 reports whether raw carries a UTF-16 or UTF-32 byte-order
// mark — the unsupported encodings Issue #30 owns, whose raw bytes
// Issue #29's per-submatch byte validation does not check.
func utf16Or32(raw []byte) bool {
	return bytes.HasPrefix(raw, []byte{0xff, 0xfe, 0x00, 0x00}) || // UTF-32 LE
		bytes.HasPrefix(raw, []byte{0x00, 0x00, 0xfe, 0xff}) || // UTF-32 BE
		bytes.HasPrefix(raw, []byte{0xff, 0xfe}) || // UTF-16 LE
		bytes.HasPrefix(raw, []byte{0xfe, 0xff}) // UTF-16 BE
}

// mapRecorded maps a stop's prepared highlight coverage onto its line
// unchecked — the UTF-16/32 path Issue #30 owns, and the
// no-recorded-submatches case inside validation.
func (b *Buffer) mapRecorded(st searchindex.Stop) {
	if row := int(st.Number) - 1; row >= 0 && row < len(b.lines) {
		b.lines[row].mapSpans(st.Highlights)
	}
}

// validateStop checks the stop's recorded submatches against the
// loaded line — the Issue #29 best-effort stale-match guard run on
// first load and every reload. Each submatch must satisfy line
// existence, range validity against the line's search bytes — the
// terminator-including, BOM-adjusted rg-line view — and byte equality
// with its recorded bytes; any failure drops that submatch and marks
// the buffer stale while valid submatches keep their highlights. The
// line's reveal target then resolves to the first survivor's start
// cell, or — when every recorded submatch dropped — to the first
// recorded start clamped to a valid display cell. A stop recording no
// submatches maps its prepared union coverage unchecked.
func (b *Buffer) validateStop(st searchindex.Stop) {
	row := int(st.Number) - 1
	if row < 0 || row >= len(b.lines) {
		// The recorded line is gone: every submatch fails line
		// existence. The stop keeps a landing — StopTarget resolves it
		// to the last source line's start.
		if len(st.Submatches) > 0 {
			b.stale = true
		}
		return
	}
	if len(st.Submatches) == 0 {
		b.mapRecorded(st)
		return
	}
	l := &b.lines[row]
	var surv []searchindex.Submatch
	sb := l.SearchBytes()
	for _, sm := range st.Submatches {
		if sm.Start < 0 || sm.End < sm.Start || sm.End > len(sb) ||
			!bytes.Equal(sb[sm.Start:sm.End], sm.Bytes) {
			b.stale = true
			continue
		}
		surv = append(surv, sm)
	}
	l.mapSpans(unionSubRanges(surv))
	l.hasTarget = true
	if len(surv) > 0 {
		// The first surviving submatch's start cell — a zero-width
		// survivor's marker position.
		s := surv[0]
		end := s.End
		if end <= s.Start {
			end = s.Start + 1
		}
		l.target, _, _ = l.CellsCovering(s.Start, end)
		return
	}
	// No survivors: the first recorded start clamped to the line's
	// bytes and mapped to a valid display cell — an end-of-line
	// landing clamps to the last rendered cell when no marker cell
	// sits there.
	start := st.Submatches[0].Start
	switch {
	case start > len(sb):
		start = len(sb)
	case start < 0:
		start = 0
	}
	l.target, _, _ = l.CellsCovering(start, start)
	if n := len(l.Cells); l.target >= n && !l.MarkerAt(n) {
		l.target = n - 1
	}
	if l.target < 0 {
		l.target = 0
	}
}

// mapSpans maps recorded rg-line byte ranges onto display cells as the
// line's highlight spans, each expanded outward to whole grapheme
// clusters — the single highlight source Viewport and App consume.
func (l *Line) mapSpans(spans []searchindex.Span) {
	for _, sp := range spans {
		if lo, hi, ok := l.CellsCovering(sp.Start, sp.End); ok {
			lo, hi = l.expandToClusters(lo, hi)
			l.Highlights = append(l.Highlights, searchindex.Span{Start: lo, End: hi})
		}
	}
}

// unionSubRanges merges the surviving submatches' sorted byte ranges
// into coverage spans — the union the index prepared for the recorded
// set, restricted to what still validates.
func unionSubRanges(subs []searchindex.Submatch) []searchindex.Span {
	var out []searchindex.Span
	for _, s := range subs {
		if n := len(out); n > 0 && s.Start <= out[n-1].End {
			if s.End > out[n-1].End {
				out[n-1].End = s.End
			}
			continue
		}
		out = append(out, searchindex.Span{Start: s.Start, End: s.End})
	}
	return out
}

// Load reads path and decodes the result — Read plus Decode in one
// call, so the caller's completion message carries a prepared buffer
// and its update path does no full-file work.
func Load(path []byte, stops []searchindex.Stop) (*Buffer, error) {
	raw, err := Read(path)
	if err != nil {
		return nil, err
	}
	return Decode(raw, stops), nil
}

// LineCount is the number of source lines: an empty file has zero, a
// missing final newline still yields the last line, and a trailing
// newline does not invent an empty line.
func (b *Buffer) LineCount() int { return len(b.lines) }

// Lines returns the prepared source lines in file order.
func (b *Buffer) Lines() []Line { return b.lines }

// Stale reports whether any recorded submatch failed validation
// against the loaded content — its line vanished, its range outgrew
// the line's bytes, or its bytes no longer equal the recorded ones —
// so at least one recorded highlight could not be honored (Issue
// #29). The mark is recomputed on every load and clears only on fully
// validating content.
func (b *Buffer) Stale() bool { return b.stale }

// StopTarget resolves a navigation stop to its reveal target under
// the loaded content: the destination's source line number and the
// display cell the reveal must show — Issue #29's stale-entry
// landings. A stop whose line still exists resolves to the cell
// validation recorded: the first surviving submatch's start cell —
// the marker cell for a zero-width survivor — or, when every recorded
// submatch dropped, the first recorded start clamped to the line's
// bytes and mapped to a valid display cell. A stop whose line the
// file no longer has lands at the last source line's start; an empty
// file has no rows and the stop keeps its bare recorded line.
func (b *Buffer) StopTarget(st searchindex.Stop) (line int64, cell int) {
	row := int(st.Number) - 1
	if row < 0 {
		return st.Number, 0
	}
	if row >= len(b.lines) {
		if len(b.lines) == 0 {
			return st.Number, 0
		}
		return b.lines[len(b.lines)-1].Number, 0
	}
	l := b.lines[row]
	if l.hasTarget {
		return st.Number, l.target
	}
	if len(st.Submatches) == 0 {
		return st.Number, 0
	}
	// A line validation never resolved — a stop that was not among
	// the buffer's load stops: the recorded first submatch's start
	// cell, the pre-validation mapping.
	s := st.Submatches[0]
	end := s.End
	if end <= s.Start {
		end = s.Start + 1
	}
	if lo, _, ok := l.CellsCovering(s.Start, end); ok {
		return st.Number, lo
	}
	return st.Number, len(l.Cells)
}

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

// MarkerAt reports whether a zero-width marker sits at display cell
// cell — an empty highlight span's single position (Issue #23). A
// position inside text marks the existing cell it lands on; the
// end-of-line position len(Cells) is the marker cell that extends the
// effective line width.
func (l Line) MarkerAt(cell int) bool {
	for _, s := range l.Highlights {
		if s.Start == s.End && s.Start == cell {
			return true
		}
	}
	return false
}

// Extent is the line's effective display width in cells: the display
// cell count plus one when an end-of-line marker occupies the cell
// past the last cluster — an empty matched line therefore has width
// one. Wrapping, clipping, horizontal extent, and the paintable
// boundary all measure this width (Issue #23).
func (l Line) Extent() int {
	n := len(l.Cells)
	if l.MarkerAt(n) {
		n++
	}
	return n
}

// MaxStart is the largest display-cell index where one of the line's
// grapheme clusters — or its end-of-line marker, a one-cell unit at
// the extent — begins and fits entirely within width cells: the
// paintable boundary (Issue #18) a horizontal offset may reach so at
// least one whole cluster or the marker still paints. A trailing
// cluster wider than width is skipped, falling back to the last
// fitting cluster's start, and a line with no fitting cluster reports
// 0. A marker-only line — an empty line's marker at cell 0 — has
// extent 1 and maximum offset 0.
func (l Line) MaxStart(width int) int {
	max := 0
	for _, cl := range l.Clusters {
		if w := cl.End - cl.Start; w >= 1 && w <= width && cl.Start > max {
			max = cl.Start
		}
	}
	if n := len(l.Cells); n > max && width >= 1 && l.MarkerAt(n) {
		max = n
	}
	return max
}

// makeLine prepares one raw line — BOM and terminator included — for
// display: the line is split into the leading-BOM, content, and
// terminator regions so the three coordinate views stay separate.
// Content maps through the safe-presentation core with cell byte
// offsets kept in raw-file coordinates; highlights arrive later, when
// validation maps the surviving rg byte ranges onto cells.
func makeLine(raw []byte, number int64) Line {
	// A leading UTF-8 BOM is invisible and never reaches rg's searched
	// line data: it lives only in the raw-file view. A U+FEFF anywhere
	// else is ordinary content.
	off := 0
	if number == 1 && bytes.HasPrefix(raw, utf8BOM) {
		off = len(utf8BOM)
	}
	end := len(raw)
	if end > 0 && raw[end-1] == '\n' {
		end--
		// A \r immediately before the \n is half of the CRLF
		// terminator; a standalone CR stays content and escapes as ^M.
		if end > 0 && raw[end-1] == '\r' {
			end--
		}
	}
	m := safepresentation.MapContent(raw[off:end])
	for i := range m.Cells {
		m.Cells[i].Start += off
		m.Cells[i].End += off
	}
	return Line{Mapped: m, Number: number, Raw: raw, searchOff: off, contentEnd: end}
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
