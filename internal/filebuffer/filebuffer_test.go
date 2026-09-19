package filebuffer_test

import (
	"os"
	"path/filepath"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// writeFile stores content in a fresh temp dir and returns its raw
// path bytes.
func writeFile(t *testing.T, content string) []byte {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return []byte(p)
}

// stop is one matched-line stop carrying the given union highlight
// byte ranges.
func stop(line int64, spans ...[2]int) searchindex.Stop {
	st := searchindex.Stop{Number: line}
	for _, s := range spans {
		st.Highlights = append(st.Highlights, searchindex.Span{Start: s[0], End: s[1]})
	}
	return st
}

func load(t *testing.T, path []byte, stops ...searchindex.Stop) *filebuffer.Buffer {
	t.Helper()
	b, err := filebuffer.Load(path, stops)
	if err != nil {
		t.Fatalf("Load(%q): %v", path, err)
	}
	return b
}

// Loading a file's bytes yields the source-line count and the escaped
// per-line display text.
func TestLoadCountsLines(t *testing.T) {
	b := load(t, writeFile(t, "one\ntwo\nthree\n"))
	if b.LineCount() != 3 {
		t.Fatalf("LineCount = %d, want 3", b.LineCount())
	}
	lines := b.Lines()
	for i, want := range []string{"one", "two", "three"} {
		if lines[i].Text != want {
			t.Fatalf("line %d text = %q, want %q", i, lines[i].Text, want)
		}
		if lines[i].Number != int64(i+1) {
			t.Fatalf("line %d number = %d, want %d", i, lines[i].Number, i+1)
		}
	}
}

// A missing final newline still yields the last line; a trailing
// newline does not invent an empty line.
func TestLoadLineEndings(t *testing.T) {
	if b := load(t, writeFile(t, "one\ntwo")); b.LineCount() != 2 {
		t.Fatalf("unterminated tail: LineCount = %d, want 2", b.LineCount())
	}
	if b := load(t, writeFile(t, "one\n\n")); b.LineCount() != 2 {
		t.Fatalf("trailing newline: LineCount = %d, want 2", b.LineCount())
	}
	// CRLF is a terminator, never displayed.
	b := load(t, writeFile(t, "a\r\nb\r\n"))
	if b.LineCount() != 2 || b.Lines()[0].Text != "a" || b.Lines()[1].Text != "b" {
		t.Fatalf("CRLF lines = %q, %q", b.Lines()[0].Text, b.Lines()[1].Text)
	}
}

// An empty file has zero source lines and the minimum one-digit-slot
// gutter.
func TestLoadEmptyFile(t *testing.T) {
	b := load(t, writeFile(t, ""))
	if b.LineCount() != 0 {
		t.Fatalf("LineCount = %d, want 0", b.LineCount())
	}
	if len(b.Lines()) != 0 {
		t.Fatalf("Lines() = %d entries, want an empty panel — zero source lines", len(b.Lines()))
	}
	if b.GutterWidth() != 3 {
		t.Fatalf("GutterWidth = %d, want 3 (one digit slot + two spaces)", b.GutterWidth())
	}
}

// The gutter is the digit width of the largest line number plus two
// spaces.
func TestGutterWidth(t *testing.T) {
	if b := load(t, writeFile(t, "a\nb\nc\n")); b.GutterWidth() != 3 {
		t.Fatalf("3-line file: GutterWidth = %d, want 3", b.GutterWidth())
	}
	var ten string
	for i := 0; i < 10; i++ {
		ten += "x\n"
	}
	if b := load(t, writeFile(t, ten)); b.GutterWidth() != 4 {
		t.Fatalf("10-line file: GutterWidth = %d, want 4", b.GutterWidth())
	}
}

// Content escapes land per the safe-presentation core: caret notation
// for C0/DEL, U+FFFD for invalid UTF-8, ^M for a standalone CR, and a
// tab expanded with blank cells to the next eight-column stop.
func TestLoadEscapedContent(t *testing.T) {
	b := load(t, writeFile(t, "a\x1bb\xffc\rd\te\n"))
	l := b.Lines()[0]
	// Cells: a, ^[, b, U+FFFD, c, ^M, d occupy columns 0-8; the tab at
	// column 9 expands seven blank cells to column 16, then e.
	if l.Text != "a^[bc^Md       e" {
		t.Fatalf("escaped text = %q, want %q", l.Text, "a^[bc^Md       e")
	}
	if len(l.Cells) != 17 {
		t.Fatalf("cells = %d, want 17 (one per display cell)", len(l.Cells))
	}
}

// Tabs expand to the next multiple of 8 source-display columns: every
// expansion cell is a blank mapped to the tab byte, and the following
// character lands on the stop. The columns are the line's own display
// cells — gutter width and horizontal pan can never shift them — and
// the whole expansion is a single cluster so wrapping cannot split it.
func TestTabExpandsToEightColumnStops(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		cells    int    // total display cells on the line
		tab      [2]int // cell range of the first tab's expansion
		tabByte  [2]int // the tab's source byte range
		after    int    // cell of the rune after the tab
		afterTxt string
	}{
		{"leading tab", "\tx", 9, [2]int{0, 8}, [2]int{0, 1}, 8, "x"},
		{"tab after one column", "a\tb", 9, [2]int{1, 8}, [2]int{1, 2}, 8, "b"},
		{"tab after seven columns", "abcdefg\th", 9, [2]int{7, 8}, [2]int{7, 8}, 8, "h"},
		{"tab on a stop takes a full eight", "abcdefgh\ti", 17, [2]int{8, 16}, [2]int{8, 9}, 16, "i"},
		{"wide glyph counts cells not bytes", "世\tb", 9, [2]int{2, 8}, [2]int{3, 4}, 8, "b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := load(t, writeFile(t, tc.content+"\n")).Lines()[0]
			if len(l.Cells) != tc.cells {
				t.Fatalf("cells = %d, want %d", len(l.Cells), tc.cells)
			}
			for i := tc.tab[0]; i < tc.tab[1]; i++ {
				if c := l.Cells[i]; c.Text != " " || c.Start != tc.tabByte[0] || c.End != tc.tabByte[1] {
					t.Fatalf("tab cell %d = %+v, want a blank over bytes [%d,%d)",
						i, c, tc.tabByte[0], tc.tabByte[1])
				}
			}
			if c := l.Cells[tc.after]; c.Text != tc.afterTxt {
				t.Fatalf("after-tab cell %d = %q, want %q on the stop", tc.after, c.Text, tc.afterTxt)
			}
			// The expansion is one cluster — an unbreakable wrap unit.
			found := false
			for _, cl := range l.Clusters {
				if cl.Start == tc.tab[0] && cl.End == tc.tab[1] {
					found = true
				}
			}
			if !found {
				t.Fatalf("clusters = %v, want one cluster covering the tab's cells [%d,%d)",
					l.Clusters, tc.tab[0], tc.tab[1])
			}
		})
	}
}

