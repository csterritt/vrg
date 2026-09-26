package present

import (
	"strings"
	"testing"
)

// Content presentation: the display text of one source line. LF and
// CRLF are line terminators and produce no display; a standalone CR is
// escaped ^M; other C0 controls and DEL take caret notation; C1 takes
// \uXXXX; invalid UTF-8 becomes U+FFFD; tab expands with spaces to the
// next multiple of eight source-display columns.
func TestLineText(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"plain", "plain text\n", "plain text"},
		{"no final newline", "tail", "tail"},
		{"empty line", "\n", ""},
		{"escape", "a\x1bb\n", "a^[b"},
		{"bell", "a\x07b\n", "a^Gb"},
		{"backspace", "a\x08b\n", "a^Hb"},
		{"nul", "a\x00b\n", "a^@b"},
		{"delete", "a\x7fb\n", "a^?b"},
		{"vertical tab", "a\x0bb\n", "a^Kb"},
		{"osc sequence", "\x1b]0;pwned\x07\n", "^[]0;pwned^G"},
		{"csi sequence", "\x1b[2J\n", "^[[2J"},
		{"c1 nel", "a\xc2\x85b\n", "a\\u0085b"},
		{"c1 csi", "a\xc2\x9bb\n", "a\\u009bb"},
		{"invalid utf-8", "a\xffb\n", "a\ufffdb"},
		{"invalid utf-8 run", "a\xff\xfe\xfdb\n", "a\ufffd\ufffd\ufffdb"},
		{"truncated sequence", "a\xe2\x82b\n", "a\ufffd\ufffdb"},
		{"standalone cr", "a\rb\n", "a^Mb"},
		{"crlf only line", "\r\n", ""},
		{"crlf removed", "a\r\n", "a"},
		{"lf removed", "a\n", "a"},
		{"tab expansion", "a\tb\n", "a       b"},
		{"combining cluster", "e\u0301x\n", "e\u0301x"},
		{"standalone combining mark", "\u0301x\n", "\u25cc\u0301x"},
		{"printable unicode", "héllö→世\n", "héllö→世"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := LineOf([]byte(tc.raw))
			if l.Text() != tc.want {
				t.Fatalf("LineOf(%q).Text() = %q, want %q", tc.raw, l.Text(), tc.want)
			}
		})
	}
}

// Cell counts for the printable forms: ASCII is one cell per byte, a
// wide rune two, a combining cluster one, caret escapes two, \uXXXX six,
// U+FFFD one, and a tab its expansion width to the next eight-column
// stop.
func TestLineWidth(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want int
	}{
		{"ascii", "hello\n", 5},
		{"caret escape", "a\x1bb\n", 4},
		{"c1 escape", "a\xc2\x85b\n", 8},
		{"invalid byte", "a\xffb\n", 3},
		{"standalone cr", "a\rb\n", 4},
		{"wide rune", "世x\n", 3},
		{"combining cluster", "e\u0301x\n", 2},
		{"tab expands to the next stop", "a\tb\n", 9},
		{"tab at a stop takes eight", "12345678\tb\n", 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := LineOf([]byte(tc.raw)).Width(); got != tc.want {
				t.Fatalf("LineOf(%q).Width() = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

// Byte→cell mapping: every escaped form maps its source bytes to every
// display cell it produces, so a match covering an ESC byte highlights
// both ^ and [, a match inside a multi-byte unit covers the whole unit,
// removed terminator bytes map to the end-of-line position, and empty
// ranges yield a zero-width marker position.
func TestLineSpan(t *testing.T) {
	for _, tc := range []struct {
		name       string
		raw        string
		start, end int
		want       Span
	}{
		{"ascii range", "hello\n", 0, 3, Span{0, 3}},
		{"esc byte covers both caret cells", "a\x1bb\n", 1, 2, Span{1, 3}},
		{"esc inside larger match", "a\x1bb\n", 0, 3, Span{0, 4}},
		{"invalid byte", "a\xffb\n", 1, 2, Span{1, 2}},
		{"invalid byte run", "a\xff\xfeb\n", 1, 3, Span{1, 3}},
		{"c1 rune both bytes", "a\xc2\x85b\n", 1, 3, Span{1, 7}},
		{"c1 rune second byte only", "a\xc2\x85b\n", 2, 3, Span{1, 7}},
		{"standalone cr", "a\rb\n", 1, 2, Span{1, 3}},
		{"wide rune whole", "世x\n", 0, 3, Span{0, 2}},
		{"wide rune interior byte", "世x\n", 1, 2, Span{0, 2}},
		{"combining only", "e\u0301x\n", 1, 3, Span{0, 1}},
		{"standalone mark covers its fallback cell", "\u0301x\n", 0, 2, Span{0, 1}},
		{"crlf terminator-only match", "hit\r\n", 3, 5, Span{3, 3}},
		{"lf terminator-only match", "hit\n", 3, 4, Span{3, 3}},
		{"zero-width inside terminator", "hit\r\n", 4, 4, Span{3, 3}},
		{"zero-width at start", "ab\n", 0, 0, Span{0, 0}},
		{"zero-width mid-line", "ab\n", 1, 1, Span{1, 1}},
		{"zero-width at eol", "ab\n", 2, 2, Span{2, 2}},
		{"zero-width past end clamps", "ab\n", 9, 9, Span{2, 2}},
		{"empty line zero-width", "\n", 0, 0, Span{0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := LineOf([]byte(tc.raw)).Span(tc.start, tc.end); got != tc.want {
				t.Fatalf("LineOf(%q).Span(%d,%d) = %+v, want %+v",
					tc.raw, tc.start, tc.end, got, tc.want)
			}
		})
	}
}

// A tab is never emitted raw: it expands with space cells to the next
// multiple of eight source-display columns — the cell positions Issue
// #5 deferred — and its byte maps to the whole expansion, so a match on
// the tab highlights every expansion cell.
func TestLineTabStops(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
		bAt  int // display cell of the b following the tab
	}{
		{"tab at column zero", "\tb\n", "        b", 8},
		{"tab at column one", "a\tb\n", "a       b", 8},
		{"tab ending at a stop", "1234567\tb\n", "1234567 b", 8},
		{"tab starting at a stop", "12345678\tb\n", "12345678        b", 16},
		{"double tab", "\t\tb\n", "                b", 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := LineOf([]byte(tc.raw))
			if l.Text() != tc.want {
				t.Fatalf("LineOf(%q).Text() = %q, want %q", tc.raw, l.Text(), tc.want)
			}
			if got := l.Cells()[tc.bAt]; got.Text != "b" || !got.Lead {
				t.Fatalf("cell %d = %+v, want the b cluster lead", tc.bAt, got)
			}
			if strings.Contains(l.Text(), "\t") {
				t.Fatalf("raw tab survived presentation: %q", l.Text())
			}
		})
	}
	// The tab byte's span covers its whole expansion.
	if got := LineOf([]byte("a\tb\n")).Span(1, 2); got != (Span{1, 8}) {
		t.Fatalf("tab span = %+v, want {1 8} over the expansion", got)
	}
}

// Presented lines retain their raw bytes for identity, comparison, and
// later coordinate mapping.
func TestLineRetainsRaw(t *testing.T) {
	raw := []byte("a\xff\x1bb\r\n")
	l := LineOf(raw)
	if string(l.Raw()) != string(raw) {
		t.Fatalf("Raw() = %q, want %q", l.Raw(), raw)
	}
}
