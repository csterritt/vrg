package safepresent_test

import (
	"strings"
	"testing"

	"vrg/internal/safepresent"
)

// Path escapes raw path bytes into a single safe display line: newline,
// carriage return, and tab become backslash forms, a literal backslash
// doubles, invalid UTF-8 bytes become \xNN, other C0 controls and DEL use
// caret notation, C1 controls use \uXXXX escapes, and printable text —
// including valid multi-byte Unicode — passes through untouched.
func TestPathEscapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"plain", "dir/file.txt", "dir/file.txt"},
		{"newline", "a\nb", `a\nb`},
		{"carriage return", "a\rb", `a\rb`},
		{"tab", "a\tb", `a\tb`},
		{"backslash", `a\b`, `a\\b`},
		{"escape byte", "a\x1bb", `a^[b`},
		{"bell", "a\x07b", `a^Gb`},
		{"delete", "a\x7fb", `a^?b`},
		{"invalid utf-8 byte", "a\xffb", `a\xffb`},
		{"truncated utf-8", "a\xc2b", `a\xc2b`},
		{"c1 nel", "a\u0085b", `a\u0085b`},
		{"valid unicode", "héllo/世界", "héllo/世界"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresent.Path([]byte(tc.in)); got != tc.want {
				t.Fatalf("Path(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// cellText joins cell display texts into the rendered line.
func cellText(cells []safepresent.Cell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteString(c.Text)
	}
	return b.String()
}

// Content escapes one source line's raw bytes — its terminator already
// removed — into display cells: C0 controls and DEL use caret notation, a
// standalone carriage return renders as ^M, tab is the provisional
// single-cell arrow placeholder, C1 controls use \uXXXX escapes, invalid
// UTF-8 becomes U+FFFD, and printable text passes through.
func TestContentEscapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello", "hello"},
		{"escape byte", "\x1b", "^["},
		{"osc payload", "\x1b]0;x\x07", "^[]0;x^G"},
		{"csi payload", "\x1b[2J", "^[[2J"},
		{"bell", "\x07", "^G"},
		{"backspace", "\x08", "^H"},
		{"nul", "\x00", "^@"},
		{"unit separator", "\x1f", "^_"},
		{"delete", "\x7f", "^?"},
		{"standalone carriage return", "a\rb", "a^Mb"},
		{"tab placeholder", "a\tb", "a→b"},
		{"c1 nel", "a\u0085b", `a\u0085b`},
		{"invalid utf-8 byte", "a\xffb", "a\uFFFDb"},
		{"invalid utf-8 run", "a\xff\xfeb", "a\uFFFD\uFFFDb"},
		{"valid unicode", "héllo 世界", "héllo 世界"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cells := safepresent.Content([]byte(tc.in))
			if got := cellText(cells); got != tc.want {
				t.Fatalf("Content(%q) text = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// Every content cell maps back to the exact half-open source-byte range
// it presents, so a highlight over source bytes can cover every cell of
// an escaped form.
func TestContentCellByteMappings(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []safepresent.Cell
	}{
		{"plain", "ab", []safepresent.Cell{
			{Text: "a", Start: 0, End: 1},
			{Text: "b", Start: 1, End: 2},
		}},
		{"escape byte", "\x1b", []safepresent.Cell{
			{Text: "^", Start: 0, End: 1},
			{Text: "[", Start: 0, End: 1},
		}},
		{"c1 nel", "\u0085", []safepresent.Cell{
			{Text: `\`, Start: 0, End: 2},
			{Text: `u`, Start: 0, End: 2},
			{Text: `0`, Start: 0, End: 2},
			{Text: `0`, Start: 0, End: 2},
			{Text: `8`, Start: 0, End: 2},
			{Text: `5`, Start: 0, End: 2},
		}},
		{"invalid utf-8 byte", "\xff", []safepresent.Cell{
			{Text: "\uFFFD", Start: 0, End: 1},
		}},
		{"tab placeholder", "\t", []safepresent.Cell{
			{Text: "→", Start: 0, End: 1},
		}},
		{"multi-byte rune", "é", []safepresent.Cell{
			{Text: "é", Start: 0, End: 2},
		}},
		{"standalone carriage return", "\r", []safepresent.Cell{
			{Text: "^", Start: 0, End: 1},
			{Text: "M", Start: 0, End: 1},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := safepresent.Content([]byte(tc.in))
			if len(got) != len(tc.want) {
				t.Fatalf("Content(%q) produced %d cells %+v, want %d", tc.in, len(got), got, len(tc.want))
			}
			for i, c := range got {
				if c != tc.want[i] {
					t.Fatalf("Content(%q) cell %d = %+v, want %+v", tc.in, i, c, tc.want[i])
				}
			}
		})
	}
}

// Span maps a half-open source-byte range to the half-open cell range it
// covers: a match covering an ESC byte highlights both caret cells, and a
// match inside a multi-cell escape covers the whole escape.
func TestContentSpanMapping(t *testing.T) {
	cells := safepresent.Content([]byte("a\x1bb")) // renders a ^ [ b
	if got := cellText(cells); got != "a^[b" {
		t.Fatalf("fixture text = %q, want %q", got, "a^[b")
	}
	for _, tc := range []struct {
		name       string
		start, end int
		wantS      int
		wantE      int
	}{
		{"plain byte", 0, 1, 0, 1},
		{"escape byte covers caret pair", 1, 2, 1, 3},
		{"escape plus neighbour", 0, 2, 0, 3},
		{"trailing byte", 2, 3, 3, 4},
		{"whole line", 0, 3, 0, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := safepresent.Span(cells, tc.start, tc.end)
			if s != tc.wantS || e != tc.wantE {
				t.Fatalf("Span(%d, %d) = (%d, %d), want (%d, %d)",
					tc.start, tc.end, s, e, tc.wantS, tc.wantE)
			}
		})
	}
}

// A span over a multi-byte C1 escape covers all six cells of its
// \uXXXX-style form.
func TestContentSpanCoversC1Escape(t *testing.T) {
	cells := safepresent.Content([]byte("x\u0085y"))
	s, e := safepresent.Span(cells, 1, 3)
	if s != 1 || e != 7 {
		t.Fatalf("Span(1, 3) = (%d, %d), want (1, 7)", s, e)
	}
}