// Line.Clusters exposes the shared grapheme segmentation as cell
// ranges — FileBuffer's contract to Viewport: clusters tile the line's
// cells contiguously, each cluster's width is End-Start, a multi-cell
// unit (wide glyph, tab expansion) is one cluster, and each character
// of an escaped form is its own single-cell cluster.
func TestLineClustersExposeBoundaries(t *testing.T) {
	// Cells: a(0) 文(1-2) tab(3-7, to column 8) e+́(8) ^[(9-10).
	b := load(t, writeFile(t, "a文\te\u0301\x1b\n"))
	l := b.Lines()[0]
	want := []struct{ lo, hi int }{
		{0, 1}, {1, 3}, {3, 8}, {8, 9}, {9, 10}, {10, 11},
	}
	if len(l.Clusters) != len(want) {
		t.Fatalf("Clusters = %v, want %d entries", l.Clusters, len(want))
	}
	pos := 0
	for i, cl := range l.Clusters {
		if cl.Start != want[i].lo || cl.End != want[i].hi {
			t.Fatalf("cluster %d = [%d,%d), want [%d,%d)", i, cl.Start, cl.End, want[i].lo, want[i].hi)
		}
		if cl.Start != pos || cl.End <= cl.Start {
			t.Fatalf("cluster %d = [%d,%d) breaks the contiguous tiling at cell %d",
				i, cl.Start, cl.End, pos)
		}
		pos = cl.End
	}
	if pos != len(l.Cells) {
		t.Fatalf("clusters end at cell %d, want them tiling all %d cells", pos, len(l.Cells))
	}
}

