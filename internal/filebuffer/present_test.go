package filebuffer

import (
	"strings"
	"testing"
)

// Path presentation: newline, carriage return, and tab take the short
// \n \r \t forms; backslash doubles; invalid UTF-8 bytes take \xNN;
// remaining C0 controls and DEL use caret notation; C1 controls use
// \uXXXX; valid printable Unicode passes through untouched.
func TestEscapePath(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"plain", "dir/sub/file.go", "dir/sub/file.go"},
		{"newline", "a\nb", `a\nb`},
		{"carriage return", "a\rb", `a\rb`},
		{"tab", "a\tb", `a\tb`},
		{"backslash", `a\b`, `a\\b`},
		{"escaped backslash keeps escapes distinct", `a\nb` + "\n", `a\\nb\n`},
		{"invalid utf-8 byte", "a\xffb", `a\xffb`},
		{"invalid utf-8 run", "a\xff\xfeb", `a\xff\xfeb`},
		{"truncated utf-8 tail", "a\xe2\x82", `a\xe2\x82`},
		{"escape", "a\x1bb", `a^[b`},
		{"bell", "a\x07b", `a^Gb`},
		{"nul", "a\x00b", `a^@b`},
		{"delete", "a\x7fb", `a^?b`},
		{"c1 nel", "a\xc2\x85b", "a\\u0085b"},
		{"c1 csi", "a\xc2\x9bb", "a\\u009bb"},
		{"printable unicode", "héllö→世", "héllö→世"},
		{"all forms together", "x\ny\tz\\w\xff\x1bv", `x\ny\tz\\w\xff^[v`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := EscapePath([]byte(tc.raw)); got != tc.want {
				t.Fatalf("EscapePath(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// An escaped path never emits a raw control byte: every byte that could
// move the terminal is spelled out.
func TestEscapePathNeverEmitsControls(t *testing.T) {
	for i := 0; i < 0x20; i++ {
		raw := []byte{'a', byte(i), 'b'}
		got := EscapePath(raw)
		for j := 0; j < len(got); j++ {
			if c := got[j]; c < 0x20 || c == 0x7f {
				t.Fatalf("EscapePath(%q) emitted control byte 0x%02x: %q", raw, c, got)
			}
		}
	}
}

// Content presentation: the display text of one source line. LF and
// CRLF are line terminators and produce no display; a standalone CR is
// escaped ^M; other C0 controls and DEL take caret notation; C1 takes
// \uXXXX; invalid UTF-8 becomes U+FFFD; tab renders as the provisional
// one-cell → placeholder until Issue 16's stop expansion.
func TestPresentLineText(t *testing.T) {
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
		{"tab placeholder", "a\tb\n", "a→b"},
		{"combining cluster", "e\u0301x\n", "e\u0301x"},
		{"printable unicode", "héllö→世\n", "héllö→世"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := presentLine([]byte(tc.raw))
			if p.Text() != tc.want {
				t.Fatalf("presentLine(%q).Text() = %q, want %q", tc.raw, p.Text(), tc.want)
			}
		})
	}
}

// Cell counts for the printable forms: ASCII is one cell per byte, a
// wide rune two, a combining cluster one, caret escapes two, \uXXXX six,
// U+FFFD one, and the provisional tab form a single → cell.
func TestPresentLineWidth(t *testing.T) {
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := presentLine([]byte(tc.raw)).Width(); got != tc.want {
				t.Fatalf("presentLine(%q).Width() = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

// Byte→cell mapping: every escaped form maps its source bytes to every
// display cell it produces, so a match covering an ESC byte highlights
// both ^ and [, a match inside a multi-byte unit covers the whole unit,
// removed terminator bytes map to the end-of-line position, and empty
// ranges yield a zero-width marker position.
func TestPresentLineSpan(t *testing.T) {
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
			if got := presentLine([]byte(tc.raw)).Span(tc.start, tc.end); got != tc.want {
				t.Fatalf("presentLine(%q).Span(%d,%d) = %+v, want %+v",
					tc.raw, tc.start, tc.end, got, tc.want)
			}
		})
	}
}

// A tab is never emitted raw and occupies exactly one provisional →
// cell; per Issue 5 nothing on a tab-containing line asserts specific
// cell positions beyond that single-cell form.
func TestPresentLineTabForm(t *testing.T) {
	p := presentLine([]byte("a\tb\n"))
	if p.Text() != "a→b" {
		t.Fatalf("tab presentation = %q, want %q", p.Text(), "a→b")
	}
	if got := p.Span(1, 2); got.Start != 1 || got.End != 2 {
		t.Fatalf("tab span = %+v, want exactly one cell", got)
	}
	if strings.Contains(p.Text(), "\t") {
		t.Fatalf("raw tab survived presentation: %q", p.Text())
	}
}

// Presented lines retain their raw bytes for identity, comparison, and
// later coordinate mapping.
func TestPresentLineRetainsRaw(t *testing.T) {
	raw := []byte("a\xff\x1bb\r\n")
	p := presentLine(raw)
	if string(p.Raw()) != string(raw) {
		t.Fatalf("Raw() = %q, want %q", p.Raw(), raw)
	}
}