// A match covering a tab byte highlights every cell of its expansion.
func TestHighlightCoversTabExpansion(t *testing.T) {
	b := load(t, writeFile(t, "a\tb\n"), stop(1, [2]int{1, 2}))
	got := b.Lines()[0].Highlights
	if len(got) != 1 || got[0].Start != 1 || got[0].End != 8 {
		t.Fatalf("highlights = %v, want [{1 8}] covering the whole tab stop", got)
	}
}

// Highlight spans for a matched line map byte ranges through the
// line's byte→cell map.
func TestHighlightSpans(t *testing.T) {
	b := load(t, writeFile(t, "zero\nalpha here\n"), stop(2, [2]int{0, 5}))
	lines := b.Lines()
	if len(lines[0].Highlights) != 0 {
		t.Fatalf("line 1 highlights = %v, want none", lines[0].Highlights)
	}
	got := lines[1].Highlights
	if len(got) != 1 || got[0].Start != 0 || got[0].End != 5 {
		t.Fatalf("line 2 highlights = %v, want [{0 5}]", got)
	}
}

// A match covering the ESC byte highlights both cells of its ^[ form.
func TestHighlightCoversEscapedCells(t *testing.T) {
	b := load(t, writeFile(t, "a\x1bb\n"), stop(1, [2]int{1, 2}))
	got := b.Lines()[0].Highlights
	if len(got) != 1 || got[0].Start != 1 || got[0].End != 3 {
		t.Fatalf("highlights = %v, want [{1 3}] covering ^[", got)
	}
}

// A nonempty span partially covering a grapheme cluster expands
// outward to the whole cluster — whether the span's start lands
// inside, its end lands inside, or both — so highlight cell ranges
// always align to cluster boundaries (Issue #21).
func TestHighlightExpandsPartialCluster(t *testing.T) {
	// "cafe\xcc\x81x": é is one cluster over bytes [3,6) mapping to
	// cell 3; 'x' is byte 6, cell 4.
	decomposed := "cafe\xcc\x81x\n"
	// "a文b日c": 文 is bytes [1,4) over cells [1,3), 日 bytes [5,8)
	// over cells [4,6).
	wide := "a文b日c\n"
	cases := []struct {
		name    string
		content string
		span    [2]int
		want    [2]int
	}{
		{"start inside a cluster", decomposed, [2]int{4, 7}, [2]int{3, 5}},
		{"end inside a cluster", decomposed, [2]int{1, 5}, [2]int{1, 4}},
		{"both ends inside one cluster", decomposed, [2]int{4, 5}, [2]int{3, 4}},
		{"both ends inside different clusters", wide, [2]int{2, 6}, [2]int{1, 6}},
		{"wide cluster start inside", wide, [2]int{2, 4}, [2]int{1, 3}},
		{"wide cluster end inside", wide, [2]int{6, 8}, [2]int{4, 6}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := load(t, writeFile(t, tc.content), stop(1, tc.span))
			got := b.Lines()[0].Highlights
			if len(got) != 1 || got[0].Start != tc.want[0] || got[0].End != tc.want[1] {
				t.Fatalf("span %v: highlights = %v, want [%d %d]", tc.span, got, tc.want[0], tc.want[1])
			}
		})
	}
}

// A match consisting only of combining-mark bytes inside a base
// cluster highlights that whole cluster — the whole é glyph, not a
// zero-width sliver of it.
func TestHighlightCombiningOnlyMatchExpandsToBaseCluster(t *testing.T) {
	b := load(t, writeFile(t, "cafe\xcc\x81x\n"), stop(1, [2]int{4, 6}))
	got := b.Lines()[0].Highlights
	if len(got) != 1 || got[0].Start != 3 || got[0].End != 4 {
		t.Fatalf("combining-only match: highlights = %v, want [{3 4}] covering the whole é", got)
	}
}

// A cluster with no base or independent visible cell still gets a
// visible fallback cell — the recorded Issue #43 ◌-plus-marks form —
// so a match on it is never a zero-cell highlight.
func TestHighlightStandaloneCombiningGetsFallbackCell(t *testing.T) {
	b := load(t, writeFile(t, "\xcc\x81x\n"), stop(1, [2]int{0, 2}))
	l := b.Lines()[0]
	if l.Cells[0].Text != "◌́" {
		t.Fatalf("standalone combining cell text = %q, want the ◌ fallback form", l.Cells[0].Text)
	}
	if len(l.Clusters) == 0 || l.Clusters[0].End != 1 {
		t.Fatalf("clusters = %v, want the fallback occupying exactly one cell", l.Clusters)
	}
	if len(l.Highlights) != 1 || l.Highlights[0].Start != 0 || l.Highlights[0].End != 1 {
		t.Fatalf("highlights = %v, want [{0 1}] — the fallback cell highlighted, never zero cells",
			l.Highlights)
	}
}

// A two-cell glyph is never split by a highlight boundary: a match
// touching any of its bytes covers both cells together.
func TestHighlightWideGlyphPairNeverSplit(t *testing.T) {
	for _, span := range [][2]int{{2, 3}, {3, 4}, {4, 5}, {2, 5}} {
		b := load(t, writeFile(t, "ab文cd\n"), stop(1, span))
		got := b.Lines()[0].Highlights
		if len(got) != 1 || got[0].Start != 2 || got[0].End != 4 {
			t.Fatalf("span %v on a wide glyph: highlights = %v, want [{2 4}] — both cells together",
				span, got)
		}
	}
}

// An emoji ZWJ sequence is one cluster under the shared policy: a
// match inside its bytes highlights the whole two-cell cluster.
func TestHighlightEmojiZWJCluster(t *testing.T) {
	b := load(t, writeFile(t, "x👨‍👩‍👧y\n"), stop(1, [2]int{5, 10}))
	l := b.Lines()[0]
	if len(l.Highlights) != 1 || l.Highlights[0].Start != 1 || l.Highlights[0].End != 3 {
		t.Fatalf("ZWJ partial match: highlights = %v, want [{1 3}] — the whole sequence", l.Highlights)
	}
}

// LF, CRLF, and a mix of both terminate lines without being
// displayed, while each line's Raw retains the original bytes —
// terminator included — for byte-coordinate mapping and Issue #29's
// stale validation.
func TestTerminatorsRemovedFromDisplayRetainedInRaw(t *testing.T) {
	b := load(t, writeFile(t, "a\r\nb\nc\r\n"))
	want := []struct{ text, raw string }{
		{"a", "a\r\n"},
		{"b", "b\n"},
		{"c", "c\r\n"},
	}
	if b.LineCount() != len(want) {
		t.Fatalf("LineCount = %d, want %d", b.LineCount(), len(want))
	}
	for i, w := range want {
		l := b.Lines()[i]
		if l.Text != w.text {
			t.Fatalf("line %d text = %q, want %q — no terminator cells", i, l.Text, w.text)
		}
		if string(l.Raw) != w.raw {
			t.Fatalf("line %d raw = %q, want %q — original bytes retained", i, l.Raw, w.raw)
		}
	}
}

// A standalone CR — one with no following LF — is not a terminator:
// the safe-presentation core escapes it as ^M, whether it sits
// mid-line or ends an unterminated final line.
func TestStandaloneCREscapesNotTerminator(t *testing.T) {
	b := load(t, writeFile(t, "x\ry\r\nz\r"))
	if b.LineCount() != 2 {
		t.Fatalf("LineCount = %d, want 2", b.LineCount())
	}
	if l := b.Lines()[0]; l.Text != "x^My" || string(l.Raw) != "x\ry\r\n" {
		t.Fatalf("line 1 = %q raw %q, want %q raw %q — the mid-line CR is content",
			l.Text, l.Raw, "x^My", "x\ry\r\n")
	}
	if l := b.Lines()[1]; l.Text != "z^M" || string(l.Raw) != "z\r" {
		t.Fatalf("line 2 = %q raw %q, want %q raw %q — a trailing CR with no LF is content",
			l.Text, l.Raw, "z^M", "z\r")
	}
}

// Bytes the terminator stripped from display still map: a zero-width
// position inside the terminator (the rg `$` at byte 4 of hit\r\n) and
// a span covering only terminator bytes both land on the display
// end-of-line position — one past the last cell — the marker position
// Issue #23 paints.
func TestTerminatorBytesMapToEndOfLine(t *testing.T) {
	cases := []struct {
		name    string
		content string
		span    [2]int
		want    [2]int
	}{
		{"zero-width inside CRLF", "hit\r\n", [2]int{4, 4}, [2]int{3, 3}},
		{"CR byte only", "hit\r\n", [2]int{3, 4}, [2]int{3, 3}},
		{"whole CRLF", "hit\r\n", [2]int{3, 5}, [2]int{3, 3}},
		{"LF terminator byte", "hit\n", [2]int{3, 4}, [2]int{3, 3}},
		{"zero-width at LF end", "hit\n", [2]int{3, 3}, [2]int{3, 3}},
		{"zero-width on an empty line", "\n", [2]int{0, 0}, [2]int{0, 0}},
		{"empty line's terminator", "\n", [2]int{0, 1}, [2]int{0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := load(t, writeFile(t, tc.content), stop(1, tc.span)).Lines()[0]
			if len(l.Highlights) != 1 || l.Highlights[0].Start != tc.want[0] || l.Highlights[0].End != tc.want[1] {
				t.Fatalf("span %v on %q: highlights = %v, want [%d %d] — the end-of-line position",
					tc.span, tc.content, l.Highlights, tc.want[0], tc.want[1])
			}
		})
	}
}

// A span covering visible text plus terminator bytes highlights only
// the visible text — the removed bytes add no cell of their own.
func TestSpanCrossingTerminatorHighlightsVisibleTextOnly(t *testing.T) {
	cases := []struct {
		name string
		span [2]int
		want [2]int
	}{
		{"rg dot-star covers text and CR", [2]int{0, 4}, [2]int{0, 3}},
		{"mid-line start through CRLF", [2]int{1, 5}, [2]int{1, 3}},
		{"whole line including CRLF", [2]int{0, 5}, [2]int{0, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := load(t, writeFile(t, "hit\r\n"), stop(1, tc.span)).Lines()[0]
			if len(l.Highlights) != 1 || l.Highlights[0].Start != tc.want[0] || l.Highlights[0].End != tc.want[1] {
				t.Fatalf("span %v: highlights = %v, want [%d %d] — visible text only",
					tc.span, l.Highlights, tc.want[0], tc.want[1])
			}
		})
	}
}

// A leading UTF-8 BOM is invisible in display: ripgrep's line offsets
// for the first line omit its three bytes, so the line keeps separate
// raw-file and rg-line views — Raw retains the BOM, SearchBytes is
// what rg saw — and cell byte offsets stay in raw-file coordinates, so
// an rg offset of 0 maps to raw byte 3.
func TestLeadingUTF8BOMInvisibleWithAdjustedCoordinates(t *testing.T) {
	b := load(t, writeFile(t, "\xef\xbb\xbfhit\nsecond\n"),
		stop(1, [2]int{0, 3}), stop(2, [2]int{0, 6}))
	l := b.Lines()[0]
	if l.Text != "hit" {
		t.Fatalf("BOM line text = %q, want %q — the BOM is not displayed", l.Text, "hit")
	}
	if string(l.Raw) != "\xef\xbb\xbfhit\n" {
		t.Fatalf("raw = %q, want the original BOM bytes retained", l.Raw)
	}
	if string(l.SearchBytes()) != "hit\n" {
		t.Fatalf("SearchBytes = %q, want %q — the rg-line view omits the BOM",
			l.SearchBytes(), "hit\n")
	}
	if l.Cells[0].Start != 3 || l.Cells[0].End != 4 {
		t.Fatalf("first cell maps bytes [%d,%d), want [3,4) — rg offset 0 is raw byte 3",
			l.Cells[0].Start, l.Cells[0].End)
	}
	if len(l.Highlights) != 1 || l.Highlights[0].Start != 0 || l.Highlights[0].End != 3 {
		t.Fatalf("rg span [0,3): highlights = %v, want [{0 3}]", l.Highlights)
	}
	// Lines after the first share the raw and rg views.
	l2 := b.Lines()[1]
	if l2.Cells[0].Start != 0 || string(l2.SearchBytes()) != "second\n" {
		t.Fatalf("line 2 view = %q cell start %d, want unadjusted",
			l2.SearchBytes(), l2.Cells[0].Start)
	}
}

// On a BOM line the rg view is what the terminator mapping measures
// against: a first-line span over the terminator still lands on the
// display end-of-line position.
func TestBOMLineTerminatorMapsToEndOfLine(t *testing.T) {
	l := load(t, writeFile(t, "\xef\xbb\xbfhit\n"), stop(1, [2]int{3, 4})).Lines()[0]
	if len(l.Highlights) != 1 || l.Highlights[0].Start != 3 || l.Highlights[0].End != 3 {
		t.Fatalf("terminator span on a BOM line: highlights = %v, want [{3 3}]",
			l.Highlights)
	}
}

// A U+FEFF anywhere but the very start of the file is ordinary
// content, never a BOM: the safe-presentation core escapes the
// non-printable rune and byte offsets run unadjusted.
func TestNonLeadingFEFFIsOrdinaryContent(t *testing.T) {
	b := load(t, writeFile(t, "a\xef\xbb\xbfb\n\xef\xbb\xbfz\n"),
		stop(2, [2]int{0, 3}))
	if l := b.Lines()[0]; l.Text != `a\ufeffb` {
		t.Fatalf("mid-line U+FEFF: text = %q, want %q", l.Text, `a\ufeffb`)
	}
	l := b.Lines()[1]
	if l.Text != `\ufeffz` {
		t.Fatalf("line-2 U+FEFF: text = %q, want %q — not a BOM", l.Text, `\ufeffz`)
	}
	if string(l.Raw) != "\xef\xbb\xbfz\n" || string(l.SearchBytes()) != "\xef\xbb\xbfz\n" {
		t.Fatalf("line 2 raw/search views = %q/%q, want the U+FEFF bytes unadjusted",
			l.Raw, l.SearchBytes())
	}
	// The U+FEFF match highlights its six escape cells.
	if len(l.Highlights) != 1 || l.Highlights[0].Start != 0 || l.Highlights[0].End != 6 {
		t.Fatalf("highlights = %v, want [{0 6}] covering the \\ufeff escape", l.Highlights)
	}
}

// A zero-width submatch records a marker position — the display cell
// it marks — as an empty highlight span. Positions map through the
// same coordinate rules as any other span: beginning of line lands on
// cell 0, a position inside content lands on the cell holding its
// byte — inside a cluster, the cluster's start, so no wide glyph is
// split — and end of line, removed terminator bytes, and empty-line
// positions land on the end-of-line cell one past the last.
func TestZeroWidthMarkerPositions(t *testing.T) {
	cases := []struct {
		name    string
		content string
		span    [2]int
		want    int // the marker's display cell
	}{
		{"beginning of line", "hit\n", [2]int{0, 0}, 0},
		{"inside text", "hit\n", [2]int{1, 1}, 1},
		// 文 is bytes [2,5) over cells [2,4): a position inside its
		// bytes marks the cluster's start cell — no split wide glyph.
		{"inside a wide cluster", "ab文cd\n", [2]int{3, 3}, 2},
		{"wide cluster's last byte", "ab文cd\n", [2]int{4, 4}, 2},
		// é is the cluster e+́ over bytes [3,6) at cell 3.
		{"inside a combining cluster", "cafe\xcc\x81x\n", [2]int{4, 4}, 3},
		{"end of line", "hit\n", [2]int{3, 3}, 3},
		{"the LF terminator byte", "hit\n", [2]int{3, 4}, 3},
		{"empty line", "\n", [2]int{0, 0}, 0},
		{"empty line's terminator", "\n", [2]int{0, 1}, 0},
		// The rg $ on hit\r\n reports start=end=4; terminator-only
		// spans of either byte or the whole \r\n land identically.
		{"$ on CRLF", "hit\r\n", [2]int{4, 4}, 3},
		{"CR byte only", "hit\r\n", [2]int{3, 4}, 3},
		{"whole CRLF", "hit\r\n", [2]int{3, 5}, 3},
		{"a position past the line", "hit\n", [2]int{9, 9}, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := load(t, writeFile(t, tc.content), stop(1, tc.span)).Lines()[0]
			if !l.MarkerAt(tc.want) {
				t.Fatalf("span %v on %q: no marker at cell %d — Markers/Highlights %v",
					tc.span, tc.content, tc.want, l.Highlights)
			}
			found := false
			for _, s := range l.Highlights {
				if s.Start == s.End && s.Start == tc.want {
					found = true
				}
			}
			if !found {
				t.Fatalf("span %v on %q: highlights = %v, want an empty span {%d %d} recording the marker",
					tc.span, tc.content, l.Highlights, tc.want, tc.want)
			}
		})
	}
}

// An end-of-line marker extends the effective line width by one cell:
// it is a one-cell unit past the last cluster, so an empty matched
// line has width one. A marker inside text marks an existing cell and
// adds nothing.
func TestEOLMarkerExtendsEffectiveWidth(t *testing.T) {
	cases := []struct {
		name    string
		content string
		stops   []searchindex.Stop
		want    int
	}{
		{"no marker", "hit\n", nil, 3},
		{"EOL marker", "hit\n", []searchindex.Stop{stop(1, [2]int{3, 3})}, 4},
		{"empty line's marker is width one", "\n", []searchindex.Stop{stop(1, [2]int{0, 0})}, 1},
		{"BOL marker adds no cell", "hit\n", []searchindex.Stop{stop(1, [2]int{0, 0})}, 3},
		{"mid-text marker adds no cell", "hit\n", []searchindex.Stop{stop(1, [2]int{1, 1})}, 3},
		{"wide line plus marker", "ab文\n", []searchindex.Stop{stop(1, [2]int{5, 5})}, 5},
		{"BOL and EOL markers extend once", "hit\n",
			[]searchindex.Stop{stop(1, [2]int{0, 0}, [2]int{3, 3})}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := load(t, writeFile(t, tc.content), tc.stops...).Lines()[0]
			if got := l.Extent(); got != tc.want {
				t.Fatalf("%q: Extent = %d, want %d", tc.content, got, tc.want)
			}
		})
	}
}

// The end-of-line marker joins the paintable boundary's candidate set
// like any cluster: its start cell feeds Issue #18's maximum, so the
// offset can reach the position where the marker paints alone. A
// marker-only line — an empty line's marker at cell 0 — has extent 1
// and a maximum offset of 0.
func TestMarkerFeedsPaintableBoundary(t *testing.T) {
	l := load(t, writeFile(t, "hit\n"), stop(1, [2]int{3, 3})).Lines()[0]
	if got := l.MaxStart(4); got != 3 {
		t.Fatalf("MaxStart(4) = %d, want 3 — the marker's start paints it alone", got)
	}
	if got := l.MaxStart(1); got != 3 {
		t.Fatalf("MaxStart(1) = %d, want 3 — the one-cell marker still fits", got)
	}
	empty := load(t, writeFile(t, "\n"), stop(1, [2]int{0, 0})).Lines()[0]
	if got := empty.MaxStart(10); got != 0 {
		t.Fatalf("marker-only line MaxStart = %d, want 0 — extent 1, maximum offset 0", got)
	}
	// A BOL marker adds no boundary candidate of its own: the cell 0
	// cluster start already covers it.
	bol := load(t, writeFile(t, "hit\n"), stop(1, [2]int{0, 0})).Lines()[0]
	if got := bol.MaxStart(4); got != 2 {
		t.Fatalf("BOL-marker line MaxStart(4) = %d, want 2 — the last cluster start", got)
	}
}

// Stops outside the loaded file's range contribute nothing.
func TestStopBeyondFileIgnored(t *testing.T) {
	b := load(t, writeFile(t, "one\ntwo\n"), stop(99, [2]int{0, 2}))
	for i, l := range b.Lines() {
		if len(l.Highlights) != 0 {
			t.Fatalf("line %d highlights = %v, want none", i, l.Highlights)
		}
	}
}

// A read failure is an error, never a partial buffer.
func TestLoadReadFailure(t *testing.T) {
	b, err := filebuffer.Load([]byte(filepath.Join(t.TempDir(), "gone")), nil)
	if err == nil || b != nil {
		t.Fatalf("Load(missing) = %v, %v, want nil buffer and error", b, err)
	}
}
